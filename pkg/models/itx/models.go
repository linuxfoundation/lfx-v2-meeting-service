// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package itx

// CreateZoomMeetingRequest represents the request to create a Zoom meeting in ITX
type CreateZoomMeetingRequest struct {
	// ID is only used for updates - must match the ID in the URL path
	ID string `json:"id,omitempty"`

	// Core fields (required)
	Project    string `json:"project"`    // LFX project ID
	Topic      string `json:"topic"`      // Meeting title
	StartTime  string `json:"start_time"` // RFC3339 format
	Duration   int    `json:"duration"`   // Minutes
	Timezone   string `json:"timezone"`   // IANA timezone
	Visibility string `json:"visibility"` // "public" or "private"

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
	RecordingEnabled     bool   `json:"recording_enabled"`           // Required by ITX API
	TranscriptEnabled    bool   `json:"transcript_enabled"`          // Required by ITX API
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
	MailingListGroupIDs []string    `json:"mailing_list_group_ids,omitempty"`
	Recurrence          *Recurrence `json:"recurrence,omitempty"`
}

// Committee represents a committee associated with a meeting
type Committee struct {
	ID            string   `json:"id"`
	Filters       []string `json:"filters,omitempty"` // voting_rep, alt_voting_rep, observer, emeritus
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
	OccurrenceID    string `json:"occurrence_id"` // Unix timestamp
	StartTime       string `json:"start_time"`    // RFC3339
	Duration        int    `json:"duration"`      // Minutes
	Status          string `json:"status"`        // "available" or "cancel"
	RegistrantCount int    `json:"registrant_count,omitempty"`
	Topic           string `json:"topic,omitempty"`
	Agenda          string `json:"agenda,omitempty"`
}

// MeetingCountResponse represents the meeting count response from ITX
type MeetingCountResponse struct {
	MeetingCount int `json:"meeting_count"`
}

// ErrorResponse represents an error response from ITX
type ErrorResponse struct {
	Message string `json:"message"`
	Error   string `json:"error"`
}

// ZoomMeetingRegistrant represents a meeting registrant in ITX
type ZoomMeetingRegistrant struct {
	// Read-only fields
	ID   string `json:"id,omitempty"`   // Registrant ID (read-only)
	Type string `json:"type,omitempty"` // "direct" or "committee" (read-only)

	// Identity fields
	CommitteeID string `json:"committee_id,omitempty"` // Committee ID (for committee registrants)
	UserID      string `json:"user_id,omitempty"`      // LF user ID
	Email       string `json:"email,omitempty"`        // Registrant email
	Username    string `json:"username,omitempty"`     // LF username

	// Personal info
	FirstName      string `json:"first_name,omitempty"`      // First name (required with email)
	LastName       string `json:"last_name,omitempty"`       // Last name (required with email)
	Org            string `json:"org,omitempty"`             // Organization
	JobTitle       string `json:"job_title,omitempty"`       // Job title
	ProfilePicture string `json:"profile_picture,omitempty"` // Profile picture URL

	// Meeting settings
	Host       bool   `json:"host,omitempty"`       // Access to host key
	Occurrence string `json:"occurrence,omitempty"` // Specific occurrence ID (blank = all occurrences)

	// Tracking fields (read-only)
	AttendedOccurrenceCount       int    `json:"attended_occurrence_count,omitempty"`        // Number of meetings attended
	TotalOccurrenceCount          int    `json:"total_occurrence_count,omitempty"`           // Total meetings registered
	LastInviteReceivedTime        string `json:"last_invite_received_time,omitempty"`        // Last invite timestamp (RFC3339)
	LastInviteReceivedMessageID   string `json:"last_invite_received_message_id,omitempty"`  // Last email message ID
	LastInviteDeliveryStatus      string `json:"last_invite_delivery_status,omitempty"`      // "delivered" or "failed"
	LastInviteDeliveryDescription string `json:"last_invite_delivery_description,omitempty"` // Delivery status details

	// Audit fields (read-only)
	CreatedAt  string `json:"created_at,omitempty"`  // Creation timestamp (RFC3339)
	CreatedBy  *User  `json:"created_by,omitempty"`  // Creator user info
	ModifiedAt string `json:"modified_at,omitempty"` // Last modified timestamp (RFC3339)
	UpdatedBy  *User  `json:"updated_by,omitempty"`  // Last updater user info
}

