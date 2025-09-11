// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package design

import (
	. "goa.design/goa/v3/dsl" //nolint:staticcheck // ST1001: the recommended way of using the goa DSL package is with the . import
)

// SummaryUIDAttribute is the DSL attribute for the summary UID.
func SummaryUIDAttribute() {
	Attribute("summary_uid", String, "The unique identifier of the summary", func() {
		Example("456e7890-e89b-12d3-a456-426614174000")
		Format(FormatUUID)
	})
}

// PastMeetingSummaryPastMeetingUIDAttribute is the DSL attribute for the past meeting UID in the context of summaries.
func PastMeetingSummaryPastMeetingUIDAttribute() {
	Attribute("past_meeting_uid", String, "The unique identifier of the past meeting", func() {
		Example("123e4567-e89b-12d3-a456-426614174000")
		Format(FormatUUID)
	})
}

// SummaryDetail represents a detailed summary item with label and content
var SummaryDetail = Type("SummaryDetail", func() {
	Description("Detailed summary item with label and content")

	Attribute("label", String, "Summary label", func() {
		Example("Meeting Summary Label")
	})
	Attribute("summary", String, "Summary content", func() {
		Example("Meeting summary details")
	})

	Required("label", "summary")
})

// PastMeetingSummaryZoomConfig represents Zoom-specific configuration for a past meeting summary
var PastMeetingSummaryZoomConfig = Type("PastMeetingSummaryZoomConfig", func() {
	Description("Zoom-specific configuration for a past meeting summary")

	Attribute("meeting_id", String, "Zoom meeting ID", func() {
		Example("85072380123")
	})
	Attribute("meeting_uuid", String, "Zoom meeting UUID", func() {
		Example("aDYlohsHRtCd4ii1uC2+hA==")
	})
})

// SummaryData represents the actual AI-generated summary content
var SummaryData = Type("SummaryData", func() {
	Description("AI-generated summary content for a past meeting")

	Attribute("start_time", String, "Summary start time", func() {
		Format(FormatDateTime)
		Example("2024-01-15T10:00:00Z")
	})
	Attribute("end_time", String, "Summary end time", func() {
		Format(FormatDateTime)
		Example("2024-01-15T11:00:00Z")
	})
	Attribute("title", String, "Summary title", func() {
		Example("Weekly Team Standup Meeting")
	})
	Attribute("overview", String, "Summary overview", func() {
		Example("Discussion of sprint progress and blockers")
	})
	Attribute("next_steps", ArrayOf(String), "Next steps from the meeting", func() {
		Example([]string{"Complete API documentation", "Review PR #123"})
	})
	Attribute("details", ArrayOf(SummaryDetail), "Structured summary details", func() {
		Example([]map[string]interface{}{
			{"label": "Discussion Points", "summary": "Key topics discussed during the meeting"},
		})
	})
	Attribute("edited_overview", String, "Edited summary overview", func() {
		Example("Updated discussion notes with action items")
	})
	Attribute("edited_details", ArrayOf(SummaryDetail), "Edited structured summary details", func() {
		Example([]map[string]interface{}{
			{"label": "Meeting Summary Label", "summary": "Meeting summary details"},
		})
	})
	Attribute("edited_next_steps", ArrayOf(String), "Edited next steps", func() {
		Example([]string{"Updated: Complete API documentation by Friday"})
	})

	Required("start_time", "end_time")
})

// PastMeetingSummary represents an AI-generated summary for a past meeting occurrence
var PastMeetingSummary = Type("PastMeetingSummary", func() {
	Description("AI-generated summary for a past meeting occurrence")

	Attribute("uid", String, "Unique identifier for the summary", func() {
		Format(FormatUUID)
		Example("123e4567-e89b-12d3-a456-426614174000")
	})
	Attribute("past_meeting_uid", String, "UID of the associated past meeting", func() {
		Format(FormatUUID)
		Example("456e7890-e89b-12d3-a456-426614174000")
	})
	Attribute("meeting_uid", String, "UID of the original meeting", func() {
		Format(FormatUUID)
		Example("789e0123-e89b-12d3-a456-426614174000")
	})
	Attribute("platform", String, "Meeting platform", func() {
		Example("Zoom")
		Enum("Zoom", "Teams", "Webex")
	})
	Attribute("password", String, "Password for accessing the summary (if required)", func() {
		Example("abc123")
	})
	Attribute("zoom_config", PastMeetingSummaryZoomConfig, "Zoom-specific configuration")
	Attribute("summary_data", SummaryData, "The actual summary content")
	Attribute("requires_approval", Boolean, "Whether the summary requires approval", func() {
		Example(false)
	})
	Attribute("approved", Boolean, "Whether the summary has been approved", func() {
		Example(true)
	})
	Attribute("email_sent", Boolean, "Whether summary email has been sent", func() {
		Example(true)
	})
	CreatedAtAttribute()
	UpdatedAtAttribute()

	Required("uid", "past_meeting_uid", "meeting_uid", "platform", "summary_data",
		"requires_approval", "approved", "email_sent", "created_at", "updated_at")
})
