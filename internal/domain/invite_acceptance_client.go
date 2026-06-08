// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package domain

import "context"

// InviteAcceptanceClient calls the ITX Zoom Service to enrich all DynamoDB records
// (registrants, past-meeting invitees, past-meeting attendees) for a user who has
// just accepted their LFID invite.
type InviteAcceptanceClient interface {
	// AcceptInvite enriches all records associated with email with the accepted username.
	// This is best-effort: callers should log errors but never propagate them to block
	// upstream event processing.
	AcceptInvite(ctx context.Context, email, username string) error
}
