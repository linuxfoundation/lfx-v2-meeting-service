// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package models

// NATS subjects that the meeting service sends messages about.
const (
	// IndexMeetingSubject is the subject for the meeting indexing.
	// The subject is of the form: lfx.index.meeting
	IndexMeetingSubject = "lfx.index.meeting"

	// UpdateAccessMeetingSubject is the subject for the meeting access control updates.
	// The subject is of the form: lfx.update_access.meeting
	UpdateAccessMeetingSubject = "lfx.update_access.meeting"

	// DeleteAllAccessMeetingSubject is the subject for the meeting access control deletion.
	// The subject is of the form: lfx.delete_all_access.meeting
	DeleteAllAccessMeetingSubject = "lfx.delete_all_access.meeting"
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
	UID       string   `json:"uid"`
	Public    bool     `json:"public"`
	ParentUID string   `json:"parent_uid"`
	Writers   []string `json:"writers"`
	Auditors  []string `json:"auditors"`
}
