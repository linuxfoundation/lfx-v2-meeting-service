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
	auditStamper
	summaryClient domain.ITXPastMeetingSummaryClient
}

// NewPastMeetingSummaryService creates a new ITX past meeting summary service.
// userMetadata may be nil (e.g. when NATS is disabled), in which case modified_by is
// limited to the JWT-derived username/email rather than blocking the request.
func NewPastMeetingSummaryService(summaryClient domain.ITXPastMeetingSummaryClient, userMetadata domain.UserMetadataReader) *PastMeetingSummaryService {
	return &PastMeetingSummaryService{
		auditStamper:  auditStamper{userMetadata: userMetadata},
		summaryClient: summaryClient,
	}
}

// GetPastMeetingSummary retrieves a past meeting summary via ITX proxy
func (s *PastMeetingSummaryService) GetPastMeetingSummary(ctx context.Context, pastMeetingID, summaryID string) (*itx.PastMeetingSummaryResponse, error) {
	return s.summaryClient.GetPastMeetingSummary(ctx, pastMeetingID, summaryID)
}

// UpdatePastMeetingSummary updates a past meeting summary via ITX proxy
func (s *PastMeetingSummaryService) UpdatePastMeetingSummary(ctx context.Context, pastMeetingID, summaryID string, req *itx.UpdatePastMeetingSummaryRequest) (*itx.PastMeetingSummaryResponse, error) {
	// Stamp modified_by from the authenticated principal. ITX persists whatever the
	// caller sends here; leaving it blank produces a null audit entry.
	req.ModifiedBy = s.buildRequestingUser(ctx)
	return s.summaryClient.UpdatePastMeetingSummary(ctx, pastMeetingID, summaryID, req)
}
