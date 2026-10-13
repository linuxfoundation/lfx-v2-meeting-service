// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package itx

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/models/itx"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/utils"
)

// PastMeetingParticipantService handles unified participant operations by routing to invitee/attendee endpoints
type PastMeetingParticipantService struct {
	auditStamper
	participantClient domain.ITXPastMeetingParticipantClient
	idMapper          domain.IDMapper
}

// NewPastMeetingParticipantService creates a new participant service. userMetadata may be
// nil (e.g. when NATS is disabled), in which case created_by / updated_by on invitee /
// attendee records are limited to the JWT-derived username/email rather than blocking
// the request.
func NewPastMeetingParticipantService(participantClient domain.ITXPastMeetingParticipantClient, idMapper domain.IDMapper, userMetadata domain.UserMetadataReader) *PastMeetingParticipantService {
	return &PastMeetingParticipantService{
		auditStamper:      auditStamper{userMetadata: userMetadata},
		participantClient: participantClient,
		idMapper:          idMapper,
	}
}

// ParticipantResponse represents a cohesive participant combining invitee and attendee data
type ParticipantResponse struct {
	// IDs
	InviteeID     string // Present if is_invited=true
	AttendeeID    string // Present if is_attended=true
	PastMeetingID string // Past meeting ID (meeting_id-occurrence_id)
	MeetingID     string // Meeting ID

	// Flags
	IsInvited  bool
	IsAttended bool

	// User data (prioritized from invitee if present, otherwise from attendee)
	FirstName          string
	LastName           string
	Email              string
	Username           string // LF SSO
	LFUserID           string
	OrgName            string
	JobTitle           string
	AvatarURL          string
	OrgIsMember        bool
	OrgIsProjectMember bool

	// Committee data
	CommitteeID           string
	CommitteeRole         string
	IsCommitteeMember     bool
	CommitteeVotingStatus string

	// Attendee-specific fields
	IsVerified        bool
	IsUnknown         bool
	IsAIReconciled    bool
	IsAutoMatched     bool
	ZoomUserName      string
	MappedInviteeName string
	AverageAttendance int
	Sessions          []itx.AttendeeSession

	// Audit fields (prioritized from invitee if present, otherwise from attendee)
	CreatedAt  string
	CreatedBy  *itx.User
	ModifiedAt string
	ModifiedBy *itx.User
}

// CreateParticipant creates a participant by routing to invitee and/or attendee endpoints
// based on is_invited and is_attended flags, then returns a unified response
func (s *PastMeetingParticipantService) CreateParticipant(
	ctx context.Context,
	pastMeetingID string,
	isInvited, isAttended bool,
	inviteeReq *itx.CreateInviteeRequest,
	attendeeReq *itx.CreateAttendeeRequest,
) (*ParticipantResponse, error) {
	// Validate that at least one flag is set
	if !isInvited && !isAttended {
		return nil, domain.NewValidationError("at least one of is_invited or is_attended must be true")
	}

	var inviteeResp *itx.InviteeResponse
	var attendeeResp *itx.AttendeeResponse

	// Resolve the requesting user once and stamp created_by on both invitee and
	// attendee records so the audit trail reflects who added them via the v2 API.
	creator := s.buildRequestingUser(ctx)

	// Create invitee if requested
	if isInvited {
		if inviteeReq != nil {
			inviteeReq.CreatedBy = creator
		}
		resp, err := s.participantClient.CreateInvitee(ctx, pastMeetingID, inviteeReq)
		if err != nil {
			return nil, fmt.Errorf("failed to create invitee: %w", err)
		}
		inviteeResp = resp
	}

	// Create attendee if requested
	if isAttended {
		if attendeeReq != nil {
			attendeeReq.CreatedBy = creator
		}
		resp, err := s.participantClient.CreateAttendee(ctx, pastMeetingID, attendeeReq)
		if err != nil {
			if isInvited {
				slog.WarnContext(ctx, "partial create state: invitee created but attendee creation failed",
					"past_meeting_id", pastMeetingID)
			}
			return nil, fmt.Errorf("failed to create attendee: %w", err)
		}
		attendeeResp = resp
	}

	// Merge into unified response
	return mergeParticipantResponses(pastMeetingID, inviteeResp, attendeeResp, isInvited, isAttended), nil
}

