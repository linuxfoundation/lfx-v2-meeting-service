// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	fgaconstants "github.com/linuxfoundation/lfx-v2-fga-sync/pkg/constants"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/infrastructure/eventing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func assertPastMeetingCleanupComplete(t *testing.T, mappings *memKV, parentID string) {
	t.Helper()
	var completion pastMeetingCleanupCompletion
	require.NoError(t, json.Unmarshal(mappings.data[pastMeetingCleanupKeyPrefix+parentID], &completion))
	assert.True(t, completion.Complete)
}

func TestPastMeetingBatchRepair_RevalidatesParentAndFinishesCompensation(t *testing.T) {
	for _, mode := range []string{"changed live parent", "deleted parent pending compensation"} {
		t.Run(mode, func(t *testing.T) {
			objects, mappings := newMemKV(), newMemKV()
			parent := map[string]any{"meeting_and_occurrence_id": testSummaryMeetOcc, "proj_id": "proj-sfid"}
			objects.data[testPastMeetingKey] = mustMarshalJSON(t, parent)
			mappings.data["v1_past_meetings."+testSummaryMeetOcc] = []byte("1")
			key := "itx-zoom-past-meetings-invitees.invitee-1"
			intent := newChildRepairIntent(testSummaryMeetOcc, childRepairReady)
			intent.FullReplay = false
			intent.ParentLive, intent.ParentSynchronized = true, true
			intent.ParentRevision = objects.revision(testPastMeetingKey)
			intent.ParentMappingRevision = mappings.revision("v1_past_meetings." + testSummaryMeetOcc)
			publisher := &pastMeetingCleanupPublisher{}
			h := newRetriggerHandler(nil, objects, mappings)
			h.publisher = publisher
			if mode == "changed live parent" {
				// Restoration rewrote mapping/replay state but not the source.
				_, err := mappings.Put(context.Background(), "v1_past_meetings."+testSummaryMeetOcc, []byte("1"))
				require.NoError(t, err)
				mappings.data[pastMeetingAttendeeIndexPrefix+testSummaryMeetOcc+".attendee-1"] = []byte("1")
				objects.data["itx-zoom-past-meetings-attendees.attendee-1"] = mustMarshalJSON(t, minimalAttendeeV1Data("attendee-1", testSummaryMeetOcc, "alice"))
			} else {
				parent["_sdc_deleted_at"] = "2026-01-02T00:00:00Z"
				objects.data[testPastMeetingKey] = mustMarshalJSON(t, parent)
				mappings.data["v1_past_meetings."+testSummaryMeetOcc] = []byte(tombstoneMarker)
				mappings.data[pastMeetingCleanupKeyPrefix+testSummaryMeetOcc] = mustMarshalJSON(t, pastMeetingCleanupCompletion{Complete: true, SourceRevision: objects.revision(testPastMeetingKey)})
				intent.FullReplay = true
				intent.ParticipantIDs = []string{"invitee-1", "old-sibling"}
				publisher.failSubject = eventing.IndexV1PastMeetingParticipantSubject
			}
			markerKey := pastMeetingChildRepairPrefix + key
			mappings.data[markerKey] = mustMarshalJSON(t, intent)
			drain := func() bool {
				return h.drainPastMeetingChildRepairs(context.Background(), &pastMeetingReplayState{
					parentID: testSummaryMeetOcc, ready: map[string]uint64{key: mappings.revision(markerKey)},
				})
			}
			if mode == "deleted parent pending compensation" {
				require.True(t, drain())
				assert.Contains(t, mappings.data, markerKey, "failed compensation must not be discarded by parent-only recovery")
				assert.False(t, readChildRepairIntent(t, mappings, key).ParticipantCompensated)
				assert.Zero(t, publisher.accessDeletes)
				publisher.failSubject = ""
			}
			require.False(t, drain())
			assert.NotContains(t, mappings.data, markerKey)
			if mode == "changed live parent" {
				assert.Positive(t, publisher.parentPuts)
				assert.Contains(t, publisher.participants, "attendee-1", "the batch must restore the sibling after the earlier per-user removal")
			} else {
				assert.Contains(t, publisher.indexerDeletes, eventing.IndexV1PastMeetingParticipantSubject+":invitee-1")
				assert.Contains(t, publisher.indexerDeletes, eventing.IndexV1PastMeetingParticipantSubject+":old-sibling")
				assert.Equal(t, 1, publisher.accessDeletes)
			}
		})
	}
}

