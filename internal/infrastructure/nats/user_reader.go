// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	natsgo "github.com/nats-io/nats.go"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/constants"
)

type userReader struct {
	nc *natsgo.Conn
}

// errorEnvelope matches the JSON error envelope returned by the auth service on miss.
type errorEnvelope struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
}

// SubByEmail resolves the Auth0 sub for the given primary email address.
// The auth service replies with a plain-text sub on success, or a JSON error envelope on miss.
func (u *userReader) SubByEmail(ctx context.Context, email string) (string, error) {
	if u.nc == nil {
		return "", domain.NewUnavailableError("user reader is not configured")
	}
	reply, err := u.nc.RequestMsgWithContext(ctx, &natsgo.Msg{
		Subject: constants.AuthEmailToSubSubject,
		Data:    []byte(email),
	})
	if err != nil {
		return "", fmt.Errorf("email_to_sub request failed: %w", err)
	}

	body := strings.TrimSpace(string(reply.Data))
	if body == "" {
		return "", domain.ErrUserNotFound
	}

	// Any object-shaped response (starts with '{') is a JSON error envelope.
	if body[0] == '{' {
		var env errorEnvelope
		if jsonErr := json.Unmarshal([]byte(body), &env); jsonErr == nil && !env.Success {
			return "", domain.ErrUserNotFound
		}
		return "", domain.ErrUserNotFound
	}

	return body, nil
}

// NewUserReader creates a NATS-backed UserReader.
func NewUserReader(nc *natsgo.Conn) domain.UserReader {
	return &userReader{nc: nc}
}
