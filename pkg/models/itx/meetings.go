// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package itx

// MeetingVisibility represents the visibility of a meeting.
type MeetingVisibility string

const (
	MeetingVisibilityPublic  MeetingVisibility = "public"
	MeetingVisibilityPrivate MeetingVisibility = "private"
)

// MeetingType represents the type/category of a meeting.
type MeetingType string

const (
	MeetingTypeBoard       MeetingType = "Board"
	MeetingTypeMaintainers MeetingType = "Maintainers"
	MeetingTypeMarketing   MeetingType = "Marketing"
	MeetingTypeTechnical   MeetingType = "Technical"
	MeetingTypeLegal       MeetingType = "Legal"
	MeetingTypeOther       MeetingType = "Other"
	MeetingTypeNone        MeetingType = "None"
)

// ArtifactAccess represents who can access meeting artifacts (recordings, transcripts, AI summaries).
type ArtifactAccess string

const (
	ArtifactAccessHosts        ArtifactAccess = "meeting_hosts"
	ArtifactAccessParticipants ArtifactAccess = "meeting_participants"
	ArtifactAccessPublic       ArtifactAccess = "public"
)

// CreateZoomMeetingRequest represents the request to create a Zoom meeting in ITX
type CreateZoomMeetingRequest struct {
	// ID is only used for updates - must match the ID in the URL path
	ID string `json:"id,omitempty"`

	// Core fields (required)
	Project    string            `json:"project"`    // LFX project ID
	Topic      string            `json:"topic"`      // Meeting title
	StartTime  string            `json:"start_time"` // RFC3339 format
	Duration   int               `json:"duration"`   // How many minutes the meeting is scheduled for - this is used for the pooled zoom users to organize meeting scheduling
	Visibility MeetingVisibility `json:"visibility"`

	// Optional core fields
	Timezone   string `json:"timezone"` // IANA timezone - will default to UTC if not provided
	Agenda     string `json:"agenda,omitempty"`
	Restricted bool   `json:"restricted"`

	// Committee integration
	Committee        string      `json:"committee,omitempty"` // deprecated
	Committees       []Committee `json:"committees,omitempty"`
	CommitteeFilters []string    `json:"committee_filters,omitempty"`

	// Meeting configuration
	MeetingType   MeetingType `json:"meeting_type,omitempty"`
	EarlyJoinTime int         `json:"early_join_time,omitempty"`

	// Recording settings
	RecordingEnabled     bool           `json:"recording_enabled"`
	TranscriptEnabled    bool           `json:"transcript_enabled"`
	RecordingAccess      ArtifactAccess `json:"recording_access,omitempty"`
	TranscriptAccess     ArtifactAccess `json:"transcript_access,omitempty"`
	YoutubeUploadEnabled bool           `json:"youtube_upload_enabled"`

	// AI features
	ZoomAIEnabled            bool           `json:"zoom_ai_enabled"`
	RequireAISummaryApproval bool           `json:"require_ai_summary_approval,omitempty"`
	AISummaryAccess          ArtifactAccess `json:"ai_summary_access,omitempty"`

	// Email reminders
	AutoEmailReminderEnabled bool `json:"auto_email_reminder_enabled,omitempty"`
	AutoEmailReminderTime    int  `json:"auto_email_reminder_time,omitempty"`

	// Advanced
	MailingListGroupIDs []string    `json:"mailing_list_group_ids,omitempty"`
	Recurrence          *Recurrence `json:"recurrence,omitempty"`
}

// CommitteeFilter represents the voting status filter for committee members.
type CommitteeFilter string

const (
	CommitteeFilterVotingRep    CommitteeFilter = "voting_rep"
	CommitteeFilterAltVotingRep CommitteeFilter = "alt_voting_rep"
	CommitteeFilterObserver     CommitteeFilter = "observer"
	CommitteeFilterEmeritus     CommitteeFilter = "emeritus"
)

// Committee represents a committee associated with a meeting
type Committee struct {
	ID            string            `json:"id"`
	Filters       []CommitteeFilter `json:"filters,omitempty"`
	VotingEnabled bool              `json:"voting_enabled,omitempty"`
}

// RecurrenceType represents the recurrence pattern of a meeting.
// ITX encodes this as an integer: 1=Daily, 2=Weekly, 3=Monthly.
type RecurrenceType int

