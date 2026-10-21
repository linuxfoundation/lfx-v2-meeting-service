// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	fgaconstants "github.com/linuxfoundation/lfx-v2-fga-sync/pkg/constants"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/infrastructure/eventing"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPastMeetingChildRepair_RestoreDuringRetraction(t *testing.T) {
	for _, tc := range []struct {
		name, key, subject, putEvent string
		data                         map[string]any
	}{
		{"recording", "itx-zoom-past-meetings-recordings." + testSummaryMeetOcc, eventing.IndexV1PastMeetingRecordingSubject, "recording-put", map[string]any{
			"meeting_and_occurrence_id": testSummaryMeetOcc, "proj_id": "proj-sfid",
			"recording_files": []any{map[string]any{"file_type": "TRANSCRIPT"}},
		}},
		{"between recording and transcript", "itx-zoom-past-meetings-recordings." + testSummaryMeetOcc, eventing.IndexV1PastMeetingTranscriptSubject, "transcript-put", map[string]any{
			"meeting_and_occurrence_id": testSummaryMeetOcc, "proj_id": "proj-sfid",
			"recording_files": []any{map[string]any{"file_type": "TRANSCRIPT"}},
		}},
		{"summary", testSummaryKVKey, eventing.IndexV1PastMeetingSummarySubject, "summary-put", map[string]any{
			"id": testSummaryID, "meeting_and_occurrence_id": testSummaryMeetOcc,
		}},
		{"attachment", "itx-zoom-past-meetings-attachments.attachment-1", eventing.IndexV1PastMeetingAttachmentSubject, "attachment-put", map[string]any{
			"id": "attachment-1", "meeting_and_occurrence_id": testSummaryMeetOcc,
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			objects, mappings := newMemKV(), newMemKV()
			parent := map[string]any{"meeting_and_occurrence_id": testSummaryMeetOcc, "proj_id": "proj-sfid", "_sdc_deleted_at": "2026-01-02T00:00:00Z"}
			objects.data[testPastMeetingKey] = mustMarshalJSON(t, parent)
			objects.data[tc.key] = mustMarshalJSON(t, tc.data)
			mappings.data["v1_past_meetings."+testSummaryMeetOcc] = []byte(tombstoneMarker)
			publisher := &pastMeetingCleanupPublisher{}
			h := newRetriggerHandler(nil, objects, mappings)
			h.publisher = publisher
			publisher.beforeDelete = func(subject string) {
				if subject != tc.subject {
					return
				}
				publisher.beforeDelete = nil
				delete(parent, "_sdc_deleted_at")
				objects.data[testPastMeetingKey] = mustMarshalJSON(t, parent)
				// A separate restore must retain its obligation while this
				// source's destructive publication remains owned and in flight.
				require.True(t, h.handlePastMeetingUpdate(context.Background(), testPastMeetingKey, parent))
				require.Contains(t, mappings.data, pastMeetingRestorePendingPrefix+testSummaryMeetOcc)
				require.Contains(t, mappings.data, pastMeetingChildRepairPrefix+tc.key, "the parent's restore cannot consume the child's repair intent")
			}

			require.False(t, handleKVPut(context.Background(), tc.key, tc.data, h))
			lastDelete, lastPut := -1, -1
			for i, event := range publisher.events {
				if event == tc.putEvent {
					lastPut = i
				}
			}
			// Check the subject rather than the ID: recording/transcript share the
			// parent ID while summaries and attachments have independent IDs.
			for i, event := range publisher.events {
				if strings.HasPrefix(event, "delete:"+tc.subject+":") {
					lastDelete = i
				}
			}
			require.GreaterOrEqual(t, lastDelete, 0)
			assert.Greater(t, lastPut, lastDelete, "repair must be the last writer: %v", publisher.events)
			assert.NotContains(t, mappings.data, pastMeetingChildRepairPrefix+tc.key)
		})
	}
}

