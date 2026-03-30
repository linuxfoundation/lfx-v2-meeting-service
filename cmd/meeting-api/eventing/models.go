// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
)

// =============================================================================
// Raw Input Models (v1 DynamoDB/NATS KV bucket format)
// =============================================================================
// These models represent the raw data format from v1 DynamoDB via NATS KV buckets.
// They use custom UnmarshalJSON to handle type conversions since Meltano sends
// all numeric fields as strings.

// RecurrenceDBRaw represents raw recurrence data from v1 DynamoDB
type RecurrenceDBRaw struct {
	Type           int    `json:"type"`
	RepeatInterval int    `json:"repeat_interval"`
	WeeklyDays     string `json:"weekly_days"`
	MonthlyDay     int    `json:"monthly_day"`
	MonthlyWeek    int    `json:"monthly_week"`
	MonthlyWeekDay int    `json:"monthly_week_day"`
	EndTimes       int    `json:"end_times"`
	EndDateTime    string `json:"end_date_time"`
}

// UnmarshalJSON implements custom unmarshaling to handle both string and int inputs for numeric fields.
func (r *RecurrenceDBRaw) UnmarshalJSON(data []byte) error {
	tmp := struct {
		Type           interface{} `json:"type"`
		RepeatInterval interface{} `json:"repeat_interval"`
		WeeklyDays     interface{} `json:"weekly_days"`
		MonthlyDay     interface{} `json:"monthly_day"`
		MonthlyWeek    interface{} `json:"monthly_week"`
		MonthlyWeekDay interface{} `json:"monthly_week_day"`
		EndTimes       interface{} `json:"end_times"`
		EndDateTime    interface{} `json:"end_date_time"`
	}{}

	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}

	// Handle Type (string from Meltano, int/float64 from other sources)
	switch v := tmp.Type.(type) {
	case string:
		if v != "" {
			val, err := strconv.Atoi(v)
			if err != nil {
				return fmt.Errorf("invalid type format: %w", err)
			}
			r.Type = val
		}
	case float64:
		r.Type = int(v)
	case int:
		r.Type = v
	default:
		if v != nil {
			return fmt.Errorf("invalid type for type: %T", v)
		}
	}

	// Handle RepeatInterval
	switch v := tmp.RepeatInterval.(type) {
	case string:
		if v != "" {
			val, err := strconv.Atoi(v)
			if err != nil {
				return fmt.Errorf("invalid repeat_interval format: %w", err)
			}
			r.RepeatInterval = val
		}
	case float64:
		r.RepeatInterval = int(v)
	case int:
		r.RepeatInterval = v
	default:
		if v != nil {
			return fmt.Errorf("invalid type for repeat_interval: %T", v)
		}
	}

	// Handle MonthlyDay
	switch v := tmp.MonthlyDay.(type) {
	case string:
		if v != "" {
			val, err := strconv.Atoi(v)
			if err != nil {
				return fmt.Errorf("invalid monthly_day format: %w", err)
			}
			r.MonthlyDay = val
		}
	case float64:
		r.MonthlyDay = int(v)
	case int:
		r.MonthlyDay = v
	default:
		if v != nil {
			return fmt.Errorf("invalid type for monthly_day: %T", v)
		}
	}

	// Handle MonthlyWeek
	switch v := tmp.MonthlyWeek.(type) {
	case string:
		if v != "" {
			val, err := strconv.Atoi(v)
			if err != nil {
				return fmt.Errorf("invalid monthly_week format: %w", err)
			}
			r.MonthlyWeek = val
		}
	case float64:
		r.MonthlyWeek = int(v)
	case int:
		r.MonthlyWeek = v
	default:
		if v != nil {
			return fmt.Errorf("invalid type for monthly_week: %T", v)
		}
	}

	// Handle MonthlyWeekDay
	switch v := tmp.MonthlyWeekDay.(type) {
	case string:
		if v != "" {
			val, err := strconv.Atoi(v)
			if err != nil {
				return fmt.Errorf("invalid monthly_week_day format: %w", err)
			}
			r.MonthlyWeekDay = val
		}
	case float64:
		r.MonthlyWeekDay = int(v)
	case int:
		r.MonthlyWeekDay = v
	default:
		if v != nil {
			return fmt.Errorf("invalid type for monthly_week_day: %T", v)
		}
	}

	// Handle EndTimes
	switch v := tmp.EndTimes.(type) {
	case string:
		if v != "" {
			val, err := strconv.Atoi(v)
			if err != nil {
				return fmt.Errorf("invalid end_times format: %w", err)
			}
			r.EndTimes = val
		}
	case float64:
		r.EndTimes = int(v)
	case int:
		r.EndTimes = v
	default:
		if v != nil {
			return fmt.Errorf("invalid type for end_times: %T", v)
		}
	}

	// Handle WeeklyDays
	switch v := tmp.WeeklyDays.(type) {
	case string:
		r.WeeklyDays = v
	default:
		if v != nil {
			r.WeeklyDays = fmt.Sprintf("%v", v)
		}
	}

	// Handle EndDateTime
	switch v := tmp.EndDateTime.(type) {
	case string:
		r.EndDateTime = v
	default:
		if v != nil {
			r.EndDateTime = fmt.Sprintf("%v", v)
		}
	}

	return nil
}

