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

// MessageAction represents the type of action performed on an object
type MessageAction string

const (
	ActionCreated MessageAction = "created"
	ActionUpdated MessageAction = "updated"
	ActionDeleted MessageAction = "deleted"

	authorizationHeaderValue = "Bearer lfx-v2-meeting-service"
)

// IndexerMessage is the structure for indexer messages
type IndexerMessage struct {
	Action  MessageAction     `json:"action"`
	Headers map[string]string `json:"headers"`
	Data    interface{}       `json:"data"`
	Tags    []string          `json:"tags"`
}

// GenericFGAMessage is the universal message format for all FGA operations
type GenericFGAMessage struct {
	ObjectType string                 `json:"object_type"` // e.g., "v1_meeting", "v1_past_meeting"
	Operation  string                 `json:"operation"`   // e.g., "update_access", "member_put", "member_remove"
	Data       map[string]interface{} `json:"data"`        // Operation-specific payload
}

// GenericAccessData represents the data field for update_access operations
type GenericAccessData struct {
	UID              string              `json:"uid"`
	Public           bool                `json:"public"`
	Relations        map[string][]string `json:"relations"`         // relation_name → [usernames]
	References       map[string][]string `json:"references"`        // relation_name → [object_uids]
	ExcludeRelations []string            `json:"exclude_relations"` // Optional: relations managed elsewhere
}

// GenericDeleteData represents the data field for delete_access operations
type GenericDeleteData struct {
	UID string `json:"uid"`
}

// GenericMemberData represents the data field for member_put/member_remove operations
type GenericMemberData struct {
	UID                   string   `json:"uid"`
	Username              string   `json:"username"`
	Relations             []string `json:"relations"`               // Multiple relations supported
	MutuallyExclusiveWith []string `json:"mutually_exclusive_with"` // Optional: auto-remove these
}

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
	p.logger.InfoContext(ctx, "publishing meeting event", "action", action, "meeting_id", meeting.ID)

	// Publish to indexer
	headers := make(map[string]string)
	headers["authorization"] = authorizationHeaderValue
	indexerMsg := IndexerMessage{
		Action:  MessageAction(action),
		Headers: headers,
		Data:    meeting,
		Tags:    tags,
	}

	if err := p.publish(ctx, "lfx.index.v1_meeting", indexerMsg); err != nil {
		return fmt.Errorf("failed to publish meeting to indexer: %w", err)
	}

	// Publish access control message using generic FGA format
	relations := map[string][]string{}
	references := map[string][]string{}

	if meeting.ProjectUID != "" {
		references["project"] = []string{meeting.ProjectUID}
	}

	// Add committee references if present
	if len(meeting.Committees) > 0 {
		committeeUIDs := make([]string, 0, len(meeting.Committees))
		for _, committee := range meeting.Committees {
			if committee.UID != "" {
				committeeUIDs = append(committeeUIDs, committee.UID)
			}
		}
		if len(committeeUIDs) > 0 {
			references["committee"] = committeeUIDs
		}
	}

	accessMsg := GenericFGAMessage{
		ObjectType: "v1_meeting",
		Operation:  "update_access",
		Data: map[string]interface{}{
			"uid":        meeting.ID,
			"public":     meeting.Visibility == "public",
			"relations":  relations,
			"references": references,
		},
	}

	if err := p.publish(ctx, "lfx.fga-sync.update_access", accessMsg); err != nil {
		return fmt.Errorf("failed to publish meeting access control: %w", err)
	}

	return nil
}

// PublishRegistrantEvent publishes a registrant event to indexer and FGA-sync services
func (p *NATSPublisher) PublishRegistrantEvent(ctx context.Context, action string, registrant *models.RegistrantEventData, tags []string) error {
	p.logger.InfoContext(ctx, "publishing registrant event", "action", action, "registrant_uid", registrant.UID)

	// Publish to indexer
	headers := make(map[string]string)
	headers["authorization"] = authorizationHeaderValue
	indexerMsg := IndexerMessage{
		Action:  MessageAction(action),
		Headers: headers,
		Data:    registrant,
		Tags:    tags,
	}

	if err := p.publish(ctx, "lfx.index.v1_meeting_registrant", indexerMsg); err != nil {
		return fmt.Errorf("failed to publish registrant to indexer: %w", err)
	}

	// If registrant has username (authenticated user), publish access control
	if registrant.Username != "" {
		relations := []string{"registrant"}
		if registrant.Host {
			relations = append(relations, "host")
		}

		memberMsg := GenericFGAMessage{
			ObjectType: "v1_meeting",
			Operation:  "member_put",
			Data: map[string]interface{}{
				"uid":       registrant.MeetingID,
				"username":  registrant.Username,
				"relations": relations,
			},
		}

		if err := p.publish(ctx, "lfx.fga-sync.update_access", memberMsg); err != nil {
			return fmt.Errorf("failed to publish registrant access control: %w", err)
		}
	}

	return nil
}

