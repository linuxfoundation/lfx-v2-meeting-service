// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package main

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-meeting-service/cmd/meeting-api/service"
	meetingsvc "github.com/linuxfoundation/lfx-v2-meeting-service/gen/meeting_service"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
)

// GetMeetingRegistrants gets all meeting registrants for a meeting.
func (s *MeetingsAPI) GetMeetingRegistrants(ctx context.Context, payload *meetingsvc.GetMeetingRegistrantsPayload) (*meetingsvc.GetMeetingRegistrantsResult, error) {
	if payload == nil || payload.UID == nil {
		return nil, domain.ErrValidationFailed
	}

	registrants, err := s.registrantService.GetMeetingRegistrants(ctx, *payload.UID)
	if err != nil {
		return nil, handleError(err)
	}

	var registrantsResp []*meetingsvc.Registrant
	for _, registrant := range registrants {
		registrantsResp = append(registrantsResp, service.ConvertDomainToRegistrantResponse(registrant))
	}

	return &meetingsvc.GetMeetingRegistrantsResult{
		Registrants:  registrantsResp,
		CacheControl: nil,
	}, nil
}

// CreateMeetingRegistrant creates a new meeting registrant.
func (s *MeetingsAPI) CreateMeetingRegistrant(ctx context.Context, payload *meetingsvc.CreateMeetingRegistrantPayload) (*meetingsvc.Registrant, error) {
	if payload == nil || payload.MeetingUID == "" {
		return nil, domain.ErrValidationFailed
	}

	createRegistrantReq := service.ConvertCreateRegistrantPayloadToDomain(payload)

	registrant, err := s.registrantService.CreateMeetingRegistrant(ctx, createRegistrantReq)
	if err != nil {
		return nil, handleError(err)
	}

	return service.ConvertDomainToRegistrantResponse(registrant), nil
}

// GetMeetingRegistrant gets a single meeting registrant.
func (s *MeetingsAPI) GetMeetingRegistrant(ctx context.Context, payload *meetingsvc.GetMeetingRegistrantPayload) (*meetingsvc.GetMeetingRegistrantResult, error) {
	if payload == nil || payload.MeetingUID == nil || payload.UID == nil {
		return nil, domain.ErrValidationFailed
	}

	registrant, etag, err := s.registrantService.GetMeetingRegistrant(ctx, *payload.MeetingUID, *payload.UID)
	if err != nil {
		return nil, handleError(err)
	}

	return &meetingsvc.GetMeetingRegistrantResult{
		Registrant: service.ConvertDomainToRegistrantResponse(registrant),
		Etag:       &etag,
	}, nil
}

// UpdateMeetingRegistrant updates a meeting registrant.
func (s *MeetingsAPI) UpdateMeetingRegistrant(ctx context.Context, payload *meetingsvc.UpdateMeetingRegistrantPayload) (*meetingsvc.Registrant, error) {
	if payload == nil || payload.MeetingUID == "" || payload.UID == nil {
		return nil, domain.ErrValidationFailed
	}

	etag, err := service.EtagValidator(payload.IfMatch)
	if err != nil {
		return nil, handleError(err)
	}

	updateRegistrantReq := service.ConvertUpdateRegistrantPayloadToDomain(payload)

	registrant, err := s.registrantService.UpdateMeetingRegistrant(ctx, updateRegistrantReq, etag)
	if err != nil {
		return nil, handleError(err)
	}

	return service.ConvertDomainToRegistrantResponse(registrant), nil
}

// DeleteMeetingRegistrant deletes a meeting registrant.
func (s *MeetingsAPI) DeleteMeetingRegistrant(ctx context.Context, payload *meetingsvc.DeleteMeetingRegistrantPayload) error {
	if payload == nil || payload.MeetingUID == nil || payload.UID == nil {
		return domain.ErrValidationFailed
	}

	etag, err := service.EtagValidator(payload.IfMatch)
	if err != nil {
		return handleError(err)
	}

	err = s.registrantService.DeleteMeetingRegistrant(ctx, *payload.MeetingUID, *payload.UID, etag)
	if err != nil {
		return handleError(err)
	}

	return nil
}
