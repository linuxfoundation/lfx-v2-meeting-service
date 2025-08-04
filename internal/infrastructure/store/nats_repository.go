// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package store

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
	"github.com/nats-io/nats.go/jetstream"
)

// NATS Key-Value store bucket names.
const (
	// KVStoreNameMeetings is the name of the KV store for meetings.
	KVStoreNameMeetings = "meetings"

	// KVStoreNameMeetingRegistrants is the name of the KV store for meeting registrants.
	KVStoreNameMeetingRegistrants = "meeting-registrants"
)

// INatsKeyValue is a NATS KV interface needed for the [MeetingsService].
type INatsKeyValue interface {
	ListKeys(context.Context, ...jetstream.WatchOpt) (jetstream.KeyLister, error)
	Get(ctx context.Context, key string) (jetstream.KeyValueEntry, error)
	Put(context.Context, string, []byte) (uint64, error)
	Update(context.Context, string, []byte, uint64) (uint64, error)
	Delete(context.Context, string, ...jetstream.KVDeleteOpt) error
}

// NatsRepository is the NATS KV store repository for the meetings.
type NatsRepository struct {
	Meetings           INatsKeyValue
	MeetingRegistrants INatsKeyValue
}

// NewNatsRepository creates a new NATS KV store repository for the meetings.
func NewNatsRepository(meetings INatsKeyValue, meetingRegistrants INatsKeyValue) *NatsRepository {
	return &NatsRepository{
		Meetings:           meetings,
		MeetingRegistrants: meetingRegistrants,
	}
}

func (s *NatsRepository) getMeetingBase(ctx context.Context, meetingUID string) (jetstream.KeyValueEntry, error) {
	entry, err := s.Meetings.Get(ctx, meetingUID)
	if err != nil {
		return nil, err
	}

	return entry, nil
}

func (s *NatsRepository) getMeetingBaseUnmarshal(ctx context.Context, entry jetstream.KeyValueEntry) (*models.Meeting, error) {
	meetingDB := models.Meeting{}
	err := json.Unmarshal(entry.Value(), &meetingDB)
	if err != nil {
		slog.ErrorContext(ctx, "error unmarshalling meeting from NATS KV store", logging.ErrKey, err)
		return nil, err
	}

	return &meetingDB, nil
}

// GetMeeting gets the meeting from the NATS KV store.
func (s *NatsRepository) GetMeeting(ctx context.Context, meetingUID string) (*models.Meeting, error) {
	entry, err := s.getMeetingBase(ctx, meetingUID)
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			return nil, domain.ErrMeetingNotFound
		}
		return nil, domain.ErrInternal
	}

	meetingDB, err := s.getMeetingBaseUnmarshal(ctx, entry)
	if err != nil {
		return nil, domain.ErrUnmarshal
	}

	return meetingDB, nil
}

// GetMeetingWithRevision gets the meeting from the NATS KV store along with its revision.
func (s *NatsRepository) GetMeetingWithRevision(ctx context.Context, meetingUID string) (*models.Meeting, uint64, error) {
	entry, err := s.getMeetingBase(ctx, meetingUID)
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			return nil, 0, domain.ErrMeetingNotFound
		}
		return nil, 0, domain.ErrInternal
	}

	meetingDB, err := s.getMeetingBaseUnmarshal(ctx, entry)
	if err != nil {
		return nil, 0, domain.ErrUnmarshal
	}

	return meetingDB, entry.Revision(), nil
}

// MeetingExists checks if a meeting exists in the NATS KV store.
func (s *NatsRepository) MeetingExists(ctx context.Context, meetingUID string) (bool, error) {
	_, err := s.getMeetingBase(ctx, meetingUID)
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			return false, nil
		}
		return false, domain.ErrInternal
	}

	return true, nil
}

