// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	indexerConstants "github.com/linuxfoundation/lfx-v2-indexer-service/pkg/constants"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/nats-io/nats.go/jetstream"
)

const errKey = "error"

// =============================================================================
// Meeting Event Handler
// =============================================================================

// handleMeetingUpdate processes updates to meeting records
// Returns true to retry (NAK), false to acknowledge (ACK)
func handleMeetingUpdate(
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
	funcLogger := logger.With("key", key, "handler", "meeting")
	funcLogger.DebugContext(ctx, "processing meeting update")

	// Check if this is a soft delete
	if isDeleted, ok := v1Data["_sdc_deleted_at"].(string); ok && isDeleted != "" {
		return handleMeetingDelete(ctx, key, v1Data, publisher, mappingsKV, funcLogger)
	}

	// Convert v1Data to meeting event data
	meetingData, err := convertMapToMeetingData(ctx, v1Data, idMapper, v1ObjectsKV, mappingsKV, funcLogger)
	if err != nil {
		funcLogger.With(errKey, err).ErrorContext(ctx, "failed to convert v1Data to meeting")
		if isTransientError(err) {
			return true // NAK for retry
		}
		return false // Permanent error, ACK and skip
	}

	// Validate required fields
	if meetingData.ID == "" || meetingData.ProjectUID == "" {
		funcLogger.ErrorContext(ctx, "missing required fields in meeting data")
		return false // Permanent error, ACK and skip
	}
	funcLogger = funcLogger.With("meeting_id", meetingData.ID)

	// Determine action (created vs updated)
	mappingKey := fmt.Sprintf("v1_meetings.%s", meetingData.ID)
	indexerAction := indexerConstants.ActionCreated
	if _, err := mappingsKV.Get(ctx, mappingKey); err == nil {
		indexerAction = indexerConstants.ActionUpdated
	}

	// Publish to indexer and FGA-sync
	if err := publisher.PublishMeetingEvent(ctx, string(indexerAction), meetingData); err != nil {
		funcLogger.With(errKey, err).ErrorContext(ctx, "failed to publish meeting event")
		if isTransientError(err) {
			return true // NAK for retry
		}
		return false // Permanent error, ACK and skip
	}

	// Store mapping
	if _, err := mappingsKV.Put(ctx, mappingKey, []byte("1")); err != nil {
		funcLogger.With(errKey, err).WarnContext(ctx, "failed to store meeting mapping")
	}

	funcLogger.InfoContext(ctx, "successfully processed meeting")
	return false // Success, ACK
}

// handleMeetingDelete processes meeting deletions
func handleMeetingDelete(
	ctx context.Context,
	key string,
	v1Data map[string]interface{},
	publisher domain.EventPublisher,
	mappingsKV jetstream.KeyValue,
	logger *slog.Logger,
) bool {
	meetingID := extractIDFromKey(key, "itx-zoom-meetings-v2.")
	funcLogger := logger.With("meeting_id", meetingID, "handler", "meeting_delete")
	funcLogger.InfoContext(ctx, "processing meeting deletion")

	// Create minimal event data for deletion
	eventData := &models.MeetingEventData{ID: meetingID}

	// Publish delete event
	if err := publisher.PublishMeetingEvent(ctx, string(indexerConstants.ActionDeleted), eventData); err != nil {
		funcLogger.With(errKey, err).ErrorContext(ctx, "failed to publish meeting delete event")
		if isTransientError(err) {
			return true // NAK for retry
		}
		return false // Permanent error, ACK and skip
	}

	// Remove mapping
	mappingKey := fmt.Sprintf("v1_meetings.%s", meetingID)
	_ = mappingsKV.Delete(ctx, mappingKey)

	funcLogger.InfoContext(ctx, "successfully processed meeting deletion")
	return false // Success, ACK
}

