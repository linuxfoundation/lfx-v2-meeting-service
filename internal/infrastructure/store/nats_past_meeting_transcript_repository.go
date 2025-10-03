// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package store

import (
	"context"
	"fmt"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
)

// NatsPastMeetingTranscriptRepository implements PastMeetingTranscriptRepository using NATS KV store
type NatsPastMeetingTranscriptRepository struct {
	*NatsBaseRepository[models.PastMeetingTranscript]
}

// NewNatsPastMeetingTranscriptRepository creates a new transcript repository
func NewNatsPastMeetingTranscriptRepository(kvStore INatsKeyValue) *NatsPastMeetingTranscriptRepository {
	baseRepo := NewNatsBaseRepository[models.PastMeetingTranscript](kvStore, "past meeting transcript")

	return &NatsPastMeetingTranscriptRepository{
		NatsBaseRepository: baseRepo,
	}
}

// Create creates a new past meeting transcript
func (r *NatsPastMeetingTranscriptRepository) Create(ctx context.Context, transcript *models.PastMeetingTranscript) error {
	return r.NatsBaseRepository.Create(ctx, transcript.UID, transcript)
}

// Exists checks if a past meeting transcript exists
func (r *NatsPastMeetingTranscriptRepository) Exists(ctx context.Context, transcriptUID string) (bool, error) {
	return r.NatsBaseRepository.Exists(ctx, transcriptUID)
}

// Delete removes a past meeting transcript with optimistic concurrency control
func (r *NatsPastMeetingTranscriptRepository) Delete(ctx context.Context, transcriptUID string, revision uint64) error {
	return r.NatsBaseRepository.Delete(ctx, transcriptUID, revision)
}

// Get retrieves a past meeting transcript by UID
func (r *NatsPastMeetingTranscriptRepository) Get(ctx context.Context, transcriptUID string) (*models.PastMeetingTranscript, error) {
	return r.NatsBaseRepository.Get(ctx, transcriptUID)
}

// GetWithRevision retrieves a past meeting transcript with its revision
func (r *NatsPastMeetingTranscriptRepository) GetWithRevision(ctx context.Context, transcriptUID string) (*models.PastMeetingTranscript, uint64, error) {
	return r.NatsBaseRepository.GetWithRevision(ctx, transcriptUID)
}

// Update updates an existing past meeting transcript with optimistic concurrency control
func (r *NatsPastMeetingTranscriptRepository) Update(ctx context.Context, transcript *models.PastMeetingTranscript, revision uint64) error {
	return r.NatsBaseRepository.Update(ctx, transcript.UID, transcript, revision)
}

// GetByPastMeetingUID retrieves the transcript for a specific past meeting
func (r *NatsPastMeetingTranscriptRepository) GetByPastMeetingUID(ctx context.Context, pastMeetingUID string) (*models.PastMeetingTranscript, error) {
	// List all transcripts and find the one matching the past meeting UID
	transcripts, err := r.ListAll(ctx)
	if err != nil {
		return nil, err
	}

	for _, transcript := range transcripts {
		if transcript.PastMeetingUID == pastMeetingUID {
			return transcript, nil
		}
	}

	return nil, domain.NewNotFoundError(
		fmt.Sprintf("transcript for past meeting UID '%s' not found", pastMeetingUID), nil)
}

// ListByPastMeeting retrieves all transcripts for a specific past meeting
func (r *NatsPastMeetingTranscriptRepository) ListByPastMeeting(ctx context.Context, pastMeetingUID string) ([]*models.PastMeetingTranscript, error) {
	// List all transcripts and filter by past meeting UID
	allTranscripts, err := r.ListAll(ctx)
	if err != nil {
		return nil, err
	}

	var transcripts []*models.PastMeetingTranscript
	for _, transcript := range allTranscripts {
		if transcript.PastMeetingUID == pastMeetingUID {
			transcripts = append(transcripts, transcript)
		}
	}

	return transcripts, nil
}

// ListAll retrieves all past meeting transcripts
func (r *NatsPastMeetingTranscriptRepository) ListAll(ctx context.Context) ([]*models.PastMeetingTranscript, error) {
	// List all entities using the base repository functionality
	return r.NatsBaseRepository.ListEntities(ctx, "")
}

