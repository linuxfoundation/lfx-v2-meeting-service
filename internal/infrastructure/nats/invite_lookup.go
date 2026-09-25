// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	inviteapi "github.com/linuxfoundation/lfx-v2-invite-service/pkg/api"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
)

const inviteLookupTimeout = 10 * time.Second

// NATSInviteLookup implements domain.InviteLookup using NATS request/reply against
// the invite service, which owns invite state.
type NATSInviteLookup struct {
	nc     Requester
	logger *slog.Logger
}

// NewInviteLookup creates a new NATS-based invite lookup.
func NewInviteLookup(nc Requester, logger *slog.Logger) *NATSInviteLookup {
	logger.Info("invite lookup initialized", "subject", inviteapi.GetInviteSubject)
	return &NATSInviteLookup{nc: nc, logger: logger}
}

// GetInvite fetches the stored invite record for uid from the invite service.
// Returns domain.ErrInviteNotFound when no record exists for uid.
func (l *NATSInviteLookup) GetInvite(ctx context.Context, uid string) (*inviteapi.Invite, error) {
	uid = strings.TrimSpace(uid)
	if uid == "" {
		return nil, domain.ErrInviteNotFound
	}

	payload, err := json.Marshal(inviteapi.GetInviteRequest{UID: uid})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal get_invite request: %w", err)
	}

	reqCtx, cancel := context.WithTimeout(ctx, inviteLookupTimeout)
	defer cancel()

	msg, err := l.nc.RequestWithContext(reqCtx, inviteapi.GetInviteSubject, payload)
	if err != nil {
		return nil, fmt.Errorf("get_invite request failed: %w", err)
	}

	var resp inviteapi.GetInviteResponse
	if err := json.Unmarshal(msg.Data, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse get_invite response: %w", err)
	}
	if resp.Error != "" {
		// A reply that reports the invite does not exist is the expected answer for a
		// forged or stale event, not an operational fault: map it to ErrInviteNotFound
		// so the caller discards quietly instead of raising an error for every such
		// event. The contract carries no typed error code (GetInviteResponse.Error is a
		// free-form string), so this matches on the known spellings and treats anything
		// else as a genuine failure — which still fails closed, just loudly.
		if isNotFoundError(resp.Error) {
			return nil, domain.ErrInviteNotFound
		}
		return nil, fmt.Errorf("invite service returned error: %q", truncateForLog(resp.Error))
	}
	// Invite is an embedded pointer: it stays nil when the reply carried no record.
	if resp.Invite == nil {
		return nil, domain.ErrInviteNotFound
	}
	// Guard against a reply that answers a different invite than the one asked about.
	if strings.TrimSpace(resp.UID) != uid {
		return nil, fmt.Errorf("get_invite returned a record for a different invite uid")
	}

	return resp.Invite, nil
}

// isNotFoundError reports whether an invite service error string means "no such invite".
func isNotFoundError(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	return s == "not_found" || s == "not found" || s == "invite not found"
}

// truncateForLog bounds a string taken from a reply before it reaches a log line or an
// error message. The reply is attacker-influenceable on an unauthenticated bus, so its
// length must not be.
func truncateForLog(s string) string {
	const maxLen = 120
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "…"
}

// Ensure NATSInviteLookup implements domain.InviteLookup.
var _ domain.InviteLookup = (*NATSInviteLookup)(nil)
