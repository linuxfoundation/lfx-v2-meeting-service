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
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
)

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
	return false
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
		Title:                  rawRecording.Topic,
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
