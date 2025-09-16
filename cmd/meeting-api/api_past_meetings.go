// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package main

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-meeting-service/cmd/meeting-api/service"
	meetingsvc "github.com/linuxfoundation/lfx-v2-meeting-service/gen/meeting_service"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/utils"
)

// GetPastMeetings implements the Goa service interface for listing past meetings
func (s *MeetingsAPI) GetPastMeetings(ctx context.Context, payload *meetingsvc.GetPastMeetingsPayload) (*meetingsvc.GetPastMeetingsResult, error) {
	if payload == nil {
		return nil, handleError(domain.NewValidationError("validation failed"))
	}

	pastMeetings, err := s.pastMeetingService.GetPastMeetings(ctx)
	if err != nil {
		return nil, handleError(err)
	}

	var pastMeetingsResp []*meetingsvc.PastMeeting
	for _, pastMeeting := range pastMeetings {
		pastMeetingsResp = append(pastMeetingsResp, service.ConvertDomainToPastMeetingResponse(pastMeeting))
	}

	result := &meetingsvc.GetPastMeetingsResult{
		PastMeetings: pastMeetingsResp,
		CacheControl: nil,
	}

	return result, nil
}

// CreatePastMeeting implements the Goa service interface for creating past meetings
func (s *MeetingsAPI) CreatePastMeeting(ctx context.Context, payload *meetingsvc.CreatePastMeetingPayload) (*meetingsvc.PastMeeting, error) {
	if payload == nil {
		return nil, handleError(domain.NewValidationError("validation failed"))
	}

	createPastMeetingReq := service.ConvertCreatePastMeetingPayloadToDomain(payload)
	if createPastMeetingReq == nil {
		return nil, handleError(domain.NewValidationError("validation failed"))
	}

	pastMeeting, err := s.pastMeetingService.CreatePastMeeting(ctx, createPastMeetingReq)
	if err != nil {
		return nil, handleError(err)
	}

	return service.ConvertDomainToPastMeetingResponse(pastMeeting), nil
}

// GetPastMeeting implements the Goa service interface for getting a single past meeting
func (s *MeetingsAPI) GetPastMeeting(ctx context.Context, payload *meetingsvc.GetPastMeetingPayload) (*meetingsvc.GetPastMeetingResult, error) {
	if payload == nil || payload.UID == nil {
		return nil, handleError(domain.NewValidationError("validation failed"))
	}

	pastMeeting, revision, err := s.pastMeetingService.GetPastMeeting(ctx, *payload.UID)
	if err != nil {
		return nil, handleError(err)
	}

	result := &meetingsvc.GetPastMeetingResult{
		PastMeeting: service.ConvertDomainToPastMeetingResponse(pastMeeting),
		Etag:        utils.StringPtr(revision),
	}

	return result, nil
}

// DeletePastMeeting implements the Goa service interface for deleting past meetings
func (s *MeetingsAPI) DeletePastMeeting(ctx context.Context, payload *meetingsvc.DeletePastMeetingPayload) error {
	if payload == nil || payload.UID == nil {
		return handleError(domain.NewValidationError("validation failed"))
	}

	revision, err := service.EtagValidator(payload.IfMatch)
	if err != nil {
		return handleError(err)
	}

	err = s.pastMeetingService.DeletePastMeeting(ctx, *payload.UID, revision)
	if err != nil {
		return handleError(err)
	}

	return nil
}
