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
