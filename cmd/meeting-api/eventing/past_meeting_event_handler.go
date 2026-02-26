// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	indexerConstants "github.com/linuxfoundation/lfx-v2-indexer-service/pkg/constants"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/nats-io/nats.go/jetstream"
)

// =============================================================================
// Past Meeting Event Handler
// =============================================================================

// handlePastMeetingUpdate processes updates to past meeting records
// Returns true to retry (NAK), false to acknowledge (ACK)
func (h *EventHandlers) handlePastMeetingUpdate(
	ctx context.Context,
	key string,
	v1Data map[string]interface{},
) bool {
	funcLogger := h.logger.With("key", key, "handler", "past_meeting")
	funcLogger.DebugContext(ctx, "processing past meeting update")

	// Check if this is a soft delete
	if isDeleted, ok := v1Data["_sdc_deleted_at"].(string); ok && isDeleted != "" {
		return h.handlePastMeetingDelete(ctx, key, v1Data)
	}

	// Convert v1Data to past meeting event data
	pastMeetingData, err := convertMapToPastMeetingData(ctx, v1Data, h.idMapper, h.v1ObjectsKV, h.v1MappingsKV, funcLogger)
	if err != nil {
		funcLogger.With(errKey, err).ErrorContext(ctx, "failed to convert v1Data to past meeting")
		if isTransientError(err) {
			return true // NAK for retry
		}
		return false // Permanent error, ACK and skip
	}

	// Validate required fields
	if pastMeetingData.ID == "" || pastMeetingData.ProjectUID == "" {
		funcLogger.ErrorContext(ctx, "missing required fields in past meeting data")
		return false // Permanent error, ACK and skip
	}
	funcLogger = funcLogger.With("past_meeting_id", pastMeetingData.ID)

	// Determine action (created vs updated)
	mappingKey := fmt.Sprintf("v1_past_meetings.%s", pastMeetingData.ID)
	indexerAction := indexerConstants.ActionCreated
	if _, err := h.v1MappingsKV.Get(ctx, mappingKey); err == nil {
		indexerAction = indexerConstants.ActionUpdated
	}

	// Generate tags for past meeting
	tags := generatePastMeetingTags(pastMeetingData)

	// Publish to indexer and FGA-sync
	if err := h.publisher.PublishPastMeetingEvent(ctx, string(indexerAction), pastMeetingData, tags); err != nil {
		funcLogger.With(errKey, err).ErrorContext(ctx, "failed to publish past meeting event")
		if isTransientError(err) {
			return true // NAK for retry
		}
		return false // Permanent error, ACK and skip
	}

	// Store mapping
	if _, err := h.v1MappingsKV.Put(ctx, mappingKey, []byte("1")); err != nil {
		funcLogger.With(errKey, err).WarnContext(ctx, "failed to store past meeting mapping")
	}

	funcLogger.InfoContext(ctx, "successfully processed past meeting")
	return false // Success, ACK
}

// handlePastMeetingDelete processes past meeting deletions
func (h *EventHandlers) handlePastMeetingDelete(
	ctx context.Context,
	key string,
	v1Data map[string]interface{},
) bool {
	pastMeetingID := extractIDFromKey(key, "itx-zoom-past-meetings.")
	funcLogger := h.logger.With("past_meeting_id", pastMeetingID, "handler", "past_meeting_delete")
	funcLogger.InfoContext(ctx, "processing past meeting deletion")

	// Create minimal event data for deletion
	eventData := &models.PastMeetingEventData{ID: pastMeetingID}

	// Generate tags (minimal for deletion)
	tags := generatePastMeetingTags(eventData)

	// Publish delete event
	if err := h.publisher.PublishPastMeetingEvent(ctx, string(indexerConstants.ActionDeleted), eventData, tags); err != nil {
		funcLogger.With(errKey, err).ErrorContext(ctx, "failed to publish past meeting delete event")
		if isTransientError(err) {
			return true // NAK for retry
		}
		return false // Permanent error, ACK and skip
	}

	// Remove mapping
	mappingKey := fmt.Sprintf("v1_past_meetings.%s", pastMeetingID)
	_ = h.v1MappingsKV.Delete(ctx, mappingKey)

	funcLogger.InfoContext(ctx, "successfully processed past meeting deletion")
	return false // Success, ACK
}

// convertMapToPastMeetingData converts v1 past meeting data to v2 format
func convertMapToPastMeetingData(
	ctx context.Context,
	v1Data map[string]interface{},
	idMapper domain.IDMapper,
	v1ObjectsKV jetstream.KeyValue,
	mappingsKV jetstream.KeyValue,
	logger *slog.Logger,
) (*models.PastMeetingEventData, error) {
	// Convert map to JSON bytes, then to PastMeetingDBRaw
	jsonBytes, err := json.Marshal(v1Data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal v1Data to JSON: %w", err)
	}

	var rawPastMeeting PastMeetingDBRaw
	if err := json.Unmarshal(jsonBytes, &rawPastMeeting); err != nil {
		return nil, fmt.Errorf("failed to unmarshal past meeting data: %w", err)
	}

	// Validate required fields
	if rawPastMeeting.UUID == "" || rawPastMeeting.ProjectID == "" {
		return nil, fmt.Errorf("missing required fields: uuid or proj_id")
	}

	// Map project ID from v1 SFID to v2 UID
	projectUID, err := idMapper.MapProjectV1ToV2(ctx, rawPastMeeting.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("failed to map project ID (transient): %w", err)
	}

	// Parse times
	startTime, _ := parseTime(rawPastMeeting.StartTime)
	endTime, _ := parseTime(rawPastMeeting.EndTime)
	createdAt, _ := parseTime(rawPastMeeting.CreatedAt)
	modifiedAt, _ := parseTime(rawPastMeeting.ModifiedAt)

	// Get committees from mapping index (same logic as active meetings)
	committees := getCommitteesForPastMeeting(ctx, rawPastMeeting.UUID, idMapper, mappingsKV, logger)

	// Build event data
	return &models.PastMeetingEventData{
		ID:               rawPastMeeting.UUID,
		MeetingID:        rawPastMeeting.MeetingID,
		ProjectUID:       projectUID,
		Title:            rawPastMeeting.Topic,
		Description:      rawPastMeeting.Agenda,
		StartTime:        startTime,
		EndTime:          endTime,
		Duration:         getInt(rawPastMeeting.Duration),
		Timezone:         rawPastMeeting.Timezone,
		ParticipantCount: getInt(rawPastMeeting.ParticipantsCount),
		Committees:       committees,
		HostKey:          rawPastMeeting.HostID,
		CreatedAt:        createdAt,
		ModifiedAt:       modifiedAt,
	}, nil
}

