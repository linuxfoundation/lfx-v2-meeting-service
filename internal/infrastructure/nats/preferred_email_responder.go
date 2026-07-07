// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package nats

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	natsgo "github.com/nats-io/nats.go"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/constants"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/redaction"
)

const preferredEmailCallTimeout = 15 * time.Second

// PreferredEmailProvider is the service behavior the responder needs.
type PreferredEmailProvider interface {
	// GetPreferredEmail returns the user's preferred meeting-invite email, or nil for primary.
	GetPreferredEmail(ctx context.Context, username string) (*domain.PreferredEmail, error)
	// SetPreferredEmail sets the preference from a verified address (email) or an SFDC
	// email-record ID (emailID); email takes precedence. An empty selection or "primary" clears it.
	SetPreferredEmail(ctx context.Context, username, email, emailID string) (*domain.PreferredEmail, error)
}

// preferredEmailRequest is the RPC request payload for get/set.
type preferredEmailRequest struct {
	User    string  `json:"user"`
	Email   *string `json:"email,omitempty"`
	EmailID *string `json:"email_id,omitempty"`
}

// preferredEmailReply is the RPC success reply payload.
type preferredEmailReply struct {
	EmailID *string `json:"email_id"`
	Email   *string `json:"email"`
}

// errorReply is the RPC error envelope.
type errorReply struct {
	Error string `json:"error"`
}

// PreferredEmailResponder subscribes to the preferred-email RPC subjects and replies
// with the user's preferred meeting-invite email selection.
type PreferredEmailResponder struct {
	nc      *natsgo.Conn
	service PreferredEmailProvider
	logger  *slog.Logger

	subs []*natsgo.Subscription

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewPreferredEmailResponder creates a new responder but does not start it.
func NewPreferredEmailResponder(nc *natsgo.Conn, service PreferredEmailProvider, logger *slog.Logger) *PreferredEmailResponder {
	return &PreferredEmailResponder{nc: nc, service: service, logger: logger}
}

// Start registers the QueueSubscribe handlers for both RPC subjects.
func (r *PreferredEmailResponder) Start(ctx context.Context) error {
	r.ctx, r.cancel = context.WithCancel(ctx)

	subjects := map[string]natsgo.MsgHandler{
		constants.PreferredEmailGetSubject: r.handleGet,
		constants.PreferredEmailSetSubject: r.handleSet,
	}

	for subject, handler := range subjects {
		sub, err := r.nc.QueueSubscribe(subject, constants.PreferredEmailQueueGroup, handler)
		if err != nil {
			r.stopSubscriptions()
			if r.cancel != nil {
				r.cancel()
			}
			return err
		}
		r.subs = append(r.subs, sub)
	}

	r.logger.Info("preferred_email responder started",
		"get_subject", constants.PreferredEmailGetSubject,
		"set_subject", constants.PreferredEmailSetSubject,
	)
	return nil
}

// Stop cancels in-flight handlers, drains subscriptions, and waits for completion.
func (r *PreferredEmailResponder) Stop() {
	if r.cancel != nil {
		r.cancel()
	}
	r.stopSubscriptions()
	r.wg.Wait()
}

func (r *PreferredEmailResponder) stopSubscriptions() {
	for _, sub := range r.subs {
		if sub == nil {
			continue
		}
		if err := sub.Drain(); err != nil {
			r.logger.With(logging.ErrKey, err).Warn("error draining preferred_email subscription")
		}
	}
}

// handleGet processes a preferred_email.get request.
func (r *PreferredEmailResponder) handleGet(msg *natsgo.Msg) {
	r.wg.Add(1)
	defer r.wg.Done()

	ctx, cancel := context.WithTimeout(r.ctx, preferredEmailCallTimeout)
	defer cancel()

	req, ok := r.decode(msg)
	if !ok {
		return
	}

	pref, err := r.service.GetPreferredEmail(ctx, req.User)
	if err != nil {
		r.respondError(msg, req.User, "get", err)
		return
	}
	r.respondSuccess(msg, pref)
}

// handleSet processes a preferred_email.set request.
func (r *PreferredEmailResponder) handleSet(msg *natsgo.Msg) {
	r.wg.Add(1)
	defer r.wg.Done()

	ctx, cancel := context.WithTimeout(r.ctx, preferredEmailCallTimeout)
	defer cancel()

	req, ok := r.decode(msg)
	if !ok {
		return
	}

	email := ""
	if req.Email != nil {
		email = *req.Email
	}
	emailID := ""
	if req.EmailID != nil {
		emailID = *req.EmailID
	}

	pref, err := r.service.SetPreferredEmail(ctx, req.User, email, emailID)
	if err != nil {
		r.respondError(msg, req.User, "set", err)
		return
	}
	r.respondSuccess(msg, pref)
}

// decode parses the request payload, replying with an error envelope on failure.
func (r *PreferredEmailResponder) decode(msg *natsgo.Msg) (preferredEmailRequest, bool) {
	var req preferredEmailRequest
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		r.logger.With(logging.ErrKey, err).Warn("failed to parse preferred_email request")
		r.reply(msg, errorReply{Error: "invalid request payload"})
		return preferredEmailRequest{}, false
	}
	return req, true
}

// respondSuccess replies with the preferred-email selection (null fields for primary).
func (r *PreferredEmailResponder) respondSuccess(msg *natsgo.Msg, pref *domain.PreferredEmail) {
	r.reply(msg, newPreferredEmailReply(pref))
}

// newPreferredEmailReply maps a preferred-email selection to a reply payload. A nil
// selection (or one without an EmailID) yields null email_id/email, meaning "use primary".
func newPreferredEmailReply(pref *domain.PreferredEmail) preferredEmailReply {
	var reply preferredEmailReply
	if pref != nil && pref.EmailID != "" {
		emailID := pref.EmailID
		email := pref.Email
		reply.EmailID = &emailID
		reply.Email = &email
	}
	return reply
}

// respondError logs and replies with an error envelope.
func (r *PreferredEmailResponder) respondError(msg *natsgo.Msg, user, op string, err error) {
	r.logger.With(logging.ErrKey, err).Warn("preferred_email request failed",
		"op", op,
		"user", redaction.Redact(user),
		"error_type", domain.GetErrorType(err),
	)
	r.reply(msg, errorReply{Error: err.Error()})
}

// reply marshals and sends a response, logging any transport failure.
func (r *PreferredEmailResponder) reply(msg *natsgo.Msg, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		r.logger.With(logging.ErrKey, err).Error("failed to marshal preferred_email reply")
		return
	}
	if err := msg.Respond(data); err != nil {
		r.logger.With(logging.ErrKey, err).Warn("failed to send preferred_email reply")
	}
}
