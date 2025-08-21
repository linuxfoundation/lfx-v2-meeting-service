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
	// KVStoreNameMeetingSettings is the name of the KV store for meeting settings.
	KVStoreNameMeetingSettings = "meeting-settings"
)

// NatsMeetingRepository is the NATS KV store repository for meetings.
type NatsMeetingRepository struct {
	Meetings        INatsKeyValue
	MeetingSettings INatsKeyValue
}

// NewNatsMeetingRepository creates a new NATS KV store repository for meetings.
func NewNatsMeetingRepository(meetings INatsKeyValue, meetingSettings INatsKeyValue) *NatsMeetingRepository {
	return &NatsMeetingRepository{
		Meetings:        meetings,
		MeetingSettings: meetingSettings,
	}
}

func (s *NatsMeetingRepository) getBase(ctx context.Context, meetingUID string) (jetstream.KeyValueEntry, error) {
	return s.Meetings.Get(ctx, meetingUID)
}

func (s *NatsMeetingRepository) getBaseUnmarshal(ctx context.Context, entry jetstream.KeyValueEntry) (*models.MeetingBase, error) {
	var meeting models.MeetingBase
	err := json.Unmarshal(entry.Value(), &meeting)
	if err != nil {
		slog.ErrorContext(ctx, "error unmarshaling meeting", logging.ErrKey, err)
		return nil, err
	}

	return &meeting, nil
}

func (s *NatsMeetingRepository) GetBase(ctx context.Context, meetingUID string) (*models.MeetingBase, error) {
	meeting, _, err := s.GetBaseWithRevision(ctx, meetingUID)
	if err != nil {
		return nil, err
	}
	return meeting, nil
}

func (s *NatsMeetingRepository) GetBaseWithRevision(ctx context.Context, meetingUID string) (*models.MeetingBase, uint64, error) {
	entry, err := s.getBase(ctx, meetingUID)
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			slog.WarnContext(ctx, "meeting not found", logging.ErrKey, domain.ErrMeetingNotFound)
			return nil, 0, domain.ErrMeetingNotFound
		}
		slog.ErrorContext(ctx, "error getting meeting from NATS KV", logging.ErrKey, err)
		return nil, 0, domain.ErrInternal
	}

	meeting, err := s.getBaseUnmarshal(ctx, entry)
	if err != nil {
		return nil, 0, domain.ErrUnmarshal
	}

	return meeting, entry.Revision(), nil
}

func (s *NatsMeetingRepository) getSettings(ctx context.Context, meetingUID string) (jetstream.KeyValueEntry, error) {
	return s.MeetingSettings.Get(ctx, meetingUID)
}

func (s *NatsMeetingRepository) getSettingsUnmarshal(ctx context.Context, entry jetstream.KeyValueEntry) (*models.MeetingSettings, error) {
	var meeting models.MeetingSettings
	err := json.Unmarshal(entry.Value(), &meeting)
	if err != nil {
		slog.ErrorContext(ctx, "error unmarshaling meeting settings", logging.ErrKey, err)
		return nil, err
	}

	return &meeting, nil
}

func (s *NatsMeetingRepository) GetSettings(ctx context.Context, meetingUID string) (*models.MeetingSettings, error) {
	meeting, _, err := s.GetSettingsWithRevision(ctx, meetingUID)
	if err != nil {
		return nil, err
	}
	return meeting, nil
}

func (s *NatsMeetingRepository) GetSettingsWithRevision(ctx context.Context, meetingUID string) (*models.MeetingSettings, uint64, error) {
	entry, err := s.getSettings(ctx, meetingUID)
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			slog.WarnContext(ctx, "meeting settings not found", logging.ErrKey, domain.ErrMeetingNotFound)
			return nil, 0, domain.ErrMeetingNotFound
		}
		slog.ErrorContext(ctx, "error getting meeting settings from NATS KV", logging.ErrKey, err)
		return nil, 0, domain.ErrInternal
	}

	meeting, err := s.getSettingsUnmarshal(ctx, entry)
	if err != nil {
		return nil, 0, domain.ErrUnmarshal
	}

	return meeting, entry.Revision(), nil
}

func (s *NatsMeetingRepository) Exists(ctx context.Context, meetingUID string) (bool, error) {
	_, err := s.getBase(ctx, meetingUID)
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			return false, nil
		}
		return false, domain.ErrInternal
	}
	return true, nil
}

