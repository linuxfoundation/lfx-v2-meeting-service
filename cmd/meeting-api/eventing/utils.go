// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"fmt"
	"strings"
	"time"
)

func shouldSkipSync(lastModifiedByID string) bool {
	return lastModifiedByID == "meeting-service" || lastModifiedByID == "lfx-v2-meeting-service"
}

func mapResponseType(responseType string) (string, error) {
	switch strings.ToUpper(responseType) {
	case "ACCEPTED":
		return "accepted", nil
	case "TENTATIVE":
		return "maybe", nil
	case "DECLINED":
		return "declined", nil
	}
	return "", fmt.Errorf("invalid response type: %s", responseType)
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
