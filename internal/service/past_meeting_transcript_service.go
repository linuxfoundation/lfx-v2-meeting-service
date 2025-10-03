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

// PastMeetingTranscriptService provides business logic for past meeting transcripts
type PastMeetingTranscriptService struct {
	pastMeetingTranscriptRepository  domain.PastMeetingTranscriptRepository
	pastMeetingRepository            domain.PastMeetingRepository
	pastMeetingParticipantRepository domain.PastMeetingParticipantRepository
	messageBuilder                   domain.MessageBuilder
	config                           ServiceConfig
}

// NewPastMeetingTranscriptService creates a new PastMeetingTranscriptService
func NewPastMeetingTranscriptService(
	pastMeetingTranscriptRepository domain.PastMeetingTranscriptRepository,
	pastMeetingRepository domain.PastMeetingRepository,
	pastMeetingParticipantRepository domain.PastMeetingParticipantRepository,
	messageBuilder domain.MessageBuilder,
	serviceConfig ServiceConfig,
) *PastMeetingTranscriptService {
	return &PastMeetingTranscriptService{
		pastMeetingTranscriptRepository:  pastMeetingTranscriptRepository,
		pastMeetingRepository:            pastMeetingRepository,
		pastMeetingParticipantRepository: pastMeetingParticipantRepository,
		messageBuilder:                   messageBuilder,
		config:                           serviceConfig,
	}
}

// ServiceReady checks if the service is ready to serve requests
func (s *PastMeetingTranscriptService) ServiceReady() bool {
	return s.pastMeetingTranscriptRepository != nil &&
		s.pastMeetingRepository != nil &&
		s.pastMeetingParticipantRepository != nil &&
		s.messageBuilder != nil
}

// CreateTranscript creates a new past meeting transcript
func (s *PastMeetingTranscriptService) CreateTranscript(ctx context.Context, transcript *models.PastMeetingTranscript) (*models.PastMeetingTranscript, error) {
	if transcript.UID == "" {
		transcript.UID = uuid.New().String()
	}

	// Set timestamps
	now := time.Now().UTC()
	transcript.CreatedAt = now
	transcript.UpdatedAt = now

	// Calculate total size and count
	transcript.TranscriptCount = len(transcript.TranscriptFiles)
	transcript.TotalSize = 0
	for _, file := range transcript.TranscriptFiles {
		transcript.TotalSize += file.FileSize
	}

	// err := s.pastMeetingTranscriptRepository.Create(ctx, transcript)
	// if err != nil {
	// 	slog.ErrorContext(ctx, "failed to create transcript", logging.ErrKey, err,
	// 		"transcript_uid", transcript.UID,
	// 		"past_meeting_uid", transcript.PastMeetingUID,
	// 	)
	// 	return nil, err
	// }

	// Use WorkerPool for concurrent NATS message sending
	pool := concurrent.NewWorkerPool(2) // 2 messages to send
	messages := []func() error{
		func() error {
			return s.messageBuilder.SendIndexPastMeetingTranscript(ctx, models.ActionCreated, *transcript)
		},
		func() error {
			// Get past meeting to retrieve artifact visibility
			pastMeeting, err := s.pastMeetingRepository.Get(ctx, transcript.PastMeetingUID)
			if err != nil {
				return err
			}

			// Get participants for the past meeting
			participantPointers, err := s.pastMeetingParticipantRepository.ListByPastMeeting(ctx, transcript.PastMeetingUID)
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

			return s.messageBuilder.SendUpdateAccessPastMeetingTranscript(ctx, models.PastMeetingTranscriptAccessMessage{
				UID:                transcript.UID,
				PastMeetingUID:     transcript.PastMeetingUID,
				ArtifactVisibility: pastMeeting.ArtifactVisibility,
				Participants:       participants,
			})
		},
	}

	if err := pool.Run(ctx, messages...); err != nil {
		slog.ErrorContext(ctx, "failed to send NATS messages", logging.ErrKey, err)
		// Don't fail the operation if messaging fails
	}

	slog.InfoContext(ctx, "successfully created transcript",
		"transcript_uid", transcript.UID,
		"past_meeting_uid", transcript.PastMeetingUID,
		"platform", transcript.Platform,
		"transcript_count", transcript.TranscriptCount,
	)

	return transcript, nil
}

// GetTranscriptByPastMeetingUID retrieves the transcript for a specific past meeting
func (s *PastMeetingTranscriptService) GetTranscriptByPastMeetingUID(ctx context.Context, pastMeetingUID string) (*models.PastMeetingTranscript, error) {
	transcript, err := s.pastMeetingTranscriptRepository.GetByPastMeetingUID(ctx, pastMeetingUID)
	if err != nil {
		if domain.GetErrorType(err) == domain.ErrorTypeNotFound {
			slog.DebugContext(ctx, "no transcript found for past meeting",
				"past_meeting_uid", pastMeetingUID,
			)
		} else {
			slog.ErrorContext(ctx, "failed to get transcript by past meeting UID", logging.ErrKey, err,
				"past_meeting_uid", pastMeetingUID,
			)
		}
		return nil, err
	}

	return transcript, nil
}

