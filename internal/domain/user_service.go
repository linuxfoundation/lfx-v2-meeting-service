// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package domain

import "context"

// PreferredEmail is a user's preferred meeting-invite email selection.
//
// A nil *PreferredEmail (or a zero value) means no override is set and the user's
// primary email should be used.
type PreferredEmail struct {
	// PreferenceID is the v1 user-service email-preference record UUID (Type=Meeting).
	// Empty when no preference record exists.
	PreferenceID string
	// EmailID is the Salesforce ID of the verified-email record the user selected.
	EmailID string
	// Email is the address of the selected verified email.
	Email string
}

// UserServiceClient resolves a user's Salesforce ID and manages their preferred
// meeting-invite email via the v1 user-service preferences API.
//
// Phase 1 storage lives in the v1 user-service (identical to the legacy myprofile
// path) so itx-service-zoom keeps reading the preference unchanged through cutover.
type UserServiceClient interface {
	// ResolveSFIDByUsername returns the Salesforce ID for the given LFID/username.
	// Returns ErrUserNotFound when no user matches.
	ResolveSFIDByUsername(ctx context.Context, username string) (string, error)

	// ResolveEmailID returns the Salesforce ID of the user's email record matching the
	// given address. The email must be an active, verified record on the account (invites
	// must only route to a verified address): a matching-but-unverified address is a
	// ValidationError, while an unknown address returns a retryable UnavailableError (SFDC
	// email records sync from auth0 asynchronously). The method never creates records.
	ResolveEmailID(ctx context.Context, sfid, email string) (string, error)

	// GetMeetingEmailPreference returns the user's Type=Meeting email preference.
	// Returns nil when no override is set (use primary).
	GetMeetingEmailPreference(ctx context.Context, sfid string) (*PreferredEmail, error)

	// SetMeetingEmailPreference upserts the user's Type=Meeting email preference to
	// the given verified-email record ID (EmailID) and returns the resulting selection.
	SetMeetingEmailPreference(ctx context.Context, sfid, emailID string) (*PreferredEmail, error)

	// ClearMeetingEmailPreference removes the user's Type=Meeting email preference so
	// their primary email is used. It is a no-op when no preference exists.
	ClearMeetingEmailPreference(ctx context.Context, sfid string) error
}
