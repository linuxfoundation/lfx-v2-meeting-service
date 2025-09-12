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

//
// Summary attribute functions
//

// RequiresApprovalAttribute is the DSL attribute for summary approval requirement.
func RequiresApprovalAttribute() {
	Attribute("requires_approval", Boolean, "Whether the summary requires approval", func() {
		Example(false)
	})
}

// ApprovedAttribute is the DSL attribute for summary approval status.
func ApprovedAttribute() {
	Attribute("approved", Boolean, "Whether the summary has been approved", func() {
		Example(true)
	})
}

// EmailSentAttribute is the DSL attribute for email sent status.
func EmailSentAttribute() {
	Attribute("email_sent", Boolean, "Whether summary email has been sent", func() {
		Example(true)
	})
}

// SummaryPasswordAttribute is the DSL attribute for summary password.
func SummaryPasswordAttribute() {
	Attribute("password", String, "Password for accessing the summary (if required)", func() {
		Example("abc123")
	})
}

// SummaryDataAttribute is the DSL attribute for the summary data.
func SummaryDataAttribute() {
	Attribute("summary_data", SummaryData, "The actual summary content")
}

// ZoomConfigSummaryAttribute is the DSL attribute for Zoom configuration in summaries.
func ZoomConfigSummaryAttribute() {
	Attribute("zoom_config", PastMeetingSummaryZoomConfig, "Zoom-specific configuration")
}

//
// Summary data field attributes
//

// SummaryStartTimeAttribute is the DSL attribute for summary start time.
func SummaryStartTimeAttribute() {
	Attribute("start_time", String, "Summary start time", func() {
		Format(FormatDateTime)
		Example("2024-01-15T10:00:00Z")
	})
}

// SummaryEndTimeAttribute is the DSL attribute for summary end time.
func SummaryEndTimeAttribute() {
	Attribute("end_time", String, "Summary end time", func() {
		Format(FormatDateTime)
		Example("2024-01-15T11:00:00Z")
	})
}

// SummaryTitleAttribute is the DSL attribute for summary title.
func SummaryTitleAttribute() {
	Attribute("title", String, "Summary title", func() {
		Example("Weekly Team Standup Meeting")
	})
}

// ContentAttribute is the DSL attribute for the main summary content.
func ContentAttribute() {
	Attribute("content", String, "The main AI-generated summary content", func() {
		Example("This meeting discussed sprint progress, addressed blockers, and outlined next steps for the team.")
	})
}

// DocURLAttribute is the DSL attribute for the summary document URL.
func DocURLAttribute() {
	Attribute("doc_url", String, "URL to the full summary document", func() {
		Example("https://zoom.us/rec/summary/abc123")
	})
}

// EditedContentAttribute is the DSL attribute for edited summary content.
func EditedContentAttribute() {
	Attribute("edited_content", String, "User-edited summary content", func() {
		Example("Updated meeting summary with additional details and action items.")
	})
}

//
// Zoom config attributes for summaries
//

// ZoomSummaryMeetingUUIDAttribute is the DSL attribute for Zoom meeting UUID in summaries.
func ZoomSummaryMeetingUUIDAttribute() {
	Attribute("meeting_uuid", String, "Zoom meeting UUID", func() {
		Example("aDYlohsHRtCd4ii1uC2+hA==")
	})
}

//
// Type definitions
//

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

	ZoomMeetingIDAttribute() // Reuse existing attribute
	ZoomSummaryMeetingUUIDAttribute()
})

// SummaryData represents the actual AI-generated summary content
var SummaryData = Type("SummaryData", func() {
	Description("AI-generated summary content for a past meeting")

	SummaryStartTimeAttribute()
	SummaryEndTimeAttribute()
	SummaryTitleAttribute()
	ContentAttribute()
	DocURLAttribute()
	EditedContentAttribute()

	Required("start_time", "end_time")
})

// PastMeetingSummary represents an AI-generated summary for a past meeting occurrence
var PastMeetingSummary = Type("PastMeetingSummary", func() {
	Description("AI-generated summary for a past meeting occurrence")

	UIDAttribute()
	PastMeetingParticipantPastMeetingUIDAttribute() // Reuse existing - defines "past_meeting_uid" field
	PastMeetingMeetingUIDAttribute()                // Reuse existing - defines "meeting_uid" field
	PlatformAttribute()                             // Reuse existing
	SummaryPasswordAttribute()                      // Use summary-specific password attribute
	ZoomConfigSummaryAttribute()
	SummaryDataAttribute()
	RequiresApprovalAttribute()
	ApprovedAttribute()
	EmailSentAttribute()
	CreatedAtAttribute() // Reuse existing
	UpdatedAtAttribute() // Reuse existing

	Required("uid", "past_meeting_uid", "meeting_uid", "platform", "summary_data",
		"requires_approval", "approved", "email_sent", "created_at", "updated_at")
})
