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
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/concurrent"
)

// PastMeetingRecordingService implements the business logic for past meeting recordings.
type PastMeetingRecordingService struct {
	pastMeetingRecordingRepository   domain.PastMeetingRecordingRepository
	pastMeetingRepository            domain.PastMeetingRepository
	pastMeetingParticipantRepository domain.PastMeetingParticipantRepository
	messageSender                    domain.PastMeetingRecordingMessageSender
	config                           ServiceConfig
}

// NewPastMeetingRecordingService creates a new PastMeetingRecordingService.
func NewPastMeetingRecordingService(
	pastMeetingRecordingRepository domain.PastMeetingRecordingRepository,
	pastMeetingRepository domain.PastMeetingRepository,
	pastMeetingParticipantRepository domain.PastMeetingParticipantRepository,
	messageSender domain.PastMeetingRecordingMessageSender,
	serviceConfig ServiceConfig,
) *PastMeetingRecordingService {
	return &PastMeetingRecordingService{
		pastMeetingRecordingRepository:   pastMeetingRecordingRepository,
		pastMeetingRepository:            pastMeetingRepository,
		pastMeetingParticipantRepository: pastMeetingParticipantRepository,
		messageSender:                    messageSender,
		config:                           serviceConfig,
	}
}

// ServiceReady checks if the service is ready to serve requests.
func (s *PastMeetingRecordingService) ServiceReady() bool {
	return s.pastMeetingRecordingRepository != nil &&
		s.pastMeetingRepository != nil &&
		s.pastMeetingParticipantRepository != nil &&
		s.messageSender != nil
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

	// Use WorkerPool for concurrent NATS message sending
	pool := concurrent.NewWorkerPool(2) // 2 messages to send
	messages := []func() error{
		func() error {
			return s.messageSender.SendIndexPastMeetingRecording(ctx, models.ActionCreated, *recording)
		},
		func() error {
			// Get past meeting to retrieve artifact visibility
			pastMeeting, err := s.pastMeetingRepository.Get(ctx, recording.PastMeetingUID)
			if err != nil {
				return err
			}

			// Get participants for the past meeting
			participantPointers, err := s.pastMeetingParticipantRepository.ListByPastMeeting(ctx, recording.PastMeetingUID)
			if err != nil {
				return err
			}

			// Convert to simplified access participants
			participants := make([]models.AccessParticipant, len(participantPointers))
			for i, p := range participantPointers {
				participants[i] = models.AccessParticipant{
					Username: p.Username,
					Host:     p.Host,
				}
			}

			return s.messageSender.SendUpdateAccessPastMeetingRecording(ctx, models.PastMeetingRecordingAccessMessage{
				UID:                recording.UID,
				PastMeetingUID:     recording.PastMeetingUID,
				ArtifactVisibility: pastMeeting.ArtifactVisibility,
				Participants:       participants,
			})
		},
	}

	if err := pool.Run(ctx, messages...); err != nil {
		slog.ErrorContext(ctx, "failed to send NATS messages", logging.ErrKey, err)
		// Don't fail the operation if messaging fails
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

// GetRecordingByPlatformMeetingInstanceID retrieves a recording by platform and meeting instance ID.
func (s *PastMeetingRecordingService) GetRecordingByPlatformMeetingInstanceID(ctx context.Context, platform, platformMeetingInstanceID string) (*models.PastMeetingRecording, error) {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "service not initialized", logging.PriorityCritical())
		return nil, domain.NewUnavailableError("service not initialized")
	}

	recording, err := s.pastMeetingRecordingRepository.GetByPlatformMeetingInstanceID(ctx, platform, platformMeetingInstanceID)
	if err != nil {
		if domain.GetErrorType(err) == domain.ErrorTypeNotFound {
			slog.DebugContext(ctx, "recording not found for platform instance", "platform", platform, "platform_meeting_instance_id", platformMeetingInstanceID)
		} else {
			slog.ErrorContext(ctx, "error getting recording by platform instance ID", logging.ErrKey, err, "platform", platform, "platform_meeting_instance_id", platformMeetingInstanceID)
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

	// Use WorkerPool for concurrent NATS message sending
	pool := concurrent.NewWorkerPool(2) // 2 messages to send
	messages := []func() error{
		func() error {
			return s.messageSender.SendIndexPastMeetingRecording(ctx, models.ActionUpdated, *currentRecording)
		},
		func() error {
			// Get past meeting to retrieve artifact visibility
			pastMeeting, err := s.pastMeetingRepository.Get(ctx, currentRecording.PastMeetingUID)
			if err != nil {
				return err
			}

			// Get participants for the past meeting
			participantPointers, err := s.pastMeetingParticipantRepository.ListByPastMeeting(ctx, currentRecording.PastMeetingUID)
			if err != nil {
				return err
			}

			// Convert to simplified access participants
			participants := make([]models.AccessParticipant, len(participantPointers))
			for i, p := range participantPointers {
				participants[i] = models.AccessParticipant{
					Username: p.Username,
					Host:     p.Host,
				}
			}

			return s.messageSender.SendUpdateAccessPastMeetingRecording(ctx, models.PastMeetingRecordingAccessMessage{
				UID:                currentRecording.UID,
				PastMeetingUID:     currentRecording.PastMeetingUID,
				ArtifactVisibility: pastMeeting.ArtifactVisibility,
				Participants:       participants,
			})
		},
	}

	if err := pool.Run(ctx, messages...); err != nil {
		slog.ErrorContext(ctx, "failed to send NATS messages", logging.ErrKey, err)
		// Don't fail the operation if messaging fails
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
	err = s.messageSender.SendIndexPastMeetingRecording(ctx, models.ActionDeleted, *recording)
	if err != nil {
		slog.ErrorContext(ctx, "error sending index message for deleted recording", logging.ErrKey, err, "recording_uid", recordingUID)
		// Don't fail the operation if indexing fails
	}

	slog.InfoContext(ctx, "deleted past meeting recording", "recording_uid", recordingUID)
	return nil
}
