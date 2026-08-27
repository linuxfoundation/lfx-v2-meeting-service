// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

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

func parseTime(timeStr string) (time.Time, error) {
	if timeStr == "" {
		return time.Time{}, fmt.Errorf("empty time string")
	}

	// Try RFC3339 first
	t, err := time.Parse(time.RFC3339, timeStr)
	if err == nil {
		return t, nil
	}

	// Try ISO 8601
	t, err = time.Parse("2006-01-02T15:04:05Z", timeStr)
	if err == nil {
		return t, nil
	}

	// Try with milliseconds
	t, err = time.Parse("2006-01-02T15:04:05.000Z", timeStr)
	if err == nil {
		return t, nil
	}

	// Try space-separated format
	t, err = time.Parse("2006-01-02 15:04:05", timeStr)
	if err == nil {
		return t, nil
	}

	return time.Time{}, fmt.Errorf("unable to parse time: %s", timeStr)
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

// coerceInt decodes a JSON-decoded interface{} into *dest.
// Accepts string (numeric or empty), float64, int, and nil; returns an error for any other type.
func coerceInt(dest *int, v interface{}, field string) error {
	switch val := v.(type) {
	case string:
		if val == "" {
			return nil
		}
		n, err := strconv.Atoi(val)
		if err != nil {
			return fmt.Errorf("invalid value for %s: %w", field, err)
		}
		*dest = n
	case float64:
		*dest = int(val)
	case int:
		*dest = val
	case nil:
		// leave dest unchanged
	default:
		return fmt.Errorf("invalid type for %s: %T", field, v)
	}
	return nil
}

// coerceInt64 decodes a JSON-decoded interface{} into *dest.
// Accepts string (numeric or empty), float64, int64, int, and nil; returns an error for any other type.
func coerceInt64(dest *int64, v interface{}, field string) error {
	switch val := v.(type) {
	case string:
		if val == "" {
			return nil
		}
		n, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid value for %s: %w", field, err)
		}
		*dest = n
	case float64:
		*dest = int64(val)
	case int64:
		*dest = val
	case int:
		*dest = int64(val)
	case nil:
		// leave dest unchanged
	default:
		return fmt.Errorf("invalid type for %s: %T", field, v)
	}
	return nil
}