func (s *PastMeetingParticipantService) UpdateParticipant(
	ctx context.Context,
	p *models.UpdatePastMeetingParticipant,
	inviteeReq *itx.UpdateInviteeRequest,
	attendeeReq *itx.UpdateAttendeeRequest,
) (*ParticipantResponse, error) {
	// Resolve the requesting user once and pass it through both operation paths (and
	// any create-fallback within them). Each ResolveProfile call is a NATS request
	// with a 2s timeout, so calling it independently in updateInvitee, updateAttendee,
	// createInviteeFromUpdate and createAttendeeFromUpdate would add up to 4s of
	// latency and could produce inconsistent stamps if one lookup returns a full
	// profile and the other times out. Matches CreateParticipant's approach.
	updater := s.buildRequestingUser(ctx)

	// Every invitee/attendee ID this request touches - whether supplied in the
	// body or resolved through the (global) participant ID mapping - must belong
	// to the authorized past meeting before any ITX call is made on it.
	children, err := s.fetchPastMeetingChildren(ctx, p.PastMeetingID)
	if err != nil {
		return nil, err
	}

	// Reject a body-supplied ID outside the past meeting before either operation
	// writes, so a rejected request leaves no partial change behind. The checks
	// mirror when handleInviteeOperation / handleAttendeeOperation use the ID.
	if p.InviteeID != "" && p.IsInvited != nil && !children.hasInvitee(p.InviteeID) {
		slog.WarnContext(ctx, "invitee does not belong to the past meeting; rejecting",
			"participant_id", p.ParticipantID,
			"invitee_id", p.InviteeID,
			"past_meeting_id", p.PastMeetingID)
		return nil, domain.NewNotFoundError("invitee not found in past meeting")
	}
	if p.AttendeeID != "" && (p.IsAttended != nil || attendeeReq != nil) && !children.hasAttendee(p.AttendeeID) {
		slog.WarnContext(ctx, "attendee does not belong to the past meeting; rejecting",
			"participant_id", p.ParticipantID,
			"attendee_id", p.AttendeeID,
			"past_meeting_id", p.PastMeetingID)
		return nil, domain.NewNotFoundError("attendee not found in past meeting")
	}

	// Surface a failed ITX save as an error instead of silently returning a
	// successful response that doesn't reflect what was actually persisted.
	// Abort before the attendee operation if the invitee operation already
	// failed, mirroring CreateParticipant's sequential-abort pattern - running
	// the attendee write anyway would persist partial state and encourage a
	// retry of an already-applied write.
	inviteeResp, inviteeExists, inviteeErr := s.handleInviteeOperation(ctx, children, p.PastMeetingID, p.ParticipantID, p.InviteeID, p.IsInvited, inviteeReq, updater)
	if inviteeErr != nil {
		return nil, fmt.Errorf("failed to update invitee: %w", inviteeErr)
	}

	attendeeResp, attendeeExists, attendeeErr := s.handleAttendeeOperation(ctx, children, p.PastMeetingID, p.ParticipantID, p.AttendeeID, p.IsAttended, attendeeReq, updater)
	if attendeeErr != nil {
		return nil, fmt.Errorf("failed to update attendee: %w", attendeeErr)
	}

	return mergeParticipantResponses(p.PastMeetingID, inviteeResp, attendeeResp, inviteeExists, attendeeExists), nil
}

