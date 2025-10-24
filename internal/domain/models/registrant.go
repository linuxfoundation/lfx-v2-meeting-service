// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package models

import (
	"fmt"
	"strings"
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

// GetFullName returns the registrant's full name by combining FirstName and LastName.
// The result is trimmed of leading/trailing whitespace.
func (r *Registrant) GetFullName() string {
	if r == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprintf("%s %s", r.FirstName, r.LastName))
}

// Tags generates a consistent set of tags for the registrant.
// IMPORTANT: If you modify this method, please update the Meeting Tags documentation in the README.md
// to ensure consumers understand how to use these tags for searching.
func (r *Registrant) Tags() []string {
	tags := []string{}

	if r == nil {
		return nil
	}

	if r.UID != "" {
		// without prefix
		tags = append(tags, r.UID)
		// with prefix
		tag := fmt.Sprintf("registrant_uid:%s", r.UID)
		tags = append(tags, tag)
	}

	if r.MeetingUID != "" {
		tag := fmt.Sprintf("meeting_uid:%s", r.MeetingUID)
		tags = append(tags, tag)
	}

	if r.FirstName != "" {
		tag := fmt.Sprintf("first_name:%s", r.FirstName)
		tags = append(tags, tag)
	}

	if r.LastName != "" {
		tag := fmt.Sprintf("last_name:%s", r.LastName)
		tags = append(tags, tag)
	}

	if r.Email != "" {
		tag := fmt.Sprintf("email:%s", r.Email)
		tags = append(tags, tag)
	}

	if r.Username != "" {
		tag := fmt.Sprintf("username:%s", r.Username)
		tags = append(tags, tag)
	}

	return tags
}
