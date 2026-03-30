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
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/utils"
	"github.com/nats-io/nats.go/jetstream"
)

// =============================================================================
// Past Meeting Invitee Event Handler
// =============================================================================

// handlePastMeetingInviteeUpdate processes updates to past meeting invitees
func (h *EventHandlers) handlePastMeetingInviteeUpdate(
	ctx context.Context,
	key string,
	v1Data map[string]interface{},
) (retry bool) {
	funcLogger := h.logger.With("key", key, "handler", "past_meeting_invitee")
	funcLogger.DebugContext(ctx, "processing past meeting invitee update")

	// Check if this is a soft delete
	if isDeleted, ok := v1Data["_sdc_deleted_at"].(string); ok && isDeleted != "" {
		return h.handlePastMeetingInviteeDelete(ctx, key, v1Data)
	}

	// Convert v1Data to participant event data
	participantData, err := convertMapToInviteeParticipantData(ctx, v1Data, h.userLookup, h.idMapper, h.v1ObjectsKV, funcLogger)
	if err != nil {
		funcLogger.With(logging.ErrKey, err).ErrorContext(ctx, "failed to convert v1Data to invitee participant")
		return isTransientError(err)
	}

	// Validate required fields
	if participantData.UID == "" || participantData.MeetingAndOccurrenceID == "" {
		funcLogger.ErrorContext(ctx, "missing required fields in invitee participant data")
		return false
	}
	funcLogger = funcLogger.With("participant_uid", participantData.UID)

	// Determine action (created vs updated)
	mappingKey := fmt.Sprintf("v1_past_meeting_invitees.%s", participantData.UID)
	indexerAction := indexerConstants.ActionCreated
	if _, err := h.v1MappingsKV.Get(ctx, mappingKey); err == nil {
		indexerAction = indexerConstants.ActionUpdated
	}

	// Publish to indexer and FGA-sync
	if err := h.publisher.PublishPastMeetingParticipantEvent(ctx, string(indexerAction), participantData); err != nil {
		funcLogger.With(logging.ErrKey, err).ErrorContext(ctx, "failed to publish invitee participant event")
		return isTransientError(err)
	}

	// Store mapping
	if _, err := h.v1MappingsKV.Put(ctx, mappingKey, []byte("1")); err != nil {
		funcLogger.With(logging.ErrKey, err).WarnContext(ctx, "failed to store invitee participant mapping")
	}

	funcLogger.InfoContext(ctx, "successfully processed past meeting invitee")
	return false
}

// handlePastMeetingInviteeDelete processes invitee deletions
func (h *EventHandlers) handlePastMeetingInviteeDelete(ctx context.Context, key string, _ map[string]interface{}) (retry bool) {
	inviteeID := extractIDFromKey(key, "itx-zoom-past-meetings-invitees.")
	mappingKey := fmt.Sprintf("v1_past_meeting_invitees.%s", inviteeID)
	if h.isTombstoned(ctx, mappingKey) {
		h.logger.DebugContext(ctx, "invitee delete already processed, skipping", "invitee_id", inviteeID)
		return false
	}
	return h.handleMeetingTypeDelete(ctx, key, inviteeID, []byte(inviteeID), meetingDeleteConfig{
		indexerSubject:   "lfx.index.v1_past_meeting_participant",
		tombstoneKeyFmts: []string{"v1_past_meeting_invitees.%s"},
	})
}

// =============================================================================
// Past Meeting Attendee Event Handler
// =============================================================================

