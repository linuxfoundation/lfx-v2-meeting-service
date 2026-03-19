// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	indexerConstants "github.com/linuxfoundation/lfx-v2-indexer-service/pkg/constants"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
	itx "github.com/linuxfoundation/lfx-v2-meeting-service/pkg/models/itx"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/utils"
	"github.com/nats-io/nats.go/jetstream"
)

// =============================================================================
// Past Meeting Event Handler
// =============================================================================

// handlePastMeetingUpdate processes updates to past meeting records
func (h *EventHandlers) handlePastMeetingUpdate(
	ctx context.Context,
	key string,
	v1Data map[string]interface{},
) (retry bool) {
	funcLogger := h.logger.With("key", key, "handler", "past_meeting")
	funcLogger.DebugContext(ctx, "processing past meeting update")

	// Check if this is a soft delete
	if isDeleted, ok := v1Data["_sdc_deleted_at"].(string); ok && isDeleted != "" {
		return h.handlePastMeetingDelete(ctx, key, v1Data)
	}

	// Convert v1Data to past meeting event data
	pastMeetingData, err := convertMapToPastMeetingData(ctx, v1Data, h.idMapper, h.v1ObjectsKV, h.v1MappingsKV, funcLogger)
	if err != nil {
		funcLogger.With(logging.ErrKey, err).ErrorContext(ctx, "failed to convert v1Data to past meeting")
		return isTransientError(err)
	}

	// Validate required fields
	if pastMeetingData.ID == "" || pastMeetingData.ProjectUID == "" {
		funcLogger.ErrorContext(ctx, "missing required fields in past meeting data")
		return false
	}
	funcLogger = funcLogger.With("past_meeting_id", pastMeetingData.ID)

	// Determine action (created vs updated)
	mappingKey := fmt.Sprintf("v1_past_meetings.%s", pastMeetingData.ID)
	indexerAction := indexerConstants.ActionCreated
	if _, err := h.v1MappingsKV.Get(ctx, mappingKey); err == nil {
		indexerAction = indexerConstants.ActionUpdated
	}

	// Publish to indexer and FGA-sync
	if err := h.publisher.PublishPastMeetingEvent(ctx, string(indexerAction), pastMeetingData); err != nil {
		funcLogger.With(logging.ErrKey, err).ErrorContext(ctx, "failed to publish past meeting event")
		return isTransientError(err)
	}

	// Store mapping
	if _, err := h.v1MappingsKV.Put(ctx, mappingKey, []byte("1")); err != nil {
		funcLogger.With(logging.ErrKey, err).WarnContext(ctx, "failed to store past meeting mapping")
	}

	funcLogger.InfoContext(ctx, "successfully processed past meeting")
	return false // Success, ACK
}

// handlePastMeetingDelete processes past meeting deletions
func (h *EventHandlers) handlePastMeetingDelete(ctx context.Context, key string, _ map[string]interface{}) (retry bool) {
	pastMeetingID := extractIDFromKey(key, "itx-zoom-past-meetings.")
	mappingKey := fmt.Sprintf("v1_past_meetings.%s", pastMeetingID)
	if h.isTombstoned(ctx, mappingKey) {
		h.logger.DebugContext(ctx, "past meeting delete already processed, skipping", "past_meeting_id", pastMeetingID)
		return false
	}
	return h.handleMeetingTypeDelete(ctx, key, pastMeetingID, []byte(pastMeetingID), meetingDeleteConfig{
		indexerSubject:         "lfx.index.v1_past_meeting",
		deleteAllAccessSubject: "lfx.delete_all_access.v1_past_meeting",
		tombstoneKeyFmts:       []string{"v1_past_meetings.%s"},
	})
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
		Duration:         utils.GetInt(rawPastMeeting.Duration),
		Timezone:         rawPastMeeting.Timezone,
		ParticipantCount: utils.GetInt(rawPastMeeting.ParticipantsCount),
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
) (retry bool) {
	funcLogger := h.logger.With("key", key, "handler", "past_meeting_mapping")
	funcLogger.InfoContext(ctx, "processing past meeting mapping update")

	// Check if this is a soft delete
	if isDeleted, ok := v1Data["_sdc_deleted_at"].(string); ok && isDeleted != "" {
		return h.handlePastMeetingMappingDelete(ctx, key, v1Data)
	}

	// Extract past meeting ID and mapping data
	pastMeetingUUID := utils.GetString(v1Data["past_meeting_uuid"])
	mappingID := utils.GetString(v1Data["id"])
	committeeID := utils.GetString(v1Data["committee_id"])

	if pastMeetingUUID == "" || mappingID == "" {
		funcLogger.WarnContext(ctx, "missing required fields in past meeting mapping")
		return false
	}

	// Update committee mappings in KV bucket
	if err := updatePastMeetingCommitteeMappings(ctx, pastMeetingUUID, mappingID, committeeID, v1Data, h.v1MappingsKV, funcLogger); err != nil {
		funcLogger.With(logging.ErrKey, err).ErrorContext(ctx, "failed to update past meeting committee mappings")
		return isTransientError(err)
	}

	// Re-trigger past meeting indexing
	shouldRetry := h.retriggerPastMeetingIndexing(ctx, pastMeetingUUID)
	if shouldRetry {
		return true
	}

	funcLogger.InfoContext(ctx, "successfully processed past meeting mapping update")
	return false
}