// PublishInviteResponseEvent publishes an invite response (RSVP) event to indexer service
func (p *NATSPublisher) PublishInviteResponseEvent(ctx context.Context, action string, response *models.InviteResponseEventData, tags []string) error {
	p.logger.InfoContext(ctx, "publishing invite response event", "action", action, "response_id", response.ID)

	// RSVPs only go to indexer, not access control
	headers := make(map[string]string)
	headers["authorization"] = authorizationHeaderValue
	indexerMsg := IndexerMessage{
		Action:  MessageAction(action),
		Headers: headers,
		Data:    response,
		Tags:    tags,
	}

	if err := p.publish(ctx, "lfx.index.v1_meeting_rsvp", indexerMsg); err != nil {
		return fmt.Errorf("failed to publish invite response to indexer: %w", err)
	}

	return nil
}

// PublishPastMeetingEvent publishes a past meeting event to indexer and FGA-sync services
func (p *NATSPublisher) PublishPastMeetingEvent(ctx context.Context, action string, meeting *models.PastMeetingEventData, tags []string) error {
	p.logger.InfoContext(ctx, "publishing past meeting event", "action", action, "past_meeting_id", meeting.ID)

	// Publish to indexer
	headers := make(map[string]string)
	headers["authorization"] = authorizationHeaderValue
	indexerMsg := IndexerMessage{
		Action:  MessageAction(action),
		Headers: headers,
		Data:    meeting,
		Tags:    tags,
	}

	if err := p.publish(ctx, "lfx.index.v1_past_meeting", indexerMsg); err != nil {
		return fmt.Errorf("failed to publish past meeting to indexer: %w", err)
	}

	// Publish access control message using generic FGA format
	relations := map[string][]string{}
	references := map[string][]string{}

	if meeting.ProjectUID != "" {
		references["project"] = []string{meeting.ProjectUID}
	}

	accessMsg := GenericFGAMessage{
		ObjectType: "v1_past_meeting",
		Operation:  "update_access",
		Data: map[string]interface{}{
			"uid":        meeting.ID,
			"public":     false, // Past meetings are not public
			"relations":  relations,
			"references": references,
		},
	}

	if err := p.publish(ctx, "lfx.fga-sync.update_access", accessMsg); err != nil {
		return fmt.Errorf("failed to publish past meeting access control: %w", err)
	}

	return nil
}

// PublishPastMeetingParticipantEvent publishes a participant event to indexer and FGA-sync services
func (p *NATSPublisher) PublishPastMeetingParticipantEvent(ctx context.Context, action string, participant *models.PastMeetingParticipantEventData, tags []string) error {
	p.logger.InfoContext(ctx, "publishing past meeting participant event", "action", action, "participant_uid", participant.UID)

	// Publish to indexer
	headers := make(map[string]string)
	headers["authorization"] = authorizationHeaderValue
	indexerMsg := IndexerMessage{
		Action:  MessageAction(action),
		Headers: headers,
		Data:    participant,
		Tags:    tags,
	}

	if err := p.publish(ctx, "lfx.index.v1_past_meeting_participant", indexerMsg); err != nil {
		return fmt.Errorf("failed to publish participant to indexer: %w", err)
	}

	// If participant has username (authenticated user), publish access control
	if participant.Username != "" {
		relations := []string{"participant"}
		if participant.Host {
			relations = append(relations, "host")
		}

		memberMsg := GenericFGAMessage{
			ObjectType: "v1_past_meeting",
			Operation:  "member_put",
			Data: map[string]interface{}{
				"uid":       participant.MeetingID,
				"username":  participant.Username,
				"relations": relations,
			},
		}

		if err := p.publish(ctx, "lfx.fga-sync.update_access", memberMsg); err != nil {
			return fmt.Errorf("failed to publish participant access control: %w", err)
		}
	}

	return nil
}

// PublishPastMeetingRecordingEvent publishes a recording event to indexer and FGA-sync services
// Note: This also publishes a transcript event if the recording has transcript files
func (p *NATSPublisher) PublishPastMeetingRecordingEvent(ctx context.Context, action string, recording *models.RecordingEventData, tags []string) error {
	p.logger.InfoContext(ctx, "publishing past meeting recording event", "action", action, "recording_id", recording.ID)

	// Publish recording to indexer
	headers := make(map[string]string)
	headers["authorization"] = authorizationHeaderValue
	indexerMsg := IndexerMessage{
		Action:  MessageAction(action),
		Headers: headers,
		Data:    recording,
		Tags:    tags,
	}

	if err := p.publish(ctx, "lfx.index.v1_past_meeting_recording", indexerMsg); err != nil {
		return fmt.Errorf("failed to publish recording to indexer: %w", err)
	}

	// Publish recording access control using generic FGA format
	relations := map[string][]string{}
	references := map[string][]string{}

	if recording.ProjectUID != "" {
		references["project"] = []string{recording.ProjectUID}
	}

	if recording.MeetingAndOccurrenceID != "" {
		references["past_meeting"] = []string{recording.MeetingAndOccurrenceID}
	}

	// Determine public based on recording_access
	isPublic := recording.RecordingAccess == "public"

	accessMsg := GenericFGAMessage{
		ObjectType: "v1_past_meeting_recording",
		Operation:  "update_access",
		Data: map[string]interface{}{
			"uid":        recording.ID,
			"public":     isPublic,
			"relations":  relations,
			"references": references,
		},
	}

	if err := p.publish(ctx, "lfx.fga-sync.update_access", accessMsg); err != nil {
		return fmt.Errorf("failed to publish recording access control: %w", err)
	}

	// If transcript is enabled, also publish transcript event
	if recording.TranscriptEnabled {
		transcriptData := &models.TranscriptEventData{
			ID:                     recording.ID,
			MeetingAndOccurrenceID: recording.MeetingAndOccurrenceID,
			ProjectUID:             recording.ProjectUID,
			TranscriptAccess:       recording.TranscriptAccess,
			Platform:               "Zoom",
		}

		transcriptTags := make([]string, len(tags))
		copy(transcriptTags, tags)

		if err := p.PublishPastMeetingTranscriptEvent(ctx, action, transcriptData, transcriptTags); err != nil {
			return fmt.Errorf("failed to publish transcript event: %w", err)
		}
	}

	return nil
}

