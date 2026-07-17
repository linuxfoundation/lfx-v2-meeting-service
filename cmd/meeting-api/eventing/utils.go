// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

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
		// leave as zero value
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
		// leave as zero value
	default:
		return fmt.Errorf("invalid type for %s: %T", field, v)
	}
	return nil
}
