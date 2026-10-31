// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"fmt"
	"log/slog"

	indexerConstants "github.com/linuxfoundation/lfx-v2-indexer-service/pkg/constants"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/infrastructure/eventing"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/infrastructure/v1objects"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/utils"
)

// =============================================================================
// Past Meeting Recording Event Handler
// =============================================================================

// recordingDBRaw represents raw past meeting recording data from v1 DynamoDB/NATS KV bucket.
// The shape (with its file and session entries and their numeric coercions) is shared with
// the on-demand recording record reader in internal/infrastructure/nats and lives in
// internal/infrastructure/v1objects; the alias keeps this package's decoding path unchanged.
type recordingDBRaw = v1objects.Recording

// handlePastMeetingRecordingUpdate processes updates to past meeting recordings
func (h *EventHandlers) handlePastMeetingRecordingUpdate(
	ctx context.Context,
	key string,
	v1Data map[string]interface{},
) (retry bool) {
	ctx, finish, blocked := h.startPastMeetingChildAttempt(ctx, key)
	if blocked {
		return true
	}
	defer func() {
		retry = finish(retry)
	}()
	funcLogger := h.logger.With("key", key, "handler", "past_meeting_recording")
	funcLogger.DebugContext(ctx, "processing past meeting recording update")

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
	funcLogger.InfoContext(ctx, "processing past meeting recording update")

	// Fetch authoritative fields from the parent past meeting record: committees and access settings.
	// recording_access / transcript_access on the recording KV record are a snapshot set at
	// recording-creation time and are never updated when the host changes access via PUT
	// /itx/past_meetings. The parent record is the authoritative source for both the indexer
	// public flag and the body access fields, matching the FGA tuples written by
	// PublishPastMeetingEvent.
	parentKey := pastMeetingObjectKeyPrefix + recordingData.MeetingAndOccurrenceID
	parentMappingKey := "v1_past_meetings." + recordingData.MeetingAndOccurrenceID
	state, parentData, lookupErr := h.loadV1Parent(ctx, parentKey, parentMappingKey)
	if lookupErr != nil {
		funcLogger.With(logging.ErrKey, lookupErr).WarnContext(ctx, "transient error fetching parent past meeting, will retry")
		return true
	}
	if state == parentGone {
		funcLogger.InfoContext(ctx, "parent past meeting is deleted, retracting recording and transcript")
		return h.deletePastMeetingRecordingDocuments(ctx, key, recordingData.ID)
	}
	if state != parentLive {
		// A new parent's first put republishes the recording through
		// republishPastMeetingArtifacts, including on its initial create.
		funcLogger.InfoContext(ctx, "parent past meeting is absent or unusable, skipping recording")
		return false
	}
	parent := parentPastMeetingInfo{
		PrimaryCommitteeSFID: utils.GetString(parentData["committee"]),
		RecordingAccess:      utils.GetString(parentData["recording_access"]),
		TranscriptAccess:     utils.GetString(parentData["transcript_access"]),
	}
	committees, commErr := resolveParentPastMeetingCommittees(ctx, recordingData.MeetingAndOccurrenceID, parent.PrimaryCommitteeSFID, h.idMapper, h.v1MappingsKV, funcLogger)
	if commErr != nil {
		funcLogger.With(logging.ErrKey, commErr).WarnContext(ctx, "transient error resolving parent committees for recording, will retry")
		return true
	}
	recordingData.Committees = committees

	// Use parent access values (authoritative) for both the indexer public flag and body fields.
	recordingData.RecordingAccess = parent.RecordingAccess
	if recordingData.RecordingAccess == "" {
		recordingData.RecordingAccess = defaultArtifactAccess
	}
	// Only copy transcript_access from the parent when transcript files exist; the field must
	// be omitted (omitempty) when the recording has no transcript. The raw KV snapshot may
	// carry a stale non-empty value, so clear it explicitly when TranscriptEnabled is false.
	if recordingData.TranscriptEnabled {
		recordingData.TranscriptAccess = parent.TranscriptAccess
		if recordingData.TranscriptAccess == "" {
			recordingData.TranscriptAccess = defaultArtifactAccess
		}
	} else {
		recordingData.TranscriptAccess = ""
	}

	if transcriptData != nil {
		transcriptData.Committees = committees
		transcriptData.TranscriptAccess = recordingData.TranscriptAccess
	}

	// Determine action (created vs updated)
	mappingKey := fmt.Sprintf("v1_past_meeting_recordings.%s", recordingData.ID)
	indexerAction := indexerConstants.ActionCreated
	if _, err := h.v1MappingsKV.Get(ctx, mappingKey); err == nil {
		indexerAction = indexerConstants.ActionUpdated
	}

	// Publish recording event to the indexer (recordings send no FGA message)
	// Check the parent on every exit after publication starts, including a
	// failed transcript operation after a successful recording publication.
	// This runs before the outer ownership finalizer.
	defer func() {
		after, _, err := h.loadV1Parent(ctx, parentKey, parentMappingKey)
		if err != nil {
			retry = true
			return
		}
		if after == parentGone || after == parentAbsent {
			if h.deletePastMeetingRecordingDocuments(ctx, key, recordingData.ID) {
				retry = true
			}
		}
	}()
	if err := h.publisher.PublishPastMeetingRecordingEvent(ctx, string(indexerAction), recordingData); err != nil {
		funcLogger.With(logging.ErrKey, err).ErrorContext(ctx, "failed to publish recording event")
		return true
	}

	// If the record has TRANSCRIPT or TIMELINE files, publish a separate transcript event
	if transcriptData != nil {
		if err := h.publisher.PublishPastMeetingTranscriptEvent(ctx, string(indexerAction), transcriptData); err != nil {
			funcLogger.With(logging.ErrKey, err).ErrorContext(ctx, "failed to publish transcript event")
			return true
		}
	} else {
		// The record has no transcript files. Remove any transcript document an earlier publish
		// left behind: it would otherwise keep the public flag it was last indexed with, since
		// no transcript event will republish it. This does not depend on the mapping lookup
		// above, which reads "created" when an earlier mapping write failed; deleting a document
		// that was never indexed is harmless, as on the recording delete path.
		if err := h.publisher.PublishIndexerDelete(ctx, eventing.IndexV1PastMeetingTranscriptSubject, recordingData.ID); err != nil {
			funcLogger.With(logging.ErrKey, err).ErrorContext(ctx, "failed to delete stale transcript document")
			return true
		}
	}

	// Store mapping
	if _, err := h.v1MappingsKV.Put(ctx, mappingKey, []byte("1")); err != nil {
		funcLogger.With(logging.ErrKey, err).WarnContext(ctx, "failed to store recording mapping")
	}

	funcLogger.InfoContext(ctx, "successfully processed past meeting recording", "action", string(indexerAction))
	return false
}

func (h *EventHandlers) deletePastMeetingRecordingDocuments(ctx context.Context, key, id string) (retry bool) {
	if h.beginPastMeetingChildRepair(ctx, key, id) {
		return true
	}
	for _, subject := range []string{eventing.IndexV1PastMeetingRecordingSubject, eventing.IndexV1PastMeetingTranscriptSubject} {
		if err := h.publisher.PublishIndexerDelete(ctx, subject, id); err != nil {
			h.logger.With(logging.ErrKey, err).WarnContext(ctx, "failed to retract recording of deleted past meeting, will retry", "subject", subject)
			return true
		}
	}
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
	h.logger.InfoContext(ctx, "processing past meeting recording delete", "recording_id", recordingID)
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
	rawRecording, err := decodeV1[recordingDBRaw](v1Data)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decode recording data: %w", err)
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

	// Default recording access to most restrictive; overridden by parent lookup in the handler.
	recordingAccess := rawRecording.RecordingAccess
	if recordingAccess == "" {
		recordingAccess = defaultArtifactAccess
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
		transcriptAccess = defaultArtifactAccess
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