// =============================================================================
// Past Meeting Mapping Event Handler
// =============================================================================

// handlePastMeetingMappingUpdate processes updates to past meeting-committee mappings
func (h *EventHandlers) handlePastMeetingMappingUpdate(
	ctx context.Context,
	key string,
	v1Data map[string]interface{},
) bool {
	funcLogger := h.logger.With("key", key, "handler", "past_meeting_mapping")
	funcLogger.InfoContext(ctx, "processing past meeting mapping update")

	// Check if this is a soft delete
	if isDeleted, ok := v1Data["_sdc_deleted_at"].(string); ok && isDeleted != "" {
		return h.handlePastMeetingMappingDelete(ctx, key, v1Data)
	}

	// Extract past meeting ID and mapping data
	pastMeetingUUID := getString(v1Data["past_meeting_uuid"])
	mappingID := getString(v1Data["id"])
	committeeID := getString(v1Data["committee_id"])

	if pastMeetingUUID == "" || mappingID == "" {
		funcLogger.WarnContext(ctx, "missing required fields in past meeting mapping")
		return false // ACK - permanent error
	}

	// Update committee mappings in KV bucket
	if err := updatePastMeetingCommitteeMappings(ctx, pastMeetingUUID, mappingID, committeeID, v1Data, h.v1MappingsKV, funcLogger); err != nil {
		funcLogger.With(errKey, err).ErrorContext(ctx, "failed to update past meeting committee mappings")
		if isTransientError(err) {
			return true // NAK for retry
		}
		return false // ACK - permanent error
	}

	// Re-trigger past meeting indexing
	shouldRetry := h.retriggerPastMeetingIndexing(ctx, pastMeetingUUID)
	if shouldRetry {
		return true // NAK for retry
	}

	funcLogger.InfoContext(ctx, "successfully processed past meeting mapping update")
	return false // ACK
}

// handlePastMeetingMappingDelete processes past meeting-committee mapping deletions
func (h *EventHandlers) handlePastMeetingMappingDelete(
	ctx context.Context,
	key string,
	v1Data map[string]interface{},
) bool {
	pastMeetingUUID := getString(v1Data["past_meeting_uuid"])
	mappingID := getString(v1Data["id"])

	if pastMeetingUUID == "" || mappingID == "" {
		h.logger.WarnContext(ctx, "missing required fields in past meeting mapping deletion")
		return false // ACK - permanent error
	}

	funcLogger := h.logger.With("past_meeting_uuid", pastMeetingUUID, "mapping_id", mappingID, "handler", "past_meeting_mapping_delete")
	funcLogger.InfoContext(ctx, "processing past meeting mapping deletion")

	// Remove the mapping
	if err := removePastMeetingCommitteeMapping(ctx, pastMeetingUUID, mappingID, h.v1MappingsKV, funcLogger); err != nil {
		if isTransientError(err) {
			return true // NAK for retry
		}
	}

	// Re-trigger past meeting indexing
	shouldRetry := h.retriggerPastMeetingIndexing(ctx, pastMeetingUUID)
	if shouldRetry {
		return true // NAK for retry
	}

	funcLogger.InfoContext(ctx, "successfully processed past meeting mapping deletion")
	return false // ACK
}

// =============================================================================
// Helper Functions for Past Meetings
// =============================================================================

// Data models for v1 past meeting raw data

func getCommitteesForPastMeeting(
	ctx context.Context,
	pastMeetingUUID string,
	idMapper domain.IDMapper,
	mappingsKV jetstream.KeyValue,
	logger *slog.Logger,
) []models.Committee {
	key := fmt.Sprintf("past-meeting-mappings.%s", pastMeetingUUID)
	entry, err := mappingsKV.Get(ctx, key)
	if err != nil {
		return nil
	}

	var mappings map[string]map[string]interface{}
	if err := json.Unmarshal(entry.Value(), &mappings); err != nil {
		logger.With(errKey, err).WarnContext(ctx, "failed to unmarshal past meeting committee mappings")
		return nil
	}

	committees := make([]models.Committee, 0, len(mappings))
	for _, mapping := range mappings {
		committeeID := getStringFromMap(mapping, "committee_id")
		if committeeID != "" {
			committeeUID, err := idMapper.MapCommitteeV1ToV2(ctx, committeeID)
			if err != nil {
				logger.With(errKey, err).WarnContext(ctx, "failed to map committee ID", "v1_id", committeeID)
				continue
			}

			filters := getStringSliceFromMap(mapping, "committee_filters")
			committees = append(committees, models.Committee{
				UID:                   committeeUID,
				AllowedVotingStatuses: filters,
			})
		}
	}

	return committees
}

func updatePastMeetingCommitteeMappings(
	ctx context.Context,
	pastMeetingUUID string,
	mappingID string,
	committeeID string,
	v1Data map[string]interface{},
	mappingsKV jetstream.KeyValue,
	logger *slog.Logger,
) error {
	mappingsKey := fmt.Sprintf("past-meeting-mappings.%s", pastMeetingUUID)
	var mappings map[string]map[string]interface{}

	entry, err := mappingsKV.Get(ctx, mappingsKey)
	if err != nil {
		mappings = make(map[string]map[string]interface{})
	} else {
		if err := json.Unmarshal(entry.Value(), &mappings); err != nil {
			logger.With(errKey, err).ErrorContext(ctx, "failed to unmarshal existing past meeting mappings")
			mappings = make(map[string]map[string]interface{})
		}
	}

	// Update or add mapping
	mappingData := map[string]interface{}{"committee_id": committeeID}
	if filters := getStringSliceFromMap(v1Data, "committee_filters"); len(filters) > 0 {
		mappingData["committee_filters"] = filters
	}
	mappings[mappingID] = mappingData

	// Store updated mappings
	mappingsJSON, err := json.Marshal(mappings)
	if err != nil {
		return fmt.Errorf("failed to marshal past meeting mappings: %w", err)
	}

	if _, err := mappingsKV.Put(ctx, mappingsKey, mappingsJSON); err != nil {
		return fmt.Errorf("failed to store past meeting mappings: %w", err)
	}

	return nil
}

