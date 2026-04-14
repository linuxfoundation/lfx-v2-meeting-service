// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"encoding/json"
	"errors"
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

	// ProjectSlug is the slug of the project associated with the invitee
	ProjectSlug string `json:"project_slug,omitempty"`

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
	if participantData.ProjectUID == "" {
		funcLogger.InfoContext(ctx, "skipping invitee participant sync - parent project not found in mappings")
		return false
	}
	funcLogger = funcLogger.With("participant_uid", participantData.UID)

	// If an attendee cross-reference exists for this participant, preserve is_attended=true
	// so a late-arriving invitee upsert doesn't reset a flag the attendee handler already set.
	if participantData.Username != "" {
		attendeeXrefKey := fmt.Sprintf("v1_participant_by_meeting_user.attendee.%s.%s",
			participantData.MeetingAndOccurrenceID, participantData.Username)
		if entry, err := h.v1MappingsKV.Get(ctx, attendeeXrefKey); err == nil && !entryIsTombstoned(entry) {
			participantData.IsAttended = true
		}
	}

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

	// Store invitee mapping and cross-reference (keyed by meeting+username so the attendee
	// delete handler can determine whether an invitee record still exists).
	if _, err := h.v1MappingsKV.Put(ctx, mappingKey, []byte("1")); err != nil {
		funcLogger.With(logging.ErrKey, err).WarnContext(ctx, "failed to store invitee participant mapping")
	}
	if participantData.Username != "" {
		xrefKey := fmt.Sprintf("v1_participant_by_meeting_user.invitee.%s.%s",
			participantData.MeetingAndOccurrenceID, participantData.Username)
		if _, err := h.v1MappingsKV.Put(ctx, xrefKey, []byte(participantData.UID)); err != nil {
			funcLogger.With(logging.ErrKey, err).WarnContext(ctx, "failed to store invitee cross-reference mapping")
		}
	}

	funcLogger.InfoContext(ctx, "successfully processed past meeting invitee")
	return false
}

// handlePastMeetingInviteeDelete processes invitee deletions.
// If an attendee record still exists for the same participant, a partial delete is applied:
// the indexer record is updated with is_invited=false and FGA is updated via member_put rather
// than member_remove, so the participant retains access from their attendee record.
func (h *EventHandlers) handlePastMeetingInviteeDelete(ctx context.Context, key string, v1Data map[string]interface{}) (retry bool) {
	inviteeID := extractIDFromKey(key, "itx-zoom-past-meetings-invitees.")
	funcLogger := h.logger.With("key", key, "invitee_id", inviteeID)

	mappingKey := fmt.Sprintf("v1_past_meeting_invitees.%s", inviteeID)
	if h.isTombstoned(ctx, mappingKey) {
		funcLogger.DebugContext(ctx, "invitee delete already processed, skipping")
		return false
	}

	if v1Data == nil {
		funcLogger.WarnContext(ctx, "no v1Data available for invitee delete, skipping")
		return false
	}

	username := utils.GetString(v1Data["lf_sso"])
	meetingAndOccurrenceID := utils.GetString(v1Data["meeting_and_occurrence_id"])

	// Check if an attendee record still exists for this participant.
	if username != "" && meetingAndOccurrenceID != "" {
		attendeeXrefKey := fmt.Sprintf("v1_participant_by_meeting_user.attendee.%s.%s", meetingAndOccurrenceID, username)
		if entry, err := h.v1MappingsKV.Get(ctx, attendeeXrefKey); err == nil && !entryIsTombstoned(entry) {
			survivingAttendeeID := string(entry.Value())
			funcLogger.DebugContext(ctx, "participant has active attendee record; applying partial invitee delete",
				"surviving_attendee_id", survivingAttendeeID)
			return h.handlePartialInviteeDelete(ctx, funcLogger, key, inviteeID, survivingAttendeeID, meetingAndOccurrenceID, username)
		}
	}

	// Full delete — no attendee record survives.
	return h.fullDeleteInvitee(ctx, funcLogger, key, inviteeID, meetingAndOccurrenceID, username)
}