func (s *PastMeetingParticipantService) handleInviteeOperation(
	ctx context.Context,
	children *pastMeetingChildren,
	pastMeetingID, participantID, inviteeID string,
	isInvited *bool,
	inviteeReq *itx.UpdateInviteeRequest,
	updater *itx.User,
) (*itx.InviteeResponse, bool, error) {
	if isInvited == nil {
		return nil, false, nil
	}

	var actualInviteeID string
	var inviteeExists bool

	if inviteeID != "" {
		// UpdateParticipant has already checked that inviteeID belongs to the
		// past meeting.
		actualInviteeID = inviteeID
		inviteeExists = true
	} else {
		var err error
		actualInviteeID, inviteeExists, err = s.checkInviteeExists(ctx, participantID)
		if err != nil {
			return nil, false, err
		}
		if inviteeExists && !children.hasInvitee(actualInviteeID) {
			// The mapping is global; an invitee of another past meeting is not
			// an invitee of this one.
			slog.DebugContext(ctx, "mapped invitee does not belong to the past meeting; treating as absent",
				"participant_id", participantID,
				"invitee_id", actualInviteeID,
				"past_meeting_id", pastMeetingID)
			inviteeExists = false
		}
	}

	if !*isInvited {
		if inviteeExists && actualInviteeID != "" {
			if err := s.deleteInvitee(ctx, pastMeetingID, actualInviteeID, participantID); err != nil {
				return nil, false, err
			}
		}
		return nil, false, nil
	}

	if !inviteeExists && inviteeReq != nil {
		resp, err := s.createInviteeFromUpdate(ctx, pastMeetingID, inviteeReq, updater)
		if err != nil {
			return nil, false, err
		}
		return resp, true, nil
	}

	if inviteeExists && inviteeReq != nil && actualInviteeID != "" {
		resp, err := s.updateInvitee(ctx, pastMeetingID, actualInviteeID, participantID, inviteeReq, updater)
		if err != nil {
			return nil, false, err
		}
		return resp, true, nil
	}

	return nil, inviteeExists, nil
}

func (s *PastMeetingParticipantService) handleAttendeeOperation(
	ctx context.Context,
	children *pastMeetingChildren,
	pastMeetingID, participantID, attendeeID string,
	isAttended *bool,
	attendeeReq *itx.UpdateAttendeeRequest,
	updater *itx.User,
) (*itx.AttendeeResponse, bool, error) {
	// A nil isAttended with a non-nil attendeeReq is a reconciliation-only update
	// (e.g. is_ai_reconciled/zoom_user_name) that carries no attendance status
	// change; it must still reach ITX rather than being silently dropped.
	if isAttended == nil && attendeeReq == nil {
		return nil, false, nil
	}

	var actualAttendeeID string
	var attendeeExists bool

	if attendeeID != "" {
		// UpdateParticipant has already checked that attendeeID belongs to the
		// past meeting.
		actualAttendeeID = attendeeID
		attendeeExists = true
	} else {
		var err error
		actualAttendeeID, attendeeExists, err = s.checkAttendeeExists(ctx, participantID)
		if err != nil {
			return nil, false, err
		}
		if attendeeExists && !children.hasAttendee(actualAttendeeID) {
			// The mapping is global; an attendee of another past meeting is not
			// an attendee of this one.
			slog.DebugContext(ctx, "mapped attendee does not belong to the past meeting; treating as absent",
				"participant_id", participantID,
				"attendee_id", actualAttendeeID,
				"past_meeting_id", pastMeetingID)
			attendeeExists = false
		}
	}

	if isAttended != nil && !*isAttended {
		if attendeeExists && actualAttendeeID != "" {
			if err := s.deleteAttendee(ctx, pastMeetingID, actualAttendeeID, participantID); err != nil {
				return nil, false, err
			}
		}
		return nil, false, nil
	}

	// Only create a new attendee record when the caller explicitly marked
	// attendance. A reconciliation-only update (isAttended == nil) against a
	// participant with no attendee record has nothing to reconcile yet.
	if !attendeeExists && attendeeReq != nil && isAttended != nil && *isAttended {
		resp, err := s.createAttendeeFromUpdate(ctx, pastMeetingID, attendeeReq, updater)
		if err != nil {
			return nil, false, err
		}
		return resp, true, nil
	}

	if attendeeExists && attendeeReq != nil && actualAttendeeID != "" {
		resp, err := s.updateAttendee(ctx, pastMeetingID, actualAttendeeID, participantID, attendeeReq, updater)
		if err != nil {
			return nil, false, err
		}
		return resp, true, nil
	}

	return nil, attendeeExists, nil
}

