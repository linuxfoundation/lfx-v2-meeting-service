// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

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
func handlePastMeetingUpdate(
	ctx context.Context,
	key string,
	v1Data map[string]interface{},
	publisher domain.EventPublisher,
	userLookup domain.V1UserLookup,
	idMapper domain.IDMapper,
	v1ObjectsKV jetstream.KeyValue,
	mappingsKV jetstream.KeyValue,
	logger *slog.Logger,
) bool {
	funcLogger := logger.With("key", key, "handler", "past_meeting")
	funcLogger.DebugContext(ctx, "processing past meeting update")

	// Check if this is a soft delete
	if isDeleted, ok := v1Data["_sdc_deleted_at"].(string); ok && isDeleted != "" {
		return handlePastMeetingDelete(ctx, key, v1Data, publisher, mappingsKV, funcLogger)
	}

	// Convert v1Data to past meeting event data
	pastMeetingData, err := convertMapToPastMeetingData(ctx, v1Data, idMapper, v1ObjectsKV, mappingsKV, funcLogger)
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
	if _, err := mappingsKV.Get(ctx, mappingKey); err == nil {
		indexerAction = indexerConstants.ActionUpdated
	}

	// Publish to indexer and FGA-sync
	if err := publisher.PublishPastMeetingEvent(ctx, string(indexerAction), pastMeetingData); err != nil {
		funcLogger.With(errKey, err).ErrorContext(ctx, "failed to publish past meeting event")
		if isTransientError(err) {
			return true // NAK for retry
		}
		return false // Permanent error, ACK and skip
	}

	// Store mapping
	if _, err := mappingsKV.Put(ctx, mappingKey, []byte("1")); err != nil {
		funcLogger.With(errKey, err).WarnContext(ctx, "failed to store past meeting mapping")
	}

	funcLogger.InfoContext(ctx, "successfully processed past meeting")
	return false // Success, ACK
}

// handlePastMeetingDelete processes past meeting deletions
func handlePastMeetingDelete(
	ctx context.Context,
	key string,
	v1Data map[string]interface{},
	publisher domain.EventPublisher,
	mappingsKV jetstream.KeyValue,
	logger *slog.Logger,
) bool {
	pastMeetingID := extractIDFromKey(key, "itx-zoom-past-meetings.")
	funcLogger := logger.With("past_meeting_id", pastMeetingID, "handler", "past_meeting_delete")
	funcLogger.InfoContext(ctx, "processing past meeting deletion")

	// Create minimal event data for deletion
	eventData := &models.PastMeetingEventData{ID: pastMeetingID}

	// Publish delete event
	if err := publisher.PublishPastMeetingEvent(ctx, string(indexerConstants.ActionDeleted), eventData); err != nil {
		funcLogger.With(errKey, err).ErrorContext(ctx, "failed to publish past meeting delete event")
		if isTransientError(err) {
			return true // NAK for retry
		}
		return false // Permanent error, ACK and skip
	}

	// Remove mapping
	mappingKey := fmt.Sprintf("v1_past_meetings.%s", pastMeetingID)
	_ = mappingsKV.Delete(ctx, mappingKey)

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
		Tags:             generatePastMeetingTags(&rawPastMeeting, projectUID),
	}, nil
}

// =============================================================================
// Past Meeting Mapping Event Handler
// =============================================================================

