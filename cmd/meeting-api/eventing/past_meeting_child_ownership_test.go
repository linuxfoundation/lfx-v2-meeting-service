// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/infrastructure/eventing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func readChildRepairIntent(t *testing.T, mappings *memKV, key string) childRepairIntent {
	t.Helper()
	var intent childRepairIntent
	require.NoError(t, json.Unmarshal(mappings.data[pastMeetingChildRepairPrefix+key], &intent))
	return intent
}

func TestPastMeetingChildOwnership_OverlappingDeliveryCannotConsumeWriter(t *testing.T) {
	objects, mappings := newMemKV(), newMemKV()
	parent := map[string]any{"meeting_and_occurrence_id": testSummaryMeetOcc, "proj_id": "proj-sfid", "_sdc_deleted_at": "2026-01-02T00:00:00Z"}
	objects.data[testPastMeetingKey] = mustMarshalJSON(t, parent)
	key := testSummaryKVKey
	data := map[string]any{"id": testSummaryID, "meeting_and_occurrence_id": testSummaryMeetOcc}
	objects.data[key] = mustMarshalJSON(t, data)
	publisherA, publisherB := &pastMeetingCleanupPublisher{}, &pastMeetingCleanupPublisher{}
	workerA, workerB := newRetriggerHandler(nil, objects, mappings), newRetriggerHandler(nil, objects, mappings)
	workerA.publisher, workerB.publisher = publisherA, publisherB
	publisherA.beforeDelete = func(string) {
		publisherA.beforeDelete = nil
		intent := readChildRepairIntent(t, mappings, key)
		require.Equal(t, childRepairWriting, intent.Phase)
		delete(parent, "_sdc_deleted_at")
		objects.data[testPastMeetingKey] = mustMarshalJSON(t, parent)
		require.True(t, workerB.handlePastMeetingUpdate(context.Background(), testPastMeetingKey, parent), "restore must retain its own obligation while A is still writing")
		revision := mappings.revision(pastMeetingChildRepairPrefix + key)
		// A is still paused BEFORE its delete. B must NAK, not clear A's
		// intent on the strength of a live parent and an unchanged revision.
		require.True(t, handleKVPut(context.Background(), key, data, workerB))
		assert.Equal(t, revision, mappings.revision(pastMeetingChildRepairPrefix+key))
		assert.Equal(t, intent.Owner, readChildRepairIntent(t, mappings, key).Owner)
	}

	require.False(t, handleKVPut(context.Background(), key, data, workerA))
	assert.NotContains(t, mappings.data, pastMeetingChildRepairPrefix+key)
	require.GreaterOrEqual(t, len(publisherA.events), 2)
	assert.Equal(t, "summary-put", publisherA.events[len(publisherA.events)-1], "A repairs after its destructive publish finishes")
}

func TestPastMeetingChildOwnership_RepairIsExclusivelyClaimed(t *testing.T) {
	objects, mappings := newMemKV(), newMemKV()
	objects.data[testPastMeetingKey] = mustMarshalJSON(t, map[string]any{"meeting_and_occurrence_id": testSummaryMeetOcc, "proj_id": "proj-sfid"})
	key := testSummaryKVKey
	mappings.data[pastMeetingChildRepairPrefix+key] = mustMarshalJSON(t, newChildRepairIntent(testSummaryMeetOcc, childRepairReady))
	publisher := &pastMeetingCleanupPublisher{}
	workerA, workerB := newRetriggerHandler(nil, objects, mappings), newRetriggerHandler(nil, objects, mappings)
	workerA.publisher = publisher
	workerB.publisher = &pastMeetingCleanupPublisher{}
	publisher.onParent = func() {
		publisher.onParent = nil
		intent := readChildRepairIntent(t, mappings, key)
		require.Equal(t, childRepairRunning, intent.Phase)
		require.True(t, workerB.resumePastMeetingChildRepair(context.Background(), key))
		assert.Equal(t, intent, readChildRepairIntent(t, mappings, key))
	}
	require.False(t, workerA.resumePastMeetingChildRepair(context.Background(), key))
	assert.NotContains(t, mappings.data, pastMeetingChildRepairPrefix+key)
}