const (
	RecurrenceTypeDaily   RecurrenceType = 1 // Repeat every N days
	RecurrenceTypeWeekly  RecurrenceType = 2 // Repeat every N weeks on specific days
	RecurrenceTypeMonthly RecurrenceType = 3 // Repeat every N months on a specific day or week
)

// Recurrence defines the recurrence pattern for recurring meetings
type Recurrence struct {
	Type           RecurrenceType `json:"type"`
	RepeatInterval int            `json:"repeat_interval"`            // Interval for recurrence
	WeeklyDays     string         `json:"weekly_days,omitempty"`      // Days of week for weekly meetings
	MonthlyDay     int            `json:"monthly_day,omitempty"`      // Day of month for monthly meetings
	MonthlyWeek    int            `json:"monthly_week,omitempty"`     // Week of month for monthly meetings
	MonthlyWeekDay int            `json:"monthly_week_day,omitempty"` // Day of week for monthly meetings
	EndTimes       int            `json:"end_times,omitempty"`        // Number of occurrences (1-50)
	EndDateTime    string         `json:"end_date_time,omitempty"`    // End date in RFC3339 format
}

// ZoomMeetingResponse represents the response from creating/retrieving a Zoom meeting
type ZoomMeetingResponse struct {
	// All request fields are included in the response
	Project    string            `json:"project"`
	Topic      string            `json:"topic"`
	StartTime  string            `json:"start_time"`
	Duration   int               `json:"duration"`
	Timezone   string            `json:"timezone"`
	Visibility MeetingVisibility `json:"visibility"`

	Agenda     string `json:"agenda,omitempty"`
	Restricted bool   `json:"restricted,omitempty"`

	Committee        string      `json:"committee,omitempty"`
	Committees       []Committee `json:"committees,omitempty"`
	CommitteeFilters []string    `json:"committee_filters,omitempty"`

	MeetingType   MeetingType `json:"meeting_type,omitempty"`
	EarlyJoinTime int         `json:"early_join_time,omitempty"`

	RecordingEnabled     bool           `json:"recording_enabled,omitempty"`
	TranscriptEnabled    bool           `json:"transcript_enabled,omitempty"`
	RecordingAccess      ArtifactAccess `json:"recording_access,omitempty"`
	TranscriptAccess     ArtifactAccess `json:"transcript_access,omitempty"`
	YoutubeUploadEnabled bool           `json:"youtube_upload_enabled,omitempty"`

	ZoomAIEnabled            bool           `json:"zoom_ai_enabled,omitempty"`
	RequireAISummaryApproval bool           `json:"require_ai_summary_approval,omitempty"`
	AISummaryAccess          ArtifactAccess `json:"ai_summary_access,omitempty"`

	AutoEmailReminderEnabled bool `json:"auto_email_reminder_enabled,omitempty"`
	AutoEmailReminderTime    int  `json:"auto_email_reminder_time,omitempty"`

	IsInviteResponsesEnabled bool `json:"is_invite_responses_enabled,omitempty"`
	ResponseCountYes         int  `json:"response_count_yes,omitempty"`
	ResponseCountMaybe       int  `json:"response_count_maybe,omitempty"`
	ResponseCountNo          int  `json:"response_count_no,omitempty"`

	LastBulkRegistrantJobStatus        string `json:"last_bulk_registrant_job_status,omitempty"`
	LastBulkRegistrantsJobWarningCount int    `json:"last_bulk_registrants_job_warning_count,omitempty"`

	LastMailingListMembersSyncJobStatus       string `json:"last_mailing_list_members_sync_job_status,omitempty"`
	LastMailingListMembersSyncJobFailedCount  int    `json:"last_mailing_list_members_sync_job_failed_count,omitempty"`
	LastMailingListMembersSyncJobWarningCount int    `json:"last_mailing_list_members_sync_job_warning_count,omitempty"`

	MailingListGroupIDs []string    `json:"mailing_list_group_ids,omitempty"`
	Recurrence          *Recurrence `json:"recurrence,omitempty"`

	// Read-only fields (set by ITX)
	ID                      string       `json:"id"`          // Zoom meeting ID
	HostKey                 string       `json:"host_key"`    // 6-digit PIN
	Passcode                string       `json:"passcode"`    // Zoom passcode
	Password                string       `json:"password"`    // UUID for join page
	PublicLink              string       `json:"public_link"` // Public meeting URL
	CreatedAt               string       `json:"created_at"`  // RFC3339
	ModifiedAt              string       `json:"modified_at"` // RFC3339
	CreatedBy               *User        `json:"created_by,omitempty"`
	UpdatedBy               *User        `json:"updated_by,omitempty"`
	Occurrences             []Occurrence `json:"occurrences,omitempty"`
	RegistrantCount         int          `json:"registrant_count,omitempty"`
	EmailDeliveryErrorCount int          `json:"email_delivery_error_count,omitempty"`
}

