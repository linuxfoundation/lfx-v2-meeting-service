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

	inviteeResp, inviteeExists := s.handleInviteeOperation(ctx, p.PastMeetingID, p.ParticipantID, p.InviteeID, p.IsInvited, inviteeReq, updater)
	attendeeResp, attendeeExists := s.handleAttendeeOperation(ctx, p.PastMeetingID, p.ParticipantID, p.AttendeeID, p.IsAttended, attendeeReq, updater)
	return mergeParticipantResponses(p.PastMeetingID, inviteeResp, attendeeResp, inviteeExists, attendeeExists), nil
}

func (s *PastMeetingParticipantService) handleInviteeOperation(
	ctx context.Context,
	pastMeetingID, participantID, inviteeID string,
	isInvited *bool,
	inviteeReq *itx.UpdateInviteeRequest,
	updater *itx.User,
) (*itx.InviteeResponse, bool) {
	if isInvited == nil {
		return nil, false
	}

	var actualInviteeID string
	var inviteeExists bool

	if inviteeID != "" {
		inviteeExists = s.checkInviteeExistsFromInviteeID(ctx, inviteeID)
		if inviteeExists {
			actualInviteeID = inviteeID
		}
	} else {
		actualInviteeID, inviteeExists = s.checkInviteeExists(ctx, participantID)
	}

	if !*isInvited {
		if inviteeExists && actualInviteeID != "" {
			s.deleteInvitee(ctx, pastMeetingID, actualInviteeID, participantID)
		}
		return nil, false
	}

	if !inviteeExists && inviteeReq != nil {
		return s.createInviteeFromUpdate(ctx, pastMeetingID, inviteeReq, updater), true
	}

	if inviteeExists && inviteeReq != nil && actualInviteeID != "" {
		return s.updateInvitee(ctx, pastMeetingID, actualInviteeID, participantID, inviteeReq, updater), true
	}

	return nil, inviteeExists
}

func (s *PastMeetingParticipantService) handleAttendeeOperation(
	ctx context.Context,
	pastMeetingID, participantID, attendeeID string,
	isAttended *bool,
	attendeeReq *itx.UpdateAttendeeRequest,
	updater *itx.User,
) (*itx.AttendeeResponse, bool) {
	if isAttended == nil {
		return nil, false
	}

	var actualAttendeeID string
	var attendeeExists bool

	if attendeeID != "" {
		attendeeExists = s.checkAttendeeExistsFromAttendeeID(ctx, attendeeID)
		if attendeeExists {
			actualAttendeeID = attendeeID
		}
	} else {
		actualAttendeeID, attendeeExists = s.checkAttendeeExists(ctx, participantID)
	}

	if !*isAttended {
		if attendeeExists && actualAttendeeID != "" {
			s.deleteAttendee(ctx, pastMeetingID, actualAttendeeID, participantID)
		}
		return nil, false
	}

	if !attendeeExists && attendeeReq != nil {
		return s.createAttendeeFromUpdate(ctx, pastMeetingID, attendeeReq, updater), true
	}

	if attendeeExists && attendeeReq != nil && actualAttendeeID != "" {
		return s.updateAttendee(ctx, pastMeetingID, actualAttendeeID, participantID, attendeeReq, updater), true
	}

	return nil, attendeeExists
}

// checkInviteeExists checks if invitee exists by attempting ID mapping
// Returns invitee ID and existence flag
func (s *PastMeetingParticipantService) checkInviteeExists(ctx context.Context, participantID string) (string, bool) {
	inviteeID, err := s.idMapper.MapParticipantV2ToInviteeID(ctx, participantID)
	if err != nil || inviteeID == "" {
		slog.DebugContext(ctx, "invitee does not exist (ID mapping failed or empty)",
			"participant_id", participantID,
			logging.ErrKey, err)
		return participantID, false
	}

	slog.DebugContext(ctx, "invitee exists - mapped participant ID to invitee ID",
		"participant_id", participantID,
		"invitee_id", inviteeID)
	return inviteeID, true
}

func (s *PastMeetingParticipantService) checkAttendeeExists(ctx context.Context, participantID string) (string, bool) {
	attendeeID, err := s.idMapper.MapParticipantV2ToAttendeeID(ctx, participantID)
	if err != nil || attendeeID == "" {
		slog.DebugContext(ctx, "attendee does not exist (ID mapping failed or empty)",
			"participant_id", participantID,
			logging.ErrKey, err)
		return participantID, false
	}

	slog.DebugContext(ctx, "attendee exists - mapped participant ID to attendee ID",
		"participant_id", participantID,
		"attendee_id", attendeeID)
	return attendeeID, true
}

func (s *PastMeetingParticipantService) checkInviteeExistsFromInviteeID(ctx context.Context, inviteeID string) bool {
	inviteeID, err := s.idMapper.MapInviteeIDToParticipantV2(ctx, inviteeID)
	exists := inviteeID != "" && err == nil
	slog.DebugContext(ctx, "checked invitee existence from invitee ID",
		"invitee_id", inviteeID,
		"exists", exists,
		logging.ErrKey, err)
	return exists
}

func (s *PastMeetingParticipantService) checkAttendeeExistsFromAttendeeID(ctx context.Context, attendeeID string) bool {
	attendeeID, err := s.idMapper.MapAttendeeIDToParticipantV2(ctx, attendeeID)
	exists := attendeeID != "" && err == nil
	slog.DebugContext(ctx, "checked attendee existence from attendee ID",
		"attendee_id", attendeeID,
		"exists", exists,
		logging.ErrKey, err)
	return exists
}

