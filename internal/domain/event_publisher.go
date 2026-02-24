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
	PublishMeetingEvent(ctx context.Context, action string, meeting *models.MeetingEventData, tags []string) error
	PublishRegistrantEvent(ctx context.Context, action string, registrant *models.RegistrantEventData, tags []string) error
	PublishInviteResponseEvent(ctx context.Context, action string, response *models.InviteResponseEventData, tags []string) error

	// Past meeting events
	PublishPastMeetingEvent(ctx context.Context, action string, meeting *models.PastMeetingEventData, tags []string) error
	PublishPastMeetingParticipantEvent(ctx context.Context, action string, participant *models.PastMeetingParticipantEventData, tags []string) error
	PublishPastMeetingRecordingEvent(ctx context.Context, action string, recording *models.RecordingEventData, tags []string) error
	PublishPastMeetingTranscriptEvent(ctx context.Context, action string, transcript *models.TranscriptEventData, tags []string) error
	PublishPastMeetingSummaryEvent(ctx context.Context, action string, summary *models.SummaryEventData, tags []string) error

	// Close closes the publisher and releases resources
	Close() error
}
