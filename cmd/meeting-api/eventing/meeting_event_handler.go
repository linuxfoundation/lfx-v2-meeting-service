// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	indexerConstants "github.com/linuxfoundation/lfx-v2-indexer-service/pkg/constants"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
	itx "github.com/linuxfoundation/lfx-v2-meeting-service/pkg/models/itx"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/utils"
	"github.com/nats-io/nats.go/jetstream"
)

// =============================================================================
// Meeting Event Handler
// =============================================================================

// convertMapToMeetingData converts v1 meeting data to v2 format
func convertMapToMeetingData(
	ctx context.Context,
	v1Data map[string]interface{},
	idMapper domain.IDMapper,
	mappingsKV jetstream.KeyValue,
	logger *slog.Logger,
) (*models.MeetingEventData, error) {
	// Convert map to JSON bytes, then to MeetingDBRaw
	jsonBytes, err := json.Marshal(v1Data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal v1Data to JSON: %w", err)
	}

	var rawMeeting MeetingDBRaw
	if err := json.Unmarshal(jsonBytes, &rawMeeting); err != nil {
		return nil, fmt.Errorf("failed to unmarshal meeting data: %w", err)
	}

	meeting := &models.MeetingEventData{
		ID:                   rawMeeting.MeetingID,
		Title:                rawMeeting.Topic,
		Description:          rawMeeting.Agenda,
		StartTime:            rawMeeting.StartTime,
		Duration:             rawMeeting.Duration,
		Timezone:             rawMeeting.Timezone,
		Visibility:           rawMeeting.Visibility,
		Restricted:           rawMeeting.Restricted,
		MeetingType:          rawMeeting.MeetingType,
		EarlyJoinTimeMinutes: rawMeeting.EarlyJoinTime,
		RecordingEnabled:     rawMeeting.RecordingEnabled,
		TranscriptEnabled:    rawMeeting.TranscriptEnabled,
		YoutubeUploadEnabled: rawMeeting.YoutubeUploadEnabled,
		HostKey:              rawMeeting.HostKey,
		CreatedAt:            rawMeeting.CreatedAt,
		UpdatedAt:            rawMeeting.UpdatedAt,
		CreatedBy:            rawMeeting.CreatedBy,
		UpdatedBy:            rawMeeting.UpdatedBy,
	}

	// Skip if created by this service (prevent sync loops)
	if shouldSkipSync(rawMeeting.UpdatedBy.UserID) {
		logger.InfoContext(ctx, "skipping sync - created by this service", "last_modified_by", rawMeeting.UpdatedBy.UserID)
		return nil, nil
	}

	// Validate required fields
	if rawMeeting.MeetingID == "" || rawMeeting.ProjID == "" {
		return nil, fmt.Errorf("missing required fields: meeting_id or proj_id")
	}

	// Map project ID from v1 SFID to v2 UID
	projectUID, err := idMapper.MapProjectV1ToV2(ctx, rawMeeting.ProjID)
	if err != nil {
		return nil, fmt.Errorf("failed to map project ID (transient): %w", err)
	}
	meeting.ProjectUID = projectUID

	committees := getCommitteesForMeeting(ctx, rawMeeting.MeetingID, idMapper, mappingsKV, logger)
	meeting.Committees = committees

	// Map the primary committee v1 ID to v2 UID
	if rawMeeting.Committee != "" {
		committeeUID, err := idMapper.MapCommitteeV1ToV2(ctx, rawMeeting.Committee)
		if err != nil {
			logger.With(logging.ErrKey, err).WarnContext(ctx, "failed to map primary committee ID", "v1_id", rawMeeting.Committee)
		} else {
			meeting.CommitteeUID = committeeUID
		}
	}

	// Determine artifact visibility (priority: recording > transcript > ai_summary)
	meeting.ArtifactVisibility = rawMeeting.GetArtifactVisibility()

	// Calculate occurrences if recurring
	calc := NewOccurrenceCalculator(logger)

	// Calculate 100 future occurrences (not including past ones)
	occurrences, err := calc.CalculateOccurrences(
		ctx,
		*meeting,
		false, // pastOccurrences - don't include past
		false, // includeCancelled - don't include cancelled
		100,   // numOccurrencesToReturn
	)
	if err != nil {
		logger.With(logging.ErrKey, err).WarnContext(ctx, "failed to calculate occurrences")
	}
	meeting.Occurrences = make([]models.ZoomMeetingOccurrence, len(occurrences))
	for i, occurrence := range occurrences {
		meeting.Occurrences[i] = models.ZoomMeetingOccurrence{
			OccurrenceID: occurrence.OccurrenceID,
			StartTime:    occurrence.StartTime.Format(time.RFC3339),
			Duration:     occurrence.Duration,
			IsCancelled:  occurrence.IsCancelled,
			Title:        occurrence.Title,
			Description:  occurrence.Description,
			Recurrence:   occurrence.Recurrence,
			// TODO: do we need to determine the response counts before we index the data
			ResponseCountYes: 0,
			ResponseCountNo:  0,
			RegistrantCount:  0,
		}
	}

	return meeting, nil
}

