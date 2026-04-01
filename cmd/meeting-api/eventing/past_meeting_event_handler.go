// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"

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

// PastMeetingDBRaw represents raw past meeting data from v1 DynamoDB/NATS KV bucket
type PastMeetingDBRaw struct {
	// MeetingAndOccurrenceID is the partition key of the past meeting table
	MeetingAndOccurrenceID string `json:"meeting_and_occurrence_id"`

	// ProjectID is the ID of the salesforce (v1) project associated with the past meeting
	ProjectID string `json:"proj_id"`

	// ProjectSlug is the slug of the project associated with the past meeting
	ProjectSlug string `json:"project_slug"`

	// Committee is the ID of the committee associated with the past meeting
	Committee string `json:"committee"`

	// CommitteeFilters is the list of filters associated with the committee
	CommitteeFilters []string `json:"committee_filters"`

	// Topic is the title/topic of the past meeting (v1 field name)
	Topic string `json:"topic"`

	// Agenda is the description/agenda of the past meeting (v1 field name)
	Agenda string `json:"agenda"`

	// Duration is the duration of the past meeting
	Duration int `json:"duration"`

	// MeetingID is the ID of the meeting associated with the past meeting
	MeetingID string `json:"meeting_id"`

	// OccurrenceID is the ID of the occurrence associated with the past meeting
	OccurrenceID string `json:"occurrence_id"`

	// RecordingAccess is the access type of the recording of the past meeting
	RecordingAccess string `json:"recording_access"`

	// RecordingEnabled is whether the recording of the past meeting is enabled
	RecordingEnabled bool `json:"recording_enabled"`

	// ScheduledStartTime is the scheduled start time of the past meeting.
	// This differs from the actual start time of the meeting because the [Sessions] stores
	// the actual start and end times of the meeting from Zoom of when it officially started.
	ScheduledStartTime string `json:"scheduled_start_time"`

	// ScheduledEndTime is the scheduled end time of the past meeting
	// This differs from the actual end time of the meeting because the [Sessions] stores
	// the actual start and end times of the meeting from Zoom of when it officially ended.
	ScheduledEndTime string `json:"scheduled_end_time"`

	// Sessions is the list of sessions associated with the past meeting
	Sessions []ZoomPastMeetingSession `json:"sessions"`

	// Timezone is the timezone of the past meeting
	Timezone string `json:"timezone"`

	// MeetingType is the type of the past meeting
	MeetingType string `json:"meeting_type"`

	// TranscriptAccess is the access type of the transcript of the past meeting
	TranscriptAccess string `json:"transcript_access"`

	// TranscriptEnabled is whether the transcript of the past meeting is enabled
	TranscriptEnabled bool `json:"transcript_enabled"`

	// Type is the type of the past meeting
	Type int `json:"type"`

	// Visibility is the visibility of the past meeting
	Visibility string `json:"visibility"`

	// Recurrence is the recurrence of the past meeting
	Recurrence *RecurrenceDBRaw `json:"recurrence"`

	// Restricted is whether the past meeting is restricted to only invited participants
	Restricted bool `json:"restricted"`

	// IsManuallyCreated indicates whether the past meeting was created manually
	IsManuallyCreated bool `json:"is_manually_created"`

	// RecordingPassword is the password of the past meeting recording
	// This is no longer relevant for recordings since sometime in 2023 because now the recordings
	// aren't hidden behind a password to access them.
	RecordingPassword string `json:"recording_password"`

	// ZoomAIEnabled is whether the meeting was hosted on a zoom user with AI-companion enabled
	ZoomAIEnabled *bool `json:"zoom_ai_enabled,omitempty"`

	// AISummaryAccess is the access level of the meeting AI summary within the LFX platform.
	AISummaryAccess string `json:"ai_summary_access,omitempty"`

	// RequireAISummaryApproval is whether the meeting requires approval of the AI summary
	RequireAISummaryApproval *bool `json:"require_ai_summary_approval,omitempty"`

	// EarlyJoinTime is the number of minutes before the scheduled start time that participants can join the meeting
	EarlyJoinTime int `json:"early_join_time"`

	// Artifacts is the list of artifacts for the past meeting
	Artifacts []ZoomPastMeetingArtifact `json:"artifacts"`

	// YoutubeLink is the link to the YouTube video of the past meeting
	YoutubeLink string `json:"youtube_link,omitempty"`

	// CreatedAt is the creation time of the past meeting
	CreatedAt string `json:"created_at"`

	// ModifiedAt is the last modification time in RFC3339 format of the past meeting
	ModifiedAt string `json:"modified_at"`

	// CreatedBy is the user who created the past meeting
	CreatedBy models.CreatedBy `json:"created_by"`

	// UpdatedBy is the user who last updated the past meeting
	UpdatedBy models.UpdatedBy `json:"updated_by"`

	// UpdatedByList is the list of users who have updated the past meeting
	UpdatedByList []models.UpdatedBy `json:"updated_by_list"`
}

