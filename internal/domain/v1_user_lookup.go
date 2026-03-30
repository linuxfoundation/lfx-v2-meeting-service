// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package domain

import "context"

// V1UserLookup defines the interface for looking up v1 user data
type V1UserLookup interface {
	// LookupUser retrieves v1 user data by platform ID
	LookupUser(ctx context.Context, platformID string) (*V1User, error)
	// MapUsernameToAuthSub converts a v1 username to the Auth0 "sub" format expected by v2 services.
	MapUsernameToAuthSub(username string) string
}

// V1User represents user data from the v1 system
type V1User struct {
	Username  string
	Email     string
	FirstName string
	LastName  string
	AvatarURL string
	OrgName   string
}