func removePastMeetingCommitteeMapping(
	ctx context.Context,
	pastMeetingUUID string,
	mappingID string,
	mappingsKV jetstream.KeyValue,
	logger *slog.Logger,
) error {
	mappingsKey := fmt.Sprintf("past-meeting-mappings.%s", pastMeetingUUID)
	entry, err := mappingsKV.Get(ctx, mappingsKey)
	if err != nil {
		return nil // No mappings found
	}

	var mappings map[string]map[string]interface{}
	if err := json.Unmarshal(entry.Value(), &mappings); err != nil {
		return fmt.Errorf("failed to unmarshal past meeting mappings: %w", err)
	}

	// Remove the mapping
	delete(mappings, mappingID)

	// If no mappings left, delete the key
	if len(mappings) == 0 {
		return mappingsKV.Delete(ctx, mappingsKey)
	}

	// Store updated mappings
	mappingsJSON, err := json.Marshal(mappings)
	if err != nil {
		return fmt.Errorf("failed to marshal updated past meeting mappings: %w", err)
	}

	_, err = mappingsKV.Put(ctx, mappingsKey, mappingsJSON)
	return err
}

func (h *EventHandlers) retriggerPastMeetingIndexing(
	ctx context.Context,
	pastMeetingUUID string,
) bool {
	pastMeetingKey := fmt.Sprintf("itx-zoom-past-meetings.%s", pastMeetingUUID)
	pastMeetingEntry, err := h.v1ObjectsKV.Get(ctx, pastMeetingKey)
	if err != nil {
		h.logger.With(errKey, err).WarnContext(ctx, "past meeting not found during retrigger")
		return false // Past meeting might be deleted, ACK
	}

	var pastMeetingData map[string]interface{}
	if err := json.Unmarshal(pastMeetingEntry.Value(), &pastMeetingData); err != nil {
		h.logger.With(errKey, err).ErrorContext(ctx, "failed to unmarshal past meeting data")
		return false // ACK - permanent error
	}

	// Re-process the past meeting
	return h.handlePastMeetingUpdate(ctx, pastMeetingKey, pastMeetingData)
}

func generatePastMeetingTags(pastMeeting *models.PastMeetingEventData) []string {
	tags := []string{
		"past_meeting_id:" + pastMeeting.ID,
		"meeting_id:" + pastMeeting.MeetingID,
		"project_uid:" + pastMeeting.ProjectUID,
		"title:" + pastMeeting.Title,
	}
	if pastMeeting.Timezone != "" {
		tags = append(tags, "timezone:"+pastMeeting.Timezone)
	}
	return tags
}

// =============================================================================
// Past Meeting Invitee Event Handler
// =============================================================================

// handlePastMeetingInviteeUpdate processes updates to past meeting invitees
func (h *EventHandlers) handlePastMeetingInviteeUpdate(
	ctx context.Context,
	key string,
	v1Data map[string]interface{},
) bool {
	funcLogger := h.logger.With("key", key, "handler", "past_meeting_invitee")
	funcLogger.DebugContext(ctx, "processing past meeting invitee update")

	// Check if this is a soft delete
	if isDeleted, ok := v1Data["_sdc_deleted_at"].(string); ok && isDeleted != "" {
		return h.handlePastMeetingInviteeDelete(ctx, key, v1Data)
	}

	// Convert v1Data to participant event data
	participantData, err := convertMapToInviteeParticipantData(ctx, v1Data, h.userLookup, h.idMapper, h.v1ObjectsKV, funcLogger)
	if err != nil {
		funcLogger.With(errKey, err).ErrorContext(ctx, "failed to convert v1Data to invitee participant")
		if isTransientError(err) {
			return true // NAK for retry
		}
		return false // Permanent error, ACK and skip
	}

	// Validate required fields
	if participantData.UID == "" || participantData.MeetingAndOccurrenceID == "" {
		funcLogger.ErrorContext(ctx, "missing required fields in invitee participant data")
		return false // Permanent error, ACK and skip
	}
	funcLogger = funcLogger.With("participant_uid", participantData.UID)

	// Determine action (created vs updated)
	mappingKey := fmt.Sprintf("v1_past_meeting_invitees.%s", participantData.UID)
	indexerAction := indexerConstants.ActionCreated
	if _, err := h.v1MappingsKV.Get(ctx, mappingKey); err == nil {
		indexerAction = indexerConstants.ActionUpdated
	}

	// Generate tags
	tags := generateParticipantTags(participantData)

	// Publish to indexer and FGA-sync
	if err := h.publisher.PublishPastMeetingParticipantEvent(ctx, string(indexerAction), participantData, tags); err != nil {
		funcLogger.With(errKey, err).ErrorContext(ctx, "failed to publish invitee participant event")
		if isTransientError(err) {
			return true // NAK for retry
		}
		return false // Permanent error, ACK and skip
	}

	// Store mapping
	if _, err := h.v1MappingsKV.Put(ctx, mappingKey, []byte("1")); err != nil {
		funcLogger.With(errKey, err).WarnContext(ctx, "failed to store invitee participant mapping")
	}

	funcLogger.InfoContext(ctx, "successfully processed past meeting invitee")
	return false // Success, ACK
}

// handlePastMeetingInviteeDelete processes invitee deletions
func (h *EventHandlers) handlePastMeetingInviteeDelete(
	ctx context.Context,
	key string,
	v1Data map[string]interface{},
) bool {
	inviteeID := extractIDFromKey(key, "itx-zoom-past-meetings-invitees.")
	funcLogger := h.logger.With("invitee_id", inviteeID, "handler", "past_meeting_invitee_delete")
	funcLogger.InfoContext(ctx, "processing past meeting invitee deletion")

	// Create minimal event data for deletion
	eventData := &models.PastMeetingParticipantEventData{UID: inviteeID}

	// Generate tags (minimal for deletion)
	tags := generateParticipantTags(eventData)

	// Publish delete event
	if err := h.publisher.PublishPastMeetingParticipantEvent(ctx, string(indexerConstants.ActionDeleted), eventData, tags); err != nil {
		funcLogger.With(errKey, err).ErrorContext(ctx, "failed to publish invitee delete event")
		if isTransientError(err) {
			return true // NAK for retry
		}
		return false // Permanent error, ACK and skip
	}

	// Remove mapping
	mappingKey := fmt.Sprintf("v1_past_meeting_invitees.%s", inviteeID)
	_ = h.v1MappingsKV.Delete(ctx, mappingKey)

	funcLogger.InfoContext(ctx, "successfully processed past meeting invitee deletion")
	return false // Success, ACK
}

// =============================================================================
// Past Meeting Attendee Event Handler
// =============================================================================

