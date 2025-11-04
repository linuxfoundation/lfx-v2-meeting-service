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
)

// PastMeetingAttachmentService implements the service for past meeting attachments
type PastMeetingAttachmentService struct {
	pastMeetingRepository           domain.PastMeetingRepository
	pastMeetingAttachmentRepository domain.PastMeetingAttachmentRepository
	meetingAttachmentRepository     domain.MeetingAttachmentRepository
	config                          ServiceConfig
}

// NewPastMeetingAttachmentService creates a new PastMeetingAttachmentService
func NewPastMeetingAttachmentService(
	pastMeetingRepository domain.PastMeetingRepository,
	pastMeetingAttachmentRepository domain.PastMeetingAttachmentRepository,
	meetingAttachmentRepository domain.MeetingAttachmentRepository,
	config ServiceConfig,
) *PastMeetingAttachmentService {
	return &PastMeetingAttachmentService{
		pastMeetingRepository:           pastMeetingRepository,
		pastMeetingAttachmentRepository: pastMeetingAttachmentRepository,
		meetingAttachmentRepository:     meetingAttachmentRepository,
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

// CreatePastMeetingAttachment creates a new past meeting attachment
// Can either upload a new file or reference an existing file in Object Store
func (s *PastMeetingAttachmentService) CreatePastMeetingAttachment(ctx context.Context, req *models.CreatePastMeetingAttachmentRequest) (*models.PastMeetingAttachment, error) {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "service not initialized", logging.PriorityCritical())
		return nil, domain.NewUnavailableError("service not initialized")
	}

	// Validate request
	if req == nil {
		return nil, domain.NewValidationError("request is nil")
	}

	if req.PastMeetingUID == "" {
		return nil, domain.NewValidationError("past meeting UID is required")
	}

	// Must have either source_object_uid or file data
	if req.SourceObjectUID == "" && len(req.FileData) == 0 {
		return nil, domain.NewValidationError("either source_object_uid or file data is required")
	}

	// Cannot have both
	if req.SourceObjectUID != "" && len(req.FileData) > 0 {
		return nil, domain.NewValidationError("cannot specify both source_object_uid and file data")
	}

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

		// Check file size (100MB max)
		const maxFileSize = 100 * 1024 * 1024
		if len(req.FileData) > maxFileSize {
			return nil, domain.NewValidationError(fmt.Sprintf("file size exceeds maximum allowed size of %d bytes", maxFileSize))
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
			FileName:        req.FileName,
			FileSize:        int64(len(req.FileData)),
			ContentType:     req.ContentType,
			UploadedBy:      req.Username,
			UploadedAt:      &now,
			Description:     req.Description,
			SourceObjectUID: fileUID, // Reference the newly uploaded file
		}
	}

	// Store metadata in KV store
	if err := s.pastMeetingAttachmentRepository.PutMetadata(ctx, attachment); err != nil {
		slog.ErrorContext(ctx, "failed to store attachment metadata", logging.ErrKey, err, "attachment_uid", attachment.UID)
		return nil, err
	}

	slog.InfoContext(ctx, "created past meeting attachment",
		"past_meeting_uid", req.PastMeetingUID,
		"attachment_uid", attachment.UID,
		"source_object_uid", attachment.SourceObjectUID)

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

	return nil
}