// MeetingDBRaw represents raw meeting data from v1 DynamoDB/NATS KV bucket
type MeetingDBRaw struct {
	// MeetingID is the meeting ID (can be a UUID or numeric ID)
	MeetingID string `json:"meeting_id"`

	// ProjID is the ID of the LF project
	ProjID string `json:"proj_id"`

	// Committee is the ID of the committee
	// It is a Global Secondary Index on the meeting table.
	Committee string `json:"committee"`

	// CommitteeFilters is the list of filters associated with the committee
	CommitteeFilters []string `json:"committee_filters"`

	// Committees is the list of committees associated with this meeting
	Committees []models.Committee `json:"committees,omitempty"`

	// User is the ID of the Zoom user that is set to host the meeting (who the meeting is scheduled for)
	// It is a Global Secondary Index on the meeting table.
	User string `json:"user_id"`

	// Topic is the topic of the meeting - this field exists in Zoom for a meeting
	Topic string `json:"topic"`

	// Agenda is the agenda of the meeting - this field exists in Zoom for a meeting
	Agenda string `json:"agenda"`

	// Visibility is the visibility of the meeting on the LFX platform
	Visibility string `json:"visibility"`

	// MeetingType is the type of meeting - this field exists in Zoom for a meeting
	MeetingType string `json:"meeting_type"`

	// StartTime is the start time of the meeting in RFC3339 format.
	// If the meeting is a recurring meeting, this is the start time of the first occurrence.
	StartTime string `json:"start_time"`

	// Timezone is the timezone of the meeting.
	// The value should be from the IANA Timezone Database (e.g. "America/Los_Angeles").
	Timezone string `json:"timezone"`

	// Duration is the duration of the meeting in minutes.
	Duration int `json:"-"`

	// EarlyJoinTimeMinutes is the time in minutes before the meeting start time that the user can join the meeting.
	// This is needed because these meetings are scheduled on shared Zoom users and thus the meeting scheduler
	// needs to account for this early join time buffer.
	EarlyJoinTime int `json:"early_join_time"`

	// LastEndTime is the end time of the last occurrence of the meeting in unix timestamp format.
	// If the meeting is a non-recurring meeting, this is the end time of the one-time meeting.
	LastEndTime int64 `json:"last_end_time"`

	// HostKey is the host key of the Zoom user hosting the meeting.
	// It is a six-digit PIN that is rotated weekly by our change-host-keys cron job.
	// This host key is needed to be able to claim host during a meeting.
	HostKey string `json:"host_key"`

	// JoinUrl is the URL to the meeting join page maintained by the PCC team.
	// The URL is specific to the meeting ID and the password.
	// (e.g. https://zoom-lfx.dev.platform.linuxfoundation.org/meeting/93699735000?password=111)
	JoinURL string `json:"join_url"`

	// Password is a UUID that is generated by us when a meeting is created in this service.
	// It is used for the meeting join page to make it hard to find the URL without knowing the password.
	Password string `json:"password"`

	// Restricted is a flag that indicates if the meeting is restricted to only invited users of a meeting.
	// If restricted is false, then the meeting can be joined by anyone with the meeting ID and password.
	Restricted bool `json:"restricted"`

	// RecordingEnabled is a flag that indicates if the meeting is recorded.
	// If set to true, recording is enabled in Zoom since the recording is managed by Zoom.
	RecordingEnabled bool `json:"recording_enabled"`

	// TranscriptEnabled is a flag that indicates if the meeting transcript is enabled.
	// If set to true, recording is enabled in Zoom since the transcript is managed by Zoom.
	TranscriptEnabled bool `json:"transcript_enabled"`

	// RecordingAccess is the access level of the meeting recording within the LFX platform.
	RecordingAccess string `json:"recording_access"`

	// TranscriptAccess is the access level of the meeting transcript within the LFX platform.
	TranscriptAccess string `json:"transcript_access"`

	// CreatedAt is the timestamp of when the meeting was created in RFC3339 format.
	CreatedAt string `json:"created_at"`

	// UpdatedAt is the timestamp of when the meeting was last updated in RFC3339 format.
	UpdatedAt string `json:"updated_at"`

	// CreatedBy is the user that created the meeting.
	CreatedBy models.CreatedBy `json:"created_by"`

	// UpdatedBy is the user that last updated the meeting.
	UpdatedBy models.UpdatedBy `json:"updated_by"`

	// UpdatedByList is a list of users that have updated the meeting.
	UpdatedByList []models.UpdatedBy `json:"updated_by_list,omitempty"`

	// UseNewInviteEmailAddress is a flag that indicates if the meeting should use the new invite email address.
	// In January 2024, we switched to using a new email address as the organizer for meeting invites.
	// We needed to keep the old email address for existing meetings to avoid calendar issues.
	UseNewInviteEmailAddress bool `json:"use_new_invite_email_address"`

	// Recurrence is the recurrence pattern of the meeting.
	// This is managed by this service and not by Zoom. In Zoom, all meetings are scheduled as recurring with
	// no fixed time (type 3).
	Recurrence *models.ZoomMeetingRecurrence `json:"recurrence,omitempty"`

	// Occurrences is a list of [ZoomMeetingOccurrence] objects that represent the occurrences of the meeting.
	Occurrences []models.ZoomMeetingOccurrence `json:"occurrences,omitempty"`

	// CancelledOccurrences is a list of IDs of occurrences that have been cancelled.
	CancelledOccurrences []string `json:"cancelled_occurrences,omitempty"`

	// UpdatedOccurrences is a list of [UpdatedOccurrence] objects that represent the occurrences that have been updated
	// to a new set of values. Every occurrence has details that can be specific to that occurrence or those that follow,
	// such as the start time, duration, title, and description.
	UpdatedOccurrences []models.UpdatedOccurrence `json:"updated_occurrences,omitempty"`

	// IcsUIDTimezone is a field that is used to store the timezone of a meeting that is used to
	// generate the calendar UID. This was needed because if a meeting's timezone changed, the calendar UID
	// would change if we didn't anchor the UID to the timezone.
	IcsUIDTimezone string `json:"ics_uid_timezone,omitempty"`

	// IcsAdditionalUids is a list of additional calendar event UIDs that are used in the invites sent to registrants
	// for the meeting. All meetings have one UID that is the meeting ID to represent the initial recurrence pattern,
	// but for each updated occurrence that affects all of the following occurrences, another calendar event UID is needed
	// to represent that sequence of occurrences in ICS. Those UIDs are stored in the database to keep track of them.
	IcsAdditionalUids []string `json:"ics_additional_uids,omitempty"`

	// ZoomConfig is the configuration of the meeting in Zoom.
	ZoomConfig models.ZoomConfig `json:"zoom_config"`

	// AISummaryAccess is the access level of the meeting AI summary within the LFX platform.
	// This is only relevant if [ZoomAIEnabled] is true.
	AISummaryAccess string `json:"ai_summary_access,omitempty"`

	// YoutubeUploadEnabled is a flag that indicates if the meeting's recording should be uploaded to Youtube
	YoutubeUploadEnabled bool `json:"youtube_upload_enabled,omitempty"`

	// ConcurrentZoomUserEnabled is a flag that indicates if the meeting is hosted on a zoom user with concurrent zoom licenses
	// enabled (which means it is hosted on a different set of pooled users).
	// TODO: remove the above ConcurrentZoomUserEnabled flag once all meetings have been moved to start using concurrent zoom licenses
	ConcurrentZoomUserEnabled bool `json:"concurrent_zoom_user_enabled,omitempty"`

	// LastBulkRegistrantJobStatus is the status of the last bulk insert job that was run to insert registrants
	LastBulkRegistrantJobStatus string `json:"last_bulk_registrant_job_status"`

	// LastBulkRegistrantsJobFailedCount is the total number of failed records in the last bulk insert job that was run to insert registrants
	LastBulkRegistrantsJobFailedCount int `json:"-"`

	// LastBulkRegistrantsJobWarningCount is the total number of passed records with warnings in the last bulk insert job that was run to insert registrants
	LastBulkRegistrantsJobWarningCount int `json:"-"`

	// LastMailingListMembersSyncJobStatus is the status of the last bulk insert job that was run to insert registrants
	LastMailingListMembersSyncJobStatus string `json:"last_mailing_list_members_sync_job_status"`

	// LastMailingListMembersSyncJobFailedCount is the total number of failed records in the last bulk insert job that was run to insert registrants
	LastMailingListMembersSyncJobFailedCount int `json:"-"`

	// MailingListGroupIDs is a list of group IDs that the meeting is associated with
	MailingListGroupIDs []string `json:"mailing_list_group_ids"`

	// LastMailingListMembersSyncJobWarningCount is the total number of passed records with warnings in the last bulk insert job that was run to insert registrants
	LastMailingListMembersSyncJobWarningCount int `json:"-"`

	// UseUniqueICSUID is a flag that indicates if the meeting should use a unique event ID for the calendar event.
	// Apply manually (generate uuid and store in this field) when a meeting has calendar issues, and we wish to use a separate unique uuid instead of the meeting ID.
	UseUniqueICSUID string `json:"use_unique_ics_uid"` // this is a uuid

	// ShowMeetingAttendees determines whether or not LFX One should show data about
	// meeting attendees to each other
	ShowMeetingAttendees bool `json:"show_meeting_attendees"`
}