// handlePastMeetingAttendeeUpdate processes updates to past meeting attendees
func (h *EventHandlers) handlePastMeetingAttendeeUpdate(
	ctx context.Context,
	key string,
	v1Data map[string]interface{},
) bool {
	funcLogger := h.logger.With("key", key, "handler", "past_meeting_attendee")
	funcLogger.DebugContext(ctx, "processing past meeting attendee update")

	// Check if this is a soft delete
	if isDeleted, ok := v1Data["_sdc_deleted_at"].(string); ok && isDeleted != "" {
		return h.handlePastMeetingAttendeeDelete(ctx, key, v1Data)
	}

	// Convert v1Data to participant event data
	participantData, err := convertMapToAttendeeParticipantData(ctx, v1Data, h.userLookup, h.idMapper, h.v1ObjectsKV, funcLogger)
	if err != nil {
		funcLogger.With(errKey, err).ErrorContext(ctx, "failed to convert v1Data to attendee participant")
		if isTransientError(err) {
			return true // NAK for retry
		}
		return false // Permanent error, ACK and skip
	}

	// Validate required fields
	if participantData.UID == "" || participantData.MeetingAndOccurrenceID == "" {
		funcLogger.ErrorContext(ctx, "missing required fields in attendee participant data")
		return false // Permanent error, ACK and skip
	}
	funcLogger = funcLogger.With("participant_uid", participantData.UID)

	// Determine action (created vs updated)
	mappingKey := fmt.Sprintf("v1_past_meeting_attendees.%s", participantData.UID)
	indexerAction := indexerConstants.ActionCreated
	if _, err := h.v1MappingsKV.Get(ctx, mappingKey); err == nil {
		indexerAction = indexerConstants.ActionUpdated
	}

	// Generate tags
	tags := generateParticipantTags(participantData)

	// Publish to indexer and FGA-sync
	if err := h.publisher.PublishPastMeetingParticipantEvent(ctx, string(indexerAction), participantData, tags); err != nil {
		funcLogger.With(errKey, err).ErrorContext(ctx, "failed to publish attendee participant event")
		if isTransientError(err) {
			return true // NAK for retry
		}
		return false // Permanent error, ACK and skip
	}

	// Store mapping
	if _, err := h.v1MappingsKV.Put(ctx, mappingKey, []byte("1")); err != nil {
		funcLogger.With(errKey, err).WarnContext(ctx, "failed to store attendee participant mapping")
	}

	funcLogger.InfoContext(ctx, "successfully processed past meeting attendee")
	return false // Success, ACK
}

// handlePastMeetingAttendeeDelete processes attendee deletions
func (h *EventHandlers) handlePastMeetingAttendeeDelete(
	ctx context.Context,
	key string,
	v1Data map[string]interface{},
) bool {
	attendeeID := extractIDFromKey(key, "itx-zoom-past-meetings-attendees.")
	funcLogger := h.logger.With("attendee_id", attendeeID, "handler", "past_meeting_attendee_delete")
	funcLogger.InfoContext(ctx, "processing past meeting attendee deletion")

	// Create minimal event data for deletion
	eventData := &models.PastMeetingParticipantEventData{UID: attendeeID}

	// Generate tags (minimal for deletion)
	tags := generateParticipantTags(eventData)

	// Publish delete event
	if err := h.publisher.PublishPastMeetingParticipantEvent(ctx, string(indexerConstants.ActionDeleted), eventData, tags); err != nil {
		funcLogger.With(errKey, err).ErrorContext(ctx, "failed to publish attendee delete event")
		if isTransientError(err) {
			return true // NAK for retry
		}
		return false // Permanent error, ACK and skip
	}

	// Remove mapping
	mappingKey := fmt.Sprintf("v1_past_meeting_attendees.%s", attendeeID)
	_ = h.v1MappingsKV.Delete(ctx, mappingKey)

	funcLogger.InfoContext(ctx, "successfully processed past meeting attendee deletion")
	return false // Success, ACK
}

// =============================================================================
// Participant Conversion Functions
// =============================================================================

func convertMapToInviteeParticipantData(
	ctx context.Context,
	v1Data map[string]interface{},
	userLookup domain.V1UserLookup,
	idMapper domain.IDMapper,
	v1ObjectsKV jetstream.KeyValue,
	logger *slog.Logger,
) (*models.PastMeetingParticipantEventData, error) {
	// Convert map to JSON bytes, then to InviteeDBRaw
	jsonBytes, err := json.Marshal(v1Data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal v1Data to JSON: %w", err)
	}

	var rawInvitee InviteeDBRaw
	if err := json.Unmarshal(jsonBytes, &rawInvitee); err != nil {
		return nil, fmt.Errorf("failed to unmarshal invitee data: %w", err)
	}

	// Validate required fields
	if rawInvitee.ID == "" || rawInvitee.MeetingAndOccurrenceID == "" {
		return nil, fmt.Errorf("missing required fields: id or meeting_and_occurrence_id")
	}

	// Use ProjectID from invitee record directly if available
	projectSFID := rawInvitee.ProjectID
	if projectSFID == "" {
		return nil, fmt.Errorf("invitee missing project ID")
	}

	// Map project ID
	projectUID, err := idMapper.MapProjectV1ToV2(ctx, projectSFID)
	if err != nil {
		return nil, fmt.Errorf("failed to map project ID (transient): %w", err)
	}

	// Determine if host (lookup registrant if available)
	isHost := false
	if rawInvitee.RegistrantID != "" {
		registrantKey := fmt.Sprintf("itx-zoom-meetings-registrants-v2.%s", rawInvitee.RegistrantID)
		if registrantEntry, err := v1ObjectsKV.Get(ctx, registrantKey); err == nil {
			var registrantData map[string]interface{}
			if err := json.Unmarshal(registrantEntry.Value(), &registrantData); err == nil {
				isHost = getBool(registrantData["host"])
			}
		}
	}

	// Username is lf_sso field
	username := rawInvitee.LFSSO

	// Use existing first/last name from invitee record
	firstName := rawInvitee.FirstName
	lastName := rawInvitee.LastName

	// Username resolution via V1UserLookup if lf_user_id exists and we need enrichment
	if rawInvitee.LFUserID != "" && (firstName == "" || lastName == "") {
		v1User, err := userLookup.LookupUser(ctx, rawInvitee.LFUserID)
		if err != nil {
			logger.With(errKey, err).WarnContext(ctx, "failed to lookup v1 user", "lf_user_id", rawInvitee.LFUserID)
		} else if v1User != nil {
			if firstName == "" && v1User.FirstName != "" {
				firstName = v1User.FirstName
			}
			if lastName == "" && v1User.LastName != "" {
				lastName = v1User.LastName
			}
		}
	}

	// Parse times
	createdAt, _ := parseTime(rawInvitee.CreatedAt)
	modifiedAt, _ := parseTime(rawInvitee.ModifiedAt)

	// Get org membership flags
	orgIsMember := false
	if rawInvitee.OrgIsMember != nil {
		orgIsMember = *rawInvitee.OrgIsMember
	}
	orgIsProjectMember := false
	if rawInvitee.OrgIsProjectMember != nil {
		orgIsProjectMember = *rawInvitee.OrgIsProjectMember
	}

	return &models.PastMeetingParticipantEventData{
		UID:                    rawInvitee.ID,
		MeetingAndOccurrenceID: rawInvitee.MeetingAndOccurrenceID,
		MeetingID:              rawInvitee.MeetingID,
		ProjectUID:             projectUID,
		Email:                  rawInvitee.Email,
		FirstName:              firstName,
		LastName:               lastName,
		Host:                   isHost,
		JobTitle:               rawInvitee.JobTitle,
		OrgName:                rawInvitee.Org,
		OrgIsMember:            orgIsMember,
		OrgIsProjectMember:     orgIsProjectMember,
		AvatarURL:              rawInvitee.ProfilePicture,
		Username:               username,
		IsInvited:              true,
		IsAttended:             false,
		Sessions:               nil, // Invitees don't have sessions
		CreatedAt:              createdAt,
		ModifiedAt:             modifiedAt,
	}, nil
}

