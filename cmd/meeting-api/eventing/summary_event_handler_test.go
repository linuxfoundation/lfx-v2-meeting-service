// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/infrastructure/eventing"
)

const (
	testSummaryID       = "00000000-0000-0000-0000-000000000001"
	testSummaryMeetOcc  = "12345678901-1700000000000"
	testSummaryKVKey    = "itx-zoom-past-meetings-summaries." + testSummaryID
	testSummaryMapping  = "v1_past_meeting_summaries." + testSummaryID
	testPastMeetingKey  = "itx-zoom-past-meetings." + testSummaryMeetOcc
	testPastMeetingRefs = "v1-mappings.past-meeting-mappings." + testSummaryMeetOcc
	testSummaryIndexKey = pastMeetingSummaryIndexKeyPrefix + testSummaryMeetOcc
)

// publishedSummary is one captured PublishPastMeetingSummaryEvent call.
type publishedSummary struct {
	action string
	data   *models.SummaryEventData
	access string
}

// summaryRecordingPublisher records summary publishes and no-ops everything else.
type summaryRecordingPublisher struct {
	mockParticipantPublisher
	published []publishedSummary
}

func (p *summaryRecordingPublisher) PublishPastMeetingSummaryEvent(_ context.Context, action string, data *models.SummaryEventData, access string) error {
	p.published = append(p.published, publishedSummary{action: action, data: data, access: access})
	return nil
}

// summaryV1Record returns a v1 summary record as the KV watcher would decode it.
func summaryV1Record(t *testing.T, requiresApproval, approved bool) map[string]any {
	t.Helper()
	raw := mustMarshalJSON(t, map[string]any{
		"id":                        testSummaryID,
		"meeting_and_occurrence_id": testSummaryMeetOcc,
		"meeting_id":                "12345678901",
		"zoom_meeting_topic":        "Example Foundation Board",
		"zoom_webhook_event":        `{"event":"meeting.summary_completed"}`,
		"summary_overview":          "Overview",
		"requires_approval":         requiresApproval,
		"approved":                  approved,
	})
	data, err := decodeData(raw)
	require.NoError(t, err)
	return data
}

// newSummaryTestHandlers wires handlers whose parent past meeting has the given
// ai_summary_access and whose summary mapping is absent on the first lookup and
// present afterwards, as it is after the first successful publish.
func newSummaryTestHandlers(t *testing.T, aiSummaryAccess string) (*EventHandlers, *summaryRecordingPublisher) {
	t.Helper()

	objectsKV := &mockKeyValue{}
	objectsKV.On("Get", mock.Anything, testPastMeetingKey).Return(mockKeyValueEntry{
		key: testPastMeetingKey,
		value: mustMarshalJSON(t, map[string]any{
			"meeting_and_occurrence_id": testSummaryMeetOcc,
			"proj_id":                   "proj-sfid",
			"ai_summary_access":         aiSummaryAccess,
		}),
	}, nil)

	mappingsKV := &mockKeyValue{}
	mappingsKV.On("Get", mock.Anything, testSummaryMapping).Return(nil, jetstream.ErrKeyNotFound).Once()
	mappingsKV.On("Get", mock.Anything, testSummaryMapping).Return(mockKeyValueEntry{key: testSummaryMapping, value: []byte("1")}, nil)
	mappingsKV.On("Get", mock.Anything, testPastMeetingRefs).Return(nil, jetstream.ErrKeyNotFound)
	mappingsKV.On("Put", mock.Anything, testSummaryMapping, mock.Anything).Return(uint64(1), nil)
	mappingsKV.On("Get", mock.Anything, testSummaryIndexKey).Return(nil, jetstream.ErrKeyNotFound).Once()
	mappingsKV.On("Get", mock.Anything, testSummaryIndexKey).Return(mockKeyValueEntry{key: testSummaryIndexKey, value: []byte(`["` + testSummaryID + `"]`)}, nil)
	mappingsKV.On("Create", mock.Anything, testSummaryIndexKey, []byte(`["`+testSummaryID+`"]`)).Return(uint64(1), nil)

	publisher := &summaryRecordingPublisher{}
	return &EventHandlers{
		publisher:    publisher,
		idMapper:     participantTestIDMapper{},
		v1ObjectsKV:  objectsKV,
		v1MappingsKV: mappingsKV,
		logger:       slog.Default(),
	}, publisher
}