// PublishPastMeetingTranscriptEvent publishes a transcript event to indexer and FGA-sync services
func (p *NATSPublisher) PublishPastMeetingTranscriptEvent(ctx context.Context, action string, transcript *models.TranscriptEventData, tags []string) error {
	p.logger.InfoContext(ctx, "publishing past meeting transcript event", "action", action, "transcript_id", transcript.ID)

	// Publish to indexer
	headers := make(map[string]string)
	headers["authorization"] = authorizationHeaderValue
	indexerMsg := IndexerMessage{
		Action:  MessageAction(action),
		Headers: headers,
		Data:    transcript,
		Tags:    tags,
	}

	if err := p.publish(ctx, "lfx.index.v1_past_meeting_transcript", indexerMsg); err != nil {
		return fmt.Errorf("failed to publish transcript to indexer: %w", err)
	}

	// Publish access control message using generic FGA format
	relations := map[string][]string{}
	references := map[string][]string{}

	if transcript.ProjectUID != "" {
		references["project"] = []string{transcript.ProjectUID}
	}

	if transcript.MeetingAndOccurrenceID != "" {
		references["past_meeting"] = []string{transcript.MeetingAndOccurrenceID}
	}

	// Determine public based on transcript_access
	isPublic := transcript.TranscriptAccess == "public"

	accessMsg := GenericFGAMessage{
		ObjectType: "v1_past_meeting_transcript",
		Operation:  "update_access",
		Data: map[string]interface{}{
			"uid":        transcript.ID,
			"public":     isPublic,
			"relations":  relations,
			"references": references,
		},
	}

	if err := p.publish(ctx, "lfx.fga-sync.update_access", accessMsg); err != nil {
		return fmt.Errorf("failed to publish transcript access control: %w", err)
	}

	return nil
}

// PublishPastMeetingSummaryEvent publishes a summary event to indexer and FGA-sync services
func (p *NATSPublisher) PublishPastMeetingSummaryEvent(ctx context.Context, action string, summary *models.SummaryEventData, tags []string) error {
	p.logger.InfoContext(ctx, "publishing past meeting summary event", "action", action, "summary_id", summary.ID)

	// Publish to indexer
	headers := make(map[string]string)
	headers["authorization"] = authorizationHeaderValue
	indexerMsg := IndexerMessage{
		Action:  MessageAction(action),
		Headers: headers,
		Data:    summary,
		Tags:    tags,
	}

	if err := p.publish(ctx, "lfx.index.v1_past_meeting_summary", indexerMsg); err != nil {
		return fmt.Errorf("failed to publish summary to indexer: %w", err)
	}

	// Publish access control message using generic FGA format
	// Summaries inherit access from the parent past meeting
	relations := map[string][]string{}
	references := map[string][]string{}

	if summary.ProjectUID != "" {
		references["project"] = []string{summary.ProjectUID}
	}

	if summary.MeetingAndOccurrenceID != "" {
		references["past_meeting"] = []string{summary.MeetingAndOccurrenceID}
	}

	accessMsg := GenericFGAMessage{
		ObjectType: "v1_past_meeting_summary",
		Operation:  "update_access",
		Data: map[string]interface{}{
			"uid":        summary.ID,
			"public":     false, // Summaries are not public
			"relations":  relations,
			"references": references,
		},
	}

	if err := p.publish(ctx, "lfx.fga-sync.update_access", accessMsg); err != nil {
		return fmt.Errorf("failed to publish summary access control: %w", err)
	}

	return nil
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

	p.logger.DebugContext(ctx, "successfully published event", "subject", subject, "payload_size", len(payload))
	return nil
}

// Close closes the NATS publisher and releases resources
func (p *NATSPublisher) Close() error {
	// NATS connection is managed externally, so we don't close it here
	return nil
}

// Ensure NATSPublisher implements EventPublisher
var _ domain.EventPublisher = (*NATSPublisher)(nil)
