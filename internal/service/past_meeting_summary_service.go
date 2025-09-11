// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
)

// PastMeetingSummaryService implements the business logic for past meeting summaries.
type PastMeetingSummaryService struct {
	PastMeetingSummaryRepository domain.PastMeetingSummaryRepository
	PastMeetingRepository        domain.PastMeetingRepository
	MessageBuilder               domain.MessageBuilder
	ServiceConfig                ServiceConfig
}

// NewPastMeetingSummaryService creates a new PastMeetingSummaryService.
func NewPastMeetingSummaryService(
	pastMeetingSummaryRepository domain.PastMeetingSummaryRepository,
	pastMeetingRepository domain.PastMeetingRepository,
	messageBuilder domain.MessageBuilder,
	serviceConfig ServiceConfig,
) *PastMeetingSummaryService {
	return &PastMeetingSummaryService{
		PastMeetingSummaryRepository: pastMeetingSummaryRepository,
		PastMeetingRepository:        pastMeetingRepository,
		MessageBuilder:               messageBuilder,
		ServiceConfig:                serviceConfig,
	}
}

// ServiceReady checks if the service is ready to serve requests.
func (s *PastMeetingSummaryService) ServiceReady() bool {
	return s.PastMeetingSummaryRepository != nil && s.MessageBuilder != nil
}

// ListSummariesByPastMeeting returns all summaries for a given past meeting.
func (s *PastMeetingSummaryService) ListSummariesByPastMeeting(ctx context.Context, pastMeetingUID string) ([]*models.PastMeetingSummary, error) {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "service not initialized", logging.PriorityCritical())
		return nil, domain.ErrServiceUnavailable
	}

	// Validate that the past meeting exists
	_, err := s.PastMeetingRepository.Get(ctx, pastMeetingUID)
	if err != nil {
		if err == domain.ErrPastMeetingNotFound {
			slog.WarnContext(ctx, "past meeting not found", "past_meeting_uid", pastMeetingUID)
			return nil, domain.ErrPastMeetingNotFound
		}
		slog.ErrorContext(ctx, "error checking past meeting existence", logging.ErrKey, err, "past_meeting_uid", pastMeetingUID)
		return nil, domain.ErrInternal
	}

	summaries, err := s.PastMeetingSummaryRepository.ListByPastMeeting(ctx, pastMeetingUID)
	if err != nil {
		slog.ErrorContext(ctx, "error listing summaries", logging.ErrKey, err, "past_meeting_uid", pastMeetingUID)
		return nil, domain.ErrInternal
	}

	return summaries, nil
}

// CreateSummary creates a new summary from a domain model.
func (s *PastMeetingSummaryService) CreateSummary(
	ctx context.Context,
	summary *models.PastMeetingSummary,
) (*models.PastMeetingSummary, error) {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "service not initialized", logging.PriorityCritical())
		return nil, domain.ErrServiceUnavailable
	}

	// Set system-generated fields
	now := time.Now().UTC()
	summary.UID = uuid.New().String()
	summary.CreatedAt = now
	summary.UpdatedAt = now

	// Create in repository
	err := s.PastMeetingSummaryRepository.Create(ctx, summary)
	if err != nil {
		slog.ErrorContext(ctx, "error creating summary", logging.ErrKey, err, "summary_uid", summary.UID)
		return nil, domain.ErrInternal
	}

	// Send indexing message for new summary
	err = s.MessageBuilder.SendIndexPastMeetingSummary(ctx, models.ActionCreated, *summary)
	if err != nil {
		slog.ErrorContext(ctx, "error sending index message for new summary", logging.ErrKey, err, "summary_uid", summary.UID)
		// Don't fail the operation if indexing fails
	}

	slog.InfoContext(ctx, "created new past meeting summary",
		"summary_uid", summary.UID,
		"past_meeting_uid", summary.PastMeetingUID,
		"platform", summary.Platform,
	)

	return summary, nil
}

// GetSummary retrieves a summary by UID.
func (s *PastMeetingSummaryService) GetSummary(ctx context.Context, summaryUID string) (*models.PastMeetingSummary, error) {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "service not initialized", logging.PriorityCritical())
		return nil, domain.ErrServiceUnavailable
	}

	summary, err := s.PastMeetingSummaryRepository.Get(ctx, summaryUID)
	if err != nil {
		if err == domain.ErrPastMeetingSummaryNotFound {
			slog.DebugContext(ctx, "summary not found", "summary_uid", summaryUID)
			return nil, err
		}
		slog.ErrorContext(ctx, "error getting summary", logging.ErrKey, err, "summary_uid", summaryUID)
		return nil, domain.ErrInternal
	}

	return summary, nil
}

