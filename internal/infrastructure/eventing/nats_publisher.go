// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/nats-io/nats.go"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
)

// NATSPublisher implements the EventPublisher interface using NATS JetStream
type NATSPublisher struct {
	nc     *nats.Conn
	js     nats.JetStreamContext
	logger *slog.Logger
}

// NewNATSPublisher creates a new NATS event publisher
func NewNATSPublisher(nc *nats.Conn, logger *slog.Logger) (*NATSPublisher, error) {
	js, err := nc.JetStream()
	if err != nil {
		return nil, fmt.Errorf("failed to get jetstream context: %w", err)
	}

	return &NATSPublisher{
		nc:     nc,
		js:     js,
		logger: logger,
	}, nil
}

// PublishMeetingEvent publishes a meeting event to indexer and FGA-sync services
func (p *NATSPublisher) PublishMeetingEvent(ctx context.Context, action string, meeting *models.MeetingEventData, tags []string) error {
	p.logger.InfoContext(ctx, "publishing meeting event", "action", action, "meeting_id", "TBD")
	// TODO: Implement when MeetingEventData is populated
	return fmt.Errorf("not yet implemented")
}

// PublishRegistrantEvent publishes a registrant event to indexer and FGA-sync services
func (p *NATSPublisher) PublishRegistrantEvent(ctx context.Context, action string, registrant *models.RegistrantEventData, tags []string) error {
	p.logger.InfoContext(ctx, "publishing registrant event", "action", action)
	// TODO: Implement when RegistrantEventData is populated
	return fmt.Errorf("not yet implemented")
}

// PublishInviteResponseEvent publishes an invite response (RSVP) event to indexer service
func (p *NATSPublisher) PublishInviteResponseEvent(ctx context.Context, action string, response *models.InviteResponseEventData, tags []string) error {
	p.logger.InfoContext(ctx, "publishing invite response event", "action", action)
	// TODO: Implement when InviteResponseEventData is populated
	return fmt.Errorf("not yet implemented")
}

// PublishPastMeetingEvent publishes a past meeting event to indexer and FGA-sync services
func (p *NATSPublisher) PublishPastMeetingEvent(ctx context.Context, action string, meeting *models.PastMeetingEventData, tags []string) error {
	p.logger.InfoContext(ctx, "publishing past meeting event", "action", action)
	// TODO: Implement when PastMeetingEventData is populated
	return fmt.Errorf("not yet implemented")
}

// PublishPastMeetingParticipantEvent publishes a participant event to indexer and FGA-sync services
func (p *NATSPublisher) PublishPastMeetingParticipantEvent(ctx context.Context, action string, participant *models.PastMeetingParticipantEventData, tags []string) error {
	p.logger.InfoContext(ctx, "publishing past meeting participant event", "action", action)
	// TODO: Implement when PastMeetingParticipantEventData is populated
	return fmt.Errorf("not yet implemented")
}

// PublishPastMeetingRecordingEvent publishes a recording event to indexer and FGA-sync services
func (p *NATSPublisher) PublishPastMeetingRecordingEvent(ctx context.Context, action string, recording *models.RecordingEventData, tags []string) error {
	p.logger.InfoContext(ctx, "publishing past meeting recording event", "action", action)
	// TODO: Implement when RecordingEventData is populated
	return fmt.Errorf("not yet implemented")
}

// PublishPastMeetingTranscriptEvent publishes a transcript event to indexer and FGA-sync services
func (p *NATSPublisher) PublishPastMeetingTranscriptEvent(ctx context.Context, action string, transcript *models.TranscriptEventData, tags []string) error {
	p.logger.InfoContext(ctx, "publishing past meeting transcript event", "action", action)
	// TODO: Implement when TranscriptEventData is populated
	return fmt.Errorf("not yet implemented")
}

// PublishPastMeetingSummaryEvent publishes a summary event to indexer and FGA-sync services
func (p *NATSPublisher) PublishPastMeetingSummaryEvent(ctx context.Context, action string, summary *models.SummaryEventData, tags []string) error {
	p.logger.InfoContext(ctx, "publishing past meeting summary event", "action", action)
	// TODO: Implement when SummaryEventData is populated
	return fmt.Errorf("not yet implemented")
}

// publish is a helper method to publish a message to a subject
func (p *NATSPublisher) publish(ctx context.Context, subject string, data interface{}) error {
	payload, err := json.Marshal(data)
	if err != nil {
		p.logger.With(logging.ErrKey, err).ErrorContext(ctx, "failed to marshal event data", "subject", subject)
		return fmt.Errorf("failed to marshal event data: %w", err)
	}

	if _, err := p.js.Publish(subject, payload); err != nil {
		p.logger.With(logging.ErrKey, err).ErrorContext(ctx, "failed to publish event", "subject", subject)
		return fmt.Errorf("failed to publish to %s: %w", subject, err)
	}

	p.logger.InfoContext(ctx, "successfully published event", "subject", subject)
	return nil
}

// Close closes the NATS publisher and releases resources
func (p *NATSPublisher) Close() error {
	// NATS connection is managed externally, so we don't close it here
	return nil
}

// Ensure NATSPublisher implements EventPublisher
var _ domain.EventPublisher = (*NATSPublisher)(nil)
