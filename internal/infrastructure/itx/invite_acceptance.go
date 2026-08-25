// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package itx

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/redaction"
)

type acceptInviteRequestBody struct {
	Email    string `json:"email"`
	Username string `json:"username"`
}

// AcceptInvite calls the ITX Zoom Service to enrich all DynamoDB records (registrants,
// past-meeting invitees, past-meeting attendees) tied to email with the accepted username
// and profile data from the LFX user service.
func (c *Client) AcceptInvite(ctx context.Context, email, username string) error {
	slog.InfoContext(ctx, "ITX AcceptInvite request",
		"email", redaction.RedactEmail(email),
		"username", redaction.Redact(username))

	return c.doNoContent(ctx, apiRequest{
		method: http.MethodPost,
		path:   "/v2/zoom/meetings/invite_accepted",
		body:   acceptInviteRequestBody{Email: email, Username: username},
		accept: acceptJSON,
	})
}

var _ domain.InviteAcceptanceClient = (*Client)(nil)