// UpdateTranscript updates an existing transcript or creates one if it doesn't exist
func (s *PastMeetingTranscriptService) UpdateTranscript(ctx context.Context, transcriptUID string, updatedTranscript *models.PastMeetingTranscript) (*models.PastMeetingTranscript, error) {
	// Get existing transcript with revision
	existingTranscript, revision, err := s.pastMeetingTranscriptRepository.GetWithRevision(ctx, transcriptUID)
	if err != nil {
		if domain.GetErrorType(err) == domain.ErrorTypeNotFound {
			// Create new transcript if it doesn't exist
			slog.InfoContext(ctx, "transcript not found, creating new one",
				"transcript_uid", transcriptUID,
			)
			updatedTranscript.UID = transcriptUID
			return s.CreateTranscript(ctx, updatedTranscript)
		}
		slog.ErrorContext(ctx, "failed to get existing transcript", logging.ErrKey, err,
			"transcript_uid", transcriptUID,
		)
		return nil, err
	}

	// Merge transcript files and sessions from new data
	existingTranscript.AddTranscriptFiles(updatedTranscript.TranscriptFiles)
	for _, session := range updatedTranscript.Sessions {
		existingTranscript.AddTranscriptSession(session)
	}

	// Update metadata
	existingTranscript.UpdatedAt = time.Now().UTC()

	// Update in repository
	err = s.pastMeetingTranscriptRepository.Update(ctx, existingTranscript, revision)
	if err != nil {
		slog.ErrorContext(ctx, "failed to update transcript", logging.ErrKey, err,
			"transcript_uid", transcriptUID,
		)
		return nil, err
	}

	// Use WorkerPool for concurrent NATS message sending
	pool := concurrent.NewWorkerPool(2) // 2 messages to send
	messages := []func() error{
		func() error {
			return s.messageBuilder.SendIndexPastMeetingTranscript(ctx, models.ActionUpdated, *existingTranscript)
		},
		func() error {
			// Get past meeting to retrieve artifact visibility
			pastMeeting, err := s.pastMeetingRepository.Get(ctx, existingTranscript.PastMeetingUID)
			if err != nil {
				return err
			}

			// Get participants for the past meeting
			participantPointers, err := s.pastMeetingParticipantRepository.ListByPastMeeting(ctx, existingTranscript.PastMeetingUID)
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

			return s.messageBuilder.SendUpdateAccessPastMeetingTranscript(ctx, models.PastMeetingTranscriptAccessMessage{
				UID:                existingTranscript.UID,
				PastMeetingUID:     existingTranscript.PastMeetingUID,
				ArtifactVisibility: pastMeeting.ArtifactVisibility,
				Participants:       participants,
			})
		},
	}

	if err := pool.Run(ctx, messages...); err != nil {
		slog.ErrorContext(ctx, "failed to send NATS messages", logging.ErrKey, err)
		// Don't fail the operation if messaging fails
	}

	slog.InfoContext(ctx, "successfully updated transcript",
		"transcript_uid", transcriptUID,
		"past_meeting_uid", existingTranscript.PastMeetingUID,
		"transcript_count", existingTranscript.TranscriptCount,
	)

	return existingTranscript, nil
}

// ListTranscriptsByPastMeeting retrieves all transcripts for a specific past meeting
func (s *PastMeetingTranscriptService) ListTranscriptsByPastMeeting(ctx context.Context, pastMeetingUID string) ([]*models.PastMeetingTranscript, error) {
	transcripts, err := s.pastMeetingTranscriptRepository.ListByPastMeeting(ctx, pastMeetingUID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to list transcripts by past meeting", logging.ErrKey, err,
			"past_meeting_uid", pastMeetingUID,
		)
		return nil, err
	}

	return transcripts, nil
}

// ListAllTranscripts retrieves all transcripts
func (s *PastMeetingTranscriptService) ListAllTranscripts(ctx context.Context) ([]*models.PastMeetingTranscript, error) {
	transcripts, err := s.pastMeetingTranscriptRepository.ListAll(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to list all transcripts", logging.ErrKey, err)
		return nil, err
	}

	return transcripts, nil
}

// DeleteTranscript removes a transcript
func (s *PastMeetingTranscriptService) DeleteTranscript(ctx context.Context, transcriptUID string) error {
	// Get transcript with revision for deletion
	transcript, revision, err := s.pastMeetingTranscriptRepository.GetWithRevision(ctx, transcriptUID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get transcript for deletion", logging.ErrKey, err,
			"transcript_uid", transcriptUID,
		)
		return err
	}

	err = s.pastMeetingTranscriptRepository.Delete(ctx, transcriptUID, revision)
	if err != nil {
		slog.ErrorContext(ctx, "failed to delete transcript", logging.ErrKey, err,
			"transcript_uid", transcriptUID,
		)
		return err
	}

	// Publish deletion message
	err = s.messageBuilder.SendDeleteIndexPastMeetingTranscript(ctx, transcriptUID)
	if err != nil {
		slog.WarnContext(ctx, "failed to publish transcript deletion message", logging.ErrKey, err,
			"transcript_uid", transcriptUID,
		)
		// Don't fail the operation if messaging fails
	}

	slog.InfoContext(ctx, "successfully deleted transcript",
		"transcript_uid", transcriptUID,
		"past_meeting_uid", transcript.PastMeetingUID,
	)

	return nil
}
