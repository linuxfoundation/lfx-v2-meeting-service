// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/infrastructure/eventing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPastMeetingParticipant_RestoreWithoutSourceRewriteRepairsSibling(t *testing.T) {
	for _, timing := range []string{"before ownership claim", "after second sibling lookup"} {
		t.Run(timing, func(t *testing.T) {
			objects, mappings := newMemKV(), newMemKV()
			parent := map[string]any{"meeting_and_occurrence_id": testSummaryMeetOcc, "proj_id": "proj-sfid"}
			objects.data[testPastMeetingKey] = mustMarshalJSON(t, parent)
			initialSourceRevision := objects.revision(testPastMeetingKey)
			mappings.data["v1_past_meetings."+testSummaryMeetOcc] = []byte(tombstoneMarker)
			mappings.data[pastMeetingCleanupKeyPrefix+testSummaryMeetOcc] = []byte("1")
			key := "itx-zoom-past-meetings-invitees.invitee-1"
			mappings.data["v1_past_meeting_invitees.invitee-1"] = []byte(buildRegistrantMappingValue("invitee-1", "alice", testSummaryMeetOcc))
			objects.data["itx-zoom-past-meetings-attendees.attendee-1"] = mustMarshalJSON(t, minimalAttendeeV1Data("attendee-1", testSummaryMeetOcc, "alice"))
			publisher := &pastMeetingCleanupPublisher{}
			h := newRetriggerHandler(nil, objects, mappings)
			h.publisher = publisher
			lookups := 0
			mappings.afterGet = func(readKey string) {
				if readKey != "v1_participant_by_meeting_user.attendee."+testSummaryMeetOcc+".alice" {
					return
				}
				lookups++
				if timing == "after second sibling lookup" && lookups != 2 {
					return
				}
				mappings.afterGet = nil
				// Discovery/restoration happens after a lookup returned absent,
				// but does not rewrite the already-live parent source record.
				mappings.data[pastMeetingAttendeeIndexPrefix+testSummaryMeetOcc+".attendee-1"] = []byte("1")
				require.True(t, h.handlePastMeetingUpdate(context.Background(), testPastMeetingKey, parent), "the source removal owns a tracked replay obligation")
			}
			require.False(t, routeDelete(context.Background(), key, nil, h))
			assert.Equal(t, initialSourceRevision, objects.revision(testPastMeetingKey))
			require.GreaterOrEqual(t, len(publisher.participants), 2, "restoration grants the sibling, then final repair restores the grant removed afterward")
			assert.Equal(t, "participant-put:attendee-1", publisher.events[len(publisher.events)-1])
			assert.NotContains(t, mappings.data, pastMeetingChildRepairPrefix+key)
		})
	}
}

func TestPastMeetingReplay_RepairsLiveChildAfterLateRetraction(t *testing.T) {
	for _, kind := range []string{"recording", "summary"} {
		for _, failRepair := range []bool{false, true} {
			t.Run(kind+map[bool]string{false: "/success", true: "/repair retry"}[failRepair], func(t *testing.T) {
				objects, mappings := newMemKV(), newMemKV()
				parent := map[string]any{"meeting_and_occurrence_id": testSummaryMeetOcc, "proj_id": "proj-sfid"}
				objects.data[testPastMeetingKey] = mustMarshalJSON(t, parent)
				mappings.data["v1_past_meetings."+testSummaryMeetOcc] = []byte("1")
				key := "itx-zoom-past-meetings-recordings." + testSummaryMeetOcc
				data := map[string]any{"meeting_and_occurrence_id": testSummaryMeetOcc, "proj_id": "proj-sfid"}
				subject, putEvent := eventing.IndexV1PastMeetingRecordingSubject, "recording-put"
				if kind == "summary" {
					key, subject, putEvent = testSummaryKVKey, eventing.IndexV1PastMeetingSummarySubject, "summary-put"
					data["id"] = testSummaryID
					mappings.data[testSummaryIndexKey] = []byte(`["` + testSummaryID + `"]`)
				}
				objects.data[key] = mustMarshalJSON(t, data)
				publisher := &pastMeetingCleanupPublisher{}
				h, other := newRetriggerHandler(nil, objects, mappings), newRetriggerHandler(nil, objects, mappings)
				h.publisher, other.publisher = publisher, publisher
				deleteParent := func() {
					publisher.onRecording, publisher.onSummary = nil, nil
					deleted := map[string]any{"meeting_and_occurrence_id": testSummaryMeetOcc, "proj_id": "proj-sfid", "_sdc_deleted_at": "2026-01-02T00:00:00Z"}
					_, err := objects.Put(context.Background(), testPastMeetingKey, mustMarshalJSON(t, deleted))
					require.NoError(t, err)
				}
				publisher.onRecording, publisher.onSummary = deleteParent, deleteParent
				publisher.beforeDelete = func(deleteSubject string) {
					if deleteSubject != subject {
						return
					}
					publisher.beforeDelete = nil
					_, err := objects.Put(context.Background(), testPastMeetingKey, mustMarshalJSON(t, parent))
					require.NoError(t, err)
					require.True(t, other.handlePastMeetingUpdate(context.Background(), testPastMeetingKey, parent), "foreign owner must leave this restoration retryable")
					// The in-flight child delete lands AFTER this parent restore.
					if failRepair {
						publisher.parentErr = errors.New("publisher unavailable")
					}
				}
				require.True(t, h.handlePastMeetingUpdate(context.Background(), testPastMeetingKey, parent), "the source generation/queued foreign request must keep parent ownership repairable")
				if failRepair {
					require.Equal(t, childRepairReady, readChildRepairIntent(t, mappings, key).Phase)
				}
				publisher.parentErr = nil
				require.False(t, h.handlePastMeetingUpdate(context.Background(), testPastMeetingKey, parent))
				lastDelete, lastPut := -1, -1
				for i, event := range publisher.events {
					if strings.HasPrefix(event, "delete:"+subject+":") {
						lastDelete = i
					}
					if event == putEvent {
						lastPut = i
					}
				}
				require.GreaterOrEqual(t, lastDelete, 0)
				assert.Greater(t, lastPut, lastDelete, "actual reconciliation must follow the last destructive write")
				assert.NotContains(t, mappings.data, pastMeetingChildRepairPrefix+key)
				assert.NotContains(t, mappings.data, pastMeetingOperationPrefix+testSummaryMeetOcc)
			})
		}
	}
}

