// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/nats-io/nats.go"

	fgaconstants "github.com/linuxfoundation/lfx-v2-fga-sync/pkg/constants"
	fgatypes "github.com/linuxfoundation/lfx-v2-fga-sync/pkg/types"
	indexerConstants "github.com/linuxfoundation/lfx-v2-indexer-service/pkg/constants"
	indexerTypes "github.com/linuxfoundation/lfx-v2-indexer-service/pkg/types"

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

	// NATS subjects for indexer messages.
	IndexV1MeetingSubject                = "lfx.index.v1_meeting"
	IndexV1MeetingRegistrantSubject      = "lfx.index.v1_meeting_registrant"
	IndexV1MeetingRSVPSubject            = "lfx.index.v1_meeting_rsvp"
	IndexV1PastMeetingSubject            = "lfx.index.v1_past_meeting"
	IndexV1PastMeetingParticipantSubject = "lfx.index.v1_past_meeting_participant"
	IndexV1PastMeetingRecordingSubject   = "lfx.index.v1_past_meeting_recording"
	IndexV1PastMeetingTranscriptSubject  = "lfx.index.v1_past_meeting_transcript"
	IndexV1PastMeetingSummarySubject     = "lfx.index.v1_past_meeting_summary"
	IndexV1MeetingAttachmentSubject      = "lfx.index.v1_meeting_attachment"
	IndexV1PastMeetingAttachmentSubject  = "lfx.index.v1_past_meeting_attachment"
)

// IndexerMessage is the structure for indexer messages
type IndexerMessage struct {
	Action  MessageAction     `json:"action"`
	Headers map[string]string `json:"headers"`
	Data    interface{}       `json:"data"`
	Tags    []string          `json:"tags"`
}

// NATSPublisher implements the EventPublisher interface using core NATS pub/sub
type NATSPublisher struct {
	nc     *nats.Conn
	logger *slog.Logger
}

// NewNATSPublisher creates a new NATS event publisher
func NewNATSPublisher(nc *nats.Conn, logger *slog.Logger) (*NATSPublisher, error) {
	return &NATSPublisher{
		nc:     nc,
		logger: logger,
	}, nil
}

// PublishMeetingEvent publishes a meeting event to indexer and FGA-sync services
func (p *NATSPublisher) PublishMeetingEvent(ctx context.Context, action string, meeting *models.MeetingEventData) error {
	p.logger.InfoContext(ctx, "publishing meeting event", "action", action, "meeting_id", meeting.ID)

	tags := meeting.Tags()
	isPublic := meeting.Visibility == "public"
	indexerMsg := indexerTypes.IndexerMessageEnvelope{
		Action:  indexerConstants.MessageAction(action),
		Headers: map[string]string{"authorization": authorizationHeaderValue},
		Data:    meeting,
		Tags:    tags,
		IndexingConfig: &indexerTypes.IndexingConfig{
			ObjectID:             meeting.ID,
			Public:               &isPublic,
			AccessCheckObject:    indexerConstants.ObjectTypeV1Meeting + ":" + meeting.ID,
			AccessCheckRelation:  "viewer",
			HistoryCheckObject:   indexerConstants.ObjectTypeV1Meeting + ":" + meeting.ID,
			HistoryCheckRelation: "auditor",
			ParentRefs:           meeting.ParentRefs(),
			Tags:                 tags,
			SortName:             meeting.SortName(),
			NameAndAliases:       meeting.NameAndAliases(),
			Fulltext:             meeting.FullText(),
		},
	}

	if err := p.publish(ctx, IndexV1MeetingSubject, indexerMsg); err != nil {
		return fmt.Errorf("failed to publish meeting to indexer: %w", err)
	}

	// Publish access control message using generic FGA format
	relations := map[string][]string{}
	references := map[string][]string{}

	if len(meeting.Organizers) > 0 {
		relations["organizer"] = meeting.Organizers
	}

	if meeting.ProjectUID != "" {
		references["project"] = []string{meeting.ProjectUID}
	}

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

	accessMsg := fgatypes.GenericFGAMessage{
		ObjectType: "v1_meeting",
		Operation:  "update_access",
		Data: fgatypes.GenericAccessData{
			UID:              meeting.ID,
			Public:           isPublic,
			Relations:        relations,
			References:       references,
			ExcludeRelations: []string{"participant", "host"},
		},
	}

	if err := p.publish(ctx, fgaconstants.GenericUpdateAccessSubject, accessMsg); err != nil {
		return fmt.Errorf("failed to publish meeting access control: %w", err)
	}

	return nil
}