// TestHandlePastMeetingSummaryUpdate_ApprovalReindexes verifies that each change of
// the v1 summary record republishes the document, and that the indexing config
// built from what was published follows the approval state in both directions.
func TestHandlePastMeetingSummaryUpdate_ApprovalReindexes(t *testing.T) {
	type step struct {
		requiresApproval bool
		approved         bool
		wantAction       string
		wantPublic       bool
		wantRelation     string
	}

	tests := []struct {
		name  string
		steps []step
	}{
		{
			name: "approving a pending summary widens it to ai_summary_viewer",
			steps: []step{
				{requiresApproval: true, approved: false, wantAction: "created", wantPublic: false, wantRelation: "organizer"},
				{requiresApproval: true, approved: true, wantAction: "updated", wantPublic: true, wantRelation: "ai_summary_viewer"},
			},
		},
		{
			name: "revoking an approval narrows the summary back to organizer",
			steps: []step{
				{requiresApproval: true, approved: true, wantAction: "created", wantPublic: true, wantRelation: "ai_summary_viewer"},
				{requiresApproval: true, approved: false, wantAction: "updated", wantPublic: false, wantRelation: "organizer"},
			},
		},
		{
			name: "summary that does not require approval keeps ai_summary_viewer",
			steps: []step{
				{requiresApproval: false, approved: false, wantAction: "created", wantPublic: true, wantRelation: "ai_summary_viewer"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, publisher := newSummaryTestHandlers(t, "public")

			for i, s := range tt.steps {
				retry := handleKVPut(context.Background(), testSummaryKVKey, summaryV1Record(t, s.requiresApproval, s.approved), h)
				require.False(t, retry)
				require.Len(t, publisher.published, i+1)

				got := publisher.published[i]
				assert.Equal(t, s.wantAction, got.action)
				assert.Equal(t, "public", got.access)
				assert.Equal(t, s.approved, got.data.Approved)

				cfg := eventing.PastMeetingSummaryIndexingConfig(got.data, got.access)
				require.NotNil(t, cfg.Public)
				assert.Equal(t, s.wantPublic, *cfg.Public)
				assert.Equal(t, s.wantRelation, cfg.AccessCheckRelation)
				assert.Equal(t, "v1_past_meeting:"+testSummaryMeetOcc, cfg.AccessCheckObject)
			}
		})
	}
}

// TestHandlePastMeetingSummaryUpdate_DropsWebhookPayload verifies that a v1 record
// carrying the provider webhook payload still decodes, and that the payload does
// not reach the index document.
func TestHandlePastMeetingSummaryUpdate_DropsWebhookPayload(t *testing.T) {
	h, publisher := newSummaryTestHandlers(t, "public")

	retry := handleKVPut(context.Background(), testSummaryKVKey, summaryV1Record(t, false, false), h)
	require.False(t, retry)
	require.Len(t, publisher.published, 1)

	doc, err := json.Marshal(publisher.published[0].data)
	require.NoError(t, err)

	var fields map[string]any
	require.NoError(t, json.Unmarshal(doc, &fields))
	assert.NotContains(t, fields, "zoom_webhook_event")
	assert.Equal(t, testSummaryID, fields["id"])
	assert.Equal(t, "Example Foundation Board", fields["zoom_meeting_topic"])
}

// TestConvertMapToSummaryData_Content verifies that the index document's content and
// edited_content are the shared summary markdown (pkg/summaryformat), byte for byte the
// text documented in docs/event-processing.md ("Summary Content Assembly"). A change here
// would make freshly indexed documents differ from the ones already in the index.
func TestConvertMapToSummaryData_Content(t *testing.T) {
	const documentedContent = "## Overview\nTeam discussed Q1 roadmap\n\n## Key Topics\n### Feature Planning\nPrioritized auth improvements\n\n### Bug Review\nIdentified 3 critical issues\n\n## Next Steps\n- Schedule design review\n- Update roadmap doc"

	t.Run("documented sample renders the documented content", func(t *testing.T) {
		data := map[string]any{
			"id":                        testSummaryID,
			"meeting_and_occurrence_id": testSummaryMeetOcc,
			"summary_overview":          "Team discussed Q1 roadmap",
			"summary_details": []any{
				map[string]any{"label": "Feature Planning", "summary": "Prioritized auth improvements"},
				map[string]any{"label": "Bug Review", "summary": "Identified 3 critical issues"},
			},
			"next_steps": []any{"Schedule design review", "Update roadmap doc"},
		}

		got, err := convertMapToSummaryData(context.Background(), data, slog.Default())
		require.NoError(t, err)
		assert.Equal(t, documentedContent, got.Content)
		assert.Empty(t, got.EditedContent, "no edited parts: edited_content is empty")
	})

	t.Run("edited parts render the same way into edited_content", func(t *testing.T) {
		data := map[string]any{
			"id":                        testSummaryID,
			"meeting_and_occurrence_id": testSummaryMeetOcc,
			"edited_summary_overview":   "Team discussed Q1 roadmap",
			"edited_summary_details": []any{
				map[string]any{"label": "Feature Planning", "summary": "Prioritized auth improvements"},
				map[string]any{"label": "Bug Review", "summary": "Identified 3 critical issues"},
			},
			"edited_next_steps": []any{"Schedule design review", "Update roadmap doc"},
		}

		got, err := convertMapToSummaryData(context.Background(), data, slog.Default())
		require.NoError(t, err)
		assert.Empty(t, got.Content)
		assert.Equal(t, documentedContent, got.EditedContent)
	})
}

// TestRecordPastMeetingSummary verifies that summaries are recorded under their past meeting
// without losing entries to concurrent writers: a lost entry would leave that summary's public
// flag unrevoked when the past meeting's ai_summary_access is tightened.
func TestRecordPastMeetingSummary(t *testing.T) {
	const otherSummaryID = "00000000-0000-0000-0000-000000000002"
	existing := mockKeyValueEntry{key: testSummaryIndexKey, value: []byte(`["` + otherSummaryID + `"]`)}
	merged := []byte(`["` + otherSummaryID + `","` + testSummaryID + `"]`)

	t.Run("appends to the existing set", func(t *testing.T) {
		kv := &mockKeyValue{}
		kv.On("Get", mock.Anything, testSummaryIndexKey).Return(existing, nil)
		kv.On("Update", mock.Anything, testSummaryIndexKey, merged, uint64(1)).Return(uint64(2), nil)

		require.NoError(t, recordPastMeetingSummary(context.Background(), kv, testSummaryMeetOcc, testSummaryID))
		kv.AssertExpectations(t)
	})

	t.Run("re-reads and retries after losing a create race", func(t *testing.T) {
		kv := &mockKeyValue{}
		kv.On("Get", mock.Anything, testSummaryIndexKey).Return(nil, jetstream.ErrKeyNotFound).Once()
		kv.On("Create", mock.Anything, testSummaryIndexKey, mock.Anything).Return(uint64(0), jetstream.ErrKeyExists)
		kv.On("Get", mock.Anything, testSummaryIndexKey).Return(existing, nil)
		kv.On("Update", mock.Anything, testSummaryIndexKey, merged, uint64(1)).Return(uint64(2), nil)

		require.NoError(t, recordPastMeetingSummary(context.Background(), kv, testSummaryMeetOcc, testSummaryID))
		kv.AssertExpectations(t)
	})

	t.Run("re-reads and keeps the other writer's entries after an update conflict", func(t *testing.T) {
		const concurrentSummaryID = "00000000-0000-0000-0000-000000000003"
		extended := mockKeyValueEntry{key: testSummaryIndexKey, value: []byte(`["` + otherSummaryID + `","` + concurrentSummaryID + `"]`)}
		allThree := []byte(`["` + otherSummaryID + `","` + concurrentSummaryID + `","` + testSummaryID + `"]`)

		kv := &mockKeyValue{}
		kv.On("Get", mock.Anything, testSummaryIndexKey).Return(existing, nil).Once()
		kv.On("Update", mock.Anything, testSummaryIndexKey, merged, uint64(1)).Return(uint64(0), jetstream.ErrKeyRevisionMismatch).Once()
		kv.On("Get", mock.Anything, testSummaryIndexKey).Return(extended, nil).Once()
		kv.On("Update", mock.Anything, testSummaryIndexKey, allThree, uint64(1)).Return(uint64(3), nil).Once()

		require.NoError(t, recordPastMeetingSummary(context.Background(), kv, testSummaryMeetOcc, testSummaryID))
		kv.AssertExpectations(t)
	})

	t.Run("returns an error once conflicts exhaust the attempts", func(t *testing.T) {
		kv := &mockKeyValue{}
		kv.On("Get", mock.Anything, testSummaryIndexKey).Return(existing, nil)
		kv.On("Update", mock.Anything, testSummaryIndexKey, merged, uint64(1)).Return(uint64(0), jetstream.ErrKeyRevisionMismatch)

		require.Error(t, recordPastMeetingSummary(context.Background(), kv, testSummaryMeetOcc, testSummaryID))
		kv.AssertNumberOfCalls(t, "Get", maxSummaryIndexUpdateAttempts)
		kv.AssertNumberOfCalls(t, "Update", maxSummaryIndexUpdateAttempts)
	})

	t.Run("does not rewrite a set that already holds the summary", func(t *testing.T) {
		kv := &mockKeyValue{}
		kv.On("Get", mock.Anything, testSummaryIndexKey).Return(mockKeyValueEntry{key: testSummaryIndexKey, value: []byte(`["` + testSummaryID + `"]`)}, nil)

		require.NoError(t, recordPastMeetingSummary(context.Background(), kv, testSummaryMeetOcc, testSummaryID))
		kv.AssertNotCalled(t, "Update", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})
}