// checkInviteeExists checks if invitee exists by attempting ID mapping.
// Returns invitee ID and existence flag. A non-nil error means the mapping
// lookup itself failed (e.g. NATS timeout/unavailability) - distinct from a
// genuine "not found", which the ID mapper reports as a validation error and
// is treated here as inviteeExists == false, nil.
func (s *PastMeetingParticipantService) checkInviteeExists(ctx context.Context, participantID string) (string, bool, error) {
	inviteeID, err := s.idMapper.MapParticipantV2ToInviteeID(ctx, participantID)
	if err != nil {
		if domain.GetErrorType(err) == domain.ErrorTypeValidation {
			slog.DebugContext(ctx, "invitee does not exist (mapping not found)",
				"participant_id", participantID,
				logging.ErrKey, err)
			return participantID, false, nil
		}
		slog.ErrorContext(ctx, "failed to check invitee existence via ID mapping",
			"participant_id", participantID,
			logging.ErrKey, err)
		return "", false, err
	}
	if inviteeID == "" {
		slog.DebugContext(ctx, "invitee does not exist (empty mapping result)",
			"participant_id", participantID)
		return participantID, false, nil
	}

	slog.DebugContext(ctx, "invitee exists - mapped participant ID to invitee ID",
		"participant_id", participantID,
		"invitee_id", inviteeID)
	return inviteeID, true, nil
}

// checkAttendeeExists checks if attendee exists by attempting ID mapping.
// See checkInviteeExists for the not-found vs mapping-error distinction.
func (s *PastMeetingParticipantService) checkAttendeeExists(ctx context.Context, participantID string) (string, bool, error) {
	attendeeID, err := s.idMapper.MapParticipantV2ToAttendeeID(ctx, participantID)
	if err != nil {
		if domain.GetErrorType(err) == domain.ErrorTypeValidation {
			slog.DebugContext(ctx, "attendee does not exist (mapping not found)",
				"participant_id", participantID,
				logging.ErrKey, err)
			return participantID, false, nil
		}
		slog.ErrorContext(ctx, "failed to check attendee existence via ID mapping",
			"participant_id", participantID,
			logging.ErrKey, err)
		return "", false, err
	}
	if attendeeID == "" {
		slog.DebugContext(ctx, "attendee does not exist (empty mapping result)",
			"participant_id", participantID)
		return participantID, false, nil
	}

	slog.DebugContext(ctx, "attendee exists - mapped participant ID to attendee ID",
		"participant_id", participantID,
		"attendee_id", attendeeID)
	return attendeeID, true, nil
}

// pastMeetingChildren holds the IDs of the invitees and attendees that belong
// to one past meeting, as reported by ITX.
type pastMeetingChildren struct {
	invitees  map[string]struct{}
	attendees map[string]struct{}
}

func (c *pastMeetingChildren) hasInvitee(id string) bool {
	_, ok := c.invitees[id]
	return ok
}

func (c *pastMeetingChildren) hasAttendee(id string) bool {
	_, ok := c.attendees[id]
	return ok
}

// fetchPastMeetingChildren loads the invitee and attendee IDs of a past meeting.
// Heimdall authorizes participant writes on the past meeting only, so this is
// what binds a caller-chosen child ID to that authorization. A fetch error is
// returned rather than skipping the check.
func (s *PastMeetingParticipantService) fetchPastMeetingChildren(ctx context.Context, pastMeetingID string) (*pastMeetingChildren, error) {
	ids, err := s.participantClient.GetPastMeetingParticipantIDs(ctx, pastMeetingID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to fetch past meeting participants",
			"past_meeting_id", pastMeetingID,
			logging.ErrKey, err)
		return nil, err
	}
	children := &pastMeetingChildren{
		invitees:  make(map[string]struct{}),
		attendees: make(map[string]struct{}),
	}
	if ids == nil {
		return children, nil
	}
	for _, invitee := range ids.Invitees {
		if invitee.UUID != "" {
			children.invitees[invitee.UUID] = struct{}{}
		}
	}
	for _, attendee := range ids.Attendees {
		if attendee.ID != "" {
			children.attendees[attendee.ID] = struct{}{}
		}
	}
	return children, nil
}

// deleteInvitee deletes invitee record
func (s *PastMeetingParticipantService) deleteInvitee(
	ctx context.Context,
	pastMeetingID, inviteeID, participantID string,
) error {
	if err := s.participantClient.DeleteInvitee(ctx, pastMeetingID, inviteeID); err != nil {
		slog.ErrorContext(ctx, "failed to delete invitee during update",
			"participant_id", participantID,
			"invitee_id", inviteeID,
			"past_meeting_id", pastMeetingID,
			logging.ErrKey, err)
		return err
	}
	return nil
}

