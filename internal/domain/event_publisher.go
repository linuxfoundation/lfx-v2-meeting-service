// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package domain

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
)

// EventPublisher defines the interface for publishing meeting events to downstream services
type EventPublisher interface {
	// Active meeting events
	PublishMeetingEvent(ctx context.Context, action string, meeting *models.MeetingEventData) error
	PublishRegistrantEvent(ctx context.Context, action string, registrant *models.RegistrantEventData) error
	PublishInviteResponseEvent(ctx context.Context, action string, response *models.InviteResponseEventData) error

	// Past meeting events
	PublishPastMeetingEvent(ctx context.Context, action string, meeting *models.PastMeetingEventData) error
	PublishPastMeetingParticipantEvent(ctx context.Context, action string, participant *models.PastMeetingParticipantEventData) error
	PublishPastMeetingRecordingEvent(ctx context.Context, action string, recording *models.RecordingEventData) error
	PublishPastMeetingTranscriptEvent(ctx context.Context, action string, transcript *models.TranscriptEventData) error
	PublishPastMeetingSummaryEvent(ctx context.Context, action string, summary *models.SummaryEventData) error

	// Attachment events
	PublishMeetingAttachmentEvent(ctx context.Context, action string, attachment *models.MeetingAttachmentEventData) error
	PublishPastMeetingAttachmentEvent(ctx context.Context, action string, attachment *models.PastMeetingAttachmentEventData) error

	// PublishIndexerDelete sends a "deleted" indexer message for the given resource ID to subject.
	PublishIndexerDelete(ctx context.Context, subject, id string) error
	// PublishAccessDelete sends a pre-built access control message payload to subject.
	// The caller is responsible for marshalling the payload; pass []byte(id) for simple deletes.
	PublishAccessDelete(ctx context.Context, subject string, payload []byte) error

	// Close closes the publisher and releases resources
	Close() error
}