// handlePastMeetingMappingDelete processes past meeting-committee mapping deletions
func (h *EventHandlers) handlePastMeetingMappingDelete(
	ctx context.Context,
	key string,
	v1Data map[string]interface{},
) (retry bool) {
	if v1Data == nil {
		h.logger.WarnContext(ctx, "no v1Data available for past meeting mapping delete, skipping", "key", key)
		return false
	}

	pastMeetingUUID := utils.GetString(v1Data["past_meeting_uuid"])
	mappingID := extractIDFromKey(key, "itx-zoom-past-meetings-mappings.")

	if pastMeetingUUID == "" || mappingID == "" {
		h.logger.WarnContext(ctx, "missing required fields in past meeting mapping deletion")
		return false
	}

	funcLogger := h.logger.With("past_meeting_uuid", pastMeetingUUID, "mapping_id", mappingID, "handler", "past_meeting_mapping_delete")
	funcLogger.InfoContext(ctx, "processing past meeting mapping deletion")

	// Remove the mapping
	if err := removePastMeetingCommitteeMapping(ctx, pastMeetingUUID, mappingID, h.v1MappingsKV, funcLogger); err != nil {
		if isTransientError(err) {
			return true
		}
	}

	// Re-trigger past meeting indexing
	shouldRetry := h.retriggerPastMeetingIndexing(ctx, pastMeetingUUID)
	if shouldRetry {
		return true
	}

	funcLogger.InfoContext(ctx, "successfully processed past meeting mapping deletion")
	return false
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
		if !errors.Is(err, jetstream.ErrKeyNotFound) {
			logger.With(logging.ErrKey, err).WarnContext(ctx, "failed to load past meeting committee mappings", "key", key)
		}
		return nil
	}

	var mappings map[string]map[string]interface{}
	if err := json.Unmarshal(entry.Value(), &mappings); err != nil {
		logger.With(logging.ErrKey, err).WarnContext(ctx, "failed to unmarshal past meeting committee mappings")
		return nil
	}

	committees := make([]models.Committee, 0, len(mappings))
	for _, mapping := range mappings {
		committeeID := utils.GetStringFromMap(mapping, "committee_id")
		if committeeID != "" {
			committeeUID, err := idMapper.MapCommitteeV1ToV2(ctx, committeeID)
			if err != nil {
				logger.With(logging.ErrKey, err).WarnContext(ctx, "failed to map committee ID", "v1_id", committeeID)
				continue
			}

			filters := utils.GetStringSliceFromMap(mapping, "committee_filters")
			committees = append(committees, models.Committee{
				UID:                   committeeUID,
				AllowedVotingStatuses: utils.CastSlice[itx.CommitteeFilter](filters),
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
		if !errors.Is(err, jetstream.ErrKeyNotFound) {
			return fmt.Errorf("failed to load past meeting mappings: %w", err)
		}
		mappings = make(map[string]map[string]interface{})
	} else {
		if err := json.Unmarshal(entry.Value(), &mappings); err != nil {
			return fmt.Errorf("failed to unmarshal existing past meeting mappings: %w", err)
		}
	}

	// Update or add mapping
	mappingData := map[string]interface{}{"committee_id": committeeID}
	if filters := utils.GetStringSliceFromMap(v1Data, "committee_filters"); len(filters) > 0 {
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
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			return nil // No mappings found
		}
		return fmt.Errorf("failed to load past meeting mappings: %w", err)
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
) (retry bool) {
	pastMeetingKey := fmt.Sprintf("itx-zoom-past-meetings.%s", pastMeetingUUID)
	pastMeetingEntry, err := h.v1ObjectsKV.Get(ctx, pastMeetingKey)
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			h.logger.With(logging.ErrKey, err).WarnContext(ctx, "past meeting not found during retrigger, may be deleted")
			return false
		}
		h.logger.With(logging.ErrKey, err).ErrorContext(ctx, "transient error fetching past meeting during retrigger")
		return true
	}

	var pastMeetingData map[string]interface{}
	if err := json.Unmarshal(pastMeetingEntry.Value(), &pastMeetingData); err != nil {
		h.logger.With(logging.ErrKey, err).ErrorContext(ctx, "failed to unmarshal past meeting data")
		return false
	}

	// Re-process the past meeting
	return h.handlePastMeetingUpdate(ctx, pastMeetingKey, pastMeetingData)
}

