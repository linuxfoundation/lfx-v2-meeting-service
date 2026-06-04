// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package domain

import (
	"context"
	"errors"
)

// ErrUserNotFound is returned by UserReader.SubByEmail when the email is not
// associated with any LFID account.
var ErrUserNotFound = errors.New("user not found")

// UserReader looks up LFID user data by email.
type UserReader interface {
	// SubByEmail returns the Auth0 "sub" identifier for the LFID account that owns
	// the given email address. Returns ErrUserNotFound when no account matches.
	// Returns a non-nil error (other than ErrUserNotFound) for transient failures.
	SubByEmail(ctx context.Context, email string) (string, error)
}
