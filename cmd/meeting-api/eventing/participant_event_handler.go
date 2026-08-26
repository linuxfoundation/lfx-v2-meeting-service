// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

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

// handlePastMeetingInviteeUpdate processes updates to past meeting invitees.
func (h *EventHandlers) handlePastMeetingInviteeUpdate(ctx context.Context, key string, v1Data map[string]interface{}) (retry bool) {
	return h.syncParticipantUpdate(ctx, key, v1Data, participantUpdateConfig{
		ownXrefPrefix:       "invitee",
		siblingXrefPrefix:   "attendee",
		mappingKeyPrefix:    "v1_past_meeting_invitees",
		siblingObjectPrefix: "itx-zoom-past-meetings-attendees.",
		convert:             convertMapToInviteeParticipantData,
		siblingConvert:      convertMapToAttendeeParticipantData,
		// Invitee merges attendee-only fields so a late-arriving invitee upsert doesn't
		// overwrite values the attendee handler already set (is_unknown, is_ai_reconciled, etc.).
		mergeSibling: func(ctx context.Context, self *models.PastMeetingParticipantEventData, siblingID string) error {
			self.IsAttended = true
			attendeeEntry, err := h.v1ObjectsKV.Get(ctx, "itx-zoom-past-meetings-attendees."+siblingID)
			if err == nil {
				if attendeeMap, err := decodeData(attendeeEntry.Value()); err == nil {
					if jsonBytes, err := json.Marshal(attendeeMap); err == nil {
						var rawAttendee AttendeeDBRaw
						if err := json.Unmarshal(jsonBytes, &rawAttendee); err == nil {
							self.IsUnknown = rawAttendee.IsUnknown
							self.IsAIReconciled = rawAttendee.IsAIReconciled
							self.IsAutoMatched = rawAttendee.IsAutoMatched
							self.ZoomUserName = rawAttendee.ZoomUserName
							self.MappedInviteeName = rawAttendee.MappedInviteeName
						}
					}
				}
			}
			return nil
		},
		// When the invitee's username changes, the surviving sibling is an attendee record:
		// clear its invitee relation and affirm its attendee relation.
		setSiblingFlags: func(s *models.PastMeetingParticipantEventData) {
			s.IsInvited = false
			s.IsAttended = true
		},
	})
}

// handlePastMeetingInviteeDelete processes invitee deletions.
// If an attendee record still exists for the same participant, a partial delete is applied:
// the indexer record is updated with is_invited=false and FGA is updated via member_put rather
// than member_remove, so the participant retains access from their attendee record.
func (h *EventHandlers) handlePastMeetingInviteeDelete(ctx context.Context, key string, v1Data map[string]interface{}) (retry bool) {
	inviteeID := extractIDFromKey(key, "itx-zoom-past-meetings-invitees.")
	return h.syncParticipantDelete(ctx, key, v1Data, inviteeID, participantDeleteConfig{
		ownXrefPrefix:       "invitee",
		siblingXrefPrefix:   "attendee",
		mappingKeyPrefix:    "v1_past_meeting_invitees",
		siblingObjectPrefix: "itx-zoom-past-meetings-attendees.",
		siblingConvert:      convertMapToAttendeeParticipantData,
		setSiblingFlags: func(s *models.PastMeetingParticipantEventData) {
			s.IsInvited = false
			s.IsAttended = true
		},
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

// handlePastMeetingAttendeeUpdate processes updates to past meeting attendees.
func (h *EventHandlers) handlePastMeetingAttendeeUpdate(ctx context.Context, key string, v1Data map[string]interface{}) (retry bool) {
	return h.syncParticipantUpdate(ctx, key, v1Data, participantUpdateConfig{
		ownXrefPrefix:       "attendee",
		siblingXrefPrefix:   "invitee",
		mappingKeyPrefix:    "v1_past_meeting_attendees",
		siblingObjectPrefix: "itx-zoom-past-meetings-invitees.",
		convert:             convertMapToAttendeeParticipantData,
		siblingConvert:      convertMapToInviteeParticipantData,
		// Attendee merges invitee presence with a flag only — no additional fields to copy.
		mergeSibling: func(_ context.Context, self *models.PastMeetingParticipantEventData, _ string) error {
			self.IsInvited = true
			return nil
		},
		// When the attendee's username changes, the surviving sibling is an invitee record:
		// affirm its invitee relation and clear its attendee relation.
		setSiblingFlags: func(s *models.PastMeetingParticipantEventData) {
			s.IsInvited = true
			s.IsAttended = false
		},
	})
}

// handlePastMeetingAttendeeDelete processes attendee deletions.
// If an invitee record still exists for the same participant, a partial delete is applied:
// the indexer record is updated with is_attended=false and FGA is updated via member_put rather
// than member_remove, so the participant retains access from their invitee record.
func (h *EventHandlers) handlePastMeetingAttendeeDelete(ctx context.Context, key string, v1Data map[string]interface{}) (retry bool) {
	attendeeID := extractIDFromKey(key, "itx-zoom-past-meetings-attendees.")
	return h.syncParticipantDelete(ctx, key, v1Data, attendeeID, participantDeleteConfig{
		ownXrefPrefix:       "attendee",
		siblingXrefPrefix:   "invitee",
		mappingKeyPrefix:    "v1_past_meeting_attendees",
		siblingObjectPrefix: "itx-zoom-past-meetings-invitees.",
		siblingConvert:      convertMapToInviteeParticipantData,
		setSiblingFlags: func(s *models.PastMeetingParticipantEventData) {
			s.IsInvited = true
			s.IsAttended = false
		},
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

	// Map the invitee's own committee_id to a v2 UID. Only set when the invitee record carries a
	// committee_id — a missing mapping is non-fatal.
	var committeeUID string
	if rawInvitee.CommitteeID != "" {
		uid, mapErr := idMapper.MapCommitteeV1ToV2(ctx, rawInvitee.CommitteeID)
		if mapErr != nil {
			if domain.GetErrorType(mapErr) != domain.ErrorTypeValidation {
				return nil, fmt.Errorf("failed to map committee ID (transient): %w", mapErr)
			}
			logger.With(logging.ErrKey, mapErr).WarnContext(ctx, "committee mapping not found for invitee", "v1_id", rawInvitee.CommitteeID)
		} else {
			committeeUID = uid
		}
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
		CommitteeUID:           committeeUID,
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

	// Map the attendee's own committee_id to a v2 UID. Only set when the attendee record carries a
	// committee_id — a missing mapping is non-fatal.
	var committeeUID string
	if rawAttendee.CommitteeID != "" {
		uid, mapErr := idMapper.MapCommitteeV1ToV2(ctx, rawAttendee.CommitteeID)
		if mapErr != nil {
			if domain.GetErrorType(mapErr) != domain.ErrorTypeValidation {
				return nil, fmt.Errorf("failed to map committee ID (transient): %w", mapErr)
			}
			logger.With(logging.ErrKey, mapErr).WarnContext(ctx, "committee mapping not found for attendee", "v1_id", rawAttendee.CommitteeID)
		} else {
			committeeUID = uid
		}
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
		CommitteeUID:           committeeUID,
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

	sfid, slug, _, err := lookupProjectFromPastMeeting(ctx, meetingAndOccurrenceID, v1ObjectsKV, logger)
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