// deleteAttendee deletes attendee record
func (s *PastMeetingParticipantService) deleteAttendee(
	ctx context.Context,
	pastMeetingID, attendeeID, participantID string,
) error {
	if err := s.participantClient.DeleteAttendee(ctx, pastMeetingID, attendeeID); err != nil {
		slog.ErrorContext(ctx, "failed to delete attendee during update",
			"participant_id", participantID,
			"attendee_id", attendeeID,
			"past_meeting_id", pastMeetingID,
			logging.ErrKey, err)
		return err
	}
	return nil
}

// createInviteeFromUpdate creates a new invitee from update request
func (s *PastMeetingParticipantService) createInviteeFromUpdate(
	ctx context.Context,
	pastMeetingID string,
	updateReq *itx.UpdateInviteeRequest,
	updater *itx.User,
) (*itx.InviteeResponse, error) {
	// Convert UpdateInviteeRequest to CreateInviteeRequest
	createReq := &itx.CreateInviteeRequest{
		// Identity fields
		PrimaryEmail: updateReq.PrimaryEmail,
		LFUserID:     updateReq.LFUserID,
		LFSSO:        updateReq.LFSSO,
		// Updatable fields
		FirstName:             updateReq.FirstName,
		LastName:              updateReq.LastName,
		Org:                   updateReq.Org,
		JobTitle:              updateReq.JobTitle,
		CommitteeRole:         updateReq.CommitteeRole,
		CommitteeVotingStatus: updateReq.CommitteeVotingStatus,
		// This path is exercised when an update targets an invitee that doesn't yet
		// exist; stamp created_by since ITX will treat this as a fresh record.
		// updater is resolved once in UpdateParticipant so both the invitee and
		// attendee sides of a single update carry consistent audit stamps.
		CreatedBy: updater,
	}

	resp, err := s.participantClient.CreateInvitee(ctx, pastMeetingID, createReq)
	if err != nil {
		slog.ErrorContext(ctx, "failed to create invitee during update",
			"past_meeting_id", pastMeetingID,
			logging.ErrKey, err)
		return nil, err
	}

	return resp, nil
}

// createAttendeeFromUpdate creates a new attendee from update request
func (s *PastMeetingParticipantService) createAttendeeFromUpdate(
	ctx context.Context,
	pastMeetingID string,
	updateReq *itx.UpdateAttendeeRequest,
	updater *itx.User,
) (*itx.AttendeeResponse, error) {
	// Convert UpdateAttendeeRequest to CreateAttendeeRequest
	createReq := &itx.CreateAttendeeRequest{
		Name:                  updateReq.Name,
		Email:                 updateReq.Email,
		LFUserID:              updateReq.LFUserID,
		LFSSO:                 updateReq.LFSSO,
		Org:                   updateReq.Org,
		JobTitle:              updateReq.JobTitle,
		CommitteeRole:         updateReq.CommitteeRole,
		CommitteeVotingStatus: updateReq.CommitteeVotingStatus,
		IsVerified:            updateReq.IsVerified,
		IsUnknown:             utils.BoolValue(updateReq.IsUnknown),
		IsAIReconciled:        utils.BoolValue(updateReq.IsAIReconciled),
		IsAutoMatched:         utils.BoolValue(updateReq.IsAutoMatched),
		ZoomUserName:          utils.StringValue(updateReq.ZoomUserName),
		MappedInviteeName:     utils.StringValue(updateReq.MappedInviteeName),
		// This path is exercised when an update targets an attendee that doesn't yet
		// exist; stamp created_by since ITX will treat this as a fresh record.
		// updater is resolved once in UpdateParticipant so both the invitee and
		// attendee sides of a single update carry consistent audit stamps.
		CreatedBy: updater,
	}

	resp, err := s.participantClient.CreateAttendee(ctx, pastMeetingID, createReq)
	if err != nil {
		slog.ErrorContext(ctx, "failed to create attendee during update",
			"past_meeting_id", pastMeetingID,
			logging.ErrKey, err)
		return nil, err
	}

	return resp, nil
}

