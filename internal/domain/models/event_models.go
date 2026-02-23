// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package models

import "time"

// This file contains event data models for v1→v2 meeting event transformation.

// MeetingEventData represents a meeting event for indexing and access control
type MeetingEventData struct {
	ID                   string       `json:"id"`
	ProjectUID           string       `json:"project_uid"`
	Title                string       `json:"title"`
	Description          string       `json:"description"`
	StartTime            time.Time    `json:"start_time"`
	Duration             int          `json:"duration"`
	Timezone             string       `json:"timezone"`
	Visibility           string       `json:"visibility"`
	Restricted           bool         `json:"restricted"`
	MeetingType          string       `json:"meeting_type"`
	EarlyJoinTimeMinutes int          `json:"early_join_time_minutes"`
	RecordingEnabled     bool         `json:"recording_enabled"`
	TranscriptEnabled    bool         `json:"transcript_enabled"`
	YoutubeUploadEnabled bool         `json:"youtube_upload_enabled"`
	ArtifactVisibility   string       `json:"artifact_visibility"`
	Committees           []Committee  `json:"committees"`
	Occurrences          []Occurrence `json:"occurrences"`
	HostKey              string       `json:"host_key"`
	Passcode             string       `json:"passcode"`
	PublicLink           string       `json:"public_link"`
	CreatedAt            time.Time    `json:"created_at"`
	ModifiedAt           time.Time    `json:"modified_at"`
	Tags                 []string     `json:"tags,omitempty"`
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
	UID          string    `json:"uid"`
	MeetingID    string    `json:"meeting_id"`
	ProjectUID   string    `json:"project_uid"`
	CommitteeUID string    `json:"committee_uid,omitempty"`
	UserID       string    `json:"user_id"`
	Username     string    `json:"username,omitempty"`
	Email        string    `json:"email"`
	FirstName    string    `json:"first_name"`
	LastName     string    `json:"last_name"`
	AvatarURL    string    `json:"avatar_url,omitempty"`
	OrgName      string    `json:"org_name,omitempty"`
	Host         bool      `json:"host"`
	CreatedAt    time.Time `json:"created_at"`
	ModifiedAt   time.Time `json:"modified_at"`
	Tags         []string  `json:"tags,omitempty"`
}

// InviteResponseEventData represents an RSVP event for indexing
type InviteResponseEventData struct {
	ID           string    `json:"id"`
	MeetingID    string    `json:"meeting_id"`
	ProjectUID   string    `json:"project_uid"`
	UserID       string    `json:"user_id"`
	Email        string    `json:"email"`
	ResponseType string    `json:"response_type"` // accepted, declined, maybe
	Scope        string    `json:"scope"`         // all, single, this_and_following
	OccurrenceID string    `json:"occurrence_id,omitempty"`
	IsRecurring  bool      `json:"is_recurring"`
	CreatedAt    time.Time `json:"created_at"`
	ModifiedAt   time.Time `json:"modified_at"`
	Tags         []string  `json:"tags,omitempty"`
}

