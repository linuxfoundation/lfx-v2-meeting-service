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
// meeting handlers make. getErrs and putErrs inject an error for a specific key; a value in
// arriving is reported missing on the first Get and present from then on, simulating a
// record written concurrently by another consumer.
type memKV struct {
	jetstream.KeyValue
	data     map[string][]byte
	getErrs  map[string]error
	putErrs  map[string]error
	arriving map[string][]byte
	gets     []string
}

func newMemKV() *memKV {
	return &memKV{data: map[string][]byte{}, getErrs: map[string]error{}, putErrs: map[string]error{}, arriving: map[string][]byte{}}
}

func (k *memKV) Get(_ context.Context, key string) (jetstream.KeyValueEntry, error) {
	k.gets = append(k.gets, key)
	if err, ok := k.getErrs[key]; ok {
		return nil, err
	}
	v, ok := k.data[key]
	if !ok {
		if pending, ok := k.arriving[key]; ok {
			k.data[key] = pending
			delete(k.arriving, key)
		}
		return nil, jetstream.ErrKeyNotFound
	}
	return mockKeyValueEntry{key: key, value: v}, nil
}

func (k *memKV) Put(_ context.Context, key string, value []byte) (uint64, error) {
	if err, ok := k.putErrs[key]; ok {
		return 0, err
	}
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
	lastMeeting       *models.MeetingEventData
	lastPastMeeting   *models.PastMeetingEventData
	// onPublish runs after each meeting or past meeting publish, to simulate a delete that
	// is processed concurrently with the republish.
	onPublish func()
}

func (p *retriggerTestPublisher) PublishMeetingEvent(_ context.Context, _ string, data *models.MeetingEventData) error {
	p.meetingEvents++
	p.lastMeeting = data
	if p.onPublish != nil {
		p.onPublish()
	}
	return nil
}

func (p *retriggerTestPublisher) PublishMeetingHostCredentialsEvent(_ context.Context, _ string, _ *models.MeetingHostCredentialsEventData) error {
	p.hostCredsEvents++
	return nil
}

func (p *retriggerTestPublisher) PublishPastMeetingEvent(_ context.Context, _ string, data *models.PastMeetingEventData) error {
	p.pastMeetingEvents++
	p.lastPastMeeting = data
	if p.onPublish != nil {
		p.onPublish()
	}
	return nil
}

// unmappedProjectIDMapper reports no v2 project for any v1 project, so update handlers take
// their "project not mapped yet" path.
type unmappedProjectIDMapper struct{ recordingTestIDMapper }

