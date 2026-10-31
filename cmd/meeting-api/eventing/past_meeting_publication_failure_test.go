// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/infrastructure/eventing"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPastMeetingArtifact_FailedRetractionSurvivesParentOnlyRecovery(t *testing.T) {
	for _, kind := range []string{"recording", "transcript", "summary", "attachment"} {
		for _, recovery := range []string{"parent only", "parent delete", "child redelivery"} {
			t.Run(kind+"/"+recovery, func(t *testing.T) {
				objects, mappings := newMemKV(), newMemKV()
				parent := map[string]any{"meeting_and_occurrence_id": testSummaryMeetOcc, "proj_id": "proj-sfid"}
				objects.data[testPastMeetingKey] = mustMarshalJSON(t, parent)
				mappings.data["v1_past_meetings."+testSummaryMeetOcc] = []byte("1")
				key := "itx-zoom-past-meetings-recordings." + testSummaryMeetOcc
				data := map[string]any{"meeting_and_occurrence_id": testSummaryMeetOcc, "proj_id": "proj-sfid", "recording_files": []any{map[string]any{"file_type": "TRANSCRIPT"}}}
				failureSubject := eventing.IndexV1PastMeetingRecordingSubject
				switch kind {
				case "transcript":
					failureSubject = eventing.IndexV1PastMeetingTranscriptSubject
				case "summary":
					key, failureSubject = testSummaryKVKey, eventing.IndexV1PastMeetingSummarySubject
					data["id"] = testSummaryID
				case "attachment":
					key, failureSubject = "itx-zoom-past-meetings-attachments.attachment-1", eventing.IndexV1PastMeetingAttachmentSubject
					data["id"] = "attachment-1"
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
					require.False(t, h.handlePastMeetingDelete(context.Background(), testPastMeetingKey, nil))
					// The suspended artifact publication lands AFTER completed
					// parent cleanup. Its own compensating delete will then fail.
					publisher.failSubject = failureSubject
				}
				require.True(t, handleKVPut(context.Background(), key, data, h))
				markerKey := pastMeetingChildRepairPrefix + key
				require.Equal(t, childRepairReady, readChildRepairIntent(t, mappings, key).Phase)
				generation, err := h.parentSourceRevision(context.Background(), testSummaryMeetOcc)
				require.NoError(t, err)
				complete, err := h.cleanupCompletedForRevision(context.Background(), testSummaryMeetOcc, generation)
				require.NoError(t, err)
				require.True(t, complete, "parent-wide cleanup will legitimately be skipped on recovery")
				publisher.indexerDeletes = nil
				recover := func() bool {
					if recovery == "parent only" {
						return h.handlePastMeetingUpdate(context.Background(), testPastMeetingKey, parent)
					}
					if recovery == "parent delete" {
						return routeDelete(context.Background(), testPastMeetingKey, nil, h)
					}
					return handleKVPut(context.Background(), key, data, h)
				}
				require.True(t, recover(), "another artifact delete failure must retain the obligation")
				assert.Contains(t, mappings.data, markerKey)
				assert.Equal(t, childRepairReady, readChildRepairIntent(t, mappings, key).Phase)
				publisher.failSubject = ""
				require.False(t, recover())
				assert.NotContains(t, mappings.data, markerKey)
				id := testSummaryMeetOcc
				switch kind {
				case "summary":
					id = testSummaryID
				case "attachment":
					id = "attachment-1"
				}
				assert.Contains(t, publisher.indexerDeletes, failureSubject+":"+id)
				if kind == "recording" || kind == "transcript" {
					assert.Contains(t, publisher.indexerDeletes, eventing.IndexV1PastMeetingRecordingSubject+":"+id)
					assert.Contains(t, publisher.indexerDeletes, eventing.IndexV1PastMeetingTranscriptSubject+":"+id)
				}
				assert.NotContains(t, publisher.indexerDeletes, eventing.IndexV1PastMeetingSubject+":"+testSummaryMeetOcc, "the pending leaf retraction, not a repeated parent cleanup, must finish recovery")
			})
		}
	}
}

func TestPastMeetingDelete_PublicationFailureCannotCompleteNewGeneration(t *testing.T) {
	for _, stage := range []string{"indexer", "access"} {
		for _, publicationErr := range []error{nats.ErrReconnectBufExceeded, errors.New("broker refused publication")} {
			t.Run(stage+"/"+publicationErr.Error(), func(t *testing.T) {
				assert.False(t, isTransientError(publicationErr), "this error intentionally bypasses the previous keyword classifier")
				objects, mappings := newMemKV(), newMemKV()
				deleted := map[string]any{"meeting_and_occurrence_id": testSummaryMeetOcc, "proj_id": "proj-sfid", "_sdc_deleted_at": "2026-01-02T00:00:00Z"}
				_, err := objects.Put(context.Background(), testPastMeetingKey, mustMarshalJSON(t, deleted))
				require.NoError(t, err)
				publisher := &pastMeetingCleanupPublisher{}
				h := newRetriggerHandler(nil, objects, mappings)
				h.publisher = publisher
				require.False(t, h.handlePastMeetingDelete(context.Background(), testPastMeetingKey, nil))
				completionKey := pastMeetingCleanupKeyPrefix + testSummaryMeetOcc
				previousCompletion := string(mappings.data[completionKey])
				mappingKey := "v1_past_meetings." + testSummaryMeetOcc
				previousTombstoneRevision := mappings.revision(mappingKey)
				live := map[string]any{"meeting_and_occurrence_id": testSummaryMeetOcc, "proj_id": "proj-sfid"}
				_, err = objects.Put(context.Background(), testPastMeetingKey, mustMarshalJSON(t, live))
				require.NoError(t, err)
				// Do not process the restore event: retain the previous tombstone.
				_, err = objects.Put(context.Background(), testPastMeetingKey, mustMarshalJSON(t, deleted))
				require.NoError(t, err)
				generation := objects.revision(testPastMeetingKey)
				if stage == "indexer" {
					publisher.failSubject = eventing.IndexV1PastMeetingSubject
					publisher.deleteErr = publicationErr
				} else {
					publisher.accessErr = publicationErr
				}
				require.True(t, h.handlePastMeetingDelete(context.Background(), testPastMeetingKey, nil))
				assert.Equal(t, previousTombstoneRevision, mappings.revision(mappingKey), "failed publication cannot write a new tombstone")
				assert.Equal(t, previousCompletion, string(mappings.data[completionKey]), "an old tombstone is not evidence of success in this attempt")
				complete, err := h.cleanupCompletedForRevision(context.Background(), testSummaryMeetOcc, generation)
				require.NoError(t, err)
				assert.False(t, complete)
				publisher.failSubject, publisher.deleteErr, publisher.accessErr = "", nil, nil
				publisher.indexerDeletes, publisher.accessDeletes = nil, 0
				require.False(t, h.handlePastMeetingDelete(context.Background(), testPastMeetingKey, nil))
				assert.Contains(t, publisher.indexerDeletes, eventing.IndexV1PastMeetingSubject+":"+testSummaryMeetOcc)
				assert.Equal(t, 1, publisher.accessDeletes)
				assert.Greater(t, mappings.revision(mappingKey), previousTombstoneRevision)
				var current pastMeetingCleanupCompletion
				require.NoError(t, json.Unmarshal(mappings.data[completionKey], &current))
				assert.True(t, current.Complete)
				assert.Equal(t, generation, current.SourceRevision)
			})
		}
	}
}
