// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package models

import (
	"time"
)

// RegistrantType represents the type of registrant
type RegistrantType string

const (
	// RegistrantTypeDirect represents a directly registered participant
	RegistrantTypeDirect RegistrantType = "direct"
	// RegistrantTypeCommittee represents a committee member registrant
	RegistrantTypeCommittee RegistrantType = "committee"
)

// Registrant is the key-value store representation of a meeting registrant.
type Registrant struct {
	UID                string         `json:"uid"`
	MeetingUID         string         `json:"meeting_uid"`
	Email              string         `json:"email"`
	FirstName          string         `json:"first_name"`
	LastName           string         `json:"last_name"`
	Host               bool           `json:"host"`
	Type               RegistrantType `json:"type"`
	CommitteeUID       *string        `json:"committee_uid,omitempty"`
	JobTitle           string         `json:"job_title,omitempty"`
	OccurrenceID       string         `json:"occurrence_id,omitempty"`
	OrgName            string         `json:"org_name,omitempty"`
	OrgIsMember        bool           `json:"org_is_member"`
	OrgIsProjectMember bool           `json:"org_is_project_member"`
	AvatarURL          string         `json:"avatar_url,omitempty"`
	Username           string         `json:"username,omitempty"`
	CreatedAt          *time.Time     `json:"created_at,omitempty"`
	UpdatedAt          *time.Time     `json:"updated_at,omitempty"`
}
