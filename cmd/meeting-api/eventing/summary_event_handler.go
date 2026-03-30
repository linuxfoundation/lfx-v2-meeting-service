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

// SummaryDBRaw represents raw past meeting summary data from v1 DynamoDB/NATS KV bucket
type SummaryDBRaw struct {
	// ID is the partition key of the summary record (it is a UUID).
	ID string `json:"id"`

	// MeetingAndOccurrenceID is the ID of the past meeting associated with the summary.
	MeetingAndOccurrenceID string `json:"meeting_and_occurrence_id"`

	// MeetingID is the ID of the meeting associated with the summary.
	MeetingID string `json:"meeting_id"`

	// OccurrenceID is the ID of the occurrence associated with the summary.
	OccurrenceID string `json:"occurrence_id"`

	// ZoomMeetingUUID is the UUID of the meeting associated with the summary.
	ZoomMeetingUUID string `json:"zoom_meeting_uuid"`

	// ZoomMeetingHostID is the ID of the host of the meeting associated with the summary.
	ZoomMeetingHostID string `json:"zoom_meeting_host_id"`

	// ZoomMeetingHostEmail is the email of the host of the meeting associated with the summary.
	ZoomMeetingHostEmail string `json:"zoom_meeting_host_email"`

	// ZoomMeetingTopic is the topic of the meeting associated with the summary.
	ZoomMeetingTopic string `json:"zoom_meeting_topic"`

	// ZoomWebhookEvent is the original webhook event that triggered the summary.
	ZoomWebhookEvent string `json:"zoom_webhook_event"`

	// Password is an ITX UUID-generated password for the summary that is used to access the summary.
	Password string `json:"password"`

	// SummaryCreatedTime is the creation time of the summary in RFC3339 format.
	SummaryCreatedTime string `json:"summary_created_time"`

	// SummaryLastModifiedTime is the last modification time of the summary in RFC3339 format.
	SummaryLastModifiedTime string `json:"summary_last_modified_time"`

	// SummaryStartTime is the start time of the summary in RFC3339 format.
	SummaryStartTime string `json:"summary_start_time"`

	// SummaryEndTime is the end time of the summary in RFC3339 format.
	SummaryEndTime string `json:"summary_end_time"`

	// SummaryTitle is the title of the summary.
	SummaryTitle string `json:"summary_title"`

	// SummaryOverview is the overview of the summary.
	SummaryOverview string `json:"summary_overview"`

	// SummaryDetails is the details of the summary.
	SummaryDetails []SummaryDetailDBRaw `json:"summary_details"`

	// NextSteps is the next steps of the summary.
	NextSteps []string `json:"next_steps"`

	// EditedSummaryOverview is the edited overview of the summary.
	EditedSummaryOverview string `json:"edited_summary_overview"`

	// EditedSummaryDetails is the edited details of the summary.
	EditedSummaryDetails []SummaryDetailDBRaw `json:"edited_summary_details"`

	// EditedNextSteps is the edited next steps of the summary.
	EditedNextSteps []string `json:"edited_next_steps"`

	// Content is the original content of the summary.
	// This is a v2 only attribute.
	Content string `json:"content"`

	// EditedContent is the edited content of the summary.
	// This is a v2 only attribute.
	EditedContent string `json:"edited_content"`

	// RequiresApproval is whether the summary requires approval.
	RequiresApproval bool `json:"requires_approval"`

	// Approved is whether the summary has been approved.
	Approved bool `json:"approved"`

	// Platform is the platform of the summary.
	// This is a v2 only attribute, whose value is always "Zoom".
	Platform string `json:"platform"`

	// ZoomConfig contains Zoom-specific summary configuration and metadata.
	// This is a v2 only attribute.
	ZoomConfig models.SummaryZoomConfig `json:"zoom_config"`

	// EmailSent is whether an email was sent to users about the summary.
	// An email is only sent to users who have updated the meeting, and it is only for summaries
	// that are the longest summary for a given past meeting - because we don't want to spam users
	// with emails about small summaries that aren't the main summary of the meeting.
	EmailSent bool `json:"email_sent"`

	// CreatedAt is the creation time of the summary in RFC3339 format.
	CreatedAt string `json:"created_at"`

	// CreatedBy is the user who created the summary.
	CreatedBy models.CreatedBy `json:"created_by"`

	// UpdatedAt is the last modification time of the summary in RFC3339 format.
	// This is a v2 only attribute.
	UpdatedAt string `json:"updated_at"`

	// ModifiedBy is the user who last modified the summary.
	ModifiedBy models.UpdatedBy `json:"modified_by"`
}

// SummaryDetailDBRaw represents raw summary detail data from v1 DynamoDB/NATS KV bucket
type SummaryDetailDBRaw struct {
	Label   string `json:"label"`
	Summary string `json:"summary"`
}

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

	// Consolidate original summary fields into markdown content
	content := buildSummaryMarkdown(rawSummary.SummaryOverview, rawSummary.SummaryDetails, rawSummary.NextSteps)

	// Consolidate edited summary fields into markdown edited content
	editedContent := buildSummaryMarkdown(rawSummary.EditedSummaryOverview, rawSummary.EditedSummaryDetails, rawSummary.EditedNextSteps)

	// Parse times
	createdAt, _ := parseTime(rawSummary.CreatedAt)
	updatedAt, _ := parseTime(rawSummary.UpdatedAt)

	return &models.SummaryEventData{
		ID:                      rawSummary.ID,
		MeetingAndOccurrenceID:  rawSummary.MeetingAndOccurrenceID,
		MeetingID:               rawSummary.MeetingID,
		OccurrenceID:            rawSummary.OccurrenceID,
		ZoomMeetingUUID:         rawSummary.ZoomMeetingUUID,
		ZoomMeetingHostID:       rawSummary.ZoomMeetingHostID,
		ZoomMeetingHostEmail:    rawSummary.ZoomMeetingHostEmail,
		ZoomMeetingTopic:        rawSummary.ZoomMeetingTopic,
		ZoomWebhookEvent:        rawSummary.ZoomWebhookEvent,
		SummaryTitle:            rawSummary.SummaryTitle,
		SummaryStartTime:        rawSummary.SummaryStartTime,
		SummaryEndTime:          rawSummary.SummaryEndTime,
		SummaryCreatedTime:      rawSummary.SummaryCreatedTime,
		SummaryLastModifiedTime: rawSummary.SummaryLastModifiedTime,
		Content:                 content,
		EditedContent:           editedContent,
		RequiresApproval:        rawSummary.RequiresApproval,
		Approved:                rawSummary.Approved,
		Platform:                "Zoom",
		ZoomConfig: models.SummaryZoomConfig{
			MeetingID:   rawSummary.MeetingID,
			MeetingUUID: rawSummary.ZoomMeetingUUID,
		},
		EmailSent: rawSummary.EmailSent,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
		CreatedBy: models.CreatedBy(rawSummary.CreatedBy),
		UpdatedBy: models.UpdatedBy(rawSummary.ModifiedBy),
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
