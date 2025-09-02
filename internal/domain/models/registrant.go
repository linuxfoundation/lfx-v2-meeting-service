// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package models

import (
	"fmt"
	"time"
)

// Registrant is the key-value store representation of a meeting registrant.
type Registrant struct {
	UID                string     `json:"uid"`
	MeetingUID         string     `json:"meeting_uid"`
	Email              string     `json:"email"`
	FirstName          string     `json:"first_name"`
	LastName           string     `json:"last_name"`
	Host               bool       `json:"host"`
	JobTitle           string     `json:"job_title,omitempty"`
	OccurrenceID       string     `json:"occurrence_id,omitempty"`
	OrgName            string     `json:"org_name,omitempty"`
	OrgIsMember        bool       `json:"org_is_member"`
	OrgIsProjectMember bool       `json:"org_is_project_member"`
	AvatarURL          string     `json:"avatar_url,omitempty"`
	Username           string     `json:"username,omitempty"`
	CreatedAt          *time.Time `json:"created_at,omitempty"`
	UpdatedAt          *time.Time `json:"updated_at,omitempty"`
}

// Tags generates a consistent set of tags for the registrant.
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
		// without prefix
		tags = append(tags, r.FirstName)
	}

	if r.LastName != "" {
		// without prefix
		tags = append(tags, r.LastName)
	}

	if r.Email != "" {
		// without prefix
		tags = append(tags, r.Email)
	}

	if r.Username != "" {
		// without prefix
		tags = append(tags, r.Username)
	}

	return tags
}
