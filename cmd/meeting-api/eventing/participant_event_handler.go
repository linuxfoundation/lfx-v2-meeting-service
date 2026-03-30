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

// InviteeDBRaw represents raw past meeting invitee data from v1 DynamoDB/NATS KV bucket
type InviteeDBRaw struct {
	// InviteeID is the partition key of the invitee table
	InviteeID string `json:"invitee_id"`

	// FirstName is the first name of the invitee
	FirstName string `json:"first_name"`

	// LastName is the last name of the invitee
	LastName string `json:"last_name"`

	// Email is the email of the invitee
	Email string `json:"email"`

	// ProfilePicture is the profile picture of the invitee
	ProfilePicture string `json:"profile_picture"`

	// LFSSO is the LF username of the invitee
	LFSSO string `json:"lf_sso"`

	// LFUserID is the ID of the invitee
	LFUserID string `json:"lf_user_id,omitempty"`

	// CommitteeID is the ID of the committee associated with the invitee
	CommitteeID string `json:"committee_id"`

	// CommitteeRole is the role of the invitee in the committee
	CommitteeRole string `json:"committee_role"`

	// CommitteeVotingStatus is the voting status of the invitee in the committee
	CommitteeVotingStatus string `json:"committee_voting_status"`

	// Org is the organization of the invitee
	Org string `json:"org"`

	// OrgIsMember is whether the [Org] field is an organization that is a member of the Linux Foundation
	OrgIsMember *bool `json:"org_is_member,omitempty"`

	// OrgIsProjectMember is whether the [Org] field is an organization that is a member of the project associated with the meeting
	OrgIsProjectMember *bool `json:"org_is_project_member,omitempty"`

	// JobTitle is the job title of the invitee
	JobTitle string `json:"job_title"`

	// RegistrantID is the ID of the registrant record associated with the invitee
	RegistrantID string `json:"registrant_id"`

	// ProjectID is the ID of the project associated with the invitee
	ProjectID string `json:"proj_id,omitempty"`

	// MeetingAndOccurrenceID is the ID of the meeting and occurrence associated with the invitee
	MeetingAndOccurrenceID string `json:"meeting_and_occurrence_id,omitempty"` // secondary index

	// MeetingID is the ID of the meeting associated with the invitee
	MeetingID string `json:"meeting_id,omitempty"`

	// OccurrenceID is the ID of the occurrence associated with the invitee
	OccurrenceID string `json:"occurrence_id"`

	// CreatedAt is the creation time of the invitee
	CreatedAt string `json:"created_at"`

	// ModifiedAt is the last modification time of the invitee
	ModifiedAt string `json:"modified_at"`

	// CreatedBy is the user who created the invitee
	CreatedBy models.CreatedBy `json:"created_by"`

	// UpdatedBy is the user who last updated the invitee
	UpdatedBy models.UpdatedBy `json:"updated_by"`
}

// UnmarshalJSON implements custom unmarshaling for InviteeDBRaw.
func (i *InviteeDBRaw) UnmarshalJSON(data []byte) error {
	type Alias InviteeDBRaw
	tmp := struct{ *Alias }{Alias: (*Alias)(i)}
	return json.Unmarshal(data, &tmp)
}

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