// PublishRegistrantEvent publishes a registrant event to indexer and FGA-sync services
func (p *NATSPublisher) PublishRegistrantEvent(ctx context.Context, action string, registrant *models.RegistrantEventData) error {
	p.logger.InfoContext(ctx, "publishing registrant event", "action", action, "registrant_uid", registrant.UID)

	tags := registrant.Tags()
	publicFalse := false
	indexerMsg := indexerTypes.IndexerMessageEnvelope{
		Action:  indexerConstants.MessageAction(action),
		Headers: map[string]string{"authorization": authorizationHeaderValue},
		Data:    registrant,
		Tags:    tags,
		IndexingConfig: &indexerTypes.IndexingConfig{
			ObjectID:             registrant.UID,
			Public:               &publicFalse,
			AccessCheckObject:    indexerConstants.ObjectTypeV1Meeting + ":" + registrant.MeetingID,
			AccessCheckRelation:  "viewer",
			HistoryCheckObject:   indexerConstants.ObjectTypeV1Meeting + ":" + registrant.MeetingID,
			HistoryCheckRelation: "auditor",
			ParentRefs:           registrant.ParentRefs(),
			Tags:                 tags,
			SortName:             registrant.SortName(),
			NameAndAliases:       registrant.NameAndAliases(),
			Fulltext:             registrant.FullText(),
		},
	}

	if err := p.publish(ctx, IndexV1MeetingRegistrantSubject, indexerMsg); err != nil {
		return fmt.Errorf("failed to publish registrant to indexer: %w", err)
	}

	// If registrant has username (authenticated user), publish access control.
	// fga-sync sets either "host" or "participant" exclusively — access as a participant
	// is granted transitively via the schema (participant: [user] or host).
	if registrant.Username != "" {
		// The fga-sync service expects the username in the Auth0 "sub" format.
		auth0Username, err := lookupUsernameToAuthSub(ctx, p.nc, registrant.Username, p.logger)
		if err != nil {
			return fmt.Errorf("failed to resolve auth sub for registrant: %w", err)
		}

		relation := "participant"
		mutuallyExclusive := "host"
		if registrant.Host {
			relation = "host"
			mutuallyExclusive = "participant"
		}

		memberMsg := fgatypes.GenericFGAMessage{
			ObjectType: "v1_meeting",
			Operation:  "member_put",
			Data: fgatypes.GenericMemberData{
				UID:                   registrant.MeetingID,
				Username:              auth0Username,
				Relations:             []string{relation},
				MutuallyExclusiveWith: []string{mutuallyExclusive},
			},
		}

		if err := p.publish(ctx, fgaconstants.GenericMemberPutSubject, memberMsg); err != nil {
			return fmt.Errorf("failed to publish registrant access control: %w", err)
		}
	}

	return nil
}

