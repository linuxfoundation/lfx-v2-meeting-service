// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package models

import (
	"strings"
	"time"
)

// This file contains event data models for v1→v2 meeting event transformation.

// CreatedBy represents the user that created a resource.
type CreatedBy struct {
	UserID         string `json:"user_id,omitempty"`
	Username       string `json:"username,omitempty"`
	Email          string `json:"email,omitempty"`
	Name           string `json:"name,omitempty"`
	ProfilePicture string `json:"profile_picture,omitempty"`
}

// UpdatedBy represents the user that updated a resource.
type UpdatedBy struct {
	UserID         string `json:"user_id,omitempty"`
	Username       string `json:"username,omitempty"`
	Email          string `json:"email,omitempty"`
	Name           string `json:"name,omitempty"`
	ProfilePicture string `json:"profile_picture,omitempty"`
}

// ZoomMeetingRecurrence is the schema for a meeting recurrence
type ZoomMeetingRecurrence struct {
	// Type is the type of recurrence.
	Type int `json:"type"`

	// RepeatInterval is the interval of the recurrence.
	// For example, if the recurrence type is daily, the repeat interval is the number of days between occurrences.
	RepeatInterval int `json:"repeat_interval"`

	// WeeklyDays is the days of the week that the recurrence occurs on.
	// This is only relevant for type 2 (weekly) meetings.
	WeeklyDays string `json:"weekly_days,omitempty"`

	// MonthlyDay is the day of the month that the recurrence occurs on.
	// This is only relevant for type 3 (monthly) meetings.
	MonthlyDay int `json:"monthly_day,omitempty"`

	// MonthlyWeek is the week of the month that the recurrence occurs on.
	// This is only relevant for type 3 (monthly) meetings and should not be paired with [MonthlyDay].
	MonthlyWeek int `json:"monthly_week,omitempty"`

	// MonthlyWeekDay is the day of the week that the recurrence occurs on.
	// This is only relevant for type 3 (monthly) meetings and it is paired with [MonthlyWeek].
	MonthlyWeekDay int `json:"monthly_week_day,omitempty"`

	// EndTimes is the number of times to repeat the recurrence pattern.
	// For example, if set to 30 for a daily recurring meeting, then 30 occurrences will be created.
	EndTimes int `json:"end_times,omitempty"`

	// EndDateTime is the date and time in RFC3339 format that the recurrence pattern will end.
	EndDateTime string `json:"end_date_time,omitempty"`
}

// UpdatedOccurrence is the schema for an updated meeting occurrence
type UpdatedOccurrence struct {
	// OldOccurrenceID is the original occurrence ID, which is the original start time of the occurrence
	// as unix timestamp
	OldOccurrenceID string `json:"old_occurrence_id"`

	// NewOccurrenceID is the new occurrence ID, which is the new start time of the occurrence
	// as unix timestamp.
	// If the start time of the updated occurrence did not change, then the new occurrence ID is the same as the old one.
	NewOccurrenceID string `json:"new_occurrence_id"`

	// Timezone is the updated timezone
	Timezone string `json:"timezone"`

	// Duration is the updated duration of occurrence in minutes
	Duration int `json:"duration"`

	// Title is the updated title of the occurrence
	Title string `json:"title"`

	// Description is the updated description of the occurrence
	Description string `json:"description"`

	// Recurrence is the updated recurrence pattern for the occurrence
	Recurrence *ZoomMeetingRecurrence `json:"recurrence"`

	// AllFollowing is a flag that indicates if the updated occurrence changes should be applied to all following occurrences.
	// If this is set to true, then occurrences after this updated occurrence will used these values up until the next
	// occurrence that is also updated to a new set of values.
	AllFollowing bool `json:"all_following"`
}

// ZoomConfig is the configuration of the meeting in Zoom.
type ZoomConfig struct {
	MeetingID                string `json:"meeting_id,omitempty"`
	Passcode                 string `json:"passcode,omitempty"`
	AICompanionEnabled       bool   `json:"ai_companion_enabled"`
	AISummaryRequireApproval bool   `json:"ai_summary_require_approval"`
}

// ZoomMeetingOccurrence is the schema for a meeting occurrence
// Note that occurrences only exist in this system and not in Zoom. Since meetings are scheduled as
// recurring non-fixed meetings in Zoom, we need to track the occurrences in this system to be able to
// manage the occurrences.
type ZoomMeetingOccurrence struct {
	// OccurrenceID is the start of the occurrence in unix timestamp format
	OccurrenceID string `json:"occurrence_id"`

	// StartTime is the start time of the occurrence in RFC3339 format
	StartTime string `json:"start_time"`

	// Duration is the meeting duration in minutes
	Duration int `json:"duration"`

	// IsCancelled is a flag that indicates if the occurrence has been cancelled.
	// This is a v2 only attribute, where the value should come from the "status" field in the v1 data.
	IsCancelled bool `json:"is_cancelled"`

	// Title is the title of the occurrence
	Title string `json:"title"`

	// Description is the description of the occurrence
	Description string `json:"description"`

	// Recurrence is the recurrence pattern for the occurrence
	Recurrence *ZoomMeetingRecurrence `json:"recurrence,omitempty"`

	// ResponseCountYes is the number of invites that have been accepted for the occurrence
	ResponseCountYes int `json:"response_count_yes"`

	// ResponseCountNo is the number of invites that have been declined for the occurrence
	ResponseCountNo int `json:"response_count_no"`

	// RegistrantCount is the number of registrants for the occurrence
	RegistrantCount int `json:"registrant_count"`
}

