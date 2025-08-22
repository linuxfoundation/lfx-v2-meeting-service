// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package design

import (
	. "goa.design/goa/v3/dsl" //nolint:staticcheck // ST1001: the recommended way of using the goa DSL package is with the goa DSL
)

// CreatePastMeetingPayload represents the payload for creating a past meeting
var CreatePastMeetingPayload = Type("CreatePastMeetingPayload", func() {
	Description("Payload for creating a new past meeting record")
	PastMeetingMeetingUIDAttribute()
	PastMeetingOccurrenceIDAttribute()
	ProjectUIDAttribute()
	PastMeetingScheduledStartTimeAttribute()
	PastMeetingScheduledEndTimeAttribute()
	DurationAttribute()
	TimezoneAttribute()
	RecurrenceAttribute()
	TitleAttribute()
	DescriptionAttribute()
	CommitteesAttribute()
	PlatformAttribute()
	PastMeetingPlatformMeetingIDAttribute()
	EarlyJoinTimeMinutesAttribute()
	MeetingTypeAttribute()
	VisibilityAttribute()
	RestrictedAttribute()
	ArtifactVisibilityAttribute()
	PublicLinkAttribute()
	RecordingEnabledAttribute()
	TranscriptEnabledAttribute()
	YoutubeUploadEnabledAttribute()
	ZoomConfigFullAttribute()
	PastMeetingSessionsAttribute()
	Required("meeting_uid", "project_uid", "scheduled_start_time", "scheduled_end_time", "duration", "timezone", "title", "description", "platform", "restricted", "recording_enabled", "transcript_enabled", "youtube_upload_enabled")
})

// PastMeeting is the DSL type for a past meeting record.
var PastMeeting = Type("PastMeeting", func() {
	Description("A record of a meeting that has occurred in the past.")
	PastMeetingAttributes()
})

func PastMeetingAttributes() {
	PastMeetingUIDAttribute()
	PastMeetingMeetingUIDAttribute()
	PastMeetingOccurrenceIDAttribute()
	ProjectUIDAttribute()
	PastMeetingScheduledStartTimeAttribute()
	PastMeetingScheduledEndTimeAttribute()
	DurationAttribute()
	TimezoneAttribute()
	RecurrenceAttribute()
	TitleAttribute()
	DescriptionAttribute()
	CommitteesAttribute()
	PlatformAttribute()
	PastMeetingPlatformMeetingIDAttribute()
	EarlyJoinTimeMinutesAttribute()
	MeetingTypeAttribute()
	VisibilityAttribute()
	RestrictedAttribute()
	ArtifactVisibilityAttribute()
	PublicLinkAttribute()
	RecordingEnabledAttribute()
	TranscriptEnabledAttribute()
	YoutubeUploadEnabledAttribute()
	ZoomConfigFullAttribute()
	PastMeetingSessionsAttribute()
	CreatedAtAttribute()
	UpdatedAtAttribute()
}

// Session represents a single start/end session of a meeting
var Session = Type("Session", func() {
	Description("A single start/end session of a meeting on the platform")
	Attribute("uid", String, "The unique identifier of the session", func() {
		Example("session-123")
		Format(FormatUUID)
	})
	Attribute("start_time", String, "The start time of the session", func() {
		Example("2021-01-01T10:00:00Z")
		Format(FormatDateTime)
	})
	Attribute("end_time", String, "The end time of the session (may be null if session is ongoing)", func() {
		Example("2021-01-01T11:00:00Z")
		Format(FormatDateTime)
	})
	Required("uid", "start_time")
})

// PastMeetingUIDAttribute is the DSL attribute for past meeting UID.
func PastMeetingUIDAttribute() {
	Attribute("uid", String, "The unique identifier of the past meeting", func() {
		Example("7cad5a8d-19d0-41a4-81a6-043453daf9ee")
		Format(FormatUUID)
	})
}

// PastMeetingMeetingUIDAttribute is the DSL attribute for the original meeting UID.
func PastMeetingMeetingUIDAttribute() {
	Attribute("meeting_uid", String, "The UID of the original meeting", func() {
		Example("7cad5a8d-19d0-41a4-81a6-043453daf9ee")
		Format(FormatUUID)
	})
}

// PastMeetingOccurrenceIDAttribute is the DSL attribute for occurrence ID.
func PastMeetingOccurrenceIDAttribute() {
	Attribute("occurrence_id", String, "The occurrence ID for recurring meetings", func() {
		Example("1640995200")
	})
}

// PastMeetingScheduledStartTimeAttribute is the DSL attribute for scheduled start time.
func PastMeetingScheduledStartTimeAttribute() {
	Attribute("scheduled_start_time", String, "The scheduled start time of the past meeting", func() {
		Example("2021-01-01T10:00:00Z")
		Format(FormatDateTime)
	})
}

// PastMeetingScheduledEndTimeAttribute is the DSL attribute for scheduled end time.
func PastMeetingScheduledEndTimeAttribute() {
	Attribute("scheduled_end_time", String, "The scheduled end time of the past meeting", func() {
		Example("2021-01-01T11:00:00Z")
		Format(FormatDateTime)
	})
}

// PastMeetingPlatformMeetingIDAttribute is the DSL attribute for platform meeting ID.
func PastMeetingPlatformMeetingIDAttribute() {
	Attribute("platform_meeting_id", String, "The ID of the meeting in the platform (e.g. Zoom meeting ID)", func() {
		Example("1234567890")
	})
}

// PastMeetingSessionsAttribute is the DSL attribute for meeting sessions.
func PastMeetingSessionsAttribute() {
	Attribute("sessions", ArrayOf(Session), "Array of meeting sessions (start/end times)", func() {
		Description("Sessions represent individual start/end periods if a meeting was stopped and restarted")
	})
}
