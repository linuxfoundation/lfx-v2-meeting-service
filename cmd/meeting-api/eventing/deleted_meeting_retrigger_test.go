// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"errors"
	"testing"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
)

// memKV is a map-backed jetstream.KeyValue covering the Get/Put/Delete calls the
// meeting handlers make. getErrs injects a Get error for a specific key.
type memKV struct {
	jetstream.KeyValue
	data    map[string][]byte
	getErrs map[string]error
}

func newMemKV() *memKV {
	return &memKV{data: map[string][]byte{}, getErrs: map[string]error{}}
}

func (k *memKV) Get(_ context.Context, key string) (jetstream.KeyValueEntry, error) {
	if err, ok := k.getErrs[key]; ok {
		return nil, err
	}
	v, ok := k.data[key]
	if !ok {
		return nil, jetstream.ErrKeyNotFound
	}
	return mockKeyValueEntry{key: key, value: v}, nil
}

func (k *memKV) Put(_ context.Context, key string, value []byte) (uint64, error) {
	k.data[key] = value
	return 1, nil
}

func (k *memKV) Delete(_ context.Context, key string, _ ...jetstream.KVDeleteOpt) error {
	delete(k.data, key)
	return nil
}

// retriggerTestPublisher counts the live-object publishes that a deleted meeting must not
// produce.
type retriggerTestPublisher struct {
	recordingTestPublisher
	meetingEvents     int
	hostCredsEvents   int
	pastMeetingEvents int
}

func (p *retriggerTestPublisher) PublishMeetingEvent(_ context.Context, _ string, _ *models.MeetingEventData) error {
	p.meetingEvents++
	return nil
}

func (p *retriggerTestPublisher) PublishMeetingHostCredentialsEvent(_ context.Context, _ string, _ *models.MeetingHostCredentialsEventData) error {
	p.hostCredsEvents++
	return nil
}

func (p *retriggerTestPublisher) PublishPastMeetingEvent(_ context.Context, _ string, _ *models.PastMeetingEventData) error {
	p.pastMeetingEvents++
	return nil
}

func newRetriggerHandler(pub *retriggerTestPublisher, objects, mappings *memKV) *EventHandlers {
	h := buildRecordingHandler(nil, objects, mappings)
	h.publisher = pub
	return h
}