// PublishInviteResponseEvent publishes an invite response (RSVP) event to indexer service
func (p *NATSPublisher) PublishInviteResponseEvent(ctx context.Context, action string, response *models.InviteResponseEventData) error {
	p.logger.InfoContext(ctx, "publishing invite response event", "action", action, "response_id", response.ID)

	tags := response.Tags()
	publicFalse := false
	indexerMsg := indexerTypes.IndexerMessageEnvelope{
		Action:  indexerConstants.MessageAction(action),
		Headers: map[string]string{"authorization": authorizationHeaderValue},
		Data:    response,
		Tags:    tags,
		IndexingConfig: &indexerTypes.IndexingConfig{
			ObjectID:             response.ID,
			Public:               &publicFalse,
			AccessCheckObject:    indexerConstants.ObjectTypeV1Meeting + ":" + response.MeetingID,
			AccessCheckRelation:  "viewer",
			HistoryCheckObject:   indexerConstants.ObjectTypeV1Meeting + ":" + response.MeetingID,
			HistoryCheckRelation: "auditor",
			ParentRefs:           response.ParentRefs(),
			Tags:                 tags,
			SortName:             response.SortName(),
			NameAndAliases:       response.NameAndAliases(),
			Fulltext:             response.FullText(),
		},
	}

	if err := p.publish(ctx, IndexV1MeetingRSVPSubject, indexerMsg); err != nil {
		return fmt.Errorf("failed to publish invite response to indexer: %w", err)
	}

	return nil
}

// PublishPastMeetingEvent publishes a past meeting event to indexer and FGA-sync services
func (p *NATSPublisher) PublishPastMeetingEvent(ctx context.Context, action string, meeting *models.PastMeetingEventData) error {
	if meeting.MeetingAndOccurrenceID == "" {
		return domain.NewValidationError("meeting_and_occurrence_id is required for publishing messages about the past meeting")
	}

	p.logger.InfoContext(ctx, "publishing past meeting event", "action", action, "past_meeting_id", meeting.MeetingAndOccurrenceID)

	tags := meeting.Tags()
	publicFalse := false
	indexerMsg := indexerTypes.IndexerMessageEnvelope{
		Action:  indexerConstants.MessageAction(action),
		Headers: map[string]string{"authorization": authorizationHeaderValue},
		Data:    meeting,
		Tags:    tags,
		IndexingConfig: &indexerTypes.IndexingConfig{
			ObjectID:             meeting.MeetingAndOccurrenceID,
			Public:               &publicFalse,
			AccessCheckObject:    indexerConstants.ObjectTypeV1PastMeeting + ":" + meeting.MeetingAndOccurrenceID,
			AccessCheckRelation:  "viewer",
			HistoryCheckObject:   indexerConstants.ObjectTypeV1PastMeeting + ":" + meeting.MeetingAndOccurrenceID,
			HistoryCheckRelation: "auditor",
			ParentRefs:           meeting.ParentRefs(),
			Tags:                 tags,
			SortName:             meeting.SortName(),
			NameAndAliases:       meeting.NameAndAliases(),
			Fulltext:             meeting.FullText(),
		},
	}

	if err := p.publish(ctx, IndexV1PastMeetingSubject, indexerMsg); err != nil {
		return fmt.Errorf("failed to publish past meeting to indexer: %w", err)
	}

	// Publish past meeting access control via generic FGA handler.
	// Per-artifact conditional relations (recording_viewer, transcript_viewer, ai_summary_viewer)
	// are written here — not in the artifact publishers — so FGA is updated whenever the past
	// meeting record changes, not only when an artifact is re-published.
	pastMeetingRefs := map[string][]string{}
	pastMeetingRelations := map[string][]string{}
	if meeting.MeetingID != "" {
		pastMeetingRefs["meeting"] = []string{"v1_meeting:" + meeting.MeetingID}
	}
	if meeting.ProjectUID != "" {
		pastMeetingRefs["project"] = []string{meeting.ProjectUID}
	}
	committeeUIDs := make([]string, 0, len(meeting.Committees))
	for _, c := range meeting.Committees {
		if c.UID != "" {
			committeeUIDs = append(committeeUIDs, c.UID)
		}
	}
	if len(committeeUIDs) > 0 {
		pastMeetingRefs["committee"] = committeeUIDs
	}

	// Per-artifact access: self-referential references enable role-based access
	// via the existing host/attendee/invitee tuples on the same v1_past_meeting object.
	selfRef := "v1_past_meeting:" + meeting.MeetingAndOccurrenceID

	switch meeting.RecordingAccess {
	case "public":
		pastMeetingRelations["recording_viewer"] = []string{"*"}
	case "meeting_participants":
		pastMeetingRefs["past_meeting_for_host_recording_view"] = []string{selfRef}
		pastMeetingRefs["past_meeting_for_attendee_recording_view"] = []string{selfRef}
		pastMeetingRefs["past_meeting_for_participant_recording_view"] = []string{selfRef}
	default: // meeting_hosts or unset
		pastMeetingRefs["past_meeting_for_host_recording_view"] = []string{selfRef}
	}

	switch meeting.TranscriptAccess {
	case "public":
		pastMeetingRelations["transcript_viewer"] = []string{"*"}
	case "meeting_participants":
		pastMeetingRefs["past_meeting_for_host_transcript_view"] = []string{selfRef}
		pastMeetingRefs["past_meeting_for_attendee_transcript_view"] = []string{selfRef}
		pastMeetingRefs["past_meeting_for_participant_transcript_view"] = []string{selfRef}
	default: // meeting_hosts or unset
		pastMeetingRefs["past_meeting_for_host_transcript_view"] = []string{selfRef}
	}

	switch meeting.AISummaryAccess {
	case "public":
		pastMeetingRelations["ai_summary_viewer"] = []string{"*"}
	case "meeting_participants":
		pastMeetingRefs["past_meeting_for_host_summary_view"] = []string{selfRef}
		pastMeetingRefs["past_meeting_for_attendee_summary_view"] = []string{selfRef}
		pastMeetingRefs["past_meeting_for_participant_summary_view"] = []string{selfRef}
	default: // meeting_hosts or unset
		pastMeetingRefs["past_meeting_for_host_summary_view"] = []string{selfRef}
	}

	pastMeetingAccessMsg := fgatypes.GenericFGAMessage{
		ObjectType: "v1_past_meeting",
		Operation:  "update_access",
		Data: fgatypes.GenericAccessData{
			UID:        meeting.MeetingAndOccurrenceID,
			Public:     false,
			Relations:  pastMeetingRelations,
			References: pastMeetingRefs,
			// host/invitee/attendee are managed by PublishPastMeetingParticipantEvent
			// and must not be overwritten here.
			ExcludeRelations: []string{"host", "invitee", "attendee"},
		},
	}

	if err := p.publish(ctx, fgaconstants.GenericUpdateAccessSubject, pastMeetingAccessMsg); err != nil {
		return fmt.Errorf("failed to publish past meeting access control: %w", err)
	}

	return nil
}