// fullDeleteInvitee performs a full indexer delete and FGA member_remove for an invitee
// when no sibling attendee record survives. Called from both the normal delete path and
// as a fallback when the sibling is found to be missing.
func (h *EventHandlers) fullDeleteInvitee(
	ctx context.Context,
	funcLogger *slog.Logger,
	key, inviteeID, meetingAndOccurrenceID, username string,
) (retry bool) {
	var accessPayload []byte
	var deleteAccessSubject string
	if username != "" {
		auth0Username, err := h.userLookup.MapUsernameToAuthSub(ctx, username)
		if err != nil {
			funcLogger.With(logging.ErrKey, err).ErrorContext(ctx, "failed to resolve auth sub for invitee delete")
			return true
		}
		if accessPayload, err = buildGenericMemberRemovePayload("v1_past_meeting", meetingAndOccurrenceID, auth0Username); err != nil {
			funcLogger.With(logging.ErrKey, err).ErrorContext(ctx, "failed to build member remove payload")
			return false
		}
		deleteAccessSubject = "lfx.fga-sync.member_remove"
	}

	result := h.handleMeetingTypeDelete(ctx, key, inviteeID, accessPayload, meetingDeleteConfig{
		indexerSubject:      "lfx.index.v1_past_meeting_participant",
		deleteAccessSubject: deleteAccessSubject,
		tombstoneKeyFmts:    []string{"v1_past_meeting_invitees.%s"},
	})
	if !result && username != "" && meetingAndOccurrenceID != "" {
		h.tombstoneMapping(ctx, fmt.Sprintf("v1_participant_by_meeting_user.invitee.%s.%s", meetingAndOccurrenceID, username))
	}
	return result
}

