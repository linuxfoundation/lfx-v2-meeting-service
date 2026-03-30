// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"crypto/sha512"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"

	"github.com/akamensky/base58"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
)

var (
	// safeNameRE detects usernames that are safe to use directly as Auth0 user IDs.
	safeNameRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,58}[A-Za-z0-9]$`)
	// hexUserRE detects hex strings that could collide with Auth0 native DB IDs.
	hexUserRE = regexp.MustCompile(`^[0-9a-f]{24,60}$`)
)

// NATSUserLookup implements the V1UserLookup interface using NATS KV bucket
type NATSUserLookup struct {
	v1ObjectsKV jetstream.KeyValue
	logger      *slog.Logger
}

// NewNATSUserLookup creates a new NATS-based v1 user lookup service
func NewNATSUserLookup(v1ObjectsKV jetstream.KeyValue, logger *slog.Logger) *NATSUserLookup {
	return &NATSUserLookup{
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
	if err := json.Unmarshal(entry.Value(), &userData); err != nil {
		l.logger.With(logging.ErrKey, err).ErrorContext(ctx, "failed to unmarshal v1 user data", "platform_id", platformID)
		return nil, fmt.Errorf("failed to unmarshal v1 user data: %w", err)
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

// MapUsernameToAuthSub converts a v1 username to the Auth0 "sub" format expected by v2 services.
//
// The mapping logic:
//   - Safe usernames (matching safeNameRE and not hexUserRE): use directly as userID
//   - Unsafe usernames: hash with SHA512 and encode to base58 (~80 chars) for legacy usernames
//     longer than 60 characters, with non-standard chars, or that might collide with future
//     24+ character Auth0 native DB hexadecimal hash
//
// Returns: "auth0|{userID}" format string
func (l *NATSUserLookup) MapUsernameToAuthSub(username string) string {
	return mapUsernameToAuthSub(username)
}

func mapUsernameToAuthSub(username string) string {
	if username == "" {
		return ""
	}

	var userID string
	if safeNameRE.MatchString(username) && !hexUserRE.MatchString(username) {
		userID = username
	} else {
		hash := sha512.Sum512([]byte(username))
		userID = base58.Encode(hash[:])
	}

	return "auth0|" + userID
}

// Ensure NATSUserLookup implements V1UserLookup
var _ domain.V1UserLookup = (*NATSUserLookup)(nil)