// UnmarshalJSON implements custom unmarshaling to handle both string and int inputs for numeric fields.
func (p *PastMeetingDBRaw) UnmarshalJSON(data []byte) error {
	type Alias PastMeetingDBRaw
	tmp := struct {
		Duration      interface{} `json:"duration"`
		EarlyJoinTime interface{} `json:"early_join_time"`
		Type          interface{} `json:"type"`
		*Alias
	}{
		Alias: (*Alias)(p),
	}

	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}

	// Handle Duration
	switch v := tmp.Duration.(type) {
	case string:
		if v != "" {
			val, err := strconv.Atoi(v)
			if err != nil {
				return fmt.Errorf("invalid value for duration: %w", err)
			}
			p.Duration = val
		}
	case float64:
		p.Duration = int(v)
	case nil:
	default:
		return fmt.Errorf("invalid type for duration: %T", v)
	}

	// Handle EarlyJoinTime
	switch v := tmp.EarlyJoinTime.(type) {
	case string:
		if v != "" {
			val, err := strconv.Atoi(v)
			if err != nil {
				return fmt.Errorf("invalid value for early_join_time: %w", err)
			}
			p.EarlyJoinTime = val
		}
	case float64:
		p.EarlyJoinTime = int(v)
	case nil:
	default:
		return fmt.Errorf("invalid type for early_join_time: %T", v)
	}

	// Handle Type
	switch v := tmp.Type.(type) {
	case string:
		if v != "" {
			val, err := strconv.Atoi(v)
			if err != nil {
				return fmt.Errorf("invalid value for type: %w", err)
			}
			p.Type = val
		}
	case float64:
		p.Type = int(v)
	case nil:
	default:
		return fmt.Errorf("invalid type for type: %T", v)
	}

	return nil
}

// ZoomPastMeetingSession represents a single meeting instance/session
// A meeting being started then ended is one session, then restarting it is a second session.
type ZoomPastMeetingSession struct {
	// UUID is the UUID of the session.
	// This comes from Zoom when the meeting is started and ended. It is unique to each time
	// that the meeting is run, so if the same meeting is restarted then it will have a different UUID.
	UUID string `json:"uuid"`

	// StartTime is the start time of the session in RFC3339 format
	StartTime string `json:"start_time"`

	// EndTime is the end time of the session in RFC3339 format
	EndTime string `json:"end_time"`
}