// handlePastMeetingAttendeeUpdate processes updates to past meeting attendees
func (h *EventHandlers) handlePastMeetingAttendeeUpdate(
	ctx context.Context,
	key string,
	v1Data map[string]interface{},
) (retry bool) {
	funcLogger := h.logger.With("key", key, "handler", "past_meeting_attendee")
	funcLogger.DebugContext(ctx, "processing past meeting attendee update")

	// Check if this is a soft delete
	if isDeleted, ok := v1Data["_sdc_deleted_at"].(string); ok && isDeleted != "" {
		return h.handlePastMeetingAttendeeDelete(ctx, key, v1Data)
	}

	// Convert v1Data to participant event data
	participantData, err := convertMapToAttendeeParticipantData(ctx, v1Data, h.userLookup, h.idMapper, h.v1ObjectsKV, funcLogger)
	if err != nil {
		funcLogger.With(logging.ErrKey, err).ErrorContext(ctx, "failed to convert v1Data to attendee participant")
		return isTransientError(err)
	}

	// Validate required fields
	if participantData.UID == "" || participantData.MeetingAndOccurrenceID == "" {
		funcLogger.ErrorContext(ctx, "missing required fields in attendee participant data")
		return false
	}
	funcLogger = funcLogger.With("participant_uid", participantData.UID)

	// Determine action (created vs updated)
	mappingKey := fmt.Sprintf("v1_past_meeting_attendees.%s", participantData.UID)
	indexerAction := indexerConstants.ActionCreated
	if _, err := h.v1MappingsKV.Get(ctx, mappingKey); err == nil {
		indexerAction = indexerConstants.ActionUpdated
	}

	// Publish to indexer and FGA-sync
	if err := h.publisher.PublishPastMeetingParticipantEvent(ctx, string(indexerAction), participantData); err != nil {
		funcLogger.With(logging.ErrKey, err).ErrorContext(ctx, "failed to publish attendee participant event")
		return isTransientError(err)
	}

	// Store mapping
	if _, err := h.v1MappingsKV.Put(ctx, mappingKey, []byte("1")); err != nil {
		funcLogger.With(logging.ErrKey, err).WarnContext(ctx, "failed to store attendee participant mapping")
	}

	funcLogger.InfoContext(ctx, "successfully processed past meeting attendee")
	return false
}

// handlePastMeetingAttendeeDelete processes attendee deletions
func (h *EventHandlers) handlePastMeetingAttendeeDelete(
	ctx context.Context,
	key string,
	v1Data map[string]interface{},
) (retry bool) {
	attendeeID := extractIDFromKey(key, "itx-zoom-past-meetings-attendees.")
	funcLogger := h.logger.With("key", key, "attendee_id", attendeeID)

	mappingKey := fmt.Sprintf("v1_past_meeting_attendees.%s", attendeeID)
	if h.isTombstoned(ctx, mappingKey) {
		funcLogger.DebugContext(ctx, "attendee delete already processed, skipping")
		return false
	}

	// Extract username (lf_sso) and meeting_and_occurrence_id from v1Data.
	// Only send the access control message if username is present — without it
	// the fga-sync service cannot identify which user to remove access for.
	username := utils.GetString(v1Data["lf_sso"])
	meetingAndOccurrenceID := utils.GetString(v1Data["meeting_and_occurrence_id"])

	var message []byte
	var deleteAllAccessSubject string

	if username != "" {
		accessMsg := map[string]interface{}{
			"meeting_and_occurrence_id": meetingAndOccurrenceID,
			"username":                  username,
			"is_attended":               true,
		}
		var err error
		if message, err = json.Marshal(accessMsg); err != nil {
			funcLogger.With(logging.ErrKey, err).ErrorContext(ctx, "failed to marshal attendee access message")
			return false
		}
		deleteAllAccessSubject = "lfx.remove_participant.v1_past_meeting"
	} else {
		funcLogger.DebugContext(ctx, "no username in v1Data, skipping access control message for attendee delete")
		message = []byte(attendeeID)
	}

	return h.handleMeetingTypeDelete(ctx, key, attendeeID, message, meetingDeleteConfig{
		indexerSubject:         "lfx.index.v1_past_meeting_participant",
		deleteAllAccessSubject: deleteAllAccessSubject,
		tombstoneKeyFmts:       []string{"v1_past_meeting_attendees.%s"},
	})
}

// =============================================================================
// Participant Conversion Functions
// =============================================================================

