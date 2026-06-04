// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	natsgo "github.com/nats-io/nats.go"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
)

const (
	authEmailToSubSubject = "lfx.auth-service.email_to_sub"
	userReaderTimeout     = 10 * time.Second
)

// NATSUserReader implements domain.UserReader using NATS request/reply to the auth service.
type NATSUserReader struct {
	nc *natsgo.Conn
}

// NewUserReader creates a new NATS-based user reader.
func NewUserReader(nc *natsgo.Conn) *NATSUserReader {
	return &NATSUserReader{nc: nc}
}

// SubByEmail returns the Auth0 "sub" for the LFID account that owns the given email address.
// Returns domain.ErrUserNotFound when the auth service reports no account matches.
// Returns a non-nil error for transient NATS or parsing failures.
func (r *NATSUserReader) SubByEmail(ctx context.Context, email string) (string, error) {
	if email == "" {
		return "", domain.ErrUserNotFound
	}

	reqCtx, cancel := context.WithTimeout(ctx, userReaderTimeout)
	defer cancel()

	msg, err := r.nc.RequestWithContext(reqCtx, authEmailToSubSubject, []byte(email))
	if err != nil {
		return "", fmt.Errorf("auth service email_to_sub request failed: %w", err)
	}

	sub := strings.TrimSpace(string(msg.Data))
	if sub == "" {
		return "", domain.ErrUserNotFound
	}

	// The auth service returns a plain sub string on success, or a JSON error object on failure.
	if sub[0] == '{' {
		var errResp struct {
			Success bool   `json:"success"`
			Error   string `json:"error"`
		}
		if jsonErr := json.Unmarshal(msg.Data, &errResp); jsonErr != nil {
			return "", fmt.Errorf("failed to parse auth service response: %w", jsonErr)
		}
		if !errResp.Success {
			// Treat a "not found" error as ErrUserNotFound; everything else as transient.
			lowerErr := strings.ToLower(errResp.Error)
			if strings.Contains(lowerErr, "not found") || strings.Contains(lowerErr, "no user") {
				return "", domain.ErrUserNotFound
			}
			return "", fmt.Errorf("auth service could not resolve email to sub: %s", errResp.Error)
		}
		// JSON parsed successfully and success=true; this shouldn't happen for a sub lookup —
		// treat it as an unexpected response rather than a valid LFID.
		return "", fmt.Errorf("auth service returned unexpected JSON response")
	}

	return sub, nil
}

// Ensure NATSUserReader implements domain.UserReader.
var _ domain.UserReader = (*NATSUserReader)(nil)
