package design

import . "goa.design/goa/v3/dsl"

// CreateRegistrantPayload represents the payload for creating a registrant
var CreateRegistrantPayload = Type("CreateRegistrantPayload", func() {
	Description("Payload for creating a new meeting registrant")
	RegistrantMeetingUIDAttribute()
	RegistrantEmailAttribute()
	RegistrantFirstNameAttribute()
	RegistrantLastNameAttribute()
	RegistrantHostAttribute()
	RegistrantJobTitleAttribute()
	RegistrantOccurrenceIDAttribute()
	RegistrantOrgNameAttribute()
	RegistrantOrgIsMemberAttribute()
	RegistrantOrgIsProjectMemberAttribute()
	RegistrantAvatarURLAttribute()
	RegistrantUserIDAttribute()
	Required("meeting_uid", "email", "first_name", "last_name")
})

// UpdateRegistrantPayload represents the payload for updating a registrant
var UpdateRegistrantPayload = Type("UpdateRegistrantPayload", func() {
	Description("Payload for updating an existing meeting registrant")
	RegistrantMeetingUIDAttribute()
	RegistrantEmailAttribute()
	RegistrantFirstNameAttribute()
	RegistrantLastNameAttribute()
	RegistrantHostAttribute()
	RegistrantJobTitleAttribute()
	RegistrantOccurrenceIDAttribute()
	RegistrantOrgNameAttribute()
	RegistrantOrgIsMemberAttribute()
	RegistrantOrgIsProjectMemberAttribute()
	RegistrantAvatarURLAttribute()
	RegistrantUserIDAttribute()
	Required("meeting_uid", "email", "first_name", "last_name")
})

// Registrant represents a meeting registrant
var Registrant = Type("Registrant", func() {
	Description("Meeting registrant object")
	RegistrantUIDAttribute()
	RegistrantMeetingUIDAttribute()
	RegistrantEmailAttribute()
	RegistrantFirstNameAttribute()
	RegistrantLastNameAttribute()
	RegistrantHostAttribute()
	RegistrantJobTitleAttribute()
	RegistrantOccurrenceIDAttribute()
	RegistrantOrgNameAttribute()
	RegistrantOrgIsMemberAttribute()
	RegistrantOrgIsProjectMemberAttribute()
	RegistrantAvatarURLAttribute()
	RegistrantUserIDAttribute()
	CreatedAtAttribute()
	UpdatedAtAttribute()
	Required("uid", "meeting_uid", "email", "first_name", "last_name")
})

//
// Registrant attribute helper functions
//

// RegistrantUIDAttribute is the DSL attribute for registrant UID.
func RegistrantUIDAttribute() {
	Attribute("uid", String, "The UID of the registrant", func() {
		// Read-only attribute
		Example("7cad5a8d-19d0-41a4-81a6-043453daf9ee")
		Format(FormatUUID)
	})
}

// RegistrantMeetingUIDAttribute is the DSL attribute for registrant meeting UID.
func RegistrantMeetingUIDAttribute() {
	Attribute("meeting_uid", String, "The UID of the meeting", func() {
		Example("7cad5a8d-19d0-41a4-81a6-043453daf9ee")
		Format(FormatUUID)
	})
}

// RegistrantEmailAttribute is the DSL attribute for registrant email.
func RegistrantEmailAttribute() {
	Attribute("email", String, "User's email address", func() {
		Format(FormatEmail)
		Example("user@example.com")
	})
}

// RegistrantFirstNameAttribute is the DSL attribute for registrant first name.
func RegistrantFirstNameAttribute() {
	Attribute("first_name", String, "User's first name", func() {
		Example("John")
	})
}

// RegistrantLastNameAttribute is the DSL attribute for registrant last name.
func RegistrantLastNameAttribute() {
	Attribute("last_name", String, "User's last name", func() {
		Example("Doe")
	})
}

// RegistrantHostAttribute is the DSL attribute for registrant host access.
func RegistrantHostAttribute() {
	Attribute("host", Boolean, "If user should have access as a meeting host")
}

// RegistrantJobTitleAttribute is the DSL attribute for registrant job title.
func RegistrantJobTitleAttribute() {
	Attribute("job_title", String, "User's job title", func() {
		Example("Software Engineer")
	})
}

// RegistrantOccurrenceIDAttribute is the DSL attribute for registrant occurrence ID.
func RegistrantOccurrenceIDAttribute() {
	Attribute("occurrence_id", String, "The ID of the specific occurrence the user should be invited to. If blank, user is invited to all occurrences", func() {
		Example("1640995200")
	})
}

// RegistrantOrgNameAttribute is the DSL attribute for registrant organization name.
func RegistrantOrgNameAttribute() {
	Attribute("org_name", String, "User's organization")
}

// RegistrantOrgIsMemberAttribute is the DSL attribute for registrant LF membership.
func RegistrantOrgIsMemberAttribute() {
	// Read-only attribute
	Attribute("org_is_member", Boolean, "Whether the registrant is in an organization that has a membership with the LF. If unknown, don't pass this field; the API will find the value by default")
}

// RegistrantOrgIsProjectMemberAttribute is the DSL attribute for registrant project membership.
func RegistrantOrgIsProjectMemberAttribute() {
	// Read-only attribute
	Attribute("org_is_project_member", Boolean, "Whether the registrant is in an organization that has a membership with the project (of the meeting). If unknown, don't pass this field; the API will find the value by default")
}

// RegistrantAvatarURLAttribute is the DSL attribute for registrant avatar URL.
func RegistrantAvatarURLAttribute() {
	Attribute("avatar_url", String, "User's avatar URL", func() {
		Format(FormatURI)
		Example("https://example.com/avatar.jpg")
	})
}

// RegistrantUserIDAttribute is the DSL attribute for registrant user ID.
func RegistrantUserIDAttribute() {
	Attribute("user_id", String, "User's LF ID")
}
