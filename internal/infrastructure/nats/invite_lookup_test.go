// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package nats

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"

	inviteapi "github.com/linuxfoundation/lfx-v2-invite-service/pkg/api"
	natsgo "github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
)

func TestNATSInviteLookup_GetInvite(t *testing.T) {
	const uid = "00000000-0000-0000-0000-000000000001"
	ctx := context.Background()

	replyWith := func(t *testing.T, payload any) *natsgo.Msg {
		t.Helper()
		data, err := json.Marshal(payload)
		require.NoError(t, err)
		return &natsgo.Msg{Data: data}
	}

	t.Run("returns the stored invite record", func(t *testing.T) {
		requester := &MockRequester{}
		requester.On("RequestWithContext", mock.Anything, inviteapi.GetInviteSubject, mock.Anything).
			Return(replyWith(t, inviteapi.GetInviteResponse{Invite: &inviteapi.Invite{
				UID:        uid,
				Status:     inviteapi.InviteStatusAccepted,
				Recipient:  inviteapi.Recipient{Email: "alice@example.com"},
				AcceptedBy: "alice",
			}}), nil)

		invite, err := NewInviteLookup(requester, slog.Default()).GetInvite(ctx, uid)

		require.NoError(t, err)
		require.NotNil(t, invite)
		assert.Equal(t, uid, invite.UID)
		assert.Equal(t, inviteapi.InviteStatusAccepted, invite.Status)
		assert.Equal(t, "alice", invite.AcceptedBy)

		// The request must carry the UID being verified.
		var sent inviteapi.GetInviteRequest
		require.NoError(t, json.Unmarshal(requester.Calls[0].Arguments.Get(2).([]byte), &sent))
		assert.Equal(t, uid, sent.UID)
	})

	t.Run("returns ErrInviteNotFound for an empty uid without calling NATS", func(t *testing.T) {
		requester := &MockRequester{}

		invite, err := NewInviteLookup(requester, slog.Default()).GetInvite(ctx, "   ")

		assert.ErrorIs(t, err, domain.ErrInviteNotFound)
		assert.Nil(t, invite)
		requester.AssertNotCalled(t, "RequestWithContext", mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("returns ErrInviteNotFound when the reply carries no record", func(t *testing.T) {
		requester := &MockRequester{}
		requester.On("RequestWithContext", mock.Anything, mock.Anything, mock.Anything).
			Return(&natsgo.Msg{Data: []byte(`{}`)}, nil)

		invite, err := NewInviteLookup(requester, slog.Default()).GetInvite(ctx, uid)

		assert.ErrorIs(t, err, domain.ErrInviteNotFound)
		assert.Nil(t, invite)
	})

	t.Run("maps a not-found error reply to ErrInviteNotFound", func(t *testing.T) {
		for _, spelling := range []string{"not_found", "not found", "Invite Not Found", "  NOT_FOUND  "} {
			t.Run(spelling, func(t *testing.T) {
				requester := &MockRequester{}
				requester.On("RequestWithContext", mock.Anything, mock.Anything, mock.Anything).
					Return(replyWith(t, inviteapi.GetInviteResponse{Error: spelling}), nil)

				invite, err := NewInviteLookup(requester, slog.Default()).GetInvite(ctx, uid)

				// The caller distinguishes "no such invite" (discard quietly) from an
				// operational fault (log loudly) purely by this error identity.
				require.ErrorIs(t, err, domain.ErrInviteNotFound)
				assert.Nil(t, invite)
			})
		}
	})

	t.Run("errors when the invite service reports a non-not-found error", func(t *testing.T) {
		// The reply also carries a record: a responder that sets Error must not have its
		// payload used regardless, so dropping the Error check would let this invite
		// through instead of failing closed.
		requester := &MockRequester{}
		requester.On("RequestWithContext", mock.Anything, mock.Anything, mock.Anything).
			Return(replyWith(t, inviteapi.GetInviteResponse{
				Error: "internal server error",
				Invite: &inviteapi.Invite{
					UID:        uid,
					Status:     inviteapi.InviteStatusAccepted,
					AcceptedBy: "attacker-lfid",
				},
			}), nil)

		invite, err := NewInviteLookup(requester, slog.Default()).GetInvite(ctx, uid)

		require.Error(t, err)
		assert.NotErrorIs(t, err, domain.ErrInviteNotFound)
		assert.Nil(t, invite)
	})

	t.Run("bounds an oversized error string before it reaches the error message", func(t *testing.T) {
		requester := &MockRequester{}
		requester.On("RequestWithContext", mock.Anything, mock.Anything, mock.Anything).
			Return(replyWith(t, inviteapi.GetInviteResponse{Error: strings.Repeat("A", 5000)}), nil)

		invite, err := NewInviteLookup(requester, slog.Default()).GetInvite(ctx, uid)

		require.Error(t, err)
		assert.Nil(t, invite)
		assert.Less(t, len(err.Error()), 300, "attacker-controlled reply text must not set the log line length")
	})

	t.Run("errors when the reply answers a different invite uid", func(t *testing.T) {
		requester := &MockRequester{}
		requester.On("RequestWithContext", mock.Anything, mock.Anything, mock.Anything).
			Return(replyWith(t, inviteapi.GetInviteResponse{Invite: &inviteapi.Invite{
				UID:        "00000000-0000-0000-0000-000000000002",
				Status:     inviteapi.InviteStatusAccepted,
				AcceptedBy: "attacker-lfid",
			}}), nil)

		invite, err := NewInviteLookup(requester, slog.Default()).GetInvite(ctx, uid)

		require.Error(t, err)
		assert.NotErrorIs(t, err, domain.ErrInviteNotFound)
		assert.Nil(t, invite)
	})

	t.Run("errors on an unparseable reply", func(t *testing.T) {
		requester := &MockRequester{}
		requester.On("RequestWithContext", mock.Anything, mock.Anything, mock.Anything).
			Return(&natsgo.Msg{Data: []byte("not json")}, nil)

		invite, err := NewInviteLookup(requester, slog.Default()).GetInvite(ctx, uid)

		require.Error(t, err)
		assert.Nil(t, invite)
	})

	t.Run("errors when the NATS request fails", func(t *testing.T) {
		requester := &MockRequester{}
		requester.On("RequestWithContext", mock.Anything, mock.Anything, mock.Anything).
			Return(nil, errors.New("timeout"))

		invite, err := NewInviteLookup(requester, slog.Default()).GetInvite(ctx, uid)

		require.Error(t, err)
		assert.Nil(t, invite)
	})
}
