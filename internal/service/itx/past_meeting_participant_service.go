// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package itx

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/models/itx"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/utils"
)

// PastMeetingParticipantService handles unified participant operations by routing to invitee/attendee endpoints
type PastMeetingParticipantService struct {
	participantClient domain.ITXPastMeetingParticipantClient
	idMapper          domain.IDMapper
}

// NewPastMeetingParticipantService creates a new participant service
func NewPastMeetingParticipantService(participantClient domain.ITXPastMeetingParticipantClient, idMapper domain.IDMapper) *PastMeetingParticipantService {
	return &PastMeetingParticipantService{
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

	// Create invitee if requested
	if isInvited {
		resp, err := s.participantClient.CreateInvitee(ctx, pastMeetingID, inviteeReq)
		if err != nil {
			return nil, fmt.Errorf("failed to create invitee: %w", err)
		}
		inviteeResp = resp
	}

	// Create attendee if requested
	if isAttended {
		resp, err := s.participantClient.CreateAttendee(ctx, pastMeetingID, attendeeReq)
		if err != nil {
			// If invitee was created but attendee fails, we have a partial state
			// ITX handles cleanup, but we return the error
			return nil, fmt.Errorf("failed to create attendee: %w", err)
		}
		attendeeResp = resp
	}

	// Merge into unified response
	return mergeParticipantResponses(pastMeetingID, inviteeResp, attendeeResp, isInvited, isAttended), nil
}

// UpdateParticipant updates a participant
// Handles creating, updating, and deleting invitee and attendee records based on status flags
func (s *PastMeetingParticipantService) UpdateParticipant(
	ctx context.Context,
	pastMeetingID, participantID string,
	isInvited, isAttended *bool,
	inviteeReq *itx.UpdateInviteeRequest,
	attendeeReq *itx.UpdateAttendeeRequest,
) (*ParticipantResponse, error) {
	// Handle invitee operations
	inviteeResp, inviteeExists := s.handleInviteeOperation(ctx, pastMeetingID, participantID, isInvited, inviteeReq)

	// Handle attendee operations
	attendeeResp, attendeeExists := s.handleAttendeeOperation(ctx, pastMeetingID, participantID, isAttended, attendeeReq)

	// Merge into unified response
	return mergeParticipantResponses(pastMeetingID, inviteeResp, attendeeResp, inviteeExists, attendeeExists), nil
}

// handleInviteeOperation handles CRUD operations for invitee based on is_invited flag
// Returns invitee response and whether invitee exists after the operation
func (s *PastMeetingParticipantService) handleInviteeOperation(
	ctx context.Context,
	pastMeetingID, participantID string,
	isInvited *bool,
	inviteeReq *itx.UpdateInviteeRequest,
) (*itx.InviteeResponse, bool) {
	// If is_invited is not set, no operation needed
	if isInvited == nil {
		return nil, false
	}

	// Check if invitee exists by attempting ID mapping
	inviteeID, inviteeExists := s.checkInviteeExists(ctx, participantID)

	// Determine operation based on flag and existence
	if !*isInvited {
		// Delete if is_invited=false and exists
		if inviteeExists {
			s.deleteInvitee(ctx, pastMeetingID, inviteeID, participantID)
		}
		return nil, false
	}

	// is_invited=true: create or update
	if !inviteeExists && inviteeReq != nil {
		// Create new invitee
		return s.createInviteeFromUpdate(ctx, pastMeetingID, inviteeReq), true
	}

	// Update existing invitee
	if inviteeExists && inviteeReq != nil {
		return s.updateInvitee(ctx, pastMeetingID, inviteeID, participantID, inviteeReq), true
	}

	return nil, inviteeExists
}

// handleAttendeeOperation handles CRUD operations for attendee based on is_attended flag
// Returns attendee response and whether attendee exists after the operation
func (s *PastMeetingParticipantService) handleAttendeeOperation(
	ctx context.Context,
	pastMeetingID, participantID string,
	isAttended *bool,
	attendeeReq *itx.UpdateAttendeeRequest,
) (*itx.AttendeeResponse, bool) {
	// If is_attended is not set, no operation needed
	if isAttended == nil {
		return nil, false
	}

	// Check if attendee exists by attempting ID mapping
	attendeeID, attendeeExists := s.checkAttendeeExists(ctx, participantID)

	// Determine operation based on flag and existence
	if !*isAttended {
		// Delete if is_attended=false and exists
		if attendeeExists {
			s.deleteAttendee(ctx, pastMeetingID, attendeeID, participantID)
		}
		return nil, false
	}

	// is_attended=true: create or update
	if !attendeeExists && attendeeReq != nil {
		// Create new attendee
		return s.createAttendeeFromUpdate(ctx, pastMeetingID, attendeeReq), true
	}

	// Update existing attendee
	if attendeeExists && attendeeReq != nil {
		return s.updateAttendee(ctx, pastMeetingID, attendeeID, participantID, attendeeReq), true
	}

	return nil, attendeeExists
}

// checkInviteeExists checks if invitee exists by attempting ID mapping
// Returns invitee ID and existence flag
func (s *PastMeetingParticipantService) checkInviteeExists(ctx context.Context, participantID string) (string, bool) {
	inviteeID, err := s.idMapper.MapParticipantV2ToInviteeID(ctx, participantID)
	if err != nil || inviteeID == "" {
		slog.DebugContext(ctx, "Invitee does not exist (ID mapping failed or empty)",
			"participant_id", participantID,
			"error", err)
		return participantID, false
	}

	slog.DebugContext(ctx, "Invitee exists - mapped participant ID to invitee ID",
		"participant_id", participantID,
		"invitee_id", inviteeID)
	return inviteeID, true
}

// checkAttendeeExists checks if attendee exists by attempting ID mapping
// Returns attendee ID and existence flag
func (s *PastMeetingParticipantService) checkAttendeeExists(ctx context.Context, participantID string) (string, bool) {
	attendeeID, err := s.idMapper.MapParticipantV2ToAttendeeID(ctx, participantID)
	if err != nil || attendeeID == "" {
		slog.DebugContext(ctx, "Attendee does not exist (ID mapping failed or empty)",
			"participant_id", participantID,
			"error", err)
		return participantID, false
	}

	slog.DebugContext(ctx, "Attendee exists - mapped participant ID to attendee ID",
		"participant_id", participantID,
		"attendee_id", attendeeID)
	return attendeeID, true
}

// deleteInvitee deletes invitee record
func (s *PastMeetingParticipantService) deleteInvitee(
	ctx context.Context,
	pastMeetingID, inviteeID, participantID string,
) {
	if err := s.participantClient.DeleteInvitee(ctx, pastMeetingID, inviteeID); err != nil {
		slog.WarnContext(ctx, "Failed to delete invitee during update",
			"participant_id", participantID,
			"invitee_id", inviteeID,
			"past_meeting_id", pastMeetingID,
			"error", err)
	}
}

// deleteAttendee deletes attendee record
func (s *PastMeetingParticipantService) deleteAttendee(
	ctx context.Context,
	pastMeetingID, attendeeID, participantID string,
) {
	if err := s.participantClient.DeleteAttendee(ctx, pastMeetingID, attendeeID); err != nil {
		slog.WarnContext(ctx, "Failed to delete attendee during update",
			"participant_id", participantID,
			"attendee_id", attendeeID,
			"past_meeting_id", pastMeetingID,
			"error", err)
	}
}

// createInviteeFromUpdate creates a new invitee from update request
func (s *PastMeetingParticipantService) createInviteeFromUpdate(
	ctx context.Context,
	pastMeetingID string,
	updateReq *itx.UpdateInviteeRequest,
) *itx.InviteeResponse {
	// Convert UpdateInviteeRequest to CreateInviteeRequest
	createReq := &itx.CreateInviteeRequest{
		// Identity fields
		PrimaryEmail:          updateReq.PrimaryEmail,
		LFUserID:              updateReq.LFUserID,
		LFSSO:                 updateReq.LFSSO,
		// Updatable fields
		FirstName:             updateReq.FirstName,
		LastName:              updateReq.LastName,
		Org:                   updateReq.Org,
		JobTitle:              updateReq.JobTitle,
		CommitteeRole:         updateReq.CommitteeRole,
		CommitteeVotingStatus: updateReq.CommitteeVotingStatus,
	}

	resp, err := s.participantClient.CreateInvitee(ctx, pastMeetingID, createReq)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to create invitee during update",
			"past_meeting_id", pastMeetingID,
			"error", err)
		return nil
	}

	return resp
}

