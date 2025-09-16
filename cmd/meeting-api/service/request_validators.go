// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"strconv"
	"strings"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
)

// EtagValidator validates ETag format and converts to uint64 for optimistic locking
// Supports standard HTTP ETag formats: "123", W/"123", and plain numeric "123"
func EtagValidator(etag *string) (uint64, error) {
	// Parse ETag to get revision for optimistic locking
	if etag == nil || *etag == "" {
		return 0, domain.NewValidationError("validation failed")
	}

	raw := strings.TrimSpace(*etag)

	// Handle weak ETags: W/"123" -> "123"
	if strings.HasPrefix(raw, "W/") || strings.HasPrefix(raw, "w/") {
		raw = strings.TrimSpace(raw[2:])
	}

	// Strip surrounding quotes if present: "123" -> 123
	raw = strings.Trim(raw, `"`)

	parsedRevision, errParse := strconv.ParseUint(raw, 10, 64)
	if errParse != nil {
		return 0, domain.NewValidationError("validation failed", errParse)
	}

	return parsedRevision, nil
}
