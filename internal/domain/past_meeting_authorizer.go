// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package domain

import "context"

// PastMeetingAuthorizer checks whether a principal holds the organizer relation
// on a v1_past_meeting object in OpenFGA. GetPastMeetingSummary calls it for a
// summary awaiting approval (requires_approval && !approved): Heimdall only
// checks ai_summary_viewer on that route and cannot see the summary's approval
// state, so the organizer-only gate has to be enforced in the service.
//
// A nil PastMeetingAuthorizer means NATS was not configured at startup; callers
// must then deny reads of summaries awaiting approval rather than skip the check.
type PastMeetingAuthorizer interface {
	// IsOrganizer reports whether principal (the JWT username) has the
	// "organizer" relation on v1_past_meeting:{pastMeetingID} in OpenFGA.
	// A false result with a nil error means access is explicitly denied.
	// An error means the check could not be completed.
	IsOrganizer(ctx context.Context, principal, pastMeetingID string) (bool, error)
}
