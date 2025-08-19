// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	meetingsvc "github.com/linuxfoundation/lfx-v2-meeting-service/gen/meeting_service"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/service"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/constants"
	"goa.design/goa/v3/security"
)

// MeetingsAPI implements the meetingsvc.Service interface
type MeetingsAPI struct {
	service *service.MeetingsService
}

// NewMeetingsAPI creates a new MeetingsAPI.
func NewMeetingsAPI(svc *service.MeetingsService) *MeetingsAPI {
	return &MeetingsAPI{
		service: svc,
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

// Readyz checks if the service is able to take inbound requests.
func (s *MeetingsAPI) Readyz(_ context.Context) ([]byte, error) {
	if !s.service.ServiceReady() {
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
	if !s.service.ServiceReady() {
		return nil, createResponse(http.StatusServiceUnavailable, domain.ErrServiceUnavailable)
	}

	// Parse the Heimdall-authorized principal from the token.
	principal, err := s.service.Auth.ParsePrincipal(ctx, bearerToken, slog.Default())
	if err != nil {
		return ctx, err
	}
	// Return a new context containing the principal as a value.
	return context.WithValue(ctx, constants.PrincipalContextID, principal), nil
}

// ZoomWebhook handles Zoom webhook events for meeting lifecycle, participants, and recordings.
func (s *MeetingsAPI) ZoomWebhook(ctx context.Context, payload *meetingsvc.ZoomWebhookPayload2) (*meetingsvc.ZoomWebhookResponse, error) {
	logger := slog.With("component", "meetings_api", "method", "ZoomWebhook")

	if !s.service.ServiceReady() {
		logger.ErrorContext(ctx, "Service not ready")
		return nil, createResponse(http.StatusServiceUnavailable, domain.ErrServiceUnavailable)
	}

	// Get the Zoom webhook handler from the service's webhook registry
	handler, err := s.service.WebhookRegistry.GetHandler("zoom")
	if err != nil {
		logger.ErrorContext(ctx, "Failed to get Zoom webhook handler", "error", err)
		return nil, createResponse(http.StatusInternalServerError, domain.ErrServiceUnavailable)
	}

	// Validate webhook signature if provided
	if payload.ZoomSignature != nil && payload.ZoomTimestamp != nil {
		// Marshal the body to get the raw bytes for signature validation
		bodyBytes, err := json.Marshal(payload.Body)
		if err != nil {
			logger.ErrorContext(ctx, "Failed to marshal webhook body for validation", "error", err)
			return nil, createResponse(http.StatusBadRequest, err)
		}

		if err := handler.ValidateSignature(bodyBytes, *payload.ZoomSignature, *payload.ZoomTimestamp); err != nil {
			logger.WarnContext(ctx, "Webhook signature validation failed", "error", err)
			return nil, &meetingsvc.UnauthorizedError{
				Code:    "401",
				Message: "Invalid webhook signature",
			}
		}

		logger.DebugContext(ctx, "Webhook signature validation passed")
	}

	// Validate event type and payload
	if payload.Body == nil || payload.Body.Event == "" {
		logger.WarnContext(ctx, "Webhook payload missing event field")
		return nil, createResponse(http.StatusBadRequest, fmt.Errorf("invalid webhook payload: missing event field"))
	}

	eventType := payload.Body.Event
	logger.InfoContext(ctx, "Processing Zoom webhook event", "event_type", eventType)

	// Handle the event using the domain interface
	if err := handler.HandleEvent(ctx, eventType, payload.Body.Payload); err != nil {
		logger.ErrorContext(ctx, "Failed to process webhook event", "error", err, "event_type", eventType)

		// Check error type to return appropriate HTTP status
		if err.Error() == "unsupported event type" {
			return nil, createResponse(http.StatusBadRequest, err)
		}

		return nil, createResponse(http.StatusInternalServerError, err)
	}

	logger.DebugContext(ctx, "Zoom webhook event processed successfully", "event_type", eventType)

	return &meetingsvc.ZoomWebhookResponse{
		Status:  "success",
		Message: stringPtr(fmt.Sprintf("Event %s processed successfully", eventType)),
	}, nil
}

// stringPtr returns a pointer to the given string
func stringPtr(s string) *string {
	return &s
}
