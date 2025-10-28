// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package models

import (
	"fmt"
	"strings"
	"time"
)

// PastMeetingParticipant represents a participant's involvement in a past meeting
type PastMeetingParticipant struct {
	UID                string               `json:"uid"`
	PastMeetingUID     string               `json:"past_meeting_uid"`
	MeetingUID         string               `json:"meeting_uid"`
	Email              string               `json:"email"`
	FirstName          string               `json:"first_name"`
	LastName           string               `json:"last_name"`
	Host               bool                 `json:"host"`
	JobTitle           string               `json:"job_title,omitempty"`
	OrgName            string               `json:"org_name,omitempty"`
	OrgIsMember        bool                 `json:"org_is_member"`
	OrgIsProjectMember bool                 `json:"org_is_project_member"`
	AvatarURL          string               `json:"avatar_url,omitempty"`
	Username           string               `json:"username,omitempty"`
	IsInvited          bool                 `json:"is_invited"`
	IsAttended         bool                 `json:"is_attended"`
	Sessions           []ParticipantSession `json:"sessions,omitempty"`
	CreatedAt          *time.Time           `json:"created_at,omitempty"`
	UpdatedAt          *time.Time           `json:"updated_at,omitempty"`
}

// ParticipantSession represents a single join/leave session of a participant in a meeting
// Participants can have multiple sessions if they join and leave multiple times
type ParticipantSession struct {
	UID         string     `json:"uid"`
	JoinTime    time.Time  `json:"join_time"`
	LeaveTime   *time.Time `json:"leave_time,omitempty"`
	LeaveReason string     `json:"leave_reason,omitempty"`
}

// GetFullName returns the participant's full name by combining FirstName and LastName.
// The result is trimmed of leading/trailing whitespace.
func (p *PastMeetingParticipant) GetFullName() string {
	if p == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprintf("%s %s", p.FirstName, p.LastName))
}

// Tags generates a consistent set of tags for the past meeting participant.
// IMPORTANT: If you modify this method, please update the Meeting Tags documentation in the README.md
// to ensure consumers understand how to use these tags for searching.
func (p *PastMeetingParticipant) Tags() []string {
	tags := []string{}

	if p == nil {
		return nil
	}

	if p.UID != "" {
		// without prefix
		tags = append(tags, p.UID)
		// with prefix
		tag := fmt.Sprintf("past_meeting_participant_uid:%s", p.UID)
		tags = append(tags, tag)
	}

	if p.PastMeetingUID != "" {
		tag := fmt.Sprintf("past_meeting_uid:%s", p.PastMeetingUID)
		tags = append(tags, tag)
	}

	if p.MeetingUID != "" {
		tag := fmt.Sprintf("meeting_uid:%s", p.MeetingUID)
		tags = append(tags, tag)
	}

	if p.FirstName != "" {
		tag := fmt.Sprintf("first_name:%s", p.FirstName)
		tags = append(tags, tag)
	}

	if p.LastName != "" {
		tag := fmt.Sprintf("last_name:%s", p.LastName)
		tags = append(tags, tag)
	}

	if p.Username != "" {
		tag := fmt.Sprintf("username:%s", p.Username)
		tags = append(tags, tag)
	}

	if p.Email != "" {
		tag := fmt.Sprintf("email:%s", p.Email)
		tags = append(tags, tag)
	}

	return tags
}
