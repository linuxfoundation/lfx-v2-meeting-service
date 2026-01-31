// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package main

import (
	"context"

	meetingsvc "github.com/linuxfoundation/lfx-v2-meeting-service/gen/meeting_service"
	"github.com/linuxfoundation/lfx-v2-meeting-service/cmd/meeting-api/service"
)

// CreateItxMeeting creates a meeting via ITX proxy
func (s *MeetingsAPI) CreateItxMeeting(ctx context.Context, p *meetingsvc.CreateItxMeetingPayload) (*meetingsvc.ITXZoomMeetingResponse, error) {
	// Convert Goa payload to domain request
	req := service.ConvertCreateITXMeetingPayloadToDomain(p)

	// Call ITX service
	resp, err := s.itxMeetingService.CreateMeeting(ctx, req)
	if err != nil {
		return nil, handleError(err)
	}

	// Convert ITX response to Goa response
	goaResp := service.ConvertITXMeetingResponseToGoa(resp)
	return goaResp, nil
}

// GetItxMeeting retrieves a meeting via ITX proxy
func (s *MeetingsAPI) GetItxMeeting(ctx context.Context, p *meetingsvc.GetItxMeetingPayload) (*meetingsvc.ITXZoomMeetingResponse, error) {
	// Call ITX service
	resp, err := s.itxMeetingService.GetMeeting(ctx, p.MeetingID)
	if err != nil {
		return nil, handleError(err)
	}

	// Convert ITX response to Goa response
	goaResp := service.ConvertITXMeetingResponseToGoa(resp)
	return goaResp, nil
}

// UpdateItxMeeting updates a meeting via ITX proxy
func (s *MeetingsAPI) UpdateItxMeeting(ctx context.Context, p *meetingsvc.UpdateItxMeetingPayload) error {
	// Convert Goa payload to domain request
	req := service.ConvertCreateITXMeetingPayloadToDomain(&meetingsvc.CreateItxMeetingPayload{
		BearerToken:          p.BearerToken,
		Version:              p.Version,
		XSync:                p.XSync,
		ProjectUID:           p.ProjectUID,
		Title:                p.Title,
		StartTime:            p.StartTime,
		Duration:             p.Duration,
		Timezone:             p.Timezone,
		Visibility:           p.Visibility,
		Description:          p.Description,
		Restricted:           p.Restricted,
		Committees:           p.Committees,
		MeetingType:          p.MeetingType,
		EarlyJoinTimeMinutes: p.EarlyJoinTimeMinutes,
		RecordingEnabled:     p.RecordingEnabled,
		TranscriptEnabled:    p.TranscriptEnabled,
		YoutubeUploadEnabled: p.YoutubeUploadEnabled,
		ArtifactVisibility:   p.ArtifactVisibility,
		Recurrence:           p.Recurrence,
	})

	// Call ITX service
	err := s.itxMeetingService.UpdateMeeting(ctx, p.MeetingID, req)
	if err != nil {
		return handleError(err)
	}

	return nil
}

// DeleteItxMeeting deletes a meeting via ITX proxy
func (s *MeetingsAPI) DeleteItxMeeting(ctx context.Context, p *meetingsvc.DeleteItxMeetingPayload) error {
	// Call ITX service
	err := s.itxMeetingService.DeleteMeeting(ctx, p.MeetingID)
	if err != nil {
		return handleError(err)
	}

	return nil
}

// GetItxMeetingCount retrieves the meeting count for a project via ITX proxy
func (s *MeetingsAPI) GetItxMeetingCount(ctx context.Context, p *meetingsvc.GetItxMeetingCountPayload) (*meetingsvc.ITXMeetingCountResponse, error) {
	// Call ITX service
	resp, err := s.itxMeetingService.GetMeetingCount(ctx, p.ProjectUID)
	if err != nil {
		return nil, handleError(err)
	}

	// Convert ITX response to Goa response
	goaResp := &meetingsvc.ITXMeetingCountResponse{
		MeetingCount: resp.MeetingCount,
	}
	return goaResp, nil
}

// CreateItxRegistrant creates a meeting registrant via ITX proxy
func (s *MeetingsAPI) CreateItxRegistrant(ctx context.Context, p *meetingsvc.CreateItxRegistrantPayload) (*meetingsvc.ITXZoomMeetingRegistrant, error) {
	// Convert Goa payload to ITX registrant
	req := service.ConvertCreateITXRegistrantPayloadToITX(p)

	// Call ITX service
	resp, err := s.itxMeetingService.CreateRegistrant(ctx, p.MeetingID, req)
	if err != nil {
		return nil, handleError(err)
	}

	// Convert ITX response to Goa response
	goaResp := service.ConvertITXRegistrantToGoa(resp)
	return goaResp, nil
}

// GetItxRegistrant retrieves a meeting registrant via ITX proxy
func (s *MeetingsAPI) GetItxRegistrant(ctx context.Context, p *meetingsvc.GetItxRegistrantPayload) (*meetingsvc.ITXZoomMeetingRegistrant, error) {
	// Call ITX service
	resp, err := s.itxMeetingService.GetRegistrant(ctx, p.MeetingID, p.RegistrantID)
	if err != nil {
		return nil, handleError(err)
	}

	// Convert ITX response to Goa response
	goaResp := service.ConvertITXRegistrantToGoa(resp)
	return goaResp, nil
}

// UpdateItxRegistrant updates a meeting registrant via ITX proxy
func (s *MeetingsAPI) UpdateItxRegistrant(ctx context.Context, p *meetingsvc.UpdateItxRegistrantPayload) error {
	// Convert Goa payload to ITX registrant
	req := service.ConvertUpdateITXRegistrantPayloadToITX(p)

	// Call ITX service
	err := s.itxMeetingService.UpdateRegistrant(ctx, p.MeetingID, p.RegistrantID, req)
	if err != nil {
		return handleError(err)
	}

	return nil
}

// DeleteItxRegistrant deletes a meeting registrant via ITX proxy
func (s *MeetingsAPI) DeleteItxRegistrant(ctx context.Context, p *meetingsvc.DeleteItxRegistrantPayload) error {
	// Call ITX service
	err := s.itxMeetingService.DeleteRegistrant(ctx, p.MeetingID, p.RegistrantID)
	if err != nil {
		return handleError(err)
	}

	return nil
}
