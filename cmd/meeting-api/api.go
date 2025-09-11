// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	meetingsvc "github.com/linuxfoundation/lfx-v2-meeting-service/gen/meeting_service"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/handlers"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/middleware"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/service"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/constants"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/utils"
	"goa.design/goa/v3/security"
)

// MeetingsAPI implements the meetingsvc.Service interface
type MeetingsAPI struct {
	authService                   *service.AuthService
	meetingService                *service.MeetingService
	registrantService             *service.MeetingRegistrantService
	pastMeetingService            *service.PastMeetingService
	pastMeetingParticipantService *service.PastMeetingParticipantService
	pastMeetingSummaryService     *service.PastMeetingSummaryService
	meetingHandler                *handlers.MeetingHandler
	committeeHandler              *handlers.CommitteeHandlers
	zoomWebhookHandler            *handlers.ZoomWebhookHandler
}

// NewMeetingsAPI creates a new MeetingsAPI.
func NewMeetingsAPI(
	authService *service.AuthService,
	meetingService *service.MeetingService,
	registrantService *service.MeetingRegistrantService,
	pastMeetingService *service.PastMeetingService,
	pastMeetingParticipantService *service.PastMeetingParticipantService,
	pastMeetingSummaryService *service.PastMeetingSummaryService,
	zoomWebhookHandler *handlers.ZoomWebhookHandler,
	meetingHandler *handlers.MeetingHandler,
	committeeHandler *handlers.CommitteeHandlers,
) *MeetingsAPI {
	return &MeetingsAPI{
		authService:                   authService,
		meetingService:                meetingService,
		registrantService:             registrantService,
		pastMeetingService:            pastMeetingService,
		pastMeetingParticipantService: pastMeetingParticipantService,
		pastMeetingSummaryService:     pastMeetingSummaryService,
		zoomWebhookHandler:            zoomWebhookHandler,
		meetingHandler:                meetingHandler,
		committeeHandler:              committeeHandler,
	}
}

// createResponse creates a response error based on the HTTP status code.
func createResponse(code int, err error) error {
	switch code {
	case http.StatusBadRequest:
		return &meetingsvc.BadRequestError{
			Code:    strconv.Itoa(code),
			Message: err.Error(),
		}
	case http.StatusNotFound:
		return &meetingsvc.NotFoundError{
			Code:    strconv.Itoa(code),
			Message: err.Error(),
		}
	case http.StatusConflict:
		return &meetingsvc.ConflictError{
			Code:    strconv.Itoa(code),
			Message: err.Error(),
		}
	case http.StatusInternalServerError:
		return &meetingsvc.InternalServerError{
			Code:    strconv.Itoa(code),
			Message: err.Error(),
		}
	case http.StatusServiceUnavailable:
		return &meetingsvc.ServiceUnavailableError{
			Code:    strconv.Itoa(code),
			Message: err.Error(),
		}
	default:
		return nil
	}
}

// handleError converts domain errors to HTTP errors.
// TODO: figure out solution where we don't need to update this function when new errors are added.
// Resolved once
func handleError(err error) error {
	switch err {
	case domain.ErrServiceUnavailable:
		return createResponse(http.StatusServiceUnavailable, domain.ErrServiceUnavailable)
	case domain.ErrValidationFailed:
		return createResponse(http.StatusBadRequest, domain.ErrValidationFailed)
	case domain.ErrRevisionMismatch:
		return createResponse(http.StatusBadRequest, domain.ErrRevisionMismatch)
	case domain.ErrRegistrantAlreadyExists:
		return createResponse(http.StatusConflict, domain.ErrRegistrantAlreadyExists)
	case domain.ErrPastMeetingParticipantAlreadyExists:
		return createResponse(http.StatusConflict, domain.ErrPastMeetingParticipantAlreadyExists)
	case domain.ErrMeetingNotFound:
		return createResponse(http.StatusNotFound, domain.ErrMeetingNotFound)
	case domain.ErrRegistrantNotFound:
		return createResponse(http.StatusNotFound, domain.ErrRegistrantNotFound)
	case domain.ErrPastMeetingNotFound:
		return createResponse(http.StatusNotFound, domain.ErrPastMeetingNotFound)
	case domain.ErrPastMeetingParticipantNotFound:
		return createResponse(http.StatusNotFound, domain.ErrPastMeetingParticipantNotFound)
	case domain.ErrPastMeetingSummaryNotFound:
		return createResponse(http.StatusNotFound, domain.ErrPastMeetingSummaryNotFound)
	case domain.ErrInternal, domain.ErrUnmarshal:
		return createResponse(http.StatusInternalServerError, domain.ErrInternal)
	}
	return err
}

// Readyz checks if the service is able to take inbound requests.
func (s *MeetingsAPI) Readyz(_ context.Context) ([]byte, error) {
	if !s.meetingService.ServiceReady() ||
		!s.registrantService.ServiceReady() ||
		!s.pastMeetingService.ServiceReady() ||
		!s.zoomWebhookHandler.HandlerReady() ||
		!s.meetingHandler.HandlerReady() {
		return nil, createResponse(http.StatusServiceUnavailable, domain.ErrServiceUnavailable)
	}
	return []byte("OK\n"), nil
}

// Livez checks if the service is alive.
func (s *MeetingsAPI) Livez(_ context.Context) ([]byte, error) {
	// This always returns as long as the service is still running. As this
	// endpoint is expected to be used as a Kubernetes liveness check, this
	// service must likewise self-detect non-recoverable errors and
	// self-terminate.
	return []byte("OK\n"), nil
}