// handlePartialInviteeDelete is called when an invitee record is deleted but an attendee record
// still exists. It sends an indexer UPDATE with is_invited=false and a member_put to update FGA
// relations, so the participant retains access from their attendee record.
func (h *EventHandlers) handlePartialInviteeDelete(
	ctx context.Context,
	funcLogger *slog.Logger,
	key, inviteeID, survivingAttendeeID, meetingAndOccurrenceID, username string,
) (retry bool) {
	// Fetch the surviving attendee data to build an accurate participant record.
	attendeeEntry, err := h.v1ObjectsKV.Get(ctx, fmt.Sprintf("itx-zoom-past-meetings-attendees.%s", survivingAttendeeID))
	if err != nil {
		if !errors.Is(err, jetstream.ErrKeyNotFound) {
			funcLogger.With(logging.ErrKey, err).ErrorContext(ctx, "transient error fetching attendee data for partial invitee delete")
			return true
		}
		// Sibling attendee is gone — fall back to a full invitee delete.
		funcLogger.WarnContext(ctx, "surviving attendee not found during partial invitee delete; falling back to full delete",
			"surviving_attendee_id", survivingAttendeeID)
		return h.fullDeleteInvitee(ctx, funcLogger, key, inviteeID, meetingAndOccurrenceID, username)
	}
	attendeeData, err := decodeData(attendeeEntry.Value())
	if err != nil {
		funcLogger.With(logging.ErrKey, err).ErrorContext(ctx, "failed to decode attendee data for partial invitee delete")
		return false
	}

	participantData, err := convertMapToAttendeeParticipantData(ctx, attendeeData, h.userLookup, h.idMapper, h.v1ObjectsKV, funcLogger)
	if err != nil {
		funcLogger.With(logging.ErrKey, err).ErrorContext(ctx, "failed to convert attendee data for partial invitee delete")
		return isTransientError(err)
	}
	// The invitee record is gone; the attendee record remains.
	participantData.IsInvited = false
	participantData.IsAttended = true

	if err := h.publisher.PublishIndexerDelete(ctx, "lfx.index.v1_past_meeting_participant", inviteeID); err != nil {
		funcLogger.With(logging.ErrKey, err).ErrorContext(ctx, "failed to send indexer delete for partial invitee delete")
		return isTransientError(err)
	}

	if err := h.publisher.PublishPastMeetingParticipantEvent(ctx, string(indexerConstants.ActionUpdated), participantData); err != nil {
		funcLogger.With(logging.ErrKey, err).ErrorContext(ctx, "failed to send partial invitee delete indexer update")
		return isTransientError(err)
	}

	// Tombstone the invitee mapping and cross-reference; the attendee's records remain active.
	h.tombstoneMapping(ctx, fmt.Sprintf("v1_past_meeting_invitees.%s", inviteeID))
	xrefKey := fmt.Sprintf("v1_participant_by_meeting_user.invitee.%s.%s", meetingAndOccurrenceID, username)
	h.tombstoneMapping(ctx, xrefKey)

	funcLogger.InfoContext(ctx, "successfully applied partial invitee delete (attendee record remains active)")
	return false
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

	// IsAIReconciled is true when the attendee record was updated via AI reconcile
	IsAIReconciled bool `json:"is_ai_reconciled"`

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
	if participantData.ProjectUID == "" {
		funcLogger.InfoContext(ctx, "skipping attendee participant sync - parent project not found in mappings")
		return false
	}
	funcLogger = funcLogger.With("participant_uid", participantData.UID)

	// If an invitee cross-reference exists for this participant, preserve is_invited=true
	// so a late-arriving attendee upsert doesn't reset a flag the invitee handler already set.
	if participantData.Username != "" {
		inviteeXrefKey := fmt.Sprintf("v1_participant_by_meeting_user.invitee.%s.%s",
			participantData.MeetingAndOccurrenceID, participantData.Username)
		if entry, err := h.v1MappingsKV.Get(ctx, inviteeXrefKey); err == nil && !entryIsTombstoned(entry) {
			participantData.IsInvited = true
		}
	}

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

	// Store attendee mapping and cross-reference (keyed by meeting+username so the invitee
	// delete handler can determine whether an attendee record still exists).
	if _, err := h.v1MappingsKV.Put(ctx, mappingKey, []byte("1")); err != nil {
		funcLogger.With(logging.ErrKey, err).WarnContext(ctx, "failed to store attendee participant mapping")
	}
	if participantData.Username != "" {
		xrefKey := fmt.Sprintf("v1_participant_by_meeting_user.attendee.%s.%s",
			participantData.MeetingAndOccurrenceID, participantData.Username)
		if _, err := h.v1MappingsKV.Put(ctx, xrefKey, []byte(participantData.UID)); err != nil {
			funcLogger.With(logging.ErrKey, err).WarnContext(ctx, "failed to store attendee cross-reference mapping")
		}
	}

	funcLogger.InfoContext(ctx, "successfully processed past meeting attendee")
	return false
}

// handlePastMeetingAttendeeDelete processes attendee deletions.
// If an invitee record still exists for the same participant, a partial delete is applied:
// the indexer record is updated with is_attended=false and FGA is updated via member_put rather
// than member_remove, so the participant retains access from their invitee record.
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

	if v1Data == nil {
		funcLogger.WarnContext(ctx, "no v1Data available for attendee delete, skipping")
		return false
	}

	username := utils.GetString(v1Data["lf_sso"])
	meetingAndOccurrenceID := utils.GetString(v1Data["meeting_and_occurrence_id"])

	// Check if an invitee record still exists for this participant.
	if username != "" && meetingAndOccurrenceID != "" {
		inviteeXrefKey := fmt.Sprintf("v1_participant_by_meeting_user.invitee.%s.%s", meetingAndOccurrenceID, username)
		if entry, err := h.v1MappingsKV.Get(ctx, inviteeXrefKey); err == nil && !entryIsTombstoned(entry) {
			survivingInviteeID := string(entry.Value())
			funcLogger.DebugContext(ctx, "participant has active invitee record; applying partial attendee delete",
				"surviving_invitee_id", survivingInviteeID)
			return h.handlePartialAttendeeDelete(ctx, funcLogger, key, attendeeID, survivingInviteeID, meetingAndOccurrenceID, username)
		}
	}

	// Full delete — no invitee record survives.
	return h.fullDeleteAttendee(ctx, funcLogger, key, attendeeID, meetingAndOccurrenceID, username)
}

