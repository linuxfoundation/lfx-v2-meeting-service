// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/vmihailenco/msgpack/v5"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/msgpackutil"
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
	if jsonErr := json.Unmarshal(entry.Value(), &userData); jsonErr != nil {
		// Guard against deeply-nested msgpack payloads before calling Unmarshal.
		// msgpack.Unmarshal recurses once per nesting level with no built-in depth
		// cap; a crafted payload can exhaust the goroutine stack and kill the
		// process.  msgpackutil.CheckNestingDepth scans iteratively and rejects
		// structurally malformed or excessively-nested payloads.
		if depthErr := msgpackutil.CheckNestingDepth(entry.Value()); depthErr != nil {
			l.logger.With(logging.ErrKey, depthErr).ErrorContext(ctx, "msgpack depth guard rejected v1 user record", "platform_id", platformID)
			return nil, fmt.Errorf("structural decode failure for v1 user record: %w", depthErr)
		}
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

// Ensure NATSUserLookup implements V1UserLookup
var _ domain.V1UserLookup = (*NATSUserLookup)(nil)
