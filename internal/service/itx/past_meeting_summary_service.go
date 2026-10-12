// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package itx

import (
	"context"
	"log/slog"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/constants"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/models/itx"
)

// PastMeetingSummaryService handles ITX past meeting summary operations
type PastMeetingSummaryService struct {
	auditStamper
	summaryClient    domain.ITXPastMeetingSummaryClient
	pastMeetingAuthz domain.PastMeetingAuthorizer // nil when NATS is unavailable
}

// NewPastMeetingSummaryService creates a new ITX past meeting summary service.
// userMetadata may be nil (e.g. when NATS is disabled), in which case modified_by is
// limited to the JWT-derived username/email rather than blocking the request.
// pastMeetingAuthz may also be nil when NATS is not configured; summaries awaiting
// approval are then readable by nobody through GetPastMeetingSummary, since the
// organizer-only gate cannot be evaluated.
func NewPastMeetingSummaryService(summaryClient domain.ITXPastMeetingSummaryClient, userMetadata domain.UserMetadataReader, pastMeetingAuthz domain.PastMeetingAuthorizer) *PastMeetingSummaryService {
	return &PastMeetingSummaryService{
		auditStamper:     auditStamper{userMetadata: userMetadata},
		summaryClient:    summaryClient,
		pastMeetingAuthz: pastMeetingAuthz,
	}
}

// GetPastMeetingSummary retrieves a past meeting summary via ITX proxy.
//
// Heimdall authorizes this route on ai_summary_viewer, which does not depend on
// the summary's approval state. A summary awaiting approval (requires_approval
// && !approved) is visible to the past meeting's organizers only, matching the
// search index (see docs/indexer-contract.md), so that gate is enforced here.
func (s *PastMeetingSummaryService) GetPastMeetingSummary(ctx context.Context, pastMeetingID, summaryID string) (*itx.PastMeetingSummaryResponse, error) {
	resp, err := s.summaryClient.GetPastMeetingSummary(ctx, pastMeetingID, summaryID)
	if err != nil {
		return nil, err
	}
	if resp != nil && resp.RequiresApproval && !resp.Approved {
		if err := s.authorizeAwaitingApprovalRead(ctx, pastMeetingID); err != nil {
			return nil, err
		}
	}
	return resp, nil
}

// authorizeAwaitingApprovalRead requires the requesting principal to hold
// organizer on v1_past_meeting:{pastMeetingID}. It fails closed: no principal or
// no authorizer is a 403, and an authorizer error is a 503.
func (s *PastMeetingSummaryService) authorizeAwaitingApprovalRead(ctx context.Context, pastMeetingID string) error {
	const denied = "summary is awaiting approval and is visible to meeting organizers only"

	principal, _ := ctx.Value(constants.PrincipalContextID).(string)
	if principal == "" || s.pastMeetingAuthz == nil {
		slog.WarnContext(ctx, "organizer check unavailable for summary awaiting approval; rejecting read",
			"past_meeting_id", pastMeetingID)
		return domain.NewForbiddenError(denied)
	}

	ok, err := s.pastMeetingAuthz.IsOrganizer(ctx, principal, pastMeetingID)
	if err != nil {
		slog.With(logging.ErrKey, err).WarnContext(ctx, "past meeting organizer check failed; rejecting summary read",
			"past_meeting_id", pastMeetingID)
		return domain.NewUnavailableError("cannot verify access to summary awaiting approval; please retry")
	}
	if !ok {
		return domain.NewForbiddenError(denied)
	}
	return nil
}

// UpdatePastMeetingSummary updates a past meeting summary via ITX proxy
func (s *PastMeetingSummaryService) UpdatePastMeetingSummary(ctx context.Context, pastMeetingID, summaryID string, req *itx.UpdatePastMeetingSummaryRequest) (*itx.PastMeetingSummaryResponse, error) {
	// Stamp modified_by from the authenticated principal. ITX persists whatever the
	// caller sends here; leaving it blank produces a null audit entry.
	req.ModifiedBy = s.buildRequestingUser(ctx)
	return s.summaryClient.UpdatePastMeetingSummary(ctx, pastMeetingID, summaryID, req)
}