// GetArtifactVisibility returns the artifact visibility of the meeting.
func (m *MeetingDBRaw) GetArtifactVisibility() string {
	if m.RecordingAccess != "" {
		return m.RecordingAccess
	}
	if m.TranscriptAccess != "" {
		return m.TranscriptAccess
	}
	if m.AISummaryAccess != "" {
		return m.AISummaryAccess
	}
	return "meeting_hosts"
}

// UnmarshalJSON implements custom unmarshaling to handle both string and int inputs for numeric fields.
// This struct is large, so only the 7 fields that need flexible type handling are listed as interface{}.
func (m *MeetingDBRaw) UnmarshalJSON(data []byte) error {
	tmp := struct {
		MeetingID                                 string                         `json:"meeting_id"`
		ProjID                                    string                         `json:"proj_id"`
		Committee                                 string                         `json:"committee"`
		CommitteeFilters                          []string                       `json:"committee_filters"`
		Committees                                []models.Committee             `json:"committees,omitempty"`
		User                                      string                         `json:"user_id"`
		Topic                                     string                         `json:"topic"`
		Agenda                                    string                         `json:"agenda"`
		Visibility                                string                         `json:"visibility"`
		MeetingType                               string                         `json:"meeting_type"`
		StartTime                                 string                         `json:"start_time"`
		Timezone                                  string                         `json:"timezone"`
		Duration                                  interface{}                    `json:"duration"`
		EarlyJoinTimeMinutes                      interface{}                    `json:"early_join_time_minutes"`
		LastEndTime                               interface{}                    `json:"last_end_time"`
		HostKey                                   string                         `json:"host_key"`
		JoinURL                                   string                         `json:"join_url"`
		Password                                  string                         `json:"password"`
		Restricted                                bool                           `json:"restricted"`
		RecordingEnabled                          bool                           `json:"recording_enabled"`
		TranscriptEnabled                         bool                           `json:"transcript_enabled"`
		RecordingAccess                           string                         `json:"recording_access"`
		TranscriptAccess                          string                         `json:"transcript_access"`
		CreatedAt                                 string                         `json:"created_at"`
		UpdatedAt                                 string                         `json:"updated_at"`
		CreatedBy                                 models.CreatedBy               `json:"created_by"`
		UpdatedBy                                 models.UpdatedBy               `json:"updated_by"`
		UpdatedByList                             []models.UpdatedBy             `json:"updated_by_list,omitempty"`
		UseNewInviteEmailAddress                  bool                           `json:"use_new_invite_email_address"`
		Recurrence                                *models.ZoomMeetingRecurrence  `json:"recurrence,omitempty"`
		Occurrences                               []models.ZoomMeetingOccurrence `json:"occurrences,omitempty"`
		CancelledOccurrences                      []string                       `json:"cancelled_occurrences,omitempty"`
		UpdatedOccurrences                        []models.UpdatedOccurrence     `json:"updated_occurrences,omitempty"`
		IcsUIDTimezone                            string                         `json:"ics_uid_timezone,omitempty"`
		IcsAdditionalUids                         []string                       `json:"ics_additional_uids,omitempty"`
		ZoomConfig                                models.ZoomConfig              `json:"zoom_config"`
		AISummaryAccess                           string                         `json:"ai_summary_access,omitempty"`
		YoutubeUploadEnabled                      bool                           `json:"youtube_upload_enabled,omitempty"`
		ConcurrentZoomUserEnabled                 bool                           `json:"concurrent_zoom_user_enabled,omitempty"`
		LastBulkRegistrantJobStatus               string                         `json:"last_bulk_registrant_job_status"`
		LastBulkRegistrantsJobFailedCount         interface{}                    `json:"last_bulk_registrants_job_failed_count"`
		LastBulkRegistrantsJobWarningCount        interface{}                    `json:"last_bulk_registrants_job_warning_count"`
		LastMailingListMembersSyncJobStatus       string                         `json:"last_mailing_list_members_sync_job_status"`
		LastMailingListMembersSyncJobFailedCount  interface{}                    `json:"last_mailing_list_members_sync_job_failed_count"`
		MailingListGroupIDs                       []string                       `json:"mailing_list_group_ids"`
		LastMailingListMembersSyncJobWarningCount interface{}                    `json:"last_mailing_list_members_sync_job_warning_count"`
		UseUniqueICSUID                           string                         `json:"use_unique_ics_uid"`
		ShowMeetingAttendees                      bool                           `json:"show_meeting_attendees"`
	}{}

	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}

	// Handle Duration
	switch v := tmp.Duration.(type) {
	case string:
		if v != "" {
			val, err := strconv.Atoi(v)
			if err != nil {
				return err
			}
			m.Duration = val
		}
	case float64:
		m.Duration = int(v)
	case int:
		m.Duration = v
	default:
		if v != nil {
			return fmt.Errorf("invalid type for duration: %T", v)
		}
	}

	// Handle EarlyJoinTimeMinutes
	switch v := tmp.EarlyJoinTimeMinutes.(type) {
	case string:
		if v != "" {
			val, err := strconv.Atoi(v)
			if err != nil {
				return err
			}
			m.EarlyJoinTime = val
		}
	case float64:
		m.EarlyJoinTime = int(v)
	case int:
		m.EarlyJoinTime = v
	default:
		if v != nil {
			return fmt.Errorf("invalid type for early_join_time_minutes: %T", v)
		}
	}

	// Handle LastEndTime (int64)
	switch v := tmp.LastEndTime.(type) {
	case string:
		if v != "" {
			val, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				return err
			}
			m.LastEndTime = val
		}
	case float64:
		m.LastEndTime = int64(v)
	case int64:
		m.LastEndTime = v
	case int:
		m.LastEndTime = int64(v)
	default:
		if v != nil {
			return fmt.Errorf("invalid type for last_end_time: %T", v)
		}
	}

	// Handle LastBulkRegistrantsJobFailedCount
	switch v := tmp.LastBulkRegistrantsJobFailedCount.(type) {
	case string:
		if v != "" {
			val, err := strconv.Atoi(v)
			if err != nil {
				return err
			}
			m.LastBulkRegistrantsJobFailedCount = val
		}
	case float64:
		m.LastBulkRegistrantsJobFailedCount = int(v)
	case int:
		m.LastBulkRegistrantsJobFailedCount = v
	default:
		if v != nil {
			return fmt.Errorf("invalid type for last_bulk_registrants_job_failed_count: %T", v)
		}
	}

	// Handle LastBulkRegistrantsJobWarningCount
	switch v := tmp.LastBulkRegistrantsJobWarningCount.(type) {
	case string:
		if v != "" {
			val, err := strconv.Atoi(v)
			if err != nil {
				return err
			}
			m.LastBulkRegistrantsJobWarningCount = val
		}
	case float64:
		m.LastBulkRegistrantsJobWarningCount = int(v)
	case int:
		m.LastBulkRegistrantsJobWarningCount = v
	default:
		if v != nil {
			return fmt.Errorf("invalid type for last_bulk_registrants_job_warning_count: %T", v)
		}
	}

	// Handle LastMailingListMembersSyncJobFailedCount
	switch v := tmp.LastMailingListMembersSyncJobFailedCount.(type) {
	case string:
		if v != "" {
			val, err := strconv.Atoi(v)
			if err != nil {
				return err
			}
			m.LastMailingListMembersSyncJobFailedCount = val
		}
	case float64:
		m.LastMailingListMembersSyncJobFailedCount = int(v)
	case int:
		m.LastMailingListMembersSyncJobFailedCount = v
	default:
		if v != nil {
			return fmt.Errorf("invalid type for last_mailing_list_members_sync_job_failed_count: %T", v)
		}
	}

	// Handle LastMailingListMembersSyncJobWarningCount
	switch v := tmp.LastMailingListMembersSyncJobWarningCount.(type) {
	case string:
		if v != "" {
			val, err := strconv.Atoi(v)
			if err != nil {
				return err
			}
			m.LastMailingListMembersSyncJobWarningCount = val
		}
	case float64:
		m.LastMailingListMembersSyncJobWarningCount = int(v)
	case int:
		m.LastMailingListMembersSyncJobWarningCount = v
	default:
		if v != nil {
			return fmt.Errorf("invalid type for last_mailing_list_members_sync_job_warning_count: %T", v)
		}
	}

	// Assign all other fields
	m.MeetingID = tmp.MeetingID
	m.ProjID = tmp.ProjID
	m.Committee = tmp.Committee
	m.CommitteeFilters = tmp.CommitteeFilters
	m.Committees = tmp.Committees
	m.User = tmp.User
	m.Topic = tmp.Topic
	m.Agenda = tmp.Agenda
	m.Visibility = tmp.Visibility
	m.MeetingType = tmp.MeetingType
	m.StartTime = tmp.StartTime
	m.Timezone = tmp.Timezone
	m.HostKey = tmp.HostKey
	m.JoinURL = tmp.JoinURL
	m.Password = tmp.Password
	m.Restricted = tmp.Restricted
	m.RecordingEnabled = tmp.RecordingEnabled
	m.TranscriptEnabled = tmp.TranscriptEnabled
	m.RecordingAccess = tmp.RecordingAccess
	m.TranscriptAccess = tmp.TranscriptAccess
	m.CreatedAt = tmp.CreatedAt
	m.UpdatedAt = tmp.UpdatedAt
	m.CreatedBy = tmp.CreatedBy
	m.UpdatedBy = tmp.UpdatedBy
	m.UpdatedByList = tmp.UpdatedByList
	m.UseNewInviteEmailAddress = tmp.UseNewInviteEmailAddress
	m.Recurrence = tmp.Recurrence
	m.Occurrences = tmp.Occurrences
	m.CancelledOccurrences = tmp.CancelledOccurrences
	m.UpdatedOccurrences = tmp.UpdatedOccurrences
	m.IcsUIDTimezone = tmp.IcsUIDTimezone
	m.IcsAdditionalUids = tmp.IcsAdditionalUids
	m.ZoomConfig = tmp.ZoomConfig
	m.AISummaryAccess = tmp.AISummaryAccess
	m.YoutubeUploadEnabled = tmp.YoutubeUploadEnabled
	m.ConcurrentZoomUserEnabled = tmp.ConcurrentZoomUserEnabled
	m.LastBulkRegistrantJobStatus = tmp.LastBulkRegistrantJobStatus
	m.LastMailingListMembersSyncJobStatus = tmp.LastMailingListMembersSyncJobStatus
	m.MailingListGroupIDs = tmp.MailingListGroupIDs
	m.UseUniqueICSUID = tmp.UseUniqueICSUID
	m.ShowMeetingAttendees = tmp.ShowMeetingAttendees

	return nil
}