// handlePastMeetingMappingUpdate processes updates to past meeting-committee mappings
func handlePastMeetingMappingUpdate(
	ctx context.Context,
	key string,
	v1Data map[string]interface{},
	publisher domain.EventPublisher,
	userLookup domain.V1UserLookup,
	idMapper domain.IDMapper,
	v1ObjectsKV jetstream.KeyValue,
	mappingsKV jetstream.KeyValue,
	logger *slog.Logger,
) bool {
	funcLogger := logger.With("key", key, "handler", "past_meeting_mapping")
	funcLogger.InfoContext(ctx, "processing past meeting mapping update")

	// Check if this is a soft delete
	if isDeleted, ok := v1Data["_sdc_deleted_at"].(string); ok && isDeleted != "" {
		return handlePastMeetingMappingDelete(ctx, key, v1Data, publisher, userLookup, idMapper, v1ObjectsKV, mappingsKV, funcLogger)
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
	if err := updatePastMeetingCommitteeMappings(ctx, pastMeetingUUID, mappingID, committeeID, v1Data, mappingsKV, funcLogger); err != nil {
		funcLogger.With(errKey, err).ErrorContext(ctx, "failed to update past meeting committee mappings")
		if isTransientError(err) {
			return true // NAK for retry
		}
		return false // ACK - permanent error
	}

	// Re-trigger past meeting indexing
	shouldRetry := retriggerPastMeetingIndexing(ctx, pastMeetingUUID, publisher, userLookup, idMapper, v1ObjectsKV, mappingsKV, funcLogger)
	if shouldRetry {
		return true // NAK for retry
	}

	funcLogger.InfoContext(ctx, "successfully processed past meeting mapping update")
	return false // ACK
}

// handlePastMeetingMappingDelete processes past meeting-committee mapping deletions
func handlePastMeetingMappingDelete(
	ctx context.Context,
	key string,
	v1Data map[string]interface{},
	publisher domain.EventPublisher,
	userLookup domain.V1UserLookup,
	idMapper domain.IDMapper,
	v1ObjectsKV jetstream.KeyValue,
	mappingsKV jetstream.KeyValue,
	logger *slog.Logger,
) bool {
	pastMeetingUUID := getString(v1Data["past_meeting_uuid"])
	mappingID := getString(v1Data["id"])

	if pastMeetingUUID == "" || mappingID == "" {
		logger.WarnContext(ctx, "missing required fields in past meeting mapping deletion")
		return false // ACK - permanent error
	}

	funcLogger := logger.With("past_meeting_uuid", pastMeetingUUID, "mapping_id", mappingID, "handler", "past_meeting_mapping_delete")
	funcLogger.InfoContext(ctx, "processing past meeting mapping deletion")

	// Remove the mapping
	if err := removePastMeetingCommitteeMapping(ctx, pastMeetingUUID, mappingID, mappingsKV, funcLogger); err != nil {
		if isTransientError(err) {
			return true // NAK for retry
		}
	}

	// Re-trigger past meeting indexing
	shouldRetry := retriggerPastMeetingIndexing(ctx, pastMeetingUUID, publisher, userLookup, idMapper, v1ObjectsKV, mappingsKV, funcLogger)
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

type PastMeetingDBRaw struct {
	UUID              string      `json:"uuid"`
	MeetingID         string      `json:"meeting_id"`
	ProjectID         string      `json:"proj_id"`
	Topic             string      `json:"topic"`
	Agenda            string      `json:"agenda"`
	StartTime         string      `json:"start_time"`
	EndTime           string      `json:"end_time"`
	Duration          interface{} `json:"duration"`
	Timezone          string      `json:"timezone"`
	ParticipantsCount interface{} `json:"participants_count"`
	HostID            string      `json:"host_id"`
	CreatedAt         string      `json:"created_at"`
	ModifiedAt        string      `json:"modified_at"`
}

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

func retriggerPastMeetingIndexing(
	ctx context.Context,
	pastMeetingUUID string,
	publisher domain.EventPublisher,
	userLookup domain.V1UserLookup,
	idMapper domain.IDMapper,
	v1ObjectsKV jetstream.KeyValue,
	mappingsKV jetstream.KeyValue,
	logger *slog.Logger,
) bool {
	pastMeetingKey := fmt.Sprintf("itx-zoom-past-meetings.%s", pastMeetingUUID)
	pastMeetingEntry, err := v1ObjectsKV.Get(ctx, pastMeetingKey)
	if err != nil {
		logger.With(errKey, err).WarnContext(ctx, "past meeting not found during retrigger")
		return false // Past meeting might be deleted, ACK
	}

	var pastMeetingData map[string]interface{}
	if err := json.Unmarshal(pastMeetingEntry.Value(), &pastMeetingData); err != nil {
		logger.With(errKey, err).ErrorContext(ctx, "failed to unmarshal past meeting data")
		return false // ACK - permanent error
	}

	// Re-process the past meeting
	return handlePastMeetingUpdate(ctx, pastMeetingKey, pastMeetingData, publisher, userLookup, idMapper, v1ObjectsKV, mappingsKV, logger)
}

func generatePastMeetingTags(pm *PastMeetingDBRaw, projectUID string) []string {
	tags := []string{
		"past_meeting_id:" + pm.UUID,
		"meeting_id:" + pm.MeetingID,
		"project_uid:" + projectUID,
		"title:" + pm.Topic,
	}
	if pm.Timezone != "" {
		tags = append(tags, "timezone:"+pm.Timezone)
	}
	return tags
}
