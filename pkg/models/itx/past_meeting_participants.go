// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package itx

// InviteeResponse represents an invitee from ITX
type InviteeResponse struct {
	UUID                  string `json:"uuid"`                              // UUID of the invitee record
	FirstName             string `json:"first_name,omitempty"`              // First name
	LastName              string `json:"last_name,omitempty"`               // Last name
	PrimaryEmail          string `json:"primary_email,omitempty"`           // Primary email address
	LFSSO                 string `json:"lf_sso,omitempty"`                  // LF SSO username
	LFUserID              string `json:"lf_user_id,omitempty"`              // LF user ID
	Org                   string `json:"org,omitempty"`                     // Organization name
	JobTitle              string `json:"job_title,omitempty"`               // Job title
	ProfilePicture        string `json:"profile_picture,omitempty"`         // URL to profile picture
	CommitteeID           string `json:"committee_id,omitempty"`            // Associated committee UUID
	CommitteeRole         string `json:"committee_role,omitempty"`          // Role within committee
	IsCommitteeMember     bool   `json:"is_committee_member,omitempty"`     // Whether invitee is a committee member
	CommitteeVotingStatus string `json:"committee_voting_status,omitempty"` // Voting status in committee
	OrgIsMember           bool   `json:"org_is_member,omitempty"`           // Whether org has LF membership
	OrgIsProjectMember    bool   `json:"org_is_project_member,omitempty"`   // Whether org has project membership
	CreatedAt             string `json:"created_at,omitempty"`              // Creation timestamp (RFC3339)
	CreatedBy             *User  `json:"created_by,omitempty"`              // User who created the invitee
	ModifiedAt            string `json:"modified_at,omitempty"`             // Last modification timestamp (RFC3339)
	UpdatedBy             *User  `json:"updated_by,omitempty"`              // User who last updated the invitee
}

// CreateInviteeRequest represents the request to create an invitee
type CreateInviteeRequest struct {
	FirstName             string `json:"first_name,omitempty"`              // First name of the invitee
	LastName              string `json:"last_name,omitempty"`               // Last name of the invitee
	PrimaryEmail          string `json:"primary_email,omitempty"`           // Primary email address
	LFUserID              string `json:"lf_user_id,omitempty"`              // LF user ID
	LFSSO                 string `json:"lf_sso,omitempty"`                  // LF SSO username
	Org                   string `json:"org,omitempty"`                     // Organization name
	JobTitle              string `json:"job_title,omitempty"`               // Job title
	ProfilePicture        string `json:"profile_picture,omitempty"`         // URL to profile picture
	CommitteeID           string `json:"committee_id,omitempty"`            // UUID of associated committee (if applicable)
	CommitteeRole         string `json:"committee_role,omitempty"`          // Role within the committee
	CommitteeVotingStatus string `json:"committee_voting_status,omitempty"` // Voting status in committee
	OrgIsMember           bool   `json:"org_is_member,omitempty"`           // Whether org has LF membership
	OrgIsProjectMember    bool   `json:"org_is_project_member,omitempty"`   // Whether org has project membership

	// CreatedBy identifies the requesting user at creation time. Populated by the
	// service from the authenticated principal so ITX persists the correct audit
	// trail instead of leaving it blank / attributing it to the M2M service identity.
	CreatedBy *User `json:"created_by,omitempty"`
}

// UpdateInviteeRequest represents the request to update an invitee
type UpdateInviteeRequest struct {
	// Identity fields (used only for creating invitee during update, not sent to ITX update endpoint)
	PrimaryEmail string `json:"primary_email,omitempty"` // Primary email address (for creation only)
	LFUserID     string `json:"lf_user_id,omitempty"`    // LF user ID (for creation only)
	LFSSO        string `json:"lf_sso,omitempty"`        // LF SSO username (for creation only)

	// Updatable fields
	FirstName             string `json:"first_name"`                        // First name (required by ITX API)
	LastName              string `json:"last_name"`                         // Last name (required by ITX API)
	Org                   string `json:"org,omitempty"`                     // Organization name
	JobTitle              string `json:"job_title,omitempty"`               // Job title
	CommitteeRole         string `json:"committee_role,omitempty"`          // Role within the committee
	CommitteeVotingStatus string `json:"committee_voting_status,omitempty"` // Voting status in committee

	// UpdatedBy identifies the requesting user on update requests. Populated by the
	// service from the authenticated principal so ITX overwrites the stored value
	// instead of preserving stale data.
	UpdatedBy *User `json:"updated_by,omitempty"`
}

// AttendeeSession represents a join/leave session
type AttendeeSession struct {
	ParticipantUUID string `json:"participant_uuid,omitempty"` // Zoom participant UUID
	JoinTime        string `json:"join_time,omitempty"`        // When the participant joined (RFC3339)
	LeaveTime       string `json:"leave_time,omitempty"`       // When the participant left (RFC3339)
	LeaveReason     string `json:"leave_reason,omitempty"`     // Reason for leaving
}

