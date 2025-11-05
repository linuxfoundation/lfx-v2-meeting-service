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

const (
	// MaxAttachmentSize is the maximum size for file uploads (100MB)
	MaxAttachmentSize = 100 * 1024 * 1024
)

// MeetingAttachmentService implements meeting attachment operations
type MeetingAttachmentService struct {
	attachmentRepository domain.MeetingAttachmentRepository
	meetingRepository    domain.MeetingRepository
}

// NewMeetingAttachmentService creates a new MeetingAttachmentService.
func NewMeetingAttachmentService(
	attachmentRepository domain.MeetingAttachmentRepository,
	meetingRepository domain.MeetingRepository,
) *MeetingAttachmentService {
	return &MeetingAttachmentService{
		attachmentRepository: attachmentRepository,
		meetingRepository:    meetingRepository,
	}
}

// ServiceReady checks if the service is ready for use.
func (s *MeetingAttachmentService) ServiceReady() bool {
	return s.attachmentRepository != nil && s.meetingRepository != nil
}

// UploadAttachment uploads a file or link attachment for a meeting
func (s *MeetingAttachmentService) UploadAttachment(ctx context.Context, req *models.UploadAttachmentRequest) (*models.MeetingAttachment, error) {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "service not initialized", logging.PriorityCritical())
		return nil, domain.NewUnavailableError("attachment service is not ready")
	}

	// Validate request
	if req == nil {
		return nil, domain.NewValidationError("request is nil")
	}

	ctx = logging.AppendCtx(ctx, slog.String("meeting_uid", req.MeetingUID))

	// Validate inputs
	if req.MeetingUID == "" {
		return nil, domain.NewValidationError("meeting UID is required")
	}
	if req.Username == "" {
		return nil, domain.NewValidationError("username is required")
	}

	// Validate type
	if req.Type != "file" && req.Type != "link" {
		return nil, domain.NewValidationError("type must be either 'file' or 'link'")
	}

	// Type-specific validation
	if req.Type == "link" {
		if req.Link == "" {
			return nil, domain.NewValidationError("link is required when type is 'link'")
		}
		// Link-type attachments should not have file-related fields
		if len(req.FileData) > 0 {
			return nil, domain.NewValidationError("link-type attachments cannot have file data")
		}
	} else if req.Type == "file" {
		if req.FileName == "" {
			return nil, domain.NewValidationError("file name is required when type is 'file'")
		}
		if len(req.FileData) == 0 {
			return nil, domain.NewValidationError("file data is required when type is 'file'")
		}
		// Check file size
		if len(req.FileData) > MaxAttachmentSize {
			return nil, domain.NewValidationError(fmt.Sprintf("file size exceeds maximum allowed size of %d bytes", MaxAttachmentSize))
		}
	}

	// Validate name is provided
	if req.Name == "" {
		return nil, domain.NewValidationError("name is required")
	}

	// Verify meeting exists
	_, err := s.meetingRepository.GetBase(ctx, req.MeetingUID)
	if err != nil {
		return nil, err
	}

	// Create attachment metadata
	now := time.Now()
	var attachment *models.MeetingAttachment

	if req.Type == "link" {
		// Create link-type attachment (metadata only, no file storage)
		attachment = &models.MeetingAttachment{
			UID:         uuid.New().String(),
			MeetingUID:  req.MeetingUID,
			Type:        "link",
			Link:        req.Link,
			Name:        req.Name,
			UploadedBy:  req.Username,
			UploadedAt:  &now,
			Description: req.Description,
		}
	} else {
		// Create file-type attachment
		attachment = &models.MeetingAttachment{
			UID:         uuid.New().String(),
			MeetingUID:  req.MeetingUID,
			Type:        "file",
			Name:        req.Name,
			FileName:    req.FileName,
			FileSize:    int64(len(req.FileData)),
			ContentType: req.ContentType,
			UploadedBy:  req.Username,
			UploadedAt:  &now,
			Description: req.Description,
		}

		// Upload file to Object Store first
		if err := s.attachmentRepository.PutObject(ctx, attachment.UID, req.FileData); err != nil {
			slog.ErrorContext(ctx, "failed to upload attachment file", logging.ErrKey, err, "attachment_uid", attachment.UID)
			return nil, err
		}
	}

	// Create metadata in KV store
	if err := s.attachmentRepository.PutMetadata(ctx, attachment); err != nil {
		slog.ErrorContext(ctx, "failed to create attachment metadata", logging.ErrKey, err, "attachment_uid", attachment.UID)
		// Note: File remains in Object Store even if metadata creation fails
		// This is acceptable as orphaned files can be cleaned up separately
		return nil, err
	}

	slog.InfoContext(ctx, "uploaded attachment",
		"attachment_uid", attachment.UID,
		"type", attachment.Type,
		"file_size", attachment.FileSize,
		"link", attachment.Link)

	return attachment, nil
}

