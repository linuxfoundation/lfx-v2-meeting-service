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
)

// NatsMeetingRepository is the NATS KV store repository for meetings.
type NatsMeetingRepository struct {
	Meetings INatsKeyValue
}

// NewNatsMeetingRepository creates a new NATS KV store repository for meetings.
func NewNatsMeetingRepository(meetings INatsKeyValue) *NatsMeetingRepository {
	return &NatsMeetingRepository{
		Meetings: meetings,
	}
}

func (s *NatsMeetingRepository) getBase(ctx context.Context, meetingUID string) (jetstream.KeyValueEntry, error) {
	if s.Meetings == nil {
		return nil, domain.ErrServiceUnavailable
	}
	return s.Meetings.Get(ctx, meetingUID)
}

func (s *NatsMeetingRepository) getBaseUnmarshal(ctx context.Context, entry jetstream.KeyValueEntry) (*models.Meeting, error) {
	var meeting models.Meeting
	err := json.Unmarshal(entry.Value(), &meeting)
	if err != nil {
		slog.ErrorContext(ctx, "error unmarshaling meeting", logging.ErrKey, err)
		return nil, domain.ErrUnmarshal
	}

	return &meeting, nil
}

func (s *NatsMeetingRepository) Get(ctx context.Context, meetingUID string) (*models.Meeting, error) {
	meeting, _, err := s.GetWithRevision(ctx, meetingUID)
	if err != nil {
		return nil, err
	}
	return meeting, nil
}

func (s *NatsMeetingRepository) GetWithRevision(ctx context.Context, meetingUID string) (*models.Meeting, uint64, error) {
	entry, err := s.getBase(ctx, meetingUID)
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			slog.WarnContext(ctx, "meeting not found", logging.ErrKey, domain.ErrMeetingNotFound)
			return nil, 0, domain.ErrMeetingNotFound
		}
		slog.ErrorContext(ctx, "error getting meeting from NATS KV", logging.ErrKey, err)
		return nil, 0, err
	}

	meeting, err := s.getBaseUnmarshal(ctx, entry)
	if err != nil {
		return nil, 0, err
	}

	return meeting, entry.Revision(), nil
}

func (s *NatsMeetingRepository) Exists(ctx context.Context, meetingUID string) (bool, error) {
	_, err := s.getBase(ctx, meetingUID)
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (s *NatsMeetingRepository) ListAllBase(ctx context.Context) ([]*models.Meeting, error) {
	if s.Meetings == nil {
		return nil, domain.ErrServiceUnavailable
	}

	keysLister, err := s.Meetings.ListKeys(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "error listing meeting keys from NATS KV store", logging.ErrKey, err)
		return nil, domain.ErrInternal
	}

	meetingsBase := []*models.Meeting{}
	for key := range keysLister.Keys() {
		entry, err := s.getBase(ctx, key)
		if err != nil {
			slog.ErrorContext(ctx, "error getting meeting from NATS KV store", logging.ErrKey, err, "meeting_uid", key)
			return nil, domain.ErrInternal
		}

		meetingDB, err := s.getBaseUnmarshal(ctx, entry)
		if err != nil {
			slog.ErrorContext(ctx, "error unmarshalling meeting from NATS KV store", logging.ErrKey, err, "meeting_uid", key)
			return nil, domain.ErrUnmarshal
		}

		meetingsBase = append(meetingsBase, meetingDB)
	}

	return meetingsBase, nil
}

func (s *NatsMeetingRepository) ListAll(ctx context.Context) ([]*models.Meeting, error) {
	meetings, err := s.ListAllBase(ctx)
	if err != nil {
		return nil, err
	}

	return meetings, nil
}

func (s *NatsMeetingRepository) putBase(ctx context.Context, meetingBase *models.Meeting) (uint64, error) {
	if s.Meetings == nil {
		return 0, domain.ErrServiceUnavailable
	}

	jsonData, err := json.Marshal(meetingBase)
	if err != nil {
		slog.ErrorContext(ctx, "error marshaling meeting", logging.ErrKey, err)
		return 0, domain.ErrInternal
	}

	revision, err := s.Meetings.Put(ctx, meetingBase.UID, jsonData)
	if err != nil {
		slog.ErrorContext(ctx, "error putting meeting into NATS KV store", logging.ErrKey, err)
		return 0, domain.ErrInternal
	}

	return revision, nil
}

func (s *NatsMeetingRepository) Create(ctx context.Context, meetingBase *models.Meeting) error {
	_, err := s.putBase(ctx, meetingBase)
	if err != nil {
		return err
	}

	return nil
}

func (s *NatsMeetingRepository) updateBase(ctx context.Context, meetingBase *models.Meeting, revision uint64) error {
	if s.Meetings == nil {
		return domain.ErrServiceUnavailable
	}

	jsonData, err := json.Marshal(meetingBase)
	if err != nil {
		slog.ErrorContext(ctx, "error marshaling meeting", logging.ErrKey, err)
		return domain.ErrInternal
	}

	_, err = s.Meetings.Update(ctx, meetingBase.UID, jsonData, revision)
	if err != nil {
		if strings.Contains(err.Error(), "wrong last sequence") {
			slog.WarnContext(ctx, "revision mismatch", logging.ErrKey, err)
			return domain.ErrRevisionMismatch
		}
		slog.ErrorContext(ctx, "error updating meeting in NATS KV store", logging.ErrKey, err)
		return domain.ErrInternal
	}

	return nil
}

func (s *NatsMeetingRepository) Update(ctx context.Context, meetingBase *models.Meeting, revision uint64) error {
	err := s.updateBase(ctx, meetingBase, revision)
	if err != nil {
		return err
	}

	return nil
}

func (s *NatsMeetingRepository) deleteBase(ctx context.Context, meetingUID string, revision uint64) error {
	if s.Meetings == nil {
		return domain.ErrServiceUnavailable
	}

	return s.Meetings.Delete(ctx, meetingUID, jetstream.LastRevision(revision))
}

func (s *NatsMeetingRepository) Delete(ctx context.Context, meetingUID string, revision uint64) error {
	err := s.deleteBase(ctx, meetingUID, revision)
	if err != nil {
		if strings.Contains(err.Error(), "wrong last sequence") {
			slog.WarnContext(ctx, "revision mismatch", logging.ErrKey, err)
			return domain.ErrRevisionMismatch
		}
		slog.ErrorContext(ctx, "error deleting meeting from NATS KV store", logging.ErrKey, err)
		return domain.ErrInternal
	}

	return nil
}
