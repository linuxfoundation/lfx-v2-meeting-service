// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/concurrent"
)

// PastMeetingAttachmentService implements the service for past meeting attachments
type PastMeetingAttachmentService struct {
	pastMeetingRepository           domain.PastMeetingRepository
	pastMeetingAttachmentRepository domain.PastMeetingAttachmentRepository
	meetingAttachmentRepository     domain.MeetingAttachmentRepository
	indexSender                     domain.PastMeetingAttachmentIndexSender
	accessSender                    domain.PastMeetingAttachmentAccessSender
	config                          ServiceConfig
}

// NewPastMeetingAttachmentService creates a new PastMeetingAttachmentService
func NewPastMeetingAttachmentService(
	pastMeetingRepository domain.PastMeetingRepository,
	pastMeetingAttachmentRepository domain.PastMeetingAttachmentRepository,
	meetingAttachmentRepository domain.MeetingAttachmentRepository,
	indexSender domain.PastMeetingAttachmentIndexSender,
	accessSender domain.PastMeetingAttachmentAccessSender,
	config ServiceConfig,
) *PastMeetingAttachmentService {
	return &PastMeetingAttachmentService{
		pastMeetingRepository:           pastMeetingRepository,
		pastMeetingAttachmentRepository: pastMeetingAttachmentRepository,
		meetingAttachmentRepository:     meetingAttachmentRepository,
		indexSender:                     indexSender,
		accessSender:                    accessSender,
		config:                          config,
	}
}

// ServiceReady checks if the service is ready for use
func (s *PastMeetingAttachmentService) ServiceReady() bool {
	return s.pastMeetingRepository != nil &&
		s.pastMeetingAttachmentRepository != nil &&
		s.meetingAttachmentRepository != nil
}

// ListPastMeetingAttachments retrieves all attachments for a past meeting
func (s *PastMeetingAttachmentService) ListPastMeetingAttachments(ctx context.Context, pastMeetingUID string) ([]*models.PastMeetingAttachment, error) {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "service not initialized", logging.PriorityCritical())
		return nil, domain.NewUnavailableError("service not initialized")
	}

	if pastMeetingUID == "" {
		return nil, domain.NewValidationError("past meeting UID is required")
	}

	// Check if the past meeting exists
	exists, err := s.pastMeetingRepository.Exists(ctx, pastMeetingUID)
	if err != nil {
		slog.ErrorContext(ctx, "error checking if past meeting exists", logging.ErrKey, err)
		return nil, err
	}
	if !exists {
		slog.WarnContext(ctx, "past meeting not found", "past_meeting_uid", pastMeetingUID)
		return nil, domain.NewNotFoundError("past meeting not found")
	}

	// List attachments
	attachments, err := s.pastMeetingAttachmentRepository.ListByPastMeeting(ctx, pastMeetingUID)
	if err != nil {
		slog.ErrorContext(ctx, "error listing past meeting attachments", logging.ErrKey, err)
		return nil, err
	}

	slog.InfoContext(ctx, "listed past meeting attachments",
		"past_meeting_uid", pastMeetingUID,
		"count", len(attachments))

	return attachments, nil
}

func (s *PastMeetingAttachmentService) validateCreatePastMeetingAttachmentRequest(req *models.CreatePastMeetingAttachmentRequest) error {
	// Validate request
	if req == nil {
		return domain.NewValidationError("request is nil")
	}

	if req.PastMeetingUID == "" {
		return domain.NewValidationError("past meeting UID is required")
	}
	if req.Username == "" {
		return domain.NewValidationError("username is required")
	}

	// Validate name is provided
	if req.Name == "" {
		return domain.NewValidationError("name is required")
	}

	if req.Type != models.AttachmentTypeFile && req.Type != models.AttachmentTypeLink {
		return domain.NewValidationError("type must be either 'file' or 'link'")
	}

	// Type-specific validation
	switch req.Type {
	case models.AttachmentTypeLink:
		if req.Link == "" {
			return domain.NewValidationError("link is required when type is 'link'")
		}
		// Link-type attachments should not have file-related fields
		if req.SourceObjectUID != "" || len(req.FileData) > 0 {
			return domain.NewValidationError("link-type attachments cannot have source_object_uid or file data")
		}
	case models.AttachmentTypeFile:
		if req.FileName == "" {
			return domain.NewValidationError("file name is required when type is 'file'")
		}
		// File-type attachments must have either source_object_uid or file data
		if req.SourceObjectUID == "" && len(req.FileData) == 0 {
			return domain.NewValidationError("file-type attachments require either source_object_uid or file data")
		}
		if req.SourceObjectUID != "" && len(req.FileData) > 0 {
			return domain.NewValidationError("cannot specify both source_object_uid and file data")
		}
	}

	return nil
}

