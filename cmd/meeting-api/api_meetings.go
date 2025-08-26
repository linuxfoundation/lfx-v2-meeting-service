// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package main

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-meeting-service/cmd/meeting-api/service"
	meetingsvc "github.com/linuxfoundation/lfx-v2-meeting-service/gen/meeting_service"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
)

// GetMeetings fetches all meetings
func (s *MeetingsAPI) GetMeetings(ctx context.Context, payload *meetingsvc.GetMeetingsPayload) (*meetingsvc.GetMeetingsResult, error) {
	meetings, err := s.meetingService.GetMeetings(ctx)
	if err != nil {
		return nil, handleError(err)
	}

	// Convert domain models to service models
	var respMeetings []*meetingsvc.MeetingFull
	for _, meeting := range meetings {
		respMeetings = append(respMeetings, service.ConvertDomainToFullResponse(meeting))
	}

	return &meetingsvc.GetMeetingsResult{
		Meetings:     respMeetings,
		CacheControl: nil,
	}, nil
}

// CreateMeeting creates a new meeting.
func (s *MeetingsAPI) CreateMeeting(ctx context.Context, payload *meetingsvc.CreateMeetingPayload) (*meetingsvc.MeetingFull, error) {
	createMeetingReq, err := service.ConvertCreateMeetingPayloadToDomain(payload)
	if err != nil {
		return nil, handleError(err)
	}

	meeting, err := s.meetingService.CreateMeeting(ctx, createMeetingReq)
	if err != nil {
		return nil, handleError(err)
	}
	return service.ConvertDomainToFullResponse(meeting), nil
}

// GetMeetingBase gets a single meeting's base information.
func (s *MeetingsAPI) GetMeetingBase(ctx context.Context, payload *meetingsvc.GetMeetingBasePayload) (*meetingsvc.GetMeetingBaseResult, error) {
	if payload == nil || payload.UID == nil {
		return nil, domain.ErrValidationFailed
	}

	meeting, revision, err := s.meetingService.GetMeetingBase(ctx, *payload.UID)
	if err != nil {
		return nil, handleError(err)
	}

	return &meetingsvc.GetMeetingBaseResult{
		Meeting: service.ConvertDomainToBaseResponse(meeting),
		Etag:    &revision,
	}, nil
}

// GetMeetingSettings gets settings for a specific meeting
func (s *MeetingsAPI) GetMeetingSettings(ctx context.Context, payload *meetingsvc.GetMeetingSettingsPayload) (*meetingsvc.GetMeetingSettingsResult, error) {
	if payload == nil || payload.UID == nil {
		return nil, domain.ErrValidationFailed
	}

	settings, etag, err := s.meetingService.GetMeetingSettings(ctx, *payload.UID)
	if err != nil {
		return nil, handleError(err)
	}

	return &meetingsvc.GetMeetingSettingsResult{
		MeetingSettings: service.ConvertDomainToSettingsResponse(settings),
		Etag:            &etag,
	}, nil
}

// UpdateMeetingBase updates a meeting's base information.
func (s *MeetingsAPI) UpdateMeetingBase(ctx context.Context, payload *meetingsvc.UpdateMeetingBasePayload) (*meetingsvc.MeetingBase, error) {
	if payload == nil || payload.UID == "" {
		return nil, domain.ErrValidationFailed
	}

	etag, err := service.EtagValidator(payload.IfMatch)
	if err != nil {
		return nil, handleError(err)
	}

	updatedMeetingReq := service.ConvertMeetingUpdatePayloadToDomain(payload)

	updatedMeeting, err := s.meetingService.UpdateMeetingBase(ctx, updatedMeetingReq, etag)
	if err != nil {
		return nil, handleError(err)
	}
	return service.ConvertDomainToBaseResponse(updatedMeeting), nil
}

// UpdateMeetingSettings updates a meeting's settings.
func (s *MeetingsAPI) UpdateMeetingSettings(ctx context.Context, payload *meetingsvc.UpdateMeetingSettingsPayload) (*meetingsvc.MeetingSettings, error) {
	if payload == nil || payload.UID == nil {
		return nil, domain.ErrValidationFailed
	}

	etag, err := service.EtagValidator(payload.IfMatch)
	if err != nil {
		return nil, handleError(err)
	}

	updateSettingsReq := service.ConvertUpdateSettingsPayloadToDomain(payload)
	updatedSettings, err := s.meetingService.UpdateMeetingSettings(ctx, updateSettingsReq, etag)
	if err != nil {
		return nil, handleError(err)
	}
	return service.ConvertDomainToSettingsResponse(updatedSettings), nil
}

// DeleteMeeting deletes a meeting.
func (s *MeetingsAPI) DeleteMeeting(ctx context.Context, payload *meetingsvc.DeleteMeetingPayload) error {
	if payload == nil || payload.UID == nil {
		return domain.ErrValidationFailed
	}

	etag, err := service.EtagValidator(payload.IfMatch)
	if err != nil {
		return handleError(err)
	}

	err = s.meetingService.DeleteMeeting(ctx, *payload.UID, etag)
	if err != nil {
		return handleError(err)
	}
	return nil
}