// =============================================================================
// Invite Response Raw Input Models
// =============================================================================

// InviteResponseDBRaw represents raw invite response data from v1 DynamoDB/NATS KV bucket
type InviteResponseDBRaw struct {
	ID                     string `json:"id"`
	MeetingAndOccurrenceID string `json:"meeting_and_occurrence_id"`
	MeetingID              string `json:"meeting_id"`
	OccurrenceID           string `json:"occurrence_id"`
	RegistrantID           string `json:"registrant_id"`
	Email                  string `json:"email"`
	Name                   string `json:"name"`
	UserID                 string `json:"user_id"`
	Username               string `json:"username"`
	Org                    string `json:"org"`
	JobTitle               string `json:"job_title"`
	Response               string `json:"response"`
	Scope                  string `json:"scope"`
	ResponseDate           string `json:"response_date"`
	SESMessageID           string `json:"ses_message_id"`
	EmailSubject           string `json:"email_subject"`
	EmailText              string `json:"email_text"`
	CreatedAt              string `json:"created_at"`
	ModifiedAt             string `json:"modified_at"`
}

// =============================================================================
// Past Meeting Raw Input Models
// =============================================================================

// PastMeetingDBRaw represents raw past meeting data from v1 DynamoDB/NATS KV bucket
type PastMeetingDBRaw struct {
	UUID              string      `json:"uuid"`
	MeetingID         string      `json:"meeting_id"`
	ProjectID         string      `json:"proj_id"`
	Topic             string      `json:"topic"`
	Agenda            string      `json:"agenda"`
	StartTime         string      `json:"start_time"`
	EndTime           string      `json:"end_time"`
	Duration          interface{} `json:"duration"`
	Timezone          string      `json:"timezone"`
	ParticipantsCount interface{} `json:"participants_count"`
	HostID            string      `json:"host_id"`
	CreatedAt         string      `json:"created_at"`
	ModifiedAt        string      `json:"modified_at"`
}