// handleMeetingUpdate processes updates to meeting records
func (h *EventHandlers) handleMeetingUpdate(
	ctx context.Context,
	key string,
	v1Data map[string]interface{},
) (retry bool) {
	funcLogger := h.logger.With("key", key, "handler", "meeting")
	funcLogger.DebugContext(ctx, "processing meeting update")

	// Check if this is a soft delete
	if isDeleted, ok := v1Data["_sdc_deleted_at"].(string); ok && isDeleted != "" {
		return h.handleMeetingDelete(ctx, key, v1Data)
	}

	// Convert v1Data to meeting event data
	meetingData, err := convertMapToMeetingData(ctx, v1Data, h.idMapper, h.v1MappingsKV, funcLogger)
	if err != nil {
		funcLogger.With(logging.ErrKey, err).ErrorContext(ctx, "failed to convert v1Data to meeting")
		return isTransientError(err)
	}
	if meetingData == nil {
		return false // Intentionally skipped (e.g., loop prevention)
	}

	// Validate required fields
	if meetingData.ID == "" || meetingData.ProjectUID == "" {
		funcLogger.ErrorContext(ctx, "missing required fields in meeting data")
		return false
	}
	funcLogger = funcLogger.With("meeting_id", meetingData.ID)

	// Determine action (created vs updated)
	mappingKey := fmt.Sprintf("v1_meetings.%s", meetingData.ID)
	indexerAction := indexerConstants.ActionCreated
	if _, err := h.v1MappingsKV.Get(ctx, mappingKey); err == nil {
		indexerAction = indexerConstants.ActionUpdated
	}

	// Publish to indexer and FGA-sync
	if err := h.publisher.PublishMeetingEvent(ctx, string(indexerAction), meetingData); err != nil {
		funcLogger.With(logging.ErrKey, err).ErrorContext(ctx, "failed to publish meeting event")
		return isTransientError(err)
	}

	// Store mapping
	if _, err := h.v1MappingsKV.Put(ctx, mappingKey, []byte("1")); err != nil {
		funcLogger.With(logging.ErrKey, err).WarnContext(ctx, "failed to store meeting mapping")
	}

	funcLogger.InfoContext(ctx, "successfully processed meeting")
	return false
}

// handleMeetingDelete processes meeting deletions
func (h *EventHandlers) handleMeetingDelete(ctx context.Context, key string, _ map[string]interface{}) (retry bool) {
	meetingID := extractIDFromKey(key, "itx-zoom-meetings-v2.")
	mappingKey := fmt.Sprintf("v1_meetings.%s", meetingID)
	if h.isTombstoned(ctx, mappingKey) {
		h.logger.DebugContext(ctx, "meeting delete already processed, skipping", "meeting_id", meetingID)
		return false
	}
	return h.handleMeetingTypeDelete(ctx, key, meetingID, []byte(meetingID), meetingDeleteConfig{
		indexerSubject:         "lfx.index.v1_meeting",
		deleteAllAccessSubject: "lfx.delete_all_access.v1_meeting",
		tombstoneKeyFmts:       []string{"v1_meetings.%s"},
	})
}

// =============================================================================
// Meeting Mapping Event Handler
// =============================================================================

// handleMeetingMappingUpdate processes updates to meeting-committee mappings
func (h *EventHandlers) handleMeetingMappingUpdate(
	ctx context.Context,
	key string,
	v1Data map[string]interface{},
) (retry bool) {
	funcLogger := h.logger.With("key", key, "handler", "meeting_mapping")
	funcLogger.InfoContext(ctx, "processing meeting mapping update")

	// Check if this is a soft delete
	if isDeleted, ok := v1Data["_sdc_deleted_at"].(string); ok && isDeleted != "" {
		return h.handleMeetingMappingDelete(ctx, key, v1Data)
	}

	// Extract meeting ID and mapping data
	meetingID := utils.GetString(v1Data["meeting_id"])
	mappingID := utils.GetString(v1Data["id"])
	committeeID := utils.GetString(v1Data["committee_id"])

	if meetingID == "" || mappingID == "" {
		funcLogger.WarnContext(ctx, "missing required fields in mapping")
		return false
	}

	// Update committee mappings in KV bucket
	if err := updateCommitteeMappings(ctx, meetingID, mappingID, committeeID, v1Data, h.v1MappingsKV, funcLogger); err != nil {
		funcLogger.With(logging.ErrKey, err).ErrorContext(ctx, "failed to update committee mappings")
		return isTransientError(err)
	}

	// Re-trigger meeting indexing
	shouldRetry := h.retriggerMeetingIndexing(ctx, meetingID)
	if shouldRetry {
		return true
	}

	funcLogger.InfoContext(ctx, "successfully processed meeting mapping update")
	return false
}

