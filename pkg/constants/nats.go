// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package constants

const (
	// AuthEmailToSubSubject resolves an Auth0 sub by primary email.
	// Request: plain-text email. Reply: plain-text sub on success, JSON error envelope on miss.
	AuthEmailToSubSubject = "lfx.auth-service.email_to_sub"
)
