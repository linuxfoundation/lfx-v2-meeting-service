// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"

	meetingsvc "github.com/linuxfoundation/lfx-v2-meeting-service/gen/meeting_service"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/service"
	itxservice "github.com/linuxfoundation/lfx-v2-meeting-service/internal/service/itx"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/constants"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/redaction"
	"goa.design/goa/v3/security"
)

// MeetingsAPI implements the meetingsvc.Service interface
type MeetingsAPI struct {
	authService                      *service.AuthService
	itxMeetingService                *itxservice.MeetingService
	itxRegistrantService             *itxservice.RegistrantService
	itxPastMeetingService            *itxservice.PastMeetingService
	itxPastMeetingSummaryService     *itxservice.PastMeetingSummaryService
	itxPastMeetingParticipantService *itxservice.PastMeetingParticipantService
	itxMeetingAttachmentService      *itxservice.MeetingAttachmentService
	itxPastMeetingAttachmentService  *itxservice.PastMeetingAttachmentService
}

// NewMeetingsAPI creates a new MeetingsAPI.
func NewMeetingsAPI(
	authService *service.AuthService,
	itxMeetingService *itxservice.MeetingService,
	itxRegistrantService *itxservice.RegistrantService,
	itxPastMeetingService *itxservice.PastMeetingService,
	itxPastMeetingSummaryService *itxservice.PastMeetingSummaryService,
	itxPastMeetingParticipantService *itxservice.PastMeetingParticipantService,
	itxMeetingAttachmentService *itxservice.MeetingAttachmentService,
	itxPastMeetingAttachmentService *itxservice.PastMeetingAttachmentService,
) *MeetingsAPI {
	return &MeetingsAPI{
		authService:                      authService,
		itxMeetingService:                itxMeetingService,
		itxRegistrantService:             itxRegistrantService,
		itxPastMeetingService:            itxPastMeetingService,
		itxPastMeetingSummaryService:     itxPastMeetingSummaryService,
		itxPastMeetingParticipantService: itxPastMeetingParticipantService,
		itxMeetingAttachmentService:      itxMeetingAttachmentService,
		itxPastMeetingAttachmentService:  itxPastMeetingAttachmentService,
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
	case http.StatusForbidden:
		return &meetingsvc.ForbiddenError{
			Code:    strconv.Itoa(code),
			Message: err.Error(),
		}
	default:
		return nil
	}
}

// handleError converts a domain error to its HTTP representation and logs
// 5xx-class errors (Internal, Unavailable, and unmapped) via slog so that
// every server-side failure is observable in production without the caller
// having to add its own log statement.
func handleError(ctx context.Context, err error) error {
	errorType := domain.GetErrorType(err)

	switch errorType {
	case domain.ErrorTypeValidation:
		slog.WarnContext(ctx, "bad request")
		return createResponse(http.StatusBadRequest, err)
	case domain.ErrorTypeNotFound:
		slog.WarnContext(ctx, "resource not found")
		return createResponse(http.StatusNotFound, err)
	case domain.ErrorTypeConflict:
		slog.WarnContext(ctx, "conflict")
		return createResponse(http.StatusConflict, err)
	case domain.ErrorTypeUnavailable:
		slog.ErrorContext(ctx, "service unavailable error")
		return createResponse(http.StatusServiceUnavailable, err)
	case domain.ErrorTypeForbidden:
		slog.WarnContext(ctx, "forbidden")
		return createResponse(http.StatusForbidden, err)
	case domain.ErrorTypeInternal:
		slog.ErrorContext(ctx, "internal server error")
		return createResponse(http.StatusInternalServerError, err)
	default:
		slog.ErrorContext(ctx, "unhandled error")
		return createResponse(http.StatusInternalServerError, err)
	}
}

// Readyz checks if the service is able to take inbound requests.
func (s *MeetingsAPI) Readyz(_ context.Context) ([]byte, error) {
	// ITX proxy is stateless and always ready
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

	// Parse the Heimdall-authorized principal (and, when present, email claim) from the token.
	principal, email, err := s.authService.ParsePrincipalAndEmail(ctx, bearerToken, slog.Default())
	if err != nil {
		return ctx, err
	}
	// Return a new context containing the principal (and email, if any) as values,
	// and inject the principal into the slog context so it appears in every log
	// line for this request without each call site having to add it manually.
	ctx = context.WithValue(ctx, constants.PrincipalContextID, principal)
	if email != "" {
		ctx = context.WithValue(ctx, constants.EmailContextID, email)
	}
	ctx = logging.AppendCtx(ctx, slog.String("principal", redaction.Redact(principal)))
	return ctx, nil
}