// InviteeDBRaw represents raw past meeting invitee data from v1 DynamoDB/NATS KV bucket
type InviteeDBRaw struct {
	ID                     string `json:"id"`
	InviteeID              string `json:"invitee_id"`
	FirstName              string `json:"first_name"`
	LastName               string `json:"last_name"`
	Email                  string `json:"email"`
	ProfilePicture         string `json:"profile_picture"`
	LFSSO                  string `json:"lf_sso"`
	LFUserID               string `json:"lf_user_id,omitempty"`
	Org                    string `json:"org"`
	OrgIsMember            *bool  `json:"org_is_member,omitempty"`
	OrgIsProjectMember     *bool  `json:"org_is_project_member,omitempty"`
	JobTitle               string `json:"job_title"`
	RegistrantID           string `json:"registrant_id"`
	ProjectID              string `json:"proj_id,omitempty"`
	MeetingAndOccurrenceID string `json:"meeting_and_occurrence_id,omitempty"`
	MeetingID              string `json:"meeting_id,omitempty"`
	OccurrenceID           string `json:"occurrence_id"`
	CreatedAt              string `json:"created_at"`
	ModifiedAt             string `json:"modified_at"`
}

// AttendeeDBRaw represents raw past meeting attendee data from v1 DynamoDB/NATS KV bucket
type AttendeeDBRaw struct {
	ID                     string                 `json:"id"`
	ProjectID              string                 `json:"proj_id"`
	RegistrantID           string                 `json:"registrant_id"`
	Email                  string                 `json:"email"`
	Name                   string                 `json:"name"`
	LFSSO                  string                 `json:"lf_sso"`
	LFUserID               string                 `json:"lf_user_id"`
	Org                    string                 `json:"org"`
	OrgIsMember            *bool                  `json:"org_is_member,omitempty"`
	OrgIsProjectMember     *bool                  `json:"org_is_project_member,omitempty"`
	JobTitle               string                 `json:"job_title"`
	ProfilePicture         string                 `json:"profile_picture"`
	MeetingID              string                 `json:"meeting_id"`
	OccurrenceID           string                 `json:"occurrence_id"`
	MeetingAndOccurrenceID string                 `json:"meeting_and_occurrence_id"`
	Sessions               []AttendeeSessionDBRaw `json:"sessions"`
	CreatedAt              string                 `json:"created_at"`
	ModifiedAt             string                 `json:"modified_at"`
}

