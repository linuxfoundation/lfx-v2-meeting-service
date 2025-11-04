// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"log/slog"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
)

// PastMeetingAttachmentService implements the service for past meeting attachments
type PastMeetingAttachmentService struct {
	pastMeetingRepository           domain.PastMeetingRepository
	pastMeetingAttachmentRepository domain.PastMeetingAttachmentRepository
	config                          ServiceConfig
}

// NewPastMeetingAttachmentService creates a new PastMeetingAttachmentService
func NewPastMeetingAttachmentService(
	pastMeetingRepository domain.PastMeetingRepository,
	pastMeetingAttachmentRepository domain.PastMeetingAttachmentRepository,
	config ServiceConfig,
) *PastMeetingAttachmentService {
	return &PastMeetingAttachmentService{
		pastMeetingRepository:           pastMeetingRepository,
		pastMeetingAttachmentRepository: pastMeetingAttachmentRepository,
		config:                          config,
	}
}

// ServiceReady checks if the service is ready for use
func (s *PastMeetingAttachmentService) ServiceReady() bool {
	return s.pastMeetingRepository != nil &&
		s.pastMeetingAttachmentRepository != nil
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
