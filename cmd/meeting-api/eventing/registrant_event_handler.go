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
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/utils"
	"github.com/nats-io/nats.go/jetstream"
)

// =============================================================================
// Registrant Event Handler
// =============================================================================

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

	projectSFID := utils.GetString(meetingData["proj_id"])
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
			logger.With(logging.ErrKey, err).WarnContext(ctx, "failed to map committee ID", "v1_id", rawRegistrant.CommitteeID)
			// Don't fail - just omit committee
		}
	}

	// Username resolution via V1UserLookup if username blank but user_id exists
	username := rawRegistrant.Username
	if username == "" && rawRegistrant.UserID != "" {
		v1User, err := userLookup.LookupUser(ctx, rawRegistrant.UserID)
		if err != nil {
			logger.With(logging.ErrKey, err).WarnContext(ctx, "failed to lookup v1 user", "user_id", rawRegistrant.UserID)
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

	// Parse times — propagate errors only for non-empty but malformed strings;
	// absent timestamps (empty string) remain as zero-value time.Time.
	var createdAt, modifiedAt time.Time
	if rawRegistrant.CreatedAt != "" {
		if createdAt, err = parseTime(rawRegistrant.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to parse created_at: %w", err)
		}
	}
	if rawRegistrant.ModifiedAt != "" {
		if modifiedAt, err = parseTime(rawRegistrant.ModifiedAt); err != nil {
			return nil, fmt.Errorf("failed to parse modified_at: %w", err)
		}
	}

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
		Host:         utils.GetBool(rawRegistrant.Host),
		CreatedAt:    createdAt,
		ModifiedAt:   modifiedAt,
	}, nil
}

// handleRegistrantUpdate processes updates to meeting registrants
func (h *EventHandlers) handleRegistrantUpdate(
	ctx context.Context,
	key string,
	v1Data map[string]interface{},
) (retry bool) {
	funcLogger := h.logger.With("key", key, "handler", "registrant")
	funcLogger.DebugContext(ctx, "processing registrant update")

	// Check if this is a soft delete
	if isDeleted, ok := v1Data["_sdc_deleted_at"].(string); ok && isDeleted != "" {
		return h.handleRegistrantDelete(ctx, key, v1Data)
	}

	// Convert v1Data to registrant event data
	registrantData, err := convertMapToRegistrantData(ctx, v1Data, h.userLookup, h.idMapper, h.v1ObjectsKV, funcLogger)
	if err != nil {
		funcLogger.With(logging.ErrKey, err).ErrorContext(ctx, "failed to convert v1Data to registrant")
		return isTransientError(err)
	}

	// Validate required fields
	if registrantData.UID == "" || registrantData.MeetingID == "" {
		funcLogger.ErrorContext(ctx, "missing required fields in registrant data")
		return false
	}
	funcLogger = funcLogger.With("registrant_uid", registrantData.UID)

	// Determine action (created vs updated)
	mappingKey := fmt.Sprintf("v1_registrants.%s", registrantData.UID)
	indexerAction := indexerConstants.ActionCreated
	if _, err := h.v1MappingsKV.Get(ctx, mappingKey); err == nil {
		indexerAction = indexerConstants.ActionUpdated
	}

	// Publish to indexer and FGA-sync
	if err := h.publisher.PublishRegistrantEvent(ctx, string(indexerAction), registrantData); err != nil {
		funcLogger.With(logging.ErrKey, err).ErrorContext(ctx, "failed to publish registrant event")
		return isTransientError(err)
	}

	// Store mapping
	if _, err := h.v1MappingsKV.Put(ctx, mappingKey, []byte("1")); err != nil {
		funcLogger.With(logging.ErrKey, err).WarnContext(ctx, "failed to store registrant mapping")
	}

	funcLogger.InfoContext(ctx, "successfully processed registrant")
	return false
}

// handleRegistrantDelete processes registrant deletions
func (h *EventHandlers) handleRegistrantDelete(ctx context.Context, key string, v1Data map[string]interface{}) (retry bool) {
	registrantUID := extractIDFromKey(key, "itx-zoom-meetings-registrants-v2.")
	funcLogger := h.logger.With("key", key, "registrant_uid", registrantUID)

	mappingKey := fmt.Sprintf("v1_registrants.%s", registrantUID)
	if h.isTombstoned(ctx, mappingKey) {
		funcLogger.DebugContext(ctx, "registrant delete already processed, skipping")
		return false
	}

	// Extract username and host to conditionally send the access control message.
	// Without a username, access control cannot identify which user to remove access for.
	username := utils.GetString(v1Data["username"])

	var message []byte
	var deleteAllAccessSubject string

	if username != "" {
		accessMsg := map[string]interface{}{
			"id":         registrantUID,
			"meeting_id": utils.GetString(v1Data["meeting_id"]),
			"username":   username,
			"host":       utils.GetBool(v1Data["host"]),
		}
		var err error
		if message, err = json.Marshal(accessMsg); err != nil {
			funcLogger.With(logging.ErrKey, err).ErrorContext(ctx, "failed to marshal registrant access message")
			return false
		}
		deleteAllAccessSubject = "lfx.remove_registrant.v1_meeting"
	} else {
		funcLogger.DebugContext(ctx, "no username in v1Data, skipping access control message for registrant delete")
		message = []byte(registrantUID)
	}

	return h.handleMeetingTypeDelete(ctx, key, registrantUID, message, meetingDeleteConfig{
		indexerSubject:         "lfx.index.v1_meeting_registrant",
		deleteAllAccessSubject: deleteAllAccessSubject,
		tombstoneKeyFmts:       []string{"v1_registrants.%s"},
	})
}

