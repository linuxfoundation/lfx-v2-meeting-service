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

// PastMeetingRecordingService implements the business logic for past meeting recordings.
type PastMeetingRecordingService struct {
	pastMeetingRecordingRepository domain.PastMeetingRecordingRepository
	messageBuilder                 domain.MessageBuilder
	config                         ServiceConfig
}

// NewPastMeetingRecordingService creates a new PastMeetingRecordingService.
func NewPastMeetingRecordingService(
	pastMeetingRecordingRepository domain.PastMeetingRecordingRepository,
	messageBuilder domain.MessageBuilder,
	serviceConfig ServiceConfig,
) *PastMeetingRecordingService {
	return &PastMeetingRecordingService{
		pastMeetingRecordingRepository: pastMeetingRecordingRepository,
		messageBuilder:                 messageBuilder,
		config:                         serviceConfig,
	}
}

// ServiceReady checks if the service is ready to serve requests.
func (s *PastMeetingRecordingService) ServiceReady() bool {
	return s.pastMeetingRecordingRepository != nil && s.messageBuilder != nil
}

// ListRecordingsByPastMeeting returns all recordings for a given past meeting.
func (s *PastMeetingRecordingService) ListRecordingsByPastMeeting(ctx context.Context, pastMeetingUID string) ([]*models.PastMeetingRecording, error) {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "service not initialized", logging.PriorityCritical())
		return nil, domain.NewUnavailableError("service not initialized")
	}

	recordings, err := s.pastMeetingRecordingRepository.ListByPastMeeting(ctx, pastMeetingUID)
	if err != nil {
		slog.ErrorContext(ctx, "error listing recordings", logging.ErrKey, err, "past_meeting_uid", pastMeetingUID)
		return nil, err
	}

	return recordings, nil
}

// CreateRecording creates a new recording from a domain model.
func (s *PastMeetingRecordingService) CreateRecording(
	ctx context.Context,
	recording *models.PastMeetingRecording,
) (*models.PastMeetingRecording, error) {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "service not initialized", logging.PriorityCritical())
		return nil, domain.NewUnavailableError("service not initialized")
	}

	// Set system-generated fields
	now := time.Now().UTC()
	recording.UID = uuid.New().String()
	recording.CreatedAt = now
	recording.UpdatedAt = now

	// Create in repository
	err := s.pastMeetingRecordingRepository.Create(ctx, recording)
	if err != nil {
		slog.ErrorContext(ctx, "error creating recording", logging.ErrKey, err, "recording_uid", recording.UID)
		return nil, err
	}

	// Send indexing message for new recording
	err = s.messageBuilder.SendIndexPastMeetingRecording(ctx, models.ActionCreated, *recording)
	if err != nil {
		slog.ErrorContext(ctx, "error sending index message for new recording", logging.ErrKey, err, "recording_uid", recording.UID)
		// Don't fail the operation if indexing fails
	}

	slog.InfoContext(ctx, "created new past meeting recording",
		"recording_uid", recording.UID,
		"past_meeting_uid", recording.PastMeetingUID,
		"total_files", len(recording.RecordingFiles),
		"total_size", recording.TotalSize,
	)

	return recording, nil
}

// GetRecordingByPastMeetingUID retrieves a recording by past meeting UID.
func (s *PastMeetingRecordingService) GetRecordingByPastMeetingUID(ctx context.Context, pastMeetingUID string) (*models.PastMeetingRecording, error) {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "service not initialized", logging.PriorityCritical())
		return nil, domain.NewUnavailableError("service not initialized")
	}

	recording, err := s.pastMeetingRecordingRepository.GetByPastMeetingUID(ctx, pastMeetingUID)
	if err != nil {
		if domain.GetErrorType(err) == domain.ErrorTypeNotFound {
			slog.DebugContext(ctx, "recording not found for past meeting", "past_meeting_uid", pastMeetingUID)
		} else {
			slog.ErrorContext(ctx, "error getting recording", logging.ErrKey, err, "past_meeting_uid", pastMeetingUID)
		}
		return nil, err
	}

	return recording, nil
}

