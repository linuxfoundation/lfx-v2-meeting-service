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

	slog.InfoContext(ctx, "creating meeting rsvp", "token", *payload.BearerToken)

	// Parse username from JWT token if not provided in payload
	username, err := s.authService.ParseUsername(ctx, *payload.BearerToken, slog.Default())
	if err != nil {
		slog.WarnContext(ctx, "failed to parse username from JWT token", "error", err)
		return nil, handleError(domain.NewValidationError("failed to parse username from authorization token"))
	}
	payload.Username = &username
	slog.DebugContext(ctx, "parsed username from JWT token", "username", username)

	req := service.ConvertCreateRSVPPayloadToDomain(payload)

	rsvp, err := s.meetingRSVPService.PutRSVP(ctx, req)
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
