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

	"github.com/google/uuid"
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

	// Cap for the identity values read out of an acceptance event. Addresses cannot
	// exceed the RFC 5321 limit, so this is generous; it exists to bound what a publisher
	// can put into a log line, not to validate format.
	maxInviteIdentityLen = 320

	// Length of a canonical hyphenated UUID, the form the invite service mints.
	canonicalUUIDLen = 36

	// Cap for the resource type, which arrives in a lookup reply rather than from the
	// event. The set of LFX resource types is open-ended, so this bounds rather than
	// allow-lists, leaving a new type readable in logs.
	maxResourceTypeLen = 64
)

// InviteAcceptedSubscriber subscribes to lfx.invite-service.invite_accepted events
// and calls the ITX Zoom Service to enrich all DynamoDB records tied to the acceptor's
// email with their new username and profile data.
//
// The subject lives on the shared platform NATS bus, so a message arriving here is a
// notification, not proof that an acceptance happened. Because the enrichment call binds
// an LFID to the Zoom records for an email address using the service's own privileged ITX
// credential, every event is re-verified against the invite service (the owner of invite
// state) before any ITX call is made — see processInviteAcceptedEvent.
//
// That verification is defense in depth, not a complete origin control, for two reasons.
// The get_invite lookup it relies on is reached over the same unauthenticated channel as
// the event it verifies. More importantly, the stored record is itself built from an
// unauthenticated event: the invite service takes the caller-supplied username off
// lfx.invite.accepted and writes it to accepted_by, so a publisher that knows a pending
// invite UID can have an arbitrary LFID recorded as a genuine acceptance, and re-reading
// it here returns exactly that. What re-reading does buy is that an acceptance must match
// a real pending invite and binds that invite's own recipient address, so a publisher can
// no longer name an arbitrary victim — the exposure is narrowed, not closed.
//
// Closing it needs something this service cannot do on its own: NATS permissions scoping
// lfx.invite.accepted to the self-serve identity *and* the lfx.invite-service.* subjects
// to the invite service (platform NATS configuration), or a signed acceptance assertion
// carried from whoever authenticated the user. See docs/event-processing.md.
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
		// A decode error carries bytes from the message body, and this subject is
		// unauthenticated. The span gets a fixed string — a span takes no redacted
		// value, only a constant — and the log gets a schema-derived description
		// plus the payload size, which is the one diagnostic sanitizing costs.
		span.RecordError(errors.New("invite_accepted event parse failed"))
		span.SetStatus(codes.Error, "invite_accepted event parse failed")
		s.logger.With(logging.ErrKey, redaction.RedactJSONError(err)).
			WarnContext(ctx, "failed to parse InviteServiceAcceptedEvent; discarding",
				"payload_bytes", len(msg.Data),
			)
		return
	}

	if err := processInviteAcceptedEvent(ctx, evt, s.inviteLookup, s.acceptanceClient, s.logger); err != nil {
		span.RecordError(errors.New("invite acceptance failed"))
		span.SetStatus(codes.Error, "invite acceptance failed")
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
// The event body is untrusted. `AcceptInvite` binds the supplied LFID to the registrant,
// past-meeting invitee and past-meeting attendee rows carrying the supplied email, and
// those rows feed downstream access-control state — so the binding is validated against
// the invite service rather than taken from the message. The event is therefore treated
// purely as a hint to go and re-read the authoritative record:
//
//  1. The event must name an invite UID.
//  2. That invite is re-fetched from the invite service, which owns invite state and is
//     where an acceptance is recorded. The record says what was recorded, not who
//     performed it — see the trust boundary on InviteAcceptedSubscriber above.
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

	// The invite UID is echoed into a warning on every rejected event, so it must not be
	// free text. The invite service mints it with uuid.NewString — it is the invite JWT's
	// jti — so anything that is not a canonical UUID cannot name a real invite. Requiring
	// that form bounds the value and stops a publisher parking arbitrary text in this
	// service's logs, which redaction would not catch: a UID is not an identity field, so
	// nothing redacts it.
	if !isCanonicalUUID(inviteUID) {
		logger.WarnContext(ctx, "invite_accepted event carries a non-canonical invite uid; discarding",
			"invite_uid_len", len(inviteUID),
		)
		return nil
	}

	// Addresses and usernames are redacted where they are logged, but redaction bounds a
	// username and leaves an email's domain intact, so cap both. Log the lengths, not the
	// values — echoing them is the thing being avoided.
	if len(claimedEmail) > maxInviteIdentityLen || len(claimedUsername) > maxInviteIdentityLen {
		logger.WarnContext(ctx, "invite_accepted event carries oversized identity fields; discarding",
			"email_len", len(claimedEmail),
			"username_len", len(claimedUsername),
		)
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
			"resource_type", boundedResourceType(invite.Resource.Type),
		)
	} else {
		logger.DebugContext(ctx, "received invite_accepted event",
			"email", redaction.RedactEmail(verifiedEmail),
			"username", redaction.Redact(verifiedUsername),
			"resource_type", boundedResourceType(invite.Resource.Type),
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

// isCanonicalUUID reports whether s is a UUID in the canonical hyphenated form the invite
// service mints. uuid.Parse alone also accepts the urn:, braced and unhyphenated forms, so
// the length is checked too: the point is to pin one known shape, not to be permissive.
func isCanonicalUUID(s string) bool {
	if len(s) != canonicalUUIDLen {
		return false
	}
	_, err := uuid.Parse(s)
	return err == nil
}

// boundedResourceType renders a resource type for logging. The value comes from a lookup
// reply, so its length is not ours to trust; unlike status it is not a closed set, so it
// is truncated rather than matched against an allow-list.
func boundedResourceType(resourceType string) string {
	if len(resourceType) <= maxResourceTypeLen {
		return resourceType
	}
	return resourceType[:maxResourceTypeLen] + "…"
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