// updateInvitee updates invitee record
func (s *PastMeetingParticipantService) updateInvitee(
	ctx context.Context,
	pastMeetingID, inviteeID, participantID string,
	updateReq *itx.UpdateInviteeRequest,
	updater *itx.User,
) (*itx.InviteeResponse, error) {
	// Stamp updated_by from the requester so ITX overwrites the stored value on the
	// invitee record instead of preserving stale data. updater is resolved once in
	// UpdateParticipant to keep the invitee and attendee sides consistent.
	updateReq.UpdatedBy = updater

	resp, err := s.participantClient.UpdateInvitee(ctx, pastMeetingID, inviteeID, updateReq)
	if err != nil {
		slog.ErrorContext(ctx, "failed to update invitee",
			"participant_id", participantID,
			"invitee_id", inviteeID,
			"past_meeting_id", pastMeetingID,
			logging.ErrKey, err)
		return nil, err
	}

	// resp may be nil if ITX returns 204 No Content
	return resp, nil
}

// updateAttendee updates attendee record
func (s *PastMeetingParticipantService) updateAttendee(
	ctx context.Context,
	pastMeetingID, attendeeID, participantID string,
	updateReq *itx.UpdateAttendeeRequest,
	updater *itx.User,
) (*itx.AttendeeResponse, error) {
	// Stamp updated_by from the requester so ITX overwrites the stored value on the
	// attendee record instead of preserving stale data. updater is resolved once in
	// UpdateParticipant to keep the invitee and attendee sides consistent.
	updateReq.UpdatedBy = updater

	// Snapshot the attendee's state before the update so that, if the post-update
	// refetch below fails, the degraded fallback has a real last-known value for
	// fields the update didn't touch instead of guessing. Best-effort: a failure
	// here just means the degraded fallback (if reached) has less to work with.
	preUpdate, preErr := s.participantClient.GetAttendee(ctx, pastMeetingID, attendeeID)
	if preErr != nil {
		slog.WarnContext(ctx, "failed to snapshot attendee before update",
			"participant_id", participantID,
			"attendee_id", attendeeID,
			"past_meeting_id", pastMeetingID,
			logging.ErrKey, preErr)
	}

	resp, err := s.participantClient.UpdateAttendee(ctx, pastMeetingID, attendeeID, updateReq)
	if err != nil {
		slog.ErrorContext(ctx, "failed to update attendee",
			"participant_id", participantID,
			"attendee_id", attendeeID,
			"past_meeting_id", pastMeetingID,
			logging.ErrKey, err)
		return nil, err
	}

	if resp == nil {
		// ITX returns 204 No Content on a successful update with no response body.
		// Refetch the persisted record so the synchronous response reflects the
		// true stored state. Echoing the request values back would incorrectly
		// collapse omitted pointer fields (e.g. is_unknown left nil to mean
		// "don't change it") to their zero value, falsely reporting a known
		// attendee as unknown or vice versa.
		fetched, getErr := s.participantClient.GetAttendee(ctx, pastMeetingID, attendeeID)
		if getErr != nil {
			// The update itself already succeeded (204 confirms the write). A failed
			// refetch should not fail the overall request, so degrade gracefully.
			// Merge the request's explicitly-set fields onto the pre-update snapshot
			// (when available) so fields the request left nil keep their true
			// last-known value instead of collapsing to the Go zero value.
			slog.WarnContext(ctx, "failed to refetch attendee after 204 update; synthesizing response instead",
				"participant_id", participantID,
				"attendee_id", attendeeID,
				"past_meeting_id", pastMeetingID,
				logging.ErrKey, getErr)
			resp = synthesizeAttendeeResponse(attendeeID, updateReq, preUpdate)
		} else {
			resp = fetched
		}
	}

	return resp, nil
}

