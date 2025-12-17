// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package models

import (
	"fmt"
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
	ShowMeetingAttendees bool        `json:"show_meeting_attendees"`
	ZoomConfig           *ZoomConfig `json:"zoom_config,omitempty"`
	Sessions             []Session   `json:"sessions,omitempty"`
	RecordingUIDs        []string    `json:"recording_uids,omitempty"`
	TranscriptUIDs       []string    `json:"transcript_uids,omitempty"`
	SummaryUIDs          []string    `json:"summary_uids,omitempty"`
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

func (p *PastMeeting) IsPublic() bool {
	return p != nil && p.Visibility == VisibilityPublic
}

// Tags generates a consistent set of tags for the past meeting.
// IMPORTANT: If you modify this method, please update the Meeting Tags documentation in the README.md
// to ensure consumers understand how to use these tags for searching.
func (p *PastMeeting) Tags() []string {
	tags := []string{}

	if p == nil {
		return nil
	}

	if p.UID != "" {
		// without prefix
		tags = append(tags, p.UID)
		// with prefix
		tag := fmt.Sprintf("past_meeting_uid:%s", p.UID)
		tags = append(tags, tag)
	}

	if p.MeetingUID != "" {
		tag := fmt.Sprintf("meeting_uid:%s", p.MeetingUID)
		tags = append(tags, tag)
	}

	if p.ProjectUID != "" {
		tag := fmt.Sprintf("project_uid:%s", p.ProjectUID)
		tags = append(tags, tag)
	}

	if p.OccurrenceID != "" {
		tag := fmt.Sprintf("occurrence_id:%s", p.OccurrenceID)
		tags = append(tags, tag)
	}

	for _, committee := range p.Committees {
		tag := fmt.Sprintf("committee_uid:%s", committee.UID)
		tags = append(tags, tag)
	}

	if p.Title != "" {
		tag := fmt.Sprintf("title:%s", p.Title)
		tags = append(tags, tag)
	}

	if p.Description != "" {
		tag := fmt.Sprintf("description:%s", p.Description)
		tags = append(tags, tag)
	}

	return tags
}