// convertMapToMeetingData converts v1 meeting data to v2 format
func convertMapToMeetingData(
	ctx context.Context,
	v1Data map[string]interface{},
	idMapper domain.IDMapper,
	v1ObjectsKV jetstream.KeyValue,
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

	// Skip if created by this service (prevent sync loops)
	if shouldSkipSync(rawMeeting.LastModifiedByID) {
		logger.InfoContext(ctx, "skipping sync - created by this service", "last_modified_by", rawMeeting.LastModifiedByID)
		return nil, fmt.Errorf("skipping sync to prevent loop")
	}

	// Validate required fields
	if rawMeeting.MeetingID == "" || rawMeeting.ProjectID == "" {
		return nil, fmt.Errorf("missing required fields: meeting_id or proj_id")
	}

	// Map project ID from v1 SFID to v2 UID
	projectUID, err := idMapper.MapProjectV1ToV2(ctx, rawMeeting.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("failed to map project ID (transient): %w", err)
	}

	// Parse times
	startTime, err := parseTime(rawMeeting.StartTime)
	if err != nil {
		return nil, fmt.Errorf("failed to parse start time: %w", err)
	}

	createdAt, _ := parseTime(rawMeeting.CreatedAt)
	modifiedAt, _ := parseTime(rawMeeting.ModifiedAt)

	// Calculate occurrences if recurring
	var occurrences []models.Occurrence
	if rawMeeting.Recurrence != nil {
		recInput := convertToRecurrenceInput(rawMeeting.Recurrence)
		if recInput != nil {
			calc := NewOccurrenceCalculator(logger)
			occList, err := calc.CalculateOccurrences(
				startTime,
				getInt(rawMeeting.Duration),
				recInput,
				nil, // TODO: Handle cancelled occurrences
				nil, // TODO: Handle updated occurrences
			)
			if err != nil {
				logger.With(errKey, err).WarnContext(ctx, "failed to calculate occurrences")
			} else {
				occurrences = make([]models.Occurrence, len(occList))
				for i, occ := range occList {
					occurrences[i] = models.Occurrence{
						OccurrenceID: occ.OccurrenceID,
						StartTime:    occ.StartTime,
						Duration:     occ.Duration,
						Status:       occ.Status,
					}
				}
			}
		}
	}

	// Get committees from mapping index
	committees := getCommitteesForMeeting(ctx, rawMeeting.MeetingID, idMapper, mappingsKV, logger)

	// Determine artifact visibility (priority: recording > transcript > ai_summary)
	artifactVisibility := getString(rawMeeting.RecordingAccess)
	if artifactVisibility == "" {
		artifactVisibility = getString(rawMeeting.TranscriptAccess)
	}
	if artifactVisibility == "" {
		artifactVisibility = getString(rawMeeting.AISummaryAccess)
	}
	if artifactVisibility == "" {
		artifactVisibility = "meeting_hosts"
	}

	// Build event data
	return &models.MeetingEventData{
		ID:                   rawMeeting.MeetingID,
		ProjectUID:           projectUID,
		Title:                rawMeeting.Topic,
		Description:          rawMeeting.Agenda,
		StartTime:            startTime,
		Duration:             getInt(rawMeeting.Duration),
		Timezone:             rawMeeting.Timezone,
		Visibility:           rawMeeting.Visibility,
		Restricted:           getBool(rawMeeting.Restricted),
		MeetingType:          getString(rawMeeting.MeetingType),
		EarlyJoinTimeMinutes: getInt(rawMeeting.EarlyJoinTime),
		RecordingEnabled:     getBool(rawMeeting.RecordingEnabled),
		TranscriptEnabled:    getBool(rawMeeting.TranscriptEnabled),
		YoutubeUploadEnabled: getBool(rawMeeting.YoutubeUploadEnabled),
		ArtifactVisibility:   artifactVisibility,
		Committees:           committees,
		Occurrences:          occurrences,
		HostKey:              rawMeeting.HostKey,
		Passcode:             rawMeeting.Passcode,
		PublicLink:           rawMeeting.PublicLink,
		CreatedAt:            createdAt,
		ModifiedAt:           modifiedAt,
		Tags:                 generateMeetingTags(&rawMeeting, projectUID),
	}, nil
}

// =============================================================================
// Meeting Mapping Event Handler
// =============================================================================

