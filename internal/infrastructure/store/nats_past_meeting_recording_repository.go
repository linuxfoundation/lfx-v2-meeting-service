// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
)

const (
	// KVStoreNamePastMeetingRecordings is the name of the KV store for past meeting recordings.
	KVStoreNamePastMeetingRecordings = "past-meeting-recordings"
)

// NatsPastMeetingRecordingRepository implements the domain.PastMeetingRecordingRepository interface
// using NATS JetStream Key-Value storage.
type NatsPastMeetingRecordingRepository struct {
	kv jetstream.KeyValue
}

// NewNatsPastMeetingRecordingRepository creates a new NatsPastMeetingRecordingRepository.
func NewNatsPastMeetingRecordingRepository(kv jetstream.KeyValue) *NatsPastMeetingRecordingRepository {
	return &NatsPastMeetingRecordingRepository{
		kv: kv,
	}
}

// Create creates a new past meeting recording in the NATS KV store.
func (r *NatsPastMeetingRecordingRepository) Create(ctx context.Context, recording *models.PastMeetingRecording) error {
	if recording.UID == "" {
		return domain.NewValidationError("recording UID is required")
	}

	data, err := json.Marshal(recording)
	if err != nil {
		slog.ErrorContext(ctx, "error marshalling recording", logging.ErrKey, err, "recording_uid", recording.UID)
		return domain.NewInternalError("failed to marshal past meeting recording data", err)
	}

	_, err = r.kv.Create(ctx, recording.UID, data)
	if err != nil {
		slog.ErrorContext(ctx, "error creating recording in KV store", logging.ErrKey, err, "recording_uid", recording.UID)
		return domain.NewInternalError("failed to create past meeting recording in NATS key-value store", err)
	}

	slog.DebugContext(ctx, "created past meeting recording", "recording_uid", recording.UID, "past_meeting_uid", recording.PastMeetingUID)
	return nil
}

// Exists checks if a past meeting recording exists in the NATS KV store.
func (r *NatsPastMeetingRecordingRepository) Exists(ctx context.Context, recordingUID string) (bool, error) {
	_, err := r.kv.Get(ctx, recordingUID)
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			return false, nil
		}
		slog.ErrorContext(ctx, "error checking recording existence", logging.ErrKey, err, "recording_uid", recordingUID)
		return false, domain.NewInternalError("failed to check if past meeting recording exists in NATS key-value store", err)
	}
	return true, nil
}

// Delete removes a past meeting recording from the NATS KV store.
func (r *NatsPastMeetingRecordingRepository) Delete(ctx context.Context, recordingUID string, revision uint64) error {
	err := r.kv.Delete(ctx, recordingUID, jetstream.LastRevision(revision))
	if err != nil {
		slog.ErrorContext(ctx, "error deleting recording from KV store", logging.ErrKey, err, "recording_uid", recordingUID, "revision", revision)
		return domain.NewInternalError("failed to delete past meeting recording from NATS key-value store", err)
	}

	slog.DebugContext(ctx, "deleted past meeting recording", "recording_uid", recordingUID, "revision", revision)
	return nil
}

// Get retrieves a past meeting recording from the NATS KV store.
func (r *NatsPastMeetingRecordingRepository) Get(ctx context.Context, recordingUID string) (*models.PastMeetingRecording, error) {
	entry, err := r.kv.Get(ctx, recordingUID)
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			slog.DebugContext(ctx, "recording not found", "recording_uid", recordingUID)
			return nil, domain.NewNotFoundError(fmt.Sprintf("past meeting recording with UID '%s' not found", recordingUID), err)
		}
		slog.ErrorContext(ctx, "error getting recording from KV store", logging.ErrKey, err, "recording_uid", recordingUID)
		return nil, domain.NewInternalError("failed to retrieve past meeting recording from NATS key-value store", err)
	}

	var recording models.PastMeetingRecording
	err = json.Unmarshal(entry.Value(), &recording)
	if err != nil {
		slog.ErrorContext(ctx, "error unmarshalling recording", logging.ErrKey, err, "recording_uid", recordingUID)
		return nil, domain.NewInternalError("failed to unmarshal past meeting recording data", err)
	}

	return &recording, nil
}

