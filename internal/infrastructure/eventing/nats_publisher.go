// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/nats-io/nats.go"

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

	accessMsg := GenericFGAMessage{
		ObjectType: "v1_meeting",
		Operation:  "update_access",
		Data: map[string]interface{}{
			"uid":               meeting.ID,
			"public":            isPublic,
			"relations":         relations,
			"references":        references,
			"exclude_relations": []string{"participant", "host"},
		},
	}

	if err := p.publish(ctx, "lfx.fga-sync.update_access", accessMsg); err != nil {
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

		memberMsg := GenericFGAMessage{
			ObjectType: "v1_meeting",
			Operation:  "member_put",
			Data: map[string]interface{}{
				"uid":                     registrant.MeetingID,
				"username":                auth0Username,
				"relations":               []string{relation},
				"mutually_exclusive_with": []string{mutuallyExclusive},
			},
		}

		if err := p.publish(ctx, "lfx.fga-sync.member_put", memberMsg); err != nil {
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
	p.logger.InfoContext(ctx, "publishing past meeting event", "action", action, "past_meeting_id", meeting.ID)

	tags := meeting.Tags()
	publicFalse := false
	indexerMsg := indexerTypes.IndexerMessageEnvelope{
		Action:  indexerConstants.MessageAction(action),
		Headers: map[string]string{"authorization": authorizationHeaderValue},
		Data:    meeting,
		Tags:    tags,
		IndexingConfig: &indexerTypes.IndexingConfig{
			ObjectID:             meeting.ID,
			Public:               &publicFalse,
			AccessCheckObject:    indexerConstants.ObjectTypeV1PastMeeting + ":" + meeting.ID,
			AccessCheckRelation:  "viewer",
			HistoryCheckObject:   indexerConstants.ObjectTypeV1PastMeeting + ":" + meeting.ID,
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
	pastMeetingRefs := map[string][]string{}
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

	pastMeetingAccessMsg := GenericFGAMessage{
		ObjectType: "v1_past_meeting",
		Operation:  "update_access",
		Data: map[string]interface{}{
			"uid":        meeting.ID,
			"public":     false,
			"relations":  map[string][]string{},
			"references": pastMeetingRefs,
		},
	}

	if err := p.publish(ctx, "lfx.fga-sync.update_access", pastMeetingAccessMsg); err != nil {
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

		memberMsg := GenericFGAMessage{
			ObjectType: "v1_past_meeting",
			Operation:  "member_put",
			Data: map[string]interface{}{
				"uid":                     participant.MeetingAndOccurrenceID,
				"username":                auth0Username,
				"relations":               relations,
				"mutually_exclusive_with": []string{"host", "invitee", "attendee"},
			},
		}

		if err := p.publish(ctx, "lfx.fga-sync.member_put", memberMsg); err != nil {
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
			AccessCheckRelation:  "viewer",
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

	// Publish recording access control via generic FGA handler.
	// references builds object-to-object tuples; values use "v1_past_meeting:<id>" so fga-sync
	// writes the correct type (define past_meeting: [v1_past_meeting]).
	pastMeetingRef := "v1_past_meeting:" + recording.MeetingAndOccurrenceID
	recordingRefs := map[string][]string{
		"past_meeting": {pastMeetingRef},
	}
	switch recording.RecordingAccess {
	case "public":
		// isPublic=true handles viewer access via user:*
	case "meeting_participants":
		recordingRefs["past_meeting_for_host_view"] = []string{pastMeetingRef}
		recordingRefs["past_meeting_for_attendee_view"] = []string{pastMeetingRef}
		recordingRefs["past_meeting_for_participant_view"] = []string{pastMeetingRef}
	default: // meeting_hosts or unset
		recordingRefs["past_meeting_for_host_view"] = []string{pastMeetingRef}
	}

	recordingAccessMsg := GenericFGAMessage{
		ObjectType: "v1_past_meeting_recording",
		Operation:  "update_access",
		Data: map[string]interface{}{
			"uid":        recording.ID,
			"public":     isPublic,
			"relations":  map[string][]string{},
			"references": recordingRefs,
		},
	}

	if err := p.publish(ctx, "lfx.fga-sync.update_access", recordingAccessMsg); err != nil {
		return fmt.Errorf("failed to publish recording access control: %w", err)
	}

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
			AccessCheckRelation:  "viewer",
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

	// Publish transcript access control via generic FGA handler.
	pastMeetingRef := "v1_past_meeting:" + transcript.MeetingAndOccurrenceID
	transcriptRefs := map[string][]string{
		"past_meeting": {pastMeetingRef},
	}
	switch transcript.TranscriptAccess {
	case "public":
		// isPublic=true handles viewer access via user:*
	case "meeting_participants":
		transcriptRefs["past_meeting_for_host_view"] = []string{pastMeetingRef}
		transcriptRefs["past_meeting_for_attendee_view"] = []string{pastMeetingRef}
		transcriptRefs["past_meeting_for_participant_view"] = []string{pastMeetingRef}
	default: // meeting_hosts or unset
		transcriptRefs["past_meeting_for_host_view"] = []string{pastMeetingRef}
	}

	transcriptAccessMsg := GenericFGAMessage{
		ObjectType: "v1_past_meeting_transcript",
		Operation:  "update_access",
		Data: map[string]interface{}{
			"uid":        transcript.ID,
			"public":     isPublic,
			"relations":  map[string][]string{},
			"references": transcriptRefs,
		},
	}

	if err := p.publish(ctx, "lfx.fga-sync.update_access", transcriptAccessMsg); err != nil {
		return fmt.Errorf("failed to publish transcript access control: %w", err)
	}

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
			AccessCheckRelation:  "viewer",
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

	// Publish summary access control via generic FGA handler.
	pastMeetingRef := "v1_past_meeting:" + summary.MeetingAndOccurrenceID
	summaryRefs := map[string][]string{
		"past_meeting": {pastMeetingRef},
	}
	switch summaryAccess {
	case "public":
		// isPublic=true handles viewer access via user:*
	case "meeting_participants":
		summaryRefs["past_meeting_for_host_view"] = []string{pastMeetingRef}
		summaryRefs["past_meeting_for_attendee_view"] = []string{pastMeetingRef}
		summaryRefs["past_meeting_for_participant_view"] = []string{pastMeetingRef}
	default: // meeting_hosts or unset
		summaryRefs["past_meeting_for_host_view"] = []string{pastMeetingRef}
	}

	summaryAccessMsg := GenericFGAMessage{
		ObjectType: "v1_past_meeting_summary",
		Operation:  "update_access",
		Data: map[string]interface{}{
			"uid":        summary.ID,
			"public":     isPublic,
			"relations":  map[string][]string{},
			"references": summaryRefs,
		},
	}

	if err := p.publish(ctx, "lfx.fga-sync.update_access", summaryAccessMsg); err != nil {
		return fmt.Errorf("failed to publish summary access control: %w", err)
	}

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
