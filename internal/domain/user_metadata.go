// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package domain

import "context"

// UserProfile is a user's display identity as resolved from the auth service by username.
type UserProfile struct {
	// Username is the LFX/LFID username the profile was resolved for.
	Username string
	// Name is the user's display name (falls back to given+family name when unset).
	Name string
	// Email is the user's primary email address, when resolvable.
	Email string
	// AvatarURL is the user's profile picture URL, when set.
	AvatarURL string
}

// UserMetadataReader resolves a user's display profile (name, email, avatar) from their
// LFX username, without requiring the user's own bearer token. Used to stamp meeting
// creator identity (created_by) from the requesting principal on the v2 API, which only
// carries a Heimdall-minted token (not valid against the v1 API gateway).
type UserMetadataReader interface {
	// ResolveProfile resolves the given LFX username to a display profile via the auth
	// service. Returns a non-nil error for transient/lookup failures; callers should
	// degrade gracefully (e.g. fall back to username/email only) rather than fail the
	// caller's request.
	ResolveProfile(ctx context.Context, username string) (*UserProfile, error)
}