// ZoomPastMeetingArtifact represents a a meeting artifact.
// An artifact is a link to a url where some information about the meeting can be found.
// For example a spreadsheet for meeting minutes or a link to an agenda can be represented
// by this artifact model.
type ZoomPastMeetingArtifact struct {
	// ID is the UUID of the artifact record.
	ID string `json:"id"`

	// Category is the category of the artifact.
	Category string `json:"category"`

	// Link is the link to the artifact.
	Link string `json:"link"`

	// Name is the name of the artifact.
	Name string `json:"name"`

	// CreatedAt is the creation time of the artifact in RFC3339 format.
	CreatedAt string `json:"created_at"`

	// CreatedBy is the user who created the artifact.
	CreatedBy models.CreatedBy `json:"created_by"`

	// UpdatedAt is the last modification time of the artifact in RFC3339 format.
	UpdatedAt string `json:"updated_at"`

	// UpdatedBy is the user who last updated the artifact.
	UpdatedBy models.UpdatedBy `json:"updated_by"`
}

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
	if pastMeetingData.ID == "" {
		funcLogger.ErrorContext(ctx, "missing required fields in past meeting data")
		return false
	}
	if pastMeetingData.ProjectUID == "" {
		funcLogger.InfoContext(ctx, "skipping past meeting sync - parent project not found in mappings", "project_sfid", pastMeetingData.ProjectSFID)
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
	if rawPastMeeting.MeetingAndOccurrenceID == "" || rawPastMeeting.ProjectID == "" {
		return nil, fmt.Errorf("missing required fields: meeting_and_occurrence_id or proj_id")
	}

	// Map project ID from v1 SFID to v2 UID. A missing mapping is not an error —
	// the caller checks ProjectUID == "" and skips (no retry).
	projectUID, _ := idMapper.MapProjectV1ToV2(ctx, rawPastMeeting.ProjectID)

	// Parse times
	startTime, _ := parseTime(rawPastMeeting.ScheduledStartTime)
	endTime, _ := parseTime(rawPastMeeting.ScheduledEndTime)
	createdAt, _ := parseTime(rawPastMeeting.CreatedAt)
	modifiedAt, _ := parseTime(rawPastMeeting.ModifiedAt)

	// Convert sessions
	var sessions []models.PastMeetingSession
	for _, rawSession := range rawPastMeeting.Sessions {
		sessionStart, _ := parseTime(rawSession.StartTime)
		sessionEnd, _ := parseTime(rawSession.EndTime)
		sessions = append(sessions, models.PastMeetingSession{
			UUID:      rawSession.UUID,
			StartTime: sessionStart,
			EndTime:   sessionEnd,
		})
	}

	// Get committees from mapping index (same logic as active meetings)
	committees := getCommitteesForPastMeeting(ctx, rawPastMeeting.MeetingAndOccurrenceID, idMapper, mappingsKV, logger)

	// Compute artifact visibility from access fields (fallback chain)
	artifactVisibility := rawPastMeeting.RecordingAccess
	if artifactVisibility == "" {
		artifactVisibility = rawPastMeeting.TranscriptAccess
	}
	if artifactVisibility == "" {
		artifactVisibility = rawPastMeeting.AISummaryAccess
	}
	if artifactVisibility == "" {
		artifactVisibility = "meeting_hosts"
	}

	// Build ZoomConfig from flat v1 fields
	zoomConfig := buildPastMeetingZoomConfig(&rawPastMeeting)

	// Build event data
	return &models.PastMeetingEventData{
		ID:                       rawPastMeeting.MeetingAndOccurrenceID,
		MeetingID:                rawPastMeeting.MeetingID,
		MeetingAndOccurrenceID:   rawPastMeeting.MeetingAndOccurrenceID,
		OccurrenceID:             rawPastMeeting.OccurrenceID,
		ProjectSFID:              rawPastMeeting.ProjectID,
		ProjectUID:               projectUID,
		ProjectSlug:              rawPastMeeting.ProjectSlug,
		Committee:                rawPastMeeting.Committee,
		CommitteeFilters:         rawPastMeeting.CommitteeFilters,
		Title:                    rawPastMeeting.Topic,
		Description:              rawPastMeeting.Agenda,
		StartTime:                startTime,
		EndTime:                  endTime,
		Duration:                 rawPastMeeting.Duration,
		Timezone:                 rawPastMeeting.Timezone,
		MeetingType:              rawPastMeeting.MeetingType,
		Committees:               committees,
		Visibility:               rawPastMeeting.Visibility,
		ArtifactVisibility:       artifactVisibility,
		Restricted:               rawPastMeeting.Restricted,
		RecordingEnabled:         rawPastMeeting.RecordingEnabled,
		RecordingAccess:          rawPastMeeting.RecordingAccess,
		TranscriptEnabled:        rawPastMeeting.TranscriptEnabled,
		TranscriptAccess:         rawPastMeeting.TranscriptAccess,
		ZoomAIEnabled:            rawPastMeeting.ZoomAIEnabled,
		AISummaryAccess:          rawPastMeeting.AISummaryAccess,
		RequireAISummaryApproval: rawPastMeeting.RequireAISummaryApproval,
		EarlyJoinTimeMinutes:     rawPastMeeting.EarlyJoinTime,
		YoutubeLink:              rawPastMeeting.YoutubeLink,
		RecordingPassword:        rawPastMeeting.RecordingPassword,
		ZoomConfig:               zoomConfig,
		IsManuallyCreated:        rawPastMeeting.IsManuallyCreated,
		Sessions:                 sessions,
		CreatedAt:                createdAt,
		UpdatedAt:                modifiedAt,
		CreatedBy:                models.CreatedBy(rawPastMeeting.CreatedBy),
		UpdatedBy:                models.UpdatedBy(rawPastMeeting.UpdatedBy),
		UpdatedByList:            rawPastMeeting.UpdatedByList,
	}, nil
}

// buildPastMeetingZoomConfig constructs a ZoomConfig from flat v1 fields on the raw past meeting.
// Returns nil if no source fields are present so ZoomConfig is omitted from the event payload.
func buildPastMeetingZoomConfig(m *PastMeetingDBRaw) *models.ZoomConfig {
	if m.ZoomAIEnabled == nil && m.RequireAISummaryApproval == nil {
		return nil
	}
	cfg := &models.ZoomConfig{}
	if m.ZoomAIEnabled != nil {
		cfg.AICompanionEnabled = *m.ZoomAIEnabled
	}
	if m.RequireAISummaryApproval != nil {
		cfg.AISummaryRequireApproval = *m.RequireAISummaryApproval
	}
	return cfg
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

	pastMeetingData, err := decodeData(pastMeetingEntry.Value())
	if err != nil {
		h.logger.With(logging.ErrKey, err).ErrorContext(ctx, "failed to decode past meeting data")
		return false
	}

	// Re-process the past meeting
	return h.handlePastMeetingUpdate(ctx, pastMeetingKey, pastMeetingData)
}
