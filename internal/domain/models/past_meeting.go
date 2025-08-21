// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package models

import (
	"time"
)

// PastMeeting represents a historical record of a meeting occurrence that has taken place
// It captures the state of the meeting at the time it occurred
type PastMeeting struct {
	UID                  string      `json:"uid"`
	MeetingUID           string      `json:"meeting_uid"`
	OccurrenceID         string      `json:"occurrence_id,omitempty"`
	ProjectUID           string      `json:"project_uid"`
	ScheduledStartTime   time.Time   `json:"scheduled_start_time"`
	ScheduledEndTime     time.Time   `json:"scheduled_end_time"`
	Duration             int         `json:"duration"`
	Timezone             string      `json:"timezone"`
	Recurrence           *Recurrence `json:"recurrence,omitempty"`
	Title                string      `json:"title"`
	Description          string      `json:"description"`
	Committees           []Committee `json:"committees,omitempty"`
	Platform             string      `json:"platform"`
	PlatformMeetingID    string      `json:"platform_meeting_id,omitempty"`
	EarlyJoinTimeMinutes int         `json:"early_join_time_minutes,omitempty"`
	MeetingType          string      `json:"meeting_type,omitempty"`
	Visibility           string      `json:"visibility,omitempty"`
	Restricted           bool        `json:"restricted"`
	ArtifactVisibility   string      `json:"artifact_visibility,omitempty"`
	PublicLink           string      `json:"public_link,omitempty"`
	RecordingEnabled     bool        `json:"recording_enabled"`
	TranscriptEnabled    bool        `json:"transcript_enabled"`
	YoutubeUploadEnabled bool        `json:"youtube_upload_enabled"`
	ZoomConfig           *ZoomConfig `json:"zoom_config,omitempty"`
	Sessions             []Session   `json:"sessions,omitempty"`
	CreatedAt            *time.Time  `json:"created_at,omitempty"`
	UpdatedAt            *time.Time  `json:"updated_at,omitempty"`
}

// Session represents a single start/end session of a meeting on the platform
// Meetings can have multiple sessions if they are stopped and restarted
type Session struct {
	UID       string     `json:"uid"`
	StartTime time.Time  `json:"start_time"`
	EndTime   *time.Time `json:"end_time,omitempty"`
}
