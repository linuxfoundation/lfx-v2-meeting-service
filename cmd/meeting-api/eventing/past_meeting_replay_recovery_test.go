// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"errors"
	"testing"

	fgaconstants "github.com/linuxfoundation/lfx-v2-fga-sync/pkg/constants"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/infrastructure/eventing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPastMeetingParticipant_LegacySiblingTrackingFailurePreventsRemoval(t *testing.T) {
	for _, failure := range []string{"lookup", "tracking"} {
		t.Run(failure, func(t *testing.T) {
			objects, mappings := newMemKV(), newMemKV()
			key := "itx-zoom-past-meetings-invitees.invitee-1"
			xref := "v1_participant_by_meeting_user.attendee." + testSummaryMeetOcc + ".alice"
			index := pastMeetingAttendeeIndexPrefix + testSummaryMeetOcc + ".attendee-1"
			mappings.data["v1_past_meeting_invitees.invitee-1"] = []byte(buildRegistrantMappingValue("invitee-1", "alice", testSummaryMeetOcc))
			mappings.data[xref] = []byte("attendee-1")
			objects.data["itx-zoom-past-meetings-attendees.attendee-1"] = mustMarshalJSON(t, minimalAttendeeV1Data("attendee-1", testSummaryMeetOcc, "alice"))
			if failure == "lookup" {
				mappings.getErrs[xref] = errors.New("kv unavailable")
			} else {
				mappings.putErrs[index] = errors.New("kv unavailable")
			}
			publisher := &pastMeetingCleanupPublisher{}
			h := newRetriggerHandler(nil, objects, mappings)
			h.publisher = publisher
			require.True(t, routeDelete(context.Background(), key, nil, h))
			assert.Zero(t, publisher.accessDeletes, "lookup/tracking must succeed before per-user removal")
			clear(mappings.getErrs)
			clear(mappings.putErrs)
			require.False(t, routeDelete(context.Background(), key, nil, h))
			assert.Equal(t, "1", string(mappings.data[index]))
			parent := map[string]any{"meeting_and_occurrence_id": testSummaryMeetOcc, "proj_id": "proj-sfid"}
			objects.data[testPastMeetingKey] = mustMarshalJSON(t, parent)
			require.False(t, h.handlePastMeetingUpdate(context.Background(), testPastMeetingKey, parent))
			assert.Contains(t, publisher.participants, "attendee-1", "first create must discover the legacy sibling")
		})
	}
}

func TestPastMeetingParticipant_LiveFullDeleteRacingRestorePreservesLegacySibling(t *testing.T) {
	objects, mappings := newMemKV(), newMemKV()
	parent := map[string]any{"meeting_and_occurrence_id": testSummaryMeetOcc, "proj_id": "proj-sfid"}
	objects.data[testPastMeetingKey] = mustMarshalJSON(t, parent)
	mappings.data["v1_past_meetings."+testSummaryMeetOcc] = []byte("1")
	key := "itx-zoom-past-meetings-invitees.invitee-1"
	mappings.data["v1_past_meeting_invitees.invitee-1"] = []byte(buildRegistrantMappingValue("invitee-1", "alice", testSummaryMeetOcc))
	// Neither xref nor parent-keyed tracking exists yet for this legacy sibling.
	objects.data["itx-zoom-past-meetings-attendees.attendee-1"] = mustMarshalJSON(t, minimalAttendeeV1Data("attendee-1", testSummaryMeetOcc, "alice"))
	publisher := &pastMeetingCleanupPublisher{}
	h := newRetriggerHandler(nil, objects, mappings)
	h.publisher = publisher
	publisher.beforeAccess = func(subject string) {
		if subject != fgaconstants.GenericMemberRemoveSubject {
			return
		}
		publisher.beforeAccess = nil
		require.Equal(t, childRepairWriting, readChildRepairIntent(t, mappings, key).Phase)
		deleted := map[string]any{"meeting_and_occurrence_id": testSummaryMeetOcc, "proj_id": "proj-sfid", "_sdc_deleted_at": "2026-01-02T00:00:00Z"}
		_, err := objects.Put(context.Background(), testPastMeetingKey, mustMarshalJSON(t, deleted))
		require.NoError(t, err)
		require.False(t, h.handlePastMeetingDelete(context.Background(), testPastMeetingKey, nil))
		_, err = objects.Put(context.Background(), testPastMeetingKey, mustMarshalJSON(t, parent))
		require.NoError(t, err)
		require.True(t, h.handlePastMeetingUpdate(context.Background(), testPastMeetingKey, parent), "the hard-delete source is tracked and still owned during restoration")
		// Only now does the original, formerly live-parent member_remove land.
	}

	require.False(t, routeDelete(context.Background(), key, nil, h))
	assert.Contains(t, publisher.accessSubjects, fgaconstants.GenericMemberRemoveSubject)
	assert.Equal(t, "participant-put:attendee-1", publisher.events[len(publisher.events)-1])
	assert.NotContains(t, mappings.data, pastMeetingChildRepairPrefix+key)
}

