// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"

	indexerConstants "github.com/linuxfoundation/lfx-v2-indexer-service/pkg/constants"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/models/itx"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/summaryformat"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/utils"
	"github.com/nats-io/nats.go/jetstream"
)

// =============================================================================
// Past Meeting Summary Event Handler
// =============================================================================

// summaryDBRaw represents raw past meeting summary data from v1 DynamoDB/NATS KV bucket
type summaryDBRaw struct {
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
	SummaryDetails []itx.ZoomMeetingSummaryDetails `json:"summary_details"`

	// NextSteps is the next steps of the summary.
	NextSteps []string `json:"next_steps"`

	// EditedSummaryOverview is the edited overview of the summary.
	EditedSummaryOverview string `json:"edited_summary_overview"`

	// EditedSummaryDetails is the edited details of the summary.
	EditedSummaryDetails []itx.ZoomMeetingSummaryDetails `json:"edited_summary_details"`

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

	// ModifiedAt is the last modification time of the summary in RFC3339 format.
	ModifiedAt string `json:"modified_at"`

	// ModifiedBy is the user who last modified the summary.
	ModifiedBy models.UpdatedBy `json:"modified_by"`
}

// UnmarshalJSON implements custom unmarshaling for summaryDBRaw.
func (s *summaryDBRaw) UnmarshalJSON(data []byte) error {
	type Alias summaryDBRaw
	tmp := struct{ *Alias }{Alias: (*Alias)(s)}
	return json.Unmarshal(data, &tmp)
}

