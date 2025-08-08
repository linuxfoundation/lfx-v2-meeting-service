// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package models

import (
	"time"
)

// Meeting is the key-value store representation of a meeting.
type Meeting struct {
	UID                             string       `json:"uid"`
	ProjectUID                      string       `json:"project_uid"`
	StartTime                       time.Time    `json:"start_time"`
	Duration                        int          `json:"duration"`
	Timezone                        string       `json:"timezone"`
	Recurrence                      *Recurrence  `json:"recurrence,omitempty"`
	Title                           string       `json:"title"`
	Description                     string       `json:"description"`
	Committees                      []Committee  `json:"committees,omitempty"`
	Platform                        string       `json:"platform"`
	EarlyJoinTimeMinutes            int          `json:"early_join_time_minutes,omitempty"`
	MeetingType                     string       `json:"meeting_type,omitempty"`
	Visibility                      string       `json:"visibility,omitempty"`
	Restricted                      bool         `json:"restricted"`
	ArtifactVisibility              string       `json:"artifact_visibility,omitempty"`
	PublicLink                      string       `json:"public_link,omitempty"`
	EmailDeliveryErrorCount         int          `json:"email_delivery_error_count,omitempty"`
	RecordingEnabled                bool         `json:"recording_enabled"`
	TranscriptEnabled               bool         `json:"transcript_enabled"`
	YoutubeUploadEnabled            bool         `json:"youtube_upload_enabled"`
	ZoomConfig                      *ZoomConfig  `json:"zoom_config,omitempty"`
	RegistrantCount                 int          `json:"registrant_count,omitempty"`
	RegistrantResponseDeclinedCount int          `json:"registrant_response_declined_count,omitempty"`
	RegistrantResponseAcceptedCount int          `json:"registrant_response_accepted_count,omitempty"`
	Occurrences                     []Occurrence `json:"occurrences,omitempty"`
	CreatedAt                       *time.Time   `json:"created_at,omitempty"`
	UpdatedAt                       *time.Time   `json:"updated_at,omitempty"`
}

// Committee represents a committee associated with a meeting
type Committee struct {
	UID                   string   `json:"uid"`
	AllowedVotingStatuses []string `json:"allowed_voting_statuses"`
}

// Recurrence represents the recurrence pattern of a meeting
type Recurrence struct {
	Type           int        `json:"type"`
	RepeatInterval int        `json:"repeat_interval"`
	WeeklyDays     string     `json:"weekly_days,omitempty"`
	MonthlyDay     int        `json:"monthly_day,omitempty"`
	MonthlyWeek    int        `json:"monthly_week,omitempty"`
	MonthlyWeekDay int        `json:"monthly_week_day,omitempty"`
	EndTimes       int        `json:"end_times,omitempty"`
	EndDateTime    *time.Time `json:"end_date_time,omitempty"`
}

// Occurrence represents a single occurrence of a recurring meeting
type Occurrence struct {
	OccurrenceID     string      `json:"occurrence_id"`
	StartTime        *time.Time  `json:"start_time"`
	Title            string      `json:"title,omitempty"`
	Description      string      `json:"description,omitempty"`
	Duration         int         `json:"duration,omitempty"`
	Recurrence       *Recurrence `json:"recurrence,omitempty"`
	RegistrantCount  int         `json:"registrant_count,omitempty"`
	ResponseCountNo  int         `json:"response_count_no,omitempty"`
	ResponseCountYes int         `json:"response_count_yes,omitempty"`
	Status           string      `json:"status,omitempty"`
}

// ZoomConfig represents Zoom-specific configuration for a meeting
type ZoomConfig struct {
	MeetingID                string `json:"meeting_id,omitempty"`
	AICompanionEnabled       bool   `json:"ai_companion_enabled"`
	AISummaryRequireApproval bool   `json:"ai_summary_require_approval"`
}
