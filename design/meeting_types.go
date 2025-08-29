// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package design

import (
	. "goa.design/goa/v3/dsl" //nolint:staticcheck // ST1001: the recommended way of using the goa DSL package is with the . import
)

// CreateMeetingPayload represents the payload for creating a meeting
var CreateMeetingPayload = Type("CreateMeetingPayload", func() {
	Description("Payload for creating a new meeting")
	ProjectUIDAttribute()
	StartTimeAttribute()
	DurationAttribute()
	TimezoneAttribute()
	RecurrenceAttribute()
	TitleAttribute()
	DescriptionAttribute()
	CommitteesAttribute()
	PlatformAttribute()
	EarlyJoinTimeMinutesAttribute()
	MeetingTypeAttribute()
	VisibilityAttribute()
	RestrictedAttribute()
	ArtifactVisibilityAttribute()
	RecordingEnabledAttribute()
	TranscriptEnabledAttribute()
	YoutubeUploadEnabledAttribute()
	ZoomConfigPostAttribute()
	MeetingOrganizersAttribute()
	Required("project_uid", "start_time", "duration", "timezone", "title", "description")
})

// UpdateMeetingPayload represents the payload for updating a meeting
var UpdateMeetingPayload = Type("UpdateMeetingPayload", func() {
	Description("Payload for updating an existing meeting")
	MeetingUIDAttribute()
	ProjectUIDAttribute()
	StartTimeAttribute()
	DurationAttribute()
	TimezoneAttribute()
	RecurrenceAttribute()
	TitleAttribute()
	DescriptionAttribute()
	CommitteesAttribute()
	PlatformAttribute()
	EarlyJoinTimeMinutesAttribute()
	MeetingTypeAttribute()
	VisibilityAttribute()
	RestrictedAttribute()
	ArtifactVisibilityAttribute()
	RecordingEnabledAttribute()
	TranscriptEnabledAttribute()
	YoutubeUploadEnabledAttribute()
	ZoomConfigPostAttribute()
	Required("uid", "project_uid", "start_time", "duration", "timezone", "title", "description")
})

// MeetingFull is the DSL type for a full representation of a meeting.
var MeetingFull = Type("MeetingFull", func() {
	Description("A full representation of LF Meetings with sub-objects populated.")

	MeetingBaseAttributes()
	MeetingSettingsAttributes()
})

// MeetingBase represents a base representation of a meeting.
var MeetingBase = Type("MeetingBase", func() {
	Description("A base representation of a meeting.")
	MeetingBaseAttributes()
})

func MeetingBaseAttributes() {
	MeetingUIDAttribute()
	ProjectUIDAttribute()
	StartTimeAttribute()
	DurationAttribute()
	TimezoneAttribute()
	RecurrenceAttribute()
	TitleAttribute()
	DescriptionAttribute()
	CommitteesAttribute()
	PlatformAttribute()
	EarlyJoinTimeMinutesAttribute()
	MeetingTypeAttribute()
	VisibilityAttribute()
	RestrictedAttribute()
	ArtifactVisibilityAttribute()
	JoinURLAttribute()
	PublicLinkAttribute()
	EmailDeliveryErrorCountAttribute()
	RecordingEnabledAttribute()
	TranscriptEnabledAttribute()
	YoutubeUploadEnabledAttribute()
	ZoomConfigFullAttribute()
	RegistrantCountAttribute()
	RegistrantResponseDeclinedCountAttribute()
	RegistrantResponseAcceptedCountAttribute()
	OccurrencesAttribute()
	CreatedAtAttribute()
	UpdatedAtAttribute()
}

// ProjectSettings is the DSL type for a meeting settings.
var MeetingSettings = Type("MeetingSettings", func() {
	Description("A representation of LF Meeting settings.")
	MeetingSettingsAttributes()
})

// MeetingSettingsAttributes is the DSL attributes for a meeting settings.
func MeetingSettingsAttributes() {
	MeetingUIDAttribute()
	MeetingOrganizersAttribute()
	CreatedAtAttribute()
	UpdatedAtAttribute()
}

// Committee represents a committee associated with a meeting
var Committee = Type("Committee", func() {
	Description("Committee data for association with meeting")
	Field(1, "uid", String, "The UID of the committee")
	Field(2, "allowed_voting_statuses", ArrayOf(String), "The committee voting statuses required for committee members to be added to the meeting")
	Required("uid", "allowed_voting_statuses")
})

