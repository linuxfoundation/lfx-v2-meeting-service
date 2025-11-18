// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"log/slog"

	"github.com/linuxfoundation/lfx-v2-meeting-service/cmd/meeting-api/service"
	meetingsvc "github.com/linuxfoundation/lfx-v2-meeting-service/gen/meeting_service"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
	svc "github.com/linuxfoundation/lfx-v2-meeting-service/internal/service"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/constants"
)

// GetMeetings fetches all meetings
func (s *MeetingsAPI) GetMeetings(ctx context.Context, payload *meetingsvc.GetMeetingsPayload) (*meetingsvc.GetMeetingsResult, error) {
	if payload == nil {
		return nil, handleError(domain.NewValidationError("payload is empty"))
	}

	meetings, err := s.meetingService.ListMeetings(ctx, payload.IncludeCancelledOccurrences)
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
	sync := payload.XSync != nil && *payload.XSync

	// Parse username from JWT token
	username, err := s.authService.ParsePrincipal(ctx, *payload.BearerToken, slog.Default())
	if err != nil {
		slog.WarnContext(ctx, "failed to parse username from JWT token", logging.ErrKey, err)
		return nil, handleError(domain.NewValidationError("failed to parse username from authorization token"))
	}

	// Store username in context for use by service layer
	ctx = context.WithValue(ctx, constants.UsernameContextID, username)

	createMeetingReq, err := service.ConvertCreateMeetingPayloadToDomain(payload)
	if err != nil {
		return nil, handleError(err)
	}

	meeting, err := s.meetingService.CreateMeeting(ctx, createMeetingReq, sync)
	if err != nil {
		return nil, handleError(err)
	}
	return service.ConvertDomainToFullResponse(meeting), nil
}

// GetMeetingBase gets a single meeting's base information.
func (s *MeetingsAPI) GetMeetingBase(ctx context.Context, payload *meetingsvc.GetMeetingBasePayload) (*meetingsvc.GetMeetingBaseResult, error) {
	if payload == nil || payload.UID == nil {
		return nil, handleError(domain.NewValidationError("validation failed"))
	}

	options := svc.GetMeetingBaseOptions{
		IncludeCancelledOccurrences: payload.IncludeCancelledOccurrences,
	}

	meeting, revision, err := s.meetingService.GetMeetingBase(ctx, *payload.UID, options)
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
		return nil, handleError(domain.NewValidationError("validation failed"))
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

// GetMeetingJoinURL gets the join URL for a specific meeting
func (s *MeetingsAPI) GetMeetingJoinURL(ctx context.Context, payload *meetingsvc.GetMeetingJoinURLPayload) (*meetingsvc.GetMeetingJoinURLResult, error) {
	if payload == nil || payload.UID == nil {
		return nil, handleError(domain.NewValidationError("validation failed"))
	}

	joinURL, err := s.meetingService.GetMeetingJoinURL(ctx, *payload.UID)
	if err != nil {
		return nil, handleError(err)
	}

	return &meetingsvc.GetMeetingJoinURLResult{
		JoinURL: joinURL,
	}, nil
}

// UpdateMeetingBase updates a meeting's base information.
func (s *MeetingsAPI) UpdateMeetingBase(ctx context.Context, payload *meetingsvc.UpdateMeetingBasePayload) (*meetingsvc.MeetingBase, error) {
	if payload == nil || payload.UID == "" {
		return nil, handleError(domain.NewValidationError("validation failed"))
	}

	sync := payload.XSync != nil && *payload.XSync

	// Parse username from JWT token
	username, err := s.authService.ParsePrincipal(ctx, *payload.BearerToken, slog.Default())
	if err != nil {
		slog.WarnContext(ctx, "failed to parse username from JWT token", logging.ErrKey, err)
		return nil, handleError(domain.NewValidationError("failed to parse username from authorization token"))
	}

	// Store username in context for use by service layer
	ctx = context.WithValue(ctx, constants.UsernameContextID, username)

	etag, err := service.EtagValidator(payload.IfMatch)
	if err != nil {
		return nil, handleError(err)
	}

	updatedMeetingReq, err := service.ConvertMeetingUpdatePayloadToDomain(payload)
	if err != nil {
		return nil, handleError(err)
	}

	updatedMeeting, err := s.meetingService.UpdateMeetingBase(ctx, updatedMeetingReq, etag, sync)
	if err != nil {
		return nil, handleError(err)
	}
	return service.ConvertDomainToBaseResponse(updatedMeeting), nil
}

// UpdateMeetingSettings updates a meeting's settings.
func (s *MeetingsAPI) UpdateMeetingSettings(ctx context.Context, payload *meetingsvc.UpdateMeetingSettingsPayload) (*meetingsvc.MeetingSettings, error) {
	if payload == nil || payload.UID == nil {
		return nil, handleError(domain.NewValidationError("validation failed"))
	}

	sync := payload.XSync != nil && *payload.XSync

	etag, err := service.EtagValidator(payload.IfMatch)
	if err != nil {
		return nil, handleError(err)
	}

	updateSettingsReq := service.ConvertUpdateSettingsPayloadToDomain(payload)
	updatedSettings, err := s.meetingService.UpdateMeetingSettings(ctx, updateSettingsReq, etag, sync)
	if err != nil {
		return nil, handleError(err)
	}
	return service.ConvertDomainToSettingsResponse(updatedSettings), nil
}

// DeleteMeeting deletes a meeting.
func (s *MeetingsAPI) DeleteMeeting(ctx context.Context, payload *meetingsvc.DeleteMeetingPayload) error {
	if payload == nil || payload.UID == nil {
		return handleError(domain.NewValidationError("validation failed"))
	}

	sync := payload.XSync != nil && *payload.XSync

	etag, err := service.EtagValidator(payload.IfMatch)
	if err != nil {
		return handleError(err)
	}

	err = s.meetingService.DeleteMeeting(ctx, *payload.UID, etag, sync)
	if err != nil {
		return handleError(err)
	}
	return nil
}

// DeleteMeetingOccurrence cancels a specific occurrence of a meeting.
func (s *MeetingsAPI) DeleteMeetingOccurrence(ctx context.Context, payload *meetingsvc.DeleteMeetingOccurrencePayload) error {
	if payload == nil {
		return handleError(domain.NewValidationError("payload is empty"))
	}

	sync := payload.XSync != nil && *payload.XSync

	etag, err := service.EtagValidator(payload.IfMatch)
	if err != nil {
		return handleError(err)
	}

	err = s.meetingService.CancelMeetingOccurrence(ctx, payload.UID, payload.OccurrenceID, etag, sync)
	if err != nil {
		return handleError(err)
	}
	return nil
}