// AttendeeDBRaw represents raw past meeting attendee data from v1 DynamoDB/NATS KV bucket
type AttendeeDBRaw struct {
	// ID is the partition key of the attendee table
	// This is from the v1 system
	ID string `json:"id"`

	// ProjectID is the ID of the project associated with the attendee
	ProjectID string `json:"proj_id"`

	// ProjectSlug is the slug of the project associated with the attendee
	ProjectSlug string `json:"project_slug"`

	// RegistrantID is the ID of the registrant associated with the attendee.
	// This is only populated for attendees who are registrants for the meeting.
	RegistrantID string `json:"registrant_id"`

	// Email is the email of the attendee.
	// This may be empty if the attendee is not a known LF user because Zoom does not provide the email
	// of users when they join a meeting.
	Email string `json:"email"`

	// Name is the full name of the attendee.
	// If the user is not a known LF user, then the name is just the Zoom display name of the participant.
	// Otherwise, the name comes from the LF user record.
	Name string `json:"name"`

	// ZoomUserName is the Zoom display name of the attendee.
	ZoomUserName string `json:"zoom_user_name"`

	// MappedInviteeName is the full name of the invitee that the attendee was matched to.
	// This is only populated if the attendee was auto-matched to an invitee.
	MappedInviteeName string `json:"mapped_invitee_name"`

	// LFSSO is the LF username of the attendee
	LFSSO string `json:"lf_sso"`

	// LFUserID is the ID of the attendee
	LFUserID string `json:"lf_user_id"`

	// IsVerified is whether or not the attendee is a verified user
	IsVerified bool `json:"is_verified"`

	// IsUnknown is whether or not the attendee has been marked as unknown attendee
	IsUnknown bool `json:"is_unknown"`

	// Org is the organization of the attendee
	Org string `json:"org"`

	// OrgIsMember is whether the [Org] field is an organization that is a member of the Linux Foundation
	OrgIsMember *bool `json:"org_is_member,omitempty"`

	// OrgIsProjectMember is whether the [Org] field is an organization that is a member of the project associated with the meeting
	OrgIsProjectMember *bool `json:"org_is_project_member,omitempty"`

	// JobTitle is the job title of the attendee
	JobTitle string `json:"job_title"`

	// CommitteeID is the ID of the committee associated with the attendee
	CommitteeID string `json:"committee_id"`

	// IsCommitteeMember is only relevant if the past meeting is associated with a committee.
	// It is true if the attendee is a member of that committee.
	IsCommitteeMember bool `json:"is_committee_member"`

	// CommitteeRole is only relevant if the past meeting is associated with a committee.
	// It is the role of the attendee in the committee.
	CommitteeRole string `json:"committee_role"`

	// CommitteeVotingStatus is only relevant if the past meeting is associated with a committee.
	// It is the voting status of the attendee in the committee.
	CommitteeVotingStatus string `json:"committee_voting_status"`

	// ProfilePicture is the profile picture of the attendee
	ProfilePicture string `json:"profile_picture"`

	// MeetingID is the ID of the meeting associated with the attendee
	MeetingID string `json:"meeting_id"`

	// OccurrenceID is the ID of the occurrence associated with the attendee
	OccurrenceID string `json:"occurrence_id"`

	// MeetingAndOccurrenceID is the ID of the combined meeting and occurrence associated with the attendee
	MeetingAndOccurrenceID string `json:"meeting_and_occurrence_id"`

	// AverageAttendance is the average attendance of the attendee as a percentage.
	// This is the average of the [Sessions] field.
	AverageAttendance int `json:"-"`

	// Sessions is the list of sessions associated with the attendee
	Sessions []AttendeeSessionDBRaw `json:"sessions"`

	// CreatedAt is the creation time of the attendee
	CreatedAt string `json:"created_at"`

	// ModifiedAt is the last modification time of the attendee
	ModifiedAt string `json:"modified_at"`

	// CreatedBy is the user who created the attendee
	CreatedBy models.CreatedBy `json:"created_by"`

	// UpdatedBy is the user who last updated the attendee
	UpdatedBy models.UpdatedBy `json:"updated_by"`

	// IsAutoMatched is true if the attendee name was auto-matched to a registrant's email
	IsAutoMatched bool `json:"is_auto_matched,omitempty"`
}

// UnmarshalJSON implements custom unmarshaling for AttendeeDBRaw.
func (a *AttendeeDBRaw) UnmarshalJSON(data []byte) error {
	type Alias AttendeeDBRaw
	tmp := struct{ *Alias }{Alias: (*Alias)(a)}
	return json.Unmarshal(data, &tmp)
}

// AttendeeSessionDBRaw represents raw attendee session data from v1 DynamoDB/NATS KV bucket
type AttendeeSessionDBRaw struct {
	ParticipantUUID string `json:"participant_uuid"`
	JoinTime        string `json:"join_time"`
	LeaveTime       string `json:"leave_time"`
	LeaveReason     string `json:"leave_reason"`
}

// UnmarshalJSON implements custom unmarshaling for AttendeeSessionDBRaw.
func (a *AttendeeSessionDBRaw) UnmarshalJSON(data []byte) error {
	type Alias AttendeeSessionDBRaw
	tmp := struct{ *Alias }{Alias: (*Alias)(a)}
	return json.Unmarshal(data, &tmp)
}

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
	if rawInvitee.InviteeID == "" || rawInvitee.MeetingAndOccurrenceID == "" {
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
		UID:                    rawInvitee.InviteeID,
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