// synthesizeAttendeeResponse builds a best-effort AttendeeResponse for the case where
// both the post-update refetch and (possibly) the pre-update snapshot are unavailable.
// Fields explicitly set on updateReq always win; fields left nil (pointer fields) or
// zero-valued (plain scalar fields, since ITX's omitempty means the request never sent
// them) fall back to the pre-update snapshot's true persisted value when one was
// captured, and only collapse to the Go zero value when neither source has data (pre
// is nil).
func synthesizeAttendeeResponse(attendeeID string, updateReq *itx.UpdateAttendeeRequest, pre *itx.AttendeeResponse) *itx.AttendeeResponse {
	resp := &itx.AttendeeResponse{
		ID:                    attendeeID,
		Name:                  updateReq.Name,
		Email:                 updateReq.Email,
		LFUserID:              updateReq.LFUserID,
		LFSSO:                 updateReq.LFSSO,
		Org:                   updateReq.Org,
		JobTitle:              updateReq.JobTitle,
		IsVerified:            updateReq.IsVerified,
		IsUnknown:             utils.BoolValue(updateReq.IsUnknown),
		IsAIReconciled:        utils.BoolValue(updateReq.IsAIReconciled),
		IsAutoMatched:         utils.BoolValue(updateReq.IsAutoMatched),
		ZoomUserName:          utils.StringValue(updateReq.ZoomUserName),
		MappedInviteeName:     utils.StringValue(updateReq.MappedInviteeName),
		CommitteeRole:         updateReq.CommitteeRole,
		CommitteeVotingStatus: updateReq.CommitteeVotingStatus,
	}
	if pre == nil {
		return resp
	}
	if updateReq.Name == "" {
		resp.Name = pre.Name
	}
	if updateReq.Email == "" {
		resp.Email = pre.Email
	}
	if updateReq.LFUserID == "" {
		resp.LFUserID = pre.LFUserID
	}
	if updateReq.LFSSO == "" {
		resp.LFSSO = pre.LFSSO
	}
	if updateReq.Org == "" {
		resp.Org = pre.Org
	}
	if updateReq.JobTitle == "" {
		resp.JobTitle = pre.JobTitle
	}
	if !updateReq.IsVerified {
		resp.IsVerified = pre.IsVerified
	}
	if updateReq.CommitteeRole == "" {
		resp.CommitteeRole = pre.CommitteeRole
	}
	if updateReq.CommitteeVotingStatus == "" {
		resp.CommitteeVotingStatus = pre.CommitteeVotingStatus
	}
	if updateReq.IsUnknown == nil {
		resp.IsUnknown = pre.IsUnknown
	}
	if updateReq.IsAIReconciled == nil {
		resp.IsAIReconciled = pre.IsAIReconciled
	}
	if updateReq.IsAutoMatched == nil {
		resp.IsAutoMatched = pre.IsAutoMatched
	}
	if updateReq.ZoomUserName == nil {
		resp.ZoomUserName = pre.ZoomUserName
	}
	if updateReq.MappedInviteeName == nil {
		resp.MappedInviteeName = pre.MappedInviteeName
	}
	return resp
}

// DeleteParticipant deletes a participant
// Deletes the invitee and/or attendee record that the participant ID resolves
// to, but only those that belong to the past meeting; returns not-found when
// neither does. Returns an error if either deletion fails.
func (s *PastMeetingParticipantService) DeleteParticipant(
	ctx context.Context,
	pastMeetingID, participantID string,
) error {
	children, err := s.fetchPastMeetingChildren(ctx, pastMeetingID)
	if err != nil {
		return err
	}

	// Map the V2 participant ID to invitee and attendee IDs; with no mapping the
	// participant ID may be the invitee or attendee ID itself. A failed lookup
	// is returned rather than reported as not found.
	idToUseInvitee, _, err := s.checkInviteeExists(ctx, participantID)
	if err != nil {
		return err
	}
	idToUseAttendee, _, err := s.checkAttendeeExists(ctx, participantID)
	if err != nil {
		return err
	}

	deleteInvitee := children.hasInvitee(idToUseInvitee)
	deleteAttendee := children.hasAttendee(idToUseAttendee)
	if !deleteInvitee && !deleteAttendee {
		slog.WarnContext(ctx, "participant does not belong to the past meeting; rejecting delete",
			"participant_id", participantID,
			"past_meeting_id", pastMeetingID)
		return domain.NewNotFoundError("participant not found in past meeting")
	}

	var inviteeErr, attendeeErr error
	if deleteInvitee {
		inviteeErr = s.participantClient.DeleteInvitee(ctx, pastMeetingID, idToUseInvitee)
		if inviteeErr != nil {
			slog.WarnContext(ctx, "failed to delete invitee",
				"participant_id", participantID,
				"invitee_id", idToUseInvitee,
				"past_meeting_id", pastMeetingID,
				logging.ErrKey, inviteeErr)
		}
	}

	if deleteAttendee {
		attendeeErr = s.participantClient.DeleteAttendee(ctx, pastMeetingID, idToUseAttendee)
		if attendeeErr != nil {
			slog.WarnContext(ctx, "failed to delete attendee",
				"participant_id", participantID,
				"attendee_id", idToUseAttendee,
				"past_meeting_id", pastMeetingID,
				logging.ErrKey, attendeeErr)
		}
	}

	// Return error if either deletion failed
	if inviteeErr != nil && attendeeErr != nil {
		return fmt.Errorf("failed to delete invitee: %w, and failed to delete attendee: %v", inviteeErr, attendeeErr)
	}
	if inviteeErr != nil {
		return fmt.Errorf("failed to delete invitee: %w", inviteeErr)
	}
	if attendeeErr != nil {
		return fmt.Errorf("failed to delete attendee: %w", attendeeErr)
	}

	return nil
}

