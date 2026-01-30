// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package design

import (
	. "goa.design/goa/v3/dsl" //nolint:staticcheck // ST1001: the recommended way of using the goa DSL package is with the . import
)

// ITXProjectUIDAttribute is the DSL attribute for project UID.
func ITXProjectUIDAttribute() {
	Attribute("project_uid", String, "The UID of the LF project", func() {
		Example("7cad5a8d-19d0-41a4-81a6-043453daf9ee")
	})
}

// ITXZoomMeetingResponse represents the response from creating a Zoom meeting via ITX proxy
var ITXZoomMeetingResponse = Type("ITXZoomMeetingResponse", func() {
	Description("Response from creating a Zoom meeting through ITX API proxy")

	// Request fields echoed back
	ITXProjectUIDAttribute()
	TitleAttribute()
	StartTimeAttribute()
	DurationAttribute()
	TimezoneAttribute()
	VisibilityAttribute()
	DescriptionAttribute()
	RestrictedAttribute()
	CommitteesAttribute()
	MeetingTypeAttribute()
	EarlyJoinTimeMinutesAttribute()
	RecordingEnabledAttribute()
	TranscriptEnabledAttribute()
	YoutubeUploadEnabledAttribute()
	ArtifactVisibilityAttribute()
	RecurrenceAttribute()

	// Read-only response fields from ITX
	Attribute("id", String, "Zoom meeting ID from ITX", func() {
		Example("1234567890")
	})
	Attribute("host_key", String, "6-digit host key", func() {
		Example("123456")
	})
	Attribute("passcode", String, "Zoom meeting passcode", func() {
		Example("abc123")
	})
	Attribute("password", String, "UUID password for join page", func() {
		Example("7cad5a8d-19d0-41a4-81a6-043453daf9ee")
		Format(FormatUUID)
	})
	Attribute("public_link", String, "Public meeting join URL", func() {
		Example("https://zoom-lfx.platform.linuxfoundation.org/meeting/1234567890")
		Format(FormatURI)
	})
	Attribute("created_at", String, "Creation timestamp (RFC3339)", func() {
		Example("2021-01-01T00:00:00Z")
		Format(FormatDateTime)
	})
	Attribute("modified_at", String, "Last modification timestamp (RFC3339)", func() {
		Example("2021-01-01T00:00:00Z")
		Format(FormatDateTime)
	})
	Attribute("occurrences", ArrayOf(ITXOccurrence), "Meeting occurrences (for recurring)")
	Attribute("registrant_count", Int, "Number of registrants")
})

// ITXOccurrence represents a single occurrence from ITX response
var ITXOccurrence = Type("ITXOccurrence", func() {
	Description("Meeting occurrence from ITX")
	Attribute("occurrence_id", String, "Unix timestamp", func() {
		Example("1640995200")
	})
	Attribute("start_time", String, "RFC3339 start time", func() {
		Example("2021-01-01T10:00:00Z")
		Format(FormatDateTime)
	})
	Attribute("duration", Int, "Duration in minutes")
	Attribute("status", String, "available or cancel", func() {
		Enum("available", "cancel")
	})
	Attribute("registrant_count", Int, "Number of registrants for this occurrence")
})

// ITXMeetingCountResponse represents the response from getting meeting count via ITX proxy
var ITXMeetingCountResponse = Type("ITXMeetingCountResponse", func() {
	Description("Response from getting meeting count through ITX API proxy")
	Attribute("meeting_count", Int, "Number of meetings for the project", func() {
		Example(42)
	})
	Required("meeting_count")
})

// ForbiddenError is the DSL type for a forbidden error (403).
var ForbiddenError = Type("ForbiddenError", func() {
	Attribute("code", String, "HTTP status code", func() {
		Example("403")
	})
	Attribute("message", String, "Error message", func() {
		Example("Access forbidden.")
	})
	Required("code", "message")
})