// UpdateRecording updates an existing recording with additional recording files.
func (s *PastMeetingRecordingService) UpdateRecording(
	ctx context.Context,
	recordingUID string,
	updatedRecording *models.PastMeetingRecording,
) (*models.PastMeetingRecording, error) {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "service not initialized", logging.PriorityCritical())
		return nil, domain.NewUnavailableError("service not initialized")
	}

	// Get current recording with revision
	currentRecording, revision, err := s.pastMeetingRecordingRepository.GetWithRevision(ctx, recordingUID)
	if err != nil {
		if domain.GetErrorType(err) == domain.ErrorTypeNotFound {
			slog.DebugContext(ctx, "recording not found for update", "recording_uid", recordingUID)
		} else {
			slog.ErrorContext(ctx, "error getting recording for update", logging.ErrKey, err, "recording_uid", recordingUID)
		}
		return nil, err
	}

	// Add new recording sessions to the existing recording
	for _, session := range updatedRecording.Sessions {
		currentRecording.AddRecordingSession(session)
	}

	// Add new recording files to the existing recording
	currentRecording.AddRecordingFiles(updatedRecording.RecordingFiles)

	// Update system fields
	currentRecording.UpdatedAt = time.Now().UTC()

	// Update in repository
	err = s.pastMeetingRecordingRepository.Update(ctx, currentRecording, revision)
	if err != nil {
		slog.ErrorContext(ctx, "error updating recording", logging.ErrKey, err, "recording_uid", recordingUID)
		return nil, err
	}

	// Send indexing message for updated recording
	err = s.messageBuilder.SendIndexPastMeetingRecording(ctx, models.ActionUpdated, *currentRecording)
	if err != nil {
		slog.ErrorContext(ctx, "error sending index message for updated recording", logging.ErrKey, err, "recording_uid", recordingUID)
		// Don't fail the operation if indexing fails
	}

	slog.InfoContext(ctx, "updated existing past meeting recording",
		"recording_uid", recordingUID,
		"past_meeting_uid", currentRecording.PastMeetingUID,
		"total_files", len(currentRecording.RecordingFiles),
		"total_size", currentRecording.TotalSize,
	)

	return currentRecording, nil
}

// DeleteRecording deletes a recording.
func (s *PastMeetingRecordingService) DeleteRecording(ctx context.Context, recordingUID string) error {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "service not initialized", logging.PriorityCritical())
		return domain.NewUnavailableError("service not initialized")
	}

	// Get the recording first to send delete message
	recording, revision, err := s.pastMeetingRecordingRepository.GetWithRevision(ctx, recordingUID)
	if err != nil {
		if domain.GetErrorType(err) == domain.ErrorTypeNotFound {
			slog.DebugContext(ctx, "recording not found for deletion", "recording_uid", recordingUID)
		} else {
			slog.ErrorContext(ctx, "error getting recording for deletion", logging.ErrKey, err, "recording_uid", recordingUID)
		}
		return err
	}

	// Delete from repository
	err = s.pastMeetingRecordingRepository.Delete(ctx, recordingUID, revision)
	if err != nil {
		slog.ErrorContext(ctx, "error deleting recording", logging.ErrKey, err, "recording_uid", recordingUID)
		return err
	}

	// Send indexing message for deleted recording
	err = s.messageBuilder.SendIndexPastMeetingRecording(ctx, models.ActionDeleted, *recording)
	if err != nil {
		slog.ErrorContext(ctx, "error sending index message for deleted recording", logging.ErrKey, err, "recording_uid", recordingUID)
		// Don't fail the operation if indexing fails
	}

	slog.InfoContext(ctx, "deleted past meeting recording", "recording_uid", recordingUID)
	return nil
}