// GetAttachment retrieves a file attachment by UID
func (s *MeetingAttachmentService) GetAttachment(ctx context.Context, meetingUID, attachmentUID string) (*models.MeetingAttachment, []byte, error) {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "service not initialized", logging.PriorityCritical())
		return nil, nil, domain.NewUnavailableError("attachment service is not ready")
	}

	ctx = logging.AppendCtx(ctx, slog.String("meeting_uid", meetingUID))
	ctx = logging.AppendCtx(ctx, slog.String("attachment_uid", attachmentUID))

	// Validate inputs
	if meetingUID == "" {
		return nil, nil, domain.NewValidationError("meeting UID is required")
	}
	if attachmentUID == "" {
		return nil, nil, domain.NewValidationError("attachment UID is required")
	}

	// Get attachment metadata
	attachment, err := s.attachmentRepository.GetMetadata(ctx, attachmentUID)
	if err != nil {
		return nil, nil, err
	}

	// Verify attachment belongs to the requested meeting
	if attachment.MeetingUID != meetingUID {
		return nil, nil, domain.NewNotFoundError("attachment not found for this meeting")
	}

	// Cannot download link-type attachments
	if attachment.Type == "link" {
		slog.WarnContext(ctx, "attempted to download link-type attachment",
			"attachment_uid", attachmentUID,
			"link", attachment.Link)
		return nil, nil, domain.NewValidationError("cannot download link-type attachments, use get metadata endpoint instead")
	}

	// Get file data
	fileData, err := s.attachmentRepository.GetObject(ctx, attachmentUID)
	if err != nil {
		return nil, nil, err
	}

	return attachment, fileData, nil
}

// GetAttachmentMetadata retrieves only the metadata for an attachment without downloading the file
func (s *MeetingAttachmentService) GetAttachmentMetadata(ctx context.Context, meetingUID, attachmentUID string) (*models.MeetingAttachment, error) {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "service not initialized", logging.PriorityCritical())
		return nil, domain.NewUnavailableError("attachment service is not ready")
	}

	ctx = logging.AppendCtx(ctx, slog.String("meeting_uid", meetingUID))
	ctx = logging.AppendCtx(ctx, slog.String("attachment_uid", attachmentUID))

	// Validate inputs
	if meetingUID == "" {
		return nil, domain.NewValidationError("meeting UID is required")
	}
	if attachmentUID == "" {
		return nil, domain.NewValidationError("attachment UID is required")
	}

	// Get attachment metadata
	attachment, err := s.attachmentRepository.GetMetadata(ctx, attachmentUID)
	if err != nil {
		return nil, err
	}

	// Verify attachment belongs to the requested meeting
	if attachment.MeetingUID != meetingUID {
		return nil, domain.NewNotFoundError("attachment not found for this meeting")
	}

	return attachment, nil
}

// DeleteAttachment deletes a file attachment by UID
func (s *MeetingAttachmentService) DeleteAttachment(ctx context.Context, meetingUID, attachmentUID string) error {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "service not initialized", logging.PriorityCritical())
		return domain.NewUnavailableError("attachment service is not ready")
	}

	ctx = logging.AppendCtx(ctx, slog.String("meeting_uid", meetingUID))
	ctx = logging.AppendCtx(ctx, slog.String("attachment_uid", attachmentUID))

	// Validate inputs
	if meetingUID == "" {
		return domain.NewValidationError("meeting UID is required")
	}
	if attachmentUID == "" {
		return domain.NewValidationError("attachment UID is required")
	}

	// First verify the attachment exists and belongs to this meeting
	attachment, err := s.attachmentRepository.GetMetadata(ctx, attachmentUID)
	if err != nil {
		return err
	}

	// Verify attachment belongs to the requested meeting
	if attachment.MeetingUID != meetingUID {
		return domain.NewNotFoundError("attachment not found for this meeting")
	}

	// Delete the attachment
	if err := s.attachmentRepository.Delete(ctx, attachmentUID); err != nil {
		slog.ErrorContext(ctx, "failed to delete attachment", logging.ErrKey, err, "attachment_uid", attachmentUID)
		return err
	}

	slog.InfoContext(ctx, "deleted attachment", "attachment_uid", attachmentUID)

	return nil
}
