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

func (s *NatsMeetingRepository) getMeetingBase(ctx context.Context, meetingUID string) (jetstream.KeyValueEntry, error) {
	if s.Meetings == nil {
		return nil, domain.ErrServiceUnavailable
	}
	return s.Meetings.Get(ctx, meetingUID)
}

func (s *NatsMeetingRepository) getMeetingBaseUnmarshal(ctx context.Context, entry jetstream.KeyValueEntry) (*models.Meeting, error) {
	var meeting models.Meeting
	err := json.Unmarshal(entry.Value(), &meeting)
	if err != nil {
		slog.ErrorContext(ctx, "error unmarshaling meeting", logging.ErrKey, err)
		return nil, domain.ErrUnmarshal
	}

	return &meeting, nil
}

func (s *NatsMeetingRepository) GetMeeting(ctx context.Context, meetingUID string) (*models.Meeting, error) {
	entry, err := s.getMeetingBase(ctx, meetingUID)
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			slog.WarnContext(ctx, "meeting not found", logging.ErrKey, domain.ErrMeetingNotFound)
			return nil, domain.ErrMeetingNotFound
		}
		slog.ErrorContext(ctx, "error getting meeting from NATS KV", logging.ErrKey, err)
		return nil, err
	}

	meeting, err := s.getMeetingBaseUnmarshal(ctx, entry)
	if err != nil {
		return nil, err
	}

	return meeting, nil
}

func (s *NatsMeetingRepository) GetMeetingWithRevision(ctx context.Context, meetingUID string) (*models.Meeting, uint64, error) {
	entry, err := s.getMeetingBase(ctx, meetingUID)
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			slog.WarnContext(ctx, "meeting not found", logging.ErrKey, domain.ErrMeetingNotFound)
			return nil, 0, domain.ErrMeetingNotFound
		}
		slog.ErrorContext(ctx, "error getting meeting from NATS KV", logging.ErrKey, err)
		return nil, 0, err
	}

	meeting, err := s.getMeetingBaseUnmarshal(ctx, entry)
	if err != nil {
		return nil, 0, err
	}

	return meeting, entry.Revision(), nil
}

func (s *NatsMeetingRepository) MeetingExists(ctx context.Context, meetingUID string) (bool, error) {
	_, err := s.getMeetingBase(ctx, meetingUID)
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (s *NatsMeetingRepository) ListAllMeetingsBase(ctx context.Context) ([]*models.Meeting, error) {
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

func (s *NatsMeetingRepository) ListAllMeetings(ctx context.Context) ([]*models.Meeting, error) {
	meetings, err := s.ListAllMeetingsBase(ctx)
	if err != nil {
		return nil, err
	}

	return meetings, nil
}

func (s *NatsMeetingRepository) putMeetingBase(ctx context.Context, meetingBase *models.Meeting) (uint64, error) {
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

func (s *NatsMeetingRepository) CreateMeeting(ctx context.Context, meetingBase *models.Meeting) error {
	_, err := s.putMeetingBase(ctx, meetingBase)
	if err != nil {
		return err
	}

	return nil
}

func (s *NatsMeetingRepository) updateMeetingBase(ctx context.Context, meetingBase *models.Meeting, revision uint64) error {
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

func (s *NatsMeetingRepository) UpdateMeeting(ctx context.Context, meetingBase *models.Meeting, revision uint64) error {
	err := s.updateMeetingBase(ctx, meetingBase, revision)
	if err != nil {
		return err
	}

	return nil
}

func (s *NatsMeetingRepository) deleteMeetingBase(ctx context.Context, meetingUID string, revision uint64) error {
	if s.Meetings == nil {
		return domain.ErrServiceUnavailable
	}

	return s.Meetings.Delete(ctx, meetingUID, jetstream.LastRevision(revision))
}

func (s *NatsMeetingRepository) DeleteMeeting(ctx context.Context, meetingUID string, revision uint64) error {
	err := s.deleteMeetingBase(ctx, meetingUID, revision)
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
