// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package itx

// RegistrantType represents how a registrant was added to a meeting.
type RegistrantType string

const (
	RegistrantTypeDirect      RegistrantType = "direct"
	RegistrantTypeCommittee   RegistrantType = "committee"
	RegistrantTypeMailingList RegistrantType = "mailing_list"
	RegistrantTypeBulk        RegistrantType = "bulk_registrant"
)

// ZoomMeetingRegistrant represents a meeting registrant in ITX
type ZoomMeetingRegistrant struct {
	// Read-only fields
	ID   string         `json:"id,omitempty"`   // Registrant ID (read-only)
	Type RegistrantType `json:"type,omitempty"` // read-only

	// Identity fields
	CommitteeID string `json:"committee_id,omitempty"` // Committee ID (for committee registrants)
	UserID      string `json:"user_id,omitempty"`      // LF user ID
	Email       string `json:"email,omitempty"`        // Registrant email
	Username    string `json:"username,omitempty"`     // LF username

	// Personal info
	FirstName      string `json:"first_name,omitempty"`      // First name (required with email)
	LastName       string `json:"last_name,omitempty"`       // Last name (required with email)
	Org            string `json:"org,omitempty"`             // Organization
	JobTitle       string `json:"job_title,omitempty"`       // Job title
	ProfilePicture string `json:"profile_picture,omitempty"` // Profile picture URL

	// Meeting settings
	Host       bool   `json:"host,omitempty"`       // Access to host key
	Occurrence string `json:"occurrence,omitempty"` // Specific occurrence ID (blank = all occurrences)

	// Tracking fields (read-only)
	AttendedOccurrenceCount       int    `json:"attended_occurrence_count,omitempty"`        // Number of meetings attended
	TotalOccurrenceCount          int    `json:"total_occurrence_count,omitempty"`           // Total meetings registered
	LastInviteReceivedTime        string `json:"last_invite_received_time,omitempty"`        // Last invite timestamp (RFC3339)
	LastInviteReceivedMessageID   string `json:"last_invite_received_message_id,omitempty"`  // Last email message ID
	LastInviteDeliveryStatus      string `json:"last_invite_delivery_status,omitempty"`      // "delivered" or "failed"
	LastInviteDeliveryDescription string `json:"last_invite_delivery_description,omitempty"` // Delivery status details

	// Audit fields (read-only)
	CreatedAt  string `json:"created_at,omitempty"`  // Creation timestamp (RFC3339)
	CreatedBy  *User  `json:"created_by,omitempty"`  // Creator user info
	ModifiedAt string `json:"modified_at,omitempty"` // Last modified timestamp (RFC3339)
	UpdatedBy  *User  `json:"updated_by,omitempty"`  // Last updater user info
}

// RegistrantICS represents an ICS calendar file response from ITX
type RegistrantICS struct {
	Content []byte // ICS file content
}