// handleMeetingMappingUpdate processes updates to meeting-committee mappings
func handleMeetingMappingUpdate(
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
	funcLogger := logger.With("key", key, "handler", "meeting_mapping")
	funcLogger.InfoContext(ctx, "processing meeting mapping update")

	// Check if this is a soft delete
	if isDeleted, ok := v1Data["_sdc_deleted_at"].(string); ok && isDeleted != "" {
		return handleMeetingMappingDelete(ctx, key, v1Data, publisher, userLookup, idMapper, v1ObjectsKV, mappingsKV, funcLogger)
	}

	// Extract meeting ID and mapping data
	meetingID := getString(v1Data["meeting_id"])
	mappingID := getString(v1Data["id"])
	committeeID := getString(v1Data["committee_id"])

	if meetingID == "" || mappingID == "" {
		funcLogger.WarnContext(ctx, "missing required fields in mapping")
		return false // ACK - permanent error
	}

	// Update committee mappings in KV bucket
	if err := updateCommitteeMappings(ctx, meetingID, mappingID, committeeID, v1Data, mappingsKV, funcLogger); err != nil {
		funcLogger.With(errKey, err).ErrorContext(ctx, "failed to update committee mappings")
		if isTransientError(err) {
			return true // NAK for retry
		}
		return false // ACK - permanent error
	}

	// Re-trigger meeting indexing
	shouldRetry := retriggerMeetingIndexing(ctx, meetingID, publisher, userLookup, idMapper, v1ObjectsKV, mappingsKV, funcLogger)
	if shouldRetry {
		return true // NAK for retry
	}

	funcLogger.InfoContext(ctx, "successfully processed meeting mapping update")
	return false // ACK
}

// handleMeetingMappingDelete processes meeting-committee mapping deletions
func handleMeetingMappingDelete(
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
	meetingID := getString(v1Data["meeting_id"])
	mappingID := getString(v1Data["id"])

	if meetingID == "" || mappingID == "" {
		logger.WarnContext(ctx, "missing required fields in mapping deletion")
		return false // ACK - permanent error
	}

	funcLogger := logger.With("meeting_id", meetingID, "mapping_id", mappingID, "handler", "meeting_mapping_delete")
	funcLogger.InfoContext(ctx, "processing meeting mapping deletion")

	// Remove the mapping
	if err := removeCommitteeMapping(ctx, meetingID, mappingID, mappingsKV, funcLogger); err != nil {
		if isTransientError(err) {
			return true // NAK for retry
		}
	}

	// Re-trigger meeting indexing
	shouldRetry := retriggerMeetingIndexing(ctx, meetingID, publisher, userLookup, idMapper, v1ObjectsKV, mappingsKV, funcLogger)
	if shouldRetry {
		return true // NAK for retry
	}

	funcLogger.InfoContext(ctx, "successfully processed meeting mapping deletion")
	return false // ACK
}

// =============================================================================
// Registrant Event Handler
// =============================================================================

// handleRegistrantUpdate processes updates to meeting registrants
func handleRegistrantUpdate(
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
	funcLogger := logger.With("key", key, "handler", "registrant")
	funcLogger.DebugContext(ctx, "processing registrant update")

	// Check if this is a soft delete
	if isDeleted, ok := v1Data["_sdc_deleted_at"].(string); ok && isDeleted != "" {
		return handleRegistrantDelete(ctx, key, v1Data, publisher, mappingsKV, funcLogger)
	}

	// Convert v1Data to registrant event data
	registrantData, err := convertMapToRegistrantData(ctx, v1Data, userLookup, idMapper, v1ObjectsKV, funcLogger)
	if err != nil {
		funcLogger.With(errKey, err).ErrorContext(ctx, "failed to convert v1Data to registrant")
		if isTransientError(err) {
			return true // NAK for retry
		}
		return false // Permanent error, ACK and skip
	}

	// Validate required fields
	if registrantData.UID == "" || registrantData.MeetingID == "" {
		funcLogger.ErrorContext(ctx, "missing required fields in registrant data")
		return false // Permanent error, ACK and skip
	}
	funcLogger = funcLogger.With("registrant_uid", registrantData.UID)

	// Determine action (created vs updated)
	mappingKey := fmt.Sprintf("v1_registrants.%s", registrantData.UID)
	indexerAction := indexerConstants.ActionCreated
	if _, err := mappingsKV.Get(ctx, mappingKey); err == nil {
		indexerAction = indexerConstants.ActionUpdated
	}

	// Publish to indexer and FGA-sync
	if err := publisher.PublishRegistrantEvent(ctx, string(indexerAction), registrantData); err != nil {
		funcLogger.With(errKey, err).ErrorContext(ctx, "failed to publish registrant event")
		if isTransientError(err) {
			return true // NAK for retry
		}
		return false // Permanent error, ACK and skip
	}

	// Store mapping
	if _, err := mappingsKV.Put(ctx, mappingKey, []byte("1")); err != nil {
		funcLogger.With(errKey, err).WarnContext(ctx, "failed to store registrant mapping")
	}

	funcLogger.InfoContext(ctx, "successfully processed registrant")
	return false // Success, ACK
}