// MeetingEventData represents a meeting event for indexing and access control
type MeetingEventData struct {
	// ID is the meeting ID (can be a UUID or numeric ID)
	ID string `json:"id"`

	// ProjectSFID is the salesforce ID of the LF project
	ProjectSFID string `json:"project_sfid"`

	// ProjectUID is the UID of the LF project
	// This is the v2 project UID.
	ProjectUID string `json:"project_uid"`

	// Committee is the v1 ID of the committee (SFID).
	// It is a Global Secondary Index on the meeting table.
	Committee string `json:"committee"`

	// CommitteeUID is the v2 UID of the primary committee, mapped from Committee.
	CommitteeUID string `json:"committee_uid,omitempty"`

	// CommitteeFilters is the list of filters associated with the committee
	CommitteeFilters []string `json:"committee_filters"`

	// Committees is the list of committees associated with this meeting
	Committees []Committee `json:"committees,omitempty"`

	// User is the ID of the Zoom user that is set to host the meeting (who the meeting is scheduled for)
	// It is a Global Secondary Index on the meeting table.
	User string `json:"user_id"`

	// Title is the title of the meeting - this field exists in Zoom for a meeting
	// This is a v2 only attribute, where the value should come from the "topic" field in the v1 data.
	Title string `json:"title"`

	// Description is the description of the meeting - this field exists in Zoom for a meeting
	// This is a v2 only attribute, where the value should come from the "agenda" field in the v1 data.
	Description string `json:"description"`

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
	Duration int `json:"duration"`

	// EarlyJoinTimeMinutes is the time in minutes before the meeting start time that the user can join the meeting.
	// This is needed because these meetings are scheduled on shared Zoom users and thus the meeting scheduler
	// needs to account for this early join time buffer.
	EarlyJoinTimeMinutes int `json:"early_join_time_minutes"`

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

	// ArtifactVisibility is the visibility of the meeting artifacts within the LFX platform.
	// This is a v2 only attribute, where the value should come from the "recording_access", "transcript_access", or "ai_summary_access" fields in the v1 data.
	ArtifactVisibility string `json:"artifact_visibility"`

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
	CreatedBy CreatedBy `json:"created_by"`

	// UpdatedBy is the user that last updated the meeting.
	UpdatedBy UpdatedBy `json:"updated_by"`

	// UpdatedByList is a list of users that have updated the meeting.
	UpdatedByList []UpdatedBy `json:"updated_by_list,omitempty"`

	// UseNewInviteEmailAddress is a flag that indicates if the meeting should use the new invite email address.
	// In January 2024, we switched to using a new email address as the organizer for meeting invites.
	// We needed to keep the old email address for existing meetings to avoid calendar issues.
	UseNewInviteEmailAddress bool `json:"use_new_invite_email_address"`

	// Recurrence is the recurrence pattern of the meeting.
	// This is managed by this service and not by Zoom. In Zoom, all meetings are scheduled as recurring with
	// no fixed time (type 3).
	Recurrence *ZoomMeetingRecurrence `json:"recurrence,omitempty"`

	// Occurrences is a list of [ZoomMeetingOccurrence] objects that represent the occurrences of the meeting.
	Occurrences []ZoomMeetingOccurrence `json:"occurrences,omitempty"`

	// CancelledOccurrences is a list of IDs of occurrences that have been cancelled.
	CancelledOccurrences []string `json:"cancelled_occurrences,omitempty"`

	// UpdatedOccurrences is a list of [UpdatedOccurrence] objects that represent the occurrences that have been updated
	// to a new set of values. Every occurrence has details that can be specific to that occurrence or those that follow,
	// such as the start time, duration, title, and description.
	UpdatedOccurrences []UpdatedOccurrence `json:"updated_occurrences,omitempty"`

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
	ZoomConfig ZoomConfig `json:"zoom_config"`

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
	LastBulkRegistrantsJobFailedCount int `json:"last_bulk_registrants_job_failed_count"`

	// LastBulkRegistrantsJobWarningCount is the total number of passed records with warnings in the last bulk insert job that was run to insert registrants
	LastBulkRegistrantsJobWarningCount int `json:"last_bulk_registrants_job_warning_count"`

	// LastMailingListMembersSyncJobStatus is the status of the last bulk insert job that was run to insert registrants
	LastMailingListMembersSyncJobStatus string `json:"last_mailing_list_members_sync_job_status"`

	// LastMailingListMembersSyncJobFailedCount is the total number of failed records in the last bulk insert job that was run to insert registrants
	LastMailingListMembersSyncJobFailedCount int `json:"last_mailing_list_members_sync_job_failed_count"`

	// MailingListGroupIDs is a list of group IDs that the meeting is associated with
	MailingListGroupIDs []string `json:"mailing_list_group_ids"`

	// LastMailingListMembersSyncJobWarningCount is the total number of passed records with warnings in the last bulk insert job that was run to insert registrants
	LastMailingListMembersSyncJobWarningCount int `json:"last_mailing_list_members_sync_job_warning_count"`

	// UseUniqueICSUID is a flag that indicates if the meeting should use a unique event ID for the calendar event.
	// Apply manually (generate uuid and store in this field) when a meeting has calendar issues, and we wish to use a separate unique uuid instead of the meeting ID.
	UseUniqueICSUID string `json:"use_unique_ics_uid"` // this is a uuid

	// ShowMeetingAttendees determines whether or not LFX One should show data about
	// meeting attendees to each other
	ShowMeetingAttendees bool `json:"show_meeting_attendees"`

	// Organizers is the list of usernames (Auth0 sub format) that are organizers of the meeting.
	Organizers []string `json:"organizers"`

	// AutoEmailReminderEnabled indicates whether automatic email reminders are enabled for the meeting.
	AutoEmailReminderEnabled bool `json:"auto_email_reminder_enabled"`

	// AutoEmailReminderTime is the time in minutes before the meeting start time that the reminder email is sent.
	AutoEmailReminderTime int `json:"auto_email_reminder_time"`
}

// SortName returns the primary sort name for this meeting.
func (m *MeetingEventData) SortName() string {
	return strings.TrimSpace(m.Title)
}

// NameAndAliases returns the searchable name aliases for this meeting.
func (m *MeetingEventData) NameAndAliases() []string {
	if t := strings.TrimSpace(m.Title); t != "" {
		return []string{t}
	}
	return nil
}

// FullText returns the fulltext search content for this meeting.
func (m *MeetingEventData) FullText() string {
	seen := make(map[string]bool)
	var parts []string
	for _, v := range append([]string{m.SortName()}, m.NameAndAliases()...) {
		if v != "" && !seen[v] {
			parts = append(parts, v)
			seen[v] = true
		}
	}
	if desc := strings.TrimSpace(m.Description); desc != "" && !seen[desc] {
		parts = append(parts, desc)
	}
	return strings.Join(parts, " ")
}

// Tags returns the indexer tags for this meeting.
func (m *MeetingEventData) Tags() []string {
	tags := []string{
		m.ID,
		"meeting_id:" + m.ID,
		"project_uid:" + m.ProjectUID,
		"title:" + m.Title,
	}
	if m.Visibility != "" {
		tags = append(tags, "visibility:"+m.Visibility)
	}
	if m.MeetingType != "" {
		tags = append(tags, "meeting_type:"+m.MeetingType)
	}
	return tags
}

