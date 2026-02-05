// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package main

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-meeting-service/cmd/meeting-api/service"
	meetingsvc "github.com/linuxfoundation/lfx-v2-meeting-service/gen/meeting_service"
)

// CreateItxPastMeeting creates a past meeting via ITX proxy
func (s *MeetingsAPI) CreateItxPastMeeting(ctx context.Context, p *meetingsvc.CreateItxPastMeetingPayload) (*meetingsvc.ITXPastZoomMeeting, error) {
	// Convert Goa payload to ITX request
	req := service.ConvertCreatePastMeetingPayload(p)

	// Call ITX service
	resp, err := s.itxPastMeetingService.CreatePastMeeting(ctx, req)
	if err != nil {
		return nil, handleError(err)
	}

	// Convert ITX response to Goa response
	goaResp := service.ConvertPastMeetingToGoa(resp)
	return goaResp, nil
}

// GetItxPastMeeting retrieves a past meeting via ITX proxy
func (s *MeetingsAPI) GetItxPastMeeting(ctx context.Context, p *meetingsvc.GetItxPastMeetingPayload) (*meetingsvc.ITXPastZoomMeeting, error) {
	// Call ITX service
	resp, err := s.itxPastMeetingService.GetPastMeeting(ctx, p.PastMeetingID)
	if err != nil {
		return nil, handleError(err)
	}

	// Convert ITX response to Goa response
	goaResp := service.ConvertPastMeetingToGoa(resp)
	return goaResp, nil
}

// UpdateItxPastMeeting updates a past meeting via ITX proxy
func (s *MeetingsAPI) UpdateItxPastMeeting(ctx context.Context, p *meetingsvc.UpdateItxPastMeetingPayload) (*meetingsvc.ITXPastZoomMeeting, error) {
	// Convert Goa payload to ITX request
	req := service.ConvertUpdatePastMeetingPayload(p)

	// Call ITX service
	resp, err := s.itxPastMeetingService.UpdatePastMeeting(ctx, p.PastMeetingID, req)
	if err != nil {
		return nil, handleError(err)
	}

	// Convert ITX response to Goa response
	goaResp := service.ConvertPastMeetingToGoa(resp)
	return goaResp, nil
}

// DeleteItxPastMeeting deletes a past meeting via ITX proxy
func (s *MeetingsAPI) DeleteItxPastMeeting(ctx context.Context, p *meetingsvc.DeleteItxPastMeetingPayload) error {
	// Call ITX service
	err := s.itxPastMeetingService.DeletePastMeeting(ctx, p.PastMeetingID)
	if err != nil {
		return handleError(err)
	}

	return nil
}
