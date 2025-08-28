// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package store

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
	"github.com/nats-io/nats.go/jetstream"
)

// NATS Key-Value store bucket name for past meetings.
const (
	// KVStoreNamePastMeetings is the name of the KV store for past meetings.
	KVStoreNamePastMeetings = "past-meetings"
)

// NatsPastMeetingRepository is the NATS KV store repository for past meetings.
type NatsPastMeetingRepository struct {
	PastMeetings INatsKeyValue
}

// NewNatsPastMeetingRepository creates a new NATS KV store repository for past meetings.
func NewNatsPastMeetingRepository(pastMeetings INatsKeyValue) *NatsPastMeetingRepository {
	return &NatsPastMeetingRepository{
		PastMeetings: pastMeetings,
	}
}

func (s *NatsPastMeetingRepository) get(ctx context.Context, pastMeetingUID string) (jetstream.KeyValueEntry, error) {
	return s.PastMeetings.Get(ctx, pastMeetingUID)
}

func (s *NatsPastMeetingRepository) unmarshal(ctx context.Context, entry jetstream.KeyValueEntry) (*models.PastMeeting, error) {
	var pastMeeting models.PastMeeting
	err := json.Unmarshal(entry.Value(), &pastMeeting)
	if err != nil {
		slog.ErrorContext(ctx, "error unmarshaling past meeting", logging.ErrKey, err)
		return nil, err
	}

	return &pastMeeting, nil
}

func (s *NatsPastMeetingRepository) Get(ctx context.Context, pastMeetingUID string) (*models.PastMeeting, error) {
	pastMeeting, _, err := s.GetWithRevision(ctx, pastMeetingUID)
	if err != nil {
		return nil, err
	}
	return pastMeeting, nil
}

func (s *NatsPastMeetingRepository) GetWithRevision(ctx context.Context, pastMeetingUID string) (*models.PastMeeting, uint64, error) {
	entry, err := s.get(ctx, pastMeetingUID)
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			slog.WarnContext(ctx, "past meeting not found", logging.ErrKey, domain.ErrPastMeetingNotFound)
			return nil, 0, domain.ErrPastMeetingNotFound
		}
		slog.ErrorContext(ctx, "error getting past meeting from NATS KV", logging.ErrKey, err)
		return nil, 0, domain.ErrInternal
	}

	pastMeeting, err := s.unmarshal(ctx, entry)
	if err != nil {
		return nil, 0, domain.ErrUnmarshal
	}

	return pastMeeting, entry.Revision(), nil
}

func (s *NatsPastMeetingRepository) Exists(ctx context.Context, pastMeetingUID string) (bool, error) {
	_, err := s.get(ctx, pastMeetingUID)
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			return false, nil
		}
		return false, domain.ErrInternal
	}
	return true, nil
}

func (s *NatsPastMeetingRepository) Create(ctx context.Context, pastMeeting *models.PastMeeting) error {
	if s.PastMeetings == nil {
		return domain.ErrServiceUnavailable
	}

	// Generate a new UID if not provided
	if pastMeeting.UID == "" {
		pastMeeting.UID = uuid.New().String()
	}

	// Set timestamps
	now := time.Now()
	pastMeeting.CreatedAt = &now
	pastMeeting.UpdatedAt = &now

	// Calculate scheduled end time if not set
	if pastMeeting.ScheduledEndTime.IsZero() && pastMeeting.Duration > 0 {
		pastMeeting.ScheduledEndTime = pastMeeting.ScheduledStartTime.Add(time.Duration(pastMeeting.Duration) * time.Minute)
	}

	// Marshal the past meeting
	pastMeetingBytes, err := json.Marshal(pastMeeting)
	if err != nil {
		slog.ErrorContext(ctx, "error marshaling past meeting", logging.ErrKey, err)
		return domain.ErrMarshal
	}

	// Store in NATS KV
	_, err = s.PastMeetings.Put(ctx, pastMeeting.UID, pastMeetingBytes)
	if err != nil {
		slog.ErrorContext(ctx, "error storing past meeting in NATS KV", logging.ErrKey, err)
		return domain.ErrInternal
	}

	return nil
}

func (s *NatsPastMeetingRepository) Update(ctx context.Context, pastMeeting *models.PastMeeting, revision uint64) error {
	if s.PastMeetings == nil {
		return domain.ErrServiceUnavailable
	}

	// Update timestamp
	now := time.Now()
	pastMeeting.UpdatedAt = &now

	// Calculate scheduled end time if not set
	if pastMeeting.ScheduledEndTime.IsZero() && pastMeeting.Duration > 0 {
		pastMeeting.ScheduledEndTime = pastMeeting.ScheduledStartTime.Add(time.Duration(pastMeeting.Duration) * time.Minute)
	}

	// Marshal the past meeting
	pastMeetingBytes, err := json.Marshal(pastMeeting)
	if err != nil {
		slog.ErrorContext(ctx, "error marshaling past meeting", logging.ErrKey, err)
		return domain.ErrMarshal
	}

	// Update in NATS KV with revision check
	_, err = s.PastMeetings.Update(ctx, pastMeeting.UID, pastMeetingBytes, revision)
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			return domain.ErrPastMeetingNotFound
		}
		slog.ErrorContext(ctx, "error updating past meeting in NATS KV", logging.ErrKey, err)
		return domain.ErrInternal
	}

	return nil
}

