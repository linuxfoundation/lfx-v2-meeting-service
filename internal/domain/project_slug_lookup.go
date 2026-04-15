// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package domain

import "context"

// ProjectSlugLookup resolves a v2 project UID to its URL slug via the projects API.
type ProjectSlugLookup interface {
	// GetProjectSlug returns the slug for the given project UID.
	// Returns an empty string (no error) when the project is not found.
	// Returns a non-nil error for transient failures (caller should retry).
	GetProjectSlug(ctx context.Context, projectUID string) (string, error)
}