func TestPastMeetingChildRepair_ParticipantFailuresResumeWholeParent(t *testing.T) {
	for _, failure := range []string{"intent write", "parent replay marker", "parent publish", "child replay"} {
		t.Run(failure, func(t *testing.T) {
			objects, mappings := newMemKV(), newMemKV()
			parent := map[string]any{"meeting_and_occurrence_id": testSummaryMeetOcc, "proj_id": "proj-sfid", "_sdc_deleted_at": "2026-01-02T00:00:00Z"}
			objects.data[testPastMeetingKey] = mustMarshalJSON(t, parent)
			participant := minimalInviteeV1Data("invitee-1", testSummaryMeetOcc, "alice")
			key := "itx-zoom-past-meetings-invitees.invitee-1"
			objects.data[key] = mustMarshalJSON(t, participant)
			attachmentKey := "itx-zoom-past-meetings-attachments.attachment-1"
			objects.data[attachmentKey] = mustMarshalJSON(t, map[string]any{"id": "attachment-1", "meeting_and_occurrence_id": testSummaryMeetOcc})
			mappings.data[pastMeetingAttachmentIndexPrefix+testSummaryMeetOcc] = []byte(`["attachment-1"]`)
			mappings.data[pastMeetingAttendeeIndexPrefix+testSummaryMeetOcc+".attendee-1"] = []byte("1")
			objects.data["itx-zoom-past-meetings-attendees.attendee-1"] = mustMarshalJSON(t, minimalAttendeeV1Data("attendee-1", testSummaryMeetOcc, "bob"))
			publisher := &pastMeetingCleanupPublisher{}
			h := newRetriggerHandler(nil, objects, mappings)
			h.publisher = publisher
			if failure == "intent write" {
				mappings.putErrs[pastMeetingChildRepairPrefix+key] = errors.New("kv unavailable")
			} else {
				publisher.onAccess = func() {
					publisher.onAccess = nil
					delete(parent, "_sdc_deleted_at")
					objects.data[testPastMeetingKey] = mustMarshalJSON(t, parent)
					require.True(t, h.handlePastMeetingUpdate(context.Background(), testPastMeetingKey, parent), "foreign owner must keep restoration retryable")
					switch failure {
					case "parent replay marker":
						mappings.putErrs[pastMeetingRestorePendingPrefix+testSummaryMeetOcc] = errors.New("kv unavailable")
					case "parent publish":
						publisher.parentErr = errors.New("publisher unavailable")
					case "child replay":
						objects.getErrs[attachmentKey] = errors.New("kv unavailable")
					}
				}
			}

			require.True(t, h.handlePastMeetingInviteeUpdate(context.Background(), key, participant))
			if failure == "intent write" {
				assert.Empty(t, publisher.indexerDeletes, "no destructive write may precede intent")
				assert.Zero(t, publisher.accessDeletes)
			} else {
				assert.Contains(t, mappings.data, pastMeetingChildRepairPrefix+key)
				// Simulate losing the restored parent access and unrelated child
				// in the compensation; redelivery must replay both, not just self.
				publisher.parentPuts = 0
				publisher.attachments = nil
				publisher.participants = nil
			}
			clear(mappings.putErrs)
			clear(objects.getErrs)
			publisher.parentErr = nil
			require.False(t, h.handlePastMeetingInviteeUpdate(context.Background(), key, participant))
			if failure != "intent write" {
				assert.Positive(t, publisher.parentPuts)
				assert.Contains(t, publisher.attachments, "attachment-1")
				assert.Contains(t, publisher.participants, "attendee-1")
			}
			assert.NotContains(t, mappings.data, pastMeetingChildRepairPrefix+key)
		})
	}
}

func TestPastMeetingChildRepair_ArtifactRepairFailureRetries(t *testing.T) {
	for _, kind := range []string{"recording", "summary", "attachment"} {
		t.Run(kind, func(t *testing.T) {
			objects, mappings := newMemKV(), newMemKV()
			parent := map[string]any{"meeting_and_occurrence_id": testSummaryMeetOcc, "proj_id": "proj-sfid", "_sdc_deleted_at": "2026-01-02T00:00:00Z"}
			objects.data[testPastMeetingKey] = mustMarshalJSON(t, parent)
			key := "itx-zoom-past-meetings-recordings." + testSummaryMeetOcc
			data := map[string]any{"meeting_and_occurrence_id": testSummaryMeetOcc, "proj_id": "proj-sfid"}
			switch kind {
			case "summary":
				key = testSummaryKVKey
				data["id"] = testSummaryID
			case "attachment":
				key = "itx-zoom-past-meetings-attachments.attachment-1"
				data["id"] = "attachment-1"
			}
			objects.data[key] = mustMarshalJSON(t, data)
			publisher := &pastMeetingCleanupPublisher{}
			h := newRetriggerHandler(nil, objects, mappings)
			h.publisher = publisher
			publisher.beforeDelete = func(string) {
				publisher.beforeDelete = nil
				delete(parent, "_sdc_deleted_at")
				objects.data[testPastMeetingKey] = mustMarshalJSON(t, parent)
				require.True(t, h.handlePastMeetingUpdate(context.Background(), testPastMeetingKey, parent), "restore cannot acknowledge a child blocked by another owner")
				publisher.parentErr = errors.New("publisher unavailable")
			}

			require.True(t, handleKVPut(context.Background(), key, data, h))
			assert.Contains(t, mappings.data, pastMeetingChildRepairPrefix+key)
			publisher.parentErr = nil
			publisher.parentPuts = 0
			require.False(t, handleKVPut(context.Background(), key, data, h))
			assert.Positive(t, publisher.parentPuts, "child redelivery must finish the pending parent repair")
			assert.NotContains(t, mappings.data, pastMeetingChildRepairPrefix+key)
		})
	}
}

