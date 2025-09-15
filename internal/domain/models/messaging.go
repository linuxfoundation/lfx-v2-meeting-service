// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package models

import "time"

// NATS subjects that the meeting service sends messages about.
const (
	// IndexMeetingSubject is the subject for the meeting indexing.
	// The subject is of the form: lfx.index.meeting
	IndexMeetingSubject = "lfx.index.meeting"

	// IndexMeetingSettingsSubject is the subject for the meeting settings indexing.
	// The subject is of the form: lfx.index.meeting_settings
	IndexMeetingSettingsSubject = "lfx.index.meeting_settings"

	// IndexMeetingRegistrantSubject is the subject for the meeting registrant indexing.
	// The subject is of the form: lfx.index.meeting_registrant
	IndexMeetingRegistrantSubject = "lfx.index.meeting_registrant"

	// IndexPastMeetingSubject is the subject for the past meeting indexing.
	// The subject is of the form: lfx.index.past_meeting
	IndexPastMeetingSubject = "lfx.index.past_meeting"

	// IndexPastMeetingParticipantSubject is the subject for the past meeting participant indexing.
	// The subject is of the form: lfx.index.past_meeting_participant
	IndexPastMeetingParticipantSubject = "lfx.index.past_meeting_participant"

	// IndexPastMeetingRecordingSubject is the subject for the past meeting recording indexing.
	// The subject is of the form: lfx.index.past_meeting_recording
	IndexPastMeetingRecordingSubject = "lfx.index.past_meeting_recording"

	// IndexPastMeetingSummarySubject is the subject for the past meeting summary indexing.
	// The subject is of the form: lfx.index.past_meeting_summary
	IndexPastMeetingSummarySubject = "lfx.index.past_meeting_summary"

	// UpdateAccessMeetingSubject is the subject for the meeting access control updates.
	// The subject is of the form: lfx.update_access.meeting
	UpdateAccessMeetingSubject = "lfx.update_access.meeting"

	// DeleteAllAccessMeetingSubject is the subject for the meeting access control deletion.
	// The subject is of the form: lfx.delete_all_access.meeting
	DeleteAllAccessMeetingSubject = "lfx.delete_all_access.meeting"

	// PutRegistrantMeetingSubject is the subject for the meeting registrant access control updates.
	// The subject is of the form: lfx.put_registrant.meeting
	PutRegistrantMeetingSubject = "lfx.put_registrant.meeting"

	// RemoveRegistrantMeetingSubject is the subject for the meeting registrant access control deletion.
	// The subject is of the form: lfx.remove_registrant.meeting
	RemoveRegistrantMeetingSubject = "lfx.remove_registrant.meeting"

	// UpdateAccessPastMeetingSubject is the subject for the past meeting access control updates.
	// The subject is of the form: lfx.update_access.past_meeting
	UpdateAccessPastMeetingSubject = "lfx.update_access.past_meeting"

	// DeleteAllAccessPastMeetingSubject is the subject for the past meeting access control deletion.
	// The subject is of the form: lfx.delete_all_access.past_meeting
	DeleteAllAccessPastMeetingSubject = "lfx.delete_all_access.past_meeting"

	// PutParticipantPastMeetingSubject is the subject for the past meeting participant access control updates.
	// The subject is of the form: lfx.put_participant.past_meeting
	PutParticipantPastMeetingSubject = "lfx.put_participant.past_meeting"

	// RemoveParticipantPastMeetingSubject is the subject for the past meeting participant access control deletion.
	// The subject is of the form: lfx.remove_participant.past_meeting
	RemoveParticipantPastMeetingSubject = "lfx.remove_participant.past_meeting"
)

// NATS subjects that external services handle and that the meeting service requests.
const (
	// CommitteeGetNameSubject is the subject for committee name validation.
	// The subject is of the form: lfx.committee-api.get_name
	CommitteeGetNameSubject = "lfx.committee-api.get_name"

	// CommitteeListMembersSubject is the subject for fetching committee members.
	// The subject is of the form: lfx.committee-api.list_members
	CommitteeListMembersSubject = "lfx.committee-api.list_members"

	// CommitteeMemberCreatedSubject is the subject for committee member creation events.
	// The subject is of the form: lfx.committee-api.committee_member.created
	CommitteeMemberCreatedSubject = "lfx.committee-api.committee_member.created"

	// CommitteeMemberDeletedSubject is the subject for committee member deletion events.
	// The subject is of the form: lfx.committee-api.committee_member.deleted
	CommitteeMemberDeletedSubject = "lfx.committee-api.committee_member.deleted"

	// ProjectGetNameSubject is the subject for project name validation.
	// The subject is of the form: lfx.projects-api.get_name
	ProjectGetNameSubject = "lfx.projects-api.get_name"
)