func TestPastMeetingChildOwnership_FailClosedRecovery(t *testing.T) {
	for _, failure := range []string{"writer crash", "writing flush", "writing handoff", "repairing flush", "legacy marker"} {
		t.Run(failure, func(t *testing.T) {
			objects, mappings := newMemKV(), newMemKV()
			objects.data[testPastMeetingKey] = mustMarshalJSON(t, map[string]any{"meeting_and_occurrence_id": testSummaryMeetOcc, "proj_id": "proj-sfid"})
			key := testSummaryKVKey
			markerKey := pastMeetingChildRepairPrefix + key
			publisher := &pastMeetingCleanupPublisher{}
			worker := newRetriggerHandler(nil, objects, mappings)
			worker.publisher = publisher
			switch failure {
			case "legacy marker":
				mappings.data[markerKey] = []byte(testSummaryMeetOcc)
			case "repairing flush":
				mappings.data[markerKey] = mustMarshalJSON(t, newChildRepairIntent(testSummaryMeetOcc, childRepairReady))
				publisher.flushErr = errors.New("flush unavailable")
				require.True(t, worker.resumePastMeetingChildRepair(context.Background(), key))
				require.Equal(t, childRepairRunning, readChildRepairIntent(t, mappings, key).Phase)
			default:
				ctx, finish, blocked := worker.startPastMeetingChildAttempt(context.Background(), key)
				require.False(t, blocked)
				require.False(t, worker.beginPastMeetingChildRepair(ctx, key, testSummaryMeetOcc))
				switch failure {
				case "writing flush":
					publisher.flushErr = errors.New("flush unavailable")
					require.True(t, finish(false))
				case "writing handoff":
					mappings.putErrs[markerKey] = errors.New("kv unavailable")
					require.True(t, finish(false))
				case "writer crash":
					// Deliberately never call finish: the writer disappears.
				}
				require.Equal(t, childRepairWriting, readChildRepairIntent(t, mappings, key).Phase)
			}
			clear(mappings.putErrs)
			publisher.flushErr = nil
			before := string(mappings.data[markerKey])
			otherWorker := newRetriggerHandler(nil, objects, mappings)
			otherWorker.publisher = &pastMeetingCleanupPublisher{}
			require.True(t, otherWorker.resumePastMeetingChildRepair(context.Background(), key))
			assert.Equal(t, before, string(mappings.data[markerKey]), "no timeout/second worker may steal unfinished ownership")

			// Model operator recovery AFTER the old worker is stopped and its
			// connection can no longer publish. CAS prevents a stale recovery
			// instruction from overwriting a newer owner.
			revision := mappings.revision(markerKey)
			_, err := mappings.Update(context.Background(), markerKey, mustMarshalJSON(t, newChildRepairIntent(testSummaryMeetOcc, childRepairReady)), revision)
			require.NoError(t, err)
			if value := mappings.data[pastMeetingOperationPrefix+testSummaryMeetOcc]; value != nil {
				// A failed flush can also retain the enclosing parent owner.
				// Operator recovery follows confirmed termination of both scopes.
				var parent parentOperationRecord
				require.NoError(t, json.Unmarshal(value, &parent))
				parent.Phase = childRepairReady
				_, err := mappings.Update(context.Background(), pastMeetingOperationPrefix+testSummaryMeetOcc, mustMarshalJSON(t, parent), mappings.revision(pastMeetingOperationPrefix+testSummaryMeetOcc))
				require.NoError(t, err)
			}
			require.False(t, otherWorker.resumePastMeetingChildRepair(context.Background(), key))
			assert.NotContains(t, mappings.data, markerKey)
		})
	}
}

func TestPastMeetingChildOwnership_ClaimConflictPreventsDestructivePublish(t *testing.T) {
	objects, mappings := newMemKV(), newMemKV()
	h := newRetriggerHandler(nil, objects, mappings)
	h.publisher = &pastMeetingCleanupPublisher{}
	key := testSummaryKVKey
	ctxA, finishA, blocked := h.startPastMeetingChildAttempt(context.Background(), key)
	require.False(t, blocked)
	ctxB, finishB, blocked := h.startPastMeetingChildAttempt(context.Background(), key)
	require.False(t, blocked)
	require.False(t, h.beginPastMeetingChildRepair(ctxA, key, testSummaryMeetOcc))
	require.True(t, h.beginPastMeetingChildRepair(ctxB, key, testSummaryMeetOcc))
	assert.True(t, finishB(true))
	assert.False(t, finishA(false))
}