func (unmappedProjectIDMapper) MapProjectV1ToV2(_ context.Context, _ string) (string, error) {
	return "", nil
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

		t.Run(name+" for hard-deleted meeting publishes nothing", func(t *testing.T) {
			pub, mappings, h := setup(t, false, true)
			delete(h.v1ObjectsKV.(*memKV).data, meetingKey)

			retry := handleKVPut(context.Background(), mappingKey, mappingRecord(mappingSoftDeleted), h)

			assert.False(t, retry)
			assert.Zero(t, pub.meetingEvents)
			assert.Equal(t, tombstoneMarker, string(mappings.data[liveMarkerKey]))
			assert.Equal(t, tombstoneMarker, string(mappings.data[committeesKey]))
		})

		t.Run(name+" during restore does not re-index before the meeting put", func(t *testing.T) {
			pub, mappings, h := setup(t, false, true)

			retry := handleKVPut(context.Background(), mappingKey, mappingRecord(mappingSoftDeleted), h)

			assert.False(t, retry)
			assert.Zero(t, pub.meetingEvents)
			assert.Equal(t, tombstoneMarker, string(mappings.data[liveMarkerKey]))
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

	t.Run("parent read error retries without storing the mapping", func(t *testing.T) {
		pub, mappings, h := setup(t, false, false)
		h.v1ObjectsKV.(*memKV).getErrs[meetingKey] = errors.New("kv unavailable")

		retry := handleKVPut(context.Background(), mappingKey, mappingRecord(false), h)

		assert.True(t, retry)
		assert.Zero(t, pub.meetingEvents)
		assert.NotContains(t, mappings.data, committeesKey)
	})

	t.Run("mapping put before the meeting is synced is stored for later", func(t *testing.T) {
		pub, mappings, h := setup(t, false, false)
		delete(h.v1ObjectsKV.(*memKV).data, meetingKey)

		retry := handleKVPut(context.Background(), mappingKey, mappingRecord(false), h)

		assert.False(t, retry)
		assert.Zero(t, pub.meetingEvents)
		assert.Contains(t, string(mappings.data[committeesKey]), "map-1")
	})

	t.Run("non-structural parent decode error retries without storing the mapping", func(t *testing.T) {
		pub, mappings, h := setup(t, false, false)
		h.v1ObjectsKV.(*memKV).data[meetingKey] = mustMsgpackKV(t, "not a map")

		retry := handleKVPut(context.Background(), mappingKey, mappingRecord(false), h)

		assert.True(t, retry)
		assert.Zero(t, pub.meetingEvents)
		assert.NotContains(t, mappings.data, committeesKey)
	})

	t.Run("structurally corrupt parent is skipped without retry", func(t *testing.T) {
		pub, mappings, h := setup(t, false, false)
		h.v1ObjectsKV.(*memKV).data[meetingKey] = buildMapWrappedNestedFixarrays(msgpackMaxNestingDepth + 100)

		retry := handleKVPut(context.Background(), mappingKey, mappingRecord(false), h)

		assert.False(t, retry)
		assert.Zero(t, pub.meetingEvents)
		assert.NotContains(t, mappings.data, committeesKey)
	})

	t.Run("meeting arriving after the parent check is re-indexed with the mapping", func(t *testing.T) {
		pub, mappings, h := setup(t, false, false)
		objects := h.v1ObjectsKV.(*memKV)
		objects.arriving[meetingKey] = objects.data[meetingKey]
		delete(objects.data, meetingKey)

		retry := handleKVPut(context.Background(), mappingKey, mappingRecord(false), h)

		assert.False(t, retry)
		assert.Equal(t, 1, pub.meetingEvents)
		assert.Equal(t, "1", string(mappings.data[liveMarkerKey]))
	})

	t.Run("restore keeps committee links that arrive before the meeting put", func(t *testing.T) {
		pub, mappings, h := setup(t, false, true)

		require.False(t, handleKVPut(context.Background(), mappingKey, mappingRecord(false), h))
		require.Zero(t, pub.meetingEvents)
		require.False(t, handleKVPut(context.Background(), meetingKey, meetingRecord(false), h))

		assert.Equal(t, 1, pub.meetingEvents)
		require.NotNil(t, pub.lastMeeting)
		assert.Len(t, pub.lastMeeting.Committees, 1)
		assert.Equal(t, "1", string(mappings.data[liveMarkerKey]))
	})

	t.Run("restore retries until the live marker replaces the tombstone", func(t *testing.T) {
		pub, mappings, h := setup(t, false, true)
		mappings.putErrs[liveMarkerKey] = errors.New("kv unavailable")

		require.True(t, handleKVPut(context.Background(), meetingKey, meetingRecord(false), h))
		require.Equal(t, tombstoneMarker, string(mappings.data[liveMarkerKey]))

		delete(mappings.putErrs, liveMarkerKey)
		require.False(t, handleKVPut(context.Background(), meetingKey, meetingRecord(false), h))
		assert.Equal(t, "1", string(mappings.data[liveMarkerKey]))

		// Later mapping events now re-index the restored meeting.
		before := pub.meetingEvents
		assert.False(t, handleKVPut(context.Background(), mappingKey, mappingRecord(false), h))
		assert.Equal(t, before+1, pub.meetingEvents)
	})

	t.Run("restore before the project is mapped clears the tombstone for later mapping events", func(t *testing.T) {
		pub, mappings, h := setup(t, false, true)
		h.idMapper = unmappedProjectIDMapper{}

		require.False(t, handleKVPut(context.Background(), meetingKey, meetingRecord(false), h))
		require.Zero(t, pub.meetingEvents)
		require.NotContains(t, mappings.data, liveMarkerKey)

		h.idMapper = recordingTestIDMapper{}
		assert.False(t, handleKVPut(context.Background(), mappingKey, mappingRecord(false), h))
		assert.Equal(t, 1, pub.meetingEvents)
		assert.Equal(t, "1", string(mappings.data[liveMarkerKey]))
	})

	t.Run("invalid tombstone key is treated as absent, not retried", func(t *testing.T) {
		pub, mappings, h := setup(t, false, false)
		h.idMapper = unmappedProjectIDMapper{}
		mappings.getErrs[liveMarkerKey] = jetstream.ErrInvalidKey

		retry := handleKVPut(context.Background(), meetingKey, meetingRecord(false), h)

		assert.False(t, retry)
		assert.Zero(t, pub.meetingEvents)
	})

	for _, cleanupDone := range []bool{false, true} {
		name := "delete racing the retrigger is replayed (delete not yet processed)"
		if cleanupDone {
			name = "delete racing the retrigger is replayed (delete already processed)"
		}
		t.Run(name, func(t *testing.T) {
			pub, mappings, h := setup(t, false, false)
			objects := h.v1ObjectsKV.(*memKV)
			pub.onPublish = func() {
				objects.data[meetingKey] = mustMsgpackKV(t, meetingRecord(true))
				if cleanupDone {
					mappings.data[liveMarkerKey] = []byte(tombstoneMarker)
					mappings.data[committeesKey] = []byte(tombstoneMarker)
				}
			}

			retry := handleKVPut(context.Background(), mappingKey, mappingRecord(false), h)

			assert.False(t, retry)
			assert.Equal(t, 1, pub.meetingEvents)
			assert.Contains(t, pub.indexerDeletes, "lfx.index.v1_meeting:"+meetingID)
			assert.Equal(t, tombstoneMarker, string(mappings.data[liveMarkerKey]))
			assert.Equal(t, tombstoneMarker, string(mappings.data[committeesKey]))
		})
	}

	t.Run("deleted meeting still marked live replays the delete", func(t *testing.T) {
		pub, mappings, h := setup(t, true, false)
		mappings.data[liveMarkerKey] = []byte("1")

		retry := handleKVPut(context.Background(), mappingKey, mappingRecord(true), h)

		assert.False(t, retry)
		assert.Zero(t, pub.meetingEvents)
		assert.Contains(t, pub.indexerDeletes, "lfx.index.v1_meeting:"+meetingID)
		assert.Equal(t, tombstoneMarker, string(mappings.data[liveMarkerKey]))
	})

	t.Run("corrupt meeting record marked live is not treated as deleted", func(t *testing.T) {
		pub, mappings, h := setup(t, false, false)
		h.v1ObjectsKV.(*memKV).data[meetingKey] = buildMapWrappedNestedFixarrays(msgpackMaxNestingDepth + 100)
		mappings.data[liveMarkerKey] = []byte("1")

		retry := handleKVPut(context.Background(), mappingKey, mappingRecord(true), h)

		assert.False(t, retry)
		assert.Empty(t, pub.indexerDeletes)
		assert.Equal(t, "1", string(mappings.data[liveMarkerKey]))
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

		t.Run(name+" for hard-deleted past meeting publishes nothing", func(t *testing.T) {
			pub, mappings, h := setup(t, false, true)
			delete(h.v1ObjectsKV.(*memKV).data, pastMeetingKey)

			retry := handleKVPut(context.Background(), mappingKey, mappingRecord(mappingSoftDeleted), h)

			assert.False(t, retry)
			assert.Zero(t, pub.pastMeetingEvents)
			assert.Equal(t, tombstoneMarker, string(mappings.data[liveMarkerKey]))
			assert.Equal(t, tombstoneMarker, string(mappings.data[committeesKey]))
		})

		t.Run(name+" during restore does not re-index before the past meeting put", func(t *testing.T) {
			pub, mappings, h := setup(t, false, true)

			retry := handleKVPut(context.Background(), mappingKey, mappingRecord(mappingSoftDeleted), h)

			assert.False(t, retry)
			assert.Zero(t, pub.pastMeetingEvents)
			assert.Equal(t, tombstoneMarker, string(mappings.data[liveMarkerKey]))
		})
	}

	t.Run("tombstone read error retries without publishing", func(t *testing.T) {
		pub, mappings, h := setup(t, false, false)
		mappings.getErrs[liveMarkerKey] = errors.New("kv unavailable")

		retry := handleKVPut(context.Background(), mappingKey, mappingRecord(false), h)

		assert.True(t, retry)
		assert.Zero(t, pub.pastMeetingEvents)
	})

	t.Run("parent read error retries without storing the mapping", func(t *testing.T) {
		pub, mappings, h := setup(t, false, false)
		h.v1ObjectsKV.(*memKV).getErrs[pastMeetingKey] = errors.New("kv unavailable")

		retry := handleKVPut(context.Background(), mappingKey, mappingRecord(false), h)

		assert.True(t, retry)
		assert.Zero(t, pub.pastMeetingEvents)
		assert.NotContains(t, mappings.data, committeesKey)
	})

	t.Run("mapping put before the past meeting is synced is stored for later", func(t *testing.T) {
		pub, mappings, h := setup(t, false, false)
		delete(h.v1ObjectsKV.(*memKV).data, pastMeetingKey)

		retry := handleKVPut(context.Background(), mappingKey, mappingRecord(false), h)

		assert.False(t, retry)
		assert.Zero(t, pub.pastMeetingEvents)
		assert.Contains(t, string(mappings.data[committeesKey]), "map-1")
	})

	t.Run("non-structural parent decode error retries without storing the mapping", func(t *testing.T) {
		pub, mappings, h := setup(t, false, false)
		h.v1ObjectsKV.(*memKV).data[pastMeetingKey] = mustMsgpackKV(t, "not a map")

		retry := handleKVPut(context.Background(), mappingKey, mappingRecord(false), h)

		assert.True(t, retry)
		assert.Zero(t, pub.pastMeetingEvents)
		assert.NotContains(t, mappings.data, committeesKey)
	})

	t.Run("structurally corrupt parent is skipped without retry", func(t *testing.T) {
		pub, mappings, h := setup(t, false, false)
		h.v1ObjectsKV.(*memKV).data[pastMeetingKey] = buildMapWrappedNestedFixarrays(msgpackMaxNestingDepth + 100)

		retry := handleKVPut(context.Background(), mappingKey, mappingRecord(false), h)

		assert.False(t, retry)
		assert.Zero(t, pub.pastMeetingEvents)
		assert.NotContains(t, mappings.data, committeesKey)
	})

	t.Run("past meeting arriving after the parent check is re-indexed with the mapping", func(t *testing.T) {
		pub, mappings, h := setup(t, false, false)
		objects := h.v1ObjectsKV.(*memKV)
		objects.arriving[pastMeetingKey] = objects.data[pastMeetingKey]
		delete(objects.data, pastMeetingKey)

		retry := handleKVPut(context.Background(), mappingKey, mappingRecord(false), h)

		assert.False(t, retry)
		assert.Equal(t, 1, pub.pastMeetingEvents)
		assert.Equal(t, "1", string(mappings.data[liveMarkerKey]))
	})

	t.Run("restore keeps committee links that arrive before the past meeting put", func(t *testing.T) {
		pub, mappings, h := setup(t, false, true)

		require.False(t, handleKVPut(context.Background(), mappingKey, mappingRecord(false), h))
		require.Zero(t, pub.pastMeetingEvents)
		require.False(t, handleKVPut(context.Background(), pastMeetingKey, pastMeetingRecord(false), h))

		assert.Equal(t, 1, pub.pastMeetingEvents)
		require.NotNil(t, pub.lastPastMeeting)
		assert.Len(t, pub.lastPastMeeting.Committees, 1)
		assert.Equal(t, "1", string(mappings.data[liveMarkerKey]))
	})

	t.Run("restore retries until the live marker replaces the tombstone", func(t *testing.T) {
		pub, mappings, h := setup(t, false, true)
		mappings.putErrs[liveMarkerKey] = errors.New("kv unavailable")

		require.True(t, handleKVPut(context.Background(), pastMeetingKey, pastMeetingRecord(false), h))
		require.Equal(t, tombstoneMarker, string(mappings.data[liveMarkerKey]))

		delete(mappings.putErrs, liveMarkerKey)
		require.False(t, handleKVPut(context.Background(), pastMeetingKey, pastMeetingRecord(false), h))
		assert.Equal(t, "1", string(mappings.data[liveMarkerKey]))

		// Later mapping events now re-index the restored past meeting.
		before := pub.pastMeetingEvents
		assert.False(t, handleKVPut(context.Background(), mappingKey, mappingRecord(false), h))
		assert.Equal(t, before+1, pub.pastMeetingEvents)
	})

	t.Run("restore before the project is mapped clears the tombstone for later mapping events", func(t *testing.T) {
		pub, mappings, h := setup(t, false, true)
		h.idMapper = unmappedProjectIDMapper{}

		require.False(t, handleKVPut(context.Background(), pastMeetingKey, pastMeetingRecord(false), h))
		require.Zero(t, pub.pastMeetingEvents)
		require.NotContains(t, mappings.data, liveMarkerKey)

		h.idMapper = recordingTestIDMapper{}
		assert.False(t, handleKVPut(context.Background(), mappingKey, mappingRecord(false), h))
		assert.Equal(t, 1, pub.pastMeetingEvents)
		assert.Equal(t, "1", string(mappings.data[liveMarkerKey]))
	})

	t.Run("artifacts are republished even when the live marker write fails", func(t *testing.T) {
		_, mappings, h := setup(t, false, false)
		mappings.putErrs[liveMarkerKey] = errors.New("kv unavailable")
		objects := h.v1ObjectsKV.(*memKV)

		retry := handleKVPut(context.Background(), pastMeetingKey, pastMeetingRecord(false), h)

		assert.True(t, retry)
		assert.Contains(t, objects.gets, "itx-zoom-past-meetings-recordings."+pastMeetingID)
	})

	t.Run("invalid tombstone key is treated as absent, not retried", func(t *testing.T) {
		pub, mappings, h := setup(t, false, false)
		h.idMapper = unmappedProjectIDMapper{}
		mappings.getErrs[liveMarkerKey] = jetstream.ErrInvalidKey

		retry := handleKVPut(context.Background(), pastMeetingKey, pastMeetingRecord(false), h)

		assert.False(t, retry)
		assert.Zero(t, pub.pastMeetingEvents)
	})

	for _, cleanupDone := range []bool{false, true} {
		name := "delete racing the retrigger is replayed (delete not yet processed)"
		if cleanupDone {
			name = "delete racing the retrigger is replayed (delete already processed)"
		}
		t.Run(name, func(t *testing.T) {
			pub, mappings, h := setup(t, false, false)
			objects := h.v1ObjectsKV.(*memKV)
			pub.onPublish = func() {
				objects.data[pastMeetingKey] = mustMsgpackKV(t, pastMeetingRecord(true))
				if cleanupDone {
					mappings.data[liveMarkerKey] = []byte(tombstoneMarker)
					mappings.data[committeesKey] = []byte(tombstoneMarker)
				}
			}

			retry := handleKVPut(context.Background(), mappingKey, mappingRecord(false), h)

			assert.False(t, retry)
			assert.Equal(t, 1, pub.pastMeetingEvents)
			assert.Contains(t, pub.indexerDeletes, "lfx.index.v1_past_meeting:"+pastMeetingID)
			assert.Equal(t, tombstoneMarker, string(mappings.data[liveMarkerKey]))
			assert.Equal(t, tombstoneMarker, string(mappings.data[committeesKey]))
		})
	}

	t.Run("deleted past meeting still marked live replays the delete", func(t *testing.T) {
		pub, mappings, h := setup(t, true, false)
		mappings.data[liveMarkerKey] = []byte("1")

		retry := handleKVPut(context.Background(), mappingKey, mappingRecord(true), h)

		assert.False(t, retry)
		assert.Zero(t, pub.pastMeetingEvents)
		assert.Contains(t, pub.indexerDeletes, "lfx.index.v1_past_meeting:"+pastMeetingID)
		assert.Equal(t, tombstoneMarker, string(mappings.data[liveMarkerKey]))
	})

	t.Run("corrupt past meeting record marked live is not treated as deleted", func(t *testing.T) {
		pub, mappings, h := setup(t, false, false)
		h.v1ObjectsKV.(*memKV).data[pastMeetingKey] = buildMapWrappedNestedFixarrays(msgpackMaxNestingDepth + 100)
		mappings.data[liveMarkerKey] = []byte("1")

		retry := handleKVPut(context.Background(), mappingKey, mappingRecord(true), h)

		assert.False(t, retry)
		assert.Empty(t, pub.indexerDeletes)
		assert.Equal(t, "1", string(mappings.data[liveMarkerKey]))
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
