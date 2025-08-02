package design

import . "goa.design/goa/v3/dsl"

// BearerTokenAttribute is a reusable token attribute for JWT authentication.
func BearerTokenAttribute() {
	Token("bearer_token", String, func() {
		Description("JWT token issued by Heimdall")
		Example("eyJhbGci...")
	})
}

// EtagAttribute is a reusable ETag header attribute.
func EtagAttribute() {
	Attribute("etag", String, "ETag header value", func() {
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