// PastMeetingEventData represents a past meeting event for indexing and access control
type PastMeetingEventData struct {
	ID               string      `json:"id"`         // UUID
	MeetingID        string      `json:"meeting_id"` // Original meeting ID
	ProjectUID       string      `json:"project_uid"`
	Title            string      `json:"title"`
	Description      string      `json:"description"`
	StartTime        time.Time   `json:"start_time"`
	EndTime          time.Time   `json:"end_time"`
	Duration         int         `json:"duration"` // Actual duration in minutes
	Timezone         string      `json:"timezone"`
	ParticipantCount int         `json:"participant_count"`
	Committees       []Committee `json:"committees"`
	HostKey          string      `json:"host_key"`
	CreatedAt        time.Time   `json:"created_at"`
	ModifiedAt       time.Time   `json:"modified_at"`
	Tags             []string    `json:"tags,omitempty"`
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
	ID                     string             `json:"id"`
	MeetingAndOccurrenceID string             `json:"meeting_and_occurrence_id"`
	ProjectUID             string             `json:"project_uid"`
	HostEmail              string             `json:"host_email"`
	HostID                 string             `json:"host_id"`
	MeetingID              string             `json:"meeting_id"`
	OccurrenceID           string             `json:"occurrence_id"`
	Platform               string             `json:"platform"` // Always "Zoom"
	PlatformMeetingID      string             `json:"platform_meeting_id"`
	RecordingAccess        string             `json:"recording_access"` // public, meeting_hosts, meeting_participants
	Title                  string             `json:"title"`
	TranscriptAccess       string             `json:"transcript_access,omitempty"`
	TranscriptEnabled      bool               `json:"transcript_enabled"`
	Visibility             string             `json:"visibility"`
	RecordingCount         int                `json:"recording_count"`
	RecordingFiles         []RecordingFile    `json:"recording_files"`
	Sessions               []RecordingSession `json:"sessions"`
	StartTime              time.Time          `json:"start_time"`
	TotalSize              int64              `json:"total_size"`
	CreatedAt              time.Time          `json:"created_at"`
	UpdatedAt              time.Time          `json:"updated_at"`
	Tags                   []string           `json:"tags,omitempty"`
}

// RecordingFile represents a single recording file
type RecordingFile struct {
	DownloadURL    string    `json:"download_url,omitempty"`
	FileExtension  string    `json:"file_extension"`
	FileSize       int64     `json:"file_size"`
	FileType       string    `json:"file_type"`
	ID             string    `json:"id"`
	MeetingID      string    `json:"meeting_id"`
	PlayURL        string    `json:"play_url,omitempty"`
	RecordingStart time.Time `json:"recording_start"`
	RecordingEnd   time.Time `json:"recording_end"`
	RecordingType  string    `json:"recording_type"`
	Status         string    `json:"status"`
}

// RecordingSession represents a recording session
type RecordingSession struct {
	UUID      string    `json:"uuid"`
	ShareURL  string    `json:"share_url,omitempty"`
	TotalSize int64     `json:"total_size"`
	StartTime time.Time `json:"start_time"`
}

// TranscriptEventData represents a transcript artifact event
type TranscriptEventData struct {
	ID                     string   `json:"id"`
	MeetingAndOccurrenceID string   `json:"meeting_and_occurrence_id"`
	ProjectUID             string   `json:"project_uid"`
	TranscriptAccess       string   `json:"transcript_access"` // public, meeting_hosts, meeting_participants
	Platform               string   `json:"platform"`          // Always "Zoom"
	Tags                   []string `json:"tags,omitempty"`
}

// SummaryEventData represents an AI-generated summary event
type SummaryEventData struct {
	ID                     string            `json:"id"`
	MeetingAndOccurrenceID string            `json:"meeting_and_occurrence_id"`
	ProjectUID             string            `json:"project_uid"`
	MeetingID              string            `json:"meeting_id"`
	OccurrenceID           string            `json:"occurrence_id"`
	ZoomMeetingUUID        string            `json:"zoom_meeting_uuid"`
	ZoomMeetingHostID      string            `json:"zoom_meeting_host_id"`
	ZoomMeetingHostEmail   string            `json:"zoom_meeting_host_email"`
	ZoomMeetingTopic       string            `json:"zoom_meeting_topic"`
	Content                string            `json:"content"`        // Consolidated markdown
	EditedContent          string            `json:"edited_content"` // Edited markdown
	RequiresApproval       bool              `json:"requires_approval"`
	Approved               bool              `json:"approved"`
	Platform               string            `json:"platform"` // Always "Zoom"
	ZoomConfig             SummaryZoomConfig `json:"zoom_config"`
	EmailSent              bool              `json:"email_sent"`
	CreatedAt              time.Time         `json:"created_at"`
	UpdatedAt              time.Time         `json:"updated_at"`
	Tags                   []string          `json:"tags,omitempty"`
}

// SummaryZoomConfig contains Zoom-specific configuration for summaries
type SummaryZoomConfig struct {
	MeetingID   string `json:"meeting_id"`
	MeetingUUID string `json:"meeting_uuid"`
}
