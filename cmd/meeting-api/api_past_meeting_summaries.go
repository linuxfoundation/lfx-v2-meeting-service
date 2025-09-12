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

// GetPastMeetingSummaries gets all summaries for a past meeting
func (s *MeetingsAPI) GetPastMeetingSummaries(ctx context.Context, payload *meetingsvc.GetPastMeetingSummariesPayload) (*meetingsvc.GetPastMeetingSummariesResult, error) {
	if payload == nil || payload.UID == nil {
		return nil, handleError(domain.ErrValidationFailed)
	}

	summaries, err := s.pastMeetingSummaryService.ListSummariesByPastMeeting(ctx, *payload.UID)
	if err != nil {
		return nil, handleError(err)
	}

	var responseSummaries []*meetingsvc.PastMeetingSummary
	for _, summary := range summaries {
		responseSummaries = append(responseSummaries, service.ConvertDomainToPastMeetingSummaryResponse(summary))
	}

	return &meetingsvc.GetPastMeetingSummariesResult{
		Summaries:    responseSummaries,
		CacheControl: nil,
	}, nil
}

// GetPastMeetingSummary gets a specific summary for a past meeting by UID
func (s *MeetingsAPI) GetPastMeetingSummary(ctx context.Context, payload *meetingsvc.GetPastMeetingSummaryPayload) (*meetingsvc.GetPastMeetingSummaryResult, error) {
	if payload == nil || payload.PastMeetingUID == "" || payload.SummaryUID == "" {
		return nil, handleError(domain.ErrValidationFailed)
	}

	summary, etag, err := s.pastMeetingSummaryService.GetSummary(ctx, payload.SummaryUID)
	if err != nil {
		return nil, handleError(err)
	}

	summaryResponse := service.ConvertDomainToPastMeetingSummaryResponse(summary)

	return &meetingsvc.GetPastMeetingSummaryResult{
		Summary: summaryResponse,
		Etag:    utils.StringPtr(etag),
	}, nil
}

// UpdatePastMeetingSummary updates an existing past meeting summary
func (s *MeetingsAPI) UpdatePastMeetingSummary(ctx context.Context, payload *meetingsvc.UpdatePastMeetingSummaryPayload) (*meetingsvc.PastMeetingSummary, error) {
	if payload == nil || payload.PastMeetingUID == "" || payload.SummaryUID == "" {
		return nil, handleError(domain.ErrValidationFailed)
	}

	etag, err := service.EtagValidator(payload.IfMatch)
	if err != nil {
		return nil, handleError(err)
	}

	updateSummaryReq := service.ConvertUpdatePastMeetingSummaryPayloadToDomain(payload)

	summary, err := s.pastMeetingSummaryService.UpdateSummary(ctx, updateSummaryReq, etag)
	if err != nil {
		return nil, handleError(err)
	}

	return service.ConvertDomainToPastMeetingSummaryResponse(summary), nil
}