// ParentRefs returns the indexer parent references for this meeting.
func (m *MeetingEventData) ParentRefs() []string {
	refs := []string{"project:" + m.ProjectUID}
	for _, c := range m.Committees {
		if c.UID != "" {
			refs = append(refs, "committee:"+c.UID)
		}
	}
	return refs
}

// Occurrence represents a single meeting occurrence
type Occurrence struct {
	OccurrenceID string                 `json:"occurrence_id"`
	StartTime    time.Time              `json:"start_time"`
	Duration     int                    `json:"duration"`
	IsCancelled  bool                   `json:"is_cancelled"`
	Title        string                 `json:"title"`
	Description  string                 `json:"description"`
	Recurrence   *ZoomMeetingRecurrence `json:"recurrence,omitempty"`
}

// RegistrantEventData represents a registrant event for indexing and access control
type RegistrantEventData struct {
	// UID is the partition key of the registrant (it is a UUID)
	UID string `json:"uid"`

	// MeetingID is the ID of the meeting that the registrant is associated with.
	// It is a Global Secondary Index on the registrant table.
	MeetingID string `json:"meeting_id"`

	// Type is the type of registrant
	Type string `json:"type"`

	// CommitteeUID is the UID of the committee that the registrant is associated with.
	// It is only relevant if the [Type] field is [RegistrantTypeCommittee].
	// It is a Global Secondary Index on the registrant table.
	CommitteeUID string `json:"committee_uid"`

	// UserID is the ID of the user that the registrant is associated with.
	// It is a Global Secondary Index on the registrant table.
	UserID string `json:"user_id"`

	// Email is the email of the registrant.
	// This is the email address that will receive meeting invites and notifications.
	// It is a Global Secondary Index on the registrant table.
	Email string `json:"email"`

	// CaseInsensitiveEmail is the email of the registrant in lowercase.
	// It is a Global Secondary Index on the registrant table.
	CaseInsensitiveEmail string `json:"case_insensitive_email"`

	// FirstName is the first name of the registrant
	FirstName string `json:"first_name"`

	// LastName is the last name of the registrant
	LastName string `json:"last_name"`

	// OrgName is the name of the organization of the registrant
	OrgName string `json:"org_name,omitempty"`

	// OrgIsMember is a flag that indicates if the [OrgName] field is an organization that is a member of
	// the Linux Foundation.
	OrgIsMember *bool `json:"org_is_member,omitempty"`

	// OrgIsProjectMember is a flag that indicates if the [OrgName] field is an organization that is a member of
	// the LF project that the meeting is associated with.
	OrgIsProjectMember *bool `json:"org_is_project_member,omitempty"`

	// JobTitle is the job title of the registrant
	JobTitle string `json:"job_title,omitempty"`

	// Host is a flag that indicates if the registrant is a host.
	// If the registrant is a host, then they will be able to obtain the Zoom host key in the LFX platform.
	Host bool `json:"host"`

	// Occurrence is set with an occurrence ID when a registrant is invited to a specific occurrence of a meeting.
	// We only support a registrant being invited to a single occurrence or all occurrences of a meeting.
	// If this is unset, then the registrant is invited to all occurrences of the meeting.
	Occurrence string `json:"occurrence,omitempty"`

	// AvatarURL is the profile picture of the registrant
	AvatarURL string `json:"avatar_url"`

	// Username is the LF username of the registrant
	// It is a Global Secondary Index on the registrant table.
	Username string `json:"username,omitempty"`

	// LastInviteReceivedTime is the timestamp in RFC3339 format of the last invite sent to the registrant
	// TODO: rename this field in the database to last_invite_sent_time
	LastInviteReceivedTime string `json:"last_invite_received_time"`

	// LastInviteReceivedMessageID is the SES message ID of the last invite sent to the registrant
	// TODO: rename this field in the database to last_invite_sent_message_id
	LastInviteReceivedMessageID *string `json:"last_invite_received_message_id,omitempty"`

	// LastInviteDeliverySuccessful is a flag that indicates if the last invite email was delivered (tracked by SES)
	LastInviteDeliverySuccessful *bool `json:"last_invite_delivery_successful,omitempty"`

	// LastInviteDeliveredTime is the timestamp in RFC3339 format of when the last invite email was delivered (tracked by SES)
	LastInviteDeliveredTime string `json:"last_invite_delivered_time,omitempty"`

	// LastInviteBounced is a flag that indicates if the last invite email bounced (tracked by SES)
	LastInviteBounced *bool `json:"last_invite_bounced,omitempty"`

	// LastInviteBouncedTime is the timestamp in RFC3339 format of when the last invite email bounced (tracked by SES)
	LastInviteBouncedTime string `json:"last_invite_bounced_time,omitempty"`

	// LastInviteBouncedType is the type of bounce for the last invite email
	LastInviteBouncedType string `json:"last_invite_bounced_type,omitempty"`

	// LastInviteBouncedSubType is the sub-type of bounce for the last invite email
	LastInviteBouncedSubType string `json:"last_invite_bounced_sub_type,omitempty"`

	// LastInviteBouncedDiagnosticCode is the diagnostic code for the bounce for the last invite email
	LastInviteBouncedDiagnosticCode string `json:"last_invite_bounced_diagnostic_code,omitempty"`

	// CreatedAt is the timestamp in RFC3339 format of when the registrant was created
	CreatedAt string `json:"created_at"`

	// UpdatedAt is the timestamp in RFC3339 format of when the registrant was last updated
	UpdatedAt string `json:"updated_at"`

	// CreatedBy is the user that created the registrant
	CreatedBy CreatedBy `json:"created_by"`

	// UpdatedBy is the user that last updated the registrant
	UpdatedBy UpdatedBy `json:"updated_by"`
}

// SortName returns the primary sort name for this registrant.
func (r *RegistrantEventData) SortName() string {
	return strings.TrimSpace(r.Email)
}

// NameAndAliases returns the searchable name aliases for this registrant.
func (r *RegistrantEventData) NameAndAliases() []string {
	seen := make(map[string]bool)
	var result []string
	for _, v := range []string{r.Username, r.Email} {
		v = strings.TrimSpace(v)
		if v != "" && !seen[v] {
			result = append(result, v)
			seen[v] = true
		}
	}
	return result
}

