// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package constants

// ResourceTypeMeeting is the resource_type value used in invite payloads for meeting registrant invites.
const ResourceTypeMeeting = "meeting"

// InviteRoleRegistrant is the invite-service role for meeting registrants who do not yet have an LFID.
// This is meeting-specific and is not part of inviteapi.InviteRole (Manage/View/Member).
const InviteRoleRegistrant = "Registrant"
