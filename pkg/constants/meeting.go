// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package constants

// Meeting time constraints
const (
	// MaxEarlyJoinTimeMinutes is the maximum number of minutes users can join a meeting early
	MaxEarlyJoinTimeMinutes = 60

	// MaxMeetingDurationMinutes is the maximum duration of a meeting in minutes
	MaxMeetingDurationMinutes = 600
)

// ResourceTypeMeeting is the resource_type value used in invite payloads for meeting registrant invites.
const ResourceTypeMeeting = "meeting"

// InviteRoleRegistrant is the invite-service role for meeting registrants who do not yet have an LFID.
// This is meeting-specific and is not part of inviteapi.InviteRole (Manage/View/Member).
const InviteRoleRegistrant = "Registrant"
