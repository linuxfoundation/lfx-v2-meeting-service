// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package domain

import "context"

// ProjectLookup provides access to project data via the projects API over NATS.
// See: https://github.com/linuxfoundation/lfx-v2-project-service#nats-message-handlers
type ProjectLookup interface {
	// GetProjectSlug returns the URL slug for the given project UID.
	// Returns an empty string (no error) when the project is not found.
	// Returns a non-nil error for transient failures (caller should retry).
	GetProjectSlug(ctx context.Context, projectUID string) (string, error)
}
