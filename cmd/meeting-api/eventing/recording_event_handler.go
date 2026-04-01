// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"

	indexerConstants "github.com/linuxfoundation/lfx-v2-indexer-service/pkg/constants"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
)

// =============================================================================
// Past Meeting Recording Event Handler
// =============================================================================

// RecordingDBRaw represents raw past meeting recording data from v1 DynamoDB/NATS KV bucket
type RecordingDBRaw struct {
	// MeetingAndOccurrenceID is the primary key of the recording table since there is only one recording record for a past meeting.
	MeetingAndOccurrenceID string `json:"meeting_and_occurrence_id"`

	// ProjectID is the ID of the project associated with the recording.
	ProjectID string `json:"proj_id"`

	// ProjectSlug is the slug of the project associated with the recording.
	ProjectSlug string `json:"project_slug"`

	// HostEmail is the email of the host of the recorded meeting. This comes from Zoom.
	HostEmail string `json:"host_email"`

	// HostID is the Zoom user ID of the host of the recorded meeting. This comes from Zoom.
	HostID string `json:"host_id"`

	// MeetingID is the ID of the meeting associated with the recording.
	MeetingID string `json:"meeting_id"`

	// OccurrenceID is the ID of the occurrence associated with the recording.
	OccurrenceID string `json:"occurrence_id"`

	// RecordingAccess is the access type of the recording.
	RecordingAccess string `json:"recording_access"`

	// Topic is the topic of the recorded meeting.
	Topic string `json:"topic"`

	// TranscriptAccess is the access type of the transcript of the recording.
	TranscriptAccess string `json:"transcript_access"`

	// TranscriptEnabled is whether the transcript of the recording is enabled.
	TranscriptEnabled bool `json:"transcript_enabled"`

	// Visibility is the visibility of the recording on the LFX platform.
	Visibility string `json:"visibility"`

	// RecordingCount is the number of recording files in the recording.
	// A recording record can have many files due to there being multiple sessions of the same meeting,
	// and the fact that each session has an MP4 file, M4A file, and optionally a VTT and JSON file
	// if there is a transcript available.
	RecordingCount int `json:"recording_count"`

	// RecordingFiles is the list of files in the recording.
	RecordingFiles []RecordingFileDBRaw `json:"recording_files"`

	// Sessions is the list of sessions in the recording.
	// There can be multiple sessions in a recording due to the fact that a meeting can be restarted
	// and that is considered a new session in Zoom.
	Sessions []RecordingSessionDBRaw `json:"sessions"`

	// StartTime is the start time of the recording in RFC3339 format.
	StartTime string `json:"start_time"`

	// TotalSize is the total size of the recording in bytes.
	TotalSize int64 `json:"total_size"`

	// CreatedAt is the creation time of the recording in RFC3339 format.
	CreatedAt string `json:"created_at"`

	// ModifiedAt is the last modification time of the recording in RFC3339 format.
	ModifiedAt string `json:"modified_at"`

	// CreatedBy is the user who created the recording record in this system.
	CreatedBy models.CreatedBy `json:"created_by"`

	// UpdatedBy is the user who last updated the recording record in this system.
	UpdatedBy models.UpdatedBy `json:"updated_by"`
}

// UnmarshalJSON implements custom unmarshaling to handle both string and number inputs for numeric fields.
func (r *RecordingDBRaw) UnmarshalJSON(data []byte) error {
	type Alias RecordingDBRaw
	tmp := struct {
		RecordingCount interface{} `json:"recording_count"`
		TotalSize      interface{} `json:"total_size"`
		*Alias
	}{
		Alias: (*Alias)(r),
	}

	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}

	switch v := tmp.RecordingCount.(type) {
	case string:
		if v != "" {
			val, err := strconv.Atoi(v)
			if err != nil {
				return fmt.Errorf("invalid value for recording_count: %w", err)
			}
			r.RecordingCount = val
		}
	case float64:
		r.RecordingCount = int(v)
	case nil:
		// leave as zero value
	default:
		return fmt.Errorf("invalid type for recording_count: %T", v)
	}

	switch v := tmp.TotalSize.(type) {
	case string:
		if v != "" {
			val, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				return fmt.Errorf("invalid value for total_size: %w", err)
			}
			r.TotalSize = val
		}
	case float64:
		r.TotalSize = int64(v)
	case nil:
		// leave as zero value
	default:
		return fmt.Errorf("invalid type for total_size: %T", v)
	}

	return nil
}

// RecordingFileDBRaw represents raw recording file data from v1 DynamoDB/NATS KV bucket
type RecordingFileDBRaw struct {
	DownloadURL    string `json:"download_url,omitempty"`
	FileExtension  string `json:"file_extension"`
	FileSize       int64  `json:"file_size"`
	FileType       string `json:"file_type"`
	ID             string `json:"id"`
	MeetingID      string `json:"meeting_id"`
	PlayURL        string `json:"play_url,omitempty"`
	RecordingStart string `json:"recording_start"`
	RecordingEnd   string `json:"recording_end"`
	RecordingType  string `json:"recording_type"`
	Status         string `json:"status"`
}

