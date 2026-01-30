// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package itx

// CreateZoomMeetingRequest represents the request to create a Zoom meeting in ITX
type CreateZoomMeetingRequest struct {
	// Core fields (required)
	Project    string `json:"project"`              // LFX project ID
	Topic      string `json:"topic"`                // Meeting title
	StartTime  string `json:"start_time"`           // RFC3339 format
	Duration   int    `json:"duration"`             // Minutes
	Timezone   string `json:"timezone"`             // IANA timezone
	Visibility string `json:"visibility"`           // "public" or "private"

	// Optional core fields
	Agenda     string `json:"agenda,omitempty"`
	Restricted bool   `json:"restricted,omitempty"`

	// Committee integration
	Committee        string      `json:"committee,omitempty"`
	Committees       []Committee `json:"committees,omitempty"`
	CommitteeFilters []string    `json:"committee_filters,omitempty"`

	// Meeting configuration
	MeetingType   string `json:"meeting_type,omitempty"`    // Board, Maintainers, Marketing, Technical, Legal, Other, None
	EarlyJoinTime int    `json:"early_join_time,omitempty"` // 10-60 minutes

	// Recording settings
	RecordingEnabled     bool   `json:"recording_enabled,omitempty"`
	TranscriptEnabled    bool   `json:"transcript_enabled,omitempty"`
	RecordingAccess      string `json:"recording_access,omitempty"`  // meeting_hosts, meeting_participants, public
	TranscriptAccess     string `json:"transcript_access,omitempty"` // meeting_hosts, meeting_participants, public
	YoutubeUploadEnabled bool   `json:"youtube_upload_enabled,omitempty"`

	// AI features
	ZoomAIEnabled            bool   `json:"zoom_ai_enabled,omitempty"`
	RequireAISummaryApproval bool   `json:"require_ai_summary_approval,omitempty"`
	AISummaryAccess          string `json:"ai_summary_access,omitempty"` // meeting_hosts, meeting_participants, public

	// Email reminders
	AutoEmailReminderEnabled bool `json:"auto_email_reminder_enabled,omitempty"`
	AutoEmailReminderTime    int  `json:"auto_email_reminder_time,omitempty"` // 120-1440 minutes

	// Advanced
	MailingListGroupIDs []string     `json:"mailing_list_group_ids,omitempty"`
	Recurrence          *Recurrence  `json:"recurrence,omitempty"`
}

// Committee represents a committee associated with a meeting
type Committee struct {
	ID            string   `json:"id"`
	Filters       []string `json:"filters,omitempty"`        // voting_rep, alt_voting_rep, observer, emeritus
	VotingEnabled bool     `json:"voting_enabled,omitempty"`
}

// Recurrence defines the recurrence pattern for recurring meetings
type Recurrence struct {
	Type           int    `json:"type"`                       // 1=Daily, 2=Weekly, 3=Monthly
	RepeatInterval int    `json:"repeat_interval"`            // Interval for recurrence
	WeeklyDays     string `json:"weekly_days,omitempty"`      // Days of week for weekly meetings
	MonthlyDay     int    `json:"monthly_day,omitempty"`      // Day of month for monthly meetings
	MonthlyWeek    int    `json:"monthly_week,omitempty"`     // Week of month for monthly meetings
	MonthlyWeekDay int    `json:"monthly_week_day,omitempty"` // Day of week for monthly meetings
	EndTimes       int    `json:"end_times,omitempty"`        // Number of occurrences (1-50)
	EndDateTime    string `json:"end_date_time,omitempty"`    // End date in RFC3339 format
}

// ZoomMeetingResponse represents the response from creating/retrieving a Zoom meeting
type ZoomMeetingResponse struct {
	// All request fields are included in the response
	Project    string `json:"project"`
	Topic      string `json:"topic"`
	StartTime  string `json:"start_time"`
	Duration   int    `json:"duration"`
	Timezone   string `json:"timezone"`
	Visibility string `json:"visibility"`

	Agenda     string `json:"agenda,omitempty"`
	Restricted bool   `json:"restricted,omitempty"`

	Committee        string      `json:"committee,omitempty"`
	Committees       []Committee `json:"committees,omitempty"`
	CommitteeFilters []string    `json:"committee_filters,omitempty"`

	MeetingType   string `json:"meeting_type,omitempty"`
	EarlyJoinTime int    `json:"early_join_time,omitempty"`

	RecordingEnabled     bool   `json:"recording_enabled,omitempty"`
	TranscriptEnabled    bool   `json:"transcript_enabled,omitempty"`
	RecordingAccess      string `json:"recording_access,omitempty"`
	TranscriptAccess     string `json:"transcript_access,omitempty"`
	YoutubeUploadEnabled bool   `json:"youtube_upload_enabled,omitempty"`

	ZoomAIEnabled            bool   `json:"zoom_ai_enabled,omitempty"`
	RequireAISummaryApproval bool   `json:"require_ai_summary_approval,omitempty"`
	AISummaryAccess          string `json:"ai_summary_access,omitempty"`

	AutoEmailReminderEnabled bool `json:"auto_email_reminder_enabled,omitempty"`
	AutoEmailReminderTime    int  `json:"auto_email_reminder_time,omitempty"`

	MailingListGroupIDs []string    `json:"mailing_list_group_ids,omitempty"`
	Recurrence          *Recurrence `json:"recurrence,omitempty"`

	// Read-only fields (set by ITX)
	ID                      string       `json:"id"`                        // Zoom meeting ID
	HostKey                 string       `json:"host_key"`                  // 6-digit PIN
	Passcode                string       `json:"passcode"`                  // Zoom passcode
	Password                string       `json:"password"`                  // UUID for join page
	PublicLink              string       `json:"public_link"`               // Public meeting URL
	CreatedAt               string       `json:"created_at"`                // RFC3339
	ModifiedAt              string       `json:"modified_at"`               // RFC3339
	CreatedBy               *User        `json:"created_by,omitempty"`
	UpdatedBy               *User        `json:"updated_by,omitempty"`
	Occurrences             []Occurrence `json:"occurrences,omitempty"`
	RegistrantCount         int          `json:"registrant_count,omitempty"`
	EmailDeliveryErrorCount int          `json:"email_delivery_error_count,omitempty"`
}

// User represents a user in the ITX system
type User struct {
	ID             string `json:"id"`
	Username       string `json:"username"`
	Name           string `json:"name"`
	Email          string `json:"email"`
	ProfilePicture string `json:"profile_picture,omitempty"`
}

// Occurrence represents a single occurrence of a recurring meeting
type Occurrence struct {
	OccurrenceID    string `json:"occurrence_id"`              // Unix timestamp
	StartTime       string `json:"start_time"`                 // RFC3339
	Duration        int    `json:"duration"`                   // Minutes
	Status          string `json:"status"`                     // "available" or "cancel"
	RegistrantCount int    `json:"registrant_count,omitempty"`
	Topic           string `json:"topic,omitempty"`
	Agenda          string `json:"agenda,omitempty"`
}

// ErrorResponse represents an error response from ITX
type ErrorResponse struct {
	Message string `json:"message"`
	Error   string `json:"error"`
}
