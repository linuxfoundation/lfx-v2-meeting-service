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
	ID                              string           `json:"id"`
	MeetingID                       string           `json:"meeting_id"`
	Type                            string           `json:"type"`
	CommitteeID                     string           `json:"committee_id"`
	UserID                          string           `json:"user_id"`
	Email                           string           `json:"email"`
	CaseInsensitiveEmail            string           `json:"case_insensitive_email"`
	FirstName                       string           `json:"first_name"`
	LastName                        string           `json:"last_name"`
	OrgName                         string           `json:"org_name,omitempty"`
	OrgIsMember                     *bool            `json:"org_is_member,omitempty"`
	OrgIsProjectMember              *bool            `json:"org_is_project_member,omitempty"`
	JobTitle                        string           `json:"job_title,omitempty"`
	Host                            *bool            `json:"host"`
	Occurrence                      string           `json:"occurrence,omitempty"`
	AvatarURL                       string           `json:"avatar_url"`
	Username                        string           `json:"username,omitempty"`
	LastInviteReceivedTime          string           `json:"last_invite_received_time"`
	LastInviteReceivedMessageID     *string          `json:"last_invite_received_message_id,omitempty"`
	LastInviteDeliverySuccessful    *bool            `json:"last_invite_delivery_successful,omitempty"`
	LastInviteDeliveredTime         string           `json:"last_invite_delivered_time,omitempty"`
	LastInviteBounced               *bool            `json:"last_invite_bounced,omitempty"`
	LastInviteBouncedTime           string           `json:"last_invite_bounced_time,omitempty"`
	LastInviteBouncedType           string           `json:"last_invite_bounced_type,omitempty"`
	LastInviteBouncedSubType        string           `json:"last_invite_bounced_sub_type,omitempty"`
	LastInviteBouncedDiagnosticCode string           `json:"last_invite_bounced_diagnostic_code,omitempty"`
	CreatedAt                       string           `json:"created_at"`
	UpdatedAt                       string           `json:"updated_at"`
	CreatedBy                       models.CreatedBy `json:"created_by"`
	UpdatedBy                       models.UpdatedBy `json:"updated_by"`
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

	return &models.RegistrantEventData{
		UID:                             rawRegistrant.ID,
		MeetingID:                       rawRegistrant.MeetingID,
		Type:                            rawRegistrant.Type,
		CommitteeUID:                    committeeUID,
		UserID:                          rawRegistrant.UserID,
		Email:                           rawRegistrant.Email,
		CaseInsensitiveEmail:            rawRegistrant.CaseInsensitiveEmail,
		FirstName:                       rawRegistrant.FirstName,
		LastName:                        rawRegistrant.LastName,
		OrgName:                         rawRegistrant.OrgName,
		OrgIsMember:                     rawRegistrant.OrgIsMember,
		OrgIsProjectMember:              rawRegistrant.OrgIsProjectMember,
		JobTitle:                        rawRegistrant.JobTitle,
		Host:                            utils.GetBool(rawRegistrant.Host),
		Occurrence:                      rawRegistrant.Occurrence,
		AvatarURL:                       rawRegistrant.AvatarURL,
		Username:                        username,
		LastInviteReceivedTime:          rawRegistrant.LastInviteReceivedTime,
		LastInviteReceivedMessageID:     rawRegistrant.LastInviteReceivedMessageID,
		LastInviteDeliverySuccessful:    rawRegistrant.LastInviteDeliverySuccessful,
		LastInviteDeliveredTime:         rawRegistrant.LastInviteDeliveredTime,
		LastInviteBounced:               rawRegistrant.LastInviteBounced,
		LastInviteBouncedTime:           rawRegistrant.LastInviteBouncedTime,
		LastInviteBouncedType:           rawRegistrant.LastInviteBouncedType,
		LastInviteBouncedSubType:        rawRegistrant.LastInviteBouncedSubType,
		LastInviteBouncedDiagnosticCode: rawRegistrant.LastInviteBouncedDiagnosticCode,
		CreatedAt:                       rawRegistrant.CreatedAt,
		UpdatedAt:                       rawRegistrant.UpdatedAt,
		CreatedBy:                       models.CreatedBy(rawRegistrant.CreatedBy),
		UpdatedBy:                       models.UpdatedBy(rawRegistrant.UpdatedBy),
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

	// Parent validation - meeting must exist
	// This pre-requisite ensures that the meeting is not a meeting that is filtered out and won't be added
	// to the v1-mappings KV bucket after this event is processed.
	meetingMappingKey := fmt.Sprintf("v1_meetings.%s", registrantData.MeetingID)
	_, err = h.v1MappingsKV.Get(ctx, meetingMappingKey)
	if err != nil {
		funcLogger.With(logging.ErrKey, err).InfoContext(ctx, "parent meeting not found in mappings, will retry meeting registrant sync")
		return true
	}

	// Determine action (created vs updated)
	mappingKey := fmt.Sprintf("v1_meeting_registrants.%s", registrantData.UID)
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

	mappingKey := fmt.Sprintf("v1_meeting_registrants.%s", registrantUID)
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
		// The fga-sync service expects the username in the Auth0 "sub" format.
		auth0Username := h.userLookup.MapUsernameToAuthSub(username)
		accessMsg := map[string]interface{}{
			"id":         registrantUID,
			"meeting_id": utils.GetString(v1Data["meeting_id"]),
			"username":   auth0Username,
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
		tombstoneKeyFmts:       []string{"v1_meeting_registrants.%s"},
	})
}