// CreatePastMeetingAttachment creates a new past meeting attachment
// Can create a link attachment or a file attachment (upload new file or reference existing file in Object Store)
func (s *PastMeetingAttachmentService) CreatePastMeetingAttachment(ctx context.Context, req *models.CreatePastMeetingAttachmentRequest) (*models.PastMeetingAttachment, error) {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "service not initialized", logging.PriorityCritical())
		return nil, domain.NewUnavailableError("service not initialized")
	}

	// Validate request
	if err := s.validateCreatePastMeetingAttachmentRequest(req); err != nil {
		return nil, err
	}

	ctx = logging.AppendCtx(ctx, slog.String("past_meeting_uid", req.PastMeetingUID))

	// Check if the past meeting exists
	exists, err := s.pastMeetingRepository.Exists(ctx, req.PastMeetingUID)
	if err != nil {
		slog.ErrorContext(ctx, "error checking if past meeting exists", logging.ErrKey, err)
		return nil, err
	}
	if !exists {
		slog.WarnContext(ctx, "past meeting not found", "past_meeting_uid", req.PastMeetingUID)
		return nil, domain.NewNotFoundError("past meeting not found")
	}

	var attachment *models.PastMeetingAttachment
	now := time.Now()

	switch req.Type {
	case models.AttachmentTypeLink:
		attachment = &models.PastMeetingAttachment{
			UID:            uuid.New().String(),
			PastMeetingUID: req.PastMeetingUID,
			Type:           models.AttachmentTypeLink,
			Link:           req.Link,
			Name:           req.Name,
			UploadedBy:     req.Username,
			UploadedAt:     &now,
			Description:    req.Description,
		}
	case models.AttachmentTypeFile:
		if req.SourceObjectUID != "" {
			// Reference existing file - need to get metadata from the source attachment
			sourceAttachment, err := s.meetingAttachmentRepository.GetMetadata(ctx, req.SourceObjectUID)
			if err != nil {
				slog.ErrorContext(ctx, "error getting source attachment metadata", logging.ErrKey, err)
				return nil, domain.NewNotFoundError("source attachment not found")
			}

			// Create past meeting attachment metadata referencing the existing file
			attachment = &models.PastMeetingAttachment{
				UID:             uuid.New().String(),
				PastMeetingUID:  req.PastMeetingUID,
				Type:            models.AttachmentTypeFile,
				Name:            req.Name,
				FileName:        sourceAttachment.FileName,
				FileSize:        sourceAttachment.FileSize,
				ContentType:     sourceAttachment.ContentType,
				UploadedBy:      req.Username,
				UploadedAt:      &now,
				Description:     req.Description,
				SourceObjectUID: req.SourceObjectUID,
			}
		} else {
			// Upload new file
			if req.FileName == "" {
				return nil, domain.NewValidationError("file name is required when uploading new file")
			}
			if req.Username == "" {
				return nil, domain.NewValidationError("username is required")
			}

			// Check file size
			if len(req.FileData) > MaxFileAttachmentSize {
				return nil, domain.NewValidationError(fmt.Sprintf("file size exceeds maximum allowed size of %d bytes", MaxFileAttachmentSize))
			}

			// Generate new UID for the file
			fileUID := uuid.New().String()

			// Upload file to Object Store
			if err := s.meetingAttachmentRepository.PutObject(ctx, fileUID, req.FileData); err != nil {
				slog.ErrorContext(ctx, "failed to upload attachment file", logging.ErrKey, err, "file_uid", fileUID)
				return nil, err
			}

			// Create past meeting attachment metadata
			attachment = &models.PastMeetingAttachment{
				UID:             uuid.New().String(),
				PastMeetingUID:  req.PastMeetingUID,
				Type:            models.AttachmentTypeFile,
				Name:            req.Name,
				FileName:        req.FileName,
				FileSize:        int64(len(req.FileData)),
				ContentType:     req.ContentType,
				UploadedBy:      req.Username,
				UploadedAt:      &now,
				Description:     req.Description,
				SourceObjectUID: fileUID, // Reference the newly uploaded file
			}
		}
	}

	// Store metadata
	if err := s.pastMeetingAttachmentRepository.PutMetadata(ctx, attachment); err != nil {
		slog.ErrorContext(ctx, "failed to store attachment metadata", logging.ErrKey, err, "attachment_uid", attachment.UID)
		return nil, err
	}

	slog.InfoContext(ctx, "created past meeting attachment",
		"past_meeting_uid", req.PastMeetingUID,
		"attachment_uid", attachment.UID,
		"type", attachment.Type,
		"source_object_uid", attachment.SourceObjectUID,
		"link", attachment.Link)

	// Send indexer and access control messages concurrently
	pool := concurrent.NewWorkerPool(2)
	errors := pool.RunAll(ctx,
		func() error {
			if s.indexSender == nil {
				return nil
			}
			if err := s.indexSender.SendIndexPastMeetingAttachment(ctx, models.ActionCreated, *attachment, false); err != nil {
				slog.WarnContext(ctx, "failed to send index message for past meeting attachment",
					logging.ErrKey, err,
					"attachment_uid", attachment.UID)
				return err
			}
			return nil
		},
		func() error {
			if s.accessSender == nil {
				return nil
			}
			accessMsg := models.PastMeetingAttachmentAccessMessage{
				UID:            attachment.UID,
				PastMeetingUID: attachment.PastMeetingUID,
			}
			if err := s.accessSender.SendUpdateAccessPastMeetingAttachment(ctx, accessMsg, false); err != nil {
				slog.WarnContext(ctx, "failed to send access control message for past meeting attachment",
					logging.ErrKey, err,
					"attachment_uid", attachment.UID)
				return err
			}
			return nil
		},
	)

	// Log any errors but don't fail the operation - attachment was created successfully
	if len(errors) > 0 {
		slog.WarnContext(ctx, "some messaging operations failed for past meeting attachment",
			"attachment_uid", attachment.UID,
			"error_count", len(errors))
	}

	return attachment, nil
}

