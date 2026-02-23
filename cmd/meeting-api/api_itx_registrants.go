// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package main

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-meeting-service/cmd/meeting-api/service"
	meetingsvc "github.com/linuxfoundation/lfx-v2-meeting-service/gen/meeting_service"
)

// CreateItxRegistrant creates a meeting registrant via ITX proxy
func (s *MeetingsAPI) CreateItxRegistrant(ctx context.Context, p *meetingsvc.CreateItxRegistrantPayload) (*meetingsvc.ITXZoomMeetingRegistrant, error) {
	// Convert Goa payload to ITX registrant
	req := service.ConvertCreateITXRegistrantPayloadToITX(p)

	// Call ITX service
	resp, err := s.itxRegistrantService.CreateRegistrant(ctx, p.MeetingID, req)
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
	resp, err := s.itxRegistrantService.GetRegistrant(ctx, p.MeetingID, p.RegistrantID)
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
	err := s.itxRegistrantService.UpdateRegistrant(ctx, p.MeetingID, p.RegistrantID, req)
	if err != nil {
		return handleError(err)
	}

	return nil
}

// DeleteItxRegistrant deletes a meeting registrant via ITX proxy
func (s *MeetingsAPI) DeleteItxRegistrant(ctx context.Context, p *meetingsvc.DeleteItxRegistrantPayload) error {
	// Call ITX service
	err := s.itxRegistrantService.DeleteRegistrant(ctx, p.MeetingID, p.RegistrantID)
	if err != nil {
		return handleError(err)
	}

	return nil
}

// GetItxRegistrantIcs retrieves an ICS calendar file for a meeting registrant via ITX proxy
func (s *MeetingsAPI) GetItxRegistrantIcs(ctx context.Context, p *meetingsvc.GetItxRegistrantIcsPayload) ([]byte, error) {
	// Call ITX service
	resp, err := s.itxRegistrantService.GetRegistrantICS(ctx, p.MeetingID, p.RegistrantID)
	if err != nil {
		return nil, handleError(err)
	}

	// Return raw ICS content
	return resp.Content, nil
}

// ResendItxRegistrantInvitation resends a meeting invitation to a registrant via ITX proxy
func (s *MeetingsAPI) ResendItxRegistrantInvitation(ctx context.Context, p *meetingsvc.ResendItxRegistrantInvitationPayload) error {
	// Call ITX service
	err := s.itxRegistrantService.ResendRegistrantInvitation(ctx, p.MeetingID, p.RegistrantID)
	if err != nil {
		return handleError(err)
	}

	return nil
}
