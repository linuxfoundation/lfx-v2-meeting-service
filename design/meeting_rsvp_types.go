// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package design

import (
	. "goa.design/goa/v3/dsl" //nolint:staticcheck // ST1001: the recommended way of using the goa DSL package is with the . import
)

// CreateRSVPPayload represents the payload for creating or updating an RSVP
var CreateRSVPPayload = Type("CreateRSVPPayload", func() {
	Description("Payload for creating or updating an RSVP response. Username is automatically extracted from the JWT token.")
	RSVPRegistrantIDAttribute()
	RSVPResponseAttribute()
	RSVPScopeAttribute()
	RSVPOccurrenceIDAttribute()
	Required("response", "scope")
})

// RSVPResponse represents an RSVP response
var RSVPResponse = Type("RSVPResponse", func() {
	Description("RSVP response object")
	RSVPIDAttribute()
	RSVPMeetingUIDAttribute()
	RSVPRegistrantIDAttribute()
	RSVPUsernameAttribute()
	RSVPEmailAttribute()
	RSVPResponseAttribute()
	RSVPScopeAttribute()
	RSVPOccurrenceIDAttribute()
	CreatedAtAttribute()
	UpdatedAtAttribute()
	Required("id", "meeting_uid", "registrant_id", "username", "email", "response", "scope")
})

// RSVPListResult represents a list of RSVP responses
var RSVPListResult = Type("RSVPListResult", func() {
	Description("List of RSVP responses")
	Attribute("rsvps", ArrayOf(RSVPResponse), "List of RSVP responses")
	Required("rsvps")
})

//
// RSVP attribute helper functions
//

// RSVPIDAttribute is the DSL attribute for RSVP ID.
func RSVPIDAttribute() {
	Attribute("id", String, "The unique identifier for this RSVP", func() {
		Example("7cad5a8d-19d0-41a4-81a6-043453daf9ee")
		Format(FormatUUID)
	})
}

// RSVPMeetingUIDAttribute is the DSL attribute for RSVP meeting UID.
func RSVPMeetingUIDAttribute() {
	Attribute("meeting_uid", String, "The UID of the meeting this RSVP is for", func() {
		Example("7cad5a8d-19d0-41a4-81a6-043453daf9ee")
		Format(FormatUUID)
	})
}

// RSVPRegistrantIDAttribute is the DSL attribute for RSVP registrant ID.
func RSVPRegistrantIDAttribute() {
	Attribute("registrant_id", String, "The ID of the registrant submitting this RSVP", func() {
		Example("7cad5a8d-19d0-41a4-81a6-043453daf9ee")
		Format(FormatUUID)
	})
}

// RSVPResponseAttribute is the DSL attribute for RSVP response type.
func RSVPResponseAttribute() {
	Attribute("response", String, "The RSVP response", func() {
		Enum("accepted", "maybe", "declined")
		Example("accepted")
	})
}

// RSVPScopeAttribute is the DSL attribute for RSVP scope.
func RSVPScopeAttribute() {
	Attribute("scope", String, "The scope of the RSVP (single occurrence, all occurrences, or this and following)", func() {
		Enum("single", "all", "this_and_following")
		Example("all")
	})
}

// RSVPUsernameAttribute is the DSL attribute for RSVP registrant username.
func RSVPUsernameAttribute() {
	Attribute("username", String, "The username of the registrant", func() {
		Example("jdoe")
	})
}

// RSVPEmailAttribute is the DSL attribute for RSVP registrant email.
func RSVPEmailAttribute() {
	Attribute("email", String, "The email of the registrant", func() {
		Example("john.doe@example.com")
		Format(FormatEmail)
	})
}

// RSVPOccurrenceIDAttribute is the DSL attribute for RSVP occurrence ID.
func RSVPOccurrenceIDAttribute() {
	Attribute("occurrence_id", String, "The ID of the specific occurrence (required for 'single' and 'this_and_following' scopes)", func() {
		Example("1640995200")
		Pattern("^[0-9]*$") // Validate as numeric string if it's a timestamp
	})
}
