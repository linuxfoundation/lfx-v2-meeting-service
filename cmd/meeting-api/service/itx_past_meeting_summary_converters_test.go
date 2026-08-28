// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	meetingservice "github.com/linuxfoundation/lfx-v2-meeting-service/gen/meeting_service"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/models/itx"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/utils"
)

// ── ConvertUpdatePastMeetingSummaryPayload ───────────────────────────────────

func TestConvertUpdatePastMeetingSummaryPayload(t *testing.T) {
	t.Run("nil edited_content leaves ITX fields empty", func(t *testing.T) {
		req := ConvertUpdatePastMeetingSummaryPayload(&meetingservice.UpdateItxPastMeetingSummaryPayload{})

		assert.Empty(t, req.EditedSummaryOverview)
		assert.Empty(t, req.EditedSummaryDetails)
		assert.Empty(t, req.EditedNextSteps)
		assert.Nil(t, req.Approved)
	})

	t.Run("empty string edited_content leaves ITX fields empty", func(t *testing.T) {
		empty := ""
		req := ConvertUpdatePastMeetingSummaryPayload(&meetingservice.UpdateItxPastMeetingSummaryPayload{
			EditedContent: &empty,
		})

		assert.Empty(t, req.EditedSummaryOverview)
		assert.Empty(t, req.EditedSummaryDetails)
		assert.Empty(t, req.EditedNextSteps)
	})

	t.Run("content is parsed into overview, details, and next steps", func(t *testing.T) {
		content := "Meeting went well.\n\nDecision: Approved budget.\n\nNext Steps:\n- Follow up with team"
		req := ConvertUpdatePastMeetingSummaryPayload(&meetingservice.UpdateItxPastMeetingSummaryPayload{
			EditedContent: &content,
		})

		assert.NotEmpty(t, req.EditedSummaryOverview)
		// Parser routes "Decision: ..." as a detail and "Next Steps" entries as next steps
		assert.NotEmpty(t, req.EditedNextSteps)
	})

	t.Run("approved nil is passed through as nil", func(t *testing.T) {
		req := ConvertUpdatePastMeetingSummaryPayload(&meetingservice.UpdateItxPastMeetingSummaryPayload{})
		assert.Nil(t, req.Approved)
	})

	t.Run("approved true is forwarded", func(t *testing.T) {
		req := ConvertUpdatePastMeetingSummaryPayload(&meetingservice.UpdateItxPastMeetingSummaryPayload{
			Approved: utils.BoolPtr(true),
		})
		require.NotNil(t, req.Approved)
		assert.True(t, *req.Approved)
	})

	t.Run("approved false is forwarded distinctly from nil", func(t *testing.T) {
		req := ConvertUpdatePastMeetingSummaryPayload(&meetingservice.UpdateItxPastMeetingSummaryPayload{
			Approved: utils.BoolPtr(false),
		})
		require.NotNil(t, req.Approved)
		assert.False(t, *req.Approved)
	})
}

// ── ConvertPastMeetingSummaryToGoa ───────────────────────────────────────────

