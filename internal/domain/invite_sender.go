// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package domain

import (
	"context"
	"time"

	inviteapi "github.com/linuxfoundation/lfx-v2-invite-service/pkg/api"
)

// InviteResult holds the key fields returned by the invite service after an invite is sent.
type InviteResult struct {
	InviteUID      string
	RecipientEmail string
	ExpiresAt      time.Time
}

// InviteSender sends LFID invites via the invite service over NATS.
type InviteSender interface {
	// SendInvite sends an LFID invite for the given request.
	// Returns the invite metadata on success. On failure, the caller should log and
	// continue — invite sending is always best-effort and must never block indexing.
	SendInvite(ctx context.Context, req inviteapi.SendInviteRequest) (*InviteResult, error)
}

// InviteLookup re-reads a stored invite record from the invite service, which owns
// invite state and is where an acceptance is recorded.
//
// It exists so that acceptance notifications arriving on the shared NATS bus — where any
// workload can publish — are checked against that stored record before the meeting
// service performs a privileged identity-binding write on their behalf.
//
// The stored record is evidence of what was recorded, not of who performed it. The invite
// service writes the caller-supplied username from its own unauthenticated upstream
// subject into accepted_by, so a record can say an acceptance happened without that being
// true. Checking it narrows what a publisher can cause — an acceptance has to match a real
// pending invite, and binds that invite's own recipient address — but it does not
// establish origin. Origin comes only from NATS subject authorization or a signed
// assertion; see docs/event-processing.md and the InviteAcceptedSubscriber doc comment.
// Implementations and callers should not present this as proof of who accepted.
type InviteLookup interface {
	// GetInvite returns the stored invite record for uid.
	// Returns ErrInviteNotFound when the invite service has no record for uid, and a
	// non-nil error for transient NATS or parsing failures. Callers must fail closed
	// on both: an unverifiable acceptance must not be acted on.
	GetInvite(ctx context.Context, uid string) (*inviteapi.Invite, error)
}