// ZoomMeetingJoinLink represents a join link response from ITX
type ZoomMeetingJoinLink struct {
	Link string `json:"link"` // Zoom meeting join URL
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

// RegistrantICS represents an ICS calendar file response from ITX
type RegistrantICS struct {
	Content []byte // ICS file content
}

// ResendMeetingInvitationsRequest represents the request to resend invitations to all registrants
type ResendMeetingInvitationsRequest struct {
	ExcludeRegistrantIDs []string `json:"exclude_registrant_ids,omitempty"` // Registrant IDs to exclude
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

// CreatePastMeetingRequest represents the request to create a past meeting
type CreatePastMeetingRequest struct {
	// Required fields
	MeetingID    string `json:"meeting_id"`    // Zoom meeting ID
	OccurrenceID string `json:"occurrence_id"` // Zoom occurrence ID (Unix timestamp)
	ProjectID    string `json:"project_id"`    // LF project ID
	StartTime    string `json:"start_time"`    // Meeting start time in RFC3339 format
	Duration     int    `json:"duration"`      // Meeting duration in minutes
	Timezone     string `json:"timezone"`      // Meeting timezone

	// Optional fields
	Topic             string      `json:"topic,omitempty"`              // Meeting title/topic
	Agenda            string      `json:"agenda,omitempty"`             // Meeting description/agenda
	Restricted        bool        `json:"restricted,omitempty"`         // Whether meeting was restricted
	Committees        []Committee `json:"committees,omitempty"`         // Associated committees
	CommitteeID       string      `json:"committee_id,omitempty"`       // Single committee ID
	CommitteeFilters  []string    `json:"committee_filters,omitempty"`  // Committee member filters
	MeetingType       string      `json:"meeting_type,omitempty"`       // Meeting type
	RecordingEnabled  bool        `json:"recording_enabled,omitempty"`  // Was recording enabled
	RecordingAccess   string      `json:"recording_access,omitempty"`   // Who can access recordings
	TranscriptEnabled bool        `json:"transcript_enabled,omitempty"` // Was transcription enabled
	TranscriptAccess  string      `json:"transcript_access,omitempty"`  // Who can access transcripts
	Visibility        string      `json:"visibility,omitempty"`         // Meeting visibility (public/private)
}

// PastMeetingResponse represents the response from creating/retrieving a past meeting
type PastMeetingResponse struct {
	// Identifiers
	PastMeetingID string `json:"past_meeting_id"` // Past meeting ID (meeting_id or meeting_id-occurrence_id)
	MeetingID     string `json:"meeting_id"`      // Zoom meeting ID
	OccurrenceID  string `json:"occurrence_id"`   // Zoom occurrence ID
	ProjectID     string `json:"project_id"`      // LF project ID

	// Meeting details
	Topic      string `json:"topic,omitempty"`      // Meeting title
	Agenda     string `json:"agenda,omitempty"`     // Meeting description
	StartTime  string `json:"start_time"`           // Meeting start time (RFC3339)
	Duration   int    `json:"duration"`             // Meeting duration in minutes
	Timezone   string `json:"timezone"`             // Meeting timezone
	Visibility string `json:"visibility,omitempty"` // Meeting visibility
	Restricted bool   `json:"restricted"`           // Whether meeting was restricted

	// Committee association
	Committees       []Committee `json:"committees,omitempty"`        // Associated committees
	CommitteeID      string      `json:"committee_id,omitempty"`      // Single committee ID
	CommitteeFilters []string    `json:"committee_filters,omitempty"` // Committee filters

	// Meeting type
	MeetingType string `json:"meeting_type,omitempty"` // Type of meeting

	// Recording/Transcript settings
	RecordingEnabled  bool   `json:"recording_enabled"`           // Was recording enabled
	RecordingAccess   string `json:"recording_access,omitempty"`  // Who can access recordings
	TranscriptEnabled bool   `json:"transcript_enabled"`          // Was transcription enabled
	TranscriptAccess  string `json:"transcript_access,omitempty"` // Who can access transcripts

	// Metadata
	IsManuallyCreated bool `json:"is_manually_created,omitempty"` // Whether manually created
}

// PastMeetingSummaryResponse represents a past meeting summary from ITX
type PastMeetingSummaryResponse struct {
	// Identifiers
	ID                     string `json:"id"`                          // UUID of the summary
	MeetingAndOccurrenceID string `json:"meeting_and_occurrence_id"`   // Past meeting ID
	MeetingID              string `json:"meeting_id"`                  // Zoom meeting ID
	OccurrenceID           string `json:"occurrence_id"`               // Zoom occurrence ID
	ZoomMeetingUUID        string `json:"zoom_meeting_uuid,omitempty"` // Zoom meeting UUID

	// Summary metadata
	SummaryCreatedTime      string `json:"summary_created_time,omitempty"`       // When summary was created (RFC3339)
	SummaryLastModifiedTime string `json:"summary_last_modified_time,omitempty"` // When summary was last modified (RFC3339)
	SummaryStartTime        string `json:"summary_start_time,omitempty"`         // Summary start time (RFC3339)
	SummaryEndTime          string `json:"summary_end_time,omitempty"`           // Summary end time (RFC3339)

	// Original Zoom AI summary
	SummaryTitle    string                      `json:"summary_title,omitempty"`    // Title from Zoom
	SummaryOverview string                      `json:"summary_overview,omitempty"` // Overview from Zoom
	SummaryDetails  []ZoomMeetingSummaryDetails `json:"summary_details,omitempty"`  // Details from Zoom
	NextSteps       []string                    `json:"next_steps,omitempty"`       // Next steps from Zoom

	// Edited versions
	EditedSummaryOverview string                      `json:"edited_summary_overview,omitempty"` // Edited overview
	EditedSummaryDetails  []ZoomMeetingSummaryDetails `json:"edited_summary_details,omitempty"`  // Edited details
	EditedNextSteps       []string                    `json:"edited_next_steps,omitempty"`       // Edited next steps

	// Approval workflow
	RequiresApproval bool `json:"requires_approval,omitempty"` // Whether approval is required
	Approved         bool `json:"approved,omitempty"`          // Whether approved

	// Audit fields
	CreatedAt  string `json:"created_at,omitempty"`  // Creation timestamp (RFC3339)
	CreatedBy  *User  `json:"created_by,omitempty"`  // Creator user info
	ModifiedAt string `json:"modified_at,omitempty"` // Last modified timestamp (RFC3339)
	ModifiedBy *User  `json:"modified_by,omitempty"` // Last modifier user info
}

// ZoomMeetingSummaryDetails represents a section of the meeting summary
type ZoomMeetingSummaryDetails struct {
	Label   string `json:"label,omitempty"`   // Section label
	Summary string `json:"summary,omitempty"` // Section summary text
}

// UpdatePastMeetingSummaryRequest represents the request to update a past meeting summary
type UpdatePastMeetingSummaryRequest struct {
	EditedSummaryOverview string                      `json:"edited_summary_overview,omitempty"` // Edited overview
	EditedSummaryDetails  []ZoomMeetingSummaryDetails `json:"edited_summary_details,omitempty"`  // Edited details
	EditedNextSteps       []string                    `json:"edited_next_steps,omitempty"`       // Edited next steps
	Approved              *bool                       `json:"approved,omitempty"`                // Approval status
	ModifiedBy            *User                       `json:"modified_by,omitempty"`             // User making the update
}

// InviteeResponse represents an invitee from ITX
type InviteeResponse struct {
	UUID                  string `json:"uuid"`                              // UUID of the invitee record
	FirstName             string `json:"first_name,omitempty"`              // First name
	LastName              string `json:"last_name,omitempty"`               // Last name
	PrimaryEmail          string `json:"primary_email,omitempty"`           // Primary email address
	LFSSO                 string `json:"lf_sso,omitempty"`                  // LF SSO username
	LFUserID              string `json:"lf_user_id,omitempty"`              // LF user ID
	Org                   string `json:"org,omitempty"`                     // Organization name
	JobTitle              string `json:"job_title,omitempty"`               // Job title
	ProfilePicture        string `json:"profile_picture,omitempty"`         // URL to profile picture
	CommitteeID           string `json:"committee_id,omitempty"`            // Associated committee UUID
	CommitteeRole         string `json:"committee_role,omitempty"`          // Role within committee
	IsCommitteeMember     bool   `json:"is_committee_member,omitempty"`     // Whether invitee is a committee member
	CommitteeVotingStatus string `json:"committee_voting_status,omitempty"` // Voting status in committee
	OrgIsMember           bool   `json:"org_is_member,omitempty"`           // Whether org has LF membership
	OrgIsProjectMember    bool   `json:"org_is_project_member,omitempty"`   // Whether org has project membership
	CreatedAt             string `json:"created_at,omitempty"`              // Creation timestamp (RFC3339)
	CreatedBy             *User  `json:"created_by,omitempty"`              // User who created the invitee
	ModifiedAt            string `json:"modified_at,omitempty"`             // Last modification timestamp (RFC3339)
	UpdatedBy             *User  `json:"updated_by,omitempty"`              // User who last updated the invitee
}

// CreateInviteeRequest represents the request to create an invitee
type CreateInviteeRequest struct {
	FirstName             string `json:"first_name,omitempty"`              // First name of the invitee
	LastName              string `json:"last_name,omitempty"`               // Last name of the invitee
	PrimaryEmail          string `json:"primary_email,omitempty"`           // Primary email address
	LFUserID              string `json:"lf_user_id,omitempty"`              // LF user ID
	LFSSO                 string `json:"lf_sso,omitempty"`                  // LF SSO username
	Org                   string `json:"org,omitempty"`                     // Organization name
	JobTitle              string `json:"job_title,omitempty"`               // Job title
	ProfilePicture        string `json:"profile_picture,omitempty"`         // URL to profile picture
	CommitteeID           string `json:"committee_id,omitempty"`            // UUID of associated committee (if applicable)
	CommitteeRole         string `json:"committee_role,omitempty"`          // Role within the committee
	CommitteeVotingStatus string `json:"committee_voting_status,omitempty"` // Voting status in committee
	OrgIsMember           bool   `json:"org_is_member,omitempty"`           // Whether org has LF membership
	OrgIsProjectMember    bool   `json:"org_is_project_member,omitempty"`   // Whether org has project membership
}

// UpdateInviteeRequest represents the request to update an invitee
type UpdateInviteeRequest struct {
	// Identity fields (used only for creating invitee during update, not sent to ITX update endpoint)
	PrimaryEmail string `json:"primary_email,omitempty"` // Primary email address (for creation only)
	LFUserID     string `json:"lf_user_id,omitempty"`    // LF user ID (for creation only)
	LFSSO        string `json:"lf_sso,omitempty"`        // LF SSO username (for creation only)

	// Updatable fields
	FirstName             string `json:"first_name"`                        // First name (required by ITX API)
	LastName              string `json:"last_name"`                         // Last name (required by ITX API)
	Org                   string `json:"org,omitempty"`                     // Organization name
	JobTitle              string `json:"job_title,omitempty"`               // Job title
	CommitteeRole         string `json:"committee_role,omitempty"`          // Role within the committee
	CommitteeVotingStatus string `json:"committee_voting_status,omitempty"` // Voting status in committee
}

// AttendeeSession represents a join/leave session
type AttendeeSession struct {
	ParticipantUUID string `json:"participant_uuid,omitempty"` // Zoom participant UUID
	JoinTime        string `json:"join_time,omitempty"`        // When the participant joined (RFC3339)
	LeaveTime       string `json:"leave_time,omitempty"`       // When the participant left (RFC3339)
	LeaveReason     string `json:"leave_reason,omitempty"`     // Reason for leaving
}

// AttendeeResponse represents an attendee from ITX
type AttendeeResponse struct {
	ID                    string            `json:"id"`                                // UUID of the attendee record
	RegistrantID          string            `json:"registrant_id,omitempty"`           // UUID of associated registrant (if any)
	Name                  string            `json:"name,omitempty"`                    // Full name of the attendee
	Email                 string            `json:"email,omitempty"`                   // Email address
	LFSSO                 string            `json:"lf_sso,omitempty"`                  // LF SSO username
	LFUserID              string            `json:"lf_user_id,omitempty"`              // LF user ID
	IsVerified            bool              `json:"is_verified,omitempty"`             // Whether the attendee has been verified
	IsUnknown             bool              `json:"is_unknown,omitempty"`              // Whether attendee is marked as unknown
	Org                   string            `json:"org,omitempty"`                     // Organization name
	JobTitle              string            `json:"job_title,omitempty"`               // Job title
	ProfilePicture        string            `json:"profile_picture,omitempty"`         // URL to profile picture
	AverageAttendance     int               `json:"average_attendance,omitempty"`      // Average attendance percentage (calculated)
	MeetingID             string            `json:"meeting_id,omitempty"`              // Meeting ID
	OccurrenceID          string            `json:"occurrence_id,omitempty"`           // Occurrence ID
	CommitteeID           string            `json:"committee_id,omitempty"`            // Associated committee UUID
	CommitteeRole         string            `json:"committee_role,omitempty"`          // Role within committee
	IsCommitteeMember     bool              `json:"is_committee_member,omitempty"`     // Whether attendee is a committee member
	CommitteeVotingStatus string            `json:"committee_voting_status,omitempty"` // Voting status in committee
	OrgIsMember           bool              `json:"org_is_member,omitempty"`           // Whether org has LF membership
	OrgIsProjectMember    bool              `json:"org_is_project_member,omitempty"`   // Whether org has project membership
	Sessions              []AttendeeSession `json:"sessions,omitempty"`                // Array of session objects with join/leave times
}

// CreateAttendeeRequest represents the request to create an attendee
type CreateAttendeeRequest struct {
	Name                  string            `json:"name,omitempty"`                    // Full name of the attendee
	Email                 string            `json:"email,omitempty"`                   // Email address
	LFUserID              string            `json:"lf_user_id,omitempty"`              // LF user ID
	LFSSO                 string            `json:"lf_sso,omitempty"`                  // LF SSO username
	Org                   string            `json:"org,omitempty"`                     // Organization name
	JobTitle              string            `json:"job_title,omitempty"`               // Job title
	ProfilePicture        string            `json:"profile_picture,omitempty"`         // URL to profile picture
	IsVerified            bool              `json:"is_verified,omitempty"`             // Whether the attendee has been verified
	IsUnknown             bool              `json:"is_unknown,omitempty"`              // Whether attendee is marked as unknown
	CommitteeID           string            `json:"committee_id,omitempty"`            // UUID of associated committee (if applicable)
	CommitteeRole         string            `json:"committee_role,omitempty"`          // Role within the committee
	CommitteeVotingStatus string            `json:"committee_voting_status,omitempty"` // Voting status in committee
	OrgIsMember           bool              `json:"org_is_member,omitempty"`           // Whether org has LF membership
	OrgIsProjectMember    bool              `json:"org_is_project_member,omitempty"`   // Whether org has project membership
	Sessions              []AttendeeSession `json:"sessions,omitempty"`                // Array of session objects with join/leave times
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

// UpdateAttendeeRequest represents the request to update an attendee
type UpdateAttendeeRequest struct {
	Org                   string `json:"org,omitempty"`                     // Organization name
	JobTitle              string `json:"job_title,omitempty"`               // Job title
	IsVerified            bool   `json:"is_verified,omitempty"`             // Whether the attendee has been verified
	CommitteeRole         string `json:"committee_role,omitempty"`          // Role within the committee
	CommitteeVotingStatus string `json:"committee_voting_status,omitempty"` // Voting status in committee
}

// ============================================================================
// Attachment Models
// ============================================================================

// CreatedUpdatedBy represents user information for created_by and updated_by fields
type CreatedUpdatedBy struct {
	Username string `json:"username"`        // Username of the user
	Email    string `json:"email,omitempty"` // Email of the user
	Name     string `json:"name,omitempty"`  // Name of the user
}

// CreateAttachmentPresignRequest represents the request to generate a presigned URL for attachment upload
type CreateAttachmentPresignRequest struct {
	Name        string            `json:"name"`                  // File name (required)
	Description string            `json:"description,omitempty"` // Optional description
	Category    string            `json:"category,omitempty"`    // Category: "Meeting Minutes", "Notes", "Presentation", "Other"
	FileSize    int64             `json:"file_size"`             // File size in bytes (required)
	FileType    string            `json:"file_type"`             // MIME type (required)
	CreatedBy   *CreatedUpdatedBy `json:"created_by"`            // User who created the attachment (required)
}

// MeetingAttachmentPresignResponse represents the response from presigned URL generation for meeting attachments
type MeetingAttachmentPresignResponse struct {
	ID               string            `json:"id"`                          // Attachment ID (UUID)
	MeetingID        string            `json:"meeting_id"`                  // Zoom meeting ID
	Type             string            `json:"type"`                        // Attachment type ("file" or "link")
	Category         string            `json:"category"`                    // Attachment category
	Name             string            `json:"name"`                        // File name
	Description      string            `json:"description,omitempty"`       // Description
	FileName         string            `json:"file_name,omitempty"`         // File name
	FileSize         int64             `json:"file_size,omitempty"`         // File size in bytes
	FileURL          string            `json:"file_url,omitempty"`          // Presigned S3 PUT URL (valid for 60 minutes)
	FileUploadStatus string            `json:"file_upload_status"`          // Upload status: "ongoing", "completed", "failed"
	FileContentType  string            `json:"file_content_type,omitempty"` // MIME type
	CreatedAt        string            `json:"created_at,omitempty"`        // ISO 8601 timestamp
	CreatedBy        *CreatedUpdatedBy `json:"created_by"`                  // User who created the attachment
	UpdatedAt        string            `json:"updated_at,omitempty"`        // ISO 8601 timestamp
	UpdatedBy        *CreatedUpdatedBy `json:"updated_by,omitempty"`        // User who last updated the attachment
}

// PastMeetingAttachmentPresignResponse represents the response from presigned URL generation for past meeting attachments
type PastMeetingAttachmentPresignResponse struct {
	ID                     string            `json:"id"`                          // Attachment ID (UUID)
	MeetingAndOccurrenceID string            `json:"meeting_and_occurrence_id"`   // Meeting ID and occurrence timestamp
	MeetingID              string            `json:"meeting_id,omitempty"`        // Meeting ID
	Type                   string            `json:"type"`                        // Attachment type ("file" or "link")
	Category               string            `json:"category"`                    // Attachment category
	Name                   string            `json:"name"`                        // File name
	Description            string            `json:"description,omitempty"`       // Description
	FileName               string            `json:"file_name,omitempty"`         // File name
	FileSize               int64             `json:"file_size,omitempty"`         // File size in bytes
	FileURL                string            `json:"file_url,omitempty"`          // Presigned S3 PUT URL (valid for 60 minutes)
	FileUploadStatus       string            `json:"file_upload_status"`          // Upload status: "ongoing", "completed", "failed"
	FileContentType        string            `json:"file_content_type,omitempty"` // MIME type
	CreatedAt              string            `json:"created_at,omitempty"`        // ISO 8601 timestamp
	CreatedBy              *CreatedUpdatedBy `json:"created_by"`                  // User who created the attachment
	UpdatedAt              string            `json:"updated_at,omitempty"`        // ISO 8601 timestamp
	UpdatedBy              *CreatedUpdatedBy `json:"updated_by,omitempty"`        // User who last updated the attachment
}

// AttachmentDownloadResponse represents the presigned URL response for downloading attachments
type AttachmentDownloadResponse struct {
	DownloadURL string `json:"download_url"` // Presigned S3 URL for file download (valid for 60 minutes)
}

// MeetingAttachment represents a meeting attachment
type MeetingAttachment struct {
	ID               string            `json:"id"`                           // Attachment ID (UUID)
	MeetingID        string            `json:"meeting_id"`                   // Meeting ID
	Type             string            `json:"type"`                         // "file" or "link"
	Category         string            `json:"category"`                     // "Meeting Minutes", "Notes", "Presentation", "Other"
	Link             string            `json:"link,omitempty"`               // External link URL (only for link-type attachments)
	Name             string            `json:"name"`                         // Attachment name or file name
	Description      string            `json:"description,omitempty"`        // Optional description
	FileName         string            `json:"file_name,omitempty"`          // File name (only for file-type attachments)
	FileSize         int64             `json:"file_size,omitempty"`          // File size in bytes (only for file-type attachments)
	FileURL          string            `json:"file_url,omitempty"`           // S3 key path (only for file-type attachments)
	FileUploaded     bool              `json:"file_uploaded,omitempty"`      // Whether the file has been uploaded to S3 (omitted if false)
	FileUploadStatus string            `json:"file_upload_status,omitempty"` // "ongoing", "completed", "failed"
	FileContentType  string            `json:"file_content_type,omitempty"`  // MIME type of the file
	CreatedAt        string            `json:"created_at,omitempty"`         // ISO 8601 timestamp
	CreatedBy        *CreatedUpdatedBy `json:"created_by,omitempty"`         // User who created the attachment
	UpdatedAt        string            `json:"updated_at,omitempty"`         // ISO 8601 timestamp
	UpdatedBy        *CreatedUpdatedBy `json:"updated_by,omitempty"`         // User who last updated the attachment
	FileUploadedBy   *CreatedUpdatedBy `json:"file_uploaded_by,omitempty"`   // User who uploaded the file
	FileUploadedAt   string            `json:"file_uploaded_at,omitempty"`   // ISO 8601 timestamp when file was uploaded
}

// CreateMeetingAttachmentRequest represents the request to create a meeting attachment
type CreateMeetingAttachmentRequest struct {
	Type        string            `json:"type"`                  // Required: "file" or "link"
	Category    string            `json:"category"`              // Required: "Meeting Minutes", "Notes", "Presentation", "Other"
	Link        string            `json:"link,omitempty"`        // Required if type is "link"
	Name        string            `json:"name"`                  // Required: Attachment name
	Description string            `json:"description,omitempty"` // Optional description
	CreatedBy   *CreatedUpdatedBy `json:"created_by"`            // Required: User who created the attachment
}

// UpdateMeetingAttachmentRequest represents the request to update a meeting attachment
type UpdateMeetingAttachmentRequest struct {
	Type        string            `json:"type"`                  // Required: "file" or "link"
	Category    string            `json:"category"`              // Required: "Meeting Minutes", "Notes", "Presentation", "Other"
	Link        string            `json:"link,omitempty"`        // Required if type is "link"
	Name        string            `json:"name"`                  // Required: Attachment name
	Description string            `json:"description,omitempty"` // Optional description
	UpdatedBy   *CreatedUpdatedBy `json:"updated_by"`            // Required: User who updated the attachment
}

// PastMeetingAttachment represents a past meeting attachment
type PastMeetingAttachment struct {
	ID                     string            `json:"id"`                           // Attachment ID (UUID)
	MeetingAndOccurrenceID string            `json:"meeting_and_occurrence_id"`    // Past meeting and occurrence ID
	MeetingID              string            `json:"meeting_id"`                   // Meeting ID
	Type                   string            `json:"type"`                         // "file" or "link"
	Category               string            `json:"category"`                     // "Meeting Minutes", "Notes", "Presentation", "Other"
	Link                   string            `json:"link,omitempty"`               // External link URL (only for link-type attachments)
	Name                   string            `json:"name"`                         // Attachment name or file name
	Description            string            `json:"description,omitempty"`        // Optional description
	FileName               string            `json:"file_name,omitempty"`          // File name (only for file-type attachments)
	FileSize               int64             `json:"file_size,omitempty"`          // File size in bytes (only for file-type attachments)
	FileURL                string            `json:"file_url,omitempty"`           // S3 key path (only for file-type attachments)
	FileUploaded           bool              `json:"file_uploaded,omitempty"`      // Whether the file has been uploaded to S3 (omitted if false)
	FileUploadStatus       string            `json:"file_upload_status,omitempty"` // "ongoing", "completed", "failed"
	FileContentType        string            `json:"file_content_type,omitempty"`  // MIME type of the file
	CreatedAt              string            `json:"created_at,omitempty"`         // ISO 8601 timestamp
	CreatedBy              *CreatedUpdatedBy `json:"created_by,omitempty"`         // User who created the attachment
	UpdatedAt              string            `json:"updated_at,omitempty"`         // ISO 8601 timestamp
	UpdatedBy              *CreatedUpdatedBy `json:"updated_by,omitempty"`         // User who last updated the attachment
	FileUploadedBy         *CreatedUpdatedBy `json:"file_uploaded_by,omitempty"`   // User who uploaded the file
	FileUploadedAt         string            `json:"file_uploaded_at,omitempty"`   // ISO 8601 timestamp when file was uploaded
}

// CreatePastMeetingAttachmentRequest represents the request to create a past meeting attachment
type CreatePastMeetingAttachmentRequest struct {
	Type        string            `json:"type"`                  // Required: "file" or "link"
	Category    string            `json:"category"`              // Required: "Meeting Minutes", "Notes", "Presentation", "Other"
	Link        string            `json:"link,omitempty"`        // Required if type is "link"
	Name        string            `json:"name"`                  // Required: Attachment name
	Description string            `json:"description,omitempty"` // Optional description
	CreatedBy   *CreatedUpdatedBy `json:"created_by"`            // Required: User who created the attachment
}

// UpdatePastMeetingAttachmentRequest represents the request to update a past meeting attachment
type UpdatePastMeetingAttachmentRequest struct {
	Type        string            `json:"type"`                  // Required: "file" or "link"
	Category    string            `json:"category"`              // Required: "Meeting Minutes", "Notes", "Presentation", "Other"
	Link        string            `json:"link,omitempty"`        // Required if type is "link"
	Name        string            `json:"name"`                  // Required: Attachment name
	Description string            `json:"description,omitempty"` // Optional description
	UpdatedBy   *CreatedUpdatedBy `json:"updated_by"`            // Required: User who updated the attachment
}