func TestPastMeetingParticipant_UsernameRemovalIsProtectedBeforeRestore(t *testing.T) {
	for _, failXref := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "early error and retry"}[failXref], func(t *testing.T) {
			objects, mappings := newMemKV(), newMemKV()
			parent := map[string]any{"meeting_and_occurrence_id": testSummaryMeetOcc, "proj_id": "proj-sfid"}
			objects.data[testPastMeetingKey] = mustMarshalJSON(t, parent)
			mappings.data["v1_past_meetings."+testSummaryMeetOcc] = []byte("1")
			key := "itx-zoom-past-meetings-invitees.invitee-1"
			data := minimalInviteeV1Data("invitee-1", testSummaryMeetOcc, "bob")
			objects.data[key] = mustMarshalJSON(t, data)
			mappings.data["v1_past_meeting_invitees.invitee-1"] = []byte(buildRegistrantMappingValue("invitee-1", "alice", testSummaryMeetOcc))
			oldXref := "v1_participant_by_meeting_user.invitee." + testSummaryMeetOcc + ".alice"
			mappings.data[oldXref] = []byte("invitee-1")
			objects.data["itx-zoom-past-meetings-attendees.attendee-1"] = mustMarshalJSON(t, minimalAttendeeV1Data("attendee-1", testSummaryMeetOcc, "alice"))
			publisher := &pastMeetingCleanupPublisher{}
			h, other := newRetriggerHandler(nil, objects, mappings), newRetriggerHandler(nil, objects, mappings)
			h.publisher, other.publisher = publisher, publisher
			publisher.beforeAccess = func(subject string) {
				if subject != fgaconstants.GenericMemberRemoveSubject {
					return
				}
				publisher.beforeAccess = nil
				require.Equal(t, childRepairWriting, readChildRepairIntent(t, mappings, key).Phase, "ownership must precede old-username removal")
				deleted := map[string]any{"meeting_and_occurrence_id": testSummaryMeetOcc, "proj_id": "proj-sfid", "_sdc_deleted_at": "2026-01-02T00:00:00Z"}
				_, err := objects.Put(context.Background(), testPastMeetingKey, mustMarshalJSON(t, deleted))
				require.NoError(t, err)
				require.False(t, other.handlePastMeetingDelete(context.Background(), testPastMeetingKey, nil))
				_, err = objects.Put(context.Background(), testPastMeetingKey, mustMarshalJSON(t, parent))
				require.NoError(t, err)
				require.True(t, other.handlePastMeetingUpdate(context.Background(), testPastMeetingKey, parent), "the restored parent must retain work blocked by this owner")
				if failXref {
					mappings.putErrs[oldXref] = errors.New("kv unavailable")
				}
			}
			assert.Equal(t, failXref, h.handlePastMeetingInviteeUpdate(context.Background(), key, data))
			if failXref {
				assert.Contains(t, mappings.data, pastMeetingChildRepairPrefix+key)
				clear(mappings.putErrs)
				require.False(t, h.handlePastMeetingInviteeUpdate(context.Background(), key, data))
			}
			lastRemove, lastSibling := -1, -1
			for i, event := range publisher.events {
				if event == "access:"+fgaconstants.GenericMemberRemoveSubject {
					lastRemove = i
				}
				if event == "participant-put:attendee-1" {
					lastSibling = i
				}
			}
			require.GreaterOrEqual(t, lastRemove, 0)
			assert.Greater(t, lastSibling, lastRemove)
			assert.NotContains(t, mappings.data, pastMeetingChildRepairPrefix+key)
		})
	}
}

func TestPastMeetingRestore_RetriesChildBlockedDuringOwnerFinalFlush(t *testing.T) {
	objects, mappings := newMemKV(), newMemKV()
	parent := map[string]any{"meeting_and_occurrence_id": testSummaryMeetOcc, "proj_id": "proj-sfid"}
	objects.data[testPastMeetingKey] = mustMarshalJSON(t, parent)
	mappings.data["v1_past_meetings."+testSummaryMeetOcc] = []byte("1")
	key := "itx-zoom-past-meetings-invitees.invitee-1"
	data := minimalInviteeV1Data("invitee-1", testSummaryMeetOcc, "alice")
	objects.data[key] = mustMarshalJSON(t, data)
	mappings.data[pastMeetingInviteeIndexPrefix+testSummaryMeetOcc+".invitee-1"] = []byte("1")
	publisher := &pastMeetingCleanupPublisher{}
	h, other := newRetriggerHandler(nil, objects, mappings), newRetriggerHandler(nil, objects, mappings)
	h.publisher, other.publisher = publisher, publisher
	interleaved := false
	publisher.onFlush = func() {
		if readChildRepairIntent(t, mappings, key).Phase != childRepairRunning {
			return
		}
		publisher.onFlush = nil
		interleaved = true
		deleted := map[string]any{"meeting_and_occurrence_id": testSummaryMeetOcc, "proj_id": "proj-sfid", "_sdc_deleted_at": "2026-01-02T00:00:00Z"}
		_, err := objects.Put(context.Background(), testPastMeetingKey, mustMarshalJSON(t, deleted))
		require.NoError(t, err)
		require.False(t, other.handlePastMeetingDelete(context.Background(), testPastMeetingKey, nil))
		_, err = objects.Put(context.Background(), testPastMeetingKey, mustMarshalJSON(t, parent))
		require.NoError(t, err)
		require.True(t, other.handlePastMeetingUpdate(context.Background(), testPastMeetingKey, parent))
		assert.Contains(t, mappings.data, pastMeetingRestorePendingPrefix+testSummaryMeetOcc)
	}
	require.False(t, h.handlePastMeetingInviteeUpdate(context.Background(), key, data))
	require.True(t, interleaved)
	assert.NotContains(t, mappings.data, pastMeetingChildRepairPrefix+key)
	// Redeliver only the parent event, after the foreign owner has completed.
	require.False(t, other.handlePastMeetingUpdate(context.Background(), testPastMeetingKey, parent))
	assert.NotContains(t, mappings.data, pastMeetingRestorePendingPrefix+testSummaryMeetOcc)
	assert.GreaterOrEqual(t, len(publisher.participants), 2)
	assert.Equal(t, "invitee-1", publisher.participants[len(publisher.participants)-1], "parent-only recovery restores the participant before completing the parent owner")
}