// handleRegistrantDelete processes registrant deletions
func handleRegistrantDelete(
	ctx context.Context,
	key string,
	v1Data map[string]interface{},
	publisher domain.EventPublisher,
	mappingsKV jetstream.KeyValue,
	logger *slog.Logger,
) bool {
	registrantUID := extractIDFromKey(key, "itx-zoom-meetings-registrants-v2.")
	funcLogger := logger.With("registrant_uid", registrantUID, "handler", "registrant_delete")
	funcLogger.InfoContext(ctx, "processing registrant deletion")

	// Create minimal event data for deletion
	eventData := &models.RegistrantEventData{UID: registrantUID}

	// Publish delete event
	if err := publisher.PublishRegistrantEvent(ctx, string(indexerConstants.ActionDeleted), eventData); err != nil {
		funcLogger.With(errKey, err).ErrorContext(ctx, "failed to publish registrant delete event")
		if isTransientError(err) {
			return true // NAK for retry
		}
		return false // Permanent error, ACK and skip
	}

	// Remove mapping
	mappingKey := fmt.Sprintf("v1_registrants.%s", registrantUID)
	_ = mappingsKV.Delete(ctx, mappingKey)

	funcLogger.InfoContext(ctx, "successfully processed registrant deletion")
	return false // Success, ACK
}

// convertMapToRegistrantData converts v1 registrant data to v2 format
func convertMapToRegistrantData(
	ctx context.Context,
	v1Data map[string]interface{},
	userLookup domain.V1UserLookup,
	idMapper domain.IDMapper,
	v1ObjectsKV jetstream.KeyValue,
	logger *slog.Logger,
) (*models.RegistrantEventData, error) {
	// Convert map to JSON bytes, then to RegistrantDBRaw
	jsonBytes, err := json.Marshal(v1Data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal v1Data to JSON: %w", err)
	}

	var rawRegistrant RegistrantDBRaw
	if err := json.Unmarshal(jsonBytes, &rawRegistrant); err != nil {
		return nil, fmt.Errorf("failed to unmarshal registrant data: %w", err)
	}

	// Validate required fields
	if rawRegistrant.UID == "" || rawRegistrant.MeetingID == "" {
		return nil, fmt.Errorf("missing required fields: uid or meeting_id")
	}

	// Parent validation - meeting must exist
	meetingKey := fmt.Sprintf("itx-zoom-meetings-v2.%s", rawRegistrant.MeetingID)
	meetingEntry, err := v1ObjectsKV.Get(ctx, meetingKey)
	if err != nil {
		return nil, fmt.Errorf("parent meeting not found (transient): %w", err)
	}

	// Get project ID from meeting
	var meetingData map[string]interface{}
	if err := json.Unmarshal(meetingEntry.Value(), &meetingData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal meeting data: %w", err)
	}

	projectSFID := getString(meetingData["proj_id"])
	if projectSFID == "" {
		return nil, fmt.Errorf("meeting missing project ID")
	}

	// Map project ID from v1 SFID to v2 UID
	projectUID, err := idMapper.MapProjectV1ToV2(ctx, projectSFID)
	if err != nil {
		return nil, fmt.Errorf("failed to map project ID (transient): %w", err)
	}

	// Map committee ID if present
	var committeeUID string
	if rawRegistrant.CommitteeID != "" {
		committeeUID, err = idMapper.MapCommitteeV1ToV2(ctx, rawRegistrant.CommitteeID)
		if err != nil {
			logger.With(errKey, err).WarnContext(ctx, "failed to map committee ID", "v1_id", rawRegistrant.CommitteeID)
			// Don't fail - just omit committee
		}
	}

	// Username resolution via V1UserLookup if username blank but user_id exists
	username := rawRegistrant.Username
	if username == "" && rawRegistrant.UserID != "" {
		v1User, err := userLookup.LookupUser(ctx, rawRegistrant.UserID)
		if err != nil {
			logger.With(errKey, err).WarnContext(ctx, "failed to lookup v1 user", "user_id", rawRegistrant.UserID)
		} else if v1User != nil {
			username = v1User.Username
			// Enrich with other user data if available
			if rawRegistrant.Email == "" {
				rawRegistrant.Email = v1User.Email
			}
			if rawRegistrant.FirstName == "" {
				rawRegistrant.FirstName = v1User.FirstName
			}
			if rawRegistrant.LastName == "" {
				rawRegistrant.LastName = v1User.LastName
			}
			if rawRegistrant.AvatarURL == "" {
				rawRegistrant.AvatarURL = v1User.AvatarURL
			}
			if rawRegistrant.OrgName == "" {
				rawRegistrant.OrgName = v1User.OrgName
			}
		}
	}

	// Parse times
	createdAt, _ := parseTime(rawRegistrant.CreatedAt)
	modifiedAt, _ := parseTime(rawRegistrant.ModifiedAt)

	return &models.RegistrantEventData{
		UID:          rawRegistrant.UID,
		MeetingID:    rawRegistrant.MeetingID,
		ProjectUID:   projectUID,
		CommitteeUID: committeeUID,
		UserID:       rawRegistrant.UserID,
		Username:     username,
		Email:        rawRegistrant.Email,
		FirstName:    rawRegistrant.FirstName,
		LastName:     rawRegistrant.LastName,
		AvatarURL:    rawRegistrant.AvatarURL,
		OrgName:      rawRegistrant.OrgName,
		Host:         getBool(rawRegistrant.Host),
		CreatedAt:    createdAt,
		ModifiedAt:   modifiedAt,
		Tags:         generateRegistrantTags(&rawRegistrant, projectUID, username),
	}, nil
}

