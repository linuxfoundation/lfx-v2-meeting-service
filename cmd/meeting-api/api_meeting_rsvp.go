// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"log/slog"

	"github.com/linuxfoundation/lfx-v2-meeting-service/cmd/meeting-api/service"
	meetingsvc "github.com/linuxfoundation/lfx-v2-meeting-service/gen/meeting_service"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
)

// CreateMeetingRsvp creates or updates an RSVP response for a meeting registrant
func (s *MeetingsAPI) CreateMeetingRsvp(ctx context.Context, payload *meetingsvc.CreateMeetingRsvpPayload) (*meetingsvc.RSVPResponse, error) {
	if payload == nil {
		return nil, handleError(domain.NewValidationError("validation failed"))
	}

	sync := payload.XSync != nil && *payload.XSync

	// Parse username from JWT token (principal is the username in Heimdall)
	username, err := s.authService.ParsePrincipal(ctx, *payload.BearerToken, slog.Default())
	if err != nil {
		slog.WarnContext(ctx, "failed to parse username from JWT token", "error", err)
		return nil, handleError(domain.NewValidationError("failed to parse username from authorization token"))
	}
	payload.Username = &username

	req := service.ConvertCreateRSVPPayloadToDomain(payload)

	rsvp, err := s.meetingRSVPService.PutRSVP(ctx, req, sync)
	if err != nil {
		return nil, handleError(err)
	}

	return service.ConvertRSVPToResponse(rsvp), nil
}

// GetMeetingRsvps retrieves all RSVP responses for a meeting
func (s *MeetingsAPI) GetMeetingRsvps(ctx context.Context, payload *meetingsvc.GetMeetingRsvpsPayload) (*meetingsvc.RSVPListResult, error) {
	if payload == nil || payload.MeetingUID == "" {
		return nil, handleError(domain.NewValidationError("validation failed"))
	}

	rsvps, err := s.meetingRSVPService.GetMeetingRSVPs(ctx, payload.MeetingUID)
	if err != nil {
		return nil, handleError(err)
	}

	var respRSVPs []*meetingsvc.RSVPResponse
	for _, rsvp := range rsvps {
		respRSVPs = append(respRSVPs, service.ConvertRSVPToResponse(rsvp))
	}

	return &meetingsvc.RSVPListResult{
		Rsvps: respRSVPs,
	}, nil
}