func TestPastMeetingReplay_ParentDeliveryDrainsChildRepair(t *testing.T) {
	for _, kind := range []string{"recording", "summary"} {
		for _, failCompletion := range []bool{false, true} {
			t.Run(kind+map[bool]string{false: "/success", true: "/parent-only redelivery"}[failCompletion], func(t *testing.T) {
				objects, mappings := newMemKV(), newMemKV()
				parent := map[string]any{"meeting_and_occurrence_id": testSummaryMeetOcc, "proj_id": "proj-sfid"}
				objects.data[testPastMeetingKey] = mustMarshalJSON(t, parent)
				mappings.data["v1_past_meetings."+testSummaryMeetOcc] = []byte("1")
				childKey := "itx-zoom-past-meetings-recordings." + testSummaryMeetOcc
				child := map[string]any{"meeting_and_occurrence_id": testSummaryMeetOcc, "proj_id": "proj-sfid"}
				if kind == "summary" {
					childKey = testSummaryKVKey
					child["id"] = testSummaryID
					mappings.data[testSummaryIndexKey] = []byte(`["` + testSummaryID + `"]`)
				}
				objects.data[childKey] = mustMarshalJSON(t, child)
				markerKey := pastMeetingChildRepairPrefix + childKey
				publisher := &pastMeetingCleanupPublisher{}
				h := newRetriggerHandler(nil, objects, mappings)
				h.publisher = publisher
				deleteParent := func() {
					publisher.onRecording, publisher.onSummary = nil, nil
					deleted := map[string]any{"meeting_and_occurrence_id": testSummaryMeetOcc, "proj_id": "proj-sfid", "_sdc_deleted_at": "2026-01-02T00:00:00Z"}
					objects.data[testPastMeetingKey] = mustMarshalJSON(t, deleted)
					require.True(t, h.handlePastMeetingDelete(context.Background(), testPastMeetingKey, nil), "foreign parent deletion must retain a request instead of racing the owner")
					if failCompletion {
						mappings.deleteErrs[markerKey] = errors.New("kv unavailable")
					}
				}
				publisher.onRecording = deleteParent
				publisher.onSummary = deleteParent

				require.True(t, h.handlePastMeetingUpdate(context.Background(), testPastMeetingKey, parent), "the new parent-operation request remains durable after this invocation")
				assert.Equal(t, tombstoneMarker, string(mappings.data["v1_past_meetings."+testSummaryMeetOcc]), "failed replay must not leave its stale parent live")
				if failCompletion {
					require.Equal(t, childRepairReady, readChildRepairIntent(t, mappings, childKey).Phase)
				}
				clear(mappings.deleteErrs)
				// Only the parent event is redelivered, with its original live
				// payload. There is no separate child delivery to rescue it.
				require.False(t, h.handlePastMeetingUpdate(context.Background(), testPastMeetingKey, parent))
				assert.NotContains(t, mappings.data, markerKey)
				assert.NotContains(t, mappings.data, pastMeetingOperationPrefix+testSummaryMeetOcc)
				assert.Equal(t, tombstoneMarker, string(mappings.data["v1_past_meetings."+testSummaryMeetOcc]))
				assert.Contains(t, publisher.indexerDeletes, eventing.IndexV1PastMeetingSubject+":"+testSummaryMeetOcc)
			})
		}
	}
}