// PublishPastMeetingParticipantEvent publishes a participant event to indexer and FGA-sync services
func (p *NATSPublisher) PublishPastMeetingParticipantEvent(ctx context.Context, action string, participant *models.PastMeetingParticipantEventData) error {
	if participant.MeetingAndOccurrenceID == "" {
		return domain.NewValidationError("meeting_and_occurrence_id is required for participant event")
	}
	p.logger.InfoContext(ctx, "publishing past meeting participant event", "action", action, "participant_uid", participant.UID)

	tags := participant.Tags()
	publicFalse := false
	indexerMsg := indexerTypes.IndexerMessageEnvelope{
		Action:  indexerConstants.MessageAction(action),
		Headers: map[string]string{"authorization": authorizationHeaderValue},
		Data:    participant,
		Tags:    tags,
		IndexingConfig: &indexerTypes.IndexingConfig{
			ObjectID:             participant.UID,
			Public:               &publicFalse,
			AccessCheckObject:    indexerConstants.ObjectTypeV1PastMeeting + ":" + participant.MeetingAndOccurrenceID,
			AccessCheckRelation:  "viewer",
			HistoryCheckObject:   indexerConstants.ObjectTypeV1PastMeeting + ":" + participant.MeetingAndOccurrenceID,
			HistoryCheckRelation: "auditor",
			ParentRefs:           participant.ParentRefs(),
			Tags:                 tags,
			SortName:             participant.SortName(),
			NameAndAliases:       participant.NameAndAliases(),
			Fulltext:             participant.FullText(),
		},
	}

	if err := p.publish(ctx, IndexV1PastMeetingParticipantSubject, indexerMsg); err != nil {
		return fmt.Errorf("failed to publish participant to indexer: %w", err)
	}

	// If participant has username (authenticated user), publish access control.
	if participant.Username != "" {
		// Build the set of desired relations based on participant flags.
		// v1_past_meeting uses "host", "invitee", and "attendee" relations.
		var relations []string
		if participant.Host {
			relations = append(relations, "host")
		}
		if participant.IsInvited {
			relations = append(relations, "invitee")
		}
		if participant.IsAttended {
			relations = append(relations, "attendee")
		}

		// The fga-sync service expects the username in the Auth0 "sub" format.
		auth0Username, err := lookupUsernameToAuthSub(ctx, p.nc, participant.Username, p.logger)
		if err != nil {
			return fmt.Errorf("failed to resolve auth sub for participant: %w", err)
		}

		memberMsg := fgatypes.GenericFGAMessage{
			ObjectType: "v1_past_meeting",
			Operation:  "member_put",
			Data: fgatypes.GenericMemberData{
				UID:                   participant.MeetingAndOccurrenceID,
				Username:              auth0Username,
				Relations:             relations,
				MutuallyExclusiveWith: []string{"host", "invitee", "attendee"},
			},
		}

		if err := p.publish(ctx, fgaconstants.GenericMemberPutSubject, memberMsg); err != nil {
			return fmt.Errorf("failed to publish participant access control: %w", err)
		}
	}

	return nil
}