// =============================================================================
// Invite Response (RSVP) Event Handler
// =============================================================================

// handleInviteResponseUpdate processes updates to meeting invite responses (RSVPs)
func handleInviteResponseUpdate(
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
	funcLogger := logger.With("key", key, "handler", "invite_response")
	funcLogger.DebugContext(ctx, "processing invite response update")

	// Check if this is a soft delete
	if isDeleted, ok := v1Data["_sdc_deleted_at"].(string); ok && isDeleted != "" {
		return handleInviteResponseDelete(ctx, key, v1Data, publisher, mappingsKV, funcLogger)
	}

	// Convert v1Data to invite response event data
	responseData, err := convertMapToInviteResponseData(ctx, v1Data, idMapper, v1ObjectsKV, funcLogger)
	if err != nil {
		funcLogger.With(errKey, err).ErrorContext(ctx, "failed to convert v1Data to invite response")
		if isTransientError(err) {
			return true // NAK for retry
		}
		return false // Permanent error, ACK and skip
	}

	// Validate required fields
	if responseData.ID == "" || responseData.MeetingID == "" {
		funcLogger.ErrorContext(ctx, "missing required fields in invite response data")
		return false // Permanent error, ACK and skip
	}
	funcLogger = funcLogger.With("response_id", responseData.ID)

	// Determine action (created vs updated)
	mappingKey := fmt.Sprintf("v1_invite_responses.%s", responseData.ID)
	indexerAction := indexerConstants.ActionCreated
	if _, err := mappingsKV.Get(ctx, mappingKey); err == nil {
		indexerAction = indexerConstants.ActionUpdated
	}

	// Publish to indexer
	if err := publisher.PublishInviteResponseEvent(ctx, string(indexerAction), responseData); err != nil {
		funcLogger.With(errKey, err).ErrorContext(ctx, "failed to publish invite response event")
		if isTransientError(err) {
			return true // NAK for retry
		}
		return false // Permanent error, ACK and skip
	}

	// Store mapping
	if _, err := mappingsKV.Put(ctx, mappingKey, []byte("1")); err != nil {
		funcLogger.With(errKey, err).WarnContext(ctx, "failed to store invite response mapping")
	}

	funcLogger.InfoContext(ctx, "successfully processed invite response")
	return false // Success, ACK
}

// handleInviteResponseDelete processes invite response deletions
func handleInviteResponseDelete(
	ctx context.Context,
	key string,
	v1Data map[string]interface{},
	publisher domain.EventPublisher,
	mappingsKV jetstream.KeyValue,
	logger *slog.Logger,
) bool {
	responseID := extractIDFromKey(key, "itx-zoom-meetings-invite-responses-v2.")
	funcLogger := logger.With("response_id", responseID, "handler", "invite_response_delete")
	funcLogger.InfoContext(ctx, "processing invite response deletion")

	// Create minimal event data for deletion
	eventData := &models.InviteResponseEventData{ID: responseID}

	// Publish delete event
	if err := publisher.PublishInviteResponseEvent(ctx, string(indexerConstants.ActionDeleted), eventData); err != nil {
		funcLogger.With(errKey, err).ErrorContext(ctx, "failed to publish invite response delete event")
		if isTransientError(err) {
			return true // NAK for retry
		}
		return false // Permanent error, ACK and skip
	}

	// Remove mapping
	mappingKey := fmt.Sprintf("v1_invite_responses.%s", responseID)
	_ = mappingsKV.Delete(ctx, mappingKey)

	funcLogger.InfoContext(ctx, "successfully processed invite response deletion")
	return false // Success, ACK
}

