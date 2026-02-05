// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package design

import (
	. "goa.design/goa/v3/dsl" //nolint:staticcheck // ST1001: the recommended way of using the goa DSL package is with the . import
)

// Shared attribute definitions used by ITX types

func TitleAttribute() {
	Attribute("title", String, "The title of the meeting")
}

func StartTimeAttribute() {
	Attribute("start_time", String, "The start time of the meeting in RFC3339 format", func() {
		Example("2021-01-01T00:00:00Z")
		Format(FormatDateTime)
	})
}

func DurationAttribute() {
	Attribute("duration", Int, "The duration of the meeting in minutes", func() {
		Minimum(0)
		Maximum(600)
	})
}

func TimezoneAttribute() {
	Attribute("timezone", String, "The timezone of the meeting (e.g. 'America/New_York')")
}

func VisibilityAttribute() {
	Attribute("visibility", String, "The visibility of the meeting's existence to other users", func() {
		Enum("public", "private")
	})
}

func DescriptionAttribute() {
	Attribute("description", String, "The description of the meeting", func() {
		MaxLength(2000) // Zoom's Agenda max length
	})
}

func RestrictedAttribute() {
	Attribute("restricted", Boolean, "The restrictedness of joining the meeting (i.e. is the meeting restricted to only invited users or anyone?)")
}

func CommitteesAttribute() {
	Attribute("committees", ArrayOf(Committee), "The committees associated with the meeting")
}

func MeetingTypeAttribute() {
	Attribute("meeting_type", String, "The type of meeting", func() {
		Enum("Board", "Maintainers", "Marketing", "Technical", "Legal", "Other", "None")
	})
}

func EarlyJoinTimeMinutesAttribute() {
	Attribute("early_join_time_minutes", Int, "The number of minutes that users are allowed to join the meeting early", func() {
		Minimum(10)
		Maximum(60)
	})
}

func RecordingEnabledAttribute() {
	Attribute("recording_enabled", Boolean, "Whether recording is enabled for the meeting")
}

func TranscriptEnabledAttribute() {
	Attribute("transcript_enabled", Boolean, "Whether transcription is enabled for the meeting")
}

func YoutubeUploadEnabledAttribute() {
	Attribute("youtube_upload_enabled", Boolean, "Whether automatic youtube uploading is enabled for the meeting")
}

func ArtifactVisibilityAttribute() {
	Attribute("artifact_visibility", String, "The visibility of artifacts to users", func() {
		Enum("meeting_hosts", "meeting_participants", "public")
	})
}

func RecurrenceAttribute() {
	Attribute("recurrence", Recurrence, "The recurrence of the meeting")
}

// Committee represents a committee associated with a meeting
var Committee = Type("Committee", func() {
	Description("A committee associated with a meeting")
	Attribute("uid", String, "Committee UID", func() {
		Example("7cad5a8d-19d0-41a4-81a6-043453daf9ee")
		Format(FormatUUID)
	})
	Attribute("allowed_voting_statuses", ArrayOf(String), "Allowed voting statuses for committee members")
})

// Recurrence represents meeting recurrence settings
var Recurrence = Type("Recurrence", func() {
	Description("Meeting recurrence settings")
	Attribute("type", Int, "Recurrence type: 1=Daily, 2=Weekly, 3=Monthly", func() {
		Enum(1, 2, 3)
		Example(2)
	})
	Attribute("repeat_interval", Int, "Repeat interval")
	Attribute("weekly_days", String, "Days of week for weekly recurrence")
	Attribute("monthly_day", Int, "Day of month for monthly recurrence")
	Attribute("monthly_week", Int, "Week of month for monthly recurrence")
	Attribute("monthly_week_day", Int, "Day of week for monthly recurrence")
	Attribute("end_times", Int, "Number of occurrences")
	Attribute("end_date_time", String, "End date/time in RFC3339", func() {
		Format(FormatDateTime)
	})
})

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

// ITXUser represents a user in the ITX system
var ITXUser = Type("ITXUser", func() {
	Description("User information from ITX")
	Attribute("username", String, "Username", func() {
		Example("jdoe")
	})
	Attribute("name", String, "Full name", func() {
		Example("John Doe")
	})
	Attribute("email", String, "Email address", func() {
		Example("john.doe@example.com")
		Format(FormatEmail)
	})
	Attribute("profile_picture", String, "Profile picture URL", func() {
		Example("https://example.com/avatar.jpg")
		Format(FormatURI)
	})
})

