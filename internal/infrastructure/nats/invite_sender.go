// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package nats

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"log/slog"

	inviteapi "github.com/linuxfoundation/lfx-v2-invite-service/pkg/api"
	natsgo "github.com/nats-io/nats.go"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
)

type inviteSender struct {
	nc *natsgo.Conn
}

// SendInvite sends a request/reply to the invite service for a user who does not
// yet have an LFID and returns the invite metadata from the response.
func (s *inviteSender) SendInvite(ctx context.Context, req inviteapi.SendInviteRequest) (domain.InviteResult, error) {
	if s.nc == nil {
		return domain.InviteResult{}, domain.NewUnavailableError("invite sender is not configured")
	}

	if err := ctx.Err(); err != nil {
		return domain.InviteResult{}, domain.NewUnavailableError("context cancelled before sending invite", err)
	}

	data, err := json.Marshal(req)
	if err != nil {
		slog.ErrorContext(ctx, "failed to marshal invite request", "error", err)
		return domain.InviteResult{}, domain.NewInternalError("failed to marshal invite request", err)
	}

	reply, err := s.nc.RequestMsgWithContext(ctx, &natsgo.Msg{
		Subject: inviteapi.SendInviteSubject,
		Data:    data,
	})
	if err != nil {
		slog.ErrorContext(ctx, "invite service request failed", "error", err)
		return domain.InviteResult{}, domain.NewUnavailableError("invite service unavailable", err)
	}

	var resp inviteapi.SendInviteResponse
	if len(reply.Data) > 0 {
		if jsonErr := json.Unmarshal(reply.Data, &resp); jsonErr != nil {
			slog.ErrorContext(ctx, "error unmarshalling invite response", "error", jsonErr)
			return domain.InviteResult{}, domain.NewInternalError("failed to parse invite service response", jsonErr)
		}
		if resp.Error != "" {
			return domain.InviteResult{}, domain.NewInternalError("invite service returned an error", stderrors.New(resp.Error))
		}
	}

	var result domain.InviteResult
	if resp.Invite != nil {
		result.InviteUID = resp.Invite.UID
		result.RecipientEmail = resp.Invite.Email
		result.ExpiresAt = resp.Invite.ExpiresAt
	}
	slog.DebugContext(ctx, "invite service replied", "invite_uid", result.InviteUID, "expires_at", result.ExpiresAt)
	return result, nil
}

// NewInviteSender creates a NATS-backed InviteSender.
func NewInviteSender(nc *natsgo.Conn) domain.InviteSender {
	return &inviteSender{nc: nc}
}
