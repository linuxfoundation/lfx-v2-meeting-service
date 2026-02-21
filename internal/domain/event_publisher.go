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

	// Close closes the publisher and releases resources
	Close() error
}