func convertMapToAttendeeParticipantData(
	ctx context.Context,
	v1Data map[string]interface{},
	userLookup domain.V1UserLookup,
	idMapper domain.IDMapper,
	v1ObjectsKV jetstream.KeyValue,
	logger *slog.Logger,
) (*models.PastMeetingParticipantEventData, error) {
	// Convert map to JSON bytes, then to AttendeeDBRaw
	jsonBytes, err := json.Marshal(v1Data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal v1Data to JSON: %w", err)
	}

	var rawAttendee AttendeeDBRaw
	if err := json.Unmarshal(jsonBytes, &rawAttendee); err != nil {
		return nil, fmt.Errorf("failed to unmarshal attendee data: %w", err)
	}

	// Validate required fields
	if rawAttendee.ID == "" || rawAttendee.MeetingAndOccurrenceID == "" {
		return nil, fmt.Errorf("missing required fields: id or meeting_and_occurrence_id")
	}

	// Use ProjectID from attendee record directly
	projectSFID := rawAttendee.ProjectID
	if projectSFID == "" {
		return nil, fmt.Errorf("attendee missing project ID")
	}

	// Map project ID
	projectUID, err := idMapper.MapProjectV1ToV2(ctx, projectSFID)
	if err != nil {
		return nil, fmt.Errorf("failed to map project ID (transient): %w", err)
	}

	// Check if this user was also invited (registrant_id present)
	isInvited := rawAttendee.RegistrantID != ""

	// Parse name
	firstName, lastName := parseName(rawAttendee.Name)

	// Username is lf_sso field
	username := rawAttendee.LFSSO

	// Username resolution via V1UserLookup if lf_user_id exists and we need enrichment
	if rawAttendee.LFUserID != "" && (firstName == "" || lastName == "") {
		v1User, err := userLookup.LookupUser(ctx, rawAttendee.LFUserID)
		if err != nil {
			logger.With(errKey, err).WarnContext(ctx, "failed to lookup v1 user", "lf_user_id", rawAttendee.LFUserID)
		} else if v1User != nil {
			if firstName == "" && v1User.FirstName != "" {
				firstName = v1User.FirstName
			}
			if lastName == "" && v1User.LastName != "" {
				lastName = v1User.LastName
			}
		}
	}

	// Convert sessions
	var sessions []models.ParticipantSession
	for _, rawSession := range rawAttendee.Sessions {
		joinTime, _ := parseTime(rawSession.JoinTime)
		leaveTime, _ := parseTime(rawSession.LeaveTime)
		sessions = append(sessions, models.ParticipantSession{
			UID:         rawSession.ParticipantUUID,
			JoinTime:    &joinTime,
			LeaveTime:   &leaveTime,
			LeaveReason: rawSession.LeaveReason,
		})
	}

	// Parse times
	createdAt, _ := parseTime(rawAttendee.CreatedAt)
	modifiedAt, _ := parseTime(rawAttendee.ModifiedAt)

	// Get org membership flags
	orgIsMember := false
	if rawAttendee.OrgIsMember != nil {
		orgIsMember = *rawAttendee.OrgIsMember
	}
	orgIsProjectMember := false
	if rawAttendee.OrgIsProjectMember != nil {
		orgIsProjectMember = *rawAttendee.OrgIsProjectMember
	}

	return &models.PastMeetingParticipantEventData{
		UID:                    rawAttendee.ID,
		MeetingAndOccurrenceID: rawAttendee.MeetingAndOccurrenceID,
		MeetingID:              rawAttendee.MeetingID,
		ProjectUID:             projectUID,
		Email:                  rawAttendee.Email,
		FirstName:              firstName,
		LastName:               lastName,
		Host:                   false, // Attendee records don't have host info
		JobTitle:               rawAttendee.JobTitle,
		OrgName:                rawAttendee.Org,
		OrgIsMember:            orgIsMember,
		OrgIsProjectMember:     orgIsProjectMember,
		AvatarURL:              rawAttendee.ProfilePicture,
		Username:               username,
		IsInvited:              isInvited,
		IsAttended:             true,
		Sessions:               sessions,
		CreatedAt:              createdAt,
		ModifiedAt:             modifiedAt,
	}, nil
}

// parseName splits a full name into first and last name
func parseName(fullName string) (firstName, lastName string) {
	if fullName == "" {
		return "", ""
	}

	parts := strings.Fields(fullName)
	if len(parts) == 0 {
		return "", ""
	}
	if len(parts) == 1 {
		return parts[0], ""
	}
	// First part is first name, everything else is last name
	return parts[0], strings.Join(parts[1:], " ")
}

func generateParticipantTags(participant *models.PastMeetingParticipantEventData) []string {
	tags := []string{
		"past_meeting_participant_uid:" + participant.UID,
		"meeting_and_occurrence_id:" + participant.MeetingAndOccurrenceID,
		"project_uid:" + participant.ProjectUID,
	}
	if participant.Username != "" {
		tags = append(tags, "username:"+participant.Username)
	}
	if participant.Email != "" {
		tags = append(tags, "email:"+participant.Email)
	}
	if participant.IsInvited {
		tags = append(tags, "is_invited:true")
	}
	if participant.IsAttended {
		tags = append(tags, "is_attended:true")
	}
	return tags
}

