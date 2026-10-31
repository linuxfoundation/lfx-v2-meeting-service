// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"testing"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/infrastructure/eventing"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPastMeetingAttachment_ReplayRespectsNewerSourceDeletion(t *testing.T) {
	for _, timing := range []string{"after source read", "during publication", "retraction fails", "recreated during retraction"} {
		t.Run(timing, func(t *testing.T) {
			objects, mappings := newMemKV(), newMemKV()
			parent := map[string]any{"meeting_and_occurrence_id": testSummaryMeetOcc, "proj_id": "proj-sfid"}
			objects.data[testPastMeetingKey] = mustMarshalJSON(t, parent)
			mappings.data["v1_past_meetings."+testSummaryMeetOcc] = []byte(tombstoneMarker)
			mappings.data[pastMeetingCleanupKeyPrefix+testSummaryMeetOcc] = []byte("1")
			key := "itx-zoom-past-meetings-attachments.attachment-1"
			objects.data[key] = mustMarshalJSON(t, map[string]any{"id": "attachment-1", "meeting_and_occurrence_id": testSummaryMeetOcc, "name": "Original attachment"})
			mappings.data[pastMeetingAttachmentIndexPrefix+testSummaryMeetOcc] = []byte(`["attachment-1"]`)
			mappingKey := "v1_past_meeting_attachments.attachment-1"
			mappings.data[mappingKey] = []byte("1")
			publisher := &pastMeetingCleanupPublisher{}
			h, other := newRetriggerHandler(nil, objects, mappings), newRetriggerHandler(nil, objects, mappings)
			h.publisher, other.publisher = publisher, publisher
			deleteSource := func() {
				objects.afterGet, publisher.beforeArtifact = nil, nil
				require.NoError(t, objects.Delete(context.Background(), key))
				require.False(t, routeDelete(context.Background(), key, nil, other))
				require.Equal(t, tombstoneMarker, string(mappings.data[mappingKey]))
				if timing == "retraction fails" {
					publisher.failSubject = eventing.IndexV1PastMeetingAttachmentSubject
					publisher.deleteErr = nats.ErrReconnectBufExceeded
				}
				if timing == "recreated during retraction" {
					publisher.onDelete = func(subject string) {
						if subject != eventing.IndexV1PastMeetingAttachmentSubject {
							return
						}
						publisher.onDelete = nil
						_, err := objects.Put(context.Background(), key, mustMarshalJSON(t, map[string]any{
							"id": "attachment-1", "meeting_and_occurrence_id": testSummaryMeetOcc, "name": "Replacement attachment",
						}))
						require.NoError(t, err)
					}
				}
			}
			if timing == "after source read" {
				objects.afterGet = func(readKey string) {
					if readKey == key {
						deleteSource()
					}
				}
			} else {
				publisher.beforeArtifact = func(subject string) {
					if subject == eventing.IndexV1PastMeetingAttachmentSubject {
						deleteSource()
					}
				}
			}
			retry := h.handlePastMeetingUpdate(context.Background(), testPastMeetingKey, parent)
			if timing == "retraction fails" || timing == "recreated during retraction" {
				require.True(t, retry, "events: %v", publisher.events)
				assert.Equal(t, childRepairReady, readParentOperation(t, mappings).Phase)
				assert.True(t, readChildRepairIntent(t, mappings, key).AttachmentSourceDelete)
				if timing == "retraction fails" {
					require.True(t, other.handlePastMeetingUpdate(context.Background(), testPastMeetingKey, parent), "a live parent cannot discharge a failed child-source retraction")
					assert.Contains(t, mappings.data, pastMeetingChildRepairPrefix+key)
					publisher.failSubject, publisher.deleteErr = "", nil
				}
				require.False(t, other.handlePastMeetingUpdate(context.Background(), testPastMeetingKey, parent))
			} else {
				require.False(t, retry)
			}
			assert.NotContains(t, mappings.data, pastMeetingChildRepairPrefix+key)
			assert.NotContains(t, mappings.data, pastMeetingOperationPrefix+testSummaryMeetOcc)
			if timing == "recreated during retraction" {
				require.NotNil(t, publisher.lastAttachment)
				assert.Equal(t, "Replacement attachment", publisher.lastAttachment.Name)
				assert.Equal(t, "1", string(mappings.data[mappingKey]))
			} else {
				assert.Equal(t, tombstoneMarker, string(mappings.data[mappingKey]), "saved replay bytes must not replace a newer deletion tombstone")
				lastPut, lastDelete := -1, -1
				for i, event := range publisher.events {
					if event == "attachment-put" {
						lastPut = i
					}
					if event == "delete:"+eventing.IndexV1PastMeetingAttachmentSubject+":attachment-1" {
						lastDelete = i
					}
				}
				assert.Greater(t, lastDelete, lastPut, "unconditional source retraction must follow any stale publication")
			}
		})
	}
}

func TestPastMeetingAttachment_ReplayRetriesNewerLiveSource(t *testing.T) {
	objects, mappings := newMemKV(), newMemKV()
	parent := map[string]any{"meeting_and_occurrence_id": testSummaryMeetOcc, "proj_id": "proj-sfid"}
	objects.data[testPastMeetingKey] = mustMarshalJSON(t, parent)
	mappings.data["v1_past_meetings."+testSummaryMeetOcc] = []byte(tombstoneMarker)
	key := "itx-zoom-past-meetings-attachments.attachment-1"
	objects.data[key] = mustMarshalJSON(t, map[string]any{"id": "attachment-1", "meeting_and_occurrence_id": testSummaryMeetOcc, "name": "Original attachment"})
	mappings.data[pastMeetingAttachmentIndexPrefix+testSummaryMeetOcc] = []byte(`["attachment-1"]`)
	objects.afterGet = func(readKey string) {
		if readKey != key {
			return
		}
		objects.afterGet = nil
		_, err := objects.Put(context.Background(), key, mustMarshalJSON(t, map[string]any{"id": "attachment-1", "meeting_and_occurrence_id": testSummaryMeetOcc, "name": "Replacement attachment"}))
		require.NoError(t, err)
	}
	publisher := &pastMeetingCleanupPublisher{}
	h := newRetriggerHandler(nil, objects, mappings)
	h.publisher = publisher
	require.True(t, h.handlePastMeetingUpdate(context.Background(), testPastMeetingKey, parent))
	assert.Empty(t, publisher.attachments, "do not publish a live snapshot already superseded by a newer source revision")
	assert.Equal(t, childRepairReady, readParentOperation(t, mappings).Phase)
	require.False(t, h.handlePastMeetingUpdate(context.Background(), testPastMeetingKey, parent))
	require.NotNil(t, publisher.lastAttachment)
	assert.Equal(t, "Replacement attachment", publisher.lastAttachment.Name)
	assert.NotContains(t, mappings.data, pastMeetingOperationPrefix+testSummaryMeetOcc)
}