// FullText returns the fulltext search content for this registrant.
func (r *RegistrantEventData) FullText() string {
	seen := make(map[string]bool)
	var parts []string
	for _, v := range append([]string{r.SortName()}, r.NameAndAliases()...) {
		if v != "" && !seen[v] {
			parts = append(parts, v)
			seen[v] = true
		}
	}
	return strings.Join(parts, " ")
}

// Tags returns the indexer tags for this registrant.
func (r *RegistrantEventData) Tags() []string {
	tags := []string{"registrant_uid:" + r.UID}
	if r.Username != "" {
		tags = append(tags, "username:"+r.Username)
	}
	if r.Email != "" {
		tags = append(tags, "email:"+r.Email)
	}
	if r.Host {
		tags = append(tags, "host:true")
	}
	return tags
}

// ParentRefs returns the indexer parent references for this registrant.
func (r *RegistrantEventData) ParentRefs() []string {
	refs := []string{"meeting:" + r.MeetingID}
	if r.CommitteeUID != "" {
		refs = append(refs, "committee:"+r.CommitteeUID)
	}
	return refs
}

// InviteResponseEventData represents an RSVP event for indexing
type InviteResponseEventData struct {
	ID                     string    `json:"id"`
	MeetingAndOccurrenceID string    `json:"meeting_and_occurrence_id"`
	MeetingID              string    `json:"meeting_id"`
	OccurrenceID           string    `json:"occurrence_id,omitempty"`
	RegistrantID           string    `json:"registrant_id"`
	ProjectUID             string    `json:"project_uid"`
	UserID                 string    `json:"user_id"`
	Username               string    `json:"username,omitempty"`
	Name                   string    `json:"name,omitempty"`
	Email                  string    `json:"email"`
	Org                    string    `json:"org,omitempty"`
	JobTitle               string    `json:"job_title,omitempty"`
	ResponseType           string    `json:"response_type"` // accepted, declined, maybe
	Scope                  string    `json:"scope"`         // all, single, this_and_following
	IsRecurring            bool      `json:"is_recurring"`
	CreatedAt              time.Time `json:"created_at"`
	ModifiedAt             time.Time `json:"modified_at"`
}

// SortName returns the primary sort name for this invite response.
func (r *InviteResponseEventData) SortName() string {
	return strings.TrimSpace(r.Email)
}

// NameAndAliases returns the searchable name aliases for this invite response.
func (r *InviteResponseEventData) NameAndAliases() []string {
	seen := make(map[string]bool)
	var result []string
	for _, v := range []string{r.Username, r.Email} {
		v = strings.TrimSpace(v)
		if v != "" && !seen[v] {
			result = append(result, v)
			seen[v] = true
		}
	}
	return result
}

// FullText returns the fulltext search content for this invite response.
func (r *InviteResponseEventData) FullText() string {
	seen := make(map[string]bool)
	var parts []string
	for _, v := range append([]string{r.SortName()}, r.NameAndAliases()...) {
		if v != "" && !seen[v] {
			parts = append(parts, v)
			seen[v] = true
		}
	}
	return strings.Join(parts, " ")
}

// Tags returns the indexer tags for this invite response.
func (r *InviteResponseEventData) Tags() []string {
	tags := []string{
		r.ID,
		"invite_response_uid:" + r.ID,
		"meeting_and_occurrence_id:" + r.MeetingAndOccurrenceID,
		"meeting_id:" + r.MeetingID,
		"registrant_uid:" + r.RegistrantID,
		"email:" + r.Email,
	}
	if r.Username != "" {
		tags = append(tags, "username:"+r.Username)
	}
	return tags
}

// ParentRefs returns the indexer parent references for this invite response.
func (r *InviteResponseEventData) ParentRefs() []string {
	return []string{"meeting:" + r.MeetingID}
}

// PastMeetingEventData represents a past meeting event for indexing and access control
type PastMeetingEventData struct {
	ID                       string               `json:"id"`         // UUID
	MeetingID                string               `json:"meeting_id"` // Original meeting ID
	MeetingAndOccurrenceID   string               `json:"meeting_and_occurrence_id"`
	OccurrenceID             string               `json:"occurrence_id,omitempty"`
	ProjectSFID              string               `json:"proj_id,omitempty"`
	ProjectUID               string               `json:"project_uid"`
	ProjectSlug              string               `json:"project_slug,omitempty"`
	Committee                string               `json:"committee,omitempty"`
	CommitteeUID             string               `json:"committee_uid,omitempty"`
	CommitteeFilters         []string             `json:"committee_filters,omitempty"`
	Title                    string               `json:"title"`
	Description              string               `json:"description"`
	StartTime                time.Time            `json:"start_time"`
	EndTime                  time.Time            `json:"end_time"`
	Duration                 int                  `json:"duration"` // Actual duration in minutes
	Timezone                 string               `json:"timezone"`
	MeetingType              string               `json:"meeting_type,omitempty"`
	Committees               []Committee          `json:"committees"`
	Visibility               string               `json:"visibility,omitempty"`
	ArtifactVisibility       string               `json:"artifact_visibility,omitempty"`
	Restricted               bool                 `json:"restricted"`
	RecordingEnabled         bool                 `json:"recording_enabled"`
	RecordingAccess          string               `json:"recording_access,omitempty"`
	TranscriptEnabled        bool                 `json:"transcript_enabled"`
	TranscriptAccess         string               `json:"transcript_access,omitempty"`
	ZoomAIEnabled            *bool                `json:"zoom_ai_enabled,omitempty"`
	AISummaryAccess          string               `json:"ai_summary_access,omitempty"`
	RequireAISummaryApproval *bool                `json:"require_ai_summary_approval,omitempty"`
	EarlyJoinTimeMinutes     int                  `json:"early_join_time_minutes,omitempty"`
	YoutubeLink              string               `json:"youtube_link,omitempty"`
	Platform                 string               `json:"platform,omitempty"`
	PlatformMeetingID        string               `json:"platform_meeting_id,omitempty"`
	RecordingPassword        string               `json:"recording_password,omitempty"`
	MeetingPassword          string               `json:"meeting_password,omitempty"`
	ZoomConfig               *ZoomConfig          `json:"zoom_config,omitempty"`
	IsManuallyCreated        bool                 `json:"is_manually_created,omitempty"`
	Sessions                 []PastMeetingSession `json:"sessions,omitempty"`
	CreatedAt                time.Time            `json:"created_at"`
	UpdatedAt                time.Time            `json:"updated_at"`
	CreatedBy                CreatedBy            `json:"created_by"`
	UpdatedBy                UpdatedBy            `json:"updated_by"`
	UpdatedByList            []UpdatedBy          `json:"updated_by_list,omitempty"`
}