// NATS wildcard subjects that the meeting service handles messages about.
const (
	// MeetingsAPIQueue is the subject name for the meetings API.
	// The subject is of the form: lfx.meetings-api.queue
	MeetingsAPIQueue = "lfx.meetings-api.queue"
)

// NATS specific subjects that the meeting service handles messages about.
const (
	// MeetingGetTitleSubject is the subject for the meeting get title.
	// The subject is of the form: lfx.meetings-api.get_title
	MeetingGetTitleSubject = "lfx.meetings-api.get_title"

	// MeetingDeletedSubject is the subject for meeting deletion events.
	// The subject is of the form: lfx.meetings-api.meeting_deleted
	MeetingDeletedSubject = "lfx.meetings-api.meeting_deleted"

	// MeetingCreatedSubject is the subject for meeting creation events.
	// The subject is of the form: lfx.meetings-api.meeting_created
	MeetingCreatedSubject = "lfx.meetings-api.meeting_created"

	// MeetingUpdatedSubject is the subject for meeting update events.
	// The subject is of the form: lfx.meetings-api.meeting_updated
	MeetingUpdatedSubject = "lfx.meetings-api.meeting_updated"

	// Zoom webhook event subjects - mirrors the actual Zoom webhook event names
	ZoomWebhookMeetingStartedSubject               = "lfx.webhook.zoom.meeting.started"
	ZoomWebhookMeetingEndedSubject                 = "lfx.webhook.zoom.meeting.ended"
	ZoomWebhookMeetingDeletedSubject               = "lfx.webhook.zoom.meeting.deleted"
	ZoomWebhookMeetingParticipantJoinedSubject     = "lfx.webhook.zoom.meeting.participant_joined"
	ZoomWebhookMeetingParticipantLeftSubject       = "lfx.webhook.zoom.meeting.participant_left"
	ZoomWebhookRecordingCompletedSubject           = "lfx.webhook.zoom.recording.completed"
	ZoomWebhookRecordingTranscriptCompletedSubject = "lfx.webhook.zoom.recording.transcript_completed"
	ZoomWebhookMeetingSummaryCompletedSubject      = "lfx.webhook.zoom.meeting.summary_completed"
)

// MessageAction is a type for the action of a meeting message.
type MessageAction string

// MessageAction constants for the action of a meeting message.
const (
	// ActionCreated is the action for a resource creation message.
	ActionCreated MessageAction = "created"
	// ActionUpdated is the action for a resource update message.
	ActionUpdated MessageAction = "updated"
	// ActionDeleted is the action for a resource deletion message.
	ActionDeleted MessageAction = "deleted"
)

// MeetingIndexerMessage is a NATS message schema for sending messages related to meetings CRUD operations.
type MeetingIndexerMessage struct {
	Action  MessageAction     `json:"action"`
	Headers map[string]string `json:"headers"`
	Data    any               `json:"data"`
	// Tags is a list of tags to be set on the indexed resource for search.
	Tags []string `json:"tags"`
}

// MeetingAccessMessage is the schema for the data in the message sent to the fga-sync service.
// These are the fields that the fga-sync service needs in order to update the OpenFGA permissions.
type MeetingAccessMessage struct {
	UID        string   `json:"uid"`
	Public     bool     `json:"public"`
	ProjectUID string   `json:"project_uid"`
	Organizers []string `json:"organizers"`
	Committees []string `json:"committees"`
}

// MeetingRegistrantAccessMessage is the schema for the data in the message sent to the fga-sync service.
// These are the fields that the fga-sync service needs in order to update the OpenFGA permissions.
type MeetingRegistrantAccessMessage struct {
	UID        string `json:"uid"`
	MeetingUID string `json:"meeting_uid"`
	Username   string `json:"username"`
	Host       bool   `json:"host"`
}

// MeetingDeletedMessage is the schema for the message sent when a meeting is deleted.
// This message is used internally to trigger cleanup of all associated registrants.
type MeetingDeletedMessage struct {
	MeetingUID string `json:"meeting_uid"`
}