func convertMapToInviteeParticipantData(
	ctx context.Context,
	v1Data map[string]interface{},
	userLookup domain.V1UserLookup,
	idMapper domain.IDMapper,
	v1ObjectsKV jetstream.KeyValue,
	logger *slog.Logger,
) (*models.PastMeetingParticipantEventData, error) {
	// Convert map to JSON bytes, then to InviteeDBRaw
	jsonBytes, err := json.Marshal(v1Data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal v1Data to JSON: %w", err)
	}

	var rawInvitee InviteeDBRaw
	if err := json.Unmarshal(jsonBytes, &rawInvitee); err != nil {
		return nil, fmt.Errorf("failed to unmarshal invitee data: %w", err)
	}

	// Validate required fields
	if rawInvitee.ID == "" || rawInvitee.MeetingAndOccurrenceID == "" {
		return nil, fmt.Errorf("missing required fields: id or meeting_and_occurrence_id")
	}

	// Use ProjectID from invitee record directly if available
	projectSFID := rawInvitee.ProjectID
	if projectSFID == "" {
		return nil, fmt.Errorf("invitee missing project ID")
	}

	// Map project ID
	projectUID, err := idMapper.MapProjectV1ToV2(ctx, projectSFID)
	if err != nil {
		return nil, fmt.Errorf("failed to map project ID (transient): %w", err)
	}

	// Determine if host (lookup registrant if available)
	isHost := false
	if rawInvitee.RegistrantID != "" {
		registrantKey := fmt.Sprintf("itx-zoom-meetings-registrants-v2.%s", rawInvitee.RegistrantID)
		if registrantEntry, err := v1ObjectsKV.Get(ctx, registrantKey); err == nil {
			var registrantData map[string]interface{}
			if err := json.Unmarshal(registrantEntry.Value(), &registrantData); err == nil {
				isHost = utils.GetBool(registrantData["host"])
			}
		}
	}

	// Username is lf_sso field
	username := rawInvitee.LFSSO

	// Use existing first/last name from invitee record
	firstName := rawInvitee.FirstName
	lastName := rawInvitee.LastName

	// Username resolution via V1UserLookup if lf_user_id exists and we need enrichment
	if rawInvitee.LFUserID != "" && (firstName == "" || lastName == "") {
		v1User, err := userLookup.LookupUser(ctx, rawInvitee.LFUserID)
		if err != nil {
			logger.With(logging.ErrKey, err).WarnContext(ctx, "failed to lookup v1 user", "lf_user_id", rawInvitee.LFUserID)
		} else if v1User != nil {
			if firstName == "" && v1User.FirstName != "" {
				firstName = v1User.FirstName
			}
			if lastName == "" && v1User.LastName != "" {
				lastName = v1User.LastName
			}
		}
	}

	// Parse times
	createdAt, _ := parseTime(rawInvitee.CreatedAt)
	modifiedAt, _ := parseTime(rawInvitee.ModifiedAt)

	// Get org membership flags
	orgIsMember := false
	if rawInvitee.OrgIsMember != nil {
		orgIsMember = *rawInvitee.OrgIsMember
	}
	orgIsProjectMember := false
	if rawInvitee.OrgIsProjectMember != nil {
		orgIsProjectMember = *rawInvitee.OrgIsProjectMember
	}

	return &models.PastMeetingParticipantEventData{
		UID:                    rawInvitee.ID,
		MeetingAndOccurrenceID: rawInvitee.MeetingAndOccurrenceID,
		MeetingID:              rawInvitee.MeetingID,
		ProjectUID:             projectUID,
		Email:                  rawInvitee.Email,
		FirstName:              firstName,
		LastName:               lastName,
		Host:                   isHost,
		JobTitle:               rawInvitee.JobTitle,
		OrgName:                rawInvitee.Org,
		OrgIsMember:            orgIsMember,
		OrgIsProjectMember:     orgIsProjectMember,
		AvatarURL:              rawInvitee.ProfilePicture,
		Username:               username,
		IsInvited:              true,
		IsAttended:             false,
		Sessions:               nil, // Invitees don't have sessions
		CreatedAt:              createdAt,
		ModifiedAt:             modifiedAt,
	}, nil
}

