// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package design

import (
	. "goa.design/goa/v3/dsl" //nolint:staticcheck // ST1001: the recommended way of using the goa DSL package is with the . import
)

// JWTAuth is the DSL JWT security type for authentication.
var JWTAuth = JWTSecurity("jwt", func() {
	Description("Heimdall authorization")
})

var _ = Service("Meeting Service", func() {
	Description("The ITX Meeting Proxy service provides a lightweight proxy layer to the ITX Zoom API for LF projects.")

	Method("readyz", func() {
		Description("Check if the service is able to take inbound requests.")
		Meta("swagger:generate", "false")
		Result(Bytes, func() {
			Example("OK")
		})
		Error("ServiceUnavailable", ServiceUnavailableError, "Service is unavailable")
		HTTP(func() {
			GET("/readyz")
			Response(StatusOK, func() {
				ContentType("text/plain")
			})
			Response("ServiceUnavailable", StatusServiceUnavailable)
		})
	})

	Method("livez", func() {
		Description("Check if the service is alive.")
		Meta("swagger:generate", "false")
		Result(Bytes, func() {
			Example("OK")
		})
		HTTP(func() {
			GET("/livez")
			Response(StatusOK, func() {
				ContentType("text/plain")
			})
		})
	})

	// ITX Zoom API Proxy endpoints
	Method("create-itx-meeting", func() {
		Description("Create a Zoom meeting through ITX API proxy")

		Security(JWTAuth)

		Payload(func() {
			BearerTokenAttribute()
			VersionAttribute()
			XSyncAttribute()
			// Request fields
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
			Required("project_uid", "title", "start_time", "duration", "timezone", "visibility")
		})

		Result(ITXZoomMeetingResponse)

		Error("BadRequest", BadRequestError, "Bad request")
		Error("Unauthorized", UnauthorizedError, "Unauthorized")
		Error("Forbidden", ForbiddenError, "Forbidden")
		Error("Conflict", ConflictError, "Conflict with existing meeting")
		Error("InternalServerError", InternalServerError, "Internal server error")
		Error("ServiceUnavailable", ServiceUnavailableError, "Service unavailable")

		HTTP(func() {
			POST("/itx/meetings")
			Param("version:v")
			Header("bearer_token:Authorization")
			Header("x_sync:X-Sync")
			Response(StatusCreated)
			Response("BadRequest", StatusBadRequest)
			Response("Unauthorized", StatusUnauthorized)
			Response("Forbidden", StatusForbidden)
			Response("Conflict", StatusConflict)
			Response("InternalServerError", StatusInternalServerError)
			Response("ServiceUnavailable", StatusServiceUnavailable)
		})
	})

	Method("get-itx-meeting", func() {
		Description("Get a Zoom meeting through ITX API proxy")

		Security(JWTAuth)

		Payload(func() {
			BearerTokenAttribute()
			VersionAttribute()
			Attribute("meeting_id", String, "The Zoom meeting ID", func() {
				Example("1234567890")
			})
			Required("meeting_id")
		})

		Result(ITXZoomMeetingResponse)

		Error("BadRequest", BadRequestError, "Bad request")
		Error("Unauthorized", UnauthorizedError, "Unauthorized")
		Error("Forbidden", ForbiddenError, "Forbidden")
		Error("NotFound", NotFoundError, "Meeting not found")
		Error("InternalServerError", InternalServerError, "Internal server error")
		Error("ServiceUnavailable", ServiceUnavailableError, "Service unavailable")

		HTTP(func() {
			GET("/itx/meetings/{meeting_id}")
			Param("version:v")
			Param("meeting_id")
			Header("bearer_token:Authorization")
			Response(StatusOK)
			Response("BadRequest", StatusBadRequest)
			Response("Unauthorized", StatusUnauthorized)
			Response("Forbidden", StatusForbidden)
			Response("NotFound", StatusNotFound)
			Response("InternalServerError", StatusInternalServerError)
			Response("ServiceUnavailable", StatusServiceUnavailable)
		})
	})

	Method("delete-itx-meeting", func() {
		Description("Delete a Zoom meeting through ITX API proxy")

		Security(JWTAuth)

		Payload(func() {
			BearerTokenAttribute()
			VersionAttribute()
			Attribute("meeting_id", String, "The Zoom meeting ID", func() {
				Example("1234567890")
			})
			Required("meeting_id")
		})

		Error("BadRequest", BadRequestError, "Bad request")
		Error("Unauthorized", UnauthorizedError, "Unauthorized")
		Error("Forbidden", ForbiddenError, "Forbidden")
		Error("NotFound", NotFoundError, "Meeting not found")
		Error("InternalServerError", InternalServerError, "Internal server error")
		Error("ServiceUnavailable", ServiceUnavailableError, "Service unavailable")

		HTTP(func() {
			DELETE("/itx/meetings/{meeting_id}")
			Param("version:v")
			Param("meeting_id")
			Header("bearer_token:Authorization")
			Response(StatusNoContent)
			Response("BadRequest", StatusBadRequest)
			Response("Unauthorized", StatusUnauthorized)
			Response("Forbidden", StatusForbidden)
			Response("NotFound", StatusNotFound)
			Response("InternalServerError", StatusInternalServerError)
			Response("ServiceUnavailable", StatusServiceUnavailable)
		})
	})

	Method("update-itx-meeting", func() {
		Description("Update a Zoom meeting through ITX API proxy")

		Security(JWTAuth)

		Payload(func() {
			BearerTokenAttribute()
			VersionAttribute()
			XSyncAttribute()
			Attribute("meeting_id", String, "The Zoom meeting ID", func() {
				Example("1234567890")
			})
			// Request fields (same as create)
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
			Required("meeting_id", "project_uid", "title", "start_time", "duration", "timezone", "visibility")
		})

		Error("BadRequest", BadRequestError, "Bad request")
		Error("Unauthorized", UnauthorizedError, "Unauthorized")
		Error("Forbidden", ForbiddenError, "Forbidden")
		Error("NotFound", NotFoundError, "Meeting not found")
		Error("Conflict", ConflictError, "Conflict with existing meeting")
		Error("InternalServerError", InternalServerError, "Internal server error")
		Error("ServiceUnavailable", ServiceUnavailableError, "Service unavailable")

		HTTP(func() {
			PUT("/itx/meetings/{meeting_id}")
			Param("version:v")
			Param("meeting_id")
			Header("bearer_token:Authorization")
			Header("x_sync:X-Sync")
			Response(StatusNoContent)
			Response("BadRequest", StatusBadRequest)
			Response("Unauthorized", StatusUnauthorized)
			Response("Forbidden", StatusForbidden)
			Response("NotFound", StatusNotFound)
			Response("Conflict", StatusConflict)
			Response("InternalServerError", StatusInternalServerError)
			Response("ServiceUnavailable", StatusServiceUnavailable)
		})
	})

	Method("get-itx-meeting-count", func() {
		Description("Get the count of Zoom meetings for a project through ITX API proxy")

		Security(JWTAuth)

		Payload(func() {
			BearerTokenAttribute()
			VersionAttribute()
			ITXProjectUIDAttribute()
			Required("project_uid")
		})

		Result(ITXMeetingCountResponse)

		Error("BadRequest", BadRequestError, "Bad request")
		Error("Unauthorized", UnauthorizedError, "Unauthorized")
		Error("Forbidden", ForbiddenError, "Forbidden")
		Error("NotFound", NotFoundError, "Project not found")
		Error("InternalServerError", InternalServerError, "Internal server error")
		Error("ServiceUnavailable", ServiceUnavailableError, "Service unavailable")

		HTTP(func() {
			GET("/itx/meeting_count")
			Param("version:v")
			Param("project_uid")
			Header("bearer_token:Authorization")
			Response(StatusOK)
			Response("BadRequest", StatusBadRequest)
			Response("Unauthorized", StatusUnauthorized)
			Response("Forbidden", StatusForbidden)
			Response("NotFound", StatusNotFound)
			Response("InternalServerError", StatusInternalServerError)
			Response("ServiceUnavailable", StatusServiceUnavailable)
		})
	})

	Method("create-itx-registrant", func() {
		Description("Create a meeting registrant through ITX API proxy")

		Security(JWTAuth)

		Payload(func() {
			BearerTokenAttribute()
			VersionAttribute()
			Attribute("meeting_id", String, "The ID of the meeting", func() {
				Example("1234567890")
			})
			Extend(ITXZoomMeetingRegistrant)
			Required("meeting_id")
		})

		Result(ITXZoomMeetingRegistrant)

		Error("BadRequest", BadRequestError, "Bad request")
		Error("Unauthorized", UnauthorizedError, "Unauthorized")
		Error("Forbidden", ForbiddenError, "Forbidden")
		Error("NotFound", NotFoundError, "Meeting not found")
		Error("Conflict", ConflictError, "Registrant already exists")
		Error("InternalServerError", InternalServerError, "Internal server error")
		Error("ServiceUnavailable", ServiceUnavailableError, "Service unavailable")

		HTTP(func() {
			POST("/itx/meetings/{meeting_id}/registrants")
			Param("version:v")
			Header("bearer_token:Authorization")
			Response(StatusCreated)
			Response("BadRequest", StatusBadRequest)
			Response("Unauthorized", StatusUnauthorized)
			Response("Forbidden", StatusForbidden)
			Response("NotFound", StatusNotFound)
			Response("Conflict", StatusConflict)
			Response("InternalServerError", StatusInternalServerError)
			Response("ServiceUnavailable", StatusServiceUnavailable)
		})
	})

	Method("get-itx-registrant", func() {
		Description("Get a meeting registrant through ITX API proxy")

		Security(JWTAuth)

		Payload(func() {
			BearerTokenAttribute()
			VersionAttribute()
			Attribute("meeting_id", String, "The ID of the meeting", func() {
				Example("1234567890")
			})
			Attribute("registrant_id", String, "The ID of the registrant", func() {
				Example("zjkfsdfjdfhg")
			})
			Required("meeting_id", "registrant_id")
		})

		Result(ITXZoomMeetingRegistrant)

		Error("BadRequest", BadRequestError, "Bad request")
		Error("Unauthorized", UnauthorizedError, "Unauthorized")
		Error("Forbidden", ForbiddenError, "Forbidden")
		Error("NotFound", NotFoundError, "Registrant not found")
		Error("InternalServerError", InternalServerError, "Internal server error")
		Error("ServiceUnavailable", ServiceUnavailableError, "Service unavailable")

		HTTP(func() {
			GET("/itx/meetings/{meeting_id}/registrants/{registrant_id}")
			Param("version:v")
			Header("bearer_token:Authorization")
			Response(StatusOK)
			Response("BadRequest", StatusBadRequest)
			Response("Unauthorized", StatusUnauthorized)
			Response("Forbidden", StatusForbidden)
			Response("NotFound", StatusNotFound)
			Response("InternalServerError", StatusInternalServerError)
			Response("ServiceUnavailable", StatusServiceUnavailable)
		})
	})

	Method("update-itx-registrant", func() {
		Description("Update a meeting registrant through ITX API proxy")

		Security(JWTAuth)

		Payload(func() {
			BearerTokenAttribute()
			VersionAttribute()
			Attribute("meeting_id", String, "The ID of the meeting", func() {
				Example("1234567890")
			})
			Attribute("registrant_id", String, "The ID of the registrant", func() {
				Example("zjkfsdfjdfhg")
			})
			Extend(ITXZoomMeetingRegistrant)
			Required("meeting_id", "registrant_id")
		})

		Error("BadRequest", BadRequestError, "Bad request")
		Error("Unauthorized", UnauthorizedError, "Unauthorized")
		Error("Forbidden", ForbiddenError, "Forbidden")
		Error("NotFound", NotFoundError, "Registrant not found")
		Error("InternalServerError", InternalServerError, "Internal server error")
		Error("ServiceUnavailable", ServiceUnavailableError, "Service unavailable")

		HTTP(func() {
			PUT("/itx/meetings/{meeting_id}/registrants/{registrant_id}")
			Param("version:v")
			Header("bearer_token:Authorization")
			Response(StatusNoContent)
			Response("BadRequest", StatusBadRequest)
			Response("Unauthorized", StatusUnauthorized)
			Response("Forbidden", StatusForbidden)
			Response("NotFound", StatusNotFound)
			Response("InternalServerError", StatusInternalServerError)
			Response("ServiceUnavailable", StatusServiceUnavailable)
		})
	})

	Method("delete-itx-registrant", func() {
		Description("Delete a meeting registrant through ITX API proxy")

		Security(JWTAuth)

		Payload(func() {
			BearerTokenAttribute()
			VersionAttribute()
			Attribute("meeting_id", String, "The ID of the meeting", func() {
				Example("1234567890")
			})
			Attribute("registrant_id", String, "The ID of the registrant", func() {
				Example("zjkfsdfjdfhg")
			})
			Required("meeting_id", "registrant_id")
		})

		Error("BadRequest", BadRequestError, "Bad request")
		Error("Unauthorized", UnauthorizedError, "Unauthorized")
		Error("Forbidden", ForbiddenError, "Forbidden")
		Error("NotFound", NotFoundError, "Registrant not found")
		Error("InternalServerError", InternalServerError, "Internal server error")
		Error("ServiceUnavailable", ServiceUnavailableError, "Service unavailable")

		HTTP(func() {
			DELETE("/itx/meetings/{meeting_id}/registrants/{registrant_id}")
			Param("version:v")
			Header("bearer_token:Authorization")
			Response(StatusNoContent)
			Response("BadRequest", StatusBadRequest)
			Response("Unauthorized", StatusUnauthorized)
			Response("Forbidden", StatusForbidden)
			Response("NotFound", StatusNotFound)
			Response("InternalServerError", StatusInternalServerError)
			Response("ServiceUnavailable", StatusServiceUnavailable)
		})
	})

	Method("get-itx-join-link", func() {
		Description("Get join link for a meeting through ITX API proxy")

		Security(JWTAuth)

		Payload(func() {
			BearerTokenAttribute()
			VersionAttribute()
			Attribute("meeting_id", String, "The ID of the meeting", func() {
				Example("1234567890")
			})
			Attribute("use_email", Boolean, "Use email for identification instead of user_id")
			Attribute("user_id", String, "LF user ID", func() {
				Example("user123")
			})
			Attribute("name", String, "User's full name", func() {
				Example("John Doe")
			})
			Attribute("email", String, "User's email address", func() {
				Example("john.doe@example.com")
				Format(FormatEmail)
			})
			Attribute("register", Boolean, "Register user as guest if not already registered")
			Required("meeting_id")
		})

		Result(ITXZoomMeetingJoinLink)

		Error("BadRequest", BadRequestError, "Bad request")
		Error("Unauthorized", UnauthorizedError, "Unauthorized")
		Error("Forbidden", ForbiddenError, "Forbidden")
		Error("NotFound", NotFoundError, "Meeting not found")
		Error("InternalServerError", InternalServerError, "Internal server error")
		Error("ServiceUnavailable", ServiceUnavailableError, "Service unavailable")

		HTTP(func() {
			GET("/itx/meetings/{meeting_id}/join_link")
			Param("version:v")
			Param("use_email")
			Param("user_id")
			Param("name")
			Param("email")
			Param("register")
			Header("bearer_token:Authorization")
			Response(StatusOK)
			Response("BadRequest", StatusBadRequest)
			Response("Unauthorized", StatusUnauthorized)
			Response("Forbidden", StatusForbidden)
			Response("NotFound", StatusNotFound)
			Response("InternalServerError", StatusInternalServerError)
			Response("ServiceUnavailable", StatusServiceUnavailable)
		})
	})

	Method("get-itx-registrant-ics", func() {
		Description("Get ICS calendar file for a meeting registrant through ITX API proxy")

		Security(JWTAuth)

		Payload(func() {
			BearerTokenAttribute()
			VersionAttribute()
			Attribute("meeting_id", String, "The ID of the meeting", func() {
				Example("1234567890")
			})
			Attribute("registrant_id", String, "The ID of the registrant", func() {
				Example("zjkfsdfjdfhg")
			})
			Required("meeting_id", "registrant_id")
		})

		Result(Bytes)

		Error("BadRequest", BadRequestError, "Bad request")
		Error("Unauthorized", UnauthorizedError, "Unauthorized")
		Error("Forbidden", ForbiddenError, "Forbidden")
		Error("NotFound", NotFoundError, "Registrant not found")
		Error("InternalServerError", InternalServerError, "Internal server error")
		Error("ServiceUnavailable", ServiceUnavailableError, "Service unavailable")

		HTTP(func() {
			GET("/itx/meetings/{meeting_id}/registrants/{registrant_id}/ics")
			Param("version:v")
			Header("bearer_token:Authorization")
			Response(StatusOK, func() {
				ContentType("text/calendar")
			})
			Response("BadRequest", StatusBadRequest)
			Response("Unauthorized", StatusUnauthorized)
			Response("Forbidden", StatusForbidden)
			Response("NotFound", StatusNotFound)
			Response("InternalServerError", StatusInternalServerError)
			Response("ServiceUnavailable", StatusServiceUnavailable)
		})
	})

	Method("resend-itx-registrant-invitation", func() {
		Description("Resend meeting invitation to a registrant through ITX API proxy")

		Security(JWTAuth)

		Payload(func() {
			BearerTokenAttribute()
			VersionAttribute()
			Attribute("meeting_id", String, "The ID of the meeting", func() {
				Example("1234567890")
			})
			Attribute("registrant_id", String, "The ID of the registrant", func() {
				Example("zjkfsdfjdfhg")
			})
			Required("meeting_id", "registrant_id")
		})

		Error("BadRequest", BadRequestError, "Bad request")
		Error("Unauthorized", UnauthorizedError, "Unauthorized")
		Error("Forbidden", ForbiddenError, "Forbidden")
		Error("NotFound", NotFoundError, "Registrant not found")
		Error("InternalServerError", InternalServerError, "Internal server error")
		Error("ServiceUnavailable", ServiceUnavailableError, "Service unavailable")

		HTTP(func() {
			POST("/itx/meetings/{meeting_id}/registrants/{registrant_id}/resend")
			Param("version:v")
			Header("bearer_token:Authorization")
			Response(StatusNoContent)
			Response("BadRequest", StatusBadRequest)
			Response("Unauthorized", StatusUnauthorized)
			Response("Forbidden", StatusForbidden)
			Response("NotFound", StatusNotFound)
			Response("InternalServerError", StatusInternalServerError)
			Response("ServiceUnavailable", StatusServiceUnavailable)
		})
	})

	Method("resend-itx-meeting-invitations", func() {
		Description("Resend meeting invitations to all registrants through ITX API proxy")

		Security(JWTAuth)

		Payload(func() {
			BearerTokenAttribute()
			VersionAttribute()
			Attribute("meeting_id", String, "The ID of the meeting", func() {
				Example("1234567890")
			})
			Attribute("exclude_registrant_ids", ArrayOf(String), "Registrant IDs to exclude from resend", func() {
				Example([]string{"reg123", "reg456"})
			})
			Required("meeting_id")
		})

		Error("BadRequest", BadRequestError, "Bad request")
		Error("Unauthorized", UnauthorizedError, "Unauthorized")
		Error("Forbidden", ForbiddenError, "Forbidden")
		Error("NotFound", NotFoundError, "Meeting not found")
		Error("InternalServerError", InternalServerError, "Internal server error")
		Error("ServiceUnavailable", ServiceUnavailableError, "Service unavailable")

		HTTP(func() {
			POST("/itx/meetings/{meeting_id}/resend")
			Param("version:v")
			Header("bearer_token:Authorization")
			Response(StatusNoContent)
			Response("BadRequest", StatusBadRequest)
			Response("Unauthorized", StatusUnauthorized)
			Response("Forbidden", StatusForbidden)
			Response("NotFound", StatusNotFound)
			Response("InternalServerError", StatusInternalServerError)
			Response("ServiceUnavailable", StatusServiceUnavailable)
		})
	})

	Method("register-itx-committee-members", func() {
		Description("Register committee members to a meeting asynchronously through ITX API proxy")

		Security(JWTAuth)

		Payload(func() {
			BearerTokenAttribute()
			VersionAttribute()
			Attribute("meeting_id", String, "The ID of the meeting", func() {
				Example("1234567890")
			})
			Required("meeting_id")
		})

		Error("BadRequest", BadRequestError, "Bad request")
		Error("Unauthorized", UnauthorizedError, "Unauthorized")
		Error("Forbidden", ForbiddenError, "Forbidden")
		Error("NotFound", NotFoundError, "Meeting not found")
		Error("InternalServerError", InternalServerError, "Internal server error")
		Error("ServiceUnavailable", ServiceUnavailableError, "Service unavailable")

		HTTP(func() {
			POST("/itx/meetings/{meeting_id}/register_committee_members")
			Param("version:v")
			Header("bearer_token:Authorization")
			Response(StatusNoContent)
			Response("BadRequest", StatusBadRequest)
			Response("Unauthorized", StatusUnauthorized)
			Response("Forbidden", StatusForbidden)
			Response("NotFound", StatusNotFound)
			Response("InternalServerError", StatusInternalServerError)
			Response("ServiceUnavailable", StatusServiceUnavailable)
		})
	})

	Method("update-itx-occurrence", func() {
		Description("Update a specific occurrence of a recurring meeting through ITX API proxy")

		Security(JWTAuth)

		Payload(func() {
			BearerTokenAttribute()
			VersionAttribute()
			Attribute("meeting_id", String, "The ID of the meeting", func() {
				Example("1234567890")
			})
			Attribute("occurrence_id", String, "The ID of the occurrence (Unix timestamp)", func() {
				Example("1640995200")
			})
			Attribute("start_time", String, "Meeting start time in RFC3339 format", func() {
				Example("2024-01-15T10:00:00Z")
				Format(FormatDateTime)
			})
			Attribute("duration", Int, "Meeting duration in minutes", func() {
				Example(60)
				Minimum(1)
			})
			Attribute("topic", String, "Meeting topic/title")
			Attribute("agenda", String, "Meeting agenda/description")
			Attribute("recurrence", Recurrence, "Recurrence settings")
			Required("meeting_id", "occurrence_id")
		})

		Error("BadRequest", BadRequestError, "Bad request")
		Error("Unauthorized", UnauthorizedError, "Unauthorized")
		Error("Forbidden", ForbiddenError, "Forbidden")
		Error("NotFound", NotFoundError, "Meeting or occurrence not found")
		Error("InternalServerError", InternalServerError, "Internal server error")
		Error("ServiceUnavailable", ServiceUnavailableError, "Service unavailable")

		HTTP(func() {
			PUT("/itx/meetings/{meeting_id}/occurrences/{occurrence_id}")
			Param("version:v")
			Header("bearer_token:Authorization")
			Response(StatusNoContent)
			Response("BadRequest", StatusBadRequest)
			Response("Unauthorized", StatusUnauthorized)
			Response("Forbidden", StatusForbidden)
			Response("NotFound", StatusNotFound)
			Response("InternalServerError", StatusInternalServerError)
			Response("ServiceUnavailable", StatusServiceUnavailable)
		})
	})

	Method("delete-itx-occurrence", func() {
		Description("Delete a specific occurrence of a recurring meeting through ITX API proxy")

		Security(JWTAuth)

		Payload(func() {
			BearerTokenAttribute()
			VersionAttribute()
			Attribute("meeting_id", String, "The ID of the meeting", func() {
				Example("1234567890")
			})
			Attribute("occurrence_id", String, "The ID of the occurrence (Unix timestamp)", func() {
				Example("1640995200")
			})
			Required("meeting_id", "occurrence_id")
		})

		Error("BadRequest", BadRequestError, "Bad request")
		Error("Unauthorized", UnauthorizedError, "Unauthorized")
		Error("Forbidden", ForbiddenError, "Forbidden")
		Error("NotFound", NotFoundError, "Meeting or occurrence not found")
		Error("InternalServerError", InternalServerError, "Internal server error")
		Error("ServiceUnavailable", ServiceUnavailableError, "Service unavailable")

		HTTP(func() {
			DELETE("/itx/meetings/{meeting_id}/occurrences/{occurrence_id}")
			Param("version:v")
			Header("bearer_token:Authorization")
			Response(StatusNoContent)
			Response("BadRequest", StatusBadRequest)
			Response("Unauthorized", StatusUnauthorized)
			Response("Forbidden", StatusForbidden)
			Response("NotFound", StatusNotFound)
			Response("InternalServerError", StatusInternalServerError)
			Response("ServiceUnavailable", StatusServiceUnavailable)
		})
	})

	Method("create-itx-past-meeting", func() {
		Description("Create a past meeting through ITX API proxy")

		Security(JWTAuth)

		Payload(func() {
			BearerTokenAttribute()
			VersionAttribute()

			// Required fields
			Attribute("meeting_id", String, "Zoom meeting ID", func() {
				Example("12343245463")
			})
			Attribute("occurrence_id", String, "Zoom occurrence ID (Unix timestamp)", func() {
				Example("1630560600000")
			})
			ITXProjectUIDAttribute()
			StartTimeAttribute()
			DurationAttribute()
			TimezoneAttribute()

			// Optional fields
			DescriptionAttribute()
			RestrictedAttribute()
			CommitteesAttribute()
			MeetingTypeAttribute()
			RecordingEnabledAttribute()
			TranscriptEnabledAttribute()
			ArtifactVisibilityAttribute()
			VisibilityAttribute()
			TitleAttribute()

			Required("meeting_id", "occurrence_id", "project_uid", "start_time", "duration", "timezone")
		})

		Result(ITXPastZoomMeeting)

		Error("BadRequest", BadRequestError, "Bad request")
		Error("Unauthorized", UnauthorizedError, "Unauthorized")
		Error("Forbidden", ForbiddenError, "Forbidden")
		Error("NotFound", NotFoundError, "Project or meeting not found")
		Error("Conflict", ConflictError, "Past meeting already exists")
		Error("InternalServerError", InternalServerError, "Internal server error")
		Error("ServiceUnavailable", ServiceUnavailableError, "Service unavailable")

		HTTP(func() {
			POST("/itx/past_meetings")
			Param("version:v")
			Header("bearer_token:Authorization")
			Response(StatusCreated)
			Response("BadRequest", StatusBadRequest)
			Response("Unauthorized", StatusUnauthorized)
			Response("Forbidden", StatusForbidden)
			Response("NotFound", StatusNotFound)
			Response("Conflict", StatusConflict)
			Response("InternalServerError", StatusInternalServerError)
			Response("ServiceUnavailable", StatusServiceUnavailable)
		})
	})

	Method("get-itx-past-meeting", func() {
		Description("Get a past meeting through ITX API proxy")

		Security(JWTAuth)

		Payload(func() {
			BearerTokenAttribute()
			VersionAttribute()
			Attribute("past_meeting_id", String, "Past meeting ID (meeting_id or meeting_id-occurrence_id)", func() {
				Example("12343245463-1630560600000")
			})
			Required("past_meeting_id")
		})

		Result(ITXPastZoomMeeting)

		Error("BadRequest", BadRequestError, "Bad request")
		Error("Unauthorized", UnauthorizedError, "Unauthorized")
		Error("Forbidden", ForbiddenError, "Forbidden")
		Error("NotFound", NotFoundError, "Past meeting not found")
		Error("InternalServerError", InternalServerError, "Internal server error")
		Error("ServiceUnavailable", ServiceUnavailableError, "Service unavailable")

		HTTP(func() {
			GET("/itx/past_meetings/{past_meeting_id}")
			Param("version:v")
			Header("bearer_token:Authorization")
			Response(StatusOK)
			Response("BadRequest", StatusBadRequest)
			Response("Unauthorized", StatusUnauthorized)
			Response("Forbidden", StatusForbidden)
			Response("NotFound", StatusNotFound)
			Response("InternalServerError", StatusInternalServerError)
			Response("ServiceUnavailable", StatusServiceUnavailable)
		})
	})

	Method("delete-itx-past-meeting", func() {
		Description("Delete a past meeting through ITX API proxy")

		Security(JWTAuth)

		Payload(func() {
			BearerTokenAttribute()
			VersionAttribute()
			Attribute("past_meeting_id", String, "Past meeting ID (meeting_id or meeting_id-occurrence_id)", func() {
				Example("12343245463-1630560600000")
			})
			Required("past_meeting_id")
		})

		Error("BadRequest", BadRequestError, "Bad request")
		Error("Unauthorized", UnauthorizedError, "Unauthorized")
		Error("Forbidden", ForbiddenError, "Forbidden")
		Error("NotFound", NotFoundError, "Past meeting not found")
		Error("InternalServerError", InternalServerError, "Internal server error")
		Error("ServiceUnavailable", ServiceUnavailableError, "Service unavailable")

		HTTP(func() {
			DELETE("/itx/past_meetings/{past_meeting_id}")
			Param("version:v")
			Header("bearer_token:Authorization")
			Response(StatusNoContent)
			Response("BadRequest", StatusBadRequest)
			Response("Unauthorized", StatusUnauthorized)
			Response("Forbidden", StatusForbidden)
			Response("NotFound", StatusNotFound)
			Response("InternalServerError", StatusInternalServerError)
			Response("ServiceUnavailable", StatusServiceUnavailable)
		})
	})

	Method("update-itx-past-meeting", func() {
		Description("Update a past meeting through ITX API proxy")

		Security(JWTAuth)

		Payload(func() {
			BearerTokenAttribute()
			VersionAttribute()
			Attribute("past_meeting_id", String, "Past meeting ID (meeting_id or meeting_id-occurrence_id)", func() {
				Example("12343245463-1630560600000")
			})
			Attribute("project_uid", String, "Project UID (v2)", func() {
				Example("a09eaa48-231b-43e5-93ba-91c2e0a0e5f1")
			})
			Attribute("meeting_id", String, "Zoom meeting ID", func() {
				Example("12343245463")
			})
			Attribute("occurrence_id", String, "Zoom occurrence ID", func() {
				Example("1630560600000")
			})
			Attribute("start_time", String, "Meeting start time in RFC3339 format", func() {
				Example("2024-01-15T10:00:00Z")
				Format(FormatDateTime)
			})
			Attribute("duration", Int, "Meeting duration in minutes", func() {
				Example(60)
				Minimum(1)
			})
			Attribute("timezone", String, "Meeting timezone", func() {
				Example("UTC")
			})
			Attribute("title", String, "Meeting title/topic")
			Attribute("description", String, "Meeting description/agenda")
			Attribute("restricted", Boolean, "Whether the meeting is restricted")
			Attribute("meeting_type", String, "Type of meeting (e.g., regular, webinar)", func() {
				Enum("regular", "webinar")
			})
			Attribute("visibility", String, "Meeting visibility", func() {
				Enum("public", "private")
			})
			Attribute("recording_enabled", Boolean, "Whether recording is enabled")
			Attribute("transcript_enabled", Boolean, "Whether transcript is enabled")
			Attribute("artifact_visibility", String, "Visibility of meeting artifacts (recordings, transcripts)", func() {
				Enum("meeting_hosts", "meeting_participants", "public")
			})
			Attribute("committees", ArrayOf(Committee), "Committees associated with the meeting")
			Required("past_meeting_id")
		})

		Result(ITXPastZoomMeeting)

		Error("BadRequest", BadRequestError, "Bad request")
		Error("Unauthorized", UnauthorizedError, "Unauthorized")
		Error("Forbidden", ForbiddenError, "Forbidden")
		Error("NotFound", NotFoundError, "Past meeting not found")
		Error("InternalServerError", InternalServerError, "Internal server error")
		Error("ServiceUnavailable", ServiceUnavailableError, "Service unavailable")

		HTTP(func() {
			PUT("/itx/past_meetings/{past_meeting_id}")
			Param("version:v")
			Header("bearer_token:Authorization")
			Response(StatusOK)
			Response("BadRequest", StatusBadRequest)
			Response("Unauthorized", StatusUnauthorized)
			Response("Forbidden", StatusForbidden)
			Response("NotFound", StatusNotFound)
			Response("InternalServerError", StatusInternalServerError)
			Response("ServiceUnavailable", StatusServiceUnavailable)
		})
	})

	// Serve the file gen/http/openapi3.json for requests sent to /openapi.json.
	Files("/_meetings/openapi.json", "gen/http/openapi.json", func() {
		Meta("swagger:generate", "false")
	})
	Files("/_meetings/openapi.yaml", "gen/http/openapi.yaml", func() {
		Meta("swagger:generate", "false")
	})
	Files("/_meetings/openapi3.json", "gen/http/openapi3.json", func() {
		Meta("swagger:generate", "false")
	})
	Files("/_meetings/openapi3.yaml", "gen/http/openapi3.yaml", func() {
		Meta("swagger:generate", "false")
	})
})
