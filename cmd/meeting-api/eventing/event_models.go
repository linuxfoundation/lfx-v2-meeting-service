// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"encoding/json"
	"fmt"
	"strconv"
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
// This is only used for unmarshaling - numeric and boolean fields come as strings from DynamoDB
type MeetingDBRaw struct {
	MeetingID            string           `json:"meeting_id"`
	ProjectID            string           `json:"proj_id"`
	Topic                string           `json:"topic"`
	Agenda               string           `json:"agenda"`
	StartTime            string           `json:"start_time"`
	Duration             int              `json:"duration"`
	Timezone             string           `json:"timezone"`
	Visibility           string           `json:"visibility"`
	Restricted           bool             `json:"restricted"`
	MeetingType          string           `json:"meeting_type"`
	EarlyJoinTime        int              `json:"early_join_time"`
	RecordingEnabled     bool             `json:"recording_enabled"`
	TranscriptEnabled    bool             `json:"transcript_enabled"`
	YoutubeUploadEnabled bool             `json:"youtube_upload_enabled"`
	RecordingAccess      string           `json:"recording_access"`
	TranscriptAccess     string           `json:"transcript_access"`
	AISummaryAccess      string           `json:"ai_summary_access"`
	Recurrence           *RecurrenceDBRaw `json:"recurrence"`
	HostKey              string           `json:"host_key"`
	Passcode             string           `json:"passcode"`
	Password             string           `json:"password"`
	PublicLink           string           `json:"public_link"`
	CreatedAt            string           `json:"created_at"`
	ModifiedAt           string           `json:"modified_at"`
	LastModifiedByID     string           `json:"lastmodifiedbyid"`
}