// GetWithRevision retrieves a past meeting recording from the NATS KV store with its revision.
func (r *NatsPastMeetingRecordingRepository) GetWithRevision(ctx context.Context, recordingUID string) (*models.PastMeetingRecording, uint64, error) {
	entry, err := r.kv.Get(ctx, recordingUID)
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			slog.DebugContext(ctx, "recording not found", "recording_uid", recordingUID)
			return nil, 0, domain.NewNotFoundError(fmt.Sprintf("past meeting recording with UID '%s' not found", recordingUID), err)
		}
		slog.ErrorContext(ctx, "error getting recording from KV store", logging.ErrKey, err, "recording_uid", recordingUID)
		return nil, 0, domain.NewInternalError("failed to retrieve past meeting recording from NATS key-value store", err)
	}

	var recording models.PastMeetingRecording
	err = json.Unmarshal(entry.Value(), &recording)
	if err != nil {
		slog.ErrorContext(ctx, "error unmarshalling recording", logging.ErrKey, err, "recording_uid", recordingUID)
		return nil, 0, domain.NewInternalError("failed to unmarshal past meeting recording data", err)
	}

	return &recording, entry.Revision(), nil
}

// Update updates a past meeting recording in the NATS KV store.
func (r *NatsPastMeetingRecordingRepository) Update(ctx context.Context, recording *models.PastMeetingRecording, revision uint64) error {
	if recording.UID == "" {
		return domain.NewValidationError("recording UID is required")
	}

	data, err := json.Marshal(recording)
	if err != nil {
		slog.ErrorContext(ctx, "error marshalling recording", logging.ErrKey, err, "recording_uid", recording.UID)
		return domain.NewInternalError("failed to marshal past meeting recording data", err)
	}

	_, err = r.kv.Update(ctx, recording.UID, data, revision)
	if err != nil {
		slog.ErrorContext(ctx, "error updating recording in KV store", logging.ErrKey, err, "recording_uid", recording.UID, "revision", revision)
		return domain.NewInternalError("failed to update past meeting recording in NATS key-value store", err)
	}

	slog.DebugContext(ctx, "updated past meeting recording", "recording_uid", recording.UID, "past_meeting_uid", recording.PastMeetingUID, "revision", revision)
	return nil
}

// GetByPastMeetingUID retrieves a past meeting recording by past meeting UID.
func (r *NatsPastMeetingRecordingRepository) GetByPastMeetingUID(ctx context.Context, pastMeetingUID string) (*models.PastMeetingRecording, error) {
	// Since we need to search by past meeting UID, we'll need to scan all recordings
	// This could be optimized with secondary indexes in the future
	recordings, err := r.ListAll(ctx)
	if err != nil {
		return nil, domain.NewInternalError("failed to list past meeting recordings from NATS key-value store", err)
	}

	for _, recording := range recordings {
		if recording.PastMeetingUID == pastMeetingUID {
			return recording, nil
		}
	}

	return nil, domain.NewNotFoundError(fmt.Sprintf("no past meeting recording found for past meeting '%s'", pastMeetingUID), nil)
}

// ListByPastMeeting returns all recordings for a given past meeting.
func (r *NatsPastMeetingRecordingRepository) ListByPastMeeting(ctx context.Context, pastMeetingUID string) ([]*models.PastMeetingRecording, error) {
	recordings, err := r.ListAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list recordings: %w", err)
	}

	var results []*models.PastMeetingRecording
	for _, recording := range recordings {
		if recording.PastMeetingUID == pastMeetingUID {
			results = append(results, recording)
		}
	}

	return results, nil
}

// ListAll returns all past meeting recordings from the NATS KV store.
func (r *NatsPastMeetingRecordingRepository) ListAll(ctx context.Context) ([]*models.PastMeetingRecording, error) {
	keysLister, err := r.kv.ListKeys(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "error listing recording keys from NATS KV store", logging.ErrKey, err)
		return nil, domain.NewInternalError("failed to list past meeting recording keys from NATS key-value store", err)
	}

	var recordings []*models.PastMeetingRecording
	for key := range keysLister.Keys() {
		recording, err := r.Get(ctx, key)
		if err != nil {
			if domain.GetErrorType(err) != domain.ErrorTypeNotFound {
				slog.ErrorContext(ctx, "error getting recording from NATS KV store", logging.ErrKey, err, "key", key)
			}
			continue
		}
		recordings = append(recordings, recording)
	}

	slog.DebugContext(ctx, "listed past meeting recordings", "count", len(recordings))
	return recordings, nil
}