// handleMeetingMappingDelete processes meeting-committee mapping deletions
func (h *EventHandlers) handleMeetingMappingDelete(
	ctx context.Context,
	key string,
	v1Data map[string]interface{},
) (retry bool) {
	if v1Data == nil {
		h.logger.WarnContext(ctx, "no v1Data available for meeting mapping delete, skipping", "key", key)
		return false
	}

	meetingID := utils.GetString(v1Data["meeting_id"])
	mappingID := extractIDFromKey(key, "itx-zoom-meetings-mappings-v2.")

	if meetingID == "" || mappingID == "" {
		h.logger.WarnContext(ctx, "missing required fields in mapping deletion")
		return false
	}

	funcLogger := h.logger.With("meeting_id", meetingID, "mapping_id", mappingID, "handler", "meeting_mapping_delete")
	funcLogger.InfoContext(ctx, "processing meeting mapping deletion")

	if err := removeCommitteeMapping(ctx, meetingID, mappingID, h.v1MappingsKV, funcLogger); err != nil {
		if isTransientError(err) {
			return true
		}
		funcLogger.With(logging.ErrKey, err).ErrorContext(ctx, "failed to remove committee mapping")
		return false
	}

	shouldRetry := h.retriggerMeetingIndexing(ctx, meetingID)
	if shouldRetry {
		return true
	}

	funcLogger.InfoContext(ctx, "successfully processed meeting mapping deletion")
	return false
}

// =============================================================================
// Helper Functions
// =============================================================================

// Conversion and utility functions

func getCommitteesForMeeting(
	ctx context.Context,
	meetingID string,
	idMapper domain.IDMapper,
	mappingsKV jetstream.KeyValue,
	logger *slog.Logger,
) []models.Committee {
	key := fmt.Sprintf("meeting-mappings.%s", meetingID)
	entry, err := mappingsKV.Get(ctx, key)
	if err != nil {
		return nil
	}

	var mappings map[string]map[string]interface{}
	if err := json.Unmarshal(entry.Value(), &mappings); err != nil {
		logger.With(logging.ErrKey, err).WarnContext(ctx, "failed to unmarshal committee mappings")
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

func updateCommitteeMappings(
	ctx context.Context,
	meetingID string,
	mappingID string,
	committeeID string,
	v1Data map[string]interface{},
	mappingsKV jetstream.KeyValue,
	logger *slog.Logger,
) error {
	mappingsKey := fmt.Sprintf("meeting-mappings.%s", meetingID)
	var mappings map[string]map[string]interface{}

	entry, err := mappingsKV.Get(ctx, mappingsKey)
	if err != nil {
		mappings = make(map[string]map[string]interface{})
	} else {
		if err := json.Unmarshal(entry.Value(), &mappings); err != nil {
			logger.With(logging.ErrKey, err).ErrorContext(ctx, "failed to unmarshal existing mappings")
			return fmt.Errorf("failed to unmarshal existing mappings: %w", err)
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
		return fmt.Errorf("failed to marshal mappings: %w", err)
	}

	if _, err := mappingsKV.Put(ctx, mappingsKey, mappingsJSON); err != nil {
		return fmt.Errorf("failed to store mappings: %w", err)
	}

	return nil
}

func removeCommitteeMapping(
	ctx context.Context,
	meetingID string,
	mappingID string,
	mappingsKV jetstream.KeyValue,
	logger *slog.Logger,
) error {
	mappingsKey := fmt.Sprintf("meeting-mappings.%s", meetingID)
	entry, err := mappingsKV.Get(ctx, mappingsKey)
	if err != nil {
		return nil // No mappings found
	}

	var mappings map[string]map[string]interface{}
	if err := json.Unmarshal(entry.Value(), &mappings); err != nil {
		return fmt.Errorf("failed to unmarshal mappings: %w", err)
	}

	delete(mappings, mappingID)

	if len(mappings) == 0 {
		return mappingsKV.Delete(ctx, mappingsKey)
	}

	mappingsJSON, err := json.Marshal(mappings)
	if err != nil {
		return fmt.Errorf("failed to marshal updated mappings: %w", err)
	}

	_, err = mappingsKV.Put(ctx, mappingsKey, mappingsJSON)
	return err
}

func (h *EventHandlers) retriggerMeetingIndexing(
	ctx context.Context,
	meetingID string,
) (retry bool) {
	meetingKey := fmt.Sprintf("itx-zoom-meetings-v2.%s", meetingID)
	meetingEntry, err := h.v1ObjectsKV.Get(ctx, meetingKey)
	if err != nil {
		h.logger.With(logging.ErrKey, err).WarnContext(ctx, "meeting not found during retrigger")
		return false // Meeting might be deleted
	}

	var meetingData map[string]interface{}
	if err := json.Unmarshal(meetingEntry.Value(), &meetingData); err != nil {
		h.logger.With(logging.ErrKey, err).ErrorContext(ctx, "failed to unmarshal meeting data")
		return false
	}

	// Re-process the meeting
	return h.handleMeetingUpdate(ctx, meetingKey, meetingData)
}
