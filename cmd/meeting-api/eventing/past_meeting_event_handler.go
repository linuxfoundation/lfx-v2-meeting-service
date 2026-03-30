// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

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