// AttendeeSessionDBRaw represents raw attendee session data from v1 DynamoDB/NATS KV bucket
type AttendeeSessionDBRaw struct {
	ParticipantUUID string `json:"participant_uuid"`
	JoinTime        string `json:"join_time"`
	LeaveTime       string `json:"leave_time"`
	LeaveReason     string `json:"leave_reason"`
}

// RecordingDBRaw represents raw past meeting recording data from v1 DynamoDB/NATS KV bucket
type RecordingDBRaw struct {
	ID                     string                  `json:"id"`
	MeetingAndOccurrenceID string                  `json:"meeting_and_occurrence_id"`
	ProjectID              string                  `json:"proj_id"`
	HostEmail              string                  `json:"host_email"`
	HostID                 string                  `json:"host_id"`
	MeetingID              string                  `json:"meeting_id"`
	OccurrenceID           string                  `json:"occurrence_id,omitempty"`
	PlatformMeetingID      string                  `json:"platform_meeting_id"`
	RecordingAccess        string                  `json:"recording_access,omitempty"`
	Topic                  string                  `json:"topic"` // v1 field name
	TranscriptAccess       string                  `json:"transcript_access,omitempty"`
	TranscriptEnabled      bool                    `json:"transcript_enabled"`
	Visibility             string                  `json:"visibility,omitempty"`
	RecordingCount         int                     `json:"recording_count"`
	RecordingFiles         []RecordingFileDBRaw    `json:"recording_files"`
	Sessions               []RecordingSessionDBRaw `json:"sessions"`
	StartTime              string                  `json:"start_time"`
	TotalSize              int64                   `json:"total_size"`
	CreatedAt              string                  `json:"created_at"`
	ModifiedAt             string                  `json:"modified_at"`
}