// TestMeetingMappingRetrigger_DeletedMeeting verifies that committee-mapping events never
// republish a meeting that was deleted, and never replace its tombstone with a live marker.
func TestMeetingMappingRetrigger_DeletedMeeting(t *testing.T) {
	const (
		meetingID     = "91000000001"
		meetingKey    = "itx-zoom-meetings-v2." + meetingID
		mappingKey    = "itx-zoom-meetings-mappings-v2.map-1"
		liveMarkerKey = "v1_meetings." + meetingID
		committeesKey = "v1-mappings.meeting-mappings." + meetingID
	)

	meetingRecord := func(softDeleted bool) map[string]any {
		m := map[string]any{
			"meeting_id": meetingID,
			"proj_id":    "proj-sfid",
			"topic":      "Test Meeting",
			"start_time": "2026-01-01T00:00:00Z",
			"duration":   30,
			"host_key":   "123456",
			"updated_by": map[string]any{"user_id": "user-1", "username": "alice"},
		}
		if softDeleted {
			m["_sdc_deleted_at"] = "2026-01-02T00:00:00Z"
		}
		return m
	}
	mappingRecord := func(softDeleted bool) map[string]any {
		m := map[string]any{"id": "map-1", "meeting_id": meetingID, "committee_id": "committee-sfid"}
		if softDeleted {
			m["_sdc_deleted_at"] = "2026-01-02T00:00:00Z"
		}
		return m
	}

	setup := func(t *testing.T, softDeleted bool, tombstoned bool) (*retriggerTestPublisher, *memKV, *EventHandlers) {
		objects, mappings := newMemKV(), newMemKV()
		objects.data[meetingKey] = mustMsgpackKV(t, meetingRecord(softDeleted))
		if tombstoned {
			mappings.data[liveMarkerKey] = []byte(tombstoneMarker)
			mappings.data[committeesKey] = []byte(tombstoneMarker)
		}
		pub := &retriggerTestPublisher{}
		return pub, mappings, newRetriggerHandler(pub, objects, mappings)
	}

	t.Run("control: live meeting is re-indexed on mapping put", func(t *testing.T) {
		pub, mappings, h := setup(t, false, false)

		retry := handleKVPut(context.Background(), mappingKey, mappingRecord(false), h)

		assert.False(t, retry)
		assert.Equal(t, 1, pub.meetingEvents)
		assert.Equal(t, 1, pub.hostCredsEvents)
		assert.Equal(t, "1", string(mappings.data[liveMarkerKey]))
	})

	for _, mappingSoftDeleted := range []bool{false, true} {
		name := "mapping put"
		if mappingSoftDeleted {
			name = "mapping soft delete"
		}

		t.Run(name+" after meeting soft delete publishes nothing", func(t *testing.T) {
			pub, mappings, h := setup(t, true, true)

			retry := handleKVPut(context.Background(), mappingKey, mappingRecord(mappingSoftDeleted), h)

			assert.False(t, retry)
			assert.Zero(t, pub.meetingEvents)
			assert.Zero(t, pub.hostCredsEvents)
			assert.Equal(t, tombstoneMarker, string(mappings.data[liveMarkerKey]))
			assert.Equal(t, tombstoneMarker, string(mappings.data[committeesKey]))
		})

		t.Run(name+" for soft-deleted record without tombstone publishes nothing", func(t *testing.T) {
			pub, mappings, h := setup(t, true, false)

			retry := handleKVPut(context.Background(), mappingKey, mappingRecord(mappingSoftDeleted), h)

			assert.False(t, retry)
			assert.Zero(t, pub.meetingEvents)
			assert.Zero(t, pub.hostCredsEvents)
			assert.NotContains(t, mappings.data, liveMarkerKey)
		})

		t.Run(name+" for tombstoned meeting with live record publishes nothing", func(t *testing.T) {
			pub, mappings, h := setup(t, false, true)

			retry := handleKVPut(context.Background(), mappingKey, mappingRecord(mappingSoftDeleted), h)

			assert.False(t, retry)
			assert.Zero(t, pub.meetingEvents)
			assert.Zero(t, pub.hostCredsEvents)
			assert.Equal(t, tombstoneMarker, string(mappings.data[liveMarkerKey]))
			assert.Equal(t, tombstoneMarker, string(mappings.data[committeesKey]))
		})
	}

	t.Run("tombstone read error retries without publishing", func(t *testing.T) {
		pub, mappings, h := setup(t, false, false)
		mappings.getErrs[liveMarkerKey] = errors.New("kv unavailable")

		retry := handleKVPut(context.Background(), mappingKey, mappingRecord(false), h)

		assert.True(t, retry)
		assert.Zero(t, pub.meetingEvents)
		assert.Zero(t, pub.hostCredsEvents)
	})

	t.Run("restore of a live record still re-indexes a tombstoned meeting", func(t *testing.T) {
		pub, mappings, h := setup(t, false, true)

		retry := handleKVPut(context.Background(), meetingKey, meetingRecord(false), h)

		assert.False(t, retry)
		assert.Equal(t, 1, pub.meetingEvents)
		assert.Equal(t, 1, pub.hostCredsEvents)
		assert.Equal(t, "1", string(mappings.data[liveMarkerKey]))
	})

	t.Run("handleMeetingUpdate ignores a soft-deleted record", func(t *testing.T) {
		pub, mappings, h := setup(t, false, false)

		retry := h.handleMeetingUpdate(context.Background(), meetingKey, meetingRecord(true))

		assert.False(t, retry)
		assert.Zero(t, pub.meetingEvents)
		assert.Zero(t, pub.hostCredsEvents)
		assert.NotContains(t, mappings.data, liveMarkerKey)
	})
}

