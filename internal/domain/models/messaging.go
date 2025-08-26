// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package models

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
}

// ZoomWebhookEventMessage is the schema for Zoom webhook events sent via NATS for async processing.
// This maintains backward compatibility while new handlers can use the typed payload structs.
type ZoomWebhookEventMessage struct {
	EventType string                 `json:"event_type"`
	EventTS   int64                  `json:"event_ts"`
	Payload   map[string]interface{} `json:"payload"`
}