// GetSummaryByPastMeetingUID retrieves a summary by past meeting UID.
func (s *PastMeetingSummaryService) GetSummaryByPastMeetingUID(ctx context.Context, pastMeetingUID string) (*models.PastMeetingSummary, error) {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "service not initialized", logging.PriorityCritical())
		return nil, domain.ErrServiceUnavailable
	}

	summary, err := s.PastMeetingSummaryRepository.GetByPastMeetingUID(ctx, pastMeetingUID)
	if err != nil {
		if err == domain.ErrPastMeetingSummaryNotFound {
			slog.DebugContext(ctx, "summary not found for past meeting", "past_meeting_uid", pastMeetingUID)
			return nil, err
		}
		slog.ErrorContext(ctx, "error getting summary", logging.ErrKey, err, "past_meeting_uid", pastMeetingUID)
		return nil, domain.ErrInternal
	}

	return summary, nil
}

// UpdateSummary updates an existing summary.
func (s *PastMeetingSummaryService) UpdateSummary(
	ctx context.Context,
	summaryUID string,
	updatedSummary *models.PastMeetingSummary,
) (*models.PastMeetingSummary, error) {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "service not initialized", logging.PriorityCritical())
		return nil, domain.ErrServiceUnavailable
	}

	// Get current summary with revision
	currentSummary, revision, err := s.PastMeetingSummaryRepository.GetWithRevision(ctx, summaryUID)
	if err != nil {
		if err == domain.ErrPastMeetingSummaryNotFound {
			slog.DebugContext(ctx, "summary not found for update", "summary_uid", summaryUID)
			return nil, err
		}
		slog.ErrorContext(ctx, "error getting summary for update", logging.ErrKey, err, "summary_uid", summaryUID)
		return nil, domain.ErrInternal
	}

	// Update the summary with new data
	updatedSummary.UID = currentSummary.UID
	updatedSummary.CreatedAt = currentSummary.CreatedAt
	updatedSummary.UpdatedAt = time.Now().UTC()

	// Update in repository
	err = s.PastMeetingSummaryRepository.Update(ctx, updatedSummary, revision)
	if err != nil {
		slog.ErrorContext(ctx, "error updating summary", logging.ErrKey, err, "summary_uid", summaryUID)
		return nil, domain.ErrInternal
	}

	// Send indexing message for updated summary
	err = s.MessageBuilder.SendIndexPastMeetingSummary(ctx, models.ActionUpdated, *updatedSummary)
	if err != nil {
		slog.ErrorContext(ctx, "error sending index message for updated summary", logging.ErrKey, err, "summary_uid", summaryUID)
		// Don't fail the operation if indexing fails
	}

	slog.InfoContext(ctx, "updated existing past meeting summary",
		"summary_uid", summaryUID,
		"past_meeting_uid", updatedSummary.PastMeetingUID,
	)

	return updatedSummary, nil
}

// DeleteSummary deletes a summary.
func (s *PastMeetingSummaryService) DeleteSummary(ctx context.Context, summaryUID string) error {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "service not initialized", logging.PriorityCritical())
		return domain.ErrServiceUnavailable
	}

	// Get the summary first to send delete message
	summary, revision, err := s.PastMeetingSummaryRepository.GetWithRevision(ctx, summaryUID)
	if err != nil {
		if err == domain.ErrPastMeetingSummaryNotFound {
			slog.DebugContext(ctx, "summary not found for deletion", "summary_uid", summaryUID)
			return err
		}
		slog.ErrorContext(ctx, "error getting summary for deletion", logging.ErrKey, err, "summary_uid", summaryUID)
		return domain.ErrInternal
	}

	// Delete from repository
	err = s.PastMeetingSummaryRepository.Delete(ctx, summaryUID, revision)
	if err != nil {
		slog.ErrorContext(ctx, "error deleting summary", logging.ErrKey, err, "summary_uid", summaryUID)
		return domain.ErrInternal
	}

	// Send indexing message for deleted summary
	err = s.MessageBuilder.SendIndexPastMeetingSummary(ctx, models.ActionDeleted, *summary)
	if err != nil {
		slog.ErrorContext(ctx, "error sending index message for deleted summary", logging.ErrKey, err, "summary_uid", summaryUID)
		// Don't fail the operation if indexing fails
	}

	slog.InfoContext(ctx, "deleted past meeting summary", "summary_uid", summaryUID)
	return nil
}