func TestPastMeetingCleanup_CompletionMatchesDeletionGeneration(t *testing.T) {
	for _, deletion := range []string{"soft", "hard", "legacy completion"} {
		t.Run(deletion, func(t *testing.T) {
			objects, mappings := newMemKV(), newMemKV()
			parent := map[string]any{"meeting_and_occurrence_id": testSummaryMeetOcc, "proj_id": "proj-sfid", "recording_access": "public", "ai_summary_access": "public"}
			deleted := map[string]any{"meeting_and_occurrence_id": testSummaryMeetOcc, "proj_id": "proj-sfid", "_sdc_deleted_at": "2026-01-02T00:00:00Z"}
			_, err := objects.Put(context.Background(), testPastMeetingKey, mustMarshalJSON(t, deleted))
			require.NoError(t, err)
			recordingKey := "itx-zoom-past-meetings-recordings." + testSummaryMeetOcc
			recording := map[string]any{"meeting_and_occurrence_id": testSummaryMeetOcc, "proj_id": "proj-sfid"}
			summary := map[string]any{"id": testSummaryID, "meeting_and_occurrence_id": testSummaryMeetOcc, "approved": true}
			objects.data[recordingKey] = mustMarshalJSON(t, recording)
			objects.data[testSummaryKVKey] = mustMarshalJSON(t, summary)
			publisher := &pastMeetingCleanupPublisher{}
			h := newRetriggerHandler(nil, objects, mappings)
			h.publisher = publisher
			require.False(t, h.handlePastMeetingDelete(context.Background(), testPastMeetingKey, nil))
			var first pastMeetingCleanupCompletion
			require.NoError(t, json.Unmarshal(mappings.data[pastMeetingCleanupKeyPrefix+testSummaryMeetOcc], &first))
			_, err = objects.Put(context.Background(), testPastMeetingKey, mustMarshalJSON(t, parent))
			require.NoError(t, err)
			// Child events can index public content before the parent restore
			// event runs. Preserve its old tombstone/completion throughout.
			require.False(t, h.handlePastMeetingRecordingUpdate(context.Background(), recordingKey, recording))
			require.False(t, h.handlePastMeetingSummaryUpdate(context.Background(), testSummaryKVKey, summary))
			require.NotNil(t, publisher.recordingData)
			require.Positive(t, publisher.summaries)
			if deletion == "hard" {
				require.NoError(t, objects.Delete(context.Background(), testPastMeetingKey))
			} else {
				_, err = objects.Put(context.Background(), testPastMeetingKey, mustMarshalJSON(t, deleted))
				require.NoError(t, err)
			}
			if deletion == "legacy completion" {
				mappings.data[pastMeetingCleanupKeyPrefix+testSummaryMeetOcc] = []byte("1")
			}
			publisher.indexerDeletes = nil
			require.False(t, h.handlePastMeetingDelete(context.Background(), testPastMeetingKey, nil))
			assert.Contains(t, publisher.indexerDeletes, eventing.IndexV1PastMeetingRecordingSubject+":"+testSummaryMeetOcc)
			assert.Contains(t, publisher.indexerDeletes, eventing.IndexV1PastMeetingSummarySubject+":"+testSummaryID)
			var current pastMeetingCleanupCompletion
			require.NoError(t, json.Unmarshal(mappings.data[pastMeetingCleanupKeyPrefix+testSummaryMeetOcc], &current))
			revision, err := h.parentSourceRevision(context.Background(), testSummaryMeetOcc)
			require.NoError(t, err)
			assert.Equal(t, revision, current.SourceRevision)
			assert.NotEqual(t, first.SourceRevision, current.SourceRevision)
			before := len(publisher.indexerDeletes)
			require.False(t, h.handlePastMeetingDelete(context.Background(), testPastMeetingKey, nil))
			assert.Len(t, publisher.indexerDeletes, before, "only a matching current-generation completion may skip cleanup")
		})
	}
}
