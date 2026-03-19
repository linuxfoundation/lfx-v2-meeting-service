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
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
)

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
	return false
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
