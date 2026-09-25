// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	inviteapi "github.com/linuxfoundation/lfx-v2-invite-service/pkg/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	meetingconstants "github.com/linuxfoundation/lfx-v2-meeting-service/pkg/constants"
)

func TestInviteAcceptedSubscriber_StopWithNoInFlightMessages(t *testing.T) {
	sub := NewInviteAcceptedSubscriber(nil, nil, nil, slog.Default())

	done := make(chan struct{})
	go func() {
		sub.Stop()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop() blocked — possible WaitGroup misuse")
	}
}

// fakeInviteLookup returns a canned invite record (or error) for any UID and records
// the UIDs it was asked about.
type fakeInviteLookup struct {
	invite      *inviteapi.Invite
	err         error
	requestedID []string
}

func (f *fakeInviteLookup) GetInvite(_ context.Context, uid string) (*inviteapi.Invite, error) {
	f.requestedID = append(f.requestedID, uid)
	if f.err != nil {
		return nil, f.err
	}
	return f.invite, nil
}

// fakeAcceptanceClient records the arguments of every AcceptInvite call.
type fakeAcceptanceClient struct {
	calls []acceptInviteCall
	err   error
}

type acceptInviteCall struct {
	email    string
	username string
}

func (f *fakeAcceptanceClient) AcceptInvite(_ context.Context, email, username string) error {
	f.calls = append(f.calls, acceptInviteCall{email: email, username: username})
	return f.err
}

// acceptedInvite builds an invite record in the state the invite service publishes
// after a genuine acceptance.
func acceptedInvite(uid, email, acceptedBy, resourceType string) *inviteapi.Invite {
	return &inviteapi.Invite{
		UID:        uid,
		Status:     inviteapi.InviteStatusAccepted,
		Recipient:  inviteapi.Recipient{Email: email},
		Resource:   inviteapi.Resource{UID: "mtg-1", Type: resourceType},
		AcceptedBy: acceptedBy,
	}
}

// acceptedEvent builds the NATS event body for a given invite record.
func acceptedEvent(uid, email, acceptedBy, resourceType string) inviteapi.InviteServiceAcceptedEvent {
	return inviteapi.InviteServiceAcceptedEvent{
		Invite: inviteapi.Invite{
			UID:        uid,
			Status:     inviteapi.InviteStatusAccepted,
			Recipient:  inviteapi.Recipient{Email: email},
			Resource:   inviteapi.Resource{UID: "mtg-1", Type: resourceType},
			AcceptedBy: acceptedBy,
		},
	}
}