// =============================================================================
// Invite Response (RSVP) Event Handler
// =============================================================================

// InviteResponseDBRaw represents raw invite response data from v1 DynamoDB/NATS KV bucket
type InviteResponseDBRaw struct {
	ID                     string `json:"id"`
	MeetingAndOccurrenceID string `json:"meeting_and_occurrence_id"`
	MeetingID              string `json:"meeting_id"`
	OccurrenceID           string `json:"occurrence_id"`
	RegistrantID           string `json:"registrant_id"`
	Email                  string `json:"email"`
	Name                   string `json:"name"`
	UserID                 string `json:"user_id"`
	Username               string `json:"username"`
	Org                    string `json:"org"`
	JobTitle               string `json:"job_title"`
	Response               string `json:"response"`
	Scope                  string `json:"scope"`
	ResponseDate           string `json:"response_date"`
	SESMessageID           string `json:"ses_message_id"`
	EmailSubject           string `json:"email_subject"`
	EmailText              string `json:"email_text"`
	CreatedAt              string `json:"created_at"`
	ModifiedAt             string `json:"modified_at"`
}

// convertMapToInviteResponseData converts v1 invite response data to v2 format
func convertMapToInviteResponseData(
	ctx context.Context,
	v1Data map[string]interface{},
	userLookup domain.V1UserLookup,
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

	// If username is blank but we have a v1 Platform ID (user_id), lookup the username.
	username := rawResponse.Username
	if username == "" && rawResponse.UserID != "" {
		if v1User, lookupErr := userLookup.LookupUser(ctx, rawResponse.UserID); lookupErr == nil && v1User != nil && v1User.Username != "" {
			username = v1User.Username
			logger.With("user_id", rawResponse.UserID, "username", v1User.Username).DebugContext(ctx, "looked up username for invite response")
		} else if lookupErr != nil {
			logger.With(logging.ErrKey, lookupErr, "user_id", rawResponse.UserID).WarnContext(ctx, "failed to lookup v1 user for invite response")
		}
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

	responseType, err := mapInviteResponseType(rawResponse.Response)
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
		Username:               username,
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
	responseData, err := convertMapToInviteResponseData(ctx, v1Data, h.userLookup, h.idMapper, h.v1ObjectsKV, funcLogger)
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

// mapInviteResponseType maps v1 invite response type to v2 invite response type
func mapInviteResponseType(inviteResponseType string) (string, error) {
	switch strings.ToUpper(inviteResponseType) {
	case "ACCEPTED":
		return "accepted", nil
	case "TENTATIVE":
		return "maybe", nil
	case "DECLINED":
		return "declined", nil
	}
	return "", fmt.Errorf("invalid invite response type: %s", inviteResponseType)
}
