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
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/handlers"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/middleware"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/service"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/constants"
	"goa.design/goa/v3/security"
)

// MeetingsAPI implements the meetingsvc.Service interface
type MeetingsAPI struct {
	authService                   *service.AuthService
	meetingService                *service.MeetingService
	registrantService             *service.MeetingRegistrantService
	meetingRSVPService            *service.MeetingRSVPService
	attachmentService             *service.MeetingAttachmentService
	pastMeetingService            *service.PastMeetingService
	pastMeetingParticipantService *service.PastMeetingParticipantService
	pastMeetingSummaryService     *service.PastMeetingSummaryService
	pastMeetingAttachmentService  *service.PastMeetingAttachmentService
	zoomWebhookService            *service.ZoomWebhookService
	meetingHandler                *handlers.MeetingHandler
	committeeHandler              *handlers.CommitteeHandlers
	zoomWebhookHandler            *handlers.ZoomWebhookHandler
}

// NewMeetingsAPI creates a new MeetingsAPI.
func NewMeetingsAPI(
	authService *service.AuthService,
	meetingService *service.MeetingService,
	registrantService *service.MeetingRegistrantService,
	meetingRSVPService *service.MeetingRSVPService,
	attachmentService *service.MeetingAttachmentService,
	pastMeetingService *service.PastMeetingService,
	pastMeetingParticipantService *service.PastMeetingParticipantService,
	pastMeetingSummaryService *service.PastMeetingSummaryService,
	pastMeetingAttachmentService *service.PastMeetingAttachmentService,
	zoomWebhookService *service.ZoomWebhookService,
	zoomWebhookHandler *handlers.ZoomWebhookHandler,
	meetingHandler *handlers.MeetingHandler,
	committeeHandler *handlers.CommitteeHandlers,
) *MeetingsAPI {
	return &MeetingsAPI{
		authService:                   authService,
		meetingService:                meetingService,
		registrantService:             registrantService,
		meetingRSVPService:            meetingRSVPService,
		attachmentService:             attachmentService,
		pastMeetingService:            pastMeetingService,
		pastMeetingParticipantService: pastMeetingParticipantService,
		pastMeetingSummaryService:     pastMeetingSummaryService,
		pastMeetingAttachmentService:  pastMeetingAttachmentService,
		zoomWebhookService:            zoomWebhookService,
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
// handleError maps domain error types to appropriate HTTP responses
// This function no longer needs updates when new domain errors are added
func handleError(err error) error {
	errorType := domain.GetErrorType(err)

	switch errorType {
	case domain.ErrorTypeValidation:
		return createResponse(http.StatusBadRequest, err)
	case domain.ErrorTypeNotFound:
		return createResponse(http.StatusNotFound, err)
	case domain.ErrorTypeConflict:
		return createResponse(http.StatusConflict, err)
	case domain.ErrorTypeUnavailable:
		return createResponse(http.StatusServiceUnavailable, err)
	case domain.ErrorTypeInternal:
		return createResponse(http.StatusInternalServerError, err)
	default:
		return createResponse(http.StatusInternalServerError, err)
	}
}

// Readyz checks if the service is able to take inbound requests.
func (s *MeetingsAPI) Readyz(_ context.Context) ([]byte, error) {
	if !s.meetingService.ServiceReady() ||
		!s.registrantService.ServiceReady() ||
		!s.pastMeetingService.ServiceReady() ||
		!s.zoomWebhookService.ServiceReady() ||
		!s.zoomWebhookHandler.HandlerReady() ||
		!s.meetingHandler.HandlerReady() {
		return nil, createResponse(http.StatusServiceUnavailable, domain.NewUnavailableError("service unavailable"))
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
		return nil, createResponse(http.StatusServiceUnavailable, domain.NewUnavailableError("service unavailable"))
	}

	// Parse the Heimdall-authorized principal from the token.
	principal, err := s.authService.ParsePrincipal(ctx, bearerToken, slog.Default())
	if err != nil {
		return ctx, err
	}
	// Return a new context containing the principal as a value.
	return context.WithValue(ctx, constants.PrincipalContextID, principal), nil
}

// ZoomWebhook handles Zoom webhook events by delegating to the service layer
func (s *MeetingsAPI) ZoomWebhook(ctx context.Context, payload *meetingsvc.ZoomWebhookPayload) (*meetingsvc.ZoomWebhookResponse, error) {
	logger := slog.With("component", "meetings_api", "method", "ZoomWebhook")

	// Log webhook info with redacted payload to protect PII
	slog.InfoContext(ctx, "Zoom webhook received",
		"event_type", payload.Event,
		"event_ts", payload.EventTs,
		"payload", payload.Payload,
	)

	// Check service readiness
	if !s.zoomWebhookService.ServiceReady() {
		logger.ErrorContext(ctx, "Zoom webhook service not ready")
		return nil, createResponse(http.StatusServiceUnavailable, domain.NewUnavailableError("service unavailable"))
	}

	// Get the raw request body from context for signature validation
	bodyBytes, ok := middleware.GetRawBodyFromContext(ctx)
	if !ok {
		logger.ErrorContext(ctx, "Raw request body not available in context")
		return nil, createResponse(http.StatusInternalServerError, fmt.Errorf("raw body not captured"))
	}

	// Create service request
	req := service.WebhookRequest{
		Event:     payload.Event,
		EventTS:   payload.EventTs,
		Payload:   payload.Payload,
		Signature: payload.ZoomSignature,
		Timestamp: payload.ZoomTimestamp,
		RawBody:   bodyBytes,
	}

	// Delegate to service layer
	response, err := s.zoomWebhookService.ProcessWebhookEvent(ctx, req)
	if err != nil {
		return nil, handleError(err)
	}

	// Convert service response to API response
	return &meetingsvc.ZoomWebhookResponse{
		Status:         response.Status,
		Message:        response.Message,
		PlainToken:     response.PlainToken,
		EncryptedToken: response.EncryptedToken,
	}, nil
}