// GetPastMeetingAttachment retrieves attachment metadata and file data
func (s *PastMeetingAttachmentService) GetPastMeetingAttachment(ctx context.Context, pastMeetingUID, attachmentUID string) (*models.PastMeetingAttachment, []byte, error) {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "service not initialized", logging.PriorityCritical())
		return nil, nil, domain.NewUnavailableError("service not initialized")
	}

	if pastMeetingUID == "" {
		return nil, nil, domain.NewValidationError("past meeting UID is required")
	}
	if attachmentUID == "" {
		return nil, nil, domain.NewValidationError("attachment UID is required")
	}

	// Get attachment metadata
	attachment, err := s.pastMeetingAttachmentRepository.GetMetadata(ctx, attachmentUID)
	if err != nil {
		return nil, nil, err
	}

	// Verify attachment belongs to the requested past meeting
	if attachment.PastMeetingUID != pastMeetingUID {
		slog.WarnContext(ctx, "attachment does not belong to past meeting",
			"attachment_uid", attachmentUID,
			"past_meeting_uid", pastMeetingUID,
			"attachment_past_meeting_uid", attachment.PastMeetingUID)
		return nil, nil, domain.NewNotFoundError("attachment not found for this past meeting")
	}

	if attachment.Type == "link" {
		slog.WarnContext(ctx, "attempted to download link-type attachment",
			"attachment_uid", attachmentUID,
			"link", attachment.Link)
		return nil, nil, domain.NewValidationError("cannot download link-type attachments, use get metadata endpoint instead")
	}

	// Get file data from Object Store using the source_object_uid
	fileData, err := s.meetingAttachmentRepository.GetObject(ctx, attachment.SourceObjectUID)
	if err != nil {
		slog.ErrorContext(ctx, "error getting file from object store",
			logging.ErrKey, err,
			"source_object_uid", attachment.SourceObjectUID)
		return nil, nil, err
	}

	slog.InfoContext(ctx, "retrieved past meeting attachment with file data",
		"past_meeting_uid", pastMeetingUID,
		"attachment_uid", attachmentUID,
		"source_object_uid", attachment.SourceObjectUID,
		"file_size", len(fileData))

	return attachment, fileData, nil
}