// =============================================================================
// Past Meeting Recording Event Handler
// =============================================================================

// handlePastMeetingRecordingUpdate processes updates to past meeting recordings
func (h *EventHandlers) handlePastMeetingRecordingUpdate(
	ctx context.Context,
	key string,
	v1Data map[string]interface{},
) bool {
	funcLogger := h.logger.With("key", key, "handler", "past_meeting_recording")
	funcLogger.DebugContext(ctx, "processing past meeting recording update")

	// Check if this is a soft delete
	if isDeleted, ok := v1Data["_sdc_deleted_at"].(string); ok && isDeleted != "" {
		return h.handlePastMeetingRecordingDelete(ctx, key, v1Data)
	}

	// Convert v1Data to recording event data
	recordingData, transcriptData, err := convertMapToRecordingData(ctx, v1Data, h.idMapper, funcLogger)
	if err != nil {
		funcLogger.With(errKey, err).ErrorContext(ctx, "failed to convert v1Data to recording")
		if isTransientError(err) {
			return true // NAK for retry
		}
		return false // Permanent error, ACK and skip
	}

	// Validate required fields
	if recordingData.ID == "" || recordingData.MeetingAndOccurrenceID == "" {
		funcLogger.ErrorContext(ctx, "missing required fields in recording data")
		return false // Permanent error, ACK and skip
	}
	funcLogger = funcLogger.With("recording_id", recordingData.ID)

	// Determine action (created vs updated)
	mappingKey := fmt.Sprintf("v1_past_meeting_recordings.%s", recordingData.ID)
	indexerAction := indexerConstants.ActionCreated
	if _, err := h.v1MappingsKV.Get(ctx, mappingKey); err == nil {
		indexerAction = indexerConstants.ActionUpdated
	}

	// Generate recording tags
	recordingTags := generateRecordingTags(recordingData.ID, recordingData.MeetingAndOccurrenceID, recordingData.PlatformMeetingID, recordingData.Sessions)

	// Publish recording event to indexer and FGA-sync
	if err := h.publisher.PublishPastMeetingRecordingEvent(ctx, string(indexerAction), recordingData, recordingTags); err != nil {
		funcLogger.With(errKey, err).ErrorContext(ctx, "failed to publish recording event")
		if isTransientError(err) {
			return true // NAK for retry
		}
		return false // Permanent error, ACK and skip
	}

	// If transcript is enabled, publish separate transcript event
	if transcriptData != nil {
		transcriptTags := generateTranscriptTags(transcriptData.ID, transcriptData.MeetingAndOccurrenceID)
		if err := h.publisher.PublishPastMeetingTranscriptEvent(ctx, string(indexerAction), transcriptData, transcriptTags); err != nil {
			funcLogger.With(errKey, err).ErrorContext(ctx, "failed to publish transcript event")
			if isTransientError(err) {
				return true // NAK for retry
			}
			return false // Permanent error, ACK and skip
		}
	}

	// Store mapping
	if _, err := h.v1MappingsKV.Put(ctx, mappingKey, []byte("1")); err != nil {
		funcLogger.With(errKey, err).WarnContext(ctx, "failed to store recording mapping")
	}

	funcLogger.InfoContext(ctx, "successfully processed past meeting recording")
	return false // Success, ACK
}

// handlePastMeetingRecordingDelete processes recording deletions
func (h *EventHandlers) handlePastMeetingRecordingDelete(
	ctx context.Context,
	key string,
	v1Data map[string]interface{},
) bool {
	recordingID := extractIDFromKey(key, "itx-zoom-past-meetings-recordings.")
	funcLogger := h.logger.With("recording_id", recordingID, "handler", "past_meeting_recording_delete")
	funcLogger.InfoContext(ctx, "processing past meeting recording deletion")

	// Create minimal event data for deletion
	eventData := &models.RecordingEventData{ID: recordingID}

	// Generate tags (minimal for deletion)
	recordingTags := generateRecordingTags(recordingID, "", "", nil)

	// Publish recording delete event
	if err := h.publisher.PublishPastMeetingRecordingEvent(ctx, string(indexerConstants.ActionDeleted), eventData, recordingTags); err != nil {
		funcLogger.With(errKey, err).ErrorContext(ctx, "failed to publish recording delete event")
		if isTransientError(err) {
			return true // NAK for retry
		}
		return false // Permanent error, ACK and skip
	}

	// Also publish transcript delete event
	transcriptData := &models.TranscriptEventData{ID: recordingID}
	transcriptTags := generateTranscriptTags(recordingID, "")
	if err := h.publisher.PublishPastMeetingTranscriptEvent(ctx, string(indexerConstants.ActionDeleted), transcriptData, transcriptTags); err != nil {
		funcLogger.With(errKey, err).WarnContext(ctx, "failed to publish transcript delete event")
	}

	// Remove mapping
	mappingKey := fmt.Sprintf("v1_past_meeting_recordings.%s", recordingID)
	_ = h.v1MappingsKV.Delete(ctx, mappingKey)

	funcLogger.InfoContext(ctx, "successfully processed past meeting recording deletion")
	return false // Success, ACK
}

// =============================================================================
// Past Meeting Summary Event Handler
// =============================================================================

