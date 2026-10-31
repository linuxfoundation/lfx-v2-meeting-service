// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPastMeetingParticipant_SoftDeleteRetryCompletesOwnedSource(t *testing.T) {
	for _, side := range []string{"invitee", "attendee"} {
		for _, failure := range []string{"xref", "mapping"} {
			for _, parentChanged := range []bool{false, true} {
				t.Run(side+"/"+failure+map[bool]string{false: "/unchanged parent", true: "/changed parent requires replay"}[parentChanged], func(t *testing.T) {
					objects, mappings := newMemKV(), newMemKV()
					parent := map[string]any{"meeting_and_occurrence_id": testSummaryMeetOcc, "proj_id": "proj-sfid"}
					objects.data[testPastMeetingKey] = mustMarshalJSON(t, parent)
					mappings.data["v1_past_meetings."+testSummaryMeetOcc] = []byte("1")
					id, prefix, mappingPrefix, indexPrefix := "invitee-1", "itx-zoom-past-meetings-invitees.", "v1_past_meeting_invitees.", pastMeetingInviteeIndexPrefix
					data := minimalInviteeV1Data(id, testSummaryMeetOcc, "alice")
					if side == "attendee" {
						id, prefix, mappingPrefix, indexPrefix = "attendee-1", "itx-zoom-past-meetings-attendees.", "v1_past_meeting_attendees.", pastMeetingAttendeeIndexPrefix
						data = minimalAttendeeV1Data(id, testSummaryMeetOcc, "alice")
					}
					data["_sdc_deleted_at"] = "2026-01-02T00:00:00Z"
					key, mappingKey := prefix+id, mappingPrefix+id
					objects.data[key] = mustMarshalJSON(t, data)
					mappings.data[indexPrefix+testSummaryMeetOcc+"."+id] = []byte("1")
					mappings.data[mappingKey] = []byte(buildRegistrantMappingValue(id, "alice", testSummaryMeetOcc))
					xref := "v1_participant_by_meeting_user." + side + "." + testSummaryMeetOcc + ".alice"
					mappings.data[xref] = []byte(id)
					failureKey := xref
					if failure == "mapping" {
						failureKey = mappingKey
					}
					mappings.putErrs[failureKey] = errors.New("kv unavailable")
					publisher := &pastMeetingCleanupPublisher{}
					h := newRetriggerHandler(nil, objects, mappings)
					h.publisher = publisher
					require.True(t, routeDelete(context.Background(), key, data, h))
					require.Equal(t, childRepairReady, readChildRepairIntent(t, mappings, key).Phase)
					assert.NotEqual(t, tombstoneMarker, string(mappings.data[mappingKey]))
					clear(mappings.putErrs)
					if parentChanged {
						// A real KV rewrite advances the authoritative revision even
						// when the current live payload has the same fields.
						_, err := objects.Put(context.Background(), testPastMeetingKey, mustMarshalJSON(t, parent))
						require.NoError(t, err)
						otherWorker := newRetriggerHandler(nil, objects, mappings)
						otherWorker.publisher = &pastMeetingCleanupPublisher{}
						publisher.beforeAccess = func(string) {
							publisher.beforeAccess = nil
							require.Equal(t, childRepairRunning, readChildRepairIntent(t, mappings, key).Phase)
							require.True(t, routeDelete(context.Background(), key, data, otherWorker), "only the owning replay may borrow its repairing intent")
						}
					}
					// The second delivery must finish within the default retry
					// budget, including when parent replay encounters this same
					// retained soft-delete source under its repairing marker.
					require.False(t, routeDelete(context.Background(), key, data, h))
					assert.Equal(t, tombstoneMarker, string(mappings.data[mappingKey]))
					assert.NotContains(t, mappings.data, pastMeetingChildRepairPrefix+key)
					if parentChanged {
						assert.Positive(t, publisher.parentPuts)
					} else {
						assert.Zero(t, publisher.parentPuts)
					}
				})
			}
		}
	}
}

func TestPastMeetingParticipant_OrdinaryBulkRemovalDoesNotReplayWholeParent(t *testing.T) {
	for _, count := range []int{10, 40} {
		t.Run(fmt.Sprintf("%d removals", count), func(t *testing.T) {
			objects, mappings := newMemKV(), newMemKV()
			objects.data[testPastMeetingKey] = mustMarshalJSON(t, map[string]any{"meeting_and_occurrence_id": testSummaryMeetOcc, "proj_id": "proj-sfid"})
			mappings.data["v1_past_meetings."+testSummaryMeetOcc] = []byte("1")
			objects.data["itx-zoom-past-meetings-recordings."+testSummaryMeetOcc] = mustMarshalJSON(t, map[string]any{"meeting_and_occurrence_id": testSummaryMeetOcc, "proj_id": "proj-sfid"})
			objects.data[testSummaryKVKey] = mustMarshalJSON(t, map[string]any{"id": testSummaryID, "meeting_and_occurrence_id": testSummaryMeetOcc})
			mappings.data[testSummaryIndexKey] = []byte(`["` + testSummaryID + `"]`)
			for i := range count {
				id := fmt.Sprintf("survivor-%d", i)
				objects.data["itx-zoom-past-meetings-attendees."+id] = mustMarshalJSON(t, minimalAttendeeV1Data(id, testSummaryMeetOcc, fmt.Sprintf("test-user-%d", i)))
				mappings.data[pastMeetingAttendeeIndexPrefix+testSummaryMeetOcc+"."+id] = []byte("1")
			}
			publisher := &pastMeetingCleanupPublisher{}
			h := newRetriggerHandler(nil, objects, mappings)
			h.publisher = publisher
			for i := range count {
				id := fmt.Sprintf("invitee-%d", i)
				key := "itx-zoom-past-meetings-invitees." + id
				username := fmt.Sprintf("removed-user-%d", i)
				data := minimalInviteeV1Data(id, testSummaryMeetOcc, username)
				data["_sdc_deleted_at"] = "2026-01-02T00:00:00Z"
				objects.data[key] = mustMarshalJSON(t, data)
				mappings.data[pastMeetingInviteeIndexPrefix+testSummaryMeetOcc+"."+id] = []byte("1")
				mappings.data["v1_past_meeting_invitees."+id] = []byte(buildRegistrantMappingValue(id, username, testSummaryMeetOcc))
				require.False(t, routeDelete(context.Background(), key, data, h))
				assert.NotContains(t, mappings.data, pastMeetingChildRepairPrefix+key)
			}
			assert.Zero(t, publisher.parentPuts)
			assert.Empty(t, publisher.participants)
			assert.Empty(t, publisher.attachments)
			assert.Zero(t, publisher.summaries)
			assert.Nil(t, publisher.recordingData)
			assert.Equal(t, count, publisher.accessDeletes)
			assert.Len(t, publisher.indexerDeletes, count)
			assert.LessOrEqual(t, len(objects.gets), 4*count, "ordinary deletion must use bounded per-record reads, not scan/replay N children per removal")
		})
	}
}