// JWTAuth implements Auther interface for the JWT security scheme.
func (s *MeetingsAPI) JWTAuth(ctx context.Context, bearerToken string, _ *security.JWTScheme) (context.Context, error) {
	if !s.authService.ServiceReady() {
		return nil, createResponse(http.StatusServiceUnavailable, domain.ErrServiceUnavailable)
	}

	// Parse the Heimdall-authorized principal from the token.
	principal, err := s.authService.Auth.ParsePrincipal(ctx, bearerToken, slog.Default())
	if err != nil {
		return ctx, err
	}
	// Return a new context containing the principal as a value.
	return context.WithValue(ctx, constants.PrincipalContextID, principal), nil
}

// ZoomWebhook handles Zoom webhook events by validating signatures and forwarding to NATS for async processing.
// TODO: consider refactoring logic in this function to be done in the service layer. Ideally the application layer
// shouldn't have to use the MessageBuilder or WebhookValidator directly.
func (s *MeetingsAPI) ZoomWebhook(ctx context.Context, payload *meetingsvc.ZoomWebhookPayload) (*meetingsvc.ZoomWebhookResponse, error) {
	logger := slog.With("component", "meetings_api", "method", "ZoomWebhook")
	slog.InfoContext(ctx, "Zoom webhook payload", "payload", payload)

	if !s.zoomWebhookHandler.HandlerReady() {
		logger.ErrorContext(ctx, "Service not ready")
		return nil, createResponse(http.StatusServiceUnavailable, domain.ErrServiceUnavailable)
	}

	// Validate Zoom webhook signature if provided
	if payload.ZoomSignature != nil && payload.ZoomTimestamp != nil {
		// Check if Zoom webhook validator is configured
		if s.zoomWebhookHandler.WebhookValidator == nil {
			logger.ErrorContext(ctx, "Zoom webhook validator not configured")
			return nil, createResponse(http.StatusInternalServerError, fmt.Errorf("zoom webhook validation not configured"))
		}

		// Get the raw request body from context for signature validation
		bodyBytes, ok := middleware.GetRawBodyFromContext(ctx)
		if !ok {
			logger.ErrorContext(ctx, "Raw request body not available in context")
			return nil, createResponse(http.StatusInternalServerError, fmt.Errorf("raw body not captured"))
		}

		if err := s.zoomWebhookHandler.WebhookValidator.ValidateSignature(bodyBytes, *payload.ZoomSignature, *payload.ZoomTimestamp); err != nil {
			logger.WarnContext(ctx, "Zoom webhook signature validation failed", "error", err)
			return nil, &meetingsvc.UnauthorizedError{
				Code:    "401",
				Message: "Invalid webhook signature",
			}
		}

		logger.DebugContext(ctx, "Zoom webhook signature validation passed")
	}

	// Validate event type and payload
	if payload.Payload == nil || payload.Event == "" {
		logger.WarnContext(ctx, "Webhook payload missing event field")
		return nil, createResponse(http.StatusBadRequest, fmt.Errorf("invalid webhook payload: missing event field"))
	}

	eventType := payload.Event

	// Map event type to NATS subject
	subject := getZoomWebhookSubject(eventType)
	if subject == "" {
		logger.WarnContext(ctx, "Unsupported Zoom webhook event type", "event_type", eventType)
		return nil, createResponse(http.StatusBadRequest, fmt.Errorf("unsupported event type: %s", eventType))
	}

	// Create webhook event message for NATS
	payloadMap, ok := payload.Payload.(map[string]interface{})
	if !ok {
		logger.ErrorContext(ctx, "Webhook payload is not a valid map", "payload_type", fmt.Sprintf("%T", payload.Payload))
		return nil, createResponse(http.StatusBadRequest, fmt.Errorf("invalid webhook payload format"))
	}

	webhookMessage := models.ZoomWebhookEventMessage{
		EventType: eventType,
		EventTS:   payload.EventTs,
		Payload:   payloadMap,
	}

	// Publish to NATS for async processing
	if err := s.meetingService.MessageBuilder.PublishZoomWebhookEvent(ctx, subject, webhookMessage); err != nil {
		logger.ErrorContext(ctx, "Failed to publish webhook event to NATS", "error", err, "event_type", eventType, "subject", subject)
		return nil, createResponse(http.StatusInternalServerError, fmt.Errorf("failed to process webhook event"))
	}

	logger.InfoContext(ctx, "Zoom webhook event published to NATS successfully", "event_type", eventType, "subject", subject)

	return &meetingsvc.ZoomWebhookResponse{
		Status:  "success",
		Message: utils.StringPtr(fmt.Sprintf("Event %s queued for processing", eventType)),
	}, nil
}

// getZoomWebhookSubject maps Zoom event types to NATS subjects
func getZoomWebhookSubject(eventType string) string {
	eventSubjectMap := map[string]string{
		"meeting.started":                models.ZoomWebhookMeetingStartedSubject,
		"meeting.ended":                  models.ZoomWebhookMeetingEndedSubject,
		"meeting.deleted":                models.ZoomWebhookMeetingDeletedSubject,
		"meeting.participant_joined":     models.ZoomWebhookMeetingParticipantJoinedSubject,
		"meeting.participant_left":       models.ZoomWebhookMeetingParticipantLeftSubject,
		"recording.completed":            models.ZoomWebhookRecordingCompletedSubject,
		"recording.transcript_completed": models.ZoomWebhookRecordingTranscriptCompletedSubject,
		"meeting.summary_completed":      models.ZoomWebhookMeetingSummaryCompletedSubject,
	}

	return eventSubjectMap[eventType]
}