// handlePastMeetingSummaryUpdate processes updates to past meeting summaries
func (h *EventHandlers) handlePastMeetingSummaryUpdate(
	ctx context.Context,
	key string,
	v1Data map[string]interface{},
) bool {
	funcLogger := h.logger.With("key", key, "handler", "past_meeting_summary")
	funcLogger.DebugContext(ctx, "processing past meeting summary update")

	// Check if this is a soft delete
	if isDeleted, ok := v1Data["_sdc_deleted_at"].(string); ok && isDeleted != "" {
		return h.handlePastMeetingSummaryDelete(ctx, key, v1Data)
	}

	// Convert v1Data to summary event data
	summaryData, err := convertMapToSummaryData(ctx, v1Data, h.idMapper, funcLogger)
	if err != nil {
		funcLogger.With(errKey, err).ErrorContext(ctx, "failed to convert v1Data to summary")
		if isTransientError(err) {
			return true // NAK for retry
		}
		return false // Permanent error, ACK and skip
	}

	// Validate required fields
	if summaryData.ID == "" || summaryData.MeetingAndOccurrenceID == "" {
		funcLogger.ErrorContext(ctx, "missing required fields in summary data")
		return false // Permanent error, ACK and skip
	}
	funcLogger = funcLogger.With("summary_id", summaryData.ID)

	// Determine action (created vs updated)
	mappingKey := fmt.Sprintf("v1_past_meeting_summaries.%s", summaryData.ID)
	indexerAction := indexerConstants.ActionCreated
	if _, err := h.v1MappingsKV.Get(ctx, mappingKey); err == nil {
		indexerAction = indexerConstants.ActionUpdated
	}

	// Generate tags
	tags := generateSummaryTags(summaryData)

	// Publish to indexer and FGA-sync
	if err := h.publisher.PublishPastMeetingSummaryEvent(ctx, string(indexerAction), summaryData, tags); err != nil {
		funcLogger.With(errKey, err).ErrorContext(ctx, "failed to publish summary event")
		if isTransientError(err) {
			return true // NAK for retry
		}
		return false // Permanent error, ACK and skip
	}

	// Store mapping
	if _, err := h.v1MappingsKV.Put(ctx, mappingKey, []byte("1")); err != nil {
		funcLogger.With(errKey, err).WarnContext(ctx, "failed to store summary mapping")
	}

	funcLogger.InfoContext(ctx, "successfully processed past meeting summary")
	return false // Success, ACK
}

// handlePastMeetingSummaryDelete processes summary deletions
func (h *EventHandlers) handlePastMeetingSummaryDelete(
	ctx context.Context,
	key string,
	v1Data map[string]interface{},
) bool {
	summaryID := extractIDFromKey(key, "itx-zoom-past-meetings-summaries.")
	funcLogger := h.logger.With("summary_id", summaryID, "handler", "past_meeting_summary_delete")
	funcLogger.InfoContext(ctx, "processing past meeting summary deletion")

	// Create minimal event data for deletion
	eventData := &models.SummaryEventData{ID: summaryID}

	// Generate tags (minimal for deletion)
	tags := generateSummaryTags(eventData)

	// Publish delete event
	if err := h.publisher.PublishPastMeetingSummaryEvent(ctx, string(indexerConstants.ActionDeleted), eventData, tags); err != nil {
		funcLogger.With(errKey, err).ErrorContext(ctx, "failed to publish summary delete event")
		if isTransientError(err) {
			return true // NAK for retry
		}
		return false // Permanent error, ACK and skip
	}

	// Remove mapping
	mappingKey := fmt.Sprintf("v1_past_meeting_summaries.%s", summaryID)
	_ = h.v1MappingsKV.Delete(ctx, mappingKey)

	funcLogger.InfoContext(ctx, "successfully processed past meeting summary deletion")
	return false // Success, ACK
}

// =============================================================================
// Recording Conversion Functions
// =============================================================================

func convertMapToRecordingData(
	ctx context.Context,
	v1Data map[string]interface{},
	idMapper domain.IDMapper,
	logger *slog.Logger,
) (*models.RecordingEventData, *models.TranscriptEventData, error) {
	// Convert map to JSON bytes, then to RecordingDBRaw
	jsonBytes, err := json.Marshal(v1Data)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal v1Data to JSON: %w", err)
	}

	var rawRecording RecordingDBRaw
	if err := json.Unmarshal(jsonBytes, &rawRecording); err != nil {
		return nil, nil, fmt.Errorf("failed to unmarshal recording data: %w", err)
	}

	// Validate required fields
	if rawRecording.ID == "" || rawRecording.MeetingAndOccurrenceID == "" {
		return nil, nil, fmt.Errorf("missing required fields: id or meeting_and_occurrence_id")
	}

	// Use ProjectID from recording record directly
	projectSFID := rawRecording.ProjectID
	if projectSFID == "" {
		return nil, nil, fmt.Errorf("recording missing project ID")
	}

	// Map project ID
	projectUID, err := idMapper.MapProjectV1ToV2(ctx, projectSFID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to map project ID (transient): %w", err)
	}

	// Default recording access to meeting_hosts (most restrictive)
	recordingAccess := rawRecording.RecordingAccess
	if recordingAccess == "" {
		recordingAccess = "meeting_hosts"
	}

	// Convert recording files
	var recordingFiles []models.RecordingFile
	hasTranscript := false
	for _, rawFile := range rawRecording.RecordingFiles {
		recordingStart, _ := parseTime(rawFile.RecordingStart)
		recordingEnd, _ := parseTime(rawFile.RecordingEnd)

		recordingFiles = append(recordingFiles, models.RecordingFile{
			DownloadURL:    rawFile.DownloadURL,
			FileExtension:  rawFile.FileExtension,
			FileSize:       rawFile.FileSize,
			FileType:       rawFile.FileType,
			ID:             rawFile.ID,
			MeetingID:      rawFile.MeetingID,
			PlayURL:        rawFile.PlayURL,
			RecordingStart: recordingStart,
			RecordingEnd:   recordingEnd,
			RecordingType:  rawFile.RecordingType,
			Status:         rawFile.Status,
		})

		// Check if this is a transcript file
		if rawFile.FileType == "TRANSCRIPT" || rawFile.FileType == "TIMELINE" {
			hasTranscript = true
		}
	}

	// Convert recording sessions
	var sessions []models.RecordingSession
	for _, rawSession := range rawRecording.Sessions {
		startTime, _ := parseTime(rawSession.StartTime)
		sessions = append(sessions, models.RecordingSession{
			UUID:      rawSession.UUID,
			ShareURL:  rawSession.ShareURL,
			TotalSize: rawSession.TotalSize,
			StartTime: startTime,
		})
	}

	// Parse times
	startTime, _ := parseTime(rawRecording.StartTime)
	createdAt, _ := parseTime(rawRecording.CreatedAt)
	updatedAt, _ := parseTime(rawRecording.ModifiedAt)

	// Set transcript enabled flag
	transcriptEnabled := hasTranscript
	transcriptAccess := rawRecording.TranscriptAccess
	if hasTranscript && transcriptAccess == "" {
		transcriptAccess = "meeting_hosts" // Default to most restrictive
	}

	recordingData := &models.RecordingEventData{
		ID:                     rawRecording.ID,
		MeetingAndOccurrenceID: rawRecording.MeetingAndOccurrenceID,
		ProjectUID:             projectUID,
		HostEmail:              rawRecording.HostEmail,
		HostID:                 rawRecording.HostID,
		MeetingID:              rawRecording.MeetingID,
		OccurrenceID:           rawRecording.OccurrenceID,
		Platform:               "Zoom",
		PlatformMeetingID:      rawRecording.PlatformMeetingID,
		RecordingAccess:        recordingAccess,
		Title:                  rawRecording.Topic, // Map topic to title
		TranscriptAccess:       transcriptAccess,
		TranscriptEnabled:      transcriptEnabled,
		Visibility:             rawRecording.Visibility,
		RecordingCount:         rawRecording.RecordingCount,
		RecordingFiles:         recordingFiles,
		Sessions:               sessions,
		StartTime:              startTime,
		TotalSize:              rawRecording.TotalSize,
		CreatedAt:              createdAt,
		UpdatedAt:              updatedAt,
	}

	// Create transcript event data if transcript is enabled
	var transcriptData *models.TranscriptEventData
	if hasTranscript {
		transcriptData = &models.TranscriptEventData{
			ID:                     rawRecording.ID,
			MeetingAndOccurrenceID: rawRecording.MeetingAndOccurrenceID,
			ProjectUID:             projectUID,
			TranscriptAccess:       transcriptAccess,
			Platform:               "Zoom",
		}
	}

	return recordingData, transcriptData, nil
}

