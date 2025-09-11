// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package main

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-meeting-service/cmd/meeting-api/service"
	meetingsvc "github.com/linuxfoundation/lfx-v2-meeting-service/gen/meeting_service"
)

// GetPastMeetingSummaries gets all summaries for a past meeting
func (s *MeetingsAPI) GetPastMeetingSummaries(ctx context.Context, payload *meetingsvc.GetPastMeetingSummariesPayload) (*meetingsvc.GetPastMeetingSummariesResult, error) {
	// Get all summaries for this past meeting using the service layer
	// The service layer will handle validation and checking if the past meeting exists
	summaries, err := s.pastMeetingSummaryService.ListSummariesByPastMeeting(ctx, *payload.UID)
	if err != nil {
		return nil, handleError(err)
	}

	// Convert domain models to API response format
	var responseSummaries []*meetingsvc.PastMeetingSummary
	for _, summary := range summaries {
		responseSummaries = append(responseSummaries, service.ConvertDomainToPastMeetingSummaryResponse(summary))
	}

	// If no summaries exist, return empty array (not an error)
	if responseSummaries == nil {
		responseSummaries = []*meetingsvc.PastMeetingSummary{}
	}

	return &meetingsvc.GetPastMeetingSummariesResult{
		Summaries:    responseSummaries,
		CacheControl: nil, // Can be set if caching is desired
	}, nil
}
