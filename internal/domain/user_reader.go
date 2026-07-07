// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package domain

import "context"

// UserReader looks up LFID user data by email.
type UserReader interface {
	// UsernameByEmail returns the LFX username for the LFID account that owns the given
	// email address. Returns ErrUserNotFound when no account matches.
	// Returns a non-nil error (other than ErrUserNotFound) for transient failures.
	UsernameByEmail(ctx context.Context, email string) (string, error)
}