// UnmarshalJSON implements custom unmarshaling to handle both string and number inputs for numeric fields.
func (r *RecordingFileDBRaw) UnmarshalJSON(data []byte) error {
	type Alias RecordingFileDBRaw
	tmp := struct {
		FileSize interface{} `json:"file_size"`
		*Alias
	}{
		Alias: (*Alias)(r),
	}

	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}

	switch v := tmp.FileSize.(type) {
	case string:
		if v != "" {
			val, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				return fmt.Errorf("invalid value for file_size: %w", err)
			}
			r.FileSize = val
		}
	case float64:
		r.FileSize = int64(v)
	case nil:
		// leave as zero value
	default:
		return fmt.Errorf("invalid type for file_size: %T", v)
	}

	return nil
}

// RecordingSessionDBRaw represents raw recording session data from v1 DynamoDB/NATS KV bucket
type RecordingSessionDBRaw struct {
	UUID      string `json:"uuid"`
	ShareURL  string `json:"share_url"`
	TotalSize int64  `json:"total_size"`
	StartTime string `json:"start_time"`
	Password  string `json:"password"`
}

// UnmarshalJSON implements custom unmarshaling to handle both string and number inputs for numeric fields.
func (r *RecordingSessionDBRaw) UnmarshalJSON(data []byte) error {
	type Alias RecordingSessionDBRaw
	tmp := struct {
		TotalSize interface{} `json:"total_size"`
		*Alias
	}{
		Alias: (*Alias)(r),
	}

	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}

	switch v := tmp.TotalSize.(type) {
	case string:
		if v != "" {
			val, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				return fmt.Errorf("invalid value for total_size: %w", err)
			}
			r.TotalSize = val
		}
	case float64:
		r.TotalSize = int64(v)
	case nil:
		// leave as zero value
	default:
		return fmt.Errorf("invalid type for total_size: %T", v)
	}

	return nil
}

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
	if recordingData.ProjectUID == "" {
		funcLogger.InfoContext(ctx, "skipping recording sync - parent project not found in mappings")
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
	if rawRecording.MeetingAndOccurrenceID == "" {
		return nil, nil, fmt.Errorf("missing required fields: meeting_and_occurrence_id")
	}

	// Use ProjectID from recording record directly
	projectSFID := rawRecording.ProjectID
	if projectSFID == "" {
		return nil, nil, fmt.Errorf("recording missing project ID")
	}

	// Map project ID. A missing mapping means the project isn't in v2 yet — the caller skips.
	// Any other error is transient and propagated for retry.
	projectUID, err := idMapper.MapProjectV1ToV2(ctx, projectSFID)
	if err != nil && domain.GetErrorType(err) != domain.ErrorTypeValidation {
		return nil, nil, fmt.Errorf("failed to map project ID (transient): %w", err)
	}

	// Default recording access to meeting_hosts (most restrictive)
	recordingAccess := rawRecording.RecordingAccess
	if recordingAccess == "" {
		recordingAccess = "meeting_hosts"
	}

	// Split recording files into recording-only and transcript-only lists.
	// Transcript file types from Zoom: TRANSCRIPT (VTT), TIMELINE (JSON timeline).
	// All other file types (MP4, M4A, CC, etc.) belong to the recording.
	var recordingFiles []models.RecordingFile
	var transcriptFiles []models.RecordingFile
	for _, rawFile := range rawRecording.RecordingFiles {
		recordingStart, _ := parseTime(rawFile.RecordingStart)
		recordingEnd, _ := parseTime(rawFile.RecordingEnd)

		file := models.RecordingFile{
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
		}

		if rawFile.FileType == "TRANSCRIPT" || rawFile.FileType == "TIMELINE" {
			transcriptFiles = append(transcriptFiles, file)
		} else {
			recordingFiles = append(recordingFiles, file)
		}
	}
	hasTranscript := len(transcriptFiles) > 0

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
		ID:                     rawRecording.MeetingAndOccurrenceID,
		MeetingAndOccurrenceID: rawRecording.MeetingAndOccurrenceID,
		ProjectUID:             projectUID,
		HostEmail:              rawRecording.HostEmail,
		HostID:                 rawRecording.HostID,
		MeetingID:              rawRecording.MeetingID,
		OccurrenceID:           rawRecording.OccurrenceID,
		Platform:               "Zoom",
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
		ProjectSlug:            rawRecording.ProjectSlug,
		CreatedAt:              createdAt,
		UpdatedAt:              updatedAt,
		CreatedBy:              rawRecording.CreatedBy,
		UpdatedBy:              rawRecording.UpdatedBy,
	}

	// Create transcript event data if transcript files are present
	var transcriptData *models.TranscriptEventData
	if hasTranscript {
		transcriptData = &models.TranscriptEventData{
			ID:                     rawRecording.MeetingAndOccurrenceID,
			MeetingAndOccurrenceID: rawRecording.MeetingAndOccurrenceID,
			ProjectUID:             projectUID,
			ProjectSlug:            rawRecording.ProjectSlug,
			HostEmail:              rawRecording.HostEmail,
			HostID:                 rawRecording.HostID,
			MeetingID:              rawRecording.MeetingID,
			OccurrenceID:           rawRecording.OccurrenceID,
			Platform:               "Zoom",
			TranscriptAccess:       transcriptAccess,
			Title:                  rawRecording.Topic,
			Visibility:             rawRecording.Visibility,
			RecordingFiles:         transcriptFiles,
			Sessions:               sessions,
			StartTime:              startTime,
			TotalSize:              rawRecording.TotalSize,
			CreatedAt:              createdAt,
			UpdatedAt:              updatedAt,
			CreatedBy:              rawRecording.CreatedBy,
			UpdatedBy:              rawRecording.UpdatedBy,
		}
	}

	return recordingData, transcriptData, nil
}