// PublishPastMeetingRecordingEvent publishes a recording event to indexer and FGA-sync services
func (p *NATSPublisher) PublishPastMeetingRecordingEvent(ctx context.Context, action string, recording *models.RecordingEventData) error {
	p.logger.InfoContext(ctx, "publishing past meeting recording event", "action", action, "recording_id", recording.ID)

	tags := recording.Tags()
	isPublic := recording.RecordingAccess == "public"
	indexerMsg := indexerTypes.IndexerMessageEnvelope{
		Action:  indexerConstants.MessageAction(action),
		Headers: map[string]string{"authorization": authorizationHeaderValue},
		Data:    recording,
		Tags:    tags,
		IndexingConfig: &indexerTypes.IndexingConfig{
			ObjectID:             recording.ID,
			Public:               &isPublic,
			AccessCheckObject:    indexerConstants.ObjectTypeV1PastMeeting + ":" + recording.MeetingAndOccurrenceID,
			AccessCheckRelation:  "recording_viewer",
			HistoryCheckObject:   indexerConstants.ObjectTypeV1PastMeeting + ":" + recording.MeetingAndOccurrenceID,
			HistoryCheckRelation: "auditor",
			ParentRefs:           recording.ParentRefs(),
			Tags:                 tags,
			SortName:             recording.SortName(),
			NameAndAliases:       recording.NameAndAliases(),
			Fulltext:             recording.FullText(),
		},
	}

	if err := p.publish(ctx, IndexV1PastMeetingRecordingSubject, indexerMsg); err != nil {
		return fmt.Errorf("failed to publish recording to indexer: %w", err)
	}

	// FGA access for recordings is managed in PublishPastMeetingEvent, not here,
	// because recording_access lives on the past meeting record. This ensures FGA
	// stays in sync when the access setting changes without a new recording event.
	return nil
}