// MeetingCreatedMessage is the schema for the message sent when a meeting is created.
// This message is used internally to trigger post-creation tasks like committee member sync.
type MeetingCreatedMessage struct {
	MeetingUID string           `json:"meeting_uid"`
	Base       *MeetingBase     `json:"base"`
	Settings   *MeetingSettings `json:"settings"`
}

// MeetingUpdatedMessage is the schema for the message sent when a meeting is updated.
// This message is used internally to trigger post-update tasks like committee member sync changes.
type MeetingUpdatedMessage struct {
	MeetingUID   string           `json:"meeting_uid"`
	UpdatedBase  *MeetingBase     `json:"updated_base"`
	PreviousBase *MeetingBase     `json:"previous_base"`
	Settings     *MeetingSettings `json:"settings"`
	Changes      map[string]any   `json:"changes"` // Map of field names to their new values
}

// CommitteeEvent represents a generic event emitted for committee service operations
type CommitteeEvent struct {
	// EventType identifies the type of event (e.g., committee_member.created)
	EventType string `json:"event_type"`
	// Subject is the subject of the event (e.g. lfx.committee-api.committee_member.created)
	Subject string `json:"subject"`
	// Timestamp is when the event occurred
	Timestamp time.Time `json:"timestamp"`
	// Version is the event schema version
	Version string `json:"version"`
	// Data contains the event data
	Data any `json:"data,omitempty"`
}

// CommitteeMemberBase represents the base committee member attributes
type CommitteeMember struct {
	UID           string                      `json:"uid"`
	Username      string                      `json:"username"`
	Email         string                      `json:"email"`
	FirstName     string                      `json:"first_name"`
	LastName      string                      `json:"last_name"`
	JobTitle      string                      `json:"job_title,omitempty"`
	Role          CommitteeMemberRole         `json:"role"`
	AppointedBy   string                      `json:"appointed_by"`
	Status        string                      `json:"status"`
	Voting        CommitteeMemberVotingInfo   `json:"voting"`
	Agency        string                      `json:"agency,omitempty"`
	Country       string                      `json:"country,omitempty"`
	Organization  CommitteeMemberOrganization `json:"organization"`
	CommitteeUID  string                      `json:"committee_uid"`
	CommitteeName string                      `json:"committee_name"`
	CreatedAt     time.Time                   `json:"created_at"`
	UpdatedAt     time.Time                   `json:"updated_at"`
}

// Role represents committee role information
type CommitteeMemberRole struct {
	Name      string `json:"name"`
	StartDate string `json:"start_date,omitempty"`
	EndDate   string `json:"end_date,omitempty"`
}

// VotingInfo represents voting information for the committee member
type CommitteeMemberVotingInfo struct {
	Status    string `json:"status"`
	StartDate string `json:"start_date,omitempty"`
	EndDate   string `json:"end_date,omitempty"`
}

// Organization represents organization information for the committee member
type CommitteeMemberOrganization struct {
	Name    string `json:"name"`
	Website string `json:"website,omitempty"`
}

// PastMeetingAccessMessage is the schema for the data in the message sent to the fga-sync service.
// These are the fields that the fga-sync service needs in order to update the OpenFGA permissions.
// Past meetings don't have organizers, but they have a reference to the original meeting.
type PastMeetingAccessMessage struct {
	UID        string   `json:"uid"`
	MeetingUID string   `json:"meeting_uid"`
	Public     bool     `json:"public"`
	ProjectUID string   `json:"project_uid"`
	Committees []string `json:"committees"`
}

// PastMeetingParticipantAccessMessage is the schema for the data in the message sent to the fga-sync service.
// These are the fields that the fga-sync service needs in order to update the OpenFGA permissions.
type PastMeetingParticipantAccessMessage struct {
	UID            string `json:"uid"`
	PastMeetingUID string `json:"past_meeting_uid"`
	Username       string `json:"username"`
	Host           bool   `json:"host"`
	IsInvited      bool   `json:"is_invited"`
	IsAttended     bool   `json:"is_attended"`
}

// ZoomWebhookEventMessage is the schema for Zoom webhook events sent via NATS for async processing.
// This maintains backward compatibility while new handlers can use the typed payload structs.
type ZoomWebhookEventMessage struct {
	EventType string                 `json:"event_type"`
	EventTS   int64                  `json:"event_ts"`
	Payload   map[string]interface{} `json:"payload"`
}