func TestPastMeetingChildRepair_DeletePreservesRestoredSibling(t *testing.T) {
	for _, side := range []string{"invitee", "attendee"} {
		for _, failRepair := range []bool{false, true} {
			t.Run(side+map[bool]string{false: "/success", true: "/repair retry after tombstone"}[failRepair], func(t *testing.T) {
				objects, mappings := newMemKV(), newMemKV()
				parent := map[string]any{"meeting_and_occurrence_id": testSummaryMeetOcc, "proj_id": "proj-sfid", "_sdc_deleted_at": "2026-01-02T00:00:00Z"}
				objects.data[testPastMeetingKey] = mustMarshalJSON(t, parent)
				ownID, siblingID := "invitee-1", "attendee-1"
				ownPrefix, siblingPrefix := "itx-zoom-past-meetings-invitees.", "itx-zoom-past-meetings-attendees."
				ownMapping, siblingIndex := "v1_past_meeting_invitees.", pastMeetingAttendeeIndexPrefix
				siblingSide := "attendee"
				sibling := minimalAttendeeV1Data(siblingID, testSummaryMeetOcc, "alice")
				if side == "attendee" {
					ownID, siblingID = siblingID, ownID
					ownPrefix, siblingPrefix = siblingPrefix, ownPrefix
					ownMapping, siblingIndex = "v1_past_meeting_attendees.", pastMeetingInviteeIndexPrefix
					siblingSide = "invitee"
					sibling = minimalInviteeV1Data(siblingID, testSummaryMeetOcc, "alice")
				}
				key := ownPrefix + ownID
				mappings.data[ownMapping+ownID] = []byte(buildRegistrantMappingValue(ownID, "alice", testSummaryMeetOcc))
				mappings.data["v1_participant_by_meeting_user."+siblingSide+"."+testSummaryMeetOcc+".alice"] = []byte(siblingID)
				mappings.data[siblingIndex+testSummaryMeetOcc+"."+siblingID] = []byte("1")
				objects.data[siblingPrefix+siblingID] = mustMarshalJSON(t, sibling)
				publisher := &pastMeetingCleanupPublisher{}
				h := newRetriggerHandler(nil, objects, mappings)
				h.publisher = publisher
				publisher.onAccess = func() {
					publisher.onAccess = nil
					delete(parent, "_sdc_deleted_at")
					objects.data[testPastMeetingKey] = mustMarshalJSON(t, parent)
					require.True(t, h.handlePastMeetingUpdate(context.Background(), testPastMeetingKey, parent), "new source-delete tracking must not let restoration ignore the active owner")
					if failRepair {
						publisher.parentErr = errors.New("publisher unavailable")
					}
				}

				assert.Equal(t, failRepair, routeDelete(context.Background(), key, nil, h))
				assert.Equal(t, tombstoneMarker, string(mappings.data[ownMapping+ownID]))
				assert.NotContains(t, publisher.accessSubjects, fgaconstants.GenericMemberRemoveSubject, "a restored sibling must not be revoked")
				assert.Contains(t, publisher.participants, siblingID)
				if failRepair {
					assert.Contains(t, mappings.data, pastMeetingChildRepairPrefix+key)
					publisher.parentErr = nil
					publisher.parentPuts = 0
					require.False(t, routeDelete(context.Background(), key, nil, h))
					assert.Positive(t, publisher.parentPuts, "repair must run before the child's tombstone short-circuit")
				}
				assert.NotContains(t, mappings.data, pastMeetingChildRepairPrefix+key)
			})
		}
	}
}