func TestPastMeetingParticipant_DeletedBulkCleanupIsBounded(t *testing.T) {
	for _, count := range []int{10, 40} {
		t.Run(fmt.Sprintf("%d late updates and deletes", count), func(t *testing.T) {
			objects, mappings := newMemKV(), newMemKV()
			deleted := map[string]any{"meeting_and_occurrence_id": testSummaryMeetOcc, "proj_id": "proj-sfid", "_sdc_deleted_at": "2026-01-02T00:00:00Z"}
			objects.data[testPastMeetingKey] = mustMarshalJSON(t, deleted)
			for i := range count {
				id := fmt.Sprintf("invitee-%d", i)
				objects.data["itx-zoom-past-meetings-invitees."+id] = mustMarshalJSON(t, minimalInviteeV1Data(id, testSummaryMeetOcc, "alice"))
			}
			for i := range 60 {
				id := fmt.Sprintf("historical-%d", i)
				objects.data["itx-zoom-past-meetings-attendees."+id] = mustMarshalJSON(t, minimalAttendeeV1Data(id, "other-parent", "bob"))
			}
			publisher := &pastMeetingCleanupPublisher{}
			h := newRetriggerHandler(nil, objects, mappings)
			h.publisher = publisher
			require.False(t, h.handlePastMeetingDelete(context.Background(), testPastMeetingKey, nil))
			objects.gets, objects.lists, publisher.indexerDeletes = nil, nil, nil
			for i := range count {
				id := fmt.Sprintf("invitee-%d", i)
				key := "itx-zoom-past-meetings-invitees." + id
				data := minimalInviteeV1Data(id, testSummaryMeetOcc, "alice")
				require.False(t, h.handlePastMeetingInviteeUpdate(context.Background(), key, data))
				data["_sdc_deleted_at"] = "2026-01-02T00:00:00Z"
				require.False(t, routeDelete(context.Background(), key, data, h))
			}
			assert.Empty(t, objects.lists, "completed discovery must not scan either global participant table for each late child")
			assert.LessOrEqual(t, len(publisher.indexerDeletes), 3*count, "delete only the current source's affected documents, not every sibling N times")
			for _, readKey := range objects.gets {
				assert.NotContains(t, readKey, "historical-", "late cleanup must not read unrelated historical participants")
			}
		})
	}
}

func TestPastMeetingParticipant_DiscoveryGenerationAndClaim(t *testing.T) {
	objects, mappings := newMemKV(), newMemKV()
	parent := map[string]any{"meeting_and_occurrence_id": testSummaryMeetOcc, "proj_id": "proj-sfid"}
	_, err := objects.Put(context.Background(), testPastMeetingKey, mustMarshalJSON(t, parent))
	require.NoError(t, err)
	require.NoError(t, objects.Delete(context.Background(), testPastMeetingKey))
	h, other := newRetriggerHandler(nil, objects, mappings), newRetriggerHandler(nil, objects, mappings)
	h.publisher, other.publisher = &pastMeetingCleanupPublisher{}, &pastMeetingCleanupPublisher{}
	mappings.onCreate = func(key string) {
		if key == pastMeetingParticipantDiscoveryPrefix+testSummaryMeetOcc {
			require.True(t, other.ensurePastMeetingParticipantsDiscovered(context.Background(), testSummaryMeetOcc), "another worker cannot duplicate an active discovery scan")
		}
	}
	require.False(t, h.ensurePastMeetingParticipantsDiscovered(context.Background(), testSummaryMeetOcc))
	assert.Len(t, objects.lists, 2)
	require.False(t, other.ensurePastMeetingParticipantsDiscovered(context.Background(), testSummaryMeetOcc))
	assert.Len(t, objects.lists, 2)
	// A restore and hard delete can occur before their events are processed.
	// Retained source history still identifies the newer deletion generation.
	_, err = objects.Put(context.Background(), testPastMeetingKey, mustMarshalJSON(t, parent))
	require.NoError(t, err)
	require.NoError(t, objects.Delete(context.Background(), testPastMeetingKey))
	require.False(t, h.ensurePastMeetingParticipantsDiscovered(context.Background(), testSummaryMeetOcc))
	assert.Len(t, objects.lists, 4)
}