func TestConvertPastMeetingSummaryToGoa(t *testing.T) {
	t.Run("maps identity and audit fields", func(t *testing.T) {
		resp := &itx.PastMeetingSummaryResponse{
			ID:                     "sum-001",
			MeetingAndOccurrenceID: "zoom-100-occ-200",
			MeetingID:              "zoom-100",
			SummaryStartTime:       "2026-06-01T10:00:00Z",
			SummaryEndTime:         "2026-06-01T11:00:00Z",
			RequiresApproval:       true,
			Approved:               false,
			CreatedAt:              "2026-06-01T09:00:00Z",
			ModifiedAt:             "2026-06-02T09:00:00Z",
		}

		g := ConvertPastMeetingSummaryToGoa(resp)

		assert.Equal(t, "sum-001", g.UID)
		assert.Equal(t, "zoom-100-occ-200", g.PastMeetingID)
		assert.Equal(t, "zoom-100", g.MeetingID)
		assert.Equal(t, "Zoom", g.Platform)
		assert.True(t, g.RequiresApproval)
		assert.False(t, g.Approved)
		assert.Equal(t, "2026-06-01T09:00:00Z", g.CreatedAt)
		assert.Equal(t, "2026-06-02T09:00:00Z", g.UpdatedAt)
		assert.False(t, g.EmailSent, "ITX does not track email_sent; always false")
	})

	t.Run("zoom_config populated when meeting_id is present", func(t *testing.T) {
		resp := &itx.PastMeetingSummaryResponse{
			MeetingID:       "zoom-100",
			ZoomMeetingUUID: "abc-uuid",
		}

		g := ConvertPastMeetingSummaryToGoa(resp)

		require.NotNil(t, g.ZoomConfig)
		assert.Equal(t, "zoom-100", utils.StringValue(g.ZoomConfig.MeetingID))
		assert.Equal(t, "abc-uuid", utils.StringValue(g.ZoomConfig.MeetingUUID))
	})

	t.Run("zoom_config nil when neither meeting_id nor zoom_uuid are set", func(t *testing.T) {
		g := ConvertPastMeetingSummaryToGoa(&itx.PastMeetingSummaryResponse{})
		assert.Nil(t, g.ZoomConfig)
	})

	t.Run("summary_data carries start_time end_time and title", func(t *testing.T) {
		resp := &itx.PastMeetingSummaryResponse{
			SummaryStartTime: "2026-06-01T10:00:00Z",
			SummaryEndTime:   "2026-06-01T11:00:00Z",
			SummaryTitle:     "Board Q2 Summary",
		}

		g := ConvertPastMeetingSummaryToGoa(resp)

		require.NotNil(t, g.SummaryData)
		assert.Equal(t, "2026-06-01T10:00:00Z", g.SummaryData.StartTime)
		assert.Equal(t, "2026-06-01T11:00:00Z", g.SummaryData.EndTime)
		assert.Equal(t, "Board Q2 Summary", utils.StringValue(g.SummaryData.Title))
	})
}

// ── buildContentFromITX ──────────────────────────────────────────────────────

func TestBuildContentFromITX(t *testing.T) {
	t.Run("empty response returns empty string", func(t *testing.T) {
		assert.Empty(t, buildContentFromITX(&itx.PastMeetingSummaryResponse{}))
	})

	t.Run("overview only", func(t *testing.T) {
		resp := &itx.PastMeetingSummaryResponse{SummaryOverview: "Great meeting."}
		assert.Equal(t, "Great meeting.", buildContentFromITX(resp))
	})

	t.Run("overview + details joined with double newline", func(t *testing.T) {
		resp := &itx.PastMeetingSummaryResponse{
			SummaryOverview: "Overview text.",
			SummaryDetails: []itx.ZoomMeetingSummaryDetails{
				{Label: "Decision", Summary: "Budget approved."},
			},
		}
		result := buildContentFromITX(resp)
		assert.Contains(t, result, "Overview text.")
		assert.Contains(t, result, "Decision: Budget approved.")
	})

	t.Run("details with blank label or summary are skipped", func(t *testing.T) {
		resp := &itx.PastMeetingSummaryResponse{
			SummaryDetails: []itx.ZoomMeetingSummaryDetails{
				{Label: "", Summary: "orphan"},
				{Label: "Action", Summary: ""},
				{Label: "Valid", Summary: "entry"},
			},
		}
		result := buildContentFromITX(resp)
		assert.NotContains(t, result, "orphan")
		assert.NotContains(t, result, "Action:")
		assert.Contains(t, result, "Valid: entry")
	})

	t.Run("next steps section is appended with header and dashes", func(t *testing.T) {
		resp := &itx.PastMeetingSummaryResponse{
			NextSteps: []string{"Send report", "Schedule follow-up"},
		}
		result := buildContentFromITX(resp)
		assert.Contains(t, result, "Next Steps:")
		assert.Contains(t, result, "- Send report")
		assert.Contains(t, result, "- Schedule follow-up")
	})
}