// GetPastMeetingAttachmentMetadata retrieves only the metadata for a past meeting attachment
func (s *PastMeetingAttachmentService) GetPastMeetingAttachmentMetadata(ctx context.Context, pastMeetingUID, attachmentUID string) (*models.PastMeetingAttachment, error) {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "service not initialized", logging.PriorityCritical())
		return nil, domain.NewUnavailableError("service not initialized")
	}

	if pastMeetingUID == "" {
		return nil, domain.NewValidationError("past meeting UID is required")
	}
	if attachmentUID == "" {
		return nil, domain.NewValidationError("attachment UID is required")
	}

	// Get attachment metadata
	attachment, err := s.pastMeetingAttachmentRepository.GetMetadata(ctx, attachmentUID)
	if err != nil {
		return nil, err
	}

	// Verify attachment belongs to the requested past meeting
	if attachment.PastMeetingUID != pastMeetingUID {
		slog.WarnContext(ctx, "attachment does not belong to past meeting",
			"attachment_uid", attachmentUID,
			"past_meeting_uid", pastMeetingUID,
			"attachment_past_meeting_uid", attachment.PastMeetingUID)
		return nil, domain.NewNotFoundError("attachment not found for this past meeting")
	}

	slog.InfoContext(ctx, "retrieved past meeting attachment metadata",
		"past_meeting_uid", pastMeetingUID,
		"attachment_uid", attachmentUID)

	return attachment, nil
}

// DeletePastMeetingAttachment deletes a past meeting attachment
func (s *PastMeetingAttachmentService) DeletePastMeetingAttachment(ctx context.Context, pastMeetingUID, attachmentUID string) error {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "service not initialized", logging.PriorityCritical())
		return domain.NewUnavailableError("service not initialized")
	}

	if pastMeetingUID == "" {
		return domain.NewValidationError("past meeting UID is required")
	}
	if attachmentUID == "" {
		return domain.NewValidationError("attachment UID is required")
	}

	// Check if the past meeting exists
	exists, err := s.pastMeetingRepository.Exists(ctx, pastMeetingUID)
	if err != nil {
		slog.ErrorContext(ctx, "error checking if past meeting exists", logging.ErrKey, err)
		return err
	}
	if !exists {
		slog.WarnContext(ctx, "past meeting not found", "past_meeting_uid", pastMeetingUID)
		return domain.NewNotFoundError("past meeting not found")
	}

	// Get attachment to verify it belongs to this past meeting
	attachment, err := s.pastMeetingAttachmentRepository.GetMetadata(ctx, attachmentUID)
	if err != nil {
		slog.ErrorContext(ctx, "error getting past meeting attachment", logging.ErrKey, err)
		return err
	}

	// Verify attachment belongs to the specified past meeting
	if attachment.PastMeetingUID != pastMeetingUID {
		slog.WarnContext(ctx, "attachment does not belong to past meeting",
			"attachment_uid", attachmentUID,
			"past_meeting_uid", pastMeetingUID,
			"attachment_past_meeting_uid", attachment.PastMeetingUID)
		return domain.NewNotFoundError("attachment not found for this past meeting")
	}

	// Delete the attachment metadata
	err = s.pastMeetingAttachmentRepository.Delete(ctx, attachmentUID)
	if err != nil {
		slog.ErrorContext(ctx, "error deleting past meeting attachment", logging.ErrKey, err)
		return err
	}

	slog.InfoContext(ctx, "deleted past meeting attachment",
		"past_meeting_uid", pastMeetingUID,
		"attachment_uid", attachmentUID)

	// Send indexer and access control delete messages concurrently
	pool := concurrent.NewWorkerPool(2)
	errors := pool.RunAll(ctx,
		func() error {
			if s.indexSender == nil {
				return nil
			}
			if err := s.indexSender.SendDeleteIndexPastMeetingAttachment(ctx, attachmentUID, false); err != nil {
				slog.WarnContext(ctx, "failed to send delete index message for past meeting attachment",
					logging.ErrKey, err,
					"attachment_uid", attachmentUID)
				return err
			}
			return nil
		},
		func() error {
			if s.accessSender == nil {
				return nil
			}
			if err := s.accessSender.SendDeleteAccessPastMeetingAttachment(ctx, attachmentUID, false); err != nil {
				slog.WarnContext(ctx, "failed to send delete access control message for past meeting attachment",
					logging.ErrKey, err,
					"attachment_uid", attachmentUID)
				return err
			}
			return nil
		},
	)

	// Log any errors but don't fail the operation - attachment was deleted successfully
	if len(errors) > 0 {
		slog.WarnContext(ctx, "some messaging operations failed for past meeting attachment deletion",
			"attachment_uid", attachmentUID,
			"error_count", len(errors))
	}

	return nil
}
