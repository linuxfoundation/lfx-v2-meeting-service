// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package models

import "time"

// This file contains event data models for v1→v2 meeting event transformation.

// MeetingEventData represents a meeting event for indexing and access control
type MeetingEventData struct {
	ID                   string              `json:"id"`
	ProjectUID           string              `json:"project_uid"`
	Title                string              `json:"title"`
	Description          string              `json:"description"`
	StartTime            time.Time           `json:"start_time"`
	Duration             int                 `json:"duration"`
	Timezone             string              `json:"timezone"`
	Visibility           string              `json:"visibility"`
	Restricted           bool                `json:"restricted"`
	MeetingType          string              `json:"meeting_type"`
	EarlyJoinTimeMinutes int                 `json:"early_join_time_minutes"`
	RecordingEnabled     bool                `json:"recording_enabled"`
	TranscriptEnabled    bool                `json:"transcript_enabled"`
	YoutubeUploadEnabled bool                `json:"youtube_upload_enabled"`
	ArtifactVisibility   string              `json:"artifact_visibility"`
	Committees           []Committee         `json:"committees"`
	Occurrences          []Occurrence        `json:"occurrences"`
	HostKey              string              `json:"host_key"`
	Passcode             string              `json:"passcode"`
	PublicLink           string              `json:"public_link"`
	CreatedAt            time.Time           `json:"created_at"`
	ModifiedAt           time.Time           `json:"modified_at"`
	Tags                 []string            `json:"tags,omitempty"`
}

// Occurrence represents a single meeting occurrence
type Occurrence struct {
	OccurrenceID string    `json:"occurrence_id"`
	StartTime    time.Time `json:"start_time"`
	Duration     int       `json:"duration"`
	Status       string    `json:"status"`
}

// RegistrantEventData represents a registrant event for indexing and access control
type RegistrantEventData struct {
	UID         string    `json:"uid"`
	MeetingID   string    `json:"meeting_id"`
	ProjectUID  string    `json:"project_uid"`
	CommitteeUID string   `json:"committee_uid,omitempty"`
	UserID      string    `json:"user_id"`
	Username    string    `json:"username,omitempty"`
	Email       string    `json:"email"`
	FirstName   string    `json:"first_name"`
	LastName    string    `json:"last_name"`
	AvatarURL   string    `json:"avatar_url,omitempty"`
	OrgName     string    `json:"org_name,omitempty"`
	Host        bool      `json:"host"`
	CreatedAt   time.Time `json:"created_at"`
	ModifiedAt  time.Time `json:"modified_at"`
	Tags        []string  `json:"tags,omitempty"`
}

// InviteResponseEventData represents an RSVP event for indexing
type InviteResponseEventData struct {
	ID               string    `json:"id"`
	MeetingID        string    `json:"meeting_id"`
	ProjectUID       string    `json:"project_uid"`
	UserID           string    `json:"user_id"`
	Email            string    `json:"email"`
	ResponseType     string    `json:"response_type"` // accepted, declined, maybe
	Scope            string    `json:"scope"`         // all, single, this_and_following
	OccurrenceID     string    `json:"occurrence_id,omitempty"`
	IsRecurring      bool      `json:"is_recurring"`
	CreatedAt        time.Time `json:"created_at"`
	ModifiedAt       time.Time `json:"modified_at"`
	Tags             []string  `json:"tags,omitempty"`
}

// PastMeetingEventData represents a past meeting event for indexing and access control
type PastMeetingEventData struct {
	ID               string       `json:"id"`                // UUID
	MeetingID        string       `json:"meeting_id"`        // Original meeting ID
	ProjectUID       string       `json:"project_uid"`
	Title            string       `json:"title"`
	Description      string       `json:"description"`
	StartTime        time.Time    `json:"start_time"`
	EndTime          time.Time    `json:"end_time"`
	Duration         int          `json:"duration"` // Actual duration in minutes
	Timezone         string       `json:"timezone"`
	ParticipantCount int          `json:"participant_count"`
	Committees       []Committee  `json:"committees"`
	HostKey          string       `json:"host_key"`
	CreatedAt        time.Time    `json:"created_at"`
	ModifiedAt       time.Time    `json:"modified_at"`
	Tags             []string     `json:"tags,omitempty"`
}

// PastMeetingParticipantEventData represents a participant (invitee/attendee) event
type PastMeetingParticipantEventData struct {
	UID                    string               `json:"uid"`
	MeetingAndOccurrenceID string               `json:"meeting_and_occurrence_id"`
	MeetingID              string               `json:"meeting_id"`
	ProjectUID             string               `json:"project_uid"`
	Email                  string               `json:"email"`
	FirstName              string               `json:"first_name"`
	LastName               string               `json:"last_name"`
	Host                   bool                 `json:"host"`
	JobTitle               string               `json:"job_title,omitempty"`
	OrgName                string               `json:"org_name,omitempty"`
	OrgIsMember            bool                 `json:"org_is_member"`
	OrgIsProjectMember     bool                 `json:"org_is_project_member"`
	AvatarURL              string               `json:"avatar_url,omitempty"`
	Username               string               `json:"username,omitempty"`
	IsInvited              bool                 `json:"is_invited"`
	IsAttended             bool                 `json:"is_attended"`
	Sessions               []ParticipantSession `json:"sessions,omitempty"`
	CreatedAt              time.Time            `json:"created_at"`
	ModifiedAt             time.Time            `json:"modified_at"`
	Tags                   []string             `json:"tags,omitempty"`
}

// ParticipantSession represents a join/leave session for attendees
type ParticipantSession struct {
	UID         string     `json:"uid"`
	JoinTime    *time.Time `json:"join_time,omitempty"`
	LeaveTime   *time.Time `json:"leave_time,omitempty"`
	LeaveReason string     `json:"leave_reason,omitempty"`
}

// RecordingEventData represents a recording artifact event
type RecordingEventData struct {
	// Will be populated in recording handler phase
}

// TranscriptEventData represents a transcript artifact event
type TranscriptEventData struct {
	// Will be populated in recording handler phase
}

// SummaryEventData represents an AI-generated summary event
type SummaryEventData struct {
	// Will be populated in summary handler phase
}
