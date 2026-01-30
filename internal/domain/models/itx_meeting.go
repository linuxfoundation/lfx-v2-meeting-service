// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package models

// CreateITXMeetingRequest represents a domain request to create a meeting via ITX proxy
type CreateITXMeetingRequest struct {
	ProjectUID           string
	Title                string
	StartTime            string // RFC3339 format
	Duration             int
	Timezone             string
	Visibility           string
	Description          string
	Restricted           bool
	Committees           []Committee
	MeetingType          string
	EarlyJoinTimeMinutes int
	RecordingEnabled     bool
	TranscriptEnabled    bool
	YoutubeUploadEnabled bool
	ArtifactVisibility   string
	Recurrence           *ITXRecurrence
}

// ITXRecurrence represents recurrence for ITX requests (with string EndDateTime)
type ITXRecurrence struct {
	Type           int
	RepeatInterval int
	WeeklyDays     string
	MonthlyDay     int
	MonthlyWeek    int
	MonthlyWeekDay int
	EndTimes       int
	EndDateTime    string // RFC3339 format (different from domain Recurrence which uses *time.Time)
}