// TestPastMeetingMappingRetrigger_DeletedPastMeeting mirrors the active-meeting cases for
// past meetings, whose update also republishes recording and summary artifacts.
func TestPastMeetingMappingRetrigger_DeletedPastMeeting(t *testing.T) {
	const (
		pastMeetingID  = "91000000001-1700000000"
		pastMeetingKey = "itx-zoom-past-meetings." + pastMeetingID
		mappingKey     = "itx-zoom-past-meetings-mappings.map-1"
		liveMarkerKey  = "v1_past_meetings." + pastMeetingID
		committeesKey  = "v1-mappings.past-meeting-mappings." + pastMeetingID
	)

	pastMeetingRecord := func(softDeleted bool) map[string]any {
		m := map[string]any{
			"meeting_and_occurrence_id": pastMeetingID,
			"meeting_id":                "91000000001",
			"occurrence_id":             "1700000000",
			"proj_id":                   "proj-sfid",
			"topic":                     "Test Meeting",
		}
		if softDeleted {
			m["_sdc_deleted_at"] = "2026-01-02T00:00:00Z"
		}
		return m
	}
	mappingRecord := func(softDeleted bool) map[string]any {
		m := map[string]any{"id": "map-1", "past_meeting_uuid": pastMeetingID, "committee_id": "committee-sfid"}
		if softDeleted {
			m["_sdc_deleted_at"] = "2026-01-02T00:00:00Z"
		}
		return m
	}

	setup := func(t *testing.T, softDeleted bool, tombstoned bool) (*retriggerTestPublisher, *memKV, *EventHandlers) {
		objects, mappings := newMemKV(), newMemKV()
		objects.data[pastMeetingKey] = mustMsgpackKV(t, pastMeetingRecord(softDeleted))
		if tombstoned {
			mappings.data[liveMarkerKey] = []byte(tombstoneMarker)
			mappings.data[committeesKey] = []byte(tombstoneMarker)
		}
		pub := &retriggerTestPublisher{}
		return pub, mappings, newRetriggerHandler(pub, objects, mappings)
	}

	t.Run("control: live past meeting is re-indexed on mapping put", func(t *testing.T) {
		pub, mappings, h := setup(t, false, false)

		retry := handleKVPut(context.Background(), mappingKey, mappingRecord(false), h)

		assert.False(t, retry)
		assert.Equal(t, 1, pub.pastMeetingEvents)
		assert.Equal(t, "1", string(mappings.data[liveMarkerKey]))
	})

	for _, mappingSoftDeleted := range []bool{false, true} {
		name := "mapping put"
		if mappingSoftDeleted {
			name = "mapping soft delete"
		}

		t.Run(name+" after past meeting soft delete publishes nothing", func(t *testing.T) {
			pub, mappings, h := setup(t, true, true)

			retry := handleKVPut(context.Background(), mappingKey, mappingRecord(mappingSoftDeleted), h)

			assert.False(t, retry)
			assert.Zero(t, pub.pastMeetingEvents)
			assert.Equal(t, tombstoneMarker, string(mappings.data[liveMarkerKey]))
			assert.Equal(t, tombstoneMarker, string(mappings.data[committeesKey]))
		})

		t.Run(name+" for soft-deleted record without tombstone publishes nothing", func(t *testing.T) {
			pub, mappings, h := setup(t, true, false)

			retry := handleKVPut(context.Background(), mappingKey, mappingRecord(mappingSoftDeleted), h)

			assert.False(t, retry)
			assert.Zero(t, pub.pastMeetingEvents)
			assert.NotContains(t, mappings.data, liveMarkerKey)
		})

		t.Run(name+" for tombstoned past meeting with live record publishes nothing", func(t *testing.T) {
			pub, mappings, h := setup(t, false, true)

			retry := handleKVPut(context.Background(), mappingKey, mappingRecord(mappingSoftDeleted), h)

			assert.False(t, retry)
			assert.Zero(t, pub.pastMeetingEvents)
			assert.Equal(t, tombstoneMarker, string(mappings.data[liveMarkerKey]))
			assert.Equal(t, tombstoneMarker, string(mappings.data[committeesKey]))
		})
	}

	t.Run("tombstone read error retries without publishing", func(t *testing.T) {
		pub, mappings, h := setup(t, false, false)
		mappings.getErrs[liveMarkerKey] = errors.New("kv unavailable")

		retry := handleKVPut(context.Background(), mappingKey, mappingRecord(false), h)

		assert.True(t, retry)
		assert.Zero(t, pub.pastMeetingEvents)
	})

	t.Run("restore of a live record still re-indexes a tombstoned past meeting", func(t *testing.T) {
		pub, mappings, h := setup(t, false, true)

		retry := handleKVPut(context.Background(), pastMeetingKey, pastMeetingRecord(false), h)

		assert.False(t, retry)
		assert.Equal(t, 1, pub.pastMeetingEvents)
		assert.Equal(t, "1", string(mappings.data[liveMarkerKey]))
	})

	t.Run("handlePastMeetingUpdate ignores a soft-deleted record", func(t *testing.T) {
		pub, mappings, h := setup(t, false, false)
		h.v1ObjectsKV = nil // artifact republish must not run

		retry := h.handlePastMeetingUpdate(context.Background(), pastMeetingKey, pastMeetingRecord(true))

		require.False(t, retry)
		assert.Zero(t, pub.pastMeetingEvents)
		assert.NotContains(t, mappings.data, liveMarkerKey)
	})
}
