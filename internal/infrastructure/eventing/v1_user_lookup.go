// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	nats "github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/vmihailenco/msgpack/v5"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
)

const authServiceUsernameToSubSubject = "lfx.auth-service.username_to_sub"

// NATSUserLookup implements the V1UserLookup interface using NATS KV bucket
type NATSUserLookup struct {
	nc          *nats.Conn
	v1ObjectsKV jetstream.KeyValue
	logger      *slog.Logger
}

// NewNATSUserLookup creates a new NATS-based v1 user lookup service
func NewNATSUserLookup(nc *nats.Conn, v1ObjectsKV jetstream.KeyValue, logger *slog.Logger) *NATSUserLookup {
	return &NATSUserLookup{
		nc:          nc,
		v1ObjectsKV: v1ObjectsKV,
		logger:      logger,
	}
}

// LookupUser retrieves v1 user data by platform ID from the v1-objects KV bucket
func (l *NATSUserLookup) LookupUser(ctx context.Context, platformID string) (*domain.V1User, error) {
	key := fmt.Sprintf("user.%s", platformID)

	entry, err := l.v1ObjectsKV.Get(ctx, key)
	if err != nil {
		l.logger.With(logging.ErrKey, err).WarnContext(ctx, "failed to lookup v1 user", "platform_id", platformID)
		return nil, fmt.Errorf("failed to lookup v1 user: %w", err)
	}

	var userData map[string]interface{}
	if jsonErr := json.Unmarshal(entry.Value(), &userData); jsonErr != nil {
		if err := msgpack.Unmarshal(entry.Value(), &userData); err != nil {
			l.logger.With(logging.ErrKey, jsonErr).ErrorContext(ctx, "failed to decode v1 user data", "platform_id", platformID)
			return nil, domain.NewInternalError("failed to decode v1 user data", jsonErr)
		}
	}

	user := &domain.V1User{
		Username:  getString(userData, "lf_sso"),
		Email:     getString(userData, "lf_email"),
		FirstName: getString(userData, "first_name"),
		LastName:  getString(userData, "last_name"),
		AvatarURL: getString(userData, "profile_picture"),
		OrgName:   getString(userData, "org"),
	}

	l.logger.InfoContext(ctx, "successfully looked up v1 user", "platform_id", platformID, "username", user.Username)
	return user, nil
}

// getString safely extracts a string value from a map
func getString(data map[string]interface{}, key string) string {
	if val, ok := data[key]; ok {
		if strVal, ok := val.(string); ok {
			return strVal
		}
	}
	return ""
}

// MapUsernameToAuthSub converts a v1 username to the Auth0 "sub" format by calling the
// auth service over NATS on subject lfx.auth-service.username_to_sub.
func (l *NATSUserLookup) MapUsernameToAuthSub(ctx context.Context, username string) (string, error) {
	return lookupUsernameToAuthSub(ctx, l.nc, username, l.logger)
}

// lookupUsernameToAuthSub calls the auth service over NATS to convert a v1 username
// to the Auth0 "sub" format expected by v2 services.
func lookupUsernameToAuthSub(ctx context.Context, nc *nats.Conn, username string, logger *slog.Logger) (string, error) {
	if username == "" {
		return "", nil
	}
	msg, err := nc.RequestWithContext(ctx, authServiceUsernameToSubSubject, []byte(username))
	if err != nil {
		return "", fmt.Errorf("auth service username lookup failed: %w", err)
	}
	sub := string(msg.Data)
	if sub == "" {
		return "", fmt.Errorf("auth service returned empty sub for username %q", username)
	}
	// The auth service returns a plain sub string on success, or a JSON error object on failure.
	// Detect the error case so we don't forward the raw JSON as an FGA user identifier.
	if sub[0] == '{' {
		var errResp struct {
			Success bool   `json:"success"`
			Error   string `json:"error"`
		}
		if jsonErr := json.Unmarshal(msg.Data, &errResp); jsonErr == nil && !errResp.Success {
			return "", fmt.Errorf("auth service could not resolve username %q to auth sub: %s", username, errResp.Error)
		}
	}
	return sub, nil
}

// Ensure NATSUserLookup implements V1UserLookup
var _ domain.V1UserLookup = (*NATSUserLookup)(nil)