// UnmarshalJSON implements custom unmarshaling to handle both string and int/bool inputs for fields.
func (m *MeetingDBRaw) UnmarshalJSON(data []byte) error {
	tmp := struct {
		MeetingID            string                 `json:"meeting_id"`
		ProjectID            string                 `json:"proj_id"`
		Topic                string                 `json:"topic"`
		Agenda               string                 `json:"agenda"`
		StartTime            string                 `json:"start_time"`
		Duration             interface{}            `json:"duration"`
		Timezone             string                 `json:"timezone"`
		Visibility           string                 `json:"visibility"`
		Restricted           interface{}            `json:"restricted"`
		MeetingType          interface{}            `json:"meeting_type"`
		EarlyJoinTime        interface{}            `json:"early_join_time"`
		RecordingEnabled     interface{}            `json:"recording_enabled"`
		TranscriptEnabled    interface{}            `json:"transcript_enabled"`
		YoutubeUploadEnabled interface{}            `json:"youtube_upload_enabled"`
		RecordingAccess      interface{}            `json:"recording_access"`
		TranscriptAccess     interface{}            `json:"transcript_access"`
		AISummaryAccess      interface{}            `json:"ai_summary_access"`
		Recurrence           map[string]interface{} `json:"recurrence"`
		HostKey              string                 `json:"host_key"`
		Passcode             string                 `json:"passcode"`
		Password             string                 `json:"password"`
		PublicLink           string                 `json:"public_link"`
		CreatedAt            string                 `json:"created_at"`
		ModifiedAt           string                 `json:"modified_at"`
		LastModifiedByID     string                 `json:"lastmodifiedbyid"`
	}{}

	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}

	// Handle Duration (string from Meltano, int/float64 from other sources)
	switch v := tmp.Duration.(type) {
	case string:
		if v != "" {
			val, err := strconv.Atoi(v)
			if err != nil {
				return fmt.Errorf("invalid duration format: %w", err)
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

	// Handle EarlyJoinTime (string from Meltano, int/float64 from other sources)
	switch v := tmp.EarlyJoinTime.(type) {
	case string:
		if v != "" {
			val, err := strconv.Atoi(v)
			if err != nil {
				return fmt.Errorf("invalid early_join_time format: %w", err)
			}
			m.EarlyJoinTime = val
		}
	case float64:
		m.EarlyJoinTime = int(v)
	case int:
		m.EarlyJoinTime = v
	default:
		if v != nil {
			return fmt.Errorf("invalid type for early_join_time: %T", v)
		}
	}

	// Handle Restricted (string from Meltano, bool/int from other sources)
	switch v := tmp.Restricted.(type) {
	case bool:
		m.Restricted = v
	case string:
		m.Restricted = v == "true" || v == "1"
	case int:
		m.Restricted = v != 0
	case float64:
		m.Restricted = v != 0
	default:
		if v != nil {
			return fmt.Errorf("invalid type for restricted: %T", v)
		}
	}

	// Handle RecordingEnabled (string from Meltano, bool/int from other sources)
	switch v := tmp.RecordingEnabled.(type) {
	case bool:
		m.RecordingEnabled = v
	case string:
		m.RecordingEnabled = v == "true" || v == "1"
	case int:
		m.RecordingEnabled = v != 0
	case float64:
		m.RecordingEnabled = v != 0
	default:
		if v != nil {
			return fmt.Errorf("invalid type for recording_enabled: %T", v)
		}
	}

	// Handle TranscriptEnabled (string from Meltano, bool/int from other sources)
	switch v := tmp.TranscriptEnabled.(type) {
	case bool:
		m.TranscriptEnabled = v
	case string:
		m.TranscriptEnabled = v == "true" || v == "1"
	case int:
		m.TranscriptEnabled = v != 0
	case float64:
		m.TranscriptEnabled = v != 0
	default:
		if v != nil {
			return fmt.Errorf("invalid type for transcript_enabled: %T", v)
		}
	}

	// Handle YoutubeUploadEnabled (string from Meltano, bool/int from other sources)
	switch v := tmp.YoutubeUploadEnabled.(type) {
	case bool:
		m.YoutubeUploadEnabled = v
	case string:
		m.YoutubeUploadEnabled = v == "true" || v == "1"
	case int:
		m.YoutubeUploadEnabled = v != 0
	case float64:
		m.YoutubeUploadEnabled = v != 0
	default:
		if v != nil {
			return fmt.Errorf("invalid type for youtube_upload_enabled: %T", v)
		}
	}

	// Handle MeetingType (string from Meltano or other sources)
	switch v := tmp.MeetingType.(type) {
	case string:
		m.MeetingType = v
	case int, float64:
		m.MeetingType = fmt.Sprintf("%v", v)
	default:
		if v != nil {
			return fmt.Errorf("invalid type for meeting_type: %T", v)
		}
	}

	// Handle RecordingAccess (string from Meltano or other sources)
	switch v := tmp.RecordingAccess.(type) {
	case string:
		m.RecordingAccess = v
	default:
		if v != nil {
			m.RecordingAccess = fmt.Sprintf("%v", v)
		}
	}

	// Handle TranscriptAccess (string from Meltano or other sources)
	switch v := tmp.TranscriptAccess.(type) {
	case string:
		m.TranscriptAccess = v
	default:
		if v != nil {
			m.TranscriptAccess = fmt.Sprintf("%v", v)
		}
	}

	// Handle AISummaryAccess (string from Meltano or other sources)
	switch v := tmp.AISummaryAccess.(type) {
	case string:
		m.AISummaryAccess = v
	default:
		if v != nil {
			m.AISummaryAccess = fmt.Sprintf("%v", v)
		}
	}

	// Handle Recurrence (convert map to RecurrenceDBRaw)
	if tmp.Recurrence != nil {
		recBytes, err := json.Marshal(tmp.Recurrence)
		if err != nil {
			return fmt.Errorf("failed to marshal recurrence: %w", err)
		}
		var rec RecurrenceDBRaw
		if err := json.Unmarshal(recBytes, &rec); err != nil {
			return fmt.Errorf("failed to unmarshal recurrence: %w", err)
		}
		m.Recurrence = &rec
	}

	// Assign all other fields
	m.MeetingID = tmp.MeetingID
	m.ProjectID = tmp.ProjectID
	m.Topic = tmp.Topic
	m.Agenda = tmp.Agenda
	m.StartTime = tmp.StartTime
	m.Timezone = tmp.Timezone
	m.Visibility = tmp.Visibility
	m.HostKey = tmp.HostKey
	m.Passcode = tmp.Passcode
	m.Password = tmp.Password
	m.PublicLink = tmp.PublicLink
	m.CreatedAt = tmp.CreatedAt
	m.ModifiedAt = tmp.ModifiedAt
	m.LastModifiedByID = tmp.LastModifiedByID

	return nil
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