// RecordingFileDBRaw represents raw recording file data from v1 DynamoDB/NATS KV bucket
type RecordingFileDBRaw struct {
	DownloadURL    string `json:"download_url,omitempty"`
	FileExtension  string `json:"file_extension"`
	FileSize       int64  `json:"file_size"`
	FileType       string `json:"file_type"`
	ID             string `json:"id"`
	MeetingID      string `json:"meeting_id"`
	PlayURL        string `json:"play_url,omitempty"`
	RecordingStart string `json:"recording_start"`
	RecordingEnd   string `json:"recording_end"`
	RecordingType  string `json:"recording_type"`
	Status         string `json:"status"`
}

// RecordingSessionDBRaw represents raw recording session data from v1 DynamoDB/NATS KV bucket
type RecordingSessionDBRaw struct {
	UUID      string `json:"uuid"`
	ShareURL  string `json:"share_url,omitempty"`
	TotalSize int64  `json:"total_size"`
	StartTime string `json:"start_time"`
}

// SummaryDBRaw represents raw past meeting summary data from v1 DynamoDB/NATS KV bucket
type SummaryDBRaw struct {
	ID                     string               `json:"id"`
	MeetingAndOccurrenceID string               `json:"meeting_and_occurrence_id"`
	ProjectID              string               `json:"proj_id,omitempty"`
	MeetingID              string               `json:"meeting_id"`
	OccurrenceID           string               `json:"occurrence_id,omitempty"`
	ZoomMeetingUUID        string               `json:"zoom_meeting_uuid"`
	ZoomMeetingHostID      string               `json:"zoom_meeting_host_id"`
	ZoomMeetingHostEmail   string               `json:"zoom_meeting_host_email"`
	ZoomMeetingTopic       string               `json:"zoom_meeting_topic"`
	SummaryOverview        string               `json:"summary_overview"`
	SummaryTitle           string               `json:"summary_title"`
	SummaryDetails         []SummaryDetailDBRaw `json:"summary_details"`
	NextSteps              []string             `json:"next_steps"`
	EditedSummaryOverview  string               `json:"edited_summary_overview,omitempty"`
	EditedSummaryDetails   []SummaryDetailDBRaw `json:"edited_summary_details,omitempty"`
	EditedNextSteps        []string             `json:"edited_next_steps,omitempty"`
	RequiresApproval       bool                 `json:"requires_approval"`
	Approved               bool                 `json:"approved"`
	EmailSent              bool                 `json:"email_sent"`
	CreatedAt              string               `json:"created_at"`
	ModifiedAt             string               `json:"modified_at"`
}

// SummaryDetailDBRaw represents raw summary detail data from v1 DynamoDB/NATS KV bucket
type SummaryDetailDBRaw struct {
	Label   string `json:"label"`
	Summary string `json:"summary"`
}