// PublishPastMeetingTranscriptEvent publishes a transcript event to indexer and FGA-sync services
func (p *NATSPublisher) PublishPastMeetingTranscriptEvent(ctx context.Context, action string, transcript *models.TranscriptEventData) error {
	p.logger.InfoContext(ctx, "publishing past meeting transcript event", "action", action, "transcript_id", transcript.ID)

	tags := transcript.Tags()
	isPublic := transcript.TranscriptAccess == "public"
	indexerMsg := indexerTypes.IndexerMessageEnvelope{
		Action:  indexerConstants.MessageAction(action),
		Headers: map[string]string{"authorization": authorizationHeaderValue},
		Data:    transcript,
		Tags:    tags,
		IndexingConfig: &indexerTypes.IndexingConfig{
			ObjectID:             transcript.ID,
			Public:               &isPublic,
			AccessCheckObject:    indexerConstants.ObjectTypeV1PastMeeting + ":" + transcript.MeetingAndOccurrenceID,
			AccessCheckRelation:  "transcript_viewer",
			HistoryCheckObject:   indexerConstants.ObjectTypeV1PastMeeting + ":" + transcript.MeetingAndOccurrenceID,
			HistoryCheckRelation: "auditor",
			ParentRefs:           transcript.ParentRefs(),
			Tags:                 tags,
			SortName:             transcript.SortName(),
			NameAndAliases:       transcript.NameAndAliases(),
			Fulltext:             transcript.FullText(),
		},
	}

	if err := p.publish(ctx, IndexV1PastMeetingTranscriptSubject, indexerMsg); err != nil {
		return fmt.Errorf("failed to publish transcript to indexer: %w", err)
	}

	// FGA access for transcripts is managed in PublishPastMeetingEvent, not here,
	// because transcript_access lives on the past meeting record.
	return nil
}

// PublishPastMeetingSummaryEvent publishes a summary event to indexer and FGA-sync services.
// summaryAccess is the ai_summary_access value from the parent past meeting record.
func (p *NATSPublisher) PublishPastMeetingSummaryEvent(ctx context.Context, action string, summary *models.SummaryEventData, summaryAccess string) error {
	p.logger.InfoContext(ctx, "publishing past meeting summary event", "action", action, "summary_id", summary.ID)

	isPublic := summaryAccess == "public"
	tags := summary.Tags()
	indexerMsg := indexerTypes.IndexerMessageEnvelope{
		Action:  indexerConstants.MessageAction(action),
		Headers: map[string]string{"authorization": authorizationHeaderValue},
		Data:    summary,
		Tags:    tags,
		IndexingConfig: &indexerTypes.IndexingConfig{
			ObjectID:             summary.ID,
			Public:               &isPublic,
			AccessCheckObject:    indexerConstants.ObjectTypeV1PastMeeting + ":" + summary.MeetingAndOccurrenceID,
			AccessCheckRelation:  "ai_summary_viewer",
			HistoryCheckObject:   indexerConstants.ObjectTypeV1PastMeeting + ":" + summary.MeetingAndOccurrenceID,
			HistoryCheckRelation: "auditor",
			ParentRefs:           summary.ParentRefs(),
			Tags:                 tags,
			SortName:             summary.SortName(),
			NameAndAliases:       summary.NameAndAliases(),
			Fulltext:             summary.FullText(),
		},
	}

	if err := p.publish(ctx, IndexV1PastMeetingSummarySubject, indexerMsg); err != nil {
		return fmt.Errorf("failed to publish summary to indexer: %w", err)
	}

	// FGA access for summaries is managed in PublishPastMeetingEvent, not here,
	// because ai_summary_access lives on the past meeting record.
	return nil
}

