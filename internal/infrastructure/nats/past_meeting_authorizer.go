// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package nats

import (
	"context"
	"fmt"
	"strings"
	"time"

	fgaconstants "github.com/linuxfoundation/lfx-v2-fga-sync/pkg/constants"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
)

// pastMeetingAuthorizerTimeout is the maximum time to wait for fga-sync to reply.
const pastMeetingAuthorizerTimeout = 10 * time.Second

// NATSPastMeetingAuthorizer implements domain.PastMeetingAuthorizer by calling
// the fga-sync access-check RPC over NATS (subject: lfx.access_check.request).
//
// Wire format (same as NATSCommitteeAuthorizer):
//
//	request:  "v1_past_meeting:{id}#organizer@user:{principal}"
//	response: "v1_past_meeting:{id}#organizer@user:{principal}\t{true|false}"
type NATSPastMeetingAuthorizer struct {
	nc Requester
}

// Verify interface compliance at compile time.
var _ domain.PastMeetingAuthorizer = (*NATSPastMeetingAuthorizer)(nil)

// NewNATSPastMeetingAuthorizer creates a NATSPastMeetingAuthorizer backed by nc.
func NewNATSPastMeetingAuthorizer(nc Requester) *NATSPastMeetingAuthorizer {
	return &NATSPastMeetingAuthorizer{nc: nc}
}

// IsOrganizer asks fga-sync whether principal has the "organizer" relation on
// v1_past_meeting:{pastMeetingID}. It returns (false, nil) when access is
// explicitly denied, (true, nil) when granted, and (false, err) when the RPC fails.
func (a *NATSPastMeetingAuthorizer) IsOrganizer(ctx context.Context, principal, pastMeetingID string) (bool, error) {
	// The past meeting ID comes from the request path. Refuse IDs carrying tuple
	// delimiters or whitespace so a crafted ID cannot reshape the check request.
	// The principal comes from the Heimdall-signed JWT; refuse whitespace there
	// too so it cannot add request lines. '@' and '#' stay allowed in the
	// principal: fga-sync splits on the first '@', so they only name the user.
	if pastMeetingID == "" || principal == "" ||
		strings.ContainsAny(pastMeetingID, "#@ \t\r\n") || strings.ContainsAny(principal, " \t\r\n") {
		return false, nil
	}

	tuple := fmt.Sprintf("v1_past_meeting:%s#organizer@user:%s", pastMeetingID, principal)

	// context.WithTimeout takes the sooner of the parent deadline and the
	// specified duration, so this never extends a caller's shorter deadline.
	ctx, cancel := context.WithTimeout(ctx, pastMeetingAuthorizerTimeout)
	defer cancel()

	msg, err := a.nc.RequestWithContext(ctx, fgaconstants.AccessCheckSubject, []byte(tuple))
	if err != nil {
		return false, fmt.Errorf("fga access check NATS request failed: %w", err)
	}

	return parseAccessCheckResponse(msg.Data, tuple)
}