// ListAllMeetingsBase lists all meeting base data from the NATS KV stores.
func (s *NatsRepository) ListAllMeetingsBase(ctx context.Context) ([]*models.Meeting, error) {
	keysLister, err := s.Meetings.ListKeys(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "error listing meeting keys from NATS KV store", logging.ErrKey, err)
		return nil, domain.ErrInternal
	}

	meetingsBase := []*models.Meeting{}
	for key := range keysLister.Keys() {
		entry, err := s.getMeetingBase(ctx, key)
		if err != nil {
			slog.ErrorContext(ctx, "error getting meeting from NATS KV store", logging.ErrKey, err, "meeting_uid", key)
			return nil, domain.ErrInternal
		}

		meetingDB, err := s.getMeetingBaseUnmarshal(ctx, entry)
		if err != nil {
			slog.ErrorContext(ctx, "error unmarshalling meeting from NATS KV store", logging.ErrKey, err, "meeting_uid", key)
			return nil, domain.ErrUnmarshal
		}

		meetingsBase = append(meetingsBase, meetingDB)
	}

	return meetingsBase, nil
}

// ListAllMeetings lists all meetings from the NATS KV stores.
func (s *NatsRepository) ListAllMeetings(ctx context.Context) ([]*models.Meeting, error) {
	meetingsBase, err := s.ListAllMeetingsBase(ctx)
	if err != nil {
		return nil, err
	}

	return meetingsBase, nil
}

func (s *NatsRepository) putMeetingBase(ctx context.Context, meetingBase *models.Meeting) (uint64, error) {
	meetingBaseBytes, err := json.Marshal(meetingBase)
	if err != nil {
		slog.ErrorContext(ctx, "error marshalling meeting into JSON", logging.ErrKey, err)
		return 0, err
	}

	revision, err := s.Meetings.Put(ctx, meetingBase.UID, meetingBaseBytes)
	if err != nil {
		slog.ErrorContext(ctx, "error putting meeting into NATS KV store", logging.ErrKey, err)
		return 0, err
	}

	return revision, nil
}

// CreateMeeting creates a new meeting in the NATS KV stores.
func (s *NatsRepository) CreateMeeting(ctx context.Context, meetingBase *models.Meeting) error {
	// Store the meeting base data
	_, err := s.putMeetingBase(ctx, meetingBase)
	if err != nil {
		return domain.ErrInternal
	}

	return nil
}

func (s *NatsRepository) updateMeetingBase(ctx context.Context, meetingBase *models.Meeting, revision uint64) error {
	meetingBaseBytes, err := json.Marshal(meetingBase)
	if err != nil {
		slog.ErrorContext(ctx, "error marshalling meeting into JSON", logging.ErrKey, err)
		return err
	}

	_, err = s.Meetings.Update(ctx, meetingBase.UID, meetingBaseBytes, revision)
	if err != nil {
		slog.ErrorContext(ctx, "error updating meeting in NATS KV store", logging.ErrKey, err)
		return err
	}

	return nil
}

// UpdateMeeting updates a meeting's base information in the NATS KV store.
func (s *NatsRepository) UpdateMeeting(ctx context.Context, meetingBase *models.Meeting, revision uint64) error {
	// Update the meeting base data
	err := s.updateMeetingBase(ctx, meetingBase, revision)
	if err != nil {
		if strings.Contains(err.Error(), "wrong last sequence") {
			slog.WarnContext(ctx, "revision mismatch", logging.ErrKey, err)
			return domain.ErrRevisionMismatch
		}
		return domain.ErrInternal
	}

	return nil
}

func (s *NatsRepository) deleteMeetingBase(ctx context.Context, meetingUID string, revision uint64) error {
	err := s.Meetings.Delete(ctx, meetingUID, jetstream.LastRevision(revision))
	if err != nil {
		slog.ErrorContext(ctx, "error deleting meeting from NATS KV store", logging.ErrKey, err)
		return err
	}

	return nil
}

// DeleteMeeting deletes a meeting from the NATS KV stores.
func (s *NatsRepository) DeleteMeeting(ctx context.Context, meetingUID string, revision uint64) error {
	// Delete the meeting with revision check
	err := s.deleteMeetingBase(ctx, meetingUID, revision)
	if err != nil {
		if strings.Contains(err.Error(), "wrong last sequence") {
			slog.WarnContext(ctx, "revision mismatch", logging.ErrKey, err)
			return domain.ErrRevisionMismatch
		}
		return domain.ErrInternal
	}

	return nil
}