func (s *NatsMeetingRepository) ListAllBase(ctx context.Context) ([]*models.MeetingBase, error) {
	if s.Meetings == nil {
		return nil, domain.ErrServiceUnavailable
	}

	keysLister, err := s.Meetings.ListKeys(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "error listing meeting keys from NATS KV store", logging.ErrKey, err)
		return nil, domain.ErrInternal
	}

	meetingsBase := []*models.MeetingBase{}
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

// ListAllSettings lists all meeting settings data from the NATS KV stores.
func (s *NatsMeetingRepository) ListAllSettings(ctx context.Context) ([]*models.MeetingSettings, error) {
	if s.MeetingSettings == nil {
		return nil, domain.ErrServiceUnavailable
	}

	keysLister, err := s.MeetingSettings.ListKeys(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "error listing meeting settings keys from NATS KV store", logging.ErrKey, err)
		return nil, domain.ErrInternal
	}

	meetingSettings := []*models.MeetingSettings{}
	for key := range keysLister.Keys() {
		entry, err := s.MeetingSettings.Get(ctx, key)
		if err != nil {
			slog.ErrorContext(ctx, "error getting meeting settings from NATS KV store", logging.ErrKey, err, "meeting_uid", key)
			return nil, domain.ErrInternal
		}

		meetingSettingsDB := &models.MeetingSettings{}
		err = json.Unmarshal(entry.Value(), meetingSettingsDB)
		if err != nil {
			slog.ErrorContext(ctx, "error unmarshalling meeting settings from NATS KV store", logging.ErrKey, err, "meeting_uid", key)
			return nil, domain.ErrUnmarshal
		}

		meetingSettings = append(meetingSettings, meetingSettingsDB)
	}

	return meetingSettings, nil
}

func (s *NatsMeetingRepository) ListAll(ctx context.Context) ([]*models.MeetingBase, []*models.MeetingSettings, error) {
	meetingsBase, err := s.ListAllBase(ctx)
	if err != nil {
		return nil, nil, err
	}

	allSettings, err := s.ListAllSettings(ctx)
	if err != nil {
		return nil, nil, err
	}

	// Create a map of settings by UID for efficient lookup
	settingsMap := make(map[string]*models.MeetingSettings)
	for _, setting := range allSettings {
		settingsMap[setting.UID] = setting
	}

	// Create matched settings slice in the same order as meetings
	matchedSettings := make([]*models.MeetingSettings, len(meetingsBase))
	for i, meeting := range meetingsBase {
		if setting, exists := settingsMap[meeting.UID]; exists {
			matchedSettings[i] = setting
		} else {
			matchedSettings[i] = nil
		}
	}

	return meetingsBase, matchedSettings, nil
}

func (s *NatsMeetingRepository) putBase(ctx context.Context, meetingBase *models.MeetingBase) (uint64, error) {
	jsonData, err := json.Marshal(meetingBase)
	if err != nil {
		slog.ErrorContext(ctx, "error marshaling meeting", logging.ErrKey, err)
		return 0, err
	}

	revision, err := s.Meetings.Put(ctx, meetingBase.UID, jsonData)
	if err != nil {
		slog.ErrorContext(ctx, "error putting meeting into NATS KV store", logging.ErrKey, err)
		return 0, err
	}

	return revision, nil
}

func (s *NatsMeetingRepository) putSettings(ctx context.Context, meetingSettings *models.MeetingSettings) (uint64, error) {
	meetingSettingsBytes, err := json.Marshal(meetingSettings)
	if err != nil {
		slog.ErrorContext(ctx, "error marshalling meeting settings into JSON", logging.ErrKey, err)
		return 0, err
	}

	revision, err := s.MeetingSettings.Put(ctx, meetingSettings.UID, meetingSettingsBytes)
	if err != nil {
		slog.ErrorContext(ctx, "error putting meeting settings into NATS KV store", logging.ErrKey, err)
		return 0, err
	}

	return revision, nil
}

func (s *NatsMeetingRepository) Create(ctx context.Context, meetingBase *models.MeetingBase, meetingSettings *models.MeetingSettings) error {
	_, err := s.putBase(ctx, meetingBase)
	if err != nil {
		return domain.ErrInternal
	}

	_, err = s.putSettings(ctx, meetingSettings)
	if err != nil {
		return domain.ErrInternal
	}

	return nil
}