func generateRecordingTags(id, meetingAndOccurrenceID, platformMeetingID string, sessions []models.RecordingSession) []string {
	tags := []string{
		id,
		"past_meeting_recording_id:" + id,
		"meeting_and_occurrence_id:" + meetingAndOccurrenceID,
		"platform:Zoom",
		"platform_meeting_id:" + platformMeetingID,
	}
	for _, session := range sessions {
		tags = append(tags, "platform_meeting_instance_id:"+session.UUID)
	}
	return tags
}

func generateTranscriptTags(id, meetingAndOccurrenceID string) []string {
	return []string{
		id,
		"past_meeting_transcript_id:" + id,
		"meeting_and_occurrence_id:" + meetingAndOccurrenceID,
		"platform:Zoom",
	}
}

// =============================================================================
// Summary Conversion Functions
// =============================================================================

func convertMapToSummaryData(
	ctx context.Context,
	v1Data map[string]interface{},
	idMapper domain.IDMapper,
	logger *slog.Logger,
) (*models.SummaryEventData, error) {
	// Convert map to JSON bytes, then to SummaryDBRaw
	jsonBytes, err := json.Marshal(v1Data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal v1Data to JSON: %w", err)
	}

	var rawSummary SummaryDBRaw
	if err := json.Unmarshal(jsonBytes, &rawSummary); err != nil {
		return nil, fmt.Errorf("failed to unmarshal summary data: %w", err)
	}

	// Validate required fields
	if rawSummary.ID == "" || rawSummary.MeetingAndOccurrenceID == "" {
		return nil, fmt.Errorf("missing required fields: id or meeting_and_occurrence_id")
	}

	// Get project ID - may need to look up from past meeting if not in summary
	projectSFID := rawSummary.ProjectID
	if projectSFID == "" {
		// Project ID should be available, but if not this is not fatal
		logger.WarnContext(ctx, "summary missing project ID", "summary_id", rawSummary.ID)
	}

	// Map project ID
	var projectUID string
	if projectSFID != "" {
		projectUID, err = idMapper.MapProjectV1ToV2(ctx, projectSFID)
		if err != nil {
			return nil, fmt.Errorf("failed to map project ID (transient): %w", err)
		}
	}

	// Consolidate original summary fields into markdown content
	content := buildSummaryMarkdown(rawSummary.SummaryOverview, rawSummary.SummaryDetails, rawSummary.NextSteps)

	// Consolidate edited summary fields into markdown edited content
	editedContent := buildSummaryMarkdown(rawSummary.EditedSummaryOverview, rawSummary.EditedSummaryDetails, rawSummary.EditedNextSteps)

	// Parse times
	createdAt, _ := parseTime(rawSummary.CreatedAt)
	updatedAt, _ := parseTime(rawSummary.ModifiedAt)

	return &models.SummaryEventData{
		ID:                     rawSummary.ID,
		MeetingAndOccurrenceID: rawSummary.MeetingAndOccurrenceID,
		ProjectUID:             projectUID,
		MeetingID:              rawSummary.MeetingID,
		OccurrenceID:           rawSummary.OccurrenceID,
		ZoomMeetingUUID:        rawSummary.ZoomMeetingUUID,
		ZoomMeetingHostID:      rawSummary.ZoomMeetingHostID,
		ZoomMeetingHostEmail:   rawSummary.ZoomMeetingHostEmail,
		ZoomMeetingTopic:       rawSummary.ZoomMeetingTopic,
		Content:                content,
		EditedContent:          editedContent,
		RequiresApproval:       rawSummary.RequiresApproval,
		Approved:               rawSummary.Approved,
		Platform:               "Zoom",
		ZoomConfig: models.SummaryZoomConfig{
			MeetingID:   rawSummary.MeetingID,
			MeetingUUID: rawSummary.ZoomMeetingUUID,
		},
		EmailSent: rawSummary.EmailSent,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}, nil
}

// buildSummaryMarkdown consolidates sparse summary fields into markdown format
func buildSummaryMarkdown(overview string, details []SummaryDetailDBRaw, nextSteps []string) string {
	if overview == "" && len(details) == 0 && len(nextSteps) == 0 {
		return ""
	}

	var sb strings.Builder

	if overview != "" {
		sb.WriteString("## Overview\n")
		sb.WriteString(overview)
		sb.WriteString("\n\n")
	}

	if len(details) > 0 {
		sb.WriteString("## Key Topics\n")
		for _, detail := range details {
			if detail.Label != "" {
				sb.WriteString("### ")
				sb.WriteString(detail.Label)
				sb.WriteString("\n")
			}
			if detail.Summary != "" {
				sb.WriteString(detail.Summary)
				sb.WriteString("\n\n")
			}
		}
	}

	if len(nextSteps) > 0 {
		sb.WriteString("## Next Steps\n")
		for _, step := range nextSteps {
			if step != "" {
				sb.WriteString("- ")
				sb.WriteString(step)
				sb.WriteString("\n")
			}
		}
	}

	return strings.TrimSpace(sb.String())
}

func generateSummaryTags(summary *models.SummaryEventData) []string {
	tags := []string{
		summary.ID,
		"past_meeting_summary_id:" + summary.ID,
		"meeting_and_occurrence_id:" + summary.MeetingAndOccurrenceID,
		"meeting_id:" + summary.MeetingID,
		"platform:Zoom",
	}
	if summary.ZoomMeetingTopic != "" {
		tags = append(tags, "title:"+summary.ZoomMeetingTopic)
	}
	return tags
}
