// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package itx

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/models/itx"
)

// PastMeetingSummaryService handles ITX past meeting summary operations
type PastMeetingSummaryService struct {
	summaryClient domain.ITXPastMeetingSummaryClient
}

// NewPastMeetingSummaryService creates a new ITX past meeting summary service
func NewPastMeetingSummaryService(summaryClient domain.ITXPastMeetingSummaryClient) *PastMeetingSummaryService {
	return &PastMeetingSummaryService{
		summaryClient: summaryClient,
	}
}

// GetPastMeetingSummary retrieves a past meeting summary via ITX proxy
func (s *PastMeetingSummaryService) GetPastMeetingSummary(ctx context.Context, pastMeetingID, summaryID string) (*itx.PastMeetingSummaryResponse, error) {
	return s.summaryClient.GetPastMeetingSummary(ctx, pastMeetingID, summaryID)
}

// UpdatePastMeetingSummary updates a past meeting summary via ITX proxy
func (s *PastMeetingSummaryService) UpdatePastMeetingSummary(ctx context.Context, pastMeetingID, summaryID string, req *itx.UpdatePastMeetingSummaryRequest) (*itx.PastMeetingSummaryResponse, error) {
	return s.summaryClient.UpdatePastMeetingSummary(ctx, pastMeetingID, summaryID, req)
}