func TestPastMeetingRepair_DeleteDuringParentPublish(t *testing.T) {
	for _, deletion := range []string{"soft", "hard"} {
		t.Run(deletion, func(t *testing.T) {
			objects, mappings := newMemKV(), newMemKV()
			parent := map[string]any{"meeting_and_occurrence_id": testSummaryMeetOcc, "proj_id": "proj-sfid"}
			objects.data[testPastMeetingKey] = mustMarshalJSON(t, parent)
			publisher := &pastMeetingCleanupPublisher{}
			h := newRetriggerHandler(nil, objects, mappings)
			h.publisher = publisher
			publisher.onParent = func() {
				publisher.onParent = nil
				if deletion == "soft" {
					parent["_sdc_deleted_at"] = "2026-01-02T00:00:00Z"
					objects.data[testPastMeetingKey] = mustMarshalJSON(t, parent)
				} else {
					delete(objects.data, testPastMeetingKey)
				}
				require.True(t, h.handlePastMeetingDelete(context.Background(), testPastMeetingKey, nil), "a foreign parent mutation must request reconciliation rather than race the active owner")
			}

			handled, retry := h.repairPastMeetingIfLive(context.Background(), testSummaryMeetOcc)
			require.True(t, handled)
			require.True(t, retry, "the queued parent request remains durable until a subsequent owned reconciliation")
			require.False(t, h.handlePastMeetingDelete(context.Background(), testPastMeetingKey, nil))
			assert.Equal(t, tombstoneMarker, string(mappings.data["v1_past_meetings."+testSummaryMeetOcc]))
			assertPastMeetingCleanupComplete(t, mappings, testSummaryMeetOcc)
			assert.Greater(t, slices.Index(publisher.events, "delete:"+eventing.IndexV1PastMeetingSubject+":"+testSummaryMeetOcc), slices.Index(publisher.events, "parent-put"))
		})
	}
}

func TestPastMeetingChildRepair_RetriesDeleteAfterStaleRepair(t *testing.T) {
	objects, mappings := newMemKV(), newMemKV()
	parent := map[string]any{"meeting_and_occurrence_id": testSummaryMeetOcc, "proj_id": "proj-sfid"}
	objects.data[testPastMeetingKey] = mustMarshalJSON(t, parent)
	key := "itx-zoom-past-meetings-invitees.invitee-1"
	markerKey := pastMeetingChildRepairPrefix + key
	mappings.data[markerKey] = mustMarshalJSON(t, newChildRepairIntent(testSummaryMeetOcc, childRepairReady))
	publisher := &pastMeetingCleanupPublisher{}
	h := newRetriggerHandler(nil, objects, mappings)
	h.publisher = publisher
	publisher.onParent = func() {
		publisher.onParent = nil
		delete(objects.data, testPastMeetingKey)
		require.True(t, h.handlePastMeetingDelete(context.Background(), testPastMeetingKey, nil), "foreign parent operations cannot consume the active owner")
		// The stale update will overwrite the tombstone; make its compensating
		// cleanup fail before it can remove the resurrected parent.
		publisher.failSubject = eventing.IndexV1PastMeetingSubject
	}

	require.True(t, h.resumePastMeetingChildRepair(context.Background(), key))
	assert.Contains(t, mappings.data, markerKey)
	assert.Equal(t, "1", string(mappings.data["v1_past_meetings."+testSummaryMeetOcc]))
	publisher.failSubject = ""
	require.False(t, h.resumePastMeetingChildRepair(context.Background(), key))
	assert.Equal(t, tombstoneMarker, string(mappings.data["v1_past_meetings."+testSummaryMeetOcc]))
	assert.NotContains(t, mappings.data, markerKey)
}

func TestPastMeetingChildRepair_MarkerFailurePreventsACK(t *testing.T) {
	for _, failure := range []string{"read", "clear", "revision changed"} {
		t.Run(failure, func(t *testing.T) {
			objects, mappings := newMemKV(), newMemKV()
			objects.data[testPastMeetingKey] = mustMarshalJSON(t, map[string]any{"meeting_and_occurrence_id": testSummaryMeetOcc, "proj_id": "proj-sfid"})
			key := "itx-zoom-past-meetings-invitees.invitee-1"
			markerKey := pastMeetingChildRepairPrefix + key
			mappings.data[markerKey] = mustMarshalJSON(t, newChildRepairIntent(testSummaryMeetOcc, childRepairReady))
			switch failure {
			case "read":
				mappings.getErrs[markerKey] = errors.New("kv unavailable")
			case "clear":
				mappings.deleteErrs[markerKey] = errors.New("kv unavailable")
			case "revision changed":
				mappings.deleteErrs[markerKey] = jetstream.ErrKeyRevisionMismatch
			}
			h := newRetriggerHandler(nil, objects, mappings)
			h.publisher = &pastMeetingCleanupPublisher{}
			require.True(t, h.resumePastMeetingChildRepair(context.Background(), key))
			assert.Contains(t, mappings.data, markerKey)
			clear(mappings.getErrs)
			clear(mappings.deleteErrs)
			require.False(t, h.resumePastMeetingChildRepair(context.Background(), key))
			assert.NotContains(t, mappings.data, markerKey)
		})
	}
}
