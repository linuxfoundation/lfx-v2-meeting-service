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
		return nil, fmt.Errorf("invite service returned error: %s", resp.Error)
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

// Ensure NATSInviteLookup implements domain.InviteLookup.
var _ domain.InviteLookup = (*NATSInviteLookup)(nil)
