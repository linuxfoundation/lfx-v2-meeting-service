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
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPastMeetingRecording_PublicationErrorStillChecksDeletedParent(t *testing.T) {
	for _, stage := range []string{"recording publish", "transcript publish", "stale transcript delete"} {
		t.Run(stage, func(t *testing.T) {
			objects, mappings := newMemKV(), newMemKV()
			parent := map[string]any{"meeting_and_occurrence_id": testSummaryMeetOcc, "proj_id": "proj-sfid"}
			objects.data[testPastMeetingKey] = mustMarshalJSON(t, parent)
			mappings.data["v1_past_meetings."+testSummaryMeetOcc] = []byte("1")
			key := "itx-zoom-past-meetings-recordings." + testSummaryMeetOcc
			data := map[string]any{"meeting_and_occurrence_id": testSummaryMeetOcc, "proj_id": "proj-sfid"}
			if stage == "transcript publish" {
				data["recording_files"] = []any{map[string]any{"file_type": "TRANSCRIPT"}}
			}
			objects.data[key] = mustMarshalJSON(t, data)
			publisher := &pastMeetingCleanupPublisher{}
			h := newRetriggerHandler(nil, objects, mappings)
			h.publisher = publisher
			publisher.beforeArtifact = func(string) {
				publisher.beforeArtifact = nil
				deleted := map[string]any{"meeting_and_occurrence_id": testSummaryMeetOcc, "proj_id": "proj-sfid", "_sdc_deleted_at": "2026-01-02T00:00:00Z"}
				_, err := objects.Put(context.Background(), testPastMeetingKey, mustMarshalJSON(t, deleted))
				require.NoError(t, err)
				require.False(t, routeDelete(context.Background(), testPastMeetingKey, nil, h))
				switch stage {
				case "recording publish":
					publisher.recordingErr = nats.ErrReconnectBufExceeded
				case "transcript publish":
					publisher.transcriptErr = nats.ErrReconnectBufExceeded
				case "stale transcript delete":
					publisher.failSubject = eventing.IndexV1PastMeetingTranscriptSubject
					publisher.deleteErr = nats.ErrReconnectBufExceeded
				}
			}
			require.True(t, h.handlePastMeetingRecordingUpdate(context.Background(), key, data), "every publication failure must retry even if not classified by message keywords")
			assert.Equal(t, childRepairReady, readChildRepairIntent(t, mappings, key).Phase)
			lastPut, lastDelete := -1, -1
			for i, event := range publisher.events {
				if event == "recording-put" {
					lastPut = i
				}
				if event == "delete:"+eventing.IndexV1PastMeetingRecordingSubject+":"+testSummaryMeetOcc {
					lastDelete = i
				}
			}
			if stage != "recording publish" {
				require.GreaterOrEqual(t, lastPut, 0)
				assert.Greater(t, lastDelete, lastPut, "partial publication must still reach parent compensation before returning")
			}
			publisher.recordingErr, publisher.transcriptErr, publisher.deleteErr = nil, nil, nil
			publisher.failSubject = ""
			require.False(t, routeDelete(context.Background(), testPastMeetingKey, nil, h))
			assert.NotContains(t, mappings.data, pastMeetingChildRepairPrefix+key)
		})
	}
}

func TestPastMeetingParticipant_HardDeleteResumesThroughLiveParentOnly(t *testing.T) {
	for _, side := range []string{"invitee", "attendee"} {
		t.Run(side, func(t *testing.T) {
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
			key, mappingKey := prefix+id, mappingPrefix+id
			_, err := objects.Put(context.Background(), key, mustMarshalJSON(t, data))
			require.NoError(t, err)
			require.NoError(t, objects.Delete(context.Background(), key))
			// A legacy rich mapping has no new parent-keyed tracking yet.
			mappings.data[mappingKey] = []byte(buildRegistrantMappingValue(id, "alice", testSummaryMeetOcc))
			publisher := &pastMeetingCleanupPublisher{accessErr: nats.ErrReconnectBufExceeded}
			h := newRetriggerHandler(nil, objects, mappings)
			h.publisher = publisher
			require.True(t, routeDelete(context.Background(), key, nil, h))
			assert.True(t, readChildRepairIntent(t, mappings, key).ParticipantDelete)
			assert.Equal(t, "1", string(mappings.data[indexPrefix+testSummaryMeetOcc+"."+id]), "parent-only repair must discover the missing source")
			assert.NotEqual(t, tombstoneMarker, string(mappings.data[mappingKey]))
			// A delayed parent delete sees the live parent. No child deletion
			// event is redelivered; this route must finish the retained remove.
			require.True(t, routeDelete(context.Background(), testPastMeetingKey, nil, h))
			assert.Equal(t, childRepairReady, readChildRepairIntent(t, mappings, key).Phase)
			assert.Contains(t, mappings.data, pastMeetingRestorePendingPrefix+testSummaryMeetOcc)
			publisher.accessErr = nil
			require.False(t, routeDelete(context.Background(), testPastMeetingKey, nil, h))
			assert.Equal(t, tombstoneMarker, string(mappings.data[mappingKey]))
			assert.NotContains(t, mappings.data, pastMeetingChildRepairPrefix+key)
			assert.NotContains(t, mappings.data, pastMeetingRestorePendingPrefix+testSummaryMeetOcc)
			assert.Contains(t, publisher.accessSubjects, fgaconstants.GenericMemberRemoveSubject)
			assert.NotContains(t, publisher.accessSubjects, fgaconstants.GenericDeleteAccessSubject, "a live parent's ordinary hard delete must not revoke all parent access")
		})
	}
}