// PastMeetingSession represents a single Zoom meeting instance within a past meeting
type PastMeetingSession struct {
	UUID      string    `json:"uuid"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
}

// SortName returns the primary sort name for this past meeting.
func (m *PastMeetingEventData) SortName() string {
	return strings.TrimSpace(m.Title)
}

// NameAndAliases returns the searchable name aliases for this past meeting.
func (m *PastMeetingEventData) NameAndAliases() []string {
	if t := strings.TrimSpace(m.Title); t != "" {
		return []string{t}
	}
	return nil
}

// FullText returns the fulltext search content for this past meeting.
func (m *PastMeetingEventData) FullText() string {
	seen := make(map[string]bool)
	var parts []string
	for _, v := range append([]string{m.SortName()}, m.NameAndAliases()...) {
		if v != "" && !seen[v] {
			parts = append(parts, v)
			seen[v] = true
		}
	}
	if desc := strings.TrimSpace(m.Description); desc != "" && !seen[desc] {
		parts = append(parts, desc)
	}
	return strings.Join(parts, " ")
}

// Tags returns the indexer tags for this past meeting.
func (m *PastMeetingEventData) Tags() []string {
	tags := []string{
		"past_meeting_id:" + m.ID,
		"meeting_id:" + m.MeetingID,
		"project_uid:" + m.ProjectUID,
		"title:" + m.Title,
	}
	if m.ProjectSlug != "" {
		tags = append(tags, "project_slug:"+m.ProjectSlug)
	}
	if m.Timezone != "" {
		tags = append(tags, "timezone:"+m.Timezone)
	}
	for _, c := range m.Committees {
		if c.UID != "" {
			tags = append(tags, "committee_uid:"+c.UID)
		}
	}
	return tags
}

// ParentRefs returns the indexer parent references for this past meeting.
func (m *PastMeetingEventData) ParentRefs() []string {
	refs := []string{"project:" + m.ProjectUID}
	for _, c := range m.Committees {
		if c.UID != "" {
			refs = append(refs, "committee:"+c.UID)
		}
	}
	return refs
}

// PastMeetingParticipantEventData represents a participant (invitee/attendee) event
type PastMeetingParticipantEventData struct {
	UID                    string               `json:"uid"`
	MeetingAndOccurrenceID string               `json:"meeting_and_occurrence_id"`
	MeetingID              string               `json:"meeting_id"`
	ProjectUID             string               `json:"project_uid"`
	ProjectSlug            string               `json:"project_slug,omitempty"`
	Email                  string               `json:"email"`
	FirstName              string               `json:"first_name"`
	LastName               string               `json:"last_name"`
	Host                   bool                 `json:"host"`
	JobTitle               string               `json:"job_title,omitempty"`
	OrgName                string               `json:"org_name,omitempty"`
	OrgIsMember            bool                 `json:"org_is_member"`
	OrgIsProjectMember     bool                 `json:"org_is_project_member"`
	AvatarURL              string               `json:"avatar_url,omitempty"`
	Username               string               `json:"username,omitempty"`
	IsInvited              bool                 `json:"is_invited"`
	IsAttended             bool                 `json:"is_attended"`
	IsUnknown              bool                 `json:"is_unknown"`
	IsAIReconciled         bool                 `json:"is_ai_reconciled"`
	IsAutoMatched          bool                 `json:"is_auto_matched"`
	ZoomUserName           string               `json:"zoom_user_name"`
	MappedInviteeName      string               `json:"mapped_invitee_name"`
	Sessions               []ParticipantSession `json:"sessions,omitempty"`
	CreatedAt              time.Time            `json:"created_at"`
	UpdatedAt              time.Time            `json:"updated_at"`
}

// SortName returns the primary sort name for this past meeting participant.
func (p *PastMeetingParticipantEventData) SortName() string {
	return strings.TrimSpace(p.Email)
}

// NameAndAliases returns the searchable name aliases for this past meeting participant.
func (p *PastMeetingParticipantEventData) NameAndAliases() []string {
	seen := make(map[string]bool)
	var result []string
	for _, v := range []string{p.FirstName, p.LastName, p.Username} {
		v = strings.TrimSpace(v)
		if v != "" && !seen[v] {
			result = append(result, v)
			seen[v] = true
		}
	}
	return result
}

// FullText returns the fulltext search content for this past meeting participant.
func (p *PastMeetingParticipantEventData) FullText() string {
	seen := make(map[string]bool)
	var parts []string
	for _, v := range append([]string{p.SortName()}, p.NameAndAliases()...) {
		if v != "" && !seen[v] {
			parts = append(parts, v)
			seen[v] = true
		}
	}
	return strings.Join(parts, " ")
}

// Tags returns the indexer tags for this past meeting participant.
func (p *PastMeetingParticipantEventData) Tags() []string {
	tags := []string{
		"past_meeting_participant_uid:" + p.UID,
		"meeting_and_occurrence_id:" + p.MeetingAndOccurrenceID,
	}
	if p.ProjectUID != "" {
		tags = append(tags, "project_uid:"+p.ProjectUID)
	}
	if p.ProjectSlug != "" {
		tags = append(tags, "project_slug:"+p.ProjectSlug)
	}
	if p.Username != "" {
		tags = append(tags, "username:"+p.Username)
	}
	if p.Email != "" {
		tags = append(tags, "email:"+p.Email)
	}
	if p.IsInvited {
		tags = append(tags, "is_invited:true")
	}
	if p.IsAttended {
		tags = append(tags, "is_attended:true")
	}
	return tags
}

// ParentRefs returns the indexer parent references for this past meeting participant.
func (p *PastMeetingParticipantEventData) ParentRefs() []string {
	refs := []string{"past_meeting:" + p.MeetingAndOccurrenceID}
	if p.ProjectUID != "" {
		refs = append(refs, "project:"+p.ProjectUID)
	}
	return refs
}

// ParticipantSession represents a join/leave session for attendees
type ParticipantSession struct {
	UID         string     `json:"uid"`
	JoinTime    *time.Time `json:"join_time,omitempty"`
	LeaveTime   *time.Time `json:"leave_time,omitempty"`
	LeaveReason string     `json:"leave_reason,omitempty"`
}

// RecordingEventData represents a recording artifact event
type RecordingEventData struct {
	ID                     string             `json:"id"`
	MeetingAndOccurrenceID string             `json:"meeting_and_occurrence_id"`
	ProjectUID             string             `json:"project_uid"`
	ProjectSlug            string             `json:"project_slug"`
	HostEmail              string             `json:"host_email"`
	HostID                 string             `json:"host_id"`
	MeetingID              string             `json:"meeting_id"`
	OccurrenceID           string             `json:"occurrence_id"`
	Platform               string             `json:"platform"` // Always "Zoom"
	PlatformMeetingID      string             `json:"platform_meeting_id"`
	RecordingAccess        string             `json:"recording_access"` // public, meeting_hosts, meeting_participants
	Title                  string             `json:"title"`
	TranscriptAccess       string             `json:"transcript_access,omitempty"`
	TranscriptEnabled      bool               `json:"transcript_enabled"`
	Visibility             string             `json:"visibility"`
	RecordingCount         int                `json:"recording_count"`
	RecordingFiles         []RecordingFile    `json:"recording_files"`
	Sessions               []RecordingSession `json:"sessions"`
	StartTime              time.Time          `json:"start_time"`
	TotalSize              int64              `json:"total_size"`
	Committees             []Committee        `json:"committees"`
	CreatedAt              time.Time          `json:"created_at"`
	UpdatedAt              time.Time          `json:"updated_at"`
	CreatedBy              CreatedBy          `json:"created_by"`
	UpdatedBy              UpdatedBy          `json:"updated_by"`
}

// SortName returns the primary sort name for this recording.
func (r *RecordingEventData) SortName() string {
	return strings.TrimSpace(r.Title)
}

// NameAndAliases returns the searchable name aliases for this recording.
func (r *RecordingEventData) NameAndAliases() []string {
	if t := strings.TrimSpace(r.Title); t != "" {
		return []string{t}
	}
	return nil
}

// FullText returns the fulltext search content for this recording.
func (r *RecordingEventData) FullText() string {
	seen := make(map[string]bool)
	var parts []string
	for _, v := range append([]string{r.SortName()}, r.NameAndAliases()...) {
		if v != "" && !seen[v] {
			parts = append(parts, v)
			seen[v] = true
		}
	}
	return strings.Join(parts, " ")
}

// Tags returns the indexer tags for this recording.
func (r *RecordingEventData) Tags() []string {
	tags := []string{
		r.ID,
		"past_meeting_recording_id:" + r.ID,
		"meeting_and_occurrence_id:" + r.MeetingAndOccurrenceID,
		"platform:Zoom",
		"platform_meeting_id:" + r.PlatformMeetingID,
	}
	if r.ProjectUID != "" {
		tags = append(tags, "project_uid:"+r.ProjectUID)
	}
	if r.ProjectSlug != "" {
		tags = append(tags, "project_slug:"+r.ProjectSlug)
	}
	for _, session := range r.Sessions {
		tags = append(tags, "platform_meeting_instance_id:"+session.UUID)
	}
	for _, c := range r.Committees {
		if c.UID != "" {
			tags = append(tags, "committee_uid:"+c.UID)
		}
	}
	return tags
}

// ParentRefs returns the indexer parent references for this recording.
func (r *RecordingEventData) ParentRefs() []string {
	refs := []string{"past_meeting:" + r.MeetingAndOccurrenceID}
	if r.ProjectUID != "" {
		refs = append(refs, "project:"+r.ProjectUID)
	}
	for _, c := range r.Committees {
		if c.UID != "" {
			refs = append(refs, "committee:"+c.UID)
		}
	}
	return refs
}

// RecordingFile represents a single recording file
type RecordingFile struct {
	DownloadURL    string    `json:"download_url,omitempty"`
	FileExtension  string    `json:"file_extension"`
	FileSize       int64     `json:"file_size"`
	FileType       string    `json:"file_type"`
	ID             string    `json:"id"`
	MeetingID      string    `json:"meeting_id"`
	PlayURL        string    `json:"play_url,omitempty"`
	RecordingStart time.Time `json:"recording_start"`
	RecordingEnd   time.Time `json:"recording_end"`
	RecordingType  string    `json:"recording_type"`
	Status         string    `json:"status"`
}

// RecordingSession represents a recording session
type RecordingSession struct {
	UUID      string    `json:"uuid"`
	ShareURL  string    `json:"share_url,omitempty"`
	TotalSize int64     `json:"total_size"`
	StartTime time.Time `json:"start_time"`
}

// TranscriptEventData represents a transcript artifact event
type TranscriptEventData struct {
	ID                     string             `json:"id"`
	MeetingAndOccurrenceID string             `json:"meeting_and_occurrence_id"`
	ProjectUID             string             `json:"project_uid"`
	ProjectSlug            string             `json:"project_slug"`
	HostEmail              string             `json:"host_email"`
	HostID                 string             `json:"host_id"`
	MeetingID              string             `json:"meeting_id"`
	OccurrenceID           string             `json:"occurrence_id"`
	Platform               string             `json:"platform"`          // Always "Zoom"
	TranscriptAccess       string             `json:"transcript_access"` // public, meeting_hosts, meeting_participants
	Title                  string             `json:"title"`
	Visibility             string             `json:"visibility"`
	RecordingFiles         []RecordingFile    `json:"recording_files"`
	Sessions               []RecordingSession `json:"sessions"`
	StartTime              time.Time          `json:"start_time"`
	TotalSize              int64              `json:"total_size"`
	Committees             []Committee        `json:"committees"`
	CreatedAt              time.Time          `json:"created_at"`
	UpdatedAt              time.Time          `json:"updated_at"`
	CreatedBy              CreatedBy          `json:"created_by"`
	UpdatedBy              UpdatedBy          `json:"updated_by"`
}

// SortName returns the primary sort name for this transcript.
func (t *TranscriptEventData) SortName() string {
	return strings.TrimSpace(t.Title)
}

// NameAndAliases returns the searchable name aliases for this transcript.
func (t *TranscriptEventData) NameAndAliases() []string {
	if v := strings.TrimSpace(t.Title); v != "" {
		return []string{v}
	}
	return nil
}

// FullText returns the fulltext search content for this transcript.
func (t *TranscriptEventData) FullText() string {
	seen := make(map[string]bool)
	var parts []string
	for _, v := range append([]string{t.SortName()}, t.NameAndAliases()...) {
		if v != "" && !seen[v] {
			parts = append(parts, v)
			seen[v] = true
		}
	}
	return strings.Join(parts, " ")
}

// Tags returns the indexer tags for this transcript.
func (t *TranscriptEventData) Tags() []string {
	tags := []string{
		t.ID,
		"past_meeting_transcript_id:" + t.ID,
		"meeting_and_occurrence_id:" + t.MeetingAndOccurrenceID,
		"platform:Zoom",
	}
	if t.ProjectUID != "" {
		tags = append(tags, "project_uid:"+t.ProjectUID)
	}
	if t.ProjectSlug != "" {
		tags = append(tags, "project_slug:"+t.ProjectSlug)
	}
	for _, session := range t.Sessions {
		tags = append(tags, "platform_meeting_instance_id:"+session.UUID)
	}
	for _, c := range t.Committees {
		if c.UID != "" {
			tags = append(tags, "committee_uid:"+c.UID)
		}
	}
	return tags
}

// ParentRefs returns the indexer parent references for this transcript.
func (t *TranscriptEventData) ParentRefs() []string {
	refs := []string{"past_meeting:" + t.MeetingAndOccurrenceID}
	if t.ProjectUID != "" {
		refs = append(refs, "project:"+t.ProjectUID)
	}
	for _, c := range t.Committees {
		if c.UID != "" {
			refs = append(refs, "committee:"+c.UID)
		}
	}
	return refs
}

// SummaryEventData represents an AI-generated summary event
type SummaryEventData struct {
	ID                      string            `json:"id"`
	MeetingAndOccurrenceID  string            `json:"meeting_and_occurrence_id"`
	ProjectUID              string            `json:"project_uid"`
	ProjectSlug             string            `json:"project_slug"`
	MeetingID               string            `json:"meeting_id"`
	OccurrenceID            string            `json:"occurrence_id"`
	ZoomMeetingUUID         string            `json:"zoom_meeting_uuid"`
	ZoomMeetingHostID       string            `json:"zoom_meeting_host_id"`
	ZoomMeetingHostEmail    string            `json:"zoom_meeting_host_email"`
	ZoomMeetingTopic        string            `json:"zoom_meeting_topic"`
	ZoomWebhookEvent        string            `json:"zoom_webhook_event,omitempty"`
	SummaryTitle            string            `json:"summary_title,omitempty"`
	SummaryStartTime        string            `json:"summary_start_time,omitempty"`
	SummaryEndTime          string            `json:"summary_end_time,omitempty"`
	SummaryCreatedTime      string            `json:"summary_created_time,omitempty"`
	SummaryLastModifiedTime string            `json:"summary_last_modified_time,omitempty"`
	Content                 string            `json:"content"`        // Consolidated markdown
	EditedContent           string            `json:"edited_content"` // Edited markdown
	RequiresApproval        bool              `json:"requires_approval"`
	Approved                bool              `json:"approved"`
	Platform                string            `json:"platform"` // Always "Zoom"
	ZoomConfig              SummaryZoomConfig `json:"zoom_config"`
	EmailSent               bool              `json:"email_sent"`
	Committees              []Committee       `json:"committees"`
	CreatedAt               time.Time         `json:"created_at"`
	UpdatedAt               time.Time         `json:"updated_at"`
	CreatedBy               CreatedBy         `json:"created_by"`
	UpdatedBy               UpdatedBy         `json:"updated_by"`
}

// SortName returns the primary sort name for this summary.
func (s *SummaryEventData) SortName() string {
	return strings.TrimSpace(s.ZoomMeetingTopic)
}

// NameAndAliases returns the searchable name aliases for this summary.
func (s *SummaryEventData) NameAndAliases() []string {
	if v := strings.TrimSpace(s.ZoomMeetingTopic); v != "" {
		return []string{v}
	}
	return nil
}

// FullText returns the fulltext search content for this summary.
func (s *SummaryEventData) FullText() string {
	seen := make(map[string]bool)
	var parts []string
	for _, v := range append([]string{s.SortName()}, s.NameAndAliases()...) {
		if v != "" && !seen[v] {
			parts = append(parts, v)
			seen[v] = true
		}
	}
	return strings.Join(parts, " ")
}

// Tags returns the indexer tags for this summary.
func (s *SummaryEventData) Tags() []string {
	tags := []string{
		s.ID,
		"past_meeting_summary_id:" + s.ID,
		"meeting_and_occurrence_id:" + s.MeetingAndOccurrenceID,
		"meeting_id:" + s.MeetingID,
		"platform:Zoom",
	}
	if s.ProjectUID != "" {
		tags = append(tags, "project_uid:"+s.ProjectUID)
	}
	if s.ProjectSlug != "" {
		tags = append(tags, "project_slug:"+s.ProjectSlug)
	}
	if s.ZoomMeetingTopic != "" {
		tags = append(tags, "title:"+s.ZoomMeetingTopic)
	}
	for _, c := range s.Committees {
		if c.UID != "" {
			tags = append(tags, "committee_uid:"+c.UID)
		}
	}
	return tags
}

// ParentRefs returns the indexer parent references for this summary.
func (s *SummaryEventData) ParentRefs() []string {
	refs := []string{"past_meeting:" + s.MeetingAndOccurrenceID}
	if s.ProjectUID != "" {
		refs = append(refs, "project:"+s.ProjectUID)
	}
	for _, c := range s.Committees {
		if c.UID != "" {
			refs = append(refs, "committee:"+c.UID)
		}
	}
	return refs
}

// SummaryZoomConfig contains Zoom-specific configuration for summaries
type SummaryZoomConfig struct {
	MeetingID   string `json:"meeting_id"`
	MeetingUUID string `json:"meeting_uuid"`
}

// MeetingAttachmentEventData represents an attachment on an active meeting
type MeetingAttachmentEventData struct {
	UID              string     `json:"uid"`
	MeetingID        string     `json:"meeting_id"`
	ProjectUID       string     `json:"project_uid,omitempty"`
	ProjectSlug      string     `json:"project_slug,omitempty"`
	Type             string     `json:"type"`
	Category         string     `json:"category,omitempty"`
	Link             string     `json:"link,omitempty"`
	Name             string     `json:"name"`
	Description      string     `json:"description,omitempty"`
	Source           string     `json:"source,omitempty"`
	FileName         string     `json:"file_name,omitempty"`
	FileSize         int        `json:"file_size,omitempty"`
	FileURL          string     `json:"file_url,omitempty"`
	FileUploaded     *bool      `json:"file_uploaded,omitempty"`
	FileUploadStatus string     `json:"file_upload_status,omitempty"`
	FileContentType  string     `json:"file_content_type,omitempty"`
	FileUploadedBy   *CreatedBy `json:"file_uploaded_by,omitempty"`
	FileUploadedAt   *time.Time `json:"file_uploaded_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	ModifiedAt       time.Time  `json:"modified_at"`
	CreatedBy        CreatedBy  `json:"created_by"`
	UpdatedBy        UpdatedBy  `json:"updated_by"`
}

