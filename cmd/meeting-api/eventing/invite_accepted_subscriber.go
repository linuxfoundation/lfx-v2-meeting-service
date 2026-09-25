// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	natsgo "github.com/nats-io/nats.go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	inviteapi "github.com/linuxfoundation/lfx-v2-invite-service/pkg/api"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
	meetingconstants "github.com/linuxfoundation/lfx-v2-meeting-service/pkg/constants"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/redaction"
)

const (
	inviteAcceptedQueueGroup  = "meeting-service-invite-accepted"
	inviteAcceptedCallTimeout = 30 * time.Second
)

// InviteAcceptedSubscriber subscribes to lfx.invite-service.invite_accepted events
// and calls the ITX Zoom Service to enrich all DynamoDB records tied to the acceptor's
// email with their new username and profile data.
//
// The subject lives on the shared platform NATS bus, so a message arriving here is an
// unauthenticated notification, not proof that an acceptance happened: any workload
// with reach to the bus can publish one. Because the enrichment call binds an LFID to
// every Zoom record for an email address using the service's own privileged ITX
// credential, every event is re-verified against the invite service (the owner of
// invite state) before any ITX call is made — see processInviteAcceptedEvent.
//
// That verification is defense in depth, not a complete origin control. The get_invite
// request/reply it relies on rides the same credential-free bus: a workload that can
// forge an acceptance event can also race or impersonate the responder and forge the
// reply that verifies it. Closing the hole outright needs a control this service cannot
// implement on its own — NATS account permissions restricting publish and reply on the
// invite-service subjects to the invite service identity (platform NATS configuration),
// or a cryptographically signed acceptance assertion from the invite service. Until one
// of those exists, this check raises the cost of the attack; it does not eliminate it.
type InviteAcceptedSubscriber struct {
	nc               *natsgo.Conn
	acceptanceClient domain.InviteAcceptanceClient
	inviteLookup     domain.InviteLookup
	logger           *slog.Logger
	sub              *natsgo.Subscription

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewInviteAcceptedSubscriber creates a new subscriber but does not start it.
// inviteLookup is required: without it no event can be verified, and the subscriber
// discards everything it receives rather than acting on unverified input.
func NewInviteAcceptedSubscriber(
	nc *natsgo.Conn,
	acceptanceClient domain.InviteAcceptanceClient,
	inviteLookup domain.InviteLookup,
	logger *slog.Logger,
) *InviteAcceptedSubscriber {
	return &InviteAcceptedSubscriber{
		nc:               nc,
		acceptanceClient: acceptanceClient,
		inviteLookup:     inviteLookup,
		logger:           logger,
	}
}

// Start registers the NATS QueueSubscribe and begins processing acceptance events.
func (s *InviteAcceptedSubscriber) Start(ctx context.Context) error {
	s.ctx, s.cancel = context.WithCancel(ctx)

	sub, err := s.nc.QueueSubscribe(
		inviteapi.InviteServiceAcceptedSubject,
		inviteAcceptedQueueGroup,
		s.handle,
	)
	if err != nil {
		if s.cancel != nil {
			s.cancel()
		}
		return err
	}
	s.sub = sub
	s.logger.InfoContext(ctx, "invite_accepted subscriber started", "subject", inviteapi.InviteServiceAcceptedSubject)
	return nil
}

// Stop cancels in-flight handlers, drains the subscription, and waits for handlers to finish.
func (s *InviteAcceptedSubscriber) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	if s.sub != nil {
		if err := s.sub.Drain(); err != nil {
			s.logger.With(logging.ErrKey, err).Warn("error draining invite_accepted subscription")
		}
	}
	s.wg.Wait()
}

// handle processes a single InviteServiceAcceptedEvent message.
func (s *InviteAcceptedSubscriber) handle(msg *natsgo.Msg) {
	s.wg.Add(1)
	defer s.wg.Done()

	msgCtx := otel.GetTextMapPropagator().Extract(s.ctx, natsHeaderCarrier(msg.Header))
	msgCtx, span := tracer.Start(msgCtx, "nats.process",
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithAttributes(
			attribute.String("messaging.system", "nats"),
			attribute.String("messaging.destination.name", msg.Subject),
			attribute.String("messaging.operation.type", "process"),
		),
	)
	defer span.End()

	ctx, cancel := context.WithTimeout(msgCtx, inviteAcceptedCallTimeout)
	defer cancel()

	var evt inviteapi.InviteServiceAcceptedEvent
	if err := json.Unmarshal(msg.Data, &evt); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		s.logger.With(logging.ErrKey, err).WarnContext(ctx, "failed to parse InviteServiceAcceptedEvent; discarding")
		return
	}

	if err := processInviteAcceptedEvent(ctx, evt, s.inviteLookup, s.acceptanceClient, s.logger); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		s.logger.With(logging.ErrKey, err).WarnContext(ctx, "invite_accepted enrichment failed; best-effort, not retrying",
			"email", redaction.RedactEmail(evt.Recipient.Email),
			"username", redaction.Redact(evt.AcceptedBy),
		)
	}
}