// deleteInvitee deletes invitee record
func (s *PastMeetingParticipantService) deleteInvitee(
	ctx context.Context,
	pastMeetingID, inviteeID, participantID string,
) {
	if err := s.participantClient.DeleteInvitee(ctx, pastMeetingID, inviteeID); err != nil {
		slog.WarnContext(ctx, "failed to delete invitee during update",
			"participant_id", participantID,
			"invitee_id", inviteeID,
			"past_meeting_id", pastMeetingID,
			logging.ErrKey, err)
	}
}

// deleteAttendee deletes attendee record
func (s *PastMeetingParticipantService) deleteAttendee(
	ctx context.Context,
	pastMeetingID, attendeeID, participantID string,
) {
	if err := s.participantClient.DeleteAttendee(ctx, pastMeetingID, attendeeID); err != nil {
		slog.WarnContext(ctx, "failed to delete attendee during update",
			"participant_id", participantID,
			"attendee_id", attendeeID,
			"past_meeting_id", pastMeetingID,
			logging.ErrKey, err)
	}
}

// createInviteeFromUpdate creates a new invitee from update request
func (s *PastMeetingParticipantService) createInviteeFromUpdate(
	ctx context.Context,
	pastMeetingID string,
	updateReq *itx.UpdateInviteeRequest,
	updater *itx.User,
) *itx.InviteeResponse {
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
		return nil
	}

	return resp
}

// createAttendeeFromUpdate creates a new attendee from update request
func (s *PastMeetingParticipantService) createAttendeeFromUpdate(
	ctx context.Context,
	pastMeetingID string,
	updateReq *itx.UpdateAttendeeRequest,
	updater *itx.User,
) *itx.AttendeeResponse {
	// Convert UpdateAttendeeRequest to CreateAttendeeRequest
	createReq := &itx.CreateAttendeeRequest{
		Org:                   updateReq.Org,
		JobTitle:              updateReq.JobTitle,
		CommitteeRole:         updateReq.CommitteeRole,
		CommitteeVotingStatus: updateReq.CommitteeVotingStatus,
		IsVerified:            updateReq.IsVerified,
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
		return nil
	}

	return resp
}

// updateInvitee updates invitee record
func (s *PastMeetingParticipantService) updateInvitee(
	ctx context.Context,
	pastMeetingID, inviteeID, participantID string,
	updateReq *itx.UpdateInviteeRequest,
	updater *itx.User,
) *itx.InviteeResponse {
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
		return nil
	}

	// resp may be nil if ITX returns 204 No Content
	return resp
}

// updateAttendee updates attendee record
func (s *PastMeetingParticipantService) updateAttendee(
	ctx context.Context,
	pastMeetingID, attendeeID, participantID string,
	updateReq *itx.UpdateAttendeeRequest,
	updater *itx.User,
) *itx.AttendeeResponse {
	// Stamp updated_by from the requester so ITX overwrites the stored value on the
	// attendee record instead of preserving stale data. updater is resolved once in
	// UpdateParticipant to keep the invitee and attendee sides consistent.
	updateReq.UpdatedBy = updater

	resp, err := s.participantClient.UpdateAttendee(ctx, pastMeetingID, attendeeID, updateReq)
	if err != nil {
		slog.ErrorContext(ctx, "failed to update attendee",
			"participant_id", participantID,
			"attendee_id", attendeeID,
			"past_meeting_id", pastMeetingID,
			logging.ErrKey, err)
		return nil
	}

	// resp may be nil if ITX returns 204 No Content
	return resp
}

// DeleteParticipant deletes a participant
// Attempts to delete both invitee and attendee records
// Returns an error if either deletion fails
func (s *PastMeetingParticipantService) DeleteParticipant(
	ctx context.Context,
	pastMeetingID, participantID string,
) error {
	// Try to map V2 participant ID to invitee ID
	inviteeID, inviteeMappingErr := s.idMapper.MapParticipantV2ToInviteeID(ctx, participantID)

	// Try to delete as invitee
	idToUseInvitee := participantID
	if inviteeMappingErr == nil && inviteeID != "" {
		idToUseInvitee = inviteeID
	}

	inviteeErr := s.participantClient.DeleteInvitee(ctx, pastMeetingID, idToUseInvitee)
	if inviteeErr != nil {
		slog.WarnContext(ctx, "failed to delete invitee",
			"participant_id", participantID,
			"invitee_id", idToUseInvitee,
			"past_meeting_id", pastMeetingID,
			logging.ErrKey, inviteeErr)
	}

	// Try to map V2 participant ID to attendee ID
	attendeeID, attendeeMappingErr := s.idMapper.MapParticipantV2ToAttendeeID(ctx, participantID)

	// Try to delete as attendee
	idToUseAttendee := participantID
	if attendeeMappingErr == nil && attendeeID != "" {
		idToUseAttendee = attendeeID
	}

	attendeeErr := s.participantClient.DeleteAttendee(ctx, pastMeetingID, idToUseAttendee)
	if attendeeErr != nil {
		slog.WarnContext(ctx, "failed to delete attendee",
			"participant_id", participantID,
			"attendee_id", idToUseAttendee,
			"past_meeting_id", pastMeetingID,
			logging.ErrKey, attendeeErr)
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
