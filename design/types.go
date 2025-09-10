// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package design

import (
	. "goa.design/goa/v3/dsl" //nolint:staticcheck // ST1001: the recommended way of using the goa DSL package is with the . import
)

// BearerTokenAttribute is a reusable token attribute for JWT authentication.
func BearerTokenAttribute() {
	Token("bearer_token", String, func() {
		Description("JWT token issued by Heimdall")
		Example("eyJhbGci...")
	})
}

// EtagAttribute is a reusable ETag header attribute (for responses).
func EtagAttribute() {
	Attribute("etag", String, "ETag header value", func() {
		Example("123")
	})
}

// IfMatchAttribute is a reusable If-Match header attribute (for conditional requests).
func IfMatchAttribute() {
	Attribute("if_match", String, "If-Match header value for conditional requests", func() {
		Example("123")
	})
}

// VersionAttribute is a reusable version attribute.
func VersionAttribute() {
	Attribute("version", String, "Version of the API", func() {
		Enum("1")
		Example("1")
	})
}

// CreatedAtAttribute is a reusable created timestamp attribute.
func CreatedAtAttribute() {
	// Read-only attribute
	Attribute("created_at", String, "The date and time the resource was created", func() {
		Example("2021-01-01T00:00:00Z")
		Format(FormatDateTime)
	})
}

// UpdatedAtAttribute is a reusable updated timestamp attribute.
func UpdatedAtAttribute() {
	// Read-only attribute
	Attribute("updated_at", String, "The date and time the resource was last updated", func() {
		Example("2021-01-01T00:00:00Z")
		Format(FormatDateTime)
	})
}

//
// Error types
//

// BadRequestError is the DSL type for a bad request error.
var BadRequestError = Type("BadRequestError", func() {
	Attribute("code", String, "HTTP status code", func() {
		Example("400")
	})
	Attribute("message", String, "Error message", func() {
		Example("The request was invalid.")
	})
	Required("code", "message")
})

// NotFoundError is the DSL type for a not found error.
var NotFoundError = Type("NotFoundError", func() {
	Attribute("code", String, "HTTP status code", func() {
		Example("404")
	})
	Attribute("message", String, "Error message", func() {
		Example("The resource was not found.")
	})
	Required("code", "message")
})

// ConflictError is the DSL type for a conflict error.
var ConflictError = Type("ConflictError", func() {
	Attribute("code", String, "HTTP status code", func() {
		Example("409")
	})
	Attribute("message", String, "Error message", func() {
		Example("The resource already exists.")
	})
	Required("code", "message")
})

// InternalServerError is the DSL type for an internal server error.
var InternalServerError = Type("InternalServerError", func() {
	Attribute("code", String, "HTTP status code", func() {
		Example("500")
	})
	Attribute("message", String, "Error message", func() {
		Example("An internal server error occurred.")
	})
	Required("code", "message")
})

// ServiceUnavailableError is the DSL type for a service unavailable error.
var ServiceUnavailableError = Type("ServiceUnavailableError", func() {
	Attribute("code", String, "HTTP status code", func() {
		Example("503")
	})
	Attribute("message", String, "Error message", func() {
		Example("The service is unavailable.")
	})
	Required("code", "message")
})

// UnauthorizedError is the DSL type for an unauthorized error.
var UnauthorizedError = Type("UnauthorizedError", func() {
	Attribute("code", String, "HTTP status code", func() {
		Example("401")
	})
	Attribute("message", String, "Error message", func() {
		Example("Unauthorized request.")
	})
	Required("code", "message")
})

// ZoomWebhookPayload represents the payload structure for Zoom webhook events
var ZoomWebhookPayload = Type("ZoomWebhookPayload", func() {
	Description("Zoom webhook event payload")
	Attribute("event", String, "The type of event", func() {
		Example("meeting.started")
		Enum(
			"meeting.started",
			"meeting.ended",
			"meeting.deleted",
			"meeting.participant_joined",
			"meeting.participant_left",
			"recording.completed",
			"recording.transcript_completed",
			"meeting.summary_completed",
			"endpoint.url_validation",
		)
	})
	Attribute("event_ts", Int64, "Event timestamp in milliseconds", func() {
		Example(1609459200000)
	})
	Attribute("payload", Any, "Event-specific payload data", func() {
		Description("Contains meeting, participant, or recording data depending on event type")
	})
	Attribute("zoom_signature", String, "Zoom webhook signature for verification", func() {
		Description("HMAC-SHA256 signature of the request body")
	})
	Attribute("zoom_timestamp", String, "Zoom timestamp header for replay protection", func() {
		Description("Timestamp when the webhook was sent")
	})
	Required("event", "event_ts", "payload")
})

// ZoomWebhookResponse represents the response for webhook processing
var ZoomWebhookResponse = Type("ZoomWebhookResponse", func() {
	Description("Response indicating successful webhook processing")
	Attribute("status", String, "Processing status", func() {
		Example("success")
	})
	Attribute("message", String, "Optional message", func() {
		Example("Event processed successfully")
	})
	Required("status")
})