// processInviteAcceptedEvent verifies an invite acceptance event against the invite
// service and, only if it checks out, calls ITX to enrich all Zoom records for the
// acceptor's email.
//
// The event body is untrusted. `AcceptInvite` binds the supplied LFID to every
// registrant, past-meeting invitee and past-meeting attendee row carrying the supplied
// email — and those rows flow back through the v1 KV bucket into FGA host/participant
// tuples — so acting on the message as sent would let any publisher on the bus attach
// an LFID of its choosing to another person's meeting records. The event is therefore
// treated purely as a hint to go and re-read the authoritative record:
//
//  1. The event must name an invite UID.
//  2. That invite is re-fetched from the invite service, which owns invite state and is
//     the only component that records who completed the acceptance flow.
//  3. The stored record must say the invite is accepted, and its accepted_by and
//     recipient email must match what the event claimed.
//  4. The ITX call is made with the values from the stored record, never from the event.
//
// Anything else — unknown invite, still pending, mismatch, or a lookup that could not be
// completed — results in no ITX call.
//
// Enrichment intentionally runs for every resource_type (meeting, project, committee, etc.)
// because the ITX endpoint is keyed by email, mirroring project/committee reconciliation:
// a user who accepts any LFID invite may have pending meeting registrant rows that need
// username enrichment. This is a lightweight, idempotent POST keyed by email — acceptable
// at current invite volumes; if platform-wide acceptance throughput grows significantly,
// consider filtering on resource_type or moving enrichment to a shared worker.
func processInviteAcceptedEvent(
	ctx context.Context,
	evt inviteapi.InviteServiceAcceptedEvent,
	lookup domain.InviteLookup,
	client domain.InviteAcceptanceClient,
	logger *slog.Logger,
) error {
	inviteUID := strings.TrimSpace(evt.UID)
	claimedEmail := strings.TrimSpace(evt.Recipient.Email)
	claimedUsername := strings.TrimSpace(evt.AcceptedBy)

	if inviteUID == "" || claimedEmail == "" || claimedUsername == "" {
		logger.WarnContext(ctx, "invite_accepted event missing required fields; discarding")
		return nil
	}

	if lookup == nil {
		// Fail closed: an acceptance that cannot be verified is not acted on.
		logger.WarnContext(ctx, "invite lookup unavailable; discarding unverified invite_accepted event",
			"invite_uid", inviteUID,
		)
		return nil
	}

	invite, err := lookup.GetInvite(ctx, inviteUID)
	if err != nil {
		if errors.Is(err, domain.ErrInviteNotFound) {
			logger.WarnContext(ctx, "invite_accepted event references an unknown invite; discarding",
				"invite_uid", inviteUID,
			)
			return nil
		}
		return fmt.Errorf("failed to verify invite %q: %w", inviteUID, err)
	}
	// A lookup returning no record and no error would otherwise panic on the field
	// accesses below, and this runs on a NATS callback goroutine where a panic takes
	// the process down. The interface permits it; treat it as unverified.
	if invite == nil {
		logger.WarnContext(ctx, "invite lookup returned no record; discarding unverified invite_accepted event",
			"invite_uid", inviteUID,
		)
		return nil
	}

	// Use the invite service's record, not the event body, for everything that follows.
	verifiedEmail := strings.TrimSpace(invite.Recipient.Email)
	verifiedUsername := strings.TrimSpace(invite.AcceptedBy)

	if invite.Status != inviteapi.InviteStatusAccepted || verifiedEmail == "" || verifiedUsername == "" {
		logger.WarnContext(ctx, "invite_accepted event for an invite the invite service does not report as accepted; discarding",
			"invite_uid", inviteUID,
			"status", boundedStatus(invite.Status),
		)
		return nil
	}

	if !strings.EqualFold(verifiedEmail, claimedEmail) || !strings.EqualFold(verifiedUsername, claimedUsername) {
		logger.WarnContext(ctx, "invite_accepted event does not match the stored invite record; discarding",
			"invite_uid", inviteUID,
			"claimed_email", redaction.RedactEmail(claimedEmail),
			"claimed_username", redaction.Redact(claimedUsername),
		)
		return nil
	}

	if invite.Resource.Type != "" && invite.Resource.Type != meetingconstants.ResourceTypeMeeting {
		logger.DebugContext(ctx, "received invite_accepted event for non-meeting resource; enriching Zoom records by email",
			"email", redaction.RedactEmail(verifiedEmail),
			"resource_type", invite.Resource.Type,
		)
	} else {
		logger.DebugContext(ctx, "received invite_accepted event",
			"email", redaction.RedactEmail(verifiedEmail),
			"username", redaction.Redact(verifiedUsername),
			"resource_type", invite.Resource.Type,
		)
	}

	if err := client.AcceptInvite(ctx, verifiedEmail, verifiedUsername); err != nil {
		return err
	}

	logger.InfoContext(ctx, "invite_accepted enrichment complete",
		"invite_uid", inviteUID,
		"email", redaction.RedactEmail(verifiedEmail),
		"username", redaction.Redact(verifiedUsername),
	)
	return nil
}

// boundedStatus renders an invite status for logging. InviteStatus is an open string
// type carried in a reply, so an unrecognised value is reported as a fixed placeholder
// rather than echoed at whatever length the responder chose.
func boundedStatus(status inviteapi.InviteStatus) string {
	switch status {
	case inviteapi.InviteStatusPending, inviteapi.InviteStatusAccepted:
		return string(status)
	case "":
		return "(empty)"
	default:
		return "(unrecognised)"
	}
}
