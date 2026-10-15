// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/infrastructure/v1objects"
	"github.com/nats-io/nats.go/jetstream"
)

// decodeV1 converts a map[string]interface{} v1 KV payload into a typed struct T
// by routing it through JSON: marshal the map then unmarshal into T. This is the
// canonical decode pattern for v1 bucket data throughout the eventing package.
func decodeV1[T any](v1Data map[string]interface{}) (T, error) {
	var zero T
	jsonBytes, err := json.Marshal(v1Data)
	if err != nil {
		return zero, fmt.Errorf("failed to marshal v1Data to JSON: %w", err)
	}
	var result T
	if err := json.Unmarshal(jsonBytes, &result); err != nil {
		return zero, fmt.Errorf("failed to unmarshal v1Data: %w", err)
	}
	return result, nil
}

func shouldSkipSync(lastModifiedByID string) bool {
	return lastModifiedByID == "meeting-service" || lastModifiedByID == "lfx-v2-meeting-service"
}

// parseTime parses the timestamp layouts found in v1 records. The implementation lives in
// internal/infrastructure/v1objects so readers outside this package decode timestamps the
// same way.
func parseTime(timeStr string) (time.Time, error) {
	return v1objects.ParseTime(timeStr)
}

func extractIDFromKey(key, prefix string) string {
	if len(key) > len(prefix) {
		return key[len(prefix):]
	}
	return key
}

// isKVAbsenceError reports whether err means the key does not (and cannot) exist
// in the KV store. Both ErrKeyNotFound (key was never written or was deleted) and
// ErrInvalidKey (the key string contains characters that NATS rejects, e.g. spaces
// in an lf_sso value) are treated as absence: the xref will never be found, so
// callers should proceed as if the sibling does not exist rather than retrying.
func isKVAbsenceError(err error) bool {
	return errors.Is(err, jetstream.ErrKeyNotFound) || errors.Is(err, jetstream.ErrInvalidKey)
}

func isTransientError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, errOccurrenceExpansionTimeout) {
		return true
	}
	errStr := err.Error()
	return strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "connection") ||
		strings.Contains(errStr, "unavailable") ||
		strings.Contains(errStr, "temporary") ||
		strings.Contains(errStr, "transient")
}

func parseName(fullName string) (firstName, lastName string) {
	if fullName == "" {
		return "", ""
	}

	parts := strings.Fields(fullName)
	if len(parts) == 0 {
		return "", ""
	}
	if len(parts) == 1 {
		return parts[0], ""
	}
	// First part is first name, everything else is last name
	return parts[0], strings.Join(parts[1:], " ")
}

// coerceInt and coerceInt64 decode a JSON-decoded interface{} into *dest. The implementations
// live in internal/infrastructure/v1objects (see parseTime).
func coerceInt(dest *int, v interface{}, field string) error {
	return v1objects.CoerceInt(dest, v, field)
}

func coerceInt64(dest *int64, v interface{}, field string) error {
	return v1objects.CoerceInt64(dest, v, field)
}
