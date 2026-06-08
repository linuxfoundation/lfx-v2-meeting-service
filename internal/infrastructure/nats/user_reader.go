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

const userReaderTimeout = 10 * time.Second

// NATSUserReader implements domain.UserReader using NATS request/reply to the auth service.
type NATSUserReader struct {
	nc     Requester
	logger *slog.Logger
}

// NewUserReader creates a new NATS-based user reader.
func NewUserReader(nc Requester, logger *slog.Logger) *NATSUserReader {
	logger.Info("user reader initialized", "subject", constants.AuthEmailToSubSubject)
	return &NATSUserReader{nc: nc, logger: logger}
}

// SubByEmail returns the Auth0 "sub" for the LFID account that owns the given email address.
// Returns domain.ErrUserNotFound when the auth service reports no account matches.
// Returns a non-nil error for transient NATS or parsing failures.
func (r *NATSUserReader) SubByEmail(ctx context.Context, email string) (string, error) {
	email = strings.TrimSpace(email)
	if email == "" {
		return "", domain.ErrUserNotFound
	}

	reqCtx, cancel := context.WithTimeout(ctx, userReaderTimeout)
	defer cancel()

	msg, err := r.nc.RequestWithContext(reqCtx, constants.AuthEmailToSubSubject, []byte(email))
	if err != nil {
		return "", fmt.Errorf("email_to_sub request failed: %w", err)
	}

	// The auth service sends a plain-text subject on success and a JSON error envelope on miss.
	body := strings.TrimSpace(string(msg.Data))
	if body == "" {
		return "", domain.ErrUserNotFound
	}

	if body[0] == '{' {
		var envelope struct {
			Success  *bool  `json:"success"`
			Error    string `json:"error,omitempty"`
			Sub      string `json:"sub,omitempty"`
			Username string `json:"username,omitempty"`
		}
		if err := json.Unmarshal(msg.Data, &envelope); err != nil {
			return "", fmt.Errorf("failed to parse email_to_sub response: %w", err)
		}
		if envelope.Success == nil {
			return "", fmt.Errorf("email_to_sub response missing success field")
		}
		if !*envelope.Success {
			return "", domain.ErrUserNotFound
		}
		sub := strings.TrimSpace(envelope.Sub)
		if sub == "" {
			sub = strings.TrimSpace(envelope.Username)
		}
		if sub == "" {
			return "", fmt.Errorf("email_to_sub success envelope missing subject")
		}
		return sub, nil
	}

	return body, nil
}

// Ensure NATSUserReader implements domain.UserReader.
var _ domain.UserReader = (*NATSUserReader)(nil)