// ── buildEditedContentFromITX ───────────────────────────────────────────────

func TestBuildEditedContentFromITX(t *testing.T) {
	t.Run("empty response returns empty string", func(t *testing.T) {
		assert.Empty(t, buildEditedContentFromITX(&itx.PastMeetingSummaryResponse{}))
	})

	t.Run("edited overview only", func(t *testing.T) {
		resp := &itx.PastMeetingSummaryResponse{EditedSummaryOverview: "Edited overview."}
		assert.Equal(t, "Edited overview.", buildEditedContentFromITX(resp))
	})

	t.Run("edited details with blank label or blank summary are skipped", func(t *testing.T) {
		resp := &itx.PastMeetingSummaryResponse{
			EditedSummaryDetails: []itx.ZoomMeetingSummaryDetails{
				{Label: "", Summary: "orphan"},
				{Label: "Action", Summary: ""},
				{Label: "Valid", Summary: "entry"},
			},
		}
		result := buildEditedContentFromITX(resp)
		assert.NotContains(t, result, "orphan")
		assert.NotContains(t, result, "Action:")
		assert.Contains(t, result, "Valid: entry")
	})

	t.Run("edited next steps appended with header and dashes", func(t *testing.T) {
		resp := &itx.PastMeetingSummaryResponse{
			EditedNextSteps: []string{"Send report", "Schedule follow-up"},
		}
		result := buildEditedContentFromITX(resp)
		assert.Contains(t, result, "Next Steps:")
		assert.Contains(t, result, "- Send report")
		assert.Contains(t, result, "- Schedule follow-up")
	})
}

// ── parseContentIntoITXParts ─────────────────────────────────────────────────

func TestParseContentIntoITXParts(t *testing.T) {
	t.Run("empty string returns zero values", func(t *testing.T) {
		overview, details, nextSteps := parseContentIntoITXParts("")
		assert.Empty(t, overview)
		assert.Nil(t, details)
		assert.Nil(t, nextSteps)
	})

	t.Run("plain paragraph becomes overview", func(t *testing.T) {
		overview, details, nextSteps := parseContentIntoITXParts("This was a productive meeting.")
		assert.Equal(t, "This was a productive meeting.", overview)
		assert.Empty(t, details)
		assert.Empty(t, nextSteps)
	})

	t.Run("Label: Summary lines become details", func(t *testing.T) {
		content := "Decision: Budget approved.\n\nAction: Hire two engineers."
		_, details, _ := parseContentIntoITXParts(content)
		require.Len(t, details, 2)
		assert.Equal(t, "Decision", details[0].Label)
		assert.Equal(t, "Budget approved.", details[0].Summary)
		assert.Equal(t, "Action", details[1].Label)
	})

	t.Run("Next Steps section is extracted as next steps slice", func(t *testing.T) {
		content := "Overview.\n\nNext Steps:\n- Send report\n- Schedule follow-up"
		overview, _, nextSteps := parseContentIntoITXParts(content)
		assert.Equal(t, "Overview.", overview)
		require.Len(t, nextSteps, 2)
		assert.Equal(t, "Send report", nextSteps[0])
		assert.Equal(t, "Schedule follow-up", nextSteps[1])
	})

	t.Run("round-trip: buildContentFromITX output can be parsed back", func(t *testing.T) {
		resp := &itx.PastMeetingSummaryResponse{
			SummaryOverview: "Overview.",
			NextSteps:       []string{"Follow up"},
		}
		content := buildContentFromITX(resp)
		overview, _, nextSteps := parseContentIntoITXParts(content)
		assert.Equal(t, "Overview.", overview)
		require.NotEmpty(t, nextSteps)
		assert.Equal(t, "Follow up", nextSteps[0])
	})
}