// OccurrenceStatus represents the status of a recurring meeting occurrence.
type OccurrenceStatus string

const (
	OccurrenceStatusAvailable OccurrenceStatus = "available"
	OccurrenceStatusCancel    OccurrenceStatus = "cancel"
)

// Occurrence represents a single occurrence of a recurring meeting
type Occurrence struct {
	OccurrenceID    string           `json:"occurrence_id"` // Unix timestamp
	StartTime       string           `json:"start_time"`    // RFC3339
	Duration        int              `json:"duration"`      // Minutes
	Status          OccurrenceStatus `json:"status"`
	RegistrantCount int              `json:"registrant_count,omitempty"`
	Topic           string           `json:"topic,omitempty"`
	Agenda          string           `json:"agenda,omitempty"`
}

// MeetingCountResponse represents the meeting count response from ITX
type MeetingCountResponse struct {
	MeetingCount int `json:"meeting_count"`
}

// GetJoinLinkRequest represents the request parameters for getting a join link
type GetJoinLinkRequest struct {
	MeetingID string
	UseEmail  bool
	UserID    string
	Name      string
	Email     string
	Register  bool
}

// ZoomMeetingJoinLink represents a join link response from ITX
type ZoomMeetingJoinLink struct {
	Link string `json:"link"` // Zoom meeting join URL
}

// UpdateOccurrenceRequest represents the request to update a meeting occurrence
type UpdateOccurrenceRequest struct {
	StartTime  string      `json:"start_time,omitempty"` // Meeting start time in RFC3339 format
	Duration   int         `json:"duration,omitempty"`   // Meeting duration in minutes
	Topic      string      `json:"topic,omitempty"`      // Meeting topic/title
	Agenda     string      `json:"agenda,omitempty"`     // Meeting agenda/description
	Recurrence *Recurrence `json:"recurrence,omitempty"` // Recurrence settings
	UpdatedBy  *User       `json:"updated_by,omitempty"` // User updating the occurrence (read-only, set by API)
}

// ResendMeetingInvitationsRequest represents the request to resend invitations to all registrants
type ResendMeetingInvitationsRequest struct {
	ExcludeRegistrantIDs []string `json:"exclude_registrant_ids,omitempty"` // Registrant IDs to exclude
}

// MeetingResponseRequest represents the request to submit a meeting response
type MeetingResponseRequest struct {
	Response     string `json:"response"`      // "accepted", "declined", or "maybe"
	Scope        string `json:"scope"`         // "single", "all", or "this_and_following"
	RegistrantID string `json:"registrant_id"` // UUID of the registrant
}

// MeetingResponseResult represents the result returned by ITX after submitting a meeting response
type MeetingResponseResult struct {
	ID           string `json:"id"`                      // Unique identifier for this response record
	MeetingID    string `json:"meeting_id"`              // The meeting ID this response belongs to
	RegistrantID string `json:"registrant_id"`           // The registrant ID that submitted the response
	Username     string `json:"username,omitempty"`      // Username of the registrant
	Email        string `json:"email,omitempty"`         // Email of the registrant
	Response     string `json:"response_value"`          // "accepted", "declined", or "maybe" (ITX field: response_value)
	Scope        string `json:"scope"`                   // "single", "all", or "this_and_following"
	OccurrenceID string `json:"occurrence_id,omitempty"` // Specific occurrence ID
	CreatedAt    string `json:"created_at,omitempty"`    // Creation timestamp (RFC3339)
	UpdatedAt    string `json:"updated_at,omitempty"`    // Last update timestamp (RFC3339)
}