// SortName returns the primary sort name for this meeting attachment.
func (a *MeetingAttachmentEventData) SortName() string {
	return strings.TrimSpace(a.Name)
}

// NameAndAliases returns the searchable name aliases for this meeting attachment.
func (a *MeetingAttachmentEventData) NameAndAliases() []string {
	seen := make(map[string]bool)
	var result []string
	for _, v := range []string{a.FileName, a.Link, a.Name} {
		v = strings.TrimSpace(v)
		if v != "" && !seen[v] {
			result = append(result, v)
			seen[v] = true
		}
	}
	return result
}

// FullText returns the fulltext search content for this meeting attachment.
func (a *MeetingAttachmentEventData) FullText() string {
	seen := make(map[string]bool)
	var parts []string
	for _, v := range append([]string{a.SortName()}, a.NameAndAliases()...) {
		if v != "" && !seen[v] {
			parts = append(parts, v)
			seen[v] = true
		}
	}
	if desc := strings.TrimSpace(a.Description); desc != "" && !seen[desc] {
		parts = append(parts, desc)
	}
	return strings.Join(parts, " ")
}

// Tags returns the indexer tags for this meeting attachment.
func (a *MeetingAttachmentEventData) Tags() []string {
	tags := []string{
		"meeting_attachment_uid:" + a.UID,
		"meeting_id:" + a.MeetingID,
	}
	if a.ProjectUID != "" {
		tags = append(tags, "project_uid:"+a.ProjectUID)
	}
	if a.ProjectSlug != "" {
		tags = append(tags, "project_slug:"+a.ProjectSlug)
	}
	if a.Type != "" {
		tags = append(tags, "type:"+a.Type)
	}
	return tags
}

