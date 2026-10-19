// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	meetingservice "github.com/linuxfoundation/lfx-v2-meeting-service/gen/meeting_service"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/models/itx"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/summaryformat"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/utils"
)

// indexedSummaryContent is the `content` of the v1_past_meeting_summary index document
// sample in docs/event-processing.md: what LFX Self Serve displays and sends back as
// edited_content.
const indexedSummaryContent = "## Overview\nTeam discussed Q1 roadmap.\n\n## Key Topics\n### Feature Planning\nFocus on authentication improvements.\n\n### Bug Review\nIdentified 3 critical bugs.\n\n## Next Steps\n- Schedule design review\n- Update roadmap documentation\n- Assign bug fixes"

// marshalledKeys returns the top-level JSON keys the ITX request serializes to.
func marshalledKeys(t *testing.T, req *itx.UpdatePastMeetingSummaryRequest) map[string]json.RawMessage {
	t.Helper()
	b, err := json.Marshal(req)
	require.NoError(t, err)
	var keys map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(b, &keys))
	return keys
}

// ── ConvertUpdatePastMeetingSummaryPayload ───────────────────────────────────

func TestConvertUpdatePastMeetingSummaryPayload(t *testing.T) {
	t.Run("nil edited_content sends none of the edited fields", func(t *testing.T) {
		req := ConvertUpdatePastMeetingSummaryPayload(&meetingservice.UpdateItxPastMeetingSummaryPayload{})

		assert.Nil(t, req.EditedSummaryOverview)
		assert.Nil(t, req.EditedSummaryDetails)
		assert.Nil(t, req.EditedNextSteps)
		assert.Nil(t, req.Approved)
	})

	t.Run("empty string edited_content is no edit", func(t *testing.T) {
		empty := ""
		req := ConvertUpdatePastMeetingSummaryPayload(&meetingservice.UpdateItxPastMeetingSummaryPayload{
			EditedContent: &empty,
		})

		assert.Nil(t, req.EditedSummaryOverview)
		assert.Nil(t, req.EditedSummaryDetails)
		assert.Nil(t, req.EditedNextSteps)
	})

	t.Run("whitespace-only edited_content is no edit", func(t *testing.T) {
		blank := " \n\t\n"
		req := ConvertUpdatePastMeetingSummaryPayload(&meetingservice.UpdateItxPastMeetingSummaryPayload{
			EditedContent: &blank,
		})

		assert.Nil(t, req.EditedSummaryOverview)
		assert.Nil(t, req.EditedSummaryDetails)
		assert.Nil(t, req.EditedNextSteps)
	})

	t.Run("edit in the index markdown is parsed into the three parts", func(t *testing.T) {
		content := "## Overview\nMeeting went well.\n\n## Key Topics\n### Decision\nApproved budget.\n\n## Next Steps\n- Follow up with team"
		req := ConvertUpdatePastMeetingSummaryPayload(&meetingservice.UpdateItxPastMeetingSummaryPayload{
			EditedContent: &content,
		})

		require.NotNil(t, req.EditedSummaryOverview)
		assert.Equal(t, "Meeting went well.", *req.EditedSummaryOverview)
		require.NotNil(t, req.EditedSummaryDetails)
		assert.Equal(t, []itx.ZoomMeetingSummaryDetails{{Label: "Decision", Summary: "Approved budget."}}, *req.EditedSummaryDetails)
		require.NotNil(t, req.EditedNextSteps)
		assert.Equal(t, []string{"Follow up with team"}, *req.EditedNextSteps)
	})

	t.Run("regression: the indexed sample yields two details and three steps", func(t *testing.T) {
		// The former paragraph parser read this text as one overview with the headings
		// embedded, no details and no steps (and one "Label: text" detail if a line had a colon).
		content := indexedSummaryContent
		req := ConvertUpdatePastMeetingSummaryPayload(&meetingservice.UpdateItxPastMeetingSummaryPayload{
			EditedContent: &content,
		})

		require.NotNil(t, req.EditedSummaryOverview)
		assert.Equal(t, "Team discussed Q1 roadmap.", *req.EditedSummaryOverview)
		require.NotNil(t, req.EditedSummaryDetails)
		assert.Equal(t, []itx.ZoomMeetingSummaryDetails{
			{Label: "Feature Planning", Summary: "Focus on authentication improvements."},
			{Label: "Bug Review", Summary: "Identified 3 critical bugs."},
		}, *req.EditedSummaryDetails)
		require.NotNil(t, req.EditedNextSteps)
		assert.Equal(t, []string{"Schedule design review", "Update roadmap documentation", "Assign bug fixes"}, *req.EditedNextSteps)
	})

	t.Run("an edit with no details and no steps marshals them as empty arrays", func(t *testing.T) {
		// ITX must receive [] (as the v1 UI sends for a cleared list), not an omitted key
		// that would leave stale details and steps in place.
		content := "## Overview\nOnly an overview remains."
		req := ConvertUpdatePastMeetingSummaryPayload(&meetingservice.UpdateItxPastMeetingSummaryPayload{
			EditedContent: &content,
		})

		keys := marshalledKeys(t, req)
		assert.JSONEq(t, `"Only an overview remains."`, string(keys["edited_summary_overview"]))
		assert.JSONEq(t, `[]`, string(keys["edited_summary_details"]))
		assert.JSONEq(t, `[]`, string(keys["edited_next_steps"]))
	})

	t.Run("an edit that removes the overview marshals it as an empty string", func(t *testing.T) {
		content := "## Next Steps\n- Only a step remains"
		req := ConvertUpdatePastMeetingSummaryPayload(&meetingservice.UpdateItxPastMeetingSummaryPayload{
			EditedContent: &content,
		})

		keys := marshalledKeys(t, req)
		assert.JSONEq(t, `""`, string(keys["edited_summary_overview"]))
		assert.JSONEq(t, `[]`, string(keys["edited_summary_details"]))
		assert.JSONEq(t, `["Only a step remains"]`, string(keys["edited_next_steps"]))
	})

	t.Run("approval-only payload marshals none of the edited keys", func(t *testing.T) {
		req := ConvertUpdatePastMeetingSummaryPayload(&meetingservice.UpdateItxPastMeetingSummaryPayload{
			Approved: utils.BoolPtr(true),
		})

		keys := marshalledKeys(t, req)
		assert.NotContains(t, keys, "edited_summary_overview")
		assert.NotContains(t, keys, "edited_summary_details")
		assert.NotContains(t, keys, "edited_next_steps")
		assert.JSONEq(t, `true`, string(keys["approved"]))
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

	t.Run("zoom_config populated when only meeting_id is set", func(t *testing.T) {
		resp := &itx.PastMeetingSummaryResponse{
			MeetingID: "zoom-100",
		}

		g := ConvertPastMeetingSummaryToGoa(resp)

		require.NotNil(t, g.ZoomConfig)
		assert.Equal(t, "zoom-100", utils.StringValue(g.ZoomConfig.MeetingID))
		assert.Nil(t, g.ZoomConfig.MeetingUUID)
	})

	t.Run("zoom_config populated when only zoom_uuid is set", func(t *testing.T) {
		resp := &itx.PastMeetingSummaryResponse{
			ZoomMeetingUUID: "abc-uuid",
		}

		g := ConvertPastMeetingSummaryToGoa(resp)

		require.NotNil(t, g.ZoomConfig)
		assert.Equal(t, "abc-uuid", utils.StringValue(g.ZoomConfig.MeetingUUID))
		assert.Nil(t, g.ZoomConfig.MeetingID)
	})

	t.Run("zoom_config nil when neither meeting_id nor zoom_uuid are set", func(t *testing.T) {
		g := ConvertPastMeetingSummaryToGoa(&itx.PastMeetingSummaryResponse{})
		assert.Nil(t, g.ZoomConfig)
	})

	t.Run("summary_data carries start_time, end_time, title, content, and edited_content", func(t *testing.T) {
		resp := &itx.PastMeetingSummaryResponse{
			SummaryStartTime:      "2026-06-01T10:00:00Z",
			SummaryEndTime:        "2026-06-01T11:00:00Z",
			SummaryTitle:          "Board Q2 Summary",
			SummaryOverview:       "Original overview text.",
			EditedSummaryOverview: "Edited overview text.",
		}

		g := ConvertPastMeetingSummaryToGoa(resp)

		require.NotNil(t, g.SummaryData)
		assert.Equal(t, "2026-06-01T10:00:00Z", g.SummaryData.StartTime)
		assert.Equal(t, "2026-06-01T11:00:00Z", g.SummaryData.EndTime)
		assert.Equal(t, "Board Q2 Summary", utils.StringValue(g.SummaryData.Title))
		// Original parts → Content; edited parts → EditedContent. Distinct overviews make a
		// swap detectable.
		assert.Equal(t, "## Overview\nOriginal overview text.", utils.StringValue(g.SummaryData.Content))
		assert.Equal(t, "## Overview\nEdited overview text.", utils.StringValue(g.SummaryData.EditedContent))
		// ITX does not provide doc_url; converter hard-codes StringPtrOmitEmpty("") → nil.
		assert.Nil(t, g.SummaryData.DocURL)
	})

	t.Run("content and edited_content are summaryformat.Format of the parts", func(t *testing.T) {
		resp := &itx.PastMeetingSummaryResponse{
			SummaryOverview: "Team discussed Q1 roadmap.",
			SummaryDetails: []itx.ZoomMeetingSummaryDetails{
				{Label: "Feature Planning", Summary: "Focus on authentication improvements."},
				{Label: "Bug Review", Summary: "Identified 3 critical bugs."},
			},
			NextSteps:             []string{"Schedule design review", "Update roadmap documentation", "Assign bug fixes"},
			EditedSummaryOverview: "Edited overview.",
			EditedSummaryDetails:  []itx.ZoomMeetingSummaryDetails{{Label: "Edited", Summary: "Edited detail."}},
			EditedNextSteps:       []string{"Edited step"},
		}

		g := ConvertPastMeetingSummaryToGoa(resp)

		require.NotNil(t, g.SummaryData)
		// The same bytes the index document carries for these parts.
		assert.Equal(t, indexedSummaryContent, utils.StringValue(g.SummaryData.Content))
		assert.Equal(t, summaryformat.Format(summaryformat.Parts{
			Overview:  resp.EditedSummaryOverview,
			Details:   resp.EditedSummaryDetails,
			NextSteps: resp.EditedNextSteps,
		}), utils.StringValue(g.SummaryData.EditedContent))
	})

	t.Run("empty parts omit content and edited_content", func(t *testing.T) {
		g := ConvertPastMeetingSummaryToGoa(&itx.PastMeetingSummaryResponse{})

		require.NotNil(t, g.SummaryData)
		assert.Nil(t, g.SummaryData.Content)
		assert.Nil(t, g.SummaryData.EditedContent)
	})

	t.Run("GET content round-trips through the PUT parser unchanged", func(t *testing.T) {
		// What Self Serve displays from the proxy (or the index) and sends back unedited
		// must reach ITX as the same parts.
		resp := &itx.PastMeetingSummaryResponse{
			SummaryOverview: "Team discussed Q1 roadmap.",
			SummaryDetails:  []itx.ZoomMeetingSummaryDetails{{Label: "Bug Review", Summary: "Identified 3 critical bugs."}},
			NextSteps:       []string{"Assign bug fixes"},
		}
		content := utils.StringValue(ConvertPastMeetingSummaryToGoa(resp).SummaryData.Content)

		req := ConvertUpdatePastMeetingSummaryPayload(&meetingservice.UpdateItxPastMeetingSummaryPayload{EditedContent: &content})

		require.NotNil(t, req.EditedSummaryOverview)
		assert.Equal(t, resp.SummaryOverview, *req.EditedSummaryOverview)
		require.NotNil(t, req.EditedSummaryDetails)
		assert.Equal(t, resp.SummaryDetails, *req.EditedSummaryDetails)
		require.NotNil(t, req.EditedNextSteps)
		assert.Equal(t, resp.NextSteps, *req.EditedNextSteps)
	})
}