// Recurrence represents the recurrence of a meeting
var Recurrence = Type("Recurrence", func() {
	Description("Meeting recurrence object")
	Field(1, "type", Int, "The recurrence type", func() {
		Enum(1, 2, 3)
	})
	Field(2, "repeat_interval", Int, func() {
		Description(`Define the interval at which the meeting should recur. 
For instance, if you would like to schedule a meeting that recurs every two months, 
you must set the value of this field as '2' and the value of the 'type' parameter as '3'. 
For a daily meeting, the maximum interval you can set is '90' days. 
For a weekly meeting the maximum interval that you can set is of '12' weeks. 
For a monthly meeting, there is a maximum of '3' months.`)
	})
	Field(3, "weekly_days", String, func() {
		Description(`This field is required if you're scheduling a recurring meeting of type '2' to state which day(s) 
of the week the meeting should repeat. The value for this field could be a number between '1' to '7' in string format. 
For instance, if the meeting should recur on Sunday, provide '1' as the value of this field. 
If you would like the meeting to occur on multiple days of a week, you should provide comma separated values for this field. 
For instance, if the meeting should recur on Sundays and Tuesdays provide '1,3' as the value of this field. 
1 - Sunday
2 - Monday
3 - Tuesday
4 - Wednesday
5 - Thursday
6 - Friday
7 - Saturday`)
		Pattern(`^[1-7](,[1-7])*$`)
		Example("1,3,5")
	})
	Field(4, "monthly_day", Int, func() {
		Description("Use this field only if you're scheduling a recurring meeting of type '3' to state which day in a month, the meeting should recur. The value range is from 1 to 31. For instance, if you would like the meeting to recur on 23rd of each month, provide '23' as the value of this field and '1' as the value of the 'repeat_interval' field. Instead, if you would like the meeting to recur every three months, on 23rd of the month, change the value of the 'repeat_interval' field to '3'.")
		Minimum(1)
		Maximum(31)
	})
	Field(5, "monthly_week", Int, func() {
		Description("Use this field only if you're scheduling a recurring meeting of type '3' to state the week of the month when the meeting should recur. If you use this field, you must also use the 'monthly_week_day' field to state the day of the week when the meeting should recur. '-1' - Last week of the month. 1 - First week of the month. 2 - Second week of the month. 3 - Third week of the month. 4 - Fourth week of the month.")
		Enum(-1, 1, 2, 3, 4)
	})
	Field(6, "monthly_week_day", Int, func() {
		Description("Use this field only if you're scheduling a recurring meeting of type '3' to state a specific day in a week when the monthly meeting should recur. To use this field, you must also use the 'monthly_week' field. 1 - Sunday 2 - Monday 3 - Tuesday 4 - Wednesday 5 - Thursday 6 - Friday 7 - Saturday")
		Enum(1, 2, 3, 4, 5, 6, 7)
	})
	Field(7, "end_times", Int, func() {
		Description("Select how many times the meeting should recur before it is canceled. Cannot be used with 'end_date_time'.")
	})
	Field(8, "end_date_time", String, func() {
		Description("Select the final date on which the meeting will recur before it is canceled. Cannot be used with 'end_times'. should be in GMT. should be in 'yyyy-MM-ddTHH:mm:ssZ' format.")
		Format(FormatDateTime)
	})
	Required("type", "repeat_interval")
})

// Occurrence represents a single occurrence of a recurring meeting (read-only from Zoom API)
var Occurrence = Type("Occurrence", func() {
	Description("Meeting occurrence object - read-only data from platform API")
	Attribute("occurrence_id", String, "ID of the occurrence, also the start time in unix time", func() {
		Example("1640995200") // Unix timestamp
	})
	Attribute("start_time", String, "GMT start time of occurrence", func() {
		Format(FormatDateTime)
		Example("2021-01-01T10:00:00Z")
	})
	Attribute("title", String, "Meeting title for this occurrence")
	Attribute("description", String, "Meeting description for this occurrence")
	Attribute("duration", Int, "Occurrence duration in minutes")
	Attribute("recurrence", Recurrence, "The recurrence pattern for this occurrence onwards if there is one")
	Attribute("registrant_count", Int, "Number of registrants for this meeting occurrence")
	Attribute("response_count_no", Int, "Number of registrants who declined the invite for this occurrence")
	Attribute("response_count_yes", Int, "Number of registrants who accepted the invite for this occurrence")
	Attribute("is_cancelled", Boolean, "Whether the occurrence is cancelled")
})

// MeetingOrganizersAttribute is the DSL attribute for meeting organizers.
func MeetingOrganizersAttribute() {
	Attribute("organizers", ArrayOf(String), func() {
		Description("The organizers of the meeting. This is a list of LFIDs of the meeting organizers.")
	})
}

// MeetingUIDAttribute is the DSL attribute for meeting UID.
func MeetingUIDAttribute() {
	Attribute("uid", String, "The UID of the meeting", func() {
		Example("7cad5a8d-19d0-41a4-81a6-043453daf9ee")
		Format(FormatUUID)
	})
}