// ParentRefs returns the indexer parent references for this meeting attachment.
func (a *MeetingAttachmentEventData) ParentRefs() []string {
	refs := []string{"meeting:" + a.MeetingID}
	if a.ProjectUID != "" {
		refs = append(refs, "project:"+a.ProjectUID)
	}
	return refs
}

// PastMeetingAttachmentEventData represents an attachment on a past meeting
type PastMeetingAttachmentEventData struct {
	UID                    string      `json:"uid"`
	MeetingAndOccurrenceID string      `json:"meeting_and_occurrence_id"`
	MeetingID              string      `json:"meeting_id"`
	ProjectUID             string      `json:"project_uid"`
	ProjectSlug            string      `json:"project_slug"`
	Type                   string      `json:"type"`
	Category               string      `json:"category,omitempty"`
	Link                   string      `json:"link,omitempty"`
	Name                   string      `json:"name"`
	Description            string      `json:"description,omitempty"`
	Source                 string      `json:"source,omitempty"`
	FileName               string      `json:"file_name,omitempty"`
	FileSize               int         `json:"file_size,omitempty"`
	FileURL                string      `json:"file_url,omitempty"`
	FileUploaded           *bool       `json:"file_uploaded,omitempty"`
	FileUploadStatus       string      `json:"file_upload_status,omitempty"`
	FileContentType        string      `json:"file_content_type,omitempty"`
	FileUploadedBy         *CreatedBy  `json:"file_uploaded_by,omitempty"`
	FileUploadedAt         *time.Time  `json:"file_uploaded_at,omitempty"`
	Committees             []Committee `json:"committees"`
	CreatedAt              time.Time   `json:"created_at"`
	ModifiedAt             time.Time   `json:"modified_at"`
	CreatedBy              CreatedBy   `json:"created_by"`
	UpdatedBy              UpdatedBy   `json:"updated_by"`
}

