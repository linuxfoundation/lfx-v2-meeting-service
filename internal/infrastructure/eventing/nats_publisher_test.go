// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fgatypes "github.com/linuxfoundation/lfx-v2-fga-sync/pkg/types"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
)

// TestBuildAccessMemberPutMessage pins the exact member_put shape fga-sync
// requires. A wrong field or operation name here would pass every other test
// in the synchronous creator host-grant change and silently reopen GH-4780.
func TestBuildAccessMemberPutMessage(t *testing.T) {
	msg := buildAccessMemberPutMessage("v1_meeting", "mtg-1", "alice", []string{"host"}, []string{"participant"})

	assert.Equal(t, "v1_meeting", msg.ObjectType)
	assert.Equal(t, "member_put", msg.Operation)

	data, ok := msg.Data.(fgatypes.GenericMemberData)
	require.True(t, ok, "Data must be a GenericMemberData, not left as a map or another type")
	assert.Equal(t, "mtg-1", data.UID)
	assert.Equal(t, "alice", data.Username)
	assert.Equal(t, []string{"host"}, data.Relations)
	assert.Equal(t, []string{"participant"}, data.MutuallyExclusiveWith)
}

// TestNATSPublisher_PublishAccessMemberPut_EmptyUsernameNoOp confirms the
// empty-username guard returns before ever touching the NATS connection, so
// it is safe to assert without a live server.
func TestNATSPublisher_PublishAccessMemberPut_EmptyUsernameNoOp(t *testing.T) {
	p := &NATSPublisher{}

	err := p.PublishAccessMemberPut(context.Background(), "v1_meeting", "mtg-1", "", []string{"host"}, []string{"participant"})
	assert.NoError(t, err)
}

func TestPastMeetingSummaryIndexingConfig(t *testing.T) {
	const (
		summaryID       = "00000000-0000-0000-0000-000000000001"
		meetingAndOccID = "12345678901-1700000000000"
	)

	tests := []struct {
		name             string
		requiresApproval bool
		approved         bool
		summaryAccess    string
		wantPublic       bool
		wantAccess       string
		wantHistory      string
	}{
		{
			name:             "awaiting approval with public access is organizer-only",
			requiresApproval: true,
			summaryAccess:    "public",
			wantPublic:       false,
			wantAccess:       "organizer",
			wantHistory:      "organizer",
		},
		{
			name:             "awaiting approval with participant access is organizer-only",
			requiresApproval: true,
			summaryAccess:    "meeting_participants",
			wantPublic:       false,
			wantAccess:       "organizer",
			wantHistory:      "organizer",
		},
		{
			name:             "awaiting approval with no access value is organizer-only",
			requiresApproval: true,
			summaryAccess:    "",
			wantPublic:       false,
			wantAccess:       "organizer",
			wantHistory:      "organizer",
		},
		{
			name:             "approved with public access is public",
			requiresApproval: true,
			approved:         true,
			summaryAccess:    "public",
			wantPublic:       true,
			wantAccess:       "ai_summary_viewer",
			wantHistory:      "auditor",
		},
		{
			name:             "approved with host access uses ai_summary_viewer",
			requiresApproval: true,
			approved:         true,
			summaryAccess:    "meeting_hosts",
			wantPublic:       false,
			wantAccess:       "ai_summary_viewer",
			wantHistory:      "auditor",
		},
		{
			name:          "approval not required with public access is public",
			summaryAccess: "public",
			wantPublic:    true,
			wantAccess:    "ai_summary_viewer",
			wantHistory:   "auditor",
		},
		{
			name:          "approval not required with host access uses ai_summary_viewer",
			summaryAccess: "meeting_hosts",
			wantPublic:    false,
			wantAccess:    "ai_summary_viewer",
			wantHistory:   "auditor",
		},
		{
			name:          "approval not required with no access value is not public",
			summaryAccess: "",
			wantPublic:    false,
			wantAccess:    "ai_summary_viewer",
			wantHistory:   "auditor",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary := &models.SummaryEventData{
				ID:                     summaryID,
				MeetingAndOccurrenceID: meetingAndOccID,
				MeetingID:              "12345678901",
				RequiresApproval:       tt.requiresApproval,
				Approved:               tt.approved,
			}

			cfg := PastMeetingSummaryIndexingConfig(summary, tt.summaryAccess)

			require.NotNil(t, cfg)
			require.NotNil(t, cfg.Public)
			assert.Equal(t, tt.wantPublic, *cfg.Public)
			assert.Equal(t, summaryID, cfg.ObjectID)
			assert.Equal(t, "v1_past_meeting:"+meetingAndOccID, cfg.AccessCheckObject)
			assert.Equal(t, tt.wantAccess, cfg.AccessCheckRelation)
			assert.Equal(t, "v1_past_meeting:"+meetingAndOccID, cfg.HistoryCheckObject)
			assert.Equal(t, tt.wantHistory, cfg.HistoryCheckRelation)
			assert.Equal(t, summary.Tags(), cfg.Tags)
			assert.Equal(t, summary.ParentRefs(), cfg.ParentRefs)
		})
	}
}

func TestPastMeetingSummaryIndexerMessage(t *testing.T) {
	const meetingAndOccID = "12345678901-1700000000000"

	tests := []struct {
		name        string
		approved    bool
		wantAccess  string
		wantHistory string
		wantPublic  bool
	}{
		{
			name:        "summary awaiting approval is indexed for organizers only",
			approved:    false,
			wantAccess:  "organizer",
			wantHistory: "organizer",
			wantPublic:  false,
		},
		{
			name:        "approved summary is indexed for ai summary viewers",
			approved:    true,
			wantAccess:  "ai_summary_viewer",
			wantHistory: "auditor",
			wantPublic:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary := &models.SummaryEventData{
				ID:                     "00000000-0000-0000-0000-000000000001",
				MeetingAndOccurrenceID: meetingAndOccID,
				MeetingID:              "12345678901",
				RequiresApproval:       true,
				Approved:               tt.approved,
			}

			msg := pastMeetingSummaryIndexerMessage("updated", summary, "public")

			assert.Equal(t, "updated", string(msg.Action))
			assert.Equal(t, summary, msg.Data)
			assert.Equal(t, summary.Tags(), msg.Tags)
			require.NotNil(t, msg.IndexingConfig)
			require.NotNil(t, msg.IndexingConfig.Public)
			assert.Equal(t, tt.wantPublic, *msg.IndexingConfig.Public)
			assert.Equal(t, "v1_past_meeting:"+meetingAndOccID, msg.IndexingConfig.AccessCheckObject)
			assert.Equal(t, tt.wantAccess, msg.IndexingConfig.AccessCheckRelation)
			assert.Equal(t, tt.wantHistory, msg.IndexingConfig.HistoryCheckRelation)
		})
	}
}