// =============================================================================
// Past Meeting Invitee Event Handler
// =============================================================================

// handlePastMeetingInviteeUpdate processes updates to past meeting invitees
func (h *EventHandlers) handlePastMeetingInviteeUpdate(
	ctx context.Context,
	key string,
	v1Data map[string]interface{},
) (retry bool) {
	funcLogger := h.logger.With("key", key, "handler", "past_meeting_invitee")
	funcLogger.DebugContext(ctx, "processing past meeting invitee update")

	// Check if this is a soft delete
	if isDeleted, ok := v1Data["_sdc_deleted_at"].(string); ok && isDeleted != "" {
		return h.handlePastMeetingInviteeDelete(ctx, key, v1Data)
	}

	// Convert v1Data to participant event data
	participantData, err := convertMapToInviteeParticipantData(ctx, v1Data, h.userLookup, h.idMapper, h.v1ObjectsKV, funcLogger)
	if err != nil {
		funcLogger.With(logging.ErrKey, err).ErrorContext(ctx, "failed to convert v1Data to invitee participant")
		return isTransientError(err)
	}

	// Validate required fields
	if participantData.UID == "" || participantData.MeetingAndOccurrenceID == "" {
		funcLogger.ErrorContext(ctx, "missing required fields in invitee participant data")
		return false
	}
	funcLogger = funcLogger.With("participant_uid", participantData.UID)

	// Determine action (created vs updated)
	mappingKey := fmt.Sprintf("v1_past_meeting_invitees.%s", participantData.UID)
	indexerAction := indexerConstants.ActionCreated
	if _, err := h.v1MappingsKV.Get(ctx, mappingKey); err == nil {
		indexerAction = indexerConstants.ActionUpdated
	}

	// Publish to indexer and FGA-sync
	if err := h.publisher.PublishPastMeetingParticipantEvent(ctx, string(indexerAction), participantData); err != nil {
		funcLogger.With(logging.ErrKey, err).ErrorContext(ctx, "failed to publish invitee participant event")
		return isTransientError(err)
	}

	// Store mapping
	if _, err := h.v1MappingsKV.Put(ctx, mappingKey, []byte("1")); err != nil {
		funcLogger.With(logging.ErrKey, err).WarnContext(ctx, "failed to store invitee participant mapping")
	}

	funcLogger.InfoContext(ctx, "successfully processed past meeting invitee")
	return false // Success, ACK
}

