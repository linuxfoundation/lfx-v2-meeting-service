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

// UploadAttachment uploads a file attachment for a meeting
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
	if req.FileName == "" {
		return nil, domain.NewValidationError("file name is required")
	}
	if len(req.FileData) == 0 {
		return nil, domain.NewValidationError("file data is required")
	}

	// Check file size
	if len(req.FileData) > MaxAttachmentSize {
		return nil, domain.NewValidationError(fmt.Sprintf("file size exceeds maximum allowed size of %d bytes", MaxAttachmentSize))
	}

	// Verify meeting exists
	_, err := s.meetingRepository.GetBase(ctx, req.MeetingUID)
	if err != nil {
		return nil, err
	}

	// Create attachment metadata
	now := time.Now()
	attachment := &models.MeetingAttachment{
		UID:         uuid.New().String(),
		MeetingUID:  req.MeetingUID,
		FileName:    req.FileName,
		FileSize:    int64(len(req.FileData)),
		ContentType: req.ContentType,
		UploadedBy:  req.Username,
		UploadedAt:  &now,
		Description: req.Description,
	}

	// Store the attachment
	if err := s.attachmentRepository.Put(ctx, attachment, req.FileData); err != nil {
		slog.ErrorContext(ctx, "failed to upload attachment", logging.ErrKey, err, "attachment_uid", attachment.UID)
		return nil, err
	}

	slog.InfoContext(ctx, "uploaded attachment", "attachment_uid", attachment.UID, "file_size", attachment.FileSize)

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

	// Get attachment metadata and file data
	attachment, fileData, err := s.attachmentRepository.Get(ctx, attachmentUID)
	if err != nil {
		return nil, nil, err
	}

	// Verify attachment belongs to the requested meeting
	if attachment.MeetingUID != meetingUID {
		return nil, nil, domain.NewNotFoundError("attachment not found for this meeting")
	}

	return attachment, fileData, nil
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
	attachment, err := s.attachmentRepository.GetInfo(ctx, attachmentUID)
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
