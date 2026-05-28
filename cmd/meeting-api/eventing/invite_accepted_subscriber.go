// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"

	indexerConstants "github.com/linuxfoundation/lfx-v2-indexer-service/pkg/constants"
	inviteapi "github.com/linuxfoundation/lfx-v2-invite-service/pkg/api"
	natsgo "github.com/nats-io/nats.go"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/models/itx"
)

const inviteAcceptedQueueGroup = "meeting-service-invite-accepted"

// inviteAcceptedPayload is the NATS event published by the LFX self-serve web app.
// The invite-service is expected to enrich this with email, resource_uid, and resource_type.
type inviteAcceptedPayload struct {
	InviteUID    string `json:"invite_uid"`
	Username     string `json:"username"`
	Email        string `json:"email"`
	ResourceUID  string `json:"resource_uid"`
	ResourceType string `json:"resource_type"`
}

// registrantReconciler is the subset of RegistrantService used by the subscriber.
type registrantReconciler interface {
	ReconcileAcceptedInvite(ctx context.Context, email, username string) ([]*itx.ZoomMeetingRegistrant, error)
}

// InviteAcceptedSubscriber subscribes to lfx.invite.accepted and reconciles LFID
// username updates across all ITX meeting registrants for the accepted email.
type InviteAcceptedSubscriber struct {
	nc         *natsgo.Conn
	reconciler registrantReconciler
	publisher  domain.EventPublisher
	sub        *natsgo.Subscription
}

// NewInviteAcceptedSubscriber creates the subscriber. Call Start to begin consuming.
func NewInviteAcceptedSubscriber(
	nc *natsgo.Conn,
	reconciler registrantReconciler,
	publisher domain.EventPublisher,
) *InviteAcceptedSubscriber {
	return &InviteAcceptedSubscriber{
		nc:         nc,
		reconciler: reconciler,
		publisher:  publisher,
	}
}

// Start registers the queue subscription. The ctx is passed to each handler invocation.
func (s *InviteAcceptedSubscriber) Start(ctx context.Context) error {
	sub, err := s.nc.QueueSubscribe(inviteapi.InviteAcceptedSubject, inviteAcceptedQueueGroup, func(msg *natsgo.Msg) {
		s.handle(ctx, msg)
	})
	if err != nil {
		return err
	}
	s.sub = sub
	slog.InfoContext(ctx, "invite_accepted subscriber started",
		"subject", inviteapi.InviteAcceptedSubject,
		"queue", inviteAcceptedQueueGroup)
	return nil
}

// Stop drains and unsubscribes.
func (s *InviteAcceptedSubscriber) Stop() {
	if s.sub != nil {
		if err := s.sub.Drain(); err != nil {
			slog.Warn("error draining invite_accepted subscription", "err", err)
		}
	}
}

func (s *InviteAcceptedSubscriber) handle(ctx context.Context, msg *natsgo.Msg) {
	var evt inviteAcceptedPayload
	if err := json.Unmarshal(msg.Data, &evt); err != nil {
		slog.WarnContext(ctx, "invite_accepted: failed to unmarshal payload", "err", err)
		return
	}

	if evt.InviteUID == "" || evt.Username == "" {
		slog.WarnContext(ctx, "invite_accepted: missing invite_uid or username — discarding",
			"invite_uid", evt.InviteUID)
		return
	}
	if evt.Email == "" {
		slog.WarnContext(ctx, "invite_accepted: missing email — cannot fan out; invite-service may need updating",
			"invite_uid", evt.InviteUID)
		return
	}

	updated, err := s.reconciler.ReconcileAcceptedInvite(ctx, evt.Email, evt.Username)
	if err != nil {
		slog.WarnContext(ctx, "invite_accepted: ITX reconcile failed",
			"invite_uid", evt.InviteUID, "email", evt.Email, "err", err)
		return
	}

	for _, reg := range updated {
		eventData := itxRegistrantToEventData(reg)
		if err := s.publisher.PublishRegistrantEvent(ctx, string(indexerConstants.ActionUpdated), eventData); err != nil {
			slog.WarnContext(ctx, "invite_accepted: failed to publish registrant event",
				"registrant_id", reg.ID, "meeting_id", reg.MeetingID, "err", err)
		}
	}
}

// itxRegistrantToEventData converts an ITX registrant to a RegistrantEventData for publishing.
func itxRegistrantToEventData(reg *itx.ZoomMeetingRegistrant) *models.RegistrantEventData {
	email := strings.ToLower(reg.Email)
	return &models.RegistrantEventData{
		UID:                  reg.ID,
		MeetingID:            reg.MeetingID,
		Type:                 string(reg.Type),
		CommitteeUID:         reg.CommitteeID,
		UserID:               reg.UserID,
		Email:                reg.Email,
		CaseInsensitiveEmail: email,
		FirstName:            reg.FirstName,
		LastName:             reg.LastName,
		JobTitle:             reg.JobTitle,
		Host:                 reg.Host,
		Occurrence:           reg.Occurrence,
		AvatarURL:            reg.ProfilePicture,
		Username:             reg.Username,
		CreatedAt:            reg.CreatedAt,
		UpdatedAt:            reg.ModifiedAt,
	}
}
