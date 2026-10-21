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

func readParentOperation(t *testing.T, mappings *memKV) parentOperationRecord {
	t.Helper()
	var record parentOperationRecord
	require.NoError(t, json.Unmarshal(mappings.data[pastMeetingOperationPrefix+testSummaryMeetOcc], &record))
	return record
}

func TestParentOperation_OnlyOutermostOwnerCompletes(t *testing.T) {
	objects, mappings := newMemKV(), newMemKV()
	parent := map[string]any{"meeting_and_occurrence_id": testSummaryMeetOcc, "proj_id": "proj-sfid"}
	objects.data[testPastMeetingKey] = mustMarshalJSON(t, parent)
	mappings.data["v1_past_meetings."+testSummaryMeetOcc] = []byte(tombstoneMarker)
	mappings.data[pastMeetingCleanupKeyPrefix+testSummaryMeetOcc] = []byte("1")
	key := "itx-zoom-past-meetings-attachments.attachment-1"
	objects.data[key] = mustMarshalJSON(t, map[string]any{"id": "attachment-1", "meeting_and_occurrence_id": testSummaryMeetOcc})
	mappings.data[pastMeetingAttachmentIndexPrefix+testSummaryMeetOcc] = []byte(`["attachment-1"]`)
	mappings.data[pastMeetingChildRepairPrefix+key] = mustMarshalJSON(t, newChildRepairIntent(testSummaryMeetOcc, childRepairReady))
	mappings.deleteErrs[pastMeetingChildRepairPrefix+key] = errors.New("intent deletion unavailable")
	publisher := &pastMeetingCleanupPublisher{}
	h := newRetriggerHandler(nil, objects, mappings)
	h.publisher = publisher
	nested := false
	publisher.onAttachment = func() {
		if readChildRepairIntent(t, mappings, key).Phase == childRepairRunning {
			nested = true
			assert.Equal(t, childRepairWriting, readParentOperation(t, mappings).Phase, "nested replay cannot replace or finalize the outer parent writer")
		}
	}
	require.True(t, h.handlePastMeetingUpdate(context.Background(), testPastMeetingKey, parent))
	require.True(t, nested)
	assert.Equal(t, childRepairReady, readParentOperation(t, mappings).Phase)
	assert.Equal(t, childRepairReady, readChildRepairIntent(t, mappings, key).Phase)
	assert.NotContains(t, mappings.data, pastMeetingRestorePendingPrefix+testSummaryMeetOcc, "even if nested reconciliation completed a legacy hint, the owned obligation survives")
	clear(mappings.deleteErrs)
	publisher.onAttachment = nil
	require.False(t, h.handlePastMeetingUpdate(context.Background(), testPastMeetingKey, parent))
	assert.NotContains(t, mappings.data, pastMeetingOperationPrefix+testSummaryMeetOcc)
	assert.NotContains(t, mappings.data, pastMeetingChildRepairPrefix+key)
}

func TestParentOperation_RestoreCannotConsumePartialCleanup(t *testing.T) {
	objects, mappings := newMemKV(), newMemKV()
	parent := map[string]any{"meeting_and_occurrence_id": testSummaryMeetOcc, "proj_id": "proj-sfid"}
	deleted := map[string]any{"meeting_and_occurrence_id": testSummaryMeetOcc, "proj_id": "proj-sfid", "_sdc_deleted_at": "2026-01-02T00:00:00Z"}
	_, err := objects.Put(context.Background(), testPastMeetingKey, mustMarshalJSON(t, deleted))
	require.NoError(t, err)
	mappings.data["v1_past_meetings."+testSummaryMeetOcc] = []byte("1")
	for _, id := range []string{"attachment-1", "attachment-2"} {
		objects.data["itx-zoom-past-meetings-attachments."+id] = mustMarshalJSON(t, map[string]any{"id": id, "meeting_and_occurrence_id": testSummaryMeetOcc})
	}
	objects.data["itx-zoom-past-meetings-invitees.invitee-1"] = mustMarshalJSON(t, minimalInviteeV1Data("invitee-1", testSummaryMeetOcc, "alice"))
	publisher := &pastMeetingCleanupPublisher{}
	h, other := newRetriggerHandler(nil, objects, mappings), newRetriggerHandler(nil, objects, mappings)
	h.publisher, other.publisher = publisher, publisher
	publisher.beforeDelete = func(subject string) {
		if subject != eventing.IndexV1PastMeetingAttachmentSubject {
			return
		}
		publisher.beforeDelete = nil
		before := readParentOperation(t, mappings)
		require.Equal(t, childRepairWriting, before.Phase)
		_, err := objects.Put(context.Background(), testPastMeetingKey, mustMarshalJSON(t, parent))
		require.NoError(t, err)
		require.True(t, other.handlePastMeetingUpdate(context.Background(), testPastMeetingKey, parent))
		after := readParentOperation(t, mappings)
		assert.Equal(t, before.Owner, after.Owner)
		assert.Equal(t, childRepairWriting, after.Phase)
		assert.Greater(t, after.Requests, before.Requests)
		assert.Zero(t, publisher.parentPuts, "foreign restoration cannot publish while cleanup still writes")
		// Cleanup now deletes attachments and then fails before parent tombstone.
		publisher.failSubject = eventing.IndexV1PastMeetingParticipantSubject
	}
	require.True(t, h.handlePastMeetingDelete(context.Background(), testPastMeetingKey, nil))
	assert.Equal(t, childRepairReady, readParentOperation(t, mappings).Phase)
	assert.Equal(t, "1", string(mappings.data["v1_past_meetings."+testSummaryMeetOcc]))
	assert.NotContains(t, mappings.data, pastMeetingCleanupKeyPrefix+testSummaryMeetOcc)
	assert.Contains(t, publisher.indexerDeletes, eventing.IndexV1PastMeetingAttachmentSubject+":attachment-1")
	// Model the old hints already being absent. Stop redelivering the failed
	// delete: an ordinary parent update must discover the owned obligation.
	delete(mappings.data, pastMeetingRestorePendingPrefix+testSummaryMeetOcc)
	publisher.failSubject = ""
	require.False(t, other.handlePastMeetingUpdate(context.Background(), testPastMeetingKey, parent))
	assert.Contains(t, publisher.attachments, "attachment-1")
	assert.Contains(t, publisher.attachments, "attachment-2")
	assert.Contains(t, publisher.participants, "invitee-1")
	assert.NotContains(t, mappings.data, pastMeetingOperationPrefix+testSummaryMeetOcc)
}