// handlePastMeetingInviteeDelete processes invitee deletions
func (h *EventHandlers) handlePastMeetingInviteeDelete(ctx context.Context, key string, _ map[string]interface{}) (retry bool) {
	inviteeID := extractIDFromKey(key, "itx-zoom-past-meetings-invitees.")
	mappingKey := fmt.Sprintf("v1_past_meeting_invitees.%s", inviteeID)
	if h.isTombstoned(ctx, mappingKey) {
		h.logger.DebugContext(ctx, "invitee delete already processed, skipping", "invitee_id", inviteeID)
		return false
	}
	return h.handleMeetingTypeDelete(ctx, key, inviteeID, []byte(inviteeID), meetingDeleteConfig{
		indexerSubject:   "lfx.index.v1_past_meeting_participant",
		tombstoneKeyFmts: []string{"v1_past_meeting_invitees.%s"},
	})
}

// =============================================================================
// Past Meeting Attendee Event Handler
// =============================================================================

// handlePastMeetingAttendeeUpdate processes updates to past meeting attendees
func (h *EventHandlers) handlePastMeetingAttendeeUpdate(
	ctx context.Context,
	key string,
	v1Data map[string]interface{},
) (retry bool) {
	funcLogger := h.logger.With("key", key, "handler", "past_meeting_attendee")
	funcLogger.DebugContext(ctx, "processing past meeting attendee update")

	// Check if this is a soft delete
	if isDeleted, ok := v1Data["_sdc_deleted_at"].(string); ok && isDeleted != "" {
		return h.handlePastMeetingAttendeeDelete(ctx, key, v1Data)
	}

	// Convert v1Data to participant event data
	participantData, err := convertMapToAttendeeParticipantData(ctx, v1Data, h.userLookup, h.idMapper, h.v1ObjectsKV, funcLogger)
	if err != nil {
		funcLogger.With(logging.ErrKey, err).ErrorContext(ctx, "failed to convert v1Data to attendee participant")
		return isTransientError(err)
	}

	// Validate required fields
	if participantData.UID == "" || participantData.MeetingAndOccurrenceID == "" {
		funcLogger.ErrorContext(ctx, "missing required fields in attendee participant data")
		return false
	}
	funcLogger = funcLogger.With("participant_uid", participantData.UID)

	// Determine action (created vs updated)
	mappingKey := fmt.Sprintf("v1_past_meeting_attendees.%s", participantData.UID)
	indexerAction := indexerConstants.ActionCreated
	if _, err := h.v1MappingsKV.Get(ctx, mappingKey); err == nil {
		indexerAction = indexerConstants.ActionUpdated
	}

	// Publish to indexer and FGA-sync
	if err := h.publisher.PublishPastMeetingParticipantEvent(ctx, string(indexerAction), participantData); err != nil {
		funcLogger.With(logging.ErrKey, err).ErrorContext(ctx, "failed to publish attendee participant event")
		return isTransientError(err)
	}

	// Store mapping
	if _, err := h.v1MappingsKV.Put(ctx, mappingKey, []byte("1")); err != nil {
		funcLogger.With(logging.ErrKey, err).WarnContext(ctx, "failed to store attendee participant mapping")
	}

	funcLogger.InfoContext(ctx, "successfully processed past meeting attendee")
	return false // Success, ACK
}

