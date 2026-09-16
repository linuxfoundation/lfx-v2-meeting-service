// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"encoding/json"
	"errors"
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

// inviteeDBRaw represents raw past meeting invitee data from v1 DynamoDB/NATS KV bucket
type inviteeDBRaw struct {
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

// UnmarshalJSON implements custom unmarshaling for inviteeDBRaw.
func (i *inviteeDBRaw) UnmarshalJSON(data []byte) error {
	type Alias inviteeDBRaw
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
		// Distinguish ErrKeyNotFound (attendee object absent) from transient KV errors: a
		// transient failure here would publish the invitee record with zero-valued attendee-only
		// fields, defeating the merge that preserves IsUnknown, IsAIReconciled, etc.
		mergeSibling: func(ctx context.Context, self *models.PastMeetingParticipantEventData, siblingID string) error {
			attendeeEntry, err := h.v1ObjectsKV.Get(ctx, "itx-zoom-past-meetings-attendees."+siblingID)
			if err != nil {
				if !errors.Is(err, jetstream.ErrKeyNotFound) {
					return err // transient: caller will retry before publishing
				}
				// Sibling object absent (xref is stale): don't set IsAttended so the
				// invitee record is not incorrectly published as attended.
				return nil
			}
			// Attendee object confirmed present — set the flag and merge fields.
			self.IsAttended = true
			attendeeMap, err := decodeData(attendeeEntry.Value())
			if err != nil {
				// Permanent decode failure: the payload is corrupt and won't change on retry.
				// Publish with IsAttended=true (which is confirmed correct) rather than
				// exhausting redeliveries and dropping the invitee update entirely.
				h.logger.With(logging.ErrKey, err).WarnContext(ctx, "failed to decode attendee sibling for merge; publishing without attendee-only fields")
				return nil
			}
			rawData, err := decodeAttendeeRaw(attendeeMap)
			if err != nil {
				h.logger.With(logging.ErrKey, err).WarnContext(ctx, "failed to decode attendee raw fields for merge; publishing without attendee-only fields")
				return nil
			}
			self.IsVerified = rawData.isVerified
			self.IsUnknown = rawData.isUnknown
			self.IsAIReconciled = rawData.isAIReconciled
			self.IsAutoMatched = rawData.isAutoMatched
			self.ZoomUserName = rawData.zoomUserName
			self.MappedInviteeName = rawData.mappedInviteeName
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

// attendeeDBRaw represents raw past meeting attendee data from v1 DynamoDB/NATS KV bucket
type attendeeDBRaw struct {
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
	Sessions []attendeeSessionDBRaw `json:"sessions"`

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

// UnmarshalJSON implements custom unmarshaling for attendeeDBRaw.
func (a *attendeeDBRaw) UnmarshalJSON(data []byte) error {
	type Alias attendeeDBRaw
	tmp := struct{ *Alias }{Alias: (*Alias)(a)}
	return json.Unmarshal(data, &tmp)
}

// attendeeSessionDBRaw represents raw attendee session data from v1 DynamoDB/NATS KV bucket
type attendeeSessionDBRaw struct {
	ParticipantUUID string `json:"participant_uuid"`
	JoinTime        string `json:"join_time"`
	LeaveTime       string `json:"leave_time"`
	LeaveReason     string `json:"leave_reason"`
}

// UnmarshalJSON implements custom unmarshaling for attendeeSessionDBRaw.
func (a *attendeeSessionDBRaw) UnmarshalJSON(data []byte) error {
	type Alias attendeeSessionDBRaw
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

// rawParticipantData is a normalised intermediate representation populated from
// either inviteeDBRaw or attendeeDBRaw. It lets convertParticipant run the shared
// logic (project/committee resolution, user enrichment, org flags, time parsing)
// once, regardless of which wire format was decoded.
type rawParticipantData struct {
	uid                string
	meetingAndOccID    string
	meetingID          string
	projectSFID        string
	projectSlug        string
	committeeID        string
	email              string
	firstName          string
	lastName           string
	username           string
	lfUserID           string
	jobTitle           string
	org                string
	profilePic         string
	registrantID       string
	createdAt          string
	modifiedAt         string
	orgIsMember        *bool
	orgIsProjectMember *bool
	// flags set by each side
	isInvited  bool
	isAttended bool
	isHost     bool // pre-computed by invitee side via KV lookup; always false for attendees
	// attendee-only fields (zero-valued for invitees)
	isVerified        bool
	isUnknown         bool
	isAIReconciled    bool
	isAutoMatched     bool
	zoomUserName      string
	mappedInviteeName string
	sessions          []attendeeSessionDBRaw
}

// decodeInviteeRaw decodes v1Data into rawParticipantData for the invitee side.
// It performs the registrant KV lookup needed to determine isHost.
func decodeInviteeRaw(ctx context.Context, v1Data map[string]interface{}, v1ObjectsKV jetstream.KeyValue) (rawParticipantData, error) {
	raw, err := decodeV1[inviteeDBRaw](v1Data)
	if err != nil {
		return rawParticipantData{}, err
	}
	if raw.InviteeID == "" || raw.MeetingAndOccurrenceID == "" {
		return rawParticipantData{}, fmt.Errorf("missing required fields: invitee_id or meeting_and_occurrence_id")
	}

	// Invitee host status requires a registrant record lookup.
	isHost := false
	if raw.RegistrantID != "" {
		if entry, err := v1ObjectsKV.Get(ctx, fmt.Sprintf("itx-zoom-meetings-registrants-v2.%s", raw.RegistrantID)); err == nil {
			if data, err := decodeData(entry.Value()); err == nil {
				isHost = utils.GetBool(data["host"])
			}
		}
	}

	return rawParticipantData{
		uid:                raw.InviteeID,
		meetingAndOccID:    raw.MeetingAndOccurrenceID,
		meetingID:          raw.MeetingID,
		projectSFID:        raw.ProjectID,
		projectSlug:        raw.ProjectSlug,
		committeeID:        raw.CommitteeID,
		email:              raw.Email,
		firstName:          raw.FirstName,
		lastName:           raw.LastName,
		username:           raw.LFSSO,
		lfUserID:           raw.LFUserID,
		jobTitle:           raw.JobTitle,
		org:                raw.Org,
		profilePic:         raw.ProfilePicture,
		registrantID:       raw.RegistrantID,
		createdAt:          raw.CreatedAt,
		modifiedAt:         raw.ModifiedAt,
		orgIsMember:        raw.OrgIsMember,
		orgIsProjectMember: raw.OrgIsProjectMember,
		isInvited:          true,
		isAttended:         false,
		isHost:             isHost,
	}, nil
}

// decodeAttendeeRaw decodes v1Data into rawParticipantData for the attendee side.
// No KV lookups are needed: host is always false for attendees, and isInvited is
// derived from the presence of a registrant_id on the record itself.
func decodeAttendeeRaw(v1Data map[string]interface{}) (rawParticipantData, error) {
	raw, err := decodeV1[attendeeDBRaw](v1Data)
	if err != nil {
		return rawParticipantData{}, err
	}
	if raw.ID == "" || raw.MeetingAndOccurrenceID == "" {
		return rawParticipantData{}, fmt.Errorf("missing required fields: id or meeting_and_occurrence_id")
	}

	firstName, lastName := parseName(raw.Name)

	return rawParticipantData{
		uid:                raw.ID,
		meetingAndOccID:    raw.MeetingAndOccurrenceID,
		meetingID:          raw.MeetingID,
		projectSFID:        raw.ProjectID,
		projectSlug:        raw.ProjectSlug,
		committeeID:        raw.CommitteeID,
		email:              raw.Email,
		firstName:          firstName,
		lastName:           lastName,
		username:           raw.LFSSO,
		lfUserID:           raw.LFUserID,
		jobTitle:           raw.JobTitle,
		org:                raw.Org,
		profilePic:         raw.ProfilePicture,
		registrantID:       raw.RegistrantID,
		createdAt:          raw.CreatedAt,
		modifiedAt:         raw.ModifiedAt,
		orgIsMember:        raw.OrgIsMember,
		orgIsProjectMember: raw.OrgIsProjectMember,
		isInvited:          raw.RegistrantID != "",
		isAttended:         true,
		isHost:             false, // attendee records don't carry host info
		isVerified:         raw.IsVerified,
		isUnknown:          raw.IsUnknown,
		isAIReconciled:     raw.IsAIReconciled,
		isAutoMatched:      raw.IsAutoMatched,
		zoomUserName:       raw.ZoomUserName,
		mappedInviteeName:  raw.MappedInviteeName,
		sessions:           raw.Sessions,
	}, nil
}

// convertParticipant is the shared conversion core. It takes an already-decoded
// rawParticipantData and performs project/committee resolution, user enrichment,
// session conversion, time parsing, and org-flag unpacking — logic that is
// identical for both the invitee and attendee sides.
func convertParticipant(
	ctx context.Context,
	raw rawParticipantData,
	userLookup domain.V1UserLookup,
	idMapper domain.IDMapper,
	v1ObjectsKV jetstream.KeyValue,
	logger *slog.Logger,
) (*models.PastMeetingParticipantEventData, error) {
	// Resolve project SFID and slug: prefer values from the record, fall back to the
	// parent past_meeting so records with omitted proj_id are indexed rather than dropped.
	projectSFID, projectSlug, err := resolveProjectFields(ctx, raw.meetingAndOccID, raw.projectSFID, raw.projectSlug, v1ObjectsKV, logger)
	if err != nil {
		return nil, err
	}
	if projectSFID == "" {
		return nil, fmt.Errorf("participant missing project ID: proj_id absent and parent past_meeting not yet available (transient)")
	}

	// Map project ID. A missing mapping means the project isn't in v2 yet — caller skips.
	// Any other error is transient and propagated for retry.
	projectUID, err := idMapper.MapProjectV1ToV2(ctx, projectSFID)
	if err != nil && domain.GetErrorType(err) != domain.ErrorTypeValidation {
		return nil, fmt.Errorf("failed to map project ID (transient): %w", err)
	}

	// Map committee_id to v2 UID when present; a missing mapping is non-fatal.
	var committeeUID string
	if raw.committeeID != "" {
		uid, mapErr := idMapper.MapCommitteeV1ToV2(ctx, raw.committeeID)
		if mapErr != nil {
			if domain.GetErrorType(mapErr) != domain.ErrorTypeValidation {
				return nil, fmt.Errorf("failed to map committee ID (transient): %w", mapErr)
			}
			logger.With(logging.ErrKey, mapErr).WarnContext(ctx, "committee mapping not found for participant", "v1_id", raw.committeeID)
		} else {
			committeeUID = uid
		}
	}

	// Enrich first/last name from the auth-service profile when the record is missing them.
	firstName := raw.firstName
	lastName := raw.lastName
	if raw.lfUserID != "" && (firstName == "" || lastName == "") {
		v1User, err := userLookup.LookupUser(ctx, raw.lfUserID)
		if err != nil {
			logger.With(logging.ErrKey, err).WarnContext(ctx, "failed to lookup v1 user", "lf_user_id", raw.lfUserID)
		} else if v1User != nil {
			if firstName == "" && v1User.FirstName != "" {
				firstName = v1User.FirstName
			}
			if lastName == "" && v1User.LastName != "" {
				lastName = v1User.LastName
			}
		}
	}

	// Convert sessions (non-nil only for attendees; invitees carry nil).
	var sessions []models.ParticipantSession
	for _, s := range raw.sessions {
		ps := models.ParticipantSession{
			UID:         s.ParticipantUUID,
			LeaveReason: s.LeaveReason,
		}
		if t, err := parseTime(s.JoinTime); err == nil {
			ps.JoinTime = &t
		}
		if t, err := parseTime(s.LeaveTime); err == nil {
			ps.LeaveTime = &t
		}
		sessions = append(sessions, ps)
	}

	createdAt, _ := parseTime(raw.createdAt)
	modifiedAt, _ := parseTime(raw.modifiedAt)

	orgIsMember := false
	if raw.orgIsMember != nil {
		orgIsMember = *raw.orgIsMember
	}
	orgIsProjectMember := false
	if raw.orgIsProjectMember != nil {
		orgIsProjectMember = *raw.orgIsProjectMember
	}

	return &models.PastMeetingParticipantEventData{
		UID:                    raw.uid,
		MeetingAndOccurrenceID: raw.meetingAndOccID,
		MeetingID:              raw.meetingID,
		ProjectUID:             projectUID,
		ProjectSlug:            projectSlug,
		CommitteeUID:           committeeUID,
		Email:                  raw.email,
		FirstName:              firstName,
		LastName:               lastName,
		Host:                   raw.isHost,
		JobTitle:               raw.jobTitle,
		OrgName:                raw.org,
		OrgIsMember:            orgIsMember,
		OrgIsProjectMember:     orgIsProjectMember,
		AvatarURL:              raw.profilePic,
		Username:               raw.username,
		IsInvited:              raw.isInvited,
		IsAttended:             raw.isAttended,
		IsVerified:             raw.isVerified,
		IsUnknown:              raw.isUnknown,
		IsAIReconciled:         raw.isAIReconciled,
		IsAutoMatched:          raw.isAutoMatched,
		ZoomUserName:           raw.zoomUserName,
		MappedInviteeName:      raw.mappedInviteeName,
		Sessions:               sessions,
		CreatedAt:              createdAt,
		UpdatedAt:              modifiedAt,
	}, nil
}

// convertMapToInviteeParticipantData decodes invitee v1Data and runs the shared
// conversion core. Satisfies participantConvertFn.
func convertMapToInviteeParticipantData(ctx context.Context, v1Data map[string]interface{}, userLookup domain.V1UserLookup, idMapper domain.IDMapper, v1ObjectsKV jetstream.KeyValue, logger *slog.Logger) (*models.PastMeetingParticipantEventData, error) {
	raw, err := decodeInviteeRaw(ctx, v1Data, v1ObjectsKV)
	if err != nil {
		return nil, err
	}
	return convertParticipant(ctx, raw, userLookup, idMapper, v1ObjectsKV, logger)
}

// convertMapToAttendeeParticipantData decodes attendee v1Data and runs the shared
// conversion core. Satisfies participantConvertFn.
func convertMapToAttendeeParticipantData(ctx context.Context, v1Data map[string]interface{}, userLookup domain.V1UserLookup, idMapper domain.IDMapper, v1ObjectsKV jetstream.KeyValue, logger *slog.Logger) (*models.PastMeetingParticipantEventData, error) {
	raw, err := decodeAttendeeRaw(v1Data)
	if err != nil {
		return nil, err
	}
	return convertParticipant(ctx, raw, userLookup, idMapper, v1ObjectsKV, logger)
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