// AttendeeResponse represents an attendee from ITX
type AttendeeResponse struct {
	ID                    string            `json:"id"`                                // UUID of the attendee record
	RegistrantID          string            `json:"registrant_id,omitempty"`           // UUID of associated registrant (if any)
	Name                  string            `json:"name,omitempty"`                    // Full name of the attendee
	Email                 string            `json:"email,omitempty"`                   // Email address
	LFSSO                 string            `json:"lf_sso,omitempty"`                  // LF SSO username
	LFUserID              string            `json:"lf_user_id,omitempty"`              // LF user ID
	IsVerified            bool              `json:"is_verified,omitempty"`             // Whether the attendee has been verified
	IsUnknown             bool              `json:"is_unknown,omitempty"`              // Whether attendee is marked as unknown
	IsAIReconciled        bool              `json:"is_ai_reconciled,omitempty"`        // Whether the attendee record was last updated via AI reconciliation
	IsAutoMatched         bool              `json:"is_auto_matched,omitempty"`         // Whether the attendee was automatically matched to an invitee by name
	ZoomUserName          string            `json:"zoom_user_name,omitempty"`          // Zoom display name of the attendee
	MappedInviteeName     string            `json:"mapped_invitee_name,omitempty"`     // Full name of the invitee the attendee was matched to
	Org                   string            `json:"org,omitempty"`                     // Organization name
	JobTitle              string            `json:"job_title,omitempty"`               // Job title
	ProfilePicture        string            `json:"profile_picture,omitempty"`         // URL to profile picture
	AverageAttendance     int               `json:"average_attendance,omitempty"`      // Average attendance percentage (calculated)
	MeetingID             string            `json:"meeting_id,omitempty"`              // Meeting ID
	OccurrenceID          string            `json:"occurrence_id,omitempty"`           // Occurrence ID
	CommitteeID           string            `json:"committee_id,omitempty"`            // Associated committee UUID
	CommitteeRole         string            `json:"committee_role,omitempty"`          // Role within committee
	IsCommitteeMember     bool              `json:"is_committee_member,omitempty"`     // Whether attendee is a committee member
	CommitteeVotingStatus string            `json:"committee_voting_status,omitempty"` // Voting status in committee
	OrgIsMember           bool              `json:"org_is_member,omitempty"`           // Whether org has LF membership
	OrgIsProjectMember    bool              `json:"org_is_project_member,omitempty"`   // Whether org has project membership
	Sessions              []AttendeeSession `json:"sessions,omitempty"`                // Array of session objects with join/leave times
}

// CreateAttendeeRequest represents the request to create an attendee
type CreateAttendeeRequest struct {
	Name                  string            `json:"name,omitempty"`                    // Full name of the attendee
	Email                 string            `json:"email,omitempty"`                   // Email address
	LFUserID              string            `json:"lf_user_id,omitempty"`              // LF user ID
	LFSSO                 string            `json:"lf_sso,omitempty"`                  // LF SSO username
	Org                   string            `json:"org,omitempty"`                     // Organization name
	JobTitle              string            `json:"job_title,omitempty"`               // Job title
	ProfilePicture        string            `json:"profile_picture,omitempty"`         // URL to profile picture
	IsVerified            bool              `json:"is_verified,omitempty"`             // Whether the attendee has been verified
	IsUnknown             bool              `json:"is_unknown,omitempty"`              // Whether attendee is marked as unknown
	IsAIReconciled        bool              `json:"is_ai_reconciled,omitempty"`        // Whether the attendee record was last updated via AI reconciliation
	IsAutoMatched         bool              `json:"is_auto_matched,omitempty"`         // Whether the attendee was automatically matched to an invitee by name
	ZoomUserName          string            `json:"zoom_user_name,omitempty"`          // Zoom display name of the attendee
	MappedInviteeName     string            `json:"mapped_invitee_name,omitempty"`     // Full name of the invitee the attendee was matched to
	CommitteeID           string            `json:"committee_id,omitempty"`            // UUID of associated committee (if applicable)
	CommitteeRole         string            `json:"committee_role,omitempty"`          // Role within the committee
	CommitteeVotingStatus string            `json:"committee_voting_status,omitempty"` // Voting status in committee
	OrgIsMember           bool              `json:"org_is_member,omitempty"`           // Whether org has LF membership
	OrgIsProjectMember    bool              `json:"org_is_project_member,omitempty"`   // Whether org has project membership
	Sessions              []AttendeeSession `json:"sessions,omitempty"`                // Array of session objects with join/leave times

	// CreatedBy identifies the requesting user at creation time. Populated by the
	// service from the authenticated principal so ITX persists the correct audit
	// trail instead of leaving it blank / attributing it to the M2M service identity.
	CreatedBy *User `json:"created_by,omitempty"`
}

// UpdateAttendeeRequest represents the request to update an attendee
type UpdateAttendeeRequest struct {
	Org                   string `json:"org,omitempty"`                     // Organization name
	JobTitle              string `json:"job_title,omitempty"`               // Job title
	IsVerified            bool   `json:"is_verified,omitempty"`             // Whether the attendee has been verified
	IsAIReconciled        bool   `json:"is_ai_reconciled,omitempty"`        // Whether the attendee record was last updated via AI reconciliation
	IsAutoMatched         bool   `json:"is_auto_matched,omitempty"`         // Whether the attendee was automatically matched to an invitee by name
	ZoomUserName          string `json:"zoom_user_name,omitempty"`          // Zoom display name of the attendee
	MappedInviteeName     string `json:"mapped_invitee_name,omitempty"`     // Full name of the invitee the attendee was matched to
	CommitteeRole         string `json:"committee_role,omitempty"`          // Role within the committee
	CommitteeVotingStatus string `json:"committee_voting_status,omitempty"` // Voting status in committee

	// UpdatedBy identifies the requesting user on update requests. Populated by the
	// service from the authenticated principal so ITX overwrites the stored value
	// instead of preserving stale data.
	UpdatedBy *User `json:"updated_by,omitempty"`
}