// convertMapToInviteResponseData converts v1 invite response data to v2 format
func convertMapToInviteResponseData(
	ctx context.Context,
	v1Data map[string]interface{},
	idMapper domain.IDMapper,
	v1ObjectsKV jetstream.KeyValue,
	logger *slog.Logger,
) (*models.InviteResponseEventData, error) {
	// Convert map to JSON bytes, then to InviteResponseDBRaw
	jsonBytes, err := json.Marshal(v1Data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal v1Data to JSON: %w", err)
	}

	var rawResponse InviteResponseDBRaw
	if err := json.Unmarshal(jsonBytes, &rawResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal invite response data: %w", err)
	}

	// Validate required fields
	if rawResponse.ID == "" || rawResponse.MeetingID == "" {
		return nil, fmt.Errorf("missing required fields: id or meeting_id")
	}

	// Filter out mailer daemon emails
	if strings.Contains(strings.ToLower(rawResponse.Email), "mailer-daemon@") {
		return nil, fmt.Errorf("skipping mailer daemon response")
	}

	// Get project ID from meeting
	meetingKey := fmt.Sprintf("itx-zoom-meetings-v2.%s", rawResponse.MeetingID)
	meetingEntry, err := v1ObjectsKV.Get(ctx, meetingKey)
	if err != nil {
		return nil, fmt.Errorf("parent meeting not found (transient): %w", err)
	}

	var meetingData map[string]interface{}
	if err := json.Unmarshal(meetingEntry.Value(), &meetingData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal meeting data: %w", err)
	}

	projectSFID := getString(meetingData["proj_id"])
	if projectSFID == "" {
		return nil, fmt.Errorf("meeting missing project ID")
	}

	// Map project ID
	projectUID, err := idMapper.MapProjectV1ToV2(ctx, projectSFID)
	if err != nil {
		return nil, fmt.Errorf("failed to map project ID (transient): %w", err)
	}

	// Map response type
	responseType := mapResponseType(rawResponse.ResponseType)

	// Map scope
	scope := mapResponseScope(rawResponse.Scope)

	// Determine if meeting is recurring
	isRecurring := meetingData["recurrence"] != nil

	// Parse times
	createdAt, _ := parseTime(rawResponse.CreatedAt)
	modifiedAt, _ := parseTime(rawResponse.ModifiedAt)

	return &models.InviteResponseEventData{
		ID:           rawResponse.ID,
		MeetingID:    rawResponse.MeetingID,
		ProjectUID:   projectUID,
		UserID:       rawResponse.UserID,
		Email:        rawResponse.Email,
		ResponseType: responseType,
		Scope:        scope,
		OccurrenceID: rawResponse.OccurrenceID,
		IsRecurring:  isRecurring,
		CreatedAt:    createdAt,
		ModifiedAt:   modifiedAt,
		Tags:         generateInviteResponseTags(&rawResponse, projectUID),
	}, nil
}

// =============================================================================
// Helper Functions
// =============================================================================

// Data models for v1 raw data

type MeetingDBRaw struct {
	MeetingID            string                 `json:"meeting_id"`
	ProjectID            string                 `json:"proj_id"`
	Topic                string                 `json:"topic"`
	Agenda               string                 `json:"agenda"`
	StartTime            string                 `json:"start_time"`
	Duration             interface{}            `json:"duration"`
	Timezone             string                 `json:"timezone"`
	Visibility           string                 `json:"visibility"`
	Restricted           interface{}            `json:"restricted"`
	MeetingType          interface{}            `json:"meeting_type"`
	EarlyJoinTime        interface{}            `json:"early_join_time"`
	RecordingEnabled     interface{}            `json:"recording_enabled"`
	TranscriptEnabled    interface{}            `json:"transcript_enabled"`
	YoutubeUploadEnabled interface{}            `json:"youtube_upload_enabled"`
	RecordingAccess      interface{}            `json:"recording_access"`
	TranscriptAccess     interface{}            `json:"transcript_access"`
	AISummaryAccess      interface{}            `json:"ai_summary_access"`
	Recurrence           map[string]interface{} `json:"recurrence"`
	HostKey              string                 `json:"host_key"`
	Passcode             string                 `json:"passcode"`
	Password             string                 `json:"password"`
	PublicLink           string                 `json:"public_link"`
	CreatedAt            string                 `json:"created_at"`
	ModifiedAt           string                 `json:"modified_at"`
	LastModifiedByID     string                 `json:"lastmodifiedbyid"`
}