func TestPastMeetingReplay_DrainFailureRetainsParentObligation(t *testing.T) {
	for _, kind := range []string{"attachment", "participant"} {
		for _, failure := range []string{"read", "claim"} {
			t.Run(kind+"/"+failure, func(t *testing.T) {
				objects, mappings := newMemKV(), newMemKV()
				parent := map[string]any{"meeting_and_occurrence_id": testSummaryMeetOcc, "proj_id": "proj-sfid"}
				objects.data[testPastMeetingKey] = mustMarshalJSON(t, parent)
				mappings.data["v1_past_meetings."+testSummaryMeetOcc] = []byte(tombstoneMarker)
				mappings.data[pastMeetingCleanupKeyPrefix+testSummaryMeetOcc] = []byte("1")
				key := "itx-zoom-past-meetings-attachments.attachment-1"
				data := map[string]any{"id": "attachment-1", "meeting_and_occurrence_id": testSummaryMeetOcc}
				mappings.data[pastMeetingAttachmentIndexPrefix+testSummaryMeetOcc] = []byte(`["attachment-1"]`)
				if kind == "participant" {
					key = "itx-zoom-past-meetings-invitees.invitee-1"
					data = minimalInviteeV1Data("invitee-1", testSummaryMeetOcc, "alice")
					mappings.data[pastMeetingInviteeIndexPrefix+testSummaryMeetOcc+".invitee-1"] = []byte("1")
					delete(mappings.data, pastMeetingAttachmentIndexPrefix+testSummaryMeetOcc)
				}
				objects.data[key] = mustMarshalJSON(t, data)
				markerKey := pastMeetingChildRepairPrefix + key
				mappings.data[markerKey] = mustMarshalJSON(t, newChildRepairIntent(testSummaryMeetOcc, childRepairReady))
				publisher := &pastMeetingCleanupPublisher{}
				h := newRetriggerHandler(nil, objects, mappings)
				h.publisher = publisher
				injected := false
				inject := func() {
					injected = true
					if failure == "read" {
						mappings.getErrs[markerKey] = errors.New("kv unavailable")
					} else {
						mappings.putErrs[markerKey] = errors.New("kv unavailable")
					}
				}
				if kind == "attachment" {
					publisher.onAttachment = func() { publisher.onAttachment = nil; inject() }
				} else {
					armed := false
					publisher.onParticipant = func() { publisher.onParticipant = nil; armed = true }
					mappings.afterUpdate = func(updatedKey string) {
						if updatedKey != markerKey || !armed {
							return
						}
						var intent childRepairIntent
						require.NoError(t, json.Unmarshal(mappings.data[markerKey], &intent))
						if intent.Phase == childRepairReady {
							mappings.afterUpdate = nil
							inject()
						}
					}
				}
				require.True(t, h.handlePastMeetingUpdate(context.Background(), testPastMeetingKey, parent))
				require.True(t, injected)
				assert.Contains(t, mappings.data, markerKey)
				assert.Contains(t, mappings.data, pastMeetingRestorePendingPrefix+testSummaryMeetOcc, "drain failure must leave participant/attachment replay discoverable on the next parent delivery")
				assert.Contains(t, mappings.data, pastMeetingCleanupKeyPrefix+testSummaryMeetOcc)
				clear(mappings.getErrs)
				clear(mappings.putErrs)
				require.False(t, h.handlePastMeetingUpdate(context.Background(), testPastMeetingKey, parent))
				assert.NotContains(t, mappings.data, markerKey)
				assert.NotContains(t, mappings.data, pastMeetingRestorePendingPrefix+testSummaryMeetOcc)
				assert.NotContains(t, mappings.data, pastMeetingCleanupKeyPrefix+testSummaryMeetOcc)
				if kind == "attachment" {
					assert.GreaterOrEqual(t, len(publisher.attachments), 2)
				} else {
					assert.GreaterOrEqual(t, len(publisher.participants), 2)
				}
			})
		}
	}
}

func TestPastMeetingReplay_OlderCompletionCannotClearNewerMarkers(t *testing.T) {
	for _, newer := range []string{"pending", "cleanup"} {
		t.Run(newer, func(t *testing.T) {
			objects, mappings := newMemKV(), newMemKV()
			pendingKey, cleanupKey := pastMeetingRestorePendingPrefix+testSummaryMeetOcc, pastMeetingCleanupKeyPrefix+testSummaryMeetOcc
			pending, err := mappings.Put(context.Background(), pendingKey, []byte("1"))
			require.NoError(t, err)
			completion := mustMarshalJSON(t, pastMeetingCleanupCompletion{Complete: true, SourceRevision: 1})
			cleanup, err := mappings.Put(context.Background(), cleanupKey, completion)
			require.NoError(t, err)
			replay := &pastMeetingReplayState{parentID: testSummaryMeetOcc, pendingRevision: pending, cleanupRevision: cleanup}
			newKey, value := pendingKey, []byte("1")
			if newer == "cleanup" {
				newKey, value = cleanupKey, completion
			}
			revision, err := mappings.Put(context.Background(), newKey, value)
			require.NoError(t, err)
			h := newRetriggerHandler(nil, objects, mappings)
			require.True(t, h.finalizePastMeetingReplayMarkers(context.Background(), replay))
			assert.Equal(t, revision, mappings.revision(newKey))
			assert.Equal(t, value, mappings.data[newKey], "an older success cannot discard a newer durable replay/deletion obligation")
		})
	}
}