func TestProcessInviteAcceptedEvent(t *testing.T) {
	const (
		inviteUID = "00000000-0000-0000-0000-000000000001"
		victim    = "alice@example.com"
		attacker  = "attacker-lfid"
		acceptor  = "alice"
	)
	ctx := context.Background()

	t.Run("enriches when the invite service confirms the acceptance", func(t *testing.T) {
		lookup := &fakeInviteLookup{invite: acceptedInvite(inviteUID, victim, acceptor, meetingconstants.ResourceTypeMeeting)}
		client := &fakeAcceptanceClient{}

		err := processInviteAcceptedEvent(ctx,
			acceptedEvent(inviteUID, victim, acceptor, meetingconstants.ResourceTypeMeeting),
			lookup, client, slog.Default())

		require.NoError(t, err)
		require.Len(t, client.calls, 1)
		assert.Equal(t, victim, client.calls[0].email)
		assert.Equal(t, acceptor, client.calls[0].username)
		assert.Equal(t, []string{inviteUID}, lookup.requestedID)
	})

	t.Run("enriches for non-meeting resource types", func(t *testing.T) {
		lookup := &fakeInviteLookup{invite: acceptedInvite(inviteUID, victim, acceptor, "committee")}
		client := &fakeAcceptanceClient{}

		err := processInviteAcceptedEvent(ctx,
			acceptedEvent(inviteUID, victim, acceptor, "committee"),
			lookup, client, slog.Default())

		require.NoError(t, err)
		require.Len(t, client.calls, 1)
		assert.Equal(t, victim, client.calls[0].email)
		assert.Equal(t, acceptor, client.calls[0].username)
	})

	t.Run("uses the stored record's values, not the event's", func(t *testing.T) {
		lookup := &fakeInviteLookup{invite: acceptedInvite(inviteUID, victim, acceptor, meetingconstants.ResourceTypeMeeting)}
		client := &fakeAcceptanceClient{}

		// Same identities, different casing/whitespace in the event body.
		err := processInviteAcceptedEvent(ctx,
			acceptedEvent(inviteUID, "  ALICE@Example.com ", "  Alice ", meetingconstants.ResourceTypeMeeting),
			lookup, client, slog.Default())

		require.NoError(t, err)
		require.Len(t, client.calls, 1)
		assert.Equal(t, victim, client.calls[0].email, "ITX must be called with the invite service's email")
		assert.Equal(t, acceptor, client.calls[0].username, "ITX must be called with the invite service's username")
	})

	t.Run("discards a forged event whose invite the invite service does not know", func(t *testing.T) {
		lookup := &fakeInviteLookup{err: domain.ErrInviteNotFound}
		client := &fakeAcceptanceClient{}

		err := processInviteAcceptedEvent(ctx,
			acceptedEvent("forged-uid", victim, attacker, meetingconstants.ResourceTypeMeeting),
			lookup, client, slog.Default())

		require.NoError(t, err)
		assert.Empty(t, client.calls, "no ITX call for an unknown invite")
	})

	t.Run("discards a forged event that binds an attacker username to a real invite", func(t *testing.T) {
		// The invite exists and was genuinely accepted by its recipient, but the
		// attacker replays its UID with their own accepted_by.
		lookup := &fakeInviteLookup{invite: acceptedInvite(inviteUID, victim, acceptor, meetingconstants.ResourceTypeMeeting)}
		client := &fakeAcceptanceClient{}

		err := processInviteAcceptedEvent(ctx,
			acceptedEvent(inviteUID, victim, attacker, meetingconstants.ResourceTypeMeeting),
			lookup, client, slog.Default())

		require.NoError(t, err)
		assert.Empty(t, client.calls, "no ITX call when accepted_by does not match the stored record")
	})

	t.Run("discards a forged event that swaps in a different victim email", func(t *testing.T) {
		lookup := &fakeInviteLookup{invite: acceptedInvite(inviteUID, "bob@example.com", acceptor, meetingconstants.ResourceTypeMeeting)}
		client := &fakeAcceptanceClient{}

		err := processInviteAcceptedEvent(ctx,
			acceptedEvent(inviteUID, victim, acceptor, meetingconstants.ResourceTypeMeeting),
			lookup, client, slog.Default())

		require.NoError(t, err)
		assert.Empty(t, client.calls, "no ITX call when the recipient email does not match the stored record")
	})

	t.Run("discards an event for an invite that is still pending", func(t *testing.T) {
		invite := acceptedInvite(inviteUID, victim, "", meetingconstants.ResourceTypeMeeting)
		invite.Status = inviteapi.InviteStatusPending
		lookup := &fakeInviteLookup{invite: invite}
		client := &fakeAcceptanceClient{}

		err := processInviteAcceptedEvent(ctx,
			acceptedEvent(inviteUID, victim, attacker, meetingconstants.ResourceTypeMeeting),
			lookup, client, slog.Default())

		require.NoError(t, err)
		assert.Empty(t, client.calls, "no ITX call for an invite that has not been accepted")
	})

	t.Run("discards an accepted record with an empty accepted_by", func(t *testing.T) {
		lookup := &fakeInviteLookup{invite: acceptedInvite(inviteUID, victim, "", meetingconstants.ResourceTypeMeeting)}
		client := &fakeAcceptanceClient{}

		err := processInviteAcceptedEvent(ctx,
			acceptedEvent(inviteUID, victim, attacker, meetingconstants.ResourceTypeMeeting),
			lookup, client, slog.Default())

		require.NoError(t, err)
		assert.Empty(t, client.calls)
	})

	t.Run("discards an event with no invite uid", func(t *testing.T) {
		lookup := &fakeInviteLookup{invite: acceptedInvite(inviteUID, victim, acceptor, meetingconstants.ResourceTypeMeeting)}
		client := &fakeAcceptanceClient{}

		err := processInviteAcceptedEvent(ctx,
			acceptedEvent("  ", victim, attacker, meetingconstants.ResourceTypeMeeting),
			lookup, client, slog.Default())

		require.NoError(t, err)
		assert.Empty(t, lookup.requestedID, "an event without an invite uid is dropped before any lookup")
		assert.Empty(t, client.calls)
	})

	t.Run("discards an event missing email or username", func(t *testing.T) {
		for name, evt := range map[string]inviteapi.InviteServiceAcceptedEvent{
			"no email":    acceptedEvent(inviteUID, "", acceptor, meetingconstants.ResourceTypeMeeting),
			"no username": acceptedEvent(inviteUID, victim, "", meetingconstants.ResourceTypeMeeting),
		} {
			t.Run(name, func(t *testing.T) {
				lookup := &fakeInviteLookup{invite: acceptedInvite(inviteUID, victim, acceptor, meetingconstants.ResourceTypeMeeting)}
				client := &fakeAcceptanceClient{}

				err := processInviteAcceptedEvent(ctx, evt, lookup, client, slog.Default())

				require.NoError(t, err)
				assert.Empty(t, client.calls)
			})
		}
	})

	t.Run("fails closed and reports the error when the lookup cannot be completed", func(t *testing.T) {
		lookup := &fakeInviteLookup{err: errors.New("nats timeout")}
		client := &fakeAcceptanceClient{}

		err := processInviteAcceptedEvent(ctx,
			acceptedEvent(inviteUID, victim, attacker, meetingconstants.ResourceTypeMeeting),
			lookup, client, slog.Default())

		require.Error(t, err)
		assert.Empty(t, client.calls, "a transient verification failure must not fall through to an ITX call")
	})

	t.Run("fails closed when no invite lookup is configured", func(t *testing.T) {
		client := &fakeAcceptanceClient{}

		err := processInviteAcceptedEvent(ctx,
			acceptedEvent(inviteUID, victim, attacker, meetingconstants.ResourceTypeMeeting),
			nil, client, slog.Default())

		require.NoError(t, err)
		assert.Empty(t, client.calls)
	})

	t.Run("propagates ITX errors on a verified acceptance", func(t *testing.T) {
		lookup := &fakeInviteLookup{invite: acceptedInvite(inviteUID, victim, acceptor, meetingconstants.ResourceTypeMeeting)}
		client := &fakeAcceptanceClient{err: errors.New("itx unavailable")}

		err := processInviteAcceptedEvent(ctx,
			acceptedEvent(inviteUID, victim, acceptor, meetingconstants.ResourceTypeMeeting),
			lookup, client, slog.Default())

		require.Error(t, err)
		assert.Len(t, client.calls, 1)
	})
}