// PublishMeetingAttachmentEvent publishes a meeting attachment event to indexer and FGA-sync services
func (p *NATSPublisher) PublishMeetingAttachmentEvent(ctx context.Context, action string, attachment *models.MeetingAttachmentEventData) error {
	p.logger.InfoContext(ctx, "publishing meeting attachment event", "action", action, "attachment_uid", attachment.UID)

	tags := attachment.Tags()
	isPublic := false
	indexerMsg := indexerTypes.IndexerMessageEnvelope{
		Action:  indexerConstants.MessageAction(action),
		Headers: map[string]string{"authorization": authorizationHeaderValue},
		Data:    attachment,
		Tags:    tags,
		IndexingConfig: &indexerTypes.IndexingConfig{
			ObjectID:             attachment.UID,
			Public:               &isPublic,
			AccessCheckObject:    indexerConstants.ObjectTypeV1Meeting + ":" + attachment.MeetingID,
			AccessCheckRelation:  "viewer",
			HistoryCheckObject:   indexerConstants.ObjectTypeV1Meeting + ":" + attachment.MeetingID,
			HistoryCheckRelation: "auditor",
			ParentRefs:           attachment.ParentRefs(),
			Tags:                 tags,
			SortName:             attachment.SortName(),
			NameAndAliases:       attachment.NameAndAliases(),
			Fulltext:             attachment.FullText(),
		},
	}

	if err := p.publish(ctx, IndexV1MeetingAttachmentSubject, indexerMsg); err != nil {
		return fmt.Errorf("failed to publish meeting attachment to indexer: %w", err)
	}

	return nil
}

// PublishPastMeetingAttachmentEvent publishes a past meeting attachment event to indexer and FGA-sync services
func (p *NATSPublisher) PublishPastMeetingAttachmentEvent(ctx context.Context, action string, attachment *models.PastMeetingAttachmentEventData) error {
	p.logger.InfoContext(ctx, "publishing past meeting attachment event", "action", action, "attachment_uid", attachment.UID)

	tags := attachment.Tags()
	isPublic := false
	indexerMsg := indexerTypes.IndexerMessageEnvelope{
		Action:  indexerConstants.MessageAction(action),
		Headers: map[string]string{"authorization": authorizationHeaderValue},
		Data:    attachment,
		Tags:    tags,
		IndexingConfig: &indexerTypes.IndexingConfig{
			ObjectID:             attachment.UID,
			Public:               &isPublic,
			AccessCheckObject:    indexerConstants.ObjectTypeV1PastMeeting + ":" + attachment.MeetingAndOccurrenceID,
			AccessCheckRelation:  "viewer",
			HistoryCheckObject:   indexerConstants.ObjectTypeV1PastMeeting + ":" + attachment.MeetingAndOccurrenceID,
			HistoryCheckRelation: "auditor",
			ParentRefs:           attachment.ParentRefs(),
			Tags:                 tags,
			SortName:             attachment.SortName(),
			NameAndAliases:       attachment.NameAndAliases(),
			Fulltext:             attachment.FullText(),
		},
	}

	if err := p.publish(ctx, IndexV1PastMeetingAttachmentSubject, indexerMsg); err != nil {
		return fmt.Errorf("failed to publish past meeting attachment to indexer: %w", err)
	}

	return nil
}

// PublishIndexerDelete sends a "deleted" indexer message for the given resource ID to subject.
func (p *NATSPublisher) PublishIndexerDelete(ctx context.Context, subject, id string) error {
	msg := IndexerMessage{
		Action:  ActionDeleted,
		Headers: map[string]string{"authorization": authorizationHeaderValue},
		Data:    id,
		Tags:    []string{},
	}
	return p.publish(ctx, subject, msg)
}

// PublishAccessDelete sends a pre-built access control message payload to subject.
// The caller is responsible for marshalling the payload; pass []byte(id) for simple deletes.
func (p *NATSPublisher) PublishAccessDelete(ctx context.Context, subject string, payload []byte) error {
	if err := p.nc.Publish(subject, payload); err != nil {
		p.logger.With(logging.ErrKey, err).ErrorContext(ctx, "failed to publish access delete", "subject", subject)
		return fmt.Errorf("failed to publish to %s: %w", subject, err)
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

	if err := p.nc.Publish(subject, payload); err != nil {
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
