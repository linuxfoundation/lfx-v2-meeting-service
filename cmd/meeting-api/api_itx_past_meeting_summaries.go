// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package main

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-meeting-service/cmd/meeting-api/service"
	meetingsvc "github.com/linuxfoundation/lfx-v2-meeting-service/gen/meeting_service"
)

// GetItxPastMeetingSummary retrieves a past meeting summary via ITX proxy
func (s *MeetingsAPI) GetItxPastMeetingSummary(ctx context.Context, p *meetingsvc.GetItxPastMeetingSummaryPayload) (*meetingsvc.ITXPastMeetingSummary, error) {
	// Call ITX service
	resp, err := s.itxPastMeetingSummaryService.GetPastMeetingSummary(ctx, p.PastMeetingID, p.SummaryID)
	if err != nil {
		return nil, handleError(err)
	}

	// Convert ITX response to Goa response
	goaResp := service.ConvertPastMeetingSummaryToGoa(resp)
	return goaResp, nil
}

// UpdateItxPastMeetingSummary updates a past meeting summary via ITX proxy
func (s *MeetingsAPI) UpdateItxPastMeetingSummary(ctx context.Context, p *meetingsvc.UpdateItxPastMeetingSummaryPayload) (*meetingsvc.ITXPastMeetingSummary, error) {
	// Convert Goa payload to ITX request
	req := service.ConvertUpdatePastMeetingSummaryPayload(p)

	// Call ITX service
	resp, err := s.itxPastMeetingSummaryService.UpdatePastMeetingSummary(ctx, p.PastMeetingID, p.SummaryID, req)
	if err != nil {
		return nil, handleError(err)
	}

	// Convert ITX response to Goa response
	goaResp := service.ConvertPastMeetingSummaryToGoa(resp)
	return goaResp, nil
}