func TestParentOperation_ForeignRequestRemainsDurable(t *testing.T) {
	objects, mappings := newMemKV(), newMemKV()
	objects.data[testPastMeetingKey] = mustMarshalJSON(t, map[string]any{"meeting_and_occurrence_id": testSummaryMeetOcc, "proj_id": "proj-sfid"})
	h, other := newRetriggerHandler(nil, objects, mappings), newRetriggerHandler(nil, objects, mappings)
	h.publisher, other.publisher = &pastMeetingCleanupPublisher{}, &pastMeetingCleanupPublisher{}
	foreignRan := false
	require.True(t, h.withParentOperation(context.Background(), testSummaryMeetOcc, func(ctx context.Context) bool {
		before := readParentOperation(t, mappings)
		require.True(t, other.withParentOperation(context.Background(), testSummaryMeetOcc, func(context.Context) bool { foreignRan = true; return false }))
		after := readParentOperation(t, mappings)
		assert.Equal(t, before.Owner, after.Owner)
		assert.Greater(t, after.Requests, before.Requests)
		return false
	}))
	assert.False(t, foreignRan)
	assert.Equal(t, childRepairReady, readParentOperation(t, mappings).Phase)
	require.False(t, h.handlePastMeetingUpdate(context.Background(), testPastMeetingKey, map[string]any{}))
	assert.NotContains(t, mappings.data, pastMeetingOperationPrefix+testSummaryMeetOcc)
}

func TestParentOperation_RequestRacingCompletionIsNotLost(t *testing.T) {
	objects, mappings := newMemKV(), newMemKV()
	objects.data[testPastMeetingKey] = mustMarshalJSON(t, map[string]any{"meeting_and_occurrence_id": testSummaryMeetOcc, "proj_id": "proj-sfid"})
	key := pastMeetingOperationPrefix + testSummaryMeetOcc
	mappings.data[key] = mustMarshalJSON(t, newParentOperation(testSummaryMeetOcc, childRepairWriting))
	mappings.afterGet = func(readKey string) {
		if readKey != key {
			return
		}
		mappings.afterGet = nil
		// The prior owner finishes between the requester's read and CAS.
		require.NoError(t, mappings.Delete(context.Background(), key))
	}
	h := newRetriggerHandler(nil, objects, mappings)
	h.publisher = &pastMeetingCleanupPublisher{}
	h.requestParentOperation(context.Background(), testSummaryMeetOcc)
	record := readParentOperation(t, mappings)
	assert.Equal(t, childRepairReady, record.Phase)
	assert.Equal(t, uint64(1), record.Requests)
	require.False(t, h.handlePastMeetingUpdate(context.Background(), testPastMeetingKey, map[string]any{}))
	assert.NotContains(t, mappings.data, key)
}

func TestParentOperation_FailClosedOwnerRecovery(t *testing.T) {
	for _, failure := range []string{"writer crash", "flush uncertainty"} {
		t.Run(failure, func(t *testing.T) {
			objects, mappings := newMemKV(), newMemKV()
			objects.data[testPastMeetingKey] = mustMarshalJSON(t, map[string]any{"meeting_and_occurrence_id": testSummaryMeetOcc, "proj_id": "proj-sfid"})
			publisher := &pastMeetingCleanupPublisher{}
			h, other := newRetriggerHandler(nil, objects, mappings), newRetriggerHandler(nil, objects, mappings)
			h.publisher, other.publisher = publisher, publisher
			if failure == "writer crash" {
				func() {
					defer func() { require.NotNil(t, recover()) }()
					h.withParentOperation(context.Background(), testSummaryMeetOcc, func(context.Context) bool { panic("simulated worker termination") })
				}()
			} else {
				publisher.flushErr = errors.New("flush unavailable")
				require.True(t, h.withParentOperation(context.Background(), testSummaryMeetOcc, func(context.Context) bool { return false }))
			}
			before := readParentOperation(t, mappings)
			require.Equal(t, childRepairWriting, before.Phase)
			require.True(t, other.handlePastMeetingUpdate(context.Background(), testPastMeetingKey, map[string]any{}))
			after := readParentOperation(t, mappings)
			assert.Equal(t, before.Owner, after.Owner)
			assert.Equal(t, childRepairWriting, after.Phase)
			// Operator handoff is modeled only AFTER confirmed worker/broker
			// termination; elapsed time or a failed delivery is not a lease.
			after.Phase = childRepairReady
			key := pastMeetingOperationPrefix + testSummaryMeetOcc
			_, err := mappings.Update(context.Background(), key, mustMarshalJSON(t, after), mappings.revision(key))
			require.NoError(t, err)
			publisher.flushErr = nil
			require.False(t, other.handlePastMeetingUpdate(context.Background(), testPastMeetingKey, map[string]any{}))
			assert.NotContains(t, mappings.data, key)
		})
	}
}