// StartTimeAttribute is the DSL attribute for start time.
func StartTimeAttribute() {
	Attribute("start_time", String, "The start time of the meeting in RFC3339 format", func() {
		Example("2021-01-01T00:00:00Z")
		Format(FormatDateTime)
	})
}

// DurationAttribute is the DSL attribute for duration.
func DurationAttribute() {
	Attribute("duration", Int, "The duration of the meeting in minutes", func() {
		Minimum(0)
		Maximum(600)
	})
}

// ProjectUIDAttribute is the DSL attribute for project UID.
func ProjectUIDAttribute() {
	Attribute("project_uid", String, "The UID of the LF project", func() {
		Example("7cad5a8d-19d0-41a4-81a6-043453daf9ee")
		Format(FormatUUID)
	})
}

// TimezoneAttribute is the DSL attribute for timezone.
func TimezoneAttribute() {
	Attribute("timezone", String, "The timezone of the meeting (e.g. 'America/New_York')")
}

// RecurrenceAttribute is the DSL attribute for recurrence.
func RecurrenceAttribute() {
	Attribute("recurrence", Recurrence, "The recurrence of the meeting")
}

// TitleAttribute is the DSL attribute for title.
func TitleAttribute() {
	Attribute("title", String, "The title of the meeting")
}

// DescriptionAttribute is the DSL attribute for description.
func DescriptionAttribute() {
	Attribute("description", String, "The description of the meeting")
}

// CommitteesAttribute is the DSL attribute for committees.
func CommitteesAttribute() {
	Attribute("committees", ArrayOf(Committee), "The committees associated with the meeting")
}

// PlatformAttribute is the DSL attribute for platform.
func PlatformAttribute() {
	Attribute("platform", String, "The platform name of where the meeting is hosted", func() {
		Enum("Zoom")
	})
}

// EarlyJoinTimeMinutesAttribute is the DSL attribute for early join time.
func EarlyJoinTimeMinutesAttribute() {
	Attribute("early_join_time_minutes", Int, "The number of minutes that users are allowed to join the meeting early without being kicked out", func() {
		Minimum(10)
		Maximum(60)
	})
}

// MeetingTypeAttribute is the DSL attribute for meeting type.
func MeetingTypeAttribute() {
	Attribute("meeting_type", String, "The type of meeting. This is usually dependent on the committee(s) associated with the meeting", func() {
		Enum("Board", "Maintainers", "Marketing", "Technical", "Legal", "Other", "None")
	})
}

// VisibilityAttribute is the DSL attribute for visibility.
func VisibilityAttribute() {
	Attribute("visibility", String, "The visibility of the meeting's existence to other users", func() {
		Enum("public", "private")
	})
}

// RestrictedAttribute is the DSL attribute for restricted.
func RestrictedAttribute() {
	Attribute("restricted", Boolean, "The restrictedness of joining the meeting (i.e. is the meeting restricted to only invited users or anyone?)")
}

// ArtifactVisibilityAttribute is the DSL attribute for artifact visibility.
func ArtifactVisibilityAttribute() {
	Attribute("artifact_visibility", String, "The visibility of artifacts to users (e.g. public, only for registrants, only for hosts)", func() {
		Enum("meeting_hosts", "meeting_participants", "public")
	})
}

// JoinURLAttribute is the DSL attribute for public link. It is a read-only attribute.
func JoinURLAttribute() {
	Attribute("join_url", String, func() {
		Description("The public join URL for participants to join the meeting via the LFX platform (e.g. 'https://zoom-lfx.platform.linuxfoundation.org/meeting/12343245463')")
		Format(FormatURI)
	})
}

// PublicLinkAttribute is the DSL attribute for public link. It is a read-only attribute.
func PublicLinkAttribute() {
	Attribute("public_link", String, func() {
		Description("The public join URL for participants to join the meeting via the LFX platform (e.g. 'https://zoom-lfx.platform.linuxfoundation.org/meeting/12343245463')")
		Format(FormatURI)
	})
}

// PasswordAttribute is the DSL attribute for password.
func PasswordAttribute() {
	Attribute("password", String, "Unique, non-guessable, password for the meeting - is needed to join a meeting and is included in invites", func() {
		Format(FormatUUID)
	})
}

// EmailDeliveryErrorCountAttribute is the DSL attribute for email delivery error count.
func EmailDeliveryErrorCountAttribute() {
	Attribute("email_delivery_error_count", Int, func() {
		Description("The number of registrants that have an email delivery error with their invite. The delivery errors are counted as the last invite that was sent to the registrant, so if a registrant previously had a delivery error but not in their most recent invite received, then it does not count towards this field value.")
	})
}

// RecordingEnabledAttribute is the DSL attribute for recording enabled.
func RecordingEnabledAttribute() {
	Attribute("recording_enabled", Boolean, "Whether recording is enabled for the meeting")
}