// ITXZoomMeetingRegistrant represents a meeting registrant in ITX
var ITXZoomMeetingRegistrant = Type("ITXZoomMeetingRegistrant", func() {
	Description("Meeting registrant in ITX")

	// Read-only fields
	Attribute("uid", String, "Registrant UID (read-only)")
	Attribute("type", String, "Registrant type: direct or committee (read-only)", func() {
		Enum("direct", "committee")
	})

	// Identity fields
	Attribute("committee_uid", String, "Committee UID (for committee registrants)")
	Attribute("email", String, "Registrant email", func() {
		Format(FormatEmail)
		Example("bobsmith@gmail.com")
	})
	Attribute("username", String, "LF username", func() {
		Example("testuser")
	})

	// Personal info
	Attribute("first_name", String, "First name (required with email)", func() {
		Example("Bob")
	})
	Attribute("last_name", String, "Last name (required with email)", func() {
		Example("Smith")
	})
	Attribute("org", String, "Organization", func() {
		Example("google")
	})
	Attribute("job_title", String, "Job title", func() {
		Example("developer")
	})
	Attribute("profile_picture", String, "Profile picture URL")

	// Meeting settings
	Attribute("host", Boolean, "Access to host key for the meeting")
	Attribute("occurrence", String, "Specific occurrence ID (blank = all occurrences)", func() {
		Example("1666848600")
	})

	// Tracking fields (read-only)
	Attribute("attended_occurrence_count", Int, "Number of meetings attended (read-only)")
	Attribute("total_occurrence_count", Int, "Total meetings registered (read-only)")
	Attribute("last_invite_received_time", String, "Last invite timestamp RFC3339 (read-only)")
	Attribute("last_invite_received_message_id", String, "Last email message ID (read-only)")
	Attribute("last_invite_delivery_status", String, "delivered or failed (read-only)")
	Attribute("last_invite_delivery_description", String, "Delivery status details (read-only)")

	// Audit fields (read-only)
	Attribute("created_at", String, "Creation timestamp RFC3339 (read-only)")
	Attribute("created_by", ITXUser, "Creator user info (read-only)")
	Attribute("modified_at", String, "Last modified timestamp RFC3339 (read-only)")
	Attribute("updated_by", ITXUser, "Last updater user info (read-only)")
})

// ITXZoomMeetingJoinLink represents a join link response from ITX
var ITXZoomMeetingJoinLink = Type("ITXZoomMeetingJoinLink", func() {
	Description("Zoom meeting join link from ITX API proxy")
	Attribute("link", String, "Zoom meeting join URL", func() {
		Example("https://zoom.us/j/1234567891?pwd=NTNubnB4bnpPTm9zT2VLZFJnQ1RkUT11")
		Format(FormatURI)
	})
	Required("link")
})

// ITXPastZoomMeeting represents a past meeting from ITX
var ITXPastZoomMeeting = Type("ITXPastZoomMeeting", func() {
	Description("Past Zoom meeting from ITX API proxy")

	// Identifiers (read-only)
	Attribute("id", String, "Past meeting ID (meeting_id or meeting_id-occurrence_id)", func() {
		Example("12343245463-1630560600000")
	})
	Attribute("meeting_id", String, "Zoom meeting ID", func() {
		Example("12343245463")
	})
	Attribute("occurrence_id", String, "Zoom occurrence ID (Unix timestamp)", func() {
		Example("1630560600000")
	})

	// Project association
	Attribute("project_uid", String, "LF project UID", func() {
		Example("a1234567-89ab-cdef-0123-456789abcdef")
		Format(FormatUUID)
	})

	// Meeting details
	Attribute("title", String, "Meeting title")
	Attribute("description", String, "Meeting description/agenda")
	Attribute("start_time", String, "Meeting start time (RFC3339)", func() {
		Format(FormatDateTime)
		Example("2021-06-27T05:30:00Z")
	})
	Attribute("duration", Int, "Meeting duration in minutes")
	Attribute("timezone", String, "Meeting timezone", func() {
		Example("America/Los_Angeles")
	})
	Attribute("visibility", String, "Meeting visibility", func() {
		Enum("public", "private")
	})
	Attribute("restricted", Boolean, "Whether meeting was restricted to invited users only")
	Attribute("meeting_type", String, "Type of meeting", func() {
		Enum("Board", "Maintainers", "Marketing", "Technical", "Legal", "Other", "None")
	})

	// Committee association
	Attribute("committees", ArrayOf(Committee), "Committees associated with the past meeting")

	// Recording/Transcript settings
	Attribute("recording_enabled", Boolean, "Whether recording was enabled")
	Attribute("artifact_visibility", String, "Who has access to meeting artifacts", func() {
		Enum("meeting_hosts", "meeting_participants", "public")
	})
	Attribute("transcript_enabled", Boolean, "Whether transcription was enabled")

	// Metadata
	Attribute("is_manually_created", Boolean, "Whether past meeting was manually created")
})