// fullDeleteAttendee performs a full indexer delete and FGA member_remove for an attendee
// when no sibling invitee record survives. Called from both the normal delete path and
// as a fallback when the sibling is found to be missing.
func (h *EventHandlers) fullDeleteAttendee(
	ctx context.Context,
	funcLogger *slog.Logger,
	key, attendeeID, meetingAndOccurrenceID, username string,
) (retry bool) {
	var accessPayload []byte
	var deleteAccessSubject string
	if username != "" {
		auth0Username, err := h.userLookup.MapUsernameToAuthSub(ctx, username)
		if err != nil {
			funcLogger.With(logging.ErrKey, err).ErrorContext(ctx, "failed to resolve auth sub for attendee delete")
			return true
		}
		if accessPayload, err = buildGenericMemberRemovePayload("v1_past_meeting", meetingAndOccurrenceID, auth0Username); err != nil {
			funcLogger.With(logging.ErrKey, err).ErrorContext(ctx, "failed to build member remove payload")
			return false
		}
		deleteAccessSubject = "lfx.fga-sync.member_remove"
	} else {
		funcLogger.DebugContext(ctx, "no username available, skipping access control message for attendee delete")
	}

	result := h.handleMeetingTypeDelete(ctx, key, attendeeID, accessPayload, meetingDeleteConfig{
		indexerSubject:      "lfx.index.v1_past_meeting_participant",
		deleteAccessSubject: deleteAccessSubject,
		tombstoneKeyFmts:    []string{"v1_past_meeting_attendees.%s"},
	})
	if !result && username != "" && meetingAndOccurrenceID != "" {
		h.tombstoneMapping(ctx, fmt.Sprintf("v1_participant_by_meeting_user.attendee.%s.%s", meetingAndOccurrenceID, username))
	}
	return result
}