// TranscriptEnabledAttribute is the DSL attribute for transcript enabled.
func TranscriptEnabledAttribute() {
	Attribute("transcript_enabled", Boolean, "Whether transcription is enabled for the meeting")
}

// YoutubeUploadEnabledAttribute is the DSL attribute for YouTube upload.
func YoutubeUploadEnabledAttribute() {
	Attribute("youtube_upload_enabled", Boolean, "Whether automatic youtube uploading is enabled for the meeting")
}

// RegistrantCountAttribute is the DSL attribute for registrant count.
func RegistrantCountAttribute() {
	// Read-only attribute
	Attribute("registrant_count", Int, "The number of registrants for the meeting")
}

// RegistrantResponseDeclinedCountAttribute is the DSL attribute for registrant response declined count.
func RegistrantResponseDeclinedCountAttribute() {
	// Read-only attribute
	Attribute("registrant_response_declined_count", Int, "The number of registrants that have declined the meeting invitation")
}

// RegistrantResponseAcceptedCountAttribute is the DSL attribute for registrant response accepted count.
func RegistrantResponseAcceptedCountAttribute() {
	// Read-only attribute
	Attribute("registrant_response_accepted_count", Int, "The number of registrants that have accepted the meeting invitation")
}

// OccurrencesAttribute is the DSL attribute for meeting occurrences.
func OccurrencesAttribute() {
	// Read-only attribute
	Attribute("occurrences", ArrayOf(Occurrence), "Array of meeting occurrences (read-only from platform API)")
}

//
// Zoom platform attributes
//

// ZoomMeetingIDAttribute is the DSL attribute for Zoom meeting ID.
func ZoomMeetingIDAttribute() {
	Attribute("meeting_id", String, "The ID of the created meeting in Zoom", func() {
		Pattern(`^\d{9,11}$`) // Zoom meeting IDs are 9-11 digits
		Example("1234567890")
		MinLength(9)
		MaxLength(11)
	})
}

// ZoomHostKeyAttribute is the DSL attribute for Zoom host key.
func ZoomHostKeyAttribute() {
	Attribute("host_key", String, "The host key of the created meeting in Zoom", func() {
		Pattern(`^\d{6}$`) // Zoom host keys are exactly 6 digits
		Example("123456")
		MinLength(6)
		MaxLength(6)
	})
}

// ZoomMeetingPasscodeAttribute is the DSL attribute for Zoom meeting passcode.
func ZoomMeetingPasscodeAttribute() {
	Attribute("passcode", String, func() {
		Description("The zoom-defined passcode for the meeting. Required if joining via dial-in, or by clicking 'join meeting' in the zoom client & putting in the meeting id and passcode.")
		Pattern(`^\d{6,10}$`) // Zoom meeting passcodes are 6-10 digits, cannot be consecutive
		Example("147258")
		MinLength(6)
		MaxLength(10)
	})
}

// ZoomAICompanionEnabledAttribute is the DSL attribute for Zoom AI companion.
func ZoomAICompanionEnabledAttribute() {
	Attribute("ai_companion_enabled", Boolean, "For zoom platform meetings: whether Zoom AI companion is enabled")
}

// ZoomAISummaryRequireApprovalAttribute is the DSL attribute for Zoom AI summary require approval.
func ZoomAISummaryRequireApprovalAttribute() {
	Attribute("ai_summary_require_approval", Boolean, "For zoom platform meetings: whether AI summary approval is required")
}

// ZoomConfigPost represents the meeting attributes specific to Zoom platform that are writable.
var ZoomConfigPost = Type("ZoomConfigPost", func() {
	Description("Meeting attributes specific to Zoom platform that are writable")
	ZoomAICompanionEnabledAttribute()
	ZoomAISummaryRequireApprovalAttribute() // This relates to approvals in the LFX system about Zoom meeting AI summaries.
})

// ZoomConfigFull represents the meeting attributes specific to Zoom platform that are either writable or read-only.
var ZoomConfigFull = Type("ZoomConfigFull", func() {
	Description("Meeting attributes specific to Zoom platform that contain both writable and read-only attributes")
	ZoomMeetingIDAttribute()       // Read-only attribute
	ZoomMeetingPasscodeAttribute() // Read-only attribute
	ZoomAICompanionEnabledAttribute()
	ZoomAISummaryRequireApprovalAttribute() // This relates to approvals in the LFX system about Zoom meeting AI summaries.
})

// ZoomConfigPostAttribute is the DSL attribute for Zoom configuration.
func ZoomConfigPostAttribute() {
	Attribute("zoom_config", ZoomConfigPost, "For zoom platform meetings: the configuration for the meeting")
}

// ZoomConfigFullAttribute is the DSL attribute for Zoom configuration.
func ZoomConfigFullAttribute() {
	Attribute("zoom_config", ZoomConfigFull, "For zoom platform meetings: the configuration for the meeting")
}