// createAttendeeFromUpdate creates a new attendee from update request
func (s *PastMeetingParticipantService) createAttendeeFromUpdate(
	ctx context.Context,
	pastMeetingID string,
	updateReq *itx.UpdateAttendeeRequest,
) *itx.AttendeeResponse {
	// Convert UpdateAttendeeRequest to CreateAttendeeRequest
	createReq := &itx.CreateAttendeeRequest{
		Org:                   updateReq.Org,
		JobTitle:              updateReq.JobTitle,
		CommitteeRole:         updateReq.CommitteeRole,
		CommitteeVotingStatus: updateReq.CommitteeVotingStatus,
		IsVerified:            updateReq.IsVerified,
	}

	resp, err := s.participantClient.CreateAttendee(ctx, pastMeetingID, createReq)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to create attendee during update",
			"past_meeting_id", pastMeetingID,
			"error", err)
		return nil
	}

	return resp
}

// updateInvitee updates invitee record
func (s *PastMeetingParticipantService) updateInvitee(
	ctx context.Context,
	pastMeetingID, inviteeID, participantID string,
	updateReq *itx.UpdateInviteeRequest,
) *itx.InviteeResponse {
	resp, err := s.participantClient.UpdateInvitee(ctx, pastMeetingID, inviteeID, updateReq)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to update invitee",
			"participant_id", participantID,
			"invitee_id", inviteeID,
			"past_meeting_id", pastMeetingID,
			"error", err)
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
) *itx.AttendeeResponse {
	resp, err := s.participantClient.UpdateAttendee(ctx, pastMeetingID, attendeeID, updateReq)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to update attendee",
			"participant_id", participantID,
			"attendee_id", attendeeID,
			"past_meeting_id", pastMeetingID,
			"error", err)
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
		slog.ErrorContext(ctx, "Failed to delete invitee",
			"participant_id", participantID,
			"invitee_id", idToUseInvitee,
			"past_meeting_id", pastMeetingID,
			"error", inviteeErr)
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
		slog.ErrorContext(ctx, "Failed to delete attendee",
			"participant_id", participantID,
			"attendee_id", idToUseAttendee,
			"past_meeting_id", pastMeetingID,
			"error", attendeeErr)
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
		unified.AverageAttendance = attendee.AverageAttendance
		unified.Sessions = attendee.Sessions
	}

	return unified
}