// handlePastMeetingSummaryUpdate processes updates to past meeting summaries
func (h *EventHandlers) handlePastMeetingSummaryUpdate(
	ctx context.Context,
	key string,
	v1Data map[string]interface{},
) (retry bool) {
	funcLogger := h.logger.With("key", key, "handler", "past_meeting_summary")
	funcLogger.DebugContext(ctx, "processing past meeting summary update")

	// Convert v1Data to summary event data
	summaryData, err := convertMapToSummaryData(ctx, v1Data, funcLogger)
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
	funcLogger.InfoContext(ctx, "processing past meeting summary update")

	// Record the summary under its parent past meeting before reading the parent's
	// ai_summary_access below, so that a parent access change processed from here on finds
	// this summary and republishes it (see republishPastMeetingArtifacts). A summary that
	// cannot be recorded is not published: its public flag could never be revoked.
	if err := recordPastMeetingSummary(ctx, h.v1MappingsKV, summaryData.MeetingAndOccurrenceID, summaryData.ID); err != nil {
		if errors.Is(err, jetstream.ErrInvalidKey) {
			funcLogger.With(logging.ErrKey, err).ErrorContext(ctx, "skipping summary: cannot record it under its past meeting")
			return false
		}
		funcLogger.With(logging.ErrKey, err).WarnContext(ctx, "failed to record summary under its past meeting, will retry")
		return true
	}

	// Determine action (created vs updated)
	mappingKey := fmt.Sprintf("v1_past_meeting_summaries.%s", summaryData.ID)
	indexerAction := indexerConstants.ActionCreated
	if _, err := h.v1MappingsKV.Get(ctx, mappingKey); err == nil {
		indexerAction = indexerConstants.ActionUpdated
	}

	// Look up project info and ai_summary_access from the parent past meeting record.
	// ai_summary_access only applies once the summary is approved or does not require
	// approval; a summary awaiting approval is indexed for organizers only (see
	// eventing.PastMeetingSummaryIndexingConfig).
	// ErrKeyNotFound is a permanent miss — skip without retry.
	// Any other KV or decode error is transient — retry.
	aiSummaryAccess := ""
	pastMeetingKey := fmt.Sprintf("itx-zoom-past-meetings.%s", summaryData.MeetingAndOccurrenceID)
	entry, kvErr := h.v1ObjectsKV.Get(ctx, pastMeetingKey)
	if kvErr != nil {
		if errors.Is(kvErr, jetstream.ErrKeyNotFound) {
			funcLogger.WarnContext(ctx, "skipping summary: parent past meeting not found")
			return false
		}
		funcLogger.With(logging.ErrKey, kvErr).WarnContext(ctx, "transient error fetching parent past meeting, will retry")
		return true
	}
	pastMeetingData, decErr := decodeData(entry.Value())
	if decErr != nil {
		if errors.Is(decErr, errMsgpackStructural) {
			funcLogger.With(logging.ErrKey, decErr).ErrorContext(ctx, "permanent decode failure for parent past meeting, skipping summary")
			return false
		}
		funcLogger.With(logging.ErrKey, decErr).WarnContext(ctx, "transient error decoding parent past meeting, will retry")
		return true
	}
	if access, ok := pastMeetingData["ai_summary_access"].(string); ok {
		aiSummaryAccess = access
	}
	projSFID := utils.GetString(pastMeetingData["proj_id"])
	primaryCommitteeSFID := utils.GetString(pastMeetingData["committee"])
	summaryData.ProjectSlug = utils.GetString(pastMeetingData["project_slug"])
	if projSFID != "" {
		projectUID, mapErr := h.idMapper.MapProjectV1ToV2(ctx, projSFID)
		if mapErr != nil {
			funcLogger.With(logging.ErrKey, mapErr).WarnContext(ctx, "error mapping project v1 to v2 for summary")
			return isTransientError(mapErr)
		}
		summaryData.ProjectUID = projectUID
	}

	// Skip if project is not yet mapped to v2
	if summaryData.ProjectUID == "" {
		funcLogger.WarnContext(ctx, "skipping summary: project not yet in v2")
		return false
	}

	committees, commErr := resolveParentPastMeetingCommittees(ctx, summaryData.MeetingAndOccurrenceID, primaryCommitteeSFID, h.idMapper, h.v1MappingsKV, funcLogger)
	if commErr != nil {
		funcLogger.With(logging.ErrKey, commErr).WarnContext(ctx, "transient error resolving parent committees for summary, will retry")
		return true
	}
	summaryData.Committees = committees

	// Publish to the indexer (summaries send no FGA message)
	if err := h.publisher.PublishPastMeetingSummaryEvent(ctx, string(indexerAction), summaryData, aiSummaryAccess); err != nil {
		funcLogger.With(logging.ErrKey, err).ErrorContext(ctx, "failed to publish summary event")
		return isTransientError(err)
	}

	// Store mapping
	if _, err := h.v1MappingsKV.Put(ctx, mappingKey, []byte("1")); err != nil {
		funcLogger.With(logging.ErrKey, err).WarnContext(ctx, "failed to store summary mapping")
	}

	funcLogger.InfoContext(ctx, "successfully processed past meeting summary", "action", string(indexerAction))
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
	h.logger.InfoContext(ctx, "processing past meeting summary delete", "summary_id", summaryID)
	return h.handleMeetingTypeDelete(ctx, key, summaryID, []byte(summaryID), meetingDeleteConfig{
		indexerSubject:   "lfx.index.v1_past_meeting_summary",
		tombstoneKeyFmts: []string{"v1_past_meeting_summaries.%s"},
	})
}

// pastMeetingSummaryIndexKeyPrefix prefixes the v1-mappings key that records which summaries
// belong to a past meeting: {prefix}{meeting_and_occurrence_id} holds a JSON array of summary
// IDs. The summary KV record is keyed by summary ID and carries no access field, so a change
// to the parent's ai_summary_access produces no summary event; the past meeting handler reads
// this key to republish the summaries itself.
const pastMeetingSummaryIndexKeyPrefix = "v1-mappings.past-meeting-summaries."

// maxSummaryIndexUpdateAttempts bounds the compare-and-swap retries in recordPastMeetingSummary
// when concurrent summary events for the same past meeting race on its key.
const maxSummaryIndexUpdateAttempts = 5