// handlePartialAttendeeDelete is called when an attendee record is deleted but an invitee record
// still exists. It sends an indexer UPDATE with is_attended=false and a member_put to update FGA
// relations, so the participant retains access from their invitee record.
func (h *EventHandlers) handlePartialAttendeeDelete(
	ctx context.Context,
	funcLogger *slog.Logger,
	key, attendeeID, survivingInviteeID, meetingAndOccurrenceID, username string,
) (retry bool) {
	// Fetch the surviving invitee data to build an accurate participant record.
	inviteeEntry, err := h.v1ObjectsKV.Get(ctx, fmt.Sprintf("itx-zoom-past-meetings-invitees.%s", survivingInviteeID))
	if err != nil {
		if !errors.Is(err, jetstream.ErrKeyNotFound) {
			funcLogger.With(logging.ErrKey, err).ErrorContext(ctx, "transient error fetching invitee data for partial attendee delete")
			return true
		}
		// Sibling invitee is gone — fall back to a full attendee delete.
		funcLogger.WarnContext(ctx, "surviving invitee not found during partial attendee delete; falling back to full delete",
			"surviving_invitee_id", survivingInviteeID)
		return h.fullDeleteAttendee(ctx, funcLogger, key, attendeeID, meetingAndOccurrenceID, username)
	}
	inviteeData, err := decodeData(inviteeEntry.Value())
	if err != nil {
		funcLogger.With(logging.ErrKey, err).ErrorContext(ctx, "failed to decode invitee data for partial attendee delete")
		return false
	}

	participantData, err := convertMapToInviteeParticipantData(ctx, inviteeData, h.userLookup, h.idMapper, h.v1ObjectsKV, funcLogger)
	if err != nil {
		funcLogger.With(logging.ErrKey, err).ErrorContext(ctx, "failed to convert invitee data for partial attendee delete")
		return isTransientError(err)
	}
	// The attendee record is gone; the invitee record remains.
	participantData.IsInvited = true
	participantData.IsAttended = false

	if err := h.publisher.PublishIndexerDelete(ctx, "lfx.index.v1_past_meeting_participant", attendeeID); err != nil {
		funcLogger.With(logging.ErrKey, err).ErrorContext(ctx, "failed to send indexer delete for partial attendee delete")
		return isTransientError(err)
	}

	if err := h.publisher.PublishPastMeetingParticipantEvent(ctx, string(indexerConstants.ActionUpdated), participantData); err != nil {
		funcLogger.With(logging.ErrKey, err).ErrorContext(ctx, "failed to send partial attendee delete indexer update")
		return isTransientError(err)
	}

	// Tombstone the attendee mapping and cross-reference; the invitee's records remain active.
	h.tombstoneMapping(ctx, fmt.Sprintf("v1_past_meeting_attendees.%s", attendeeID))
	xrefKey := fmt.Sprintf("v1_participant_by_meeting_user.attendee.%s.%s", meetingAndOccurrenceID, username)
	h.tombstoneMapping(ctx, xrefKey)

	funcLogger.InfoContext(ctx, "successfully applied partial attendee delete (invitee record remains active)")
	return false
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
		return nil, fmt.Errorf("missing required fields: invitee_id or meeting_and_occurrence_id")
	}

	// Get project SFID and slug: prefer the values from the invitee record, but fall back to
	// the parent past_meeting when proj_id is absent (it is omitempty and may be missing for
	// some v1 invitee records). This ensures those records are indexed rather than silently
	// dropped, and that project_slug is always propagated so the Persona Service can resolve
	// the project without per-record fetches at query time.
	projectSFID, projectSlug, err := resolveProjectFields(ctx, rawInvitee.MeetingAndOccurrenceID, rawInvitee.ProjectID, rawInvitee.ProjectSlug, v1ObjectsKV, logger)
	if err != nil {
		return nil, err
	}
	if projectSFID == "" {
		return nil, fmt.Errorf("invitee missing project ID: proj_id absent and parent past_meeting not yet available (transient)")
	}

	// Map project ID. A missing mapping means the project isn't in v2 yet — the caller skips.
	// Any other error is transient and propagated for retry.
	projectUID, err := idMapper.MapProjectV1ToV2(ctx, projectSFID)
	if err != nil && domain.GetErrorType(err) != domain.ErrorTypeValidation {
		return nil, fmt.Errorf("failed to map project ID (transient): %w", err)
	}

	// Determine if host (lookup registrant if available)
	isHost := false
	if rawInvitee.RegistrantID != "" {
		registrantKey := fmt.Sprintf("itx-zoom-meetings-registrants-v2.%s", rawInvitee.RegistrantID)
		if registrantEntry, err := v1ObjectsKV.Get(ctx, registrantKey); err == nil {
			if registrantData, err := decodeData(registrantEntry.Value()); err == nil {
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
		ProjectSlug:            projectSlug,
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
		UpdatedAt:              modifiedAt,
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

	// Get project SFID and slug from the attendee record; fall back to the parent past_meeting
	// for any missing values so the Persona Service can always resolve the project at query time.
	projectSFID, projectSlug, err := resolveProjectFields(ctx, rawAttendee.MeetingAndOccurrenceID, rawAttendee.ProjectID, rawAttendee.ProjectSlug, v1ObjectsKV, logger)
	if err != nil {
		return nil, err
	}
	if projectSFID == "" {
		return nil, fmt.Errorf("attendee missing project ID: proj_id absent and parent past_meeting not yet available (transient)")
	}

	// Map project ID. A missing mapping means the project isn't in v2 yet — the caller skips.
	// Any other error is transient and propagated for retry.
	projectUID, err := idMapper.MapProjectV1ToV2(ctx, projectSFID)
	if err != nil && domain.GetErrorType(err) != domain.ErrorTypeValidation {
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
		s := models.ParticipantSession{
			UID:         rawSession.ParticipantUUID,
			LeaveReason: rawSession.LeaveReason,
		}
		if t, err := parseTime(rawSession.JoinTime); err == nil {
			s.JoinTime = &t
		}
		if t, err := parseTime(rawSession.LeaveTime); err == nil {
			s.LeaveTime = &t
		}
		sessions = append(sessions, s)
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
		ProjectSlug:            projectSlug,
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
		IsUnknown:              rawAttendee.IsUnknown,
		IsAIReconciled:         rawAttendee.IsAIReconciled,
		IsAutoMatched:          rawAttendee.IsAutoMatched,
		ZoomUserName:           rawAttendee.ZoomUserName,
		MappedInviteeName:      rawAttendee.MappedInviteeName,
		Sessions:               sessions,
		CreatedAt:              createdAt,
		UpdatedAt:              modifiedAt,
	}, nil
}

// resolveProjectFields returns the project SFID and slug for a participant record.
// If either field is missing from the record, it falls back to a KV lookup of the
// parent past_meeting to fill the gaps.
func resolveProjectFields(
	ctx context.Context,
	meetingAndOccurrenceID, projectSFID, projectSlug string,
	v1ObjectsKV jetstream.KeyValue,
	logger *slog.Logger,
) (resolvedSFID, resolvedSlug string, err error) {
	if projectSFID != "" && projectSlug != "" {
		return projectSFID, projectSlug, nil
	}

	sfid, slug, err := lookupProjectFromPastMeeting(ctx, meetingAndOccurrenceID, v1ObjectsKV, logger)
	if err != nil {
		return "", "", fmt.Errorf("failed to lookup project from parent past_meeting (transient): %w", err)
	}

	if projectSFID != "" {
		sfid = projectSFID
	}
	if projectSlug != "" {
		slug = projectSlug
	}
	return sfid, slug, nil
}

// lookupProjectFromPastMeeting fetches the proj_id and project_slug of the parent past meeting
// from the v1-objects KV bucket. Returns empty strings (no error) when the record is not found —
// that is a permanent miss and the caller should not retry. Returns a non-nil error for transient
// KV fetch failures or decode failures (caller should retry).
func lookupProjectFromPastMeeting(
	ctx context.Context,
	meetingAndOccurrenceID string,
	v1ObjectsKV jetstream.KeyValue,
	logger *slog.Logger,
) (projSFID, projectSlug string, err error) {
	if meetingAndOccurrenceID == "" {
		return "", "", nil
	}
	pastMeetingKey := fmt.Sprintf("itx-zoom-past-meetings.%s", meetingAndOccurrenceID)
	entry, kvErr := v1ObjectsKV.Get(ctx, pastMeetingKey)
	if kvErr != nil {
		if errors.Is(kvErr, jetstream.ErrKeyNotFound) {
			logger.WarnContext(ctx, "parent past_meeting not found for project lookup", "key", pastMeetingKey)
			return "", "", nil
		}
		return "", "", fmt.Errorf("transient error fetching parent past_meeting: %w", kvErr)
	}
	pastMeetingData, decErr := decodeData(entry.Value())
	if decErr != nil {
		return "", "", fmt.Errorf("transient error decoding parent past_meeting: %w", decErr)
	}
	return utils.GetString(pastMeetingData["proj_id"]), utils.GetString(pastMeetingData["project_slug"]), nil
}
