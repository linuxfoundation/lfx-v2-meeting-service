// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package store

import (
	"context"
	"log/slog"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
)

// NatsPastMeetingRecordingRepository is the NATS KV store repository for past meeting recordings.
type NatsPastMeetingRecordingRepository struct {
	*NatsBaseRepository[models.PastMeetingRecording]
}

// NewNatsPastMeetingRecordingRepository creates a new NATS KV store repository for past meeting recordings.
func NewNatsPastMeetingRecordingRepository(kvStore INatsKeyValue) *NatsPastMeetingRecordingRepository {
	baseRepo := NewNatsBaseRepository[models.PastMeetingRecording](kvStore, "past meeting recording")

	return &NatsPastMeetingRecordingRepository{
		NatsBaseRepository: baseRepo,
	}
}

// Create creates a new past meeting recording
func (r *NatsPastMeetingRecordingRepository) Create(ctx context.Context, recording *models.PastMeetingRecording) error {
	if recording.UID == "" {
		return domain.NewValidationError("recording UID is required")
	}

	return r.NatsBaseRepository.Create(ctx, recording.UID, recording)
}

// Exists checks if a past meeting recording exists
func (r *NatsPastMeetingRecordingRepository) Exists(ctx context.Context, recordingUID string) (bool, error) {
	return r.NatsBaseRepository.Exists(ctx, recordingUID)
}

// Delete removes a past meeting recording
func (r *NatsPastMeetingRecordingRepository) Delete(ctx context.Context, recordingUID string, revision uint64) error {
	return r.NatsBaseRepository.Delete(ctx, recordingUID, revision)
}

// Get retrieves a past meeting recording by UID
func (r *NatsPastMeetingRecordingRepository) Get(ctx context.Context, recordingUID string) (*models.PastMeetingRecording, error) {
	return r.NatsBaseRepository.Get(ctx, recordingUID)
}

// GetWithRevision retrieves a past meeting recording with revision by UID
func (r *NatsPastMeetingRecordingRepository) GetWithRevision(ctx context.Context, recordingUID string) (*models.PastMeetingRecording, uint64, error) {
	return r.NatsBaseRepository.GetWithRevision(ctx, recordingUID)
}

// Update updates an existing past meeting recording
func (r *NatsPastMeetingRecordingRepository) Update(ctx context.Context, recording *models.PastMeetingRecording, revision uint64) error {
	return r.NatsBaseRepository.Update(ctx, recording.UID, recording, revision)
}

// GetByPastMeetingUID retrieves a past meeting recording by past meeting UID
func (r *NatsPastMeetingRecordingRepository) GetByPastMeetingUID(ctx context.Context, pastMeetingUID string) (*models.PastMeetingRecording, error) {
	recordings, err := r.ListByPastMeeting(ctx, pastMeetingUID)
	if err != nil {
		return nil, err
	}

	if len(recordings) == 0 {
		slog.DebugContext(ctx, "no recordings found for past meeting", "past_meeting_uid", pastMeetingUID)
		return nil, domain.NewNotFoundError("recording not found")
	}

	// Return the first recording found (there could be multiple recordings per past meeting)
	return recordings[0], nil
}

// ListByPastMeeting retrieves all past meeting recordings for a given past meeting UID
func (r *NatsPastMeetingRecordingRepository) ListByPastMeeting(ctx context.Context, pastMeetingUID string) ([]*models.PastMeetingRecording, error) {
	allRecordings, err := r.ListAll(ctx)
	if err != nil {
		return nil, err
	}

	var matchingRecordings []*models.PastMeetingRecording
	for _, recording := range allRecordings {
		if recording.PastMeetingUID == pastMeetingUID {
			matchingRecordings = append(matchingRecordings, recording)
		}
	}

	return matchingRecordings, nil
}

// ListAll lists all past meeting recordings
func (r *NatsPastMeetingRecordingRepository) ListAll(ctx context.Context) ([]*models.PastMeetingRecording, error) {
	return r.ListEntities(ctx, "")
}
