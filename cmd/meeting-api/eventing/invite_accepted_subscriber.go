// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"encoding/json"
	"log/slog"

	natsgo "github.com/nats-io/nats.go"

	inviteapi "github.com/linuxfoundation/lfx-v2-invite-service/pkg/api"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
)

const inviteAcceptedQueueGroup = "meeting-service-invite-accepted"

// InviteAcceptedSubscriber subscribes to lfx.invite-service.invite_accepted events
// and calls the ITX Zoom Service to enrich all DynamoDB records tied to the acceptor's
// email with their new username and profile data.
type InviteAcceptedSubscriber struct {
	nc               *natsgo.Conn
	acceptanceClient domain.InviteAcceptanceClient
	logger           *slog.Logger
	sub              *natsgo.Subscription
}

// NewInviteAcceptedSubscriber creates a new subscriber but does not start it.
func NewInviteAcceptedSubscriber(
	nc *natsgo.Conn,
	acceptanceClient domain.InviteAcceptanceClient,
	logger *slog.Logger,
) *InviteAcceptedSubscriber {
	return &InviteAcceptedSubscriber{
		nc:               nc,
		acceptanceClient: acceptanceClient,
		logger:           logger,
	}
}

// Start registers the NATS QueueSubscribe and begins processing acceptance events.
func (s *InviteAcceptedSubscriber) Start(_ context.Context) error {
	sub, err := s.nc.QueueSubscribe(
		inviteapi.InviteServiceAcceptedSubject,
		inviteAcceptedQueueGroup,
		s.handle,
	)
	if err != nil {
		return err
	}
	s.sub = sub
	s.logger.Info("invite_accepted subscriber started", "subject", inviteapi.InviteServiceAcceptedSubject)
	return nil
}

// Stop drains the subscription. It is a no-op if Start was never called.
func (s *InviteAcceptedSubscriber) Stop() {
	if s.sub == nil {
		return
	}
	if err := s.sub.Drain(); err != nil {
		s.logger.With(logging.ErrKey, err).Warn("error draining invite_accepted subscription")
	}
}

// handle processes a single InviteServiceAcceptedEvent message.
func (s *InviteAcceptedSubscriber) handle(msg *natsgo.Msg) {
	ctx := context.Background()

	var evt inviteapi.InviteServiceAcceptedEvent
	if err := json.Unmarshal(msg.Data, &evt); err != nil {
		s.logger.With(logging.ErrKey, err).Warn("failed to parse InviteServiceAcceptedEvent; discarding")
		return
	}

	email := evt.Recipient.Email
	username := evt.AcceptedBy

	if email == "" || username == "" {
		s.logger.Warn("invite_accepted event missing required fields; discarding")
		return
	}

	s.logger.Info("processing invite_accepted enrichment",
		"resource_type", evt.Resource.Type,
	)

	if err := s.acceptanceClient.AcceptInvite(ctx, email, username); err != nil {
		s.logger.With(logging.ErrKey, err).Warn("invite_accepted enrichment failed; best-effort, not retrying")
		return
	}

	s.logger.Info("invite_accepted enrichment complete")
}