type RegistrantDBRaw struct {
	UID         string      `json:"uid"`
	MeetingID   string      `json:"meeting_id"`
	CommitteeID string      `json:"committee_id"`
	UserID      string      `json:"user_id"`
	Username    string      `json:"username"`
	Email       string      `json:"email"`
	FirstName   string      `json:"first_name"`
	LastName    string      `json:"last_name"`
	AvatarURL   string      `json:"avatar_url"`
	OrgName     string      `json:"org_name"`
	Host        interface{} `json:"host"`
	CreatedAt   string      `json:"created_at"`
	ModifiedAt  string      `json:"modified_at"`
}

type InviteResponseDBRaw struct {
	ID           string `json:"id"`
	MeetingID    string `json:"meeting_id"`
	UserID       string `json:"user_id"`
	Email        string `json:"email"`
	ResponseType string `json:"response_type"`
	Scope        string `json:"scope"`
	OccurrenceID string `json:"occurrence_id"`
	CreatedAt    string `json:"created_at"`
	ModifiedAt   string `json:"modified_at"`
}

// Conversion and utility functions

func shouldSkipSync(lastModifiedByID string) bool {
	return lastModifiedByID == "meeting-service" || lastModifiedByID == "lfx-v2-meeting-service"
}

func convertToRecurrenceInput(rec map[string]interface{}) *RecurrenceInput {
	if rec == nil {
		return nil
	}
	return &RecurrenceInput{
		Type:           getIntFromMap(rec, "type"),
		RepeatInterval: getIntFromMap(rec, "repeat_interval"),
		WeeklyDays:     getStringFromMap(rec, "weekly_days"),
		MonthlyDay:     getIntFromMap(rec, "monthly_day"),
		MonthlyWeek:    getIntFromMap(rec, "monthly_week"),
		MonthlyWeekDay: getIntFromMap(rec, "monthly_week_day"),
		EndTimes:       getIntFromMap(rec, "end_times"),
		EndDateTime:    getStringFromMap(rec, "end_date_time"),
	}
}

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
		logger.With(errKey, err).WarnContext(ctx, "failed to unmarshal committee mappings")
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
			logger.With(errKey, err).ErrorContext(ctx, "failed to unmarshal existing mappings")
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

	// Remove the mapping
	delete(mappings, mappingID)

	// If no mappings left, delete the key
	if len(mappings) == 0 {
		return mappingsKV.Delete(ctx, mappingsKey)
	}

	// Store updated mappings
	mappingsJSON, err := json.Marshal(mappings)
	if err != nil {
		return fmt.Errorf("failed to marshal updated mappings: %w", err)
	}

	_, err = mappingsKV.Put(ctx, mappingsKey, mappingsJSON)
	return err
}

func retriggerMeetingIndexing(
	ctx context.Context,
	meetingID string,
	publisher domain.EventPublisher,
	userLookup domain.V1UserLookup,
	idMapper domain.IDMapper,
	v1ObjectsKV jetstream.KeyValue,
	mappingsKV jetstream.KeyValue,
	logger *slog.Logger,
) bool {
	meetingKey := fmt.Sprintf("itx-zoom-meetings-v2.%s", meetingID)
	meetingEntry, err := v1ObjectsKV.Get(ctx, meetingKey)
	if err != nil {
		logger.With(errKey, err).WarnContext(ctx, "meeting not found during retrigger")
		return false // Meeting might be deleted, ACK
	}

	var meetingData map[string]interface{}
	if err := json.Unmarshal(meetingEntry.Value(), &meetingData); err != nil {
		logger.With(errKey, err).ErrorContext(ctx, "failed to unmarshal meeting data")
		return false // ACK - permanent error
	}

	// Re-process the meeting
	return handleMeetingUpdate(ctx, meetingKey, meetingData, publisher, userLookup, idMapper, v1ObjectsKV, mappingsKV, logger)
}

