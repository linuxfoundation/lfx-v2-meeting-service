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

// Self is the calling user's identity as resolved from their bearer token via the
// v1 user-service /v1/me endpoint.
type Self struct {
	// SFID is the user's Salesforce ID.
	SFID string
	// Emails is the user's email records (used to resolve an address to its EmailID).
	Emails []SelfEmail
}

// SelfEmail is a single email record on the user's account.
type SelfEmail struct {
	ID       string
	Address  string
	Active   bool
	Verified bool
}

// UserServiceClient reads a user's identity and manages their preferred meeting-invite
// email via the v1 user-service preferences API, acting AS the user via their bearer token.
//
// Phase 1 storage lives in the v1 user-service (identical to the legacy myprofile path)
// so itx-service-zoom keeps reading the preference unchanged through cutover. Calls use
// the user's token (forwarded by self-serve), so they run with the user's own identity
// and authorization.
type UserServiceClient interface {
	// GetSelf resolves the calling user (SFID + email records) from their bearer token.
	GetSelf(ctx context.Context, token string) (*Self, error)

	// GetMeetingEmailPreference returns the user's Type=Meeting email preference.
	// Returns nil when no override is set (use primary).
	GetMeetingEmailPreference(ctx context.Context, token, sfid string) (*PreferredEmail, error)

	// SetMeetingEmailPreference upserts the user's Type=Meeting email preference to
	// the given verified-email record ID (EmailID) and returns the resulting selection.
	SetMeetingEmailPreference(ctx context.Context, token, sfid, emailID string) (*PreferredEmail, error)

	// ClearMeetingEmailPreference removes the user's Type=Meeting email preference so
	// their primary email is used. It is a no-op when no preference exists.
	ClearMeetingEmailPreference(ctx context.Context, token, sfid string) error
}