func (s *NatsPastMeetingRepository) Delete(ctx context.Context, pastMeetingUID string, revision uint64) error {
	if s.PastMeetings == nil {
		return domain.ErrServiceUnavailable
	}

	err := s.PastMeetings.Delete(ctx, pastMeetingUID, jetstream.LastRevision(revision))
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			return domain.ErrPastMeetingNotFound
		}
		if strings.Contains(err.Error(), "wrong last sequence") {
			slog.WarnContext(ctx, "revision mismatch", logging.ErrKey, err)
			return domain.ErrRevisionMismatch
		}
		slog.ErrorContext(ctx, "error deleting past meeting from NATS KV", logging.ErrKey, err)
		return domain.ErrInternal
	}

	return nil
}

func (s *NatsPastMeetingRepository) ListByMeeting(ctx context.Context, meetingUID string) ([]*models.PastMeeting, error) {
	if s.PastMeetings == nil {
		return nil, domain.ErrServiceUnavailable
	}

	keysLister, err := s.PastMeetings.ListKeys(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "error listing past meeting keys from NATS KV store", logging.ErrKey, err)
		return nil, domain.ErrInternal
	}

	var pastMeetings []*models.PastMeeting
	for key := range keysLister.Keys() {
		entry, err := s.get(ctx, key)
		if err != nil {
			if !errors.Is(err, jetstream.ErrKeyNotFound) {
				slog.ErrorContext(ctx, "error getting past meeting from NATS KV store", logging.ErrKey, err, "key", key)
			}
			continue
		}

		pastMeeting, err := s.unmarshal(ctx, entry)
		if err != nil {
			slog.ErrorContext(ctx, "error unmarshaling past meeting", logging.ErrKey, err, "key", key)
			continue
		}

		if pastMeeting.MeetingUID == meetingUID {
			pastMeetings = append(pastMeetings, pastMeeting)
		}
	}

	return pastMeetings, nil
}

func (s *NatsPastMeetingRepository) ListAll(ctx context.Context) ([]*models.PastMeeting, error) {
	if s.PastMeetings == nil {
		return nil, domain.ErrServiceUnavailable
	}

	keysLister, err := s.PastMeetings.ListKeys(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "error listing past meeting keys from NATS KV store", logging.ErrKey, err)
		return nil, domain.ErrInternal
	}

	var pastMeetings []*models.PastMeeting
	for key := range keysLister.Keys() {
		entry, err := s.get(ctx, key)
		if err != nil {
			if !errors.Is(err, jetstream.ErrKeyNotFound) {
				slog.ErrorContext(ctx, "error getting past meeting from NATS KV store", logging.ErrKey, err, "key", key)
			}
			continue
		}

		pastMeeting, err := s.unmarshal(ctx, entry)
		if err != nil {
			slog.ErrorContext(ctx, "error unmarshaling past meeting", logging.ErrKey, err, "key", key)
			continue
		}

		pastMeetings = append(pastMeetings, pastMeeting)
	}

	return pastMeetings, nil
}

func (s *NatsPastMeetingRepository) GetByMeetingAndOccurrence(ctx context.Context, meetingUID, occurrenceID string) (*models.PastMeeting, error) {
	if s.PastMeetings == nil {
		return nil, domain.ErrServiceUnavailable
	}

	keysLister, err := s.PastMeetings.ListKeys(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "error listing past meeting keys from NATS KV store", logging.ErrKey, err)
		return nil, domain.ErrInternal
	}

	for key := range keysLister.Keys() {
		entry, err := s.get(ctx, key)
		if err != nil {
			if !errors.Is(err, jetstream.ErrKeyNotFound) {
				slog.ErrorContext(ctx, "error getting past meeting from NATS KV store", logging.ErrKey, err, "key", key)
			}
			continue
		}

		pastMeeting, err := s.unmarshal(ctx, entry)
		if err != nil {
			slog.ErrorContext(ctx, "error unmarshaling past meeting", logging.ErrKey, err, "key", key)
			continue
		}

		if pastMeeting.MeetingUID == meetingUID && pastMeeting.OccurrenceID == occurrenceID {
			return pastMeeting, nil
		}
	}

	return nil, domain.ErrPastMeetingNotFound
}

func (s *NatsPastMeetingRepository) GetByPlatformMeetingID(ctx context.Context, platform, platformMeetingID string) (*models.PastMeeting, error) {
	if s.PastMeetings == nil {
		return nil, domain.ErrServiceUnavailable
	}

	keysLister, err := s.PastMeetings.ListKeys(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "error listing past meeting keys from NATS KV store", logging.ErrKey, err)
		return nil, domain.ErrInternal
	}

	platform = strings.ToLower(platform)

	for key := range keysLister.Keys() {
		entry, err := s.get(ctx, key)
		if err != nil {
			if !errors.Is(err, jetstream.ErrKeyNotFound) {
				slog.ErrorContext(ctx, "error getting past meeting from NATS KV store", logging.ErrKey, err, "key", key)
			}
			continue
		}

		pastMeeting, err := s.unmarshal(ctx, entry)
		if err != nil {
			slog.ErrorContext(ctx, "error unmarshaling past meeting", logging.ErrKey, err, "key", key)
			continue
		}

		if strings.ToLower(pastMeeting.Platform) == platform && pastMeeting.PlatformMeetingID == platformMeetingID {
			return pastMeeting, nil
		}
	}

	return nil, domain.ErrPastMeetingNotFound
}

// Ensure NatsPastMeetingRepository implements domain.PastMeetingRepository
var _ domain.PastMeetingRepository = (*NatsPastMeetingRepository)(nil)