func generateMeetingTags(m *MeetingDBRaw, projectUID string) []string {
	tags := []string{
		"meeting_id:" + m.MeetingID,
		"project_uid:" + projectUID,
		"title:" + m.Topic,
		"visibility:" + m.Visibility,
	}
	if m.MeetingType != nil {
		if mt := getString(m.MeetingType); mt != "" {
			tags = append(tags, "meeting_type:"+mt)
		}
	}
	return tags
}

func generateRegistrantTags(r *RegistrantDBRaw, projectUID, username string) []string {
	tags := []string{
		"registrant_uid:" + r.UID,
		"meeting_id:" + r.MeetingID,
		"project_uid:" + projectUID,
	}
	if username != "" {
		tags = append(tags, "username:"+username)
	}
	if r.Email != "" {
		tags = append(tags, "email:"+r.Email)
	}
	if r.CommitteeID != "" {
		tags = append(tags, "committee_id:"+r.CommitteeID)
	}
	if getBool(r.Host) {
		tags = append(tags, "is_host:true")
	}
	return tags
}

func generateInviteResponseTags(r *InviteResponseDBRaw, projectUID string) []string {
	tags := []string{
		"response_id:" + r.ID,
		"meeting_id:" + r.MeetingID,
		"project_uid:" + projectUID,
		"response_type:" + r.ResponseType,
	}
	if r.Email != "" {
		tags = append(tags, "email:"+r.Email)
	}
	if r.UserID != "" {
		tags = append(tags, "user_id:"+r.UserID)
	}
	return tags
}

func mapResponseType(responseType string) string {
	switch strings.ToUpper(responseType) {
	case "ACCEPTED":
		return "accepted"
	case "TENTATIVE":
		return "maybe"
	case "DECLINED":
		return "declined"
	default:
		return strings.ToLower(responseType)
	}
}

func mapResponseScope(scope string) string {
	switch strings.ToLower(scope) {
	case "all":
		return "all"
	case "single":
		return "single"
	case "this_and_following":
		return "this_and_following"
	default:
		return "single" // Default to single
	}
}

func parseTime(timeStr string) (time.Time, error) {
	if timeStr == "" {
		return time.Time{}, fmt.Errorf("empty time string")
	}

	// Try RFC3339 first
	t, err := time.Parse(time.RFC3339, timeStr)
	if err == nil {
		return t, nil
	}

	// Try ISO 8601
	t, err = time.Parse("2006-01-02T15:04:05Z", timeStr)
	if err == nil {
		return t, nil
	}

	// Try with milliseconds
	t, err = time.Parse("2006-01-02T15:04:05.000Z", timeStr)
	if err == nil {
		return t, nil
	}

	// Try space-separated format
	t, err = time.Parse("2006-01-02 15:04:05", timeStr)
	if err == nil {
		return t, nil
	}

	return time.Time{}, fmt.Errorf("unable to parse time: %s", timeStr)
}

func getInt(val interface{}) int {
	switch v := val.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case string:
		var i int
		fmt.Sscanf(v, "%d", &i)
		return i
	default:
		return 0
	}
}

func getBool(val interface{}) bool {
	switch v := val.(type) {
	case bool:
		return v
	case string:
		return v == "true" || v == "1"
	case int:
		return v != 0
	case float64:
		return v != 0
	default:
		return false
	}
}

func getString(val interface{}) string {
	if val == nil {
		return ""
	}
	if s, ok := val.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", val)
}

func getIntFromMap(m map[string]interface{}, key string) int {
	if val, ok := m[key]; ok {
		return getInt(val)
	}
	return 0
}

func getStringFromMap(m map[string]interface{}, key string) string {
	if val, ok := m[key]; ok {
		return getString(val)
	}
	return ""
}

func getStringSliceFromMap(m map[string]interface{}, key string) []string {
	if val, ok := m[key]; ok {
		if slice, ok := val.([]interface{}); ok {
			result := make([]string, 0, len(slice))
			for _, item := range slice {
				result = append(result, getString(item))
			}
			return result
		}
		if slice, ok := val.([]string); ok {
			return slice
		}
	}
	return nil
}

func extractIDFromKey(key, prefix string) string {
	if len(key) > len(prefix) {
		return key[len(prefix):]
	}
	return key
}

func isTransientError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "connection") ||
		strings.Contains(errStr, "unavailable") ||
		strings.Contains(errStr, "temporary") ||
		strings.Contains(errStr, "transient")
}
