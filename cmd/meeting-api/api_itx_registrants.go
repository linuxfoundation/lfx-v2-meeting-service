// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package main

import (
	"context"

	meetingsvc "github.com/linuxfoundation/lfx-v2-meeting-service/gen/meeting_service"
	"github.com/linuxfoundation/lfx-v2-meeting-service/cmd/meeting-api/service"
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