// SortName returns the primary sort name for this past meeting attachment.
func (a *PastMeetingAttachmentEventData) SortName() string {
	return strings.TrimSpace(a.Name)
}

// NameAndAliases returns the searchable name aliases for this past meeting attachment.
func (a *PastMeetingAttachmentEventData) NameAndAliases() []string {
	seen := make(map[string]bool)
	var result []string
	for _, v := range []string{a.FileName, a.Link, a.Name} {
		v = strings.TrimSpace(v)
		if v != "" && !seen[v] {
			result = append(result, v)
			seen[v] = true
		}
	}
	return result
}

// FullText returns the fulltext search content for this past meeting attachment.
func (a *PastMeetingAttachmentEventData) FullText() string {
	seen := make(map[string]bool)
	var parts []string
	for _, v := range append([]string{a.SortName()}, a.NameAndAliases()...) {
		if v != "" && !seen[v] {
			parts = append(parts, v)
			seen[v] = true
		}
	}
	if desc := strings.TrimSpace(a.Description); desc != "" && !seen[desc] {
		parts = append(parts, desc)
	}
	return strings.Join(parts, " ")
}

// Tags returns the indexer tags for this past meeting attachment.
func (a *PastMeetingAttachmentEventData) Tags() []string {
	tags := []string{
		"past_meeting_attachment_uid:" + a.UID,
		"meeting_and_occurrence_id:" + a.MeetingAndOccurrenceID,
		"meeting_id:" + a.MeetingID,
	}
	if a.ProjectUID != "" {
		tags = append(tags, "project_uid:"+a.ProjectUID)
	}
	if a.ProjectSlug != "" {
		tags = append(tags, "project_slug:"+a.ProjectSlug)
	}
	if a.Type != "" {
		tags = append(tags, "type:"+a.Type)
	}
	for _, c := range a.Committees {
		if c.UID != "" {
			tags = append(tags, "committee_uid:"+c.UID)
		}
	}
	return tags
}

// ParentRefs returns the indexer parent references for this past meeting attachment.
func (a *PastMeetingAttachmentEventData) ParentRefs() []string {
	refs := []string{"past_meeting:" + a.MeetingAndOccurrenceID}
	if a.ProjectUID != "" {
		refs = append(refs, "project:"+a.ProjectUID)
	}
	for _, c := range a.Committees {
		if c.UID != "" {
			refs = append(refs, "committee:"+c.UID)
		}
	}
	return refs
}