// recordPastMeetingSummary adds summaryID to the set of summaries recorded under the past
// meeting. The set is updated with compare-and-swap so concurrent summary events for the same
// past meeting cannot drop each other's entries.
func recordPastMeetingSummary(ctx context.Context, mappingsKV jetstream.KeyValue, meetingAndOccurrenceID, summaryID string) error {
	key := pastMeetingSummaryIndexKeyPrefix + meetingAndOccurrenceID
	for attempt := 0; attempt < maxSummaryIndexUpdateAttempts; attempt++ {
		entry, err := mappingsKV.Get(ctx, key)
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			value, marshalErr := json.Marshal([]string{summaryID})
			if marshalErr != nil {
				return marshalErr
			}
			_, err = mappingsKV.Create(ctx, key, value)
		} else if err == nil {
			var ids []string
			if unmarshalErr := json.Unmarshal(entry.Value(), &ids); unmarshalErr != nil {
				return fmt.Errorf("failed to decode summaries of past meeting: %w", unmarshalErr)
			}
			if slices.Contains(ids, summaryID) {
				return nil
			}
			value, marshalErr := json.Marshal(append(ids, summaryID))
			if marshalErr != nil {
				return marshalErr
			}
			_, err = mappingsKV.Update(ctx, key, value, entry.Revision())
		}
		// A lost Create race returns ErrKeyExists; an Update revision conflict returns
		// ErrKeyRevisionMismatch (which ErrKeyExists does not match on replicated streams).
		if !errors.Is(err, jetstream.ErrKeyExists) && !errors.Is(err, jetstream.ErrKeyRevisionMismatch) {
			return err
		}
	}
	return fmt.Errorf("failed to record summary under its past meeting: too many concurrent updates")
}

// listPastMeetingSummaryIDs returns the IDs of the summaries recorded under the past meeting
// by handlePastMeetingSummaryUpdate. Any KV or decode error is returned so the caller retries
// rather than acting on an incomplete list.
func listPastMeetingSummaryIDs(ctx context.Context, meetingAndOccurrenceID string, mappingsKV jetstream.KeyValue) ([]string, error) {
	entry, err := mappingsKV.Get(ctx, pastMeetingSummaryIndexKeyPrefix+meetingAndOccurrenceID)
	if err != nil {
		// An invalid key is permanent and recordPastMeetingSummary could never have written
		// it, so it means no summaries are recorded, the same as a missing key.
		if isKVAbsenceError(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to load summaries of past meeting: %w", err)
	}
	var ids []string
	if err := json.Unmarshal(entry.Value(), &ids); err != nil {
		return nil, fmt.Errorf("failed to decode summaries of past meeting: %w", err)
	}
	return ids, nil
}

// =============================================================================
// Summary Conversion Functions
// =============================================================================

func convertMapToSummaryData(
	ctx context.Context,
	v1Data map[string]interface{},
	logger *slog.Logger,
) (*models.SummaryEventData, error) {
	rawSummary, err := decodeV1[summaryDBRaw](v1Data)
	if err != nil {
		return nil, fmt.Errorf("failed to decode summary data: %w", err)
	}

	// Validate required fields
	if rawSummary.ID == "" || rawSummary.MeetingAndOccurrenceID == "" {
		return nil, fmt.Errorf("missing required fields: id or meeting_and_occurrence_id")
	}

	// Consolidate the original and the edited summary parts into the shared markdown
	// format (pkg/summaryformat), the same text the proxy returns and parses.
	content := summaryformat.Format(summaryformat.Parts{
		Overview:  rawSummary.SummaryOverview,
		Details:   rawSummary.SummaryDetails,
		NextSteps: rawSummary.NextSteps,
	})
	editedContent := summaryformat.Format(summaryformat.Parts{
		Overview:  rawSummary.EditedSummaryOverview,
		Details:   rawSummary.EditedSummaryDetails,
		NextSteps: rawSummary.EditedNextSteps,
	})

	// Parse times
	createdAt, _ := parseTime(rawSummary.CreatedAt)
	updatedAt, _ := parseTime(rawSummary.ModifiedAt)

	return &models.SummaryEventData{
		ID:                      rawSummary.ID,
		MeetingAndOccurrenceID:  rawSummary.MeetingAndOccurrenceID,
		MeetingID:               rawSummary.MeetingID,
		OccurrenceID:            rawSummary.OccurrenceID,
		ZoomMeetingUUID:         rawSummary.ZoomMeetingUUID,
		ZoomMeetingHostID:       rawSummary.ZoomMeetingHostID,
		ZoomMeetingHostEmail:    rawSummary.ZoomMeetingHostEmail,
		ZoomMeetingTopic:        rawSummary.ZoomMeetingTopic,
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