// mergeParticipantResponses merges invitee and attendee responses into a unified participant
// Prioritizes user data from invitee if present, otherwise uses attendee data
func mergeParticipantResponses(
	pastMeetingID string,
	invitee *itx.InviteeResponse,
	attendee *itx.AttendeeResponse,
	isInvited, isAttended bool,
) *ParticipantResponse {
	unified := &ParticipantResponse{
		IsInvited:  isInvited,
		IsAttended: isAttended,
	}

	// Set past meeting ID and extract meeting ID from it
	unified.PastMeetingID = pastMeetingID
	meetingID, _ := utils.ParsePastMeetingID(pastMeetingID)
	unified.MeetingID = meetingID

	// Set IDs
	if invitee != nil {
		unified.InviteeID = invitee.UUID
	}
	if attendee != nil {
		unified.AttendeeID = attendee.ID
	}

	// Prioritize user data from invitee
	if invitee != nil {
		unified.FirstName = invitee.FirstName
		unified.LastName = invitee.LastName
		unified.Email = invitee.PrimaryEmail
		unified.Username = invitee.LFSSO
		unified.LFUserID = invitee.LFUserID
		unified.OrgName = invitee.Org
		unified.JobTitle = invitee.JobTitle
		unified.AvatarURL = invitee.ProfilePicture
		unified.OrgIsMember = invitee.OrgIsMember
		unified.OrgIsProjectMember = invitee.OrgIsProjectMember
		unified.CommitteeID = invitee.CommitteeID
		unified.CommitteeRole = invitee.CommitteeRole
		unified.IsCommitteeMember = invitee.IsCommitteeMember
		unified.CommitteeVotingStatus = invitee.CommitteeVotingStatus
		unified.CreatedAt = invitee.CreatedAt
		unified.CreatedBy = invitee.CreatedBy
		unified.ModifiedAt = invitee.ModifiedAt
		unified.ModifiedBy = invitee.UpdatedBy
	} else if attendee != nil {
		// Fallback to attendee data if no invitee
		// Attendee has full name, not split first/last
		unified.FirstName = attendee.Name
		unified.LastName = ""
		unified.Email = attendee.Email
		unified.Username = attendee.LFSSO
		unified.LFUserID = attendee.LFUserID
		unified.OrgName = attendee.Org
		unified.JobTitle = attendee.JobTitle
		unified.AvatarURL = attendee.ProfilePicture
		unified.OrgIsMember = attendee.OrgIsMember
		unified.OrgIsProjectMember = attendee.OrgIsProjectMember
		unified.CommitteeID = attendee.CommitteeID
		unified.CommitteeRole = attendee.CommitteeRole
		unified.IsCommitteeMember = attendee.IsCommitteeMember
		unified.CommitteeVotingStatus = attendee.CommitteeVotingStatus
	}

	// Add attendee-specific fields if attendee exists
	if attendee != nil {
		unified.IsVerified = attendee.IsVerified
		unified.IsUnknown = attendee.IsUnknown
		unified.IsAIReconciled = attendee.IsAIReconciled
		unified.IsAutoMatched = attendee.IsAutoMatched
		unified.ZoomUserName = attendee.ZoomUserName
		unified.MappedInviteeName = attendee.MappedInviteeName
		unified.AverageAttendance = attendee.AverageAttendance
		unified.Sessions = attendee.Sessions
	}

	return unified
}
