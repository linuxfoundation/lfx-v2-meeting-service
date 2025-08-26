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
	Description("The meeting service handles all meeting-related operations for LF projects.")

	// TODO: delete this endpoint once the query service supports meeting queries
	// GET all meetings endpoint
	Method("get-meetings", func() {
		Description("Get all meetings.")

		Security(JWTAuth)

		Payload(func() {
			BearerTokenAttribute()
			VersionAttribute()
		})

		Result(func() {
			Attribute("meetings", ArrayOf(MeetingFull), "Resources found", func() {})
			Attribute("cache_control", String, "Cache control header", func() {
				Example("public, max-age=300")
			})
			Required("meetings")
		})

		Error("BadRequest", BadRequestError, "Bad request")
		Error("InternalServerError", InternalServerError, "Internal server error")
		Error("ServiceUnavailable", ServiceUnavailableError, "Service unavailable")

		HTTP(func() {
			GET("/meetings")
			Param("version:v")
			Header("bearer_token:Authorization")
			Response(StatusOK, func() {
				Header("cache_control:Cache-Control")
			})
			Response("BadRequest", StatusBadRequest)
			Response("InternalServerError", StatusInternalServerError)
			Response("ServiceUnavailable", StatusServiceUnavailable)
		})
	})

	// POST meeting endpoint
	Method("create-meeting", func() {
		Description(`Create a new meeting for a project. An actual meeting in the specific platform will be created by
		this endpoint. The meeting's occurrences and registrants are managed by this service rather than the third-party platform.`)

		Security(JWTAuth)

		Payload(func() {
			BearerTokenAttribute()
			VersionAttribute()
			// Meeting fields from CreateMeetingPayload
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
			MeetingOrganizersAttribute()
			Attribute("zoom_config", ZoomConfigPost, "For zoom platform meetings: the configuration for the meeting")
			Required("project_uid", "start_time", "duration", "timezone", "title", "description")
		})

		Result(MeetingFull)

		Error("BadRequest", BadRequestError, "Bad request")
		Error("Conflict", ConflictError, "Conflict")
		Error("InternalServerError", InternalServerError, "Internal server error")
		Error("ServiceUnavailable", ServiceUnavailableError, "Service unavailable")

		HTTP(func() {
			POST("/meetings")
			Param("version:v")
			Header("bearer_token:Authorization")
			Response(StatusCreated)
			Response("BadRequest", StatusBadRequest)
			Response("Conflict", StatusConflict)
			Response("InternalServerError", StatusInternalServerError)
			Response("ServiceUnavailable", StatusServiceUnavailable)
		})
	})

	// GET meeting base by ID endpoint
	Method("get-meeting-base", func() {
		Description("Get a meeting by ID")

		Security(JWTAuth)

		Payload(func() {
			BearerTokenAttribute()
			VersionAttribute()
			MeetingUIDAttribute()
		})

		Result(func() {
			Attribute("meeting", MeetingBase)
			EtagAttribute()
			Required("meeting")
		})

		Error("NotFound", NotFoundError, "Resource not found")
		Error("InternalServerError", InternalServerError, "Internal server error")
		Error("ServiceUnavailable", ServiceUnavailableError, "Service unavailable")

		HTTP(func() {
			GET("/meetings/{uid}")
			Param("version:v")
			Param("uid")
			Header("bearer_token:Authorization")
			Response(StatusOK, func() {
				Body("meeting")
				Header("etag:ETag")
			})
			Response("NotFound", StatusNotFound)
			Response("InternalServerError", StatusInternalServerError)
			Response("ServiceUnavailable", StatusServiceUnavailable)
		})
	})

	// GET meeting settings by ID endpoint
	Method("get-meeting-settings", func() {
		Description("Get a single meeting's settings.")

		Security(JWTAuth)

		Payload(func() {
			BearerTokenAttribute()
			VersionAttribute()
			MeetingUIDAttribute()
		})

		Result(func() {
			Attribute("meeting_settings", MeetingSettings)
			EtagAttribute()
			Required("meeting_settings")
		})

		Error("NotFound", NotFoundError, "Resource not found")
		Error("InternalServerError", InternalServerError, "Internal server error")
		Error("ServiceUnavailable", ServiceUnavailableError, "Service unavailable")

		HTTP(func() {
			GET("/meetings/{uid}/settings")
			Param("version:v")
			Param("uid")
			Header("bearer_token:Authorization")
			Response(StatusOK, func() {
				Body("meeting_settings")
				Header("etag:ETag")
			})
			Response("NotFound", StatusNotFound)
			Response("InternalServerError", StatusInternalServerError)
			Response("ServiceUnavailable", StatusServiceUnavailable)
		})
	})

	// PUT meeting base endpoint by ID
	Method("update-meeting-base", func() {
		Description("Update an existing meeting base.")

		Security(JWTAuth)

		Payload(func() {
			BearerTokenAttribute()
			IfMatchAttribute()
			VersionAttribute()
			// Meeting fields from UpdateMeetingPayload
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
			Attribute("zoom_config", ZoomConfigPost, "For zoom platform meetings: the configuration for the meeting")
			Required("uid", "project_uid", "start_time", "duration", "timezone", "title", "description")
		})

		Result(MeetingBase)

		Error("BadRequest", BadRequestError, "Bad request")
		Error("NotFound", NotFoundError, "Resource not found")
		Error("Conflict", ConflictError, "Conflict")
		Error("InternalServerError", InternalServerError, "Internal server error")
		Error("ServiceUnavailable", ServiceUnavailableError, "Service unavailable")

		HTTP(func() {
			PUT("/meetings/{uid}")
			Params(func() {
				Param("version:v")
				Param("uid")
			})
			Header("bearer_token:Authorization")
			Header("if_match:If-Match")
			Response(StatusOK)
			Response("BadRequest", StatusBadRequest)
			Response("NotFound", StatusNotFound)
			Response("Conflict", StatusConflict)
			Response("InternalServerError", StatusInternalServerError)
			Response("ServiceUnavailable", StatusServiceUnavailable)
		})
	})

	Method("update-meeting-settings", func() {
		Description("Update an existing meeting's settings.")

		Security(JWTAuth)

		Payload(func() {
			BearerTokenAttribute()
			IfMatchAttribute()
			VersionAttribute()
			MeetingUIDAttribute()
			MeetingOrganizersAttribute()
		})

		Result(MeetingSettings)

		Error("BadRequest", BadRequestError, "Bad request")
		Error("NotFound", NotFoundError, "Resource not found")
		Error("InternalServerError", InternalServerError, "Internal server error")
		Error("ServiceUnavailable", ServiceUnavailableError, "Service unavailable")

		HTTP(func() {
			PUT("/meetings/{uid}/settings")
			Params(func() {
				Param("version:v")
				Param("uid")
			})
			Header("bearer_token:Authorization")
			Header("if_match:If-Match")
			Response(StatusOK)
			Response("BadRequest", StatusBadRequest)
			Response("NotFound", StatusNotFound)
			Response("InternalServerError", StatusInternalServerError)
			Response("ServiceUnavailable", StatusServiceUnavailable)
		})
	})

	// DELETE meeting endpoint by ID
	Method("delete-meeting", func() {
		Description("Delete an existing meeting.")

		Security(JWTAuth)

		Payload(func() {
			BearerTokenAttribute()
			IfMatchAttribute()
			VersionAttribute()
			MeetingUIDAttribute()
		})

		Error("NotFound", NotFoundError, "Resource not found")
		Error("BadRequest", BadRequestError, "Bad request")
		Error("InternalServerError", InternalServerError, "Internal server error")
		Error("ServiceUnavailable", ServiceUnavailableError, "Service unavailable")

		HTTP(func() {
			DELETE("/meetings/{uid}")
			Params(func() {
				Param("version:v")
				Param("uid")
			})
			Header("bearer_token:Authorization")
			Header("if_match:If-Match")
			Response(StatusNoContent)
			Response("NotFound", StatusNotFound)
			Response("BadRequest", StatusBadRequest)
			Response("InternalServerError", StatusInternalServerError)
			Response("ServiceUnavailable", StatusServiceUnavailable)
		})
	})

	// TODO: delete this endpoint once the query service supports meeting registrant queries
	// GET meeting registrants endpoint
	Method("get-meeting-registrants", func() {
		Description("Get all registrants for a meeting")

		Security(JWTAuth)

		Payload(func() {
			BearerTokenAttribute()
			VersionAttribute()
			MeetingUIDAttribute()
		})

		Result(func() {
			Attribute("registrants", ArrayOf(Registrant), "Meeting registrants")
			Attribute("cache_control", String, "Cache control header", func() {
				Example("public, max-age=300")
			})
			Required("registrants")
		})

		Error("NotFound", NotFoundError, "Meeting not found")
		Error("InternalServerError", InternalServerError, "Internal server error")
		Error("ServiceUnavailable", ServiceUnavailableError, "Service unavailable")

		HTTP(func() {
			GET("/meetings/{uid}/registrants")
			Param("version:v")
			Param("uid")
			Header("bearer_token:Authorization")
			Response(StatusOK, func() {
				Header("cache_control:Cache-Control")
			})
			Response("NotFound", StatusNotFound)
			Response("InternalServerError", StatusInternalServerError)
			Response("ServiceUnavailable", StatusServiceUnavailable)
		})
	})

	// POST meeting registrant endpoint
	Method("create-meeting-registrant", func() {
		Description("Create a new registrant for a meeting")

		Security(JWTAuth)

		Payload(func() {
			Extend(CreateRegistrantPayload)
			BearerTokenAttribute()
			VersionAttribute()
			RegistrantMeetingUIDAttribute()
		})

		Result(Registrant)

		Error("BadRequest", BadRequestError, "Bad request")
		Error("NotFound", NotFoundError, "Meeting not found")
		Error("Conflict", ConflictError, "Registrant already exists")
		Error("InternalServerError", InternalServerError, "Internal server error")
		Error("ServiceUnavailable", ServiceUnavailableError, "Service unavailable")

		HTTP(func() {
			POST("/meetings/{meeting_uid}/registrants")
			Param("version:v")
			Param("meeting_uid")
			Header("bearer_token:Authorization")
			Response(StatusCreated)
			Response("BadRequest", StatusBadRequest)
			Response("NotFound", StatusNotFound)
			Response("Conflict", StatusConflict)
			Response("InternalServerError", StatusInternalServerError)
			Response("ServiceUnavailable", StatusServiceUnavailable)
		})
	})

	// GET meeting registrant by UID endpoint
	Method("get-meeting-registrant", func() {
		Description("Get a specific registrant for a meeting by UID")

		Security(JWTAuth)

		Payload(func() {
			BearerTokenAttribute()
			VersionAttribute()
			RegistrantMeetingUIDAttribute()
			RegistrantUIDAttribute()
		})

		Result(func() {
			Attribute("registrant", Registrant)
			EtagAttribute()
			Required("registrant")
		})

		Error("NotFound", NotFoundError, "Meeting or registrant not found")
		Error("InternalServerError", InternalServerError, "Internal server error")
		Error("ServiceUnavailable", ServiceUnavailableError, "Service unavailable")

		HTTP(func() {
			GET("/meetings/{meeting_uid}/registrants/{uid}")
			Param("version:v")
			Param("meeting_uid")
			Param("uid")
			Header("bearer_token:Authorization")
			Response(StatusOK, func() {
				Body("registrant")
				Header("etag:ETag")
			})
			Response("NotFound", StatusNotFound)
			Response("InternalServerError", StatusInternalServerError)
			Response("ServiceUnavailable", StatusServiceUnavailable)
		})
	})

	// PUT meeting registrant endpoint
	Method("update-meeting-registrant", func() {
		Description("Update an existing registrant for a meeting")

		Security(JWTAuth)

		Payload(func() {
			Extend(UpdateRegistrantPayload)
			BearerTokenAttribute()
			IfMatchAttribute()
			VersionAttribute()
			RegistrantMeetingUIDAttribute()
			RegistrantUIDAttribute()
		})

		Result(Registrant)

		Error("BadRequest", BadRequestError, "Bad request")
		Error("NotFound", NotFoundError, "Meeting or registrant not found")
		Error("Conflict", ConflictError, "Conflict")
		Error("InternalServerError", InternalServerError, "Internal server error")
		Error("ServiceUnavailable", ServiceUnavailableError, "Service unavailable")

		HTTP(func() {
			PUT("/meetings/{meeting_uid}/registrants/{uid}")
			Params(func() {
				Param("version:v")
				Param("meeting_uid")
				Param("uid")
			})
			Header("bearer_token:Authorization")
			Header("if_match:If-Match")
			Response(StatusOK)
			Response("BadRequest", StatusBadRequest)
			Response("NotFound", StatusNotFound)
			Response("Conflict", StatusConflict)
			Response("InternalServerError", StatusInternalServerError)
			Response("ServiceUnavailable", StatusServiceUnavailable)
		})
	})

	// DELETE meeting registrant endpoint
	Method("delete-meeting-registrant", func() {
		Description("Delete a registrant from a meeting")

		Security(JWTAuth)

		Payload(func() {
			BearerTokenAttribute()
			IfMatchAttribute()
			VersionAttribute()
			RegistrantMeetingUIDAttribute()
			RegistrantUIDAttribute()
		})

		Error("NotFound", NotFoundError, "Meeting or registrant not found")
		Error("BadRequest", BadRequestError, "Bad request")
		Error("InternalServerError", InternalServerError, "Internal server error")
		Error("ServiceUnavailable", ServiceUnavailableError, "Service unavailable")

		HTTP(func() {
			DELETE("/meetings/{meeting_uid}/registrants/{uid}")
			Params(func() {
				Param("version:v")
				Param("meeting_uid")
				Param("uid")
			})
			Header("bearer_token:Authorization")
			Header("if_match:If-Match")
			Response(StatusNoContent)
			Response("NotFound", StatusNotFound)
			Response("BadRequest", StatusBadRequest)
			Response("InternalServerError", StatusInternalServerError)
			Response("ServiceUnavailable", StatusServiceUnavailable)
		})
	})

	Method("zoom-webhook", func() {
		Description("Handle Zoom webhook events for meeting lifecycle, participants, and recordings.")

		// No authentication required for webhooks - validation is done via signature
		NoSecurity()

		Payload(ZoomWebhookPayload)

		Result(ZoomWebhookResponse)

		Error("BadRequest", BadRequestError, "Invalid webhook payload or signature")
		Error("Unauthorized", UnauthorizedError, "Invalid webhook signature")
		Error("InternalServerError", InternalServerError, "Internal server error")

		HTTP(func() {
			POST("/webhooks/zoom")
			Header("zoom_signature:x-zm-signature")
			Header("zoom_timestamp:x-zm-request-timestamp")
			Response(StatusOK)
			Response("BadRequest", StatusBadRequest)
			Response("Unauthorized", StatusUnauthorized)
			Response("InternalServerError", StatusInternalServerError)
		})
	})

	// GET all past meetings endpoint
	Method("get-past-meetings", func() {
		Description("Get all past meetings.")

		Security(JWTAuth)

		Payload(func() {
			BearerTokenAttribute()
			VersionAttribute()
		})

		Result(func() {
			Attribute("past_meetings", ArrayOf(PastMeeting), "Past meetings found")
			Attribute("cache_control", String, "Cache control header", func() {
				Example("public, max-age=300")
			})
			Required("past_meetings")
		})

		Error("BadRequest", BadRequestError, "Bad request")
		Error("InternalServerError", InternalServerError, "Internal server error")
		Error("ServiceUnavailable", ServiceUnavailableError, "Service unavailable")

		HTTP(func() {
			GET("/past_meetings")
			Param("version:v")
			Header("bearer_token:Authorization")
			Response(StatusOK, func() {
				Header("cache_control:Cache-Control")
			})
			Response("BadRequest", StatusBadRequest)
			Response("InternalServerError", StatusInternalServerError)
			Response("ServiceUnavailable", StatusServiceUnavailable)
		})
	})

	// POST past meeting endpoint
	Method("create-past-meeting", func() {
		Description("Create a new past meeting record. This allows manual addition of past meetings that didn't come from webhooks.")

		Security(JWTAuth)

		Payload(func() {
			Extend(CreatePastMeetingPayload)
			BearerTokenAttribute()
			VersionAttribute()
		})

		Result(PastMeeting)

		Error("BadRequest", BadRequestError, "Bad request")
		Error("Conflict", ConflictError, "Past meeting already exists")
		Error("InternalServerError", InternalServerError, "Internal server error")
		Error("ServiceUnavailable", ServiceUnavailableError, "Service unavailable")

		HTTP(func() {
			POST("/past_meetings")
			Param("version:v")
			Header("bearer_token:Authorization")
			Response(StatusCreated)
			Response("BadRequest", StatusBadRequest)
			Response("Conflict", StatusConflict)
			Response("InternalServerError", StatusInternalServerError)
			Response("ServiceUnavailable", StatusServiceUnavailable)
		})
	})

	// GET past meeting by ID endpoint
	Method("get-past-meeting", func() {
		Description("Get a past meeting by ID")

		Security(JWTAuth)

		Payload(func() {
			BearerTokenAttribute()
			VersionAttribute()
			PastMeetingUIDAttribute()
		})

		Result(func() {
			Attribute("past_meeting", PastMeeting)
			EtagAttribute()
			Required("past_meeting")
		})

		Error("NotFound", NotFoundError, "Past meeting not found")
		Error("InternalServerError", InternalServerError, "Internal server error")
		Error("ServiceUnavailable", ServiceUnavailableError, "Service unavailable")

		HTTP(func() {
			GET("/past_meetings/{uid}")
			Param("version:v")
			Param("uid")
			Header("bearer_token:Authorization")
			Response(StatusOK, func() {
				Body("past_meeting")
				Header("etag:ETag")
			})
			Response("NotFound", StatusNotFound)
			Response("InternalServerError", StatusInternalServerError)
			Response("ServiceUnavailable", StatusServiceUnavailable)
		})
	})

	// DELETE past meeting endpoint by ID
	Method("delete-past-meeting", func() {
		Description("Delete an existing past meeting.")

		Security(JWTAuth)

		Payload(func() {
			BearerTokenAttribute()
			IfMatchAttribute()
			VersionAttribute()
			PastMeetingUIDAttribute()
		})

		Error("NotFound", NotFoundError, "Past meeting not found")
		Error("BadRequest", BadRequestError, "Bad request")
		Error("InternalServerError", InternalServerError, "Internal server error")
		Error("ServiceUnavailable", ServiceUnavailableError, "Service unavailable")

		HTTP(func() {
			DELETE("/past_meetings/{uid}")
			Params(func() {
				Param("version:v")
				Param("uid")
			})
			Header("bearer_token:Authorization")
			Header("if_match:If-Match")
			Response(StatusNoContent)
			Response("NotFound", StatusNotFound)
			Response("BadRequest", StatusBadRequest)
			Response("InternalServerError", StatusInternalServerError)
			Response("ServiceUnavailable", StatusServiceUnavailable)
		})
	})

	// TODO: delete this endpoint once the query service supports meeting registrant queries
	// GET past meeting participants endpoint
	Method("get-past-meeting-participants", func() {
		Description("Get all participants for a past meeting")

		Security(JWTAuth)

		Payload(func() {
			BearerTokenAttribute()
			VersionAttribute()
			PastMeetingUIDAttribute()
		})

		Result(func() {
			Attribute("participants", ArrayOf(PastMeetingParticipant), "Past meeting participants")
			Attribute("cache_control", String, "Cache control header", func() {
				Example("public, max-age=300")
			})
			Required("participants")
		})

		Error("NotFound", NotFoundError, "Past meeting not found")
		Error("InternalServerError", InternalServerError, "Internal server error")
		Error("ServiceUnavailable", ServiceUnavailableError, "Service unavailable")

		HTTP(func() {
			GET("/past_meetings/{uid}/participants")
			Param("version:v")
			Param("uid")
			Header("bearer_token:Authorization")
			Response(StatusOK, func() {
				Header("cache_control:Cache-Control")
			})
			Response("NotFound", StatusNotFound)
			Response("InternalServerError", StatusInternalServerError)
			Response("ServiceUnavailable", StatusServiceUnavailable)
		})
	})

	// POST past meeting participant endpoint
	Method("create-past-meeting-participant", func() {
		Description("Create a new participant for a past meeting")

		Security(JWTAuth)

		Payload(func() {
			Extend(CreatePastMeetingParticipantPayload)
			BearerTokenAttribute()
			VersionAttribute()
			PastMeetingUIDAttribute()
		})

		Result(PastMeetingParticipant)

		Error("BadRequest", BadRequestError, "Bad request")
		Error("NotFound", NotFoundError, "Past meeting not found")
		Error("Conflict", ConflictError, "Past meeting participant already exists")
		Error("InternalServerError", InternalServerError, "Internal server error")
		Error("ServiceUnavailable", ServiceUnavailableError, "Service unavailable")

		HTTP(func() {
			POST("/past_meetings/{uid}/participants")
			Param("version:v")
			Param("uid")
			Header("bearer_token:Authorization")
			Response(StatusCreated)
			Response("BadRequest", StatusBadRequest)
			Response("NotFound", StatusNotFound)
			Response("Conflict", StatusConflict)
			Response("InternalServerError", StatusInternalServerError)
			Response("ServiceUnavailable", StatusServiceUnavailable)
		})
	})

	// GET past meeting participant by UID endpoint
	Method("get-past-meeting-participant", func() {
		Description("Get a specific participant for a past meeting by UID")

		Security(JWTAuth)

		Payload(func() {
			BearerTokenAttribute()
			VersionAttribute()
			PastMeetingParticipantPastMeetingUIDAttribute()
			PastMeetingParticipantUIDAttribute()
		})

		Result(func() {
			Attribute("participant", PastMeetingParticipant)
			EtagAttribute()
			Required("participant")
		})

		Error("NotFound", NotFoundError, "Past meeting or participant not found")
		Error("InternalServerError", InternalServerError, "Internal server error")
		Error("ServiceUnavailable", ServiceUnavailableError, "Service unavailable")

		HTTP(func() {
			GET("/past_meetings/{past_meeting_uid}/participants/{uid}")
			Param("version:v")
			Param("past_meeting_uid")
			Param("uid")
			Header("bearer_token:Authorization")
			Response(StatusOK, func() {
				Body("participant")
				Header("etag:ETag")
			})
			Response("NotFound", StatusNotFound)
			Response("InternalServerError", StatusInternalServerError)
			Response("ServiceUnavailable", StatusServiceUnavailable)
		})
	})

	// PUT past meeting participant endpoint
	Method("update-past-meeting-participant", func() {
		Description("Update an existing participant for a past meeting")

		Security(JWTAuth)

		Payload(func() {
			Extend(UpdatePastMeetingParticipantPayload)
			BearerTokenAttribute()
			IfMatchAttribute()
			VersionAttribute()
			PastMeetingParticipantPastMeetingUIDAttribute()
			PastMeetingParticipantUIDAttribute()
		})

		Result(PastMeetingParticipant)

		Error("BadRequest", BadRequestError, "Bad request")
		Error("NotFound", NotFoundError, "Past meeting or participant not found")
		Error("Conflict", ConflictError, "Conflict")
		Error("InternalServerError", InternalServerError, "Internal server error")
		Error("ServiceUnavailable", ServiceUnavailableError, "Service unavailable")

		HTTP(func() {
			PUT("/past_meetings/{past_meeting_uid}/participants/{uid}")
			Params(func() {
				Param("version:v")
				Param("past_meeting_uid") // past meeting uid
				Param("uid")              // past meeting participant uid
			})
			Header("bearer_token:Authorization")
			Header("if_match:If-Match")
			Response(StatusOK)
			Response("BadRequest", StatusBadRequest)
			Response("NotFound", StatusNotFound)
			Response("Conflict", StatusConflict)
			Response("InternalServerError", StatusInternalServerError)
			Response("ServiceUnavailable", StatusServiceUnavailable)
		})
	})

	// DELETE past meeting participant endpoint
	Method("delete-past-meeting-participant", func() {
		Description("Delete a participant from a past meeting")

		Security(JWTAuth)

		Payload(func() {
			BearerTokenAttribute()
			IfMatchAttribute()
			VersionAttribute()
			PastMeetingParticipantPastMeetingUIDAttribute()
			PastMeetingParticipantUIDAttribute()
		})

		Error("NotFound", NotFoundError, "Past meeting or participant not found")
		Error("BadRequest", BadRequestError, "Bad request")
		Error("InternalServerError", InternalServerError, "Internal server error")
		Error("ServiceUnavailable", ServiceUnavailableError, "Service unavailable")

		HTTP(func() {
			DELETE("/past_meetings/{past_meeting_uid}/participants/{uid}")
			Params(func() {
				Param("version:v")
				Param("past_meeting_uid") // past meeting uid
				Param("uid")              // past meeting participant uid
			})
			Header("bearer_token:Authorization")
			Header("if_match:If-Match")
			Response(StatusNoContent)
			Response("NotFound", StatusNotFound)
			Response("BadRequest", StatusBadRequest)
			Response("InternalServerError", StatusInternalServerError)
			Response("ServiceUnavailable", StatusServiceUnavailable)
		})
	})

	Method("readyz", func() {
		Description("Check if the service is able to take inbound requests.")
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

	// Serve the file gen/http/openapi3.json for requests sent to /openapi.json.
	Files("/openapi.json", "gen/http/openapi3.json")
})
