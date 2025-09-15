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

// GetPastMeetingParticipants gets all participants for a past meeting
func (s *MeetingsAPI) GetPastMeetingParticipants(ctx context.Context, payload *meetingsvc.GetPastMeetingParticipantsPayload) (*meetingsvc.GetPastMeetingParticipantsResult, error) {
	if payload == nil || payload.UID == nil {
		return nil, handleError(domain.NewValidationError("validation failed", nil))
	}

	participants, err := s.pastMeetingParticipantService.GetPastMeetingParticipants(ctx, *payload.UID)
	if err != nil {
		return nil, handleError(err)
	}

	// Convert domain models to API response format
	var responseParticipants []*meetingsvc.PastMeetingParticipant
	for _, participant := range participants {
		responseParticipants = append(responseParticipants, service.ConvertDomainToPastMeetingParticipantResponse(participant))
	}

	return &meetingsvc.GetPastMeetingParticipantsResult{
		Participants: responseParticipants,
		CacheControl: nil,
	}, nil
}

// CreatePastMeetingParticipant creates a new participant for a past meeting
func (s *MeetingsAPI) CreatePastMeetingParticipant(ctx context.Context, payload *meetingsvc.CreatePastMeetingParticipantPayload) (*meetingsvc.PastMeetingParticipant, error) {
	if payload == nil || payload.UID == nil {
		return nil, handleError(domain.NewValidationError("validation failed", nil))
	}

	createParticipantReq := service.ConvertCreatePastMeetingParticipantPayloadToDomain(payload)

	participant, err := s.pastMeetingParticipantService.CreatePastMeetingParticipant(ctx, createParticipantReq)
	if err != nil {
		return nil, handleError(err)
	}

	return service.ConvertDomainToPastMeetingParticipantResponse(participant), nil
}

// GetPastMeetingParticipant gets a specific participant for a past meeting by UID
func (s *MeetingsAPI) GetPastMeetingParticipant(ctx context.Context, payload *meetingsvc.GetPastMeetingParticipantPayload) (*meetingsvc.GetPastMeetingParticipantResult, error) {
	if payload == nil || payload.PastMeetingUID == nil || payload.UID == nil {
		return nil, handleError(domain.NewValidationError("validation failed", nil))
	}

	participant, etag, err := s.pastMeetingParticipantService.GetPastMeetingParticipant(ctx, *payload.PastMeetingUID, *payload.UID)
	if err != nil {
		return nil, handleError(err)
	}

	return &meetingsvc.GetPastMeetingParticipantResult{
		Participant: service.ConvertDomainToPastMeetingParticipantResponse(participant),
		Etag:        utils.StringPtr(etag),
	}, nil
}

// UpdatePastMeetingParticipant updates an existing participant for a past meeting
func (s *MeetingsAPI) UpdatePastMeetingParticipant(ctx context.Context, payload *meetingsvc.UpdatePastMeetingParticipantPayload) (*meetingsvc.PastMeetingParticipant, error) {
	if payload == nil || payload.PastMeetingUID == "" || payload.UID == nil {
		return nil, handleError(domain.NewValidationError("validation failed", nil))
	}

	etag, err := service.EtagValidator(payload.IfMatch)
	if err != nil {
		return nil, handleError(err)
	}

	updateParticipantReq := service.ConvertUpdatePastMeetingParticipantPayloadToDomain(payload)

	participant, err := s.pastMeetingParticipantService.UpdatePastMeetingParticipant(ctx, updateParticipantReq, etag)
	if err != nil {
		return nil, handleError(err)
	}

	return service.ConvertDomainToPastMeetingParticipantResponse(participant), nil
}

// DeletePastMeetingParticipant deletes a participant from a past meeting
func (s *MeetingsAPI) DeletePastMeetingParticipant(ctx context.Context, payload *meetingsvc.DeletePastMeetingParticipantPayload) error {
	if payload == nil || payload.PastMeetingUID == nil || payload.UID == nil {
		return handleError(domain.NewValidationError("validation failed", nil))
	}

	etag, err := service.EtagValidator(payload.IfMatch)
	if err != nil {
		return handleError(err)
	}

	err = s.pastMeetingParticipantService.DeletePastMeetingParticipant(ctx, *payload.PastMeetingUID, *payload.UID, etag)
	if err != nil {
		return handleError(err)
	}

	return nil
}
