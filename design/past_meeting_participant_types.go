// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package design

import (
	. "goa.design/goa/v3/dsl" //nolint:staticcheck // ST1001: the recommended way of using the goa DSL package is with the . import
)

// CreatePastMeetingParticipantPayload represents the payload for creating a past meeting participant
var CreatePastMeetingParticipantPayload = Type("CreatePastMeetingParticipantPayload", func() {
	Description("Payload for creating a new past meeting participant")
	PastMeetingParticipantPastMeetingUIDAttribute()
	RegistrantEmailAttribute()
	RegistrantFirstNameAttribute()
	RegistrantLastNameAttribute()
	RegistrantHostAttribute()
	RegistrantJobTitleAttribute()
	RegistrantOrgNameAttribute()
	RegistrantAvatarURLAttribute()
	RegistrantUsernameAttribute()
	PastMeetingParticipantIsInvitedAttribute()
	PastMeetingParticipantIsAttendedAttribute()
	Required("past_meeting_uid", "email", "first_name", "last_name")
})

// UpdatePastMeetingParticipantPayload represents the payload for updating a past meeting participant
var UpdatePastMeetingParticipantPayload = Type("UpdatePastMeetingParticipantPayload", func() {
	Description("Payload for updating an existing past meeting participant")
	PastMeetingParticipantPastMeetingUIDAttribute()
	RegistrantEmailAttribute()
	RegistrantFirstNameAttribute()
	RegistrantLastNameAttribute()
	RegistrantHostAttribute()
	RegistrantJobTitleAttribute()
	RegistrantOrgNameAttribute()
	RegistrantAvatarURLAttribute()
	RegistrantUsernameAttribute()
	PastMeetingParticipantIsInvitedAttribute()
	PastMeetingParticipantIsAttendedAttribute()
	Required("past_meeting_uid", "email", "first_name", "last_name")
})

// PastMeetingParticipant represents a past meeting participant
var PastMeetingParticipant = Type("PastMeetingParticipant", func() {
	Description("Past meeting participant object")
	PastMeetingParticipantUIDAttribute()
	PastMeetingParticipantPastMeetingUIDAttribute()
	RegistrantMeetingUIDAttribute()
	RegistrantEmailAttribute()
	RegistrantFirstNameAttribute()
	RegistrantLastNameAttribute()
	RegistrantHostAttribute()
	RegistrantJobTitleAttribute()
	RegistrantOrgNameAttribute()
	RegistrantOrgIsMemberAttribute()
	RegistrantOrgIsProjectMemberAttribute()
	RegistrantAvatarURLAttribute()
	RegistrantUsernameAttribute()
	PastMeetingParticipantIsInvitedAttribute()
	PastMeetingParticipantIsAttendedAttribute()
	CreatedAtAttribute()
	UpdatedAtAttribute()
	Required("uid", "past_meeting_uid", "meeting_uid", "email", "first_name", "last_name")
})

// PastMeetingParticipantUIDAttribute is the DSL attribute for past meeting participant UID.
func PastMeetingParticipantUIDAttribute() {
	Attribute("uid", String, "The UID of the past meeting participant", func() {
		// Read-only attribute
		Example("7cad5a8d-19d0-41a4-81a6-043453daf9ee")
		Format(FormatUUID)
	})
}

// PastMeetingParticipantPastMeetingUIDAttribute is the DSL attribute for past meeting UID.
func PastMeetingParticipantPastMeetingUIDAttribute() {
	Attribute("past_meeting_uid", String, "The unique identifier of the past meeting", func() {
		Example("7cad5a8d-19d0-41a4-81a6-043453daf9ee")
		Format(FormatUUID)
	})
}

// PastMeetingParticipantIsInvitedAttribute is the DSL attribute for past meeting participant is invited.
func PastMeetingParticipantIsInvitedAttribute() {
	Attribute("is_invited", Boolean, "Whether the participant was invited to this past meeting", func() {
		Example(true)
	})
}

// PastMeetingParticipantIsAttendedAttribute is the DSL attribute for past meeting participant is attended.
func PastMeetingParticipantIsAttendedAttribute() {
	Attribute("is_attended", Boolean, "Whether the participant attended this past meeting", func() {
		Example(true)
	})
}
