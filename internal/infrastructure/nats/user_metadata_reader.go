// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/constants"
)

// userMetadataReaderTimeout is intentionally short: profile enrichment is best-effort and
// must never hold up meeting creation waiting on a slow or unresponsive auth-service responder.
const userMetadataReaderTimeout = 2 * time.Second

// NATSUserMetadataReader implements domain.UserMetadataReader using NATS request/reply to
// the auth service, resolving a username to a display profile without a user bearer token.
type NATSUserMetadataReader struct {
	nc     Requester
	logger *slog.Logger
}

// NewUserMetadataReader creates a new NATS-based user metadata reader.
func NewUserMetadataReader(nc Requester, logger *slog.Logger) *NATSUserMetadataReader {
	logger.Info("user metadata reader initialized",
		"metadata_subject", constants.AuthUserMetadataSubject,
		"emails_subject", constants.AuthUserEmailsSubject,
	)
	return &NATSUserMetadataReader{nc: nc, logger: logger}
}

// userMetadataResponse is the auth service reply envelope for user_metadata.read.
type userMetadataResponse struct {
	Success *bool  `json:"success"`
	Error   string `json:"error,omitempty"`
	Data    struct {
		Name       string `json:"name"`
		GivenName  string `json:"given_name"`
		FamilyName string `json:"family_name"`
		Picture    string `json:"picture"`
	} `json:"data"`
}

// userEmailsRequest is the auth service request body for user_emails.read.
type userEmailsRequest struct {
	User struct {
		AuthToken string `json:"auth_token"`
	} `json:"user"`
}

// userEmailsResponse is the auth service reply envelope for user_emails.read.
type userEmailsResponse struct {
	Success *bool  `json:"success"`
	Error   string `json:"error,omitempty"`
	Data    struct {
		PrimaryEmail string `json:"primary_email"`
	} `json:"data"`
}

// ResolveProfile resolves the given LFX username to a display profile via the auth
// service. The name/avatar come from user_metadata.read (required); the email comes from
// user_emails.read (best-effort — a failure there is logged and leaves Email empty rather
// than failing the whole resolution).
func (r *NATSUserMetadataReader) ResolveProfile(ctx context.Context, username string) (*domain.UserProfile, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return nil, domain.NewValidationError("username is required to resolve a user profile")
	}

	reqCtx, cancel := context.WithTimeout(ctx, userMetadataReaderTimeout)
	defer cancel()

	msg, err := r.nc.RequestWithContext(reqCtx, constants.AuthUserMetadataSubject, []byte(username))
	if err != nil {
		return nil, fmt.Errorf("user_metadata request failed: %w", err)
	}

	var meta userMetadataResponse
	if err := json.Unmarshal(msg.Data, &meta); err != nil {
		return nil, fmt.Errorf("failed to parse user_metadata response: %w", err)
	}
	if meta.Success == nil {
		return nil, fmt.Errorf("user_metadata response missing success field")
	}
	if !*meta.Success {
		return nil, domain.ErrUserNotFound
	}

	name := meta.Data.Name
	if name == "" {
		name = strings.TrimSpace(meta.Data.GivenName + " " + meta.Data.FamilyName)
	}

	profile := &domain.UserProfile{
		Username:  username,
		Name:      name,
		AvatarURL: meta.Data.Picture,
	}

	email, err := r.resolveEmail(reqCtx, username)
	if err != nil {
		r.logger.WarnContext(ctx, "failed to resolve email for user profile; continuing without it",
			"username", username, "err", err)
	} else {
		profile.Email = email
	}

	return profile, nil
}

// resolveEmail looks up the user's primary email address by username via user_emails.read.
func (r *NATSUserMetadataReader) resolveEmail(ctx context.Context, username string) (string, error) {
	body := userEmailsRequest{}
	body.User.AuthToken = username
	payload, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("failed to marshal user_emails request: %w", err)
	}

	msg, err := r.nc.RequestWithContext(ctx, constants.AuthUserEmailsSubject, payload)
	if err != nil {
		return "", fmt.Errorf("user_emails request failed: %w", err)
	}

	var emails userEmailsResponse
	if err := json.Unmarshal(msg.Data, &emails); err != nil {
		return "", fmt.Errorf("failed to parse user_emails response: %w", err)
	}
	if emails.Success == nil {
		return "", fmt.Errorf("user_emails response missing success field")
	}
	if !*emails.Success {
		return "", domain.ErrUserNotFound
	}

	return emails.Data.PrimaryEmail, nil
}

// Ensure NATSUserMetadataReader implements domain.UserMetadataReader.
var _ domain.UserMetadataReader = (*NATSUserMetadataReader)(nil)
