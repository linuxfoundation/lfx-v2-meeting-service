// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package domain

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
)

// Message represents a domain message interface
type Message interface {
	Subject() string
	Data() []byte
	Respond(data []byte) error
	HasReply() bool
}

// MessageHandler defines how the service handles incoming messages
type MessageHandler interface {
	HandleMessage(ctx context.Context, msg Message)
	HandlerReady() bool
}

// MeetingIndexSender handles indexing operations for meetings.
type MeetingIndexSender interface {
	SendIndexMeeting(ctx context.Context, action models.MessageAction, data models.MeetingBase, sync bool) error
	SendDeleteIndexMeeting(ctx context.Context, data string, sync bool) error
}

// MeetingSettingsIndexSender handles indexing operations for meeting settings.
type MeetingSettingsIndexSender interface {
	SendIndexMeetingSettings(ctx context.Context, action models.MessageAction, data models.MeetingSettings, sync bool) error
	SendDeleteIndexMeetingSettings(ctx context.Context, data string, sync bool) error
}

// MeetingRegistrantIndexSender handles indexing operations for meeting registrants.
type MeetingRegistrantIndexSender interface {
	SendIndexMeetingRegistrant(ctx context.Context, action models.MessageAction, data models.Registrant, sync bool) error
	SendDeleteIndexMeetingRegistrant(ctx context.Context, data string, sync bool) error
}

// MeetingRSVPIndexSender handles indexing operations for meeting RSVPs.
type MeetingRSVPIndexSender interface {
	SendIndexMeetingRSVP(ctx context.Context, action models.MessageAction, data models.RSVPResponse, sync bool) error
	SendDeleteIndexMeetingRSVP(ctx context.Context, data string, sync bool) error
}

// PastMeetingIndexSender handles indexing operations for past meetings.
type PastMeetingIndexSender interface {
	SendIndexPastMeeting(ctx context.Context, action models.MessageAction, data models.PastMeeting, sync bool) error
	SendDeleteIndexPastMeeting(ctx context.Context, data string, sync bool) error
}

// PastMeetingParticipantIndexSender handles indexing operations for past meeting participants.
type PastMeetingParticipantIndexSender interface {
	SendIndexPastMeetingParticipant(ctx context.Context, action models.MessageAction, data models.PastMeetingParticipant, sync bool) error
	SendDeleteIndexPastMeetingParticipant(ctx context.Context, data string, sync bool) error
}

// PastMeetingRecordingIndexSender handles indexing operations for past meeting recordings.
type PastMeetingRecordingIndexSender interface {
	SendIndexPastMeetingRecording(ctx context.Context, action models.MessageAction, data models.PastMeetingRecording, sync bool) error
	SendDeleteIndexPastMeetingRecording(ctx context.Context, data string, sync bool) error
}

// PastMeetingTranscriptIndexSender handles indexing operations for past meeting transcripts.
type PastMeetingTranscriptIndexSender interface {
	SendIndexPastMeetingTranscript(ctx context.Context, action models.MessageAction, data models.PastMeetingTranscript, sync bool) error
	SendDeleteIndexPastMeetingTranscript(ctx context.Context, data string, sync bool) error
}

// PastMeetingSummaryIndexSender handles indexing operations for past meeting summaries.
type PastMeetingSummaryIndexSender interface {
	SendIndexPastMeetingSummary(ctx context.Context, action models.MessageAction, data models.PastMeetingSummary, sync bool) error
	SendDeleteIndexPastMeetingSummary(ctx context.Context, data string, sync bool) error
}

// MeetingAttachmentIndexSender handles indexing operations for meeting attachments.
type MeetingAttachmentIndexSender interface {
	SendIndexMeetingAttachment(ctx context.Context, action models.MessageAction, data models.MeetingAttachment, sync bool) error
	SendDeleteIndexMeetingAttachment(ctx context.Context, data string, sync bool) error
}

// PastMeetingAttachmentIndexSender handles indexing operations for past meeting attachments.
type PastMeetingAttachmentIndexSender interface {
	SendIndexPastMeetingAttachment(ctx context.Context, action models.MessageAction, data models.PastMeetingAttachment, sync bool) error
	SendDeleteIndexPastMeetingAttachment(ctx context.Context, data string, sync bool) error
}

// MeetingAccessSender handles access control operations for meetings.
type MeetingAccessSender interface {
	SendUpdateAccessMeeting(ctx context.Context, data models.MeetingAccessMessage, sync bool) error
	SendDeleteAllAccessMeeting(ctx context.Context, data string, sync bool) error
}

// MeetingAttachmentAccessSender handles access control operations for meeting attachments.
type MeetingAttachmentAccessSender interface {
	SendUpdateAccessMeetingAttachment(ctx context.Context, data models.MeetingAttachmentAccessMessage, sync bool) error
	SendDeleteAccessMeetingAttachment(ctx context.Context, data string, sync bool) error
}

// MeetingRegistrantAccessSender handles access control operations for meeting registrants.
type MeetingRegistrantAccessSender interface {
	SendPutMeetingRegistrantAccess(ctx context.Context, data models.MeetingRegistrantAccessMessage, sync bool) error
	SendRemoveMeetingRegistrantAccess(ctx context.Context, data models.MeetingRegistrantAccessMessage, sync bool) error
}

// PastMeetingAccessSender handles access control operations for past meetings.
type PastMeetingAccessSender interface {
	SendUpdateAccessPastMeeting(ctx context.Context, data models.PastMeetingAccessMessage, sync bool) error
	SendDeleteAllAccessPastMeeting(ctx context.Context, data string, sync bool) error
}