func convertMapToAttendeeParticipantData(
	ctx context.Context,
	v1Data map[string]interface{},
	userLookup domain.V1UserLookup,
	idMapper domain.IDMapper,
	v1ObjectsKV jetstream.KeyValue,
	logger *slog.Logger,
) (*models.PastMeetingParticipantEventData, error) {
	// Convert map to JSON bytes, then to AttendeeDBRaw
	jsonBytes, err := json.Marshal(v1Data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal v1Data to JSON: %w", err)
	}

	var rawAttendee AttendeeDBRaw
	if err := json.Unmarshal(jsonBytes, &rawAttendee); err != nil {
		return nil, fmt.Errorf("failed to unmarshal attendee data: %w", err)
	}

	// Validate required fields
	if rawAttendee.ID == "" || rawAttendee.MeetingAndOccurrenceID == "" {
		return nil, fmt.Errorf("missing required fields: id or meeting_and_occurrence_id")
	}

	// Use ProjectID from attendee record directly
	projectSFID := rawAttendee.ProjectID
	if projectSFID == "" {
		return nil, fmt.Errorf("attendee missing project ID")
	}

	// Map project ID
	projectUID, err := idMapper.MapProjectV1ToV2(ctx, projectSFID)
	if err != nil {
		return nil, fmt.Errorf("failed to map project ID (transient): %w", err)
	}

	// Check if this user was also invited (registrant_id present)
	isInvited := rawAttendee.RegistrantID != ""

	// Parse name
	firstName, lastName := parseName(rawAttendee.Name)

	// Username is lf_sso field
	username := rawAttendee.LFSSO

	// Username resolution via V1UserLookup if lf_user_id exists and we need enrichment
	if rawAttendee.LFUserID != "" && (firstName == "" || lastName == "") {
		v1User, err := userLookup.LookupUser(ctx, rawAttendee.LFUserID)
		if err != nil {
			logger.With(logging.ErrKey, err).WarnContext(ctx, "failed to lookup v1 user", "lf_user_id", rawAttendee.LFUserID)
		} else if v1User != nil {
			if firstName == "" && v1User.FirstName != "" {
				firstName = v1User.FirstName
			}
			if lastName == "" && v1User.LastName != "" {
				lastName = v1User.LastName
			}
		}
	}

	// Convert sessions
	var sessions []models.ParticipantSession
	for _, rawSession := range rawAttendee.Sessions {
		joinTime, _ := parseTime(rawSession.JoinTime)
		leaveTime, _ := parseTime(rawSession.LeaveTime)
		sessions = append(sessions, models.ParticipantSession{
			UID:         rawSession.ParticipantUUID,
			JoinTime:    &joinTime,
			LeaveTime:   &leaveTime,
			LeaveReason: rawSession.LeaveReason,
		})
	}

	// Parse times
	createdAt, _ := parseTime(rawAttendee.CreatedAt)
	modifiedAt, _ := parseTime(rawAttendee.ModifiedAt)

	// Get org membership flags
	orgIsMember := false
	if rawAttendee.OrgIsMember != nil {
		orgIsMember = *rawAttendee.OrgIsMember
	}
	orgIsProjectMember := false
	if rawAttendee.OrgIsProjectMember != nil {
		orgIsProjectMember = *rawAttendee.OrgIsProjectMember
	}

	return &models.PastMeetingParticipantEventData{
		UID:                    rawAttendee.ID,
		MeetingAndOccurrenceID: rawAttendee.MeetingAndOccurrenceID,
		MeetingID:              rawAttendee.MeetingID,
		ProjectUID:             projectUID,
		Email:                  rawAttendee.Email,
		FirstName:              firstName,
		LastName:               lastName,
		Host:                   false, // Attendee records don't have host info
		JobTitle:               rawAttendee.JobTitle,
		OrgName:                rawAttendee.Org,
		OrgIsMember:            orgIsMember,
		OrgIsProjectMember:     orgIsProjectMember,
		AvatarURL:              rawAttendee.ProfilePicture,
		Username:               username,
		IsInvited:              isInvited,
		IsAttended:             true,
		Sessions:               sessions,
		CreatedAt:              createdAt,
		ModifiedAt:             modifiedAt,
	}, nil
}