// handlePastMeetingAttendeeDelete processes attendee deletions
func (h *EventHandlers) handlePastMeetingAttendeeDelete(
	ctx context.Context,
	key string,
	v1Data map[string]interface{},
) (retry bool) {
	attendeeID := extractIDFromKey(key, "itx-zoom-past-meetings-attendees.")
	funcLogger := h.logger.With("key", key, "attendee_id", attendeeID)

	mappingKey := fmt.Sprintf("v1_past_meeting_attendees.%s", attendeeID)
	if h.isTombstoned(ctx, mappingKey) {
		funcLogger.DebugContext(ctx, "attendee delete already processed, skipping")
		return false
	}

	// Extract username (lf_sso) and meeting_and_occurrence_id from v1Data.
	// Only send the access control message if username is present — without it
	// the fga-sync service cannot identify which user to remove access for.
	username := utils.GetString(v1Data["lf_sso"])
	meetingAndOccurrenceID := utils.GetString(v1Data["meeting_and_occurrence_id"])

	var message []byte
	var deleteAllAccessSubject string

	if username != "" {
		accessMsg := map[string]interface{}{
			"meeting_and_occurrence_id": meetingAndOccurrenceID,
			"username":                  username,
			"is_attended":               true,
		}
		var err error
		if message, err = json.Marshal(accessMsg); err != nil {
			funcLogger.With(logging.ErrKey, err).ErrorContext(ctx, "failed to marshal attendee access message")
			return false
		}
		deleteAllAccessSubject = "lfx.remove_participant.v1_past_meeting"
	} else {
		funcLogger.DebugContext(ctx, "no username in v1Data, skipping access control message for attendee delete")
		message = []byte(attendeeID)
	}

	return h.handleMeetingTypeDelete(ctx, key, attendeeID, message, meetingDeleteConfig{
		indexerSubject:         "lfx.index.v1_past_meeting_participant",
		deleteAllAccessSubject: deleteAllAccessSubject,
		tombstoneKeyFmts:       []string{"v1_past_meeting_attendees.%s"},
	})
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
				isHost = utils.GetBool(registrantData["host"])
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
			logger.With(logging.ErrKey, err).WarnContext(ctx, "failed to lookup v1 user", "lf_user_id", rawInvitee.LFUserID)
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
			logger.With(logging.ErrKey, err).WarnContext(ctx, "failed to lookup v1 user", "lf_user_id", rawAttendee.LFUserID)
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

// =============================================================================
// Past Meeting Recording Event Handler
// =============================================================================

// handlePastMeetingRecordingUpdate processes updates to past meeting recordings
func (h *EventHandlers) handlePastMeetingRecordingUpdate(
	ctx context.Context,
	key string,
	v1Data map[string]interface{},
) (retry bool) {
	funcLogger := h.logger.With("key", key, "handler", "past_meeting_recording")
	funcLogger.DebugContext(ctx, "processing past meeting recording update")

	// Check if this is a soft delete
	if isDeleted, ok := v1Data["_sdc_deleted_at"].(string); ok && isDeleted != "" {
		return h.handlePastMeetingRecordingDelete(ctx, key, v1Data)
	}

	// Convert v1Data to recording event data
	recordingData, transcriptData, err := convertMapToRecordingData(ctx, v1Data, h.idMapper, funcLogger)
	if err != nil {
		funcLogger.With(logging.ErrKey, err).ErrorContext(ctx, "failed to convert v1Data to recording")
		return isTransientError(err)
	}

	// Validate required fields
	if recordingData.ID == "" || recordingData.MeetingAndOccurrenceID == "" {
		funcLogger.ErrorContext(ctx, "missing required fields in recording data")
		return false
	}
	funcLogger = funcLogger.With("recording_id", recordingData.ID)

	// Determine action (created vs updated)
	mappingKey := fmt.Sprintf("v1_past_meeting_recordings.%s", recordingData.ID)
	indexerAction := indexerConstants.ActionCreated
	if _, err := h.v1MappingsKV.Get(ctx, mappingKey); err == nil {
		indexerAction = indexerConstants.ActionUpdated
	}

	// Publish recording event to indexer and FGA-sync
	if err := h.publisher.PublishPastMeetingRecordingEvent(ctx, string(indexerAction), recordingData); err != nil {
		funcLogger.With(logging.ErrKey, err).ErrorContext(ctx, "failed to publish recording event")
		return isTransientError(err)
	}

	// If transcript is enabled, publish separate transcript event
	if transcriptData != nil {
		if err := h.publisher.PublishPastMeetingTranscriptEvent(ctx, string(indexerAction), transcriptData); err != nil {
			funcLogger.With(logging.ErrKey, err).ErrorContext(ctx, "failed to publish transcript event")
			return isTransientError(err)
		}
	}

	// Store mapping
	if _, err := h.v1MappingsKV.Put(ctx, mappingKey, []byte("1")); err != nil {
		funcLogger.With(logging.ErrKey, err).WarnContext(ctx, "failed to store recording mapping")
	}

	funcLogger.InfoContext(ctx, "successfully processed past meeting recording")
	return false // Success, ACK
}

// handlePastMeetingRecordingDelete processes recording deletions
func (h *EventHandlers) handlePastMeetingRecordingDelete(
	ctx context.Context,
	key string,
	_ map[string]interface{},
) (retry bool) {
	recordingID := extractIDFromKey(key, "itx-zoom-past-meetings-recordings.")
	mappingKey := fmt.Sprintf("v1_past_meeting_recordings.%s", recordingID)
	if h.isTombstoned(ctx, mappingKey) {
		h.logger.DebugContext(ctx, "recording delete already processed, skipping", "recording_id", recordingID)
		return false
	}
	// Delete recording from indexer first (no tombstone yet).
	if retry := h.handleMeetingTypeDelete(ctx, key, recordingID, []byte(recordingID), meetingDeleteConfig{
		indexerSubject:   "lfx.index.v1_past_meeting_recording",
		tombstoneKeyFmts: []string{},
	}); retry {
		return true
	}
	// Delete transcript from indexer and tombstone the shared mapping key.
	return h.handleMeetingTypeDelete(ctx, key, recordingID, []byte(recordingID), meetingDeleteConfig{
		indexerSubject:   "lfx.index.v1_past_meeting_transcript",
		tombstoneKeyFmts: []string{"v1_past_meeting_recordings.%s"},
	})
}

// =============================================================================
// Past Meeting Summary Event Handler
// =============================================================================

// handlePastMeetingSummaryUpdate processes updates to past meeting summaries
func (h *EventHandlers) handlePastMeetingSummaryUpdate(
	ctx context.Context,
	key string,
	v1Data map[string]interface{},
) (retry bool) {
	funcLogger := h.logger.With("key", key, "handler", "past_meeting_summary")
	funcLogger.DebugContext(ctx, "processing past meeting summary update")

	// Check if this is a soft delete
	if isDeleted, ok := v1Data["_sdc_deleted_at"].(string); ok && isDeleted != "" {
		return h.handlePastMeetingSummaryDelete(ctx, key, v1Data)
	}

	// Convert v1Data to summary event data
	summaryData, err := convertMapToSummaryData(ctx, v1Data, h.idMapper, funcLogger)
	if err != nil {
		funcLogger.With(logging.ErrKey, err).ErrorContext(ctx, "failed to convert v1Data to summary")
		return isTransientError(err)
	}

	// Validate required fields
	if summaryData.ID == "" || summaryData.MeetingAndOccurrenceID == "" {
		funcLogger.ErrorContext(ctx, "missing required fields in summary data")
		return false
	}
	funcLogger = funcLogger.With("summary_id", summaryData.ID)

	// Determine action (created vs updated)
	mappingKey := fmt.Sprintf("v1_past_meeting_summaries.%s", summaryData.ID)
	indexerAction := indexerConstants.ActionCreated
	if _, err := h.v1MappingsKV.Get(ctx, mappingKey); err == nil {
		indexerAction = indexerConstants.ActionUpdated
	}

	// Publish to indexer and FGA-sync
	if err := h.publisher.PublishPastMeetingSummaryEvent(ctx, string(indexerAction), summaryData); err != nil {
		funcLogger.With(logging.ErrKey, err).ErrorContext(ctx, "failed to publish summary event")
		return isTransientError(err)
	}

	// Store mapping
	if _, err := h.v1MappingsKV.Put(ctx, mappingKey, []byte("1")); err != nil {
		funcLogger.With(logging.ErrKey, err).WarnContext(ctx, "failed to store summary mapping")
	}

	funcLogger.InfoContext(ctx, "successfully processed past meeting summary")
	return false // Success, ACK
}

// handlePastMeetingSummaryDelete processes summary deletions
func (h *EventHandlers) handlePastMeetingSummaryDelete(
	ctx context.Context,
	key string,
	_ map[string]interface{},
) (retry bool) {
	summaryID := extractIDFromKey(key, "itx-zoom-past-meetings-summaries.")
	mappingKey := fmt.Sprintf("v1_past_meeting_summaries.%s", summaryID)
	if h.isTombstoned(ctx, mappingKey) {
		h.logger.DebugContext(ctx, "summary delete already processed, skipping", "summary_id", summaryID)
		return false
	}
	return h.handleMeetingTypeDelete(ctx, key, summaryID, []byte(summaryID), meetingDeleteConfig{
		indexerSubject:   "lfx.index.v1_past_meeting_summary",
		tombstoneKeyFmts: []string{"v1_past_meeting_summaries.%s"},
	})
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
