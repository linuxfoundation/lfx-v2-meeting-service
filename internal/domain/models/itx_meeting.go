// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package models

import itx "github.com/linuxfoundation/lfx-v2-meeting-service/pkg/models/itx"

// Committee represents a committee associated with a meeting
type Committee struct {
	UID                   string                `json:"uid"`
	AllowedVotingStatuses []itx.CommitteeFilter `json:"allowed_voting_statuses,omitempty"`
}

type UpdatePastMeetingParticipant struct {
	PastMeetingID string
	ParticipantID string
	InviteeID     string
	AttendeeID    string
	IsInvited     *bool
	IsAttended    *bool
}

// CreateITXMeetingRequest represents a domain request to create a meeting via ITX proxy
type CreateITXMeetingRequest struct {
	ID                       string // Meeting ID (only used for updates - must match URL path)
	ProjectUID               string
	Title                    string
	StartTime                string // RFC3339 format
	Duration                 int
	Timezone                 string
	Visibility               itx.MeetingVisibility
	Description              string
	Restricted               bool
	Committees               []Committee
	MeetingType              itx.MeetingType
	EarlyJoinTimeMinutes     int
	RecordingEnabled         bool
	TranscriptEnabled        bool
	YoutubeUploadEnabled     bool
	AISummaryEnabled         bool
	RequireAISummaryApproval bool
	ArtifactVisibility       itx.ArtifactAccess
	Recurrence               *ITXRecurrence
}

// ITXRecurrence represents recurrence for ITX requests (with string EndDateTime)
type ITXRecurrence struct {
	Type           itx.RecurrenceType
	RepeatInterval int
	WeeklyDays     string
	MonthlyDay     int
	MonthlyWeek    int
	MonthlyWeekDay int
	EndTimes       int
	EndDateTime    string // RFC3339 format (different from domain Recurrence which uses *time.Time)
}