func (s *NatsMeetingRepository) updateBase(ctx context.Context, meetingBase *models.MeetingBase, revision uint64) error {
	jsonData, err := json.Marshal(meetingBase)
	if err != nil {
		slog.ErrorContext(ctx, "error marshaling meeting", logging.ErrKey, err)
		return err
	}

	_, err = s.Meetings.Update(ctx, meetingBase.UID, jsonData, revision)
	if err != nil {
		slog.ErrorContext(ctx, "error updating meeting in NATS KV store", logging.ErrKey, err)
		return err
	}

	return nil
}

func (s *NatsMeetingRepository) UpdateBase(ctx context.Context, meetingBase *models.MeetingBase, revision uint64) error {
	err := s.updateBase(ctx, meetingBase, revision)
	if err != nil {
		if strings.Contains(err.Error(), "wrong last sequence") {
			slog.WarnContext(ctx, "revision mismatch", logging.ErrKey, err)
			return domain.ErrRevisionMismatch
		}
		return domain.ErrInternal
	}

	return nil
}

func (s *NatsMeetingRepository) updateSettings(ctx context.Context, meetingSettings *models.MeetingSettings, revision uint64) error {
	jsonData, err := json.Marshal(meetingSettings)
	if err != nil {
		slog.ErrorContext(ctx, "error marshaling meeting settings", logging.ErrKey, err)
		return err
	}

	_, err = s.MeetingSettings.Update(ctx, meetingSettings.UID, jsonData, revision)
	if err != nil {
		return err
	}

	return nil
}

func (s *NatsMeetingRepository) UpdateSettings(ctx context.Context, meetingSettings *models.MeetingSettings, revision uint64) error {
	err := s.updateSettings(ctx, meetingSettings, revision)
	if err != nil {
		if strings.Contains(err.Error(), "wrong last sequence") {
			slog.WarnContext(ctx, "revision mismatch", logging.ErrKey, err)
			return domain.ErrRevisionMismatch
		}
		return domain.ErrInternal
	}

	return nil
}

func (s *NatsMeetingRepository) deleteBase(ctx context.Context, meetingUID string, revision uint64) error {
	return s.Meetings.Delete(ctx, meetingUID, jetstream.LastRevision(revision))
}

func (s *NatsMeetingRepository) deleteSettings(ctx context.Context, meetingUID string) error {
	err := s.MeetingSettings.Delete(ctx, meetingUID)
	if err != nil {
		// If settings don't exist, that's okay - they may not have been created
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			return nil
		}
		slog.ErrorContext(ctx, "error deleting meeting settings from NATS KV store", logging.ErrKey, err)
		return err
	}

	return nil
}

func (s *NatsMeetingRepository) Delete(ctx context.Context, meetingUID string, revision uint64) error {
	if s.Meetings == nil {
		return domain.ErrServiceUnavailable
	}

	err := s.deleteBase(ctx, meetingUID, revision)
	if err != nil {
		if strings.Contains(err.Error(), "wrong last sequence") {
			slog.WarnContext(ctx, "revision mismatch", logging.ErrKey, err)
			return domain.ErrRevisionMismatch
		}
		slog.ErrorContext(ctx, "error deleting meeting from NATS KV store", logging.ErrKey, err)
		return domain.ErrInternal
	}

	err = s.deleteSettings(ctx, meetingUID)
	if err != nil {
		slog.ErrorContext(ctx, "error deleting meeting settings from NATS KV store", logging.ErrKey, err)
		return domain.ErrInternal
	}

	return nil
}

// GetByZoomMeetingID retrieves a meeting by its Zoom meeting ID
func (s *NatsMeetingRepository) GetByZoomMeetingID(ctx context.Context, zoomMeetingID string) (*models.MeetingBase, error) {
	if s.Meetings == nil {
		return nil, domain.ErrServiceUnavailable
	}

	// List all meetings
	meetings, _, err := s.ListAll(ctx)
	if err != nil {
		return nil, err
	}

	// Find the meeting with matching Zoom ID
	for _, meeting := range meetings {
		if meeting.Platform == "Zoom" && meeting.ZoomConfig != nil && meeting.ZoomConfig.MeetingID == zoomMeetingID {
			return meeting, nil
		}
	}

	return nil, domain.ErrMeetingNotFound
}