func TestPastMeetingParticipant_DeleteBeforeFirstCreateRepairsSurvivingSibling(t *testing.T) {
	for _, failRepair := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "repair failure and redelivery"}[failRepair], func(t *testing.T) {
			objects, mappings := newMemKV(), newMemKV()
			key := "itx-zoom-past-meetings-invitees.invitee-1"
			mappings.data["v1_past_meeting_invitees.invitee-1"] = []byte(buildRegistrantMappingValue("invitee-1", "alice", testSummaryMeetOcc))
			// Legacy sibling: only the pre-existing cross-reference is present;
			// do not seed the new parent-keyed tracking index.
			mappings.data["v1_participant_by_meeting_user.attendee."+testSummaryMeetOcc+".alice"] = []byte("attendee-1")
			objects.data["itx-zoom-past-meetings-attendees.attendee-1"] = mustMarshalJSON(t, minimalAttendeeV1Data("attendee-1", testSummaryMeetOcc, "alice"))
			publisher := &pastMeetingCleanupPublisher{}
			h := newRetriggerHandler(nil, objects, mappings)
			h.publisher = publisher
			publisher.onAccess = func() {
				publisher.onAccess = nil
				parent := map[string]any{"meeting_and_occurrence_id": testSummaryMeetOcc, "proj_id": "proj-sfid"}
				objects.data[testPastMeetingKey] = mustMarshalJSON(t, parent)
				require.True(t, h.handlePastMeetingUpdate(context.Background(), testPastMeetingKey, parent), "the tracked hard-delete source is still owned; preserve parent replay")
				if failRepair {
					publisher.parentErr = errors.New("publisher unavailable")
				}
			}
			assert.Equal(t, failRepair, routeDelete(context.Background(), key, nil, h))
			if failRepair {
				publisher.parentErr = nil
				require.False(t, routeDelete(context.Background(), key, nil, h))
			}
			assert.Equal(t, "participant-put:attendee-1", publisher.events[len(publisher.events)-1], "repair must restore sibling access after member_remove")
			assert.NotContains(t, mappings.data, pastMeetingChildRepairPrefix+key)
		})
	}
}

func TestPastMeetingParticipant_CompensationDiscoversLegacyParticipants(t *testing.T) {
	for _, failure := range []string{"none", "list", "read", "track"} {
		t.Run(failure, func(t *testing.T) {
			objects, mappings := newMemKV(), newMemKV()
			parent := map[string]any{"meeting_and_occurrence_id": testSummaryMeetOcc, "proj_id": "proj-sfid", "_sdc_deleted_at": "2026-01-02T00:00:00Z"}
			objects.data[testPastMeetingKey] = mustMarshalJSON(t, parent)
			legacyKey := "itx-zoom-past-meetings-attendees.legacy-attendee"
			objects.data[legacyKey] = mustMarshalJSON(t, minimalAttendeeV1Data("legacy-attendee", testSummaryMeetOcc, "bob"))
			legacyIndexKey := pastMeetingAttendeeIndexPrefix + testSummaryMeetOcc + ".legacy-attendee"
			key := "itx-zoom-past-meetings-invitees.invitee-1"
			data := minimalInviteeV1Data("invitee-1", testSummaryMeetOcc, "alice")
			objects.data[key] = mustMarshalJSON(t, data)
			switch failure {
			case "list":
				objects.listErrs["itx-zoom-past-meetings-attendees.>"] = errors.New("kv unavailable")
			case "read":
				objects.getErrs[legacyKey] = errors.New("kv unavailable")
			case "track":
				mappings.putErrs[legacyIndexKey] = errors.New("kv unavailable")
			}
			publisher := &pastMeetingCleanupPublisher{}
			h := newRetriggerHandler(nil, objects, mappings)
			h.publisher = publisher
			publisher.onAccess = func() {
				publisher.onAccess = nil
				require.Contains(t, mappings.data, legacyIndexKey, "legacy discovery must precede parent-wide revocation")
				delete(parent, "_sdc_deleted_at")
				objects.data[testPastMeetingKey] = mustMarshalJSON(t, parent)
				require.True(t, h.handlePastMeetingUpdate(context.Background(), testPastMeetingKey, parent), "the triggering source remains owned until compensation finishes")
			}
			assert.Equal(t, failure != "none", h.handlePastMeetingInviteeUpdate(context.Background(), key, data))
			if failure != "none" {
				assert.Zero(t, publisher.accessDeletes, "failed discovery must not revoke untracked legacy memberships")
				clear(objects.listErrs)
				clear(objects.getErrs)
				clear(mappings.putErrs)
				require.False(t, h.handlePastMeetingInviteeUpdate(context.Background(), key, data))
			}
			assert.Contains(t, publisher.participants, "legacy-attendee")
			assert.NotContains(t, publisher.indexerDeletes, eventing.IndexV1PastMeetingParticipantSubject+":legacy-attendee", "this source did not publish the unrelated legacy document")
			// A delayed parent delete now takes the live repair path. The
			// participant is already tracked, so no legacy scan is needed here.
			publisher.participants = nil
			require.False(t, h.handlePastMeetingDelete(context.Background(), testPastMeetingKey, nil))
			assert.Contains(t, publisher.participants, "legacy-attendee")
		})
	}
}
