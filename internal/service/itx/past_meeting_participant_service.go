// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package itx

import (
	"context"
	"fmt"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/models/itx"
)

// PastMeetingParticipantService handles unified participant operations by routing to invitee/attendee endpoints
type PastMeetingParticipantService struct {
	participantClient domain.ITXPastMeetingParticipantClient
}

// NewPastMeetingParticipantService creates a new participant service
func NewPastMeetingParticipantService(participantClient domain.ITXPastMeetingParticipantClient) *PastMeetingParticipantService {
	return &PastMeetingParticipantService{
		participantClient: participantClient,
	}
}

// ParticipantResponse represents a cohesive participant combining invitee and attendee data
type ParticipantResponse struct {
	// IDs
	InviteeID  string // Present if is_invited=true
	AttendeeID string // Present if is_attended=true

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
	return mergeParticipantResponses(inviteeResp, attendeeResp, isInvited, isAttended), nil
}

// UpdateParticipant updates a participant
// The participantID could be an invitee_id or attendee_id
// We attempt to update as an invitee first, then as an attendee if that fails
func (s *PastMeetingParticipantService) UpdateParticipant(
	ctx context.Context,
	pastMeetingID, participantID string,
	inviteeReq *itx.UpdateInviteeRequest,
	attendeeReq *itx.UpdateAttendeeRequest,
) (*ParticipantResponse, error) {
	var inviteeResp *itx.InviteeResponse
	var attendeeResp *itx.AttendeeResponse
	var inviteeErr, attendeeErr error

	// Try to update as invitee
	if inviteeReq != nil {
		resp, err := s.participantClient.UpdateInvitee(ctx, pastMeetingID, participantID, inviteeReq)
		if err != nil {
			inviteeErr = err
		} else {
			inviteeResp = resp
		}
	}

	// Try to update as attendee
	if attendeeReq != nil {
		resp, err := s.participantClient.UpdateAttendee(ctx, pastMeetingID, participantID, attendeeReq)
		if err != nil {
			attendeeErr = err
		} else {
			attendeeResp = resp
		}
	}

	// If both failed, return an error
	if inviteeResp == nil && attendeeResp == nil {
		if inviteeErr != nil {
			return nil, fmt.Errorf("failed to update as invitee: %w", inviteeErr)
		}
		if attendeeErr != nil {
			return nil, fmt.Errorf("failed to update as attendee: %w", attendeeErr)
		}
		return nil, domain.NewValidationError("no update requests provided")
	}

	// Merge into unified response
	isInvited := inviteeResp != nil
	isAttended := attendeeResp != nil
	return mergeParticipantResponses(inviteeResp, attendeeResp, isInvited, isAttended), nil
}

// DeleteParticipant deletes a participant
// The participantID could be an invitee_id or attendee_id
// We attempt to delete as an invitee first, then as an attendee if that fails
func (s *PastMeetingParticipantService) DeleteParticipant(
	ctx context.Context,
	pastMeetingID, participantID string,
) error {
	// Try to delete as invitee
	inviteeErr := s.participantClient.DeleteInvitee(ctx, pastMeetingID, participantID)
	if inviteeErr == nil {
		// Successfully deleted as invitee
		return nil
	}

	// Try to delete as attendee
	attendeeErr := s.participantClient.DeleteAttendee(ctx, pastMeetingID, participantID)
	if attendeeErr == nil {
		// Successfully deleted as attendee
		return nil
	}

	// Both failed - return the invitee error (likely a 404 not found)
	return inviteeErr
}

// mergeParticipantResponses merges invitee and attendee responses into a unified participant
// Prioritizes user data from invitee if present, otherwise uses attendee data
func mergeParticipantResponses(
	invitee *itx.InviteeResponse,
	attendee *itx.AttendeeResponse,
	isInvited, isAttended bool,
) *ParticipantResponse {
	unified := &ParticipantResponse{
		IsInvited:  isInvited,
		IsAttended: isAttended,
	}

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
