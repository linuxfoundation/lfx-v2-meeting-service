// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	inviteapi "github.com/linuxfoundation/lfx-v2-invite-service/pkg/api"
	natsgo "github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

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
		// The UID must be well-formed so the event actually reaches the lookup: the point
		// of this case is the ErrInviteNotFound branch, which a UID rejected earlier by
		// the canonical-UUID guard would never exercise.
		const unknownUID = "00000000-0000-0000-0000-0000000000ff"
		lookup := &fakeInviteLookup{err: domain.ErrInviteNotFound}
		client := &fakeAcceptanceClient{}

		err := processInviteAcceptedEvent(ctx,
			acceptedEvent(unknownUID, victim, attacker, meetingconstants.ResourceTypeMeeting),
			lookup, client, slog.Default())

		require.NoError(t, err, "an unknown invite is discarded quietly, not reported as an error")
		assert.Equal(t, []string{unknownUID}, lookup.requestedID,
			"the lookup must actually run; otherwise this case stops testing the not-found branch")
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

	t.Run("discards a non-accepted invite whose accepted_by matches the event", func(t *testing.T) {
		// Isolates the status check: every other guard passes, so only the status
		// gate can reject this. A re-issued, revoked or partially-written record
		// reaches this shape with a populated accepted_by.
		for _, status := range []inviteapi.InviteStatus{inviteapi.InviteStatusPending, "revoked", ""} {
			t.Run(string(status), func(t *testing.T) {
				invite := acceptedInvite(inviteUID, victim, acceptor, meetingconstants.ResourceTypeMeeting)
				invite.Status = status
				lookup := &fakeInviteLookup{invite: invite}
				client := &fakeAcceptanceClient{}

				err := processInviteAcceptedEvent(ctx,
					acceptedEvent(inviteUID, victim, acceptor, meetingconstants.ResourceTypeMeeting),
					lookup, client, slog.Default())

				require.NoError(t, err)
				assert.Empty(t, client.calls, "only status = accepted may reach ITX")
			})
		}
	})

	t.Run("discards when the lookup returns no record and no error", func(t *testing.T) {
		// The interface permits (nil, nil); this must not panic the process.
		lookup := &fakeInviteLookup{}
		client := &fakeAcceptanceClient{}

		err := processInviteAcceptedEvent(ctx,
			acceptedEvent(inviteUID, victim, attacker, meetingconstants.ResourceTypeMeeting),
			lookup, client, slog.Default())

		require.NoError(t, err)
		assert.Empty(t, client.calls)
	})

	t.Run("discards oversized fields without reaching the lookup", func(t *testing.T) {
		// The subject is publishable by any workload on the bus, and a rejected event
		// still writes a warning naming the invite UID. Without a cap a publisher could
		// drive log volume with values it knows will be rejected.
		cases := map[string]inviteapi.InviteServiceAcceptedEvent{
			"uid": acceptedEvent(strings.Repeat("A", canonicalUUIDLen+1), victim, acceptor,
				meetingconstants.ResourceTypeMeeting),
			"email": acceptedEvent(inviteUID, strings.Repeat("a", maxInviteIdentityLen)+"@example.com",
				acceptor, meetingconstants.ResourceTypeMeeting),
			"username": acceptedEvent(inviteUID, victim, strings.Repeat("u", maxInviteIdentityLen+1),
				meetingconstants.ResourceTypeMeeting),
		}

		for field, evt := range cases {
			t.Run(field, func(t *testing.T) {
				// A lookup that would otherwise verify successfully, so the only thing
				// that can reject this event is the size guard.
				lookup := &fakeInviteLookup{invite: acceptedInvite(inviteUID, victim, acceptor,
					meetingconstants.ResourceTypeMeeting)}
				client := &fakeAcceptanceClient{}

				err := processInviteAcceptedEvent(ctx, evt, lookup, client, slog.Default())

				require.NoError(t, err)
				assert.Empty(t, client.calls, "oversized fields must not reach ITX")
				assert.Empty(t, lookup.requestedID,
					"oversized fields must be rejected before the lookup, not after")
			})
		}
	})

	t.Run("rejects non-canonical invite uids before the lookup", func(t *testing.T) {
		// The invite service mints UIDs with uuid.NewString, so anything else cannot name
		// a real invite. Rejecting the non-canonical forms uuid.Parse would otherwise
		// accept keeps one known shape and stops free text reaching the logs.
		for name, uid := range map[string]string{
			"not a uuid":   "../../etc/passwd",
			"pii as uid":   "victim@example.com-not-a-uuid-padding00",
			"unhyphenated": "00000000000000000000000000000001",
			"urn form":     "urn:uuid:00000000-0000-0000-0000-000000000001",
			"braced form":  "{00000000-0000-0000-0000-000000000001}",
			"empty-ish":    "   ",
		} {
			t.Run(name, func(t *testing.T) {
				lookup := &fakeInviteLookup{invite: acceptedInvite(uid, victim, acceptor,
					meetingconstants.ResourceTypeMeeting)}
				client := &fakeAcceptanceClient{}

				err := processInviteAcceptedEvent(ctx,
					acceptedEvent(uid, victim, acceptor, meetingconstants.ResourceTypeMeeting),
					lookup, client, slog.Default())

				require.NoError(t, err)
				assert.Empty(t, client.calls, "a non-canonical uid must not reach ITX")
				assert.Empty(t, lookup.requestedID,
					"a non-canonical uid must be rejected before the lookup")
			})
		}
	})

	t.Run("bounds an oversized resource type from the lookup reply", func(t *testing.T) {
		// Resource type comes from the reply, so its length is not ours to trust.
		assert.Len(t, boundedResourceType(strings.Repeat("x", 5000)),
			maxResourceTypeLen+len("\u2026"))
		assert.Equal(t, "meeting", boundedResourceType("meeting"),
			"a normal resource type must stay readable")
	})

	t.Run("enriches an invite with no resource type", func(t *testing.T) {
		lookup := &fakeInviteLookup{invite: acceptedInvite(inviteUID, victim, acceptor, "")}
		client := &fakeAcceptanceClient{}

		err := processInviteAcceptedEvent(ctx,
			acceptedEvent(inviteUID, victim, acceptor, ""),
			lookup, client, slog.Default())

		require.NoError(t, err)
		require.Len(t, client.calls, 1)
		assert.Equal(t, victim, client.calls[0].email)
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

func TestInviteAcceptedSubscriber_Handle_ErrorSpanSanitized(t *testing.T) {
	// PII that must never reach the trace backend.
	const spanInviteUID = "00000000-0000-0000-0000-00000000000a"
	piiEmail := "alice@example.com"
	piiBody := `{"message":"alice@example.com is not registered"}`

	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	prevTP := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	defer otel.SetTracerProvider(prevTP)
	defer func() { _ = tp.Shutdown(context.Background()) }()

	client := &fakeAcceptanceClient{err: errors.New(piiBody)}
	// The event must pass verification so the ITX call is reached and its
	// PII-bearing error is what lands on the span.
	lookup := &fakeInviteLookup{invite: acceptedInvite(spanInviteUID, piiEmail, "alice-lfid", meetingconstants.ResourceTypeMeeting)}
	sub := NewInviteAcceptedSubscriber(nil, client, lookup, slog.Default())
	sub.ctx = context.Background() // set ctx directly; Start is not called (no NATS conn needed)

	evt := acceptedEvent(spanInviteUID, piiEmail, "alice-lfid", meetingconstants.ResourceTypeMeeting)
	data, _ := json.Marshal(evt)
	sub.handle(&natsgo.Msg{Data: data})

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 exported span, got %d", len(spans))
	}
	s := spans[0]

	// Check every event attribute and the status description for PII.
	for _, ev := range s.Events {
		for _, attr := range ev.Attributes {
			val := attr.Value.AsString()
			if strings.Contains(val, piiEmail) || strings.Contains(val, piiBody) {
				t.Errorf("PII found in span event %q attr %q=%q", ev.Name, attr.Key, val)
			}
		}
		// exception.message must not carry the upstream body.
		if ev.Name == "exception" {
			for _, attr := range ev.Attributes {
				if string(attr.Key) == "exception.message" {
					if strings.Contains(attr.Value.AsString(), piiEmail) || strings.Contains(attr.Value.AsString(), piiBody) {
						t.Errorf("PII in exception.message: %q", attr.Value.AsString())
					}
				}
			}
		}
	}

	// Span status description must not carry upstream error text.
	statusDesc := s.Status.Description
	if strings.Contains(statusDesc, piiEmail) || strings.Contains(statusDesc, piiBody) {
		t.Errorf("PII in span status description: %q", statusDesc)
	}
}
