// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package domain

import "context"

// ErrUserNotFound is returned when an email address has no associated LFID.
var ErrUserNotFound = NewNotFoundError("user not found by email")

// UserReader resolves Auth0 subs from email addresses via the auth service.
type UserReader interface {
	// SubByEmail returns the Auth0 sub for the given primary email address.
	// Returns ErrUserNotFound when no LFID is associated with the email.
	// Transport errors are returned unwrapped so callers can fail fast.
	SubByEmail(ctx context.Context, email string) (string, error)
}