// PastMeetingAttachmentAccessSender handles access control operations for past meeting attachments.
type PastMeetingAttachmentAccessSender interface {
	SendUpdateAccessPastMeetingAttachment(ctx context.Context, data models.PastMeetingAttachmentAccessMessage, sync bool) error
	SendDeleteAccessPastMeetingAttachment(ctx context.Context, data string, sync bool) error
}

// PastMeetingRecordingAccessSender handles access control operations for past meeting recordings.
type PastMeetingRecordingAccessSender interface {
	SendUpdateAccessPastMeetingRecording(ctx context.Context, data models.PastMeetingRecordingAccessMessage, sync bool) error
}

// PastMeetingTranscriptAccessSender handles access control operations for past meeting transcripts.
type PastMeetingTranscriptAccessSender interface {
	SendUpdateAccessPastMeetingTranscript(ctx context.Context, data models.PastMeetingTranscriptAccessMessage, sync bool) error
}

// PastMeetingSummaryAccessSender handles access control operations for past meeting summaries.
type PastMeetingSummaryAccessSender interface {
	SendUpdateAccessPastMeetingSummary(ctx context.Context, data models.PastMeetingSummaryAccessMessage, sync bool) error
}

// PastMeetingParticipantAccessSender handles access control operations for past meeting participants.
type PastMeetingParticipantAccessSender interface {
	SendPutPastMeetingParticipantAccess(ctx context.Context, data models.PastMeetingParticipantAccessMessage, sync bool) error
	SendRemovePastMeetingParticipantAccess(ctx context.Context, data models.PastMeetingParticipantAccessMessage, sync bool) error
}

// MeetingEventSender handles meeting lifecycle events.
type MeetingEventSender interface {
	SendMeetingDeleted(ctx context.Context, data models.MeetingDeletedMessage) error
	SendMeetingCreated(ctx context.Context, data models.MeetingCreatedMessage) error
	SendMeetingUpdated(ctx context.Context, data models.MeetingUpdatedMessage) error
}

// WebhookEventSender handles webhook event publishing.
type WebhookEventSender interface {
	PublishZoomWebhookEvent(ctx context.Context, subject string, message models.ZoomWebhookEventMessage) error
}

// ExternalServiceClient handles requests to external services.
type ExternalServiceClient interface {
	GetCommitteeName(ctx context.Context, committeeUID string) (string, error)
	GetCommitteeMembers(ctx context.Context, committeeUID string) ([]models.CommitteeMember, error)
	GetProjectName(ctx context.Context, projectUID string) (string, error)
	GetProjectLogo(ctx context.Context, projectUID string) (string, error)
	GetProjectSlug(ctx context.Context, projectUID string) (string, error)
	EmailToUsernameLookup(ctx context.Context, email string) (string, error)
}

// MeetingMessageSender composes all meeting-related messaging operations.
// Use this for services that manage meetings and their settings.
type MeetingMessageSender interface {
	MeetingIndexSender
	MeetingSettingsIndexSender
	MeetingAccessSender
	MeetingEventSender
}

// MeetingRegistrantMessageSender composes messaging operations for registrants and RSVPs.
// Use this for services that manage meeting registrants.
type MeetingRegistrantMessageSender interface {
	MeetingRegistrantIndexSender
	MeetingRegistrantAccessSender
	MeetingRSVPIndexSender
}

// PastMeetingBasicMessageSender composes messaging operations for past meetings only.
// Use this for services that manage past meeting records.
type PastMeetingBasicMessageSender interface {
	PastMeetingIndexSender
	PastMeetingAccessSender
}

// PastMeetingParticipantMessageSender composes messaging operations for past meeting participants.
// Use this for services that manage participant records.
type PastMeetingParticipantMessageSender interface {
	PastMeetingParticipantIndexSender
	PastMeetingParticipantAccessSender
}

// PastMeetingRecordingMessageSender composes messaging operations for past meeting recordings.
// Use this for services that manage recording records.
type PastMeetingRecordingMessageSender interface {
	PastMeetingRecordingIndexSender
	PastMeetingRecordingAccessSender
}

// PastMeetingTranscriptMessageSender composes messaging operations for past meeting transcripts.
// Use this for services that manage transcript records.
type PastMeetingTranscriptMessageSender interface {
	PastMeetingTranscriptIndexSender
	PastMeetingTranscriptAccessSender
}

// PastMeetingSummaryMessageSender composes messaging operations for past meeting summaries.
// Use this for services that manage summary records.
type PastMeetingSummaryMessageSender interface {
	PastMeetingSummaryIndexSender
	PastMeetingSummaryAccessSender
}

// PastMeetingMessageSender composes all past meeting-related messaging operations.
// Use this for services that manage past meetings and all their associated data.
type PastMeetingMessageSender interface {
	PastMeetingIndexSender
	PastMeetingParticipantIndexSender
	PastMeetingRecordingIndexSender
	PastMeetingTranscriptIndexSender
	PastMeetingSummaryIndexSender
	PastMeetingAccessSender
	PastMeetingRecordingAccessSender
	PastMeetingTranscriptAccessSender
	PastMeetingSummaryAccessSender
	PastMeetingParticipantAccessSender
}

// MessageBuilder is the main interface that composes all messaging capabilities.
// Use this when a service needs access to multiple different domains.
type MessageBuilder interface {
	MeetingMessageSender
	MeetingRegistrantMessageSender
	PastMeetingMessageSender
	WebhookEventSender
	ExternalServiceClient
}