// =============================================================================
// Invite Response (RSVP) Event Handler
// =============================================================================

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

	projectSFID := utils.GetString(meetingData["proj_id"])
	if projectSFID == "" {
		return nil, fmt.Errorf("meeting missing project ID")
	}

	// Map project ID
	projectUID, err := idMapper.MapProjectV1ToV2(ctx, projectSFID)
	if err != nil {
		return nil, fmt.Errorf("failed to map project ID (transient): %w", err)
	}

	responseType, err := mapResponseType(rawResponse.Response)
	if err != nil {
		return nil, err
	}

	// Determine if response is for recurring meeting
	isRecurring := rawResponse.OccurrenceID == "" || rawResponse.Scope == "all" || rawResponse.Scope == "this_and_following"

	// Parse times — propagate errors only for non-empty but malformed strings;
	// absent timestamps (empty string) remain as zero-value time.Time.
	var createdAt, modifiedAt time.Time
	if rawResponse.CreatedAt != "" {
		if createdAt, err = parseTime(rawResponse.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to parse created_at: %w", err)
		}
	}
	if rawResponse.ModifiedAt != "" {
		if modifiedAt, err = parseTime(rawResponse.ModifiedAt); err != nil {
			return nil, fmt.Errorf("failed to parse modified_at: %w", err)
		}
	}

	return &models.InviteResponseEventData{
		ID:                     rawResponse.ID,
		MeetingAndOccurrenceID: rawResponse.MeetingAndOccurrenceID,
		MeetingID:              rawResponse.MeetingID,
		OccurrenceID:           rawResponse.OccurrenceID,
		RegistrantID:           rawResponse.RegistrantID,
		ProjectUID:             projectUID,
		UserID:                 rawResponse.UserID,
		Username:               rawResponse.Username,
		Email:                  rawResponse.Email,
		ResponseType:           responseType,
		Scope:                  rawResponse.Scope,
		IsRecurring:            isRecurring,
		CreatedAt:              createdAt,
		ModifiedAt:             modifiedAt,
	}, nil
}

// handleInviteResponseUpdate processes updates to meeting invite responses (RSVPs)
func (h *EventHandlers) handleInviteResponseUpdate(
	ctx context.Context,
	key string,
	v1Data map[string]interface{},
) (retry bool) {
	funcLogger := h.logger.With("key", key, "handler", "invite_response")
	funcLogger.DebugContext(ctx, "processing invite response update")

	// Check if this is a soft delete
	if isDeleted, ok := v1Data["_sdc_deleted_at"].(string); ok && isDeleted != "" {
		return h.handleInviteResponseDelete(ctx, key, v1Data)
	}

	// Convert v1Data to invite response event data
	responseData, err := convertMapToInviteResponseData(ctx, v1Data, h.idMapper, h.v1ObjectsKV, funcLogger)
	if err != nil {
		funcLogger.With(logging.ErrKey, err).ErrorContext(ctx, "failed to convert v1Data to invite response")
		return isTransientError(err)
	}

	// Validate required fields
	if responseData.ID == "" || responseData.MeetingID == "" {
		funcLogger.ErrorContext(ctx, "missing required fields in invite response data")
		return false
	}
	funcLogger = funcLogger.With("response_id", responseData.ID)

	// Determine action (created vs updated)
	mappingKey := fmt.Sprintf("v1_invite_responses.%s", responseData.ID)
	indexerAction := indexerConstants.ActionCreated
	if _, err := h.v1MappingsKV.Get(ctx, mappingKey); err == nil {
		indexerAction = indexerConstants.ActionUpdated
	}

	// Publish to indexer
	if err := h.publisher.PublishInviteResponseEvent(ctx, string(indexerAction), responseData); err != nil {
		funcLogger.With(logging.ErrKey, err).ErrorContext(ctx, "failed to publish invite response event")
		return isTransientError(err)
	}

	// Store mapping
	if _, err := h.v1MappingsKV.Put(ctx, mappingKey, []byte("1")); err != nil {
		funcLogger.With(logging.ErrKey, err).WarnContext(ctx, "failed to store invite response mapping")
	}

	funcLogger.InfoContext(ctx, "successfully processed invite response")
	return false
}

// handleInviteResponseDelete processes invite response deletions
func (h *EventHandlers) handleInviteResponseDelete(ctx context.Context, key string, _ map[string]interface{}) (retry bool) {
	responseID := extractIDFromKey(key, "itx-zoom-meetings-invite-responses-v2.")
	mappingKey := fmt.Sprintf("v1_invite_responses.%s", responseID)
	if h.isTombstoned(ctx, mappingKey) {
		h.logger.DebugContext(ctx, "invite response delete already processed, skipping", "response_id", responseID)
		return false
	}
	return h.handleMeetingTypeDelete(ctx, key, responseID, []byte(responseID), meetingDeleteConfig{
		indexerSubject:   "lfx.index.v1_meeting_rsvp",
		tombstoneKeyFmts: []string{"v1_invite_responses.%s"},
	})
}
