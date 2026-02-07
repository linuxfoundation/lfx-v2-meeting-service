// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"fmt"
	"strings"

	meetingservice "github.com/linuxfoundation/lfx-v2-meeting-service/gen/meeting_service"
	itxservice "github.com/linuxfoundation/lfx-v2-meeting-service/internal/service/itx"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/models/itx"
)

// ConvertCreateParticipantPayload converts Goa create participant payload to ITX invitee and attendee requests
func ConvertCreateParticipantPayload(payload *meetingservice.CreateItxPastMeetingParticipantPayload) (*itx.CreateInviteeRequest, *itx.CreateAttendeeRequest) {
	var inviteeReq *itx.CreateInviteeRequest
	var attendeeReq *itx.CreateAttendeeRequest

	// Create invitee request if needed
	isInvited := payload.IsInvited != nil && *payload.IsInvited
	if isInvited {
		inviteeReq = &itx.CreateInviteeRequest{}

		if payload.FirstName != nil {
			inviteeReq.FirstName = *payload.FirstName
		}
		if payload.LastName != nil {
			inviteeReq.LastName = *payload.LastName
		}
		if payload.Email != nil {
			inviteeReq.PrimaryEmail = *payload.Email
		}
		if payload.Username != nil {
			inviteeReq.LFSSO = *payload.Username
		}
		if payload.LfUserID != nil {
			inviteeReq.LFUserID = *payload.LfUserID
		}
		if payload.OrgName != nil {
			inviteeReq.Org = *payload.OrgName
		}
		if payload.JobTitle != nil {
			inviteeReq.JobTitle = *payload.JobTitle
		}
		if payload.AvatarURL != nil {
			inviteeReq.ProfilePicture = *payload.AvatarURL
		}
		if payload.CommitteeID != nil {
			inviteeReq.CommitteeID = *payload.CommitteeID
		}
		if payload.CommitteeRole != nil {
			inviteeReq.CommitteeRole = *payload.CommitteeRole
		}
		if payload.CommitteeVotingStatus != nil {
			inviteeReq.CommitteeVotingStatus = *payload.CommitteeVotingStatus
		}
		if payload.OrgIsMember != nil {
			inviteeReq.OrgIsMember = *payload.OrgIsMember
		}
		if payload.OrgIsProjectMember != nil {
			inviteeReq.OrgIsProjectMember = *payload.OrgIsProjectMember
		}
	}

	// Create attendee request if needed
	isAttended := payload.IsAttended != nil && *payload.IsAttended
	if isAttended {
		attendeeReq = &itx.CreateAttendeeRequest{}

		// Attendee uses full name instead of first/last
		var name string
		if payload.FirstName != nil && payload.LastName != nil {
			name = strings.TrimSpace(fmt.Sprintf("%s %s", *payload.FirstName, *payload.LastName))
		} else if payload.FirstName != nil {
			name = *payload.FirstName
		} else if payload.LastName != nil {
			name = *payload.LastName
		}
		if name != "" {
			attendeeReq.Name = name
		}

		if payload.Email != nil {
			attendeeReq.Email = *payload.Email
		}
		if payload.Username != nil {
			attendeeReq.LFSSO = *payload.Username
		}
		if payload.LfUserID != nil {
			attendeeReq.LFUserID = *payload.LfUserID
		}
		if payload.OrgName != nil {
			attendeeReq.Org = *payload.OrgName
		}
		if payload.JobTitle != nil {
			attendeeReq.JobTitle = *payload.JobTitle
		}
		if payload.AvatarURL != nil {
			attendeeReq.ProfilePicture = *payload.AvatarURL
		}
		if payload.CommitteeID != nil {
			attendeeReq.CommitteeID = *payload.CommitteeID
		}
		if payload.CommitteeRole != nil {
			attendeeReq.CommitteeRole = *payload.CommitteeRole
		}
		if payload.CommitteeVotingStatus != nil {
			attendeeReq.CommitteeVotingStatus = *payload.CommitteeVotingStatus
		}
		if payload.OrgIsMember != nil {
			attendeeReq.OrgIsMember = *payload.OrgIsMember
		}
		if payload.OrgIsProjectMember != nil {
			attendeeReq.OrgIsProjectMember = *payload.OrgIsProjectMember
		}
		if payload.IsVerified != nil {
			attendeeReq.IsVerified = *payload.IsVerified
		}
		if payload.IsUnknown != nil {
			attendeeReq.IsUnknown = *payload.IsUnknown
		}

		// Convert sessions
		if payload.Sessions != nil {
			attendeeReq.Sessions = make([]itx.AttendeeSession, len(payload.Sessions))
			for i, s := range payload.Sessions {
				attendeeReq.Sessions[i] = itx.AttendeeSession{
					ParticipantUUID: ptrToString(s.ParticipantUUID),
					JoinTime:        ptrToString(s.JoinTime),
					LeaveTime:       ptrToString(s.LeaveTime),
					LeaveReason:     ptrToString(s.LeaveReason),
				}
			}
		}
	}

	return inviteeReq, attendeeReq
}

// ConvertUpdateParticipantPayload converts Goa update participant payload to ITX invitee and attendee update requests
func ConvertUpdateParticipantPayload(payload *meetingservice.UpdateItxPastMeetingParticipantPayload) (*itx.UpdateInviteeRequest, *itx.UpdateAttendeeRequest) {
	var inviteeReq *itx.UpdateInviteeRequest
	var attendeeReq *itx.UpdateAttendeeRequest

	// Check if any invitee-updatable fields are present
	hasInviteeFields := payload.FirstName != nil || payload.LastName != nil ||
		payload.OrgName != nil || payload.JobTitle != nil ||
		payload.CommitteeRole != nil || payload.CommitteeVotingStatus != nil ||
		payload.Email != nil || payload.LfUserID != nil || payload.Username != nil

	if hasInviteeFields {
		inviteeReq = &itx.UpdateInviteeRequest{}

		// Identity fields (used for creating invitee if it doesn't exist)
		if payload.Email != nil {
			inviteeReq.PrimaryEmail = *payload.Email
		}
		if payload.LfUserID != nil {
			inviteeReq.LFUserID = *payload.LfUserID
		}
		if payload.Username != nil {
			inviteeReq.LFSSO = *payload.Username
		}

		// Updatable fields
		// FirstName and LastName are required by ITX API
		if payload.FirstName != nil {
			inviteeReq.FirstName = *payload.FirstName
		}
		if payload.LastName != nil {
			inviteeReq.LastName = *payload.LastName
		}
		if payload.OrgName != nil {
			inviteeReq.Org = *payload.OrgName
		}
		if payload.JobTitle != nil {
			inviteeReq.JobTitle = *payload.JobTitle
		}
		if payload.CommitteeRole != nil {
			inviteeReq.CommitteeRole = *payload.CommitteeRole
		}
		if payload.CommitteeVotingStatus != nil {
			inviteeReq.CommitteeVotingStatus = *payload.CommitteeVotingStatus
		}
	}

	// Check if any attendee-updatable fields are present
	hasAttendeeFields := payload.OrgName != nil || payload.JobTitle != nil ||
		payload.CommitteeRole != nil || payload.CommitteeVotingStatus != nil ||
		payload.IsVerified != nil

	if hasAttendeeFields {
		attendeeReq = &itx.UpdateAttendeeRequest{}
		if payload.OrgName != nil {
			attendeeReq.Org = *payload.OrgName
		}
		if payload.JobTitle != nil {
			attendeeReq.JobTitle = *payload.JobTitle
		}
		if payload.CommitteeRole != nil {
			attendeeReq.CommitteeRole = *payload.CommitteeRole
		}
		if payload.CommitteeVotingStatus != nil {
			attendeeReq.CommitteeVotingStatus = *payload.CommitteeVotingStatus
		}
		if payload.IsVerified != nil {
			attendeeReq.IsVerified = *payload.IsVerified
		}
	}

	return inviteeReq, attendeeReq
}

// ConvertParticipantResponseToGoa converts service ParticipantResponse to Goa type
func ConvertParticipantResponseToGoa(resp *itxservice.ParticipantResponse) *meetingservice.ITXPastMeetingParticipant {
	goaResp := &meetingservice.ITXPastMeetingParticipant{
		// IDs
		InviteeID:     ptrIfNotEmpty(resp.InviteeID),
		AttendeeID:    ptrIfNotEmpty(resp.AttendeeID),
		PastMeetingID: ptrIfNotEmpty(resp.PastMeetingID),
		MeetingID:     ptrIfNotEmpty(resp.MeetingID),

		// Flags
		IsInvited:  ptrBool(resp.IsInvited),
		IsAttended: ptrBool(resp.IsAttended),

		// User data
		FirstName:          ptrIfNotEmpty(resp.FirstName),
		LastName:           ptrIfNotEmpty(resp.LastName),
		Email:              ptrIfNotEmpty(resp.Email),
		Username:           ptrIfNotEmpty(resp.Username),
		LfUserID:           ptrIfNotEmpty(resp.LFUserID),
		OrgName:            ptrIfNotEmpty(resp.OrgName),
		JobTitle:           ptrIfNotEmpty(resp.JobTitle),
		AvatarURL:          ptrIfNotEmpty(resp.AvatarURL),
		OrgIsMember:        ptrBool(resp.OrgIsMember),
		OrgIsProjectMember: ptrBool(resp.OrgIsProjectMember),

		// Committee data
		CommitteeID:           ptrIfNotEmpty(resp.CommitteeID),
		CommitteeRole:         ptrIfNotEmpty(resp.CommitteeRole),
		IsCommitteeMember:     ptrBool(resp.IsCommitteeMember),
		CommitteeVotingStatus: ptrIfNotEmpty(resp.CommitteeVotingStatus),

		// Attendee-specific fields
		IsVerified: ptrBool(resp.IsVerified),
		IsUnknown:  ptrBool(resp.IsUnknown),

		// Audit fields
		CreatedAt:  ptrIfNotEmpty(resp.CreatedAt),
		ModifiedAt: ptrIfNotEmpty(resp.ModifiedAt),
	}

	// Add ID (use invitee_id if present, otherwise attendee_id)
	if resp.InviteeID != "" {
		goaResp.ID = ptrIfNotEmpty(resp.InviteeID)
	} else if resp.AttendeeID != "" {
		goaResp.ID = ptrIfNotEmpty(resp.AttendeeID)
	}

	// Convert average attendance
	if resp.AverageAttendance != 0 {
		goaResp.AverageAttendance = &resp.AverageAttendance
	}

	// Convert sessions
	if resp.Sessions != nil {
		goaResp.Sessions = make([]*meetingservice.ParticipantSession, len(resp.Sessions))
		for i, s := range resp.Sessions {
			goaResp.Sessions[i] = &meetingservice.ParticipantSession{
				ParticipantUUID: ptrIfNotEmpty(s.ParticipantUUID),
				JoinTime:        ptrIfNotEmpty(s.JoinTime),
				LeaveTime:       ptrIfNotEmpty(s.LeaveTime),
				LeaveReason:     ptrIfNotEmpty(s.LeaveReason),
			}
		}
	}

	// Convert created_by
	if resp.CreatedBy != nil {
		goaResp.CreatedBy = &meetingservice.ITXUser{
			Username:       ptrIfNotEmpty(resp.CreatedBy.Username),
			Name:           ptrIfNotEmpty(resp.CreatedBy.Name),
			Email:          ptrIfNotEmpty(resp.CreatedBy.Email),
			ProfilePicture: ptrIfNotEmpty(resp.CreatedBy.ProfilePicture),
		}
	}

	// Convert modified_by
	if resp.ModifiedBy != nil {
		goaResp.ModifiedBy = &meetingservice.ITXUser{
			Username:       ptrIfNotEmpty(resp.ModifiedBy.Username),
			Name:           ptrIfNotEmpty(resp.ModifiedBy.Name),
			Email:          ptrIfNotEmpty(resp.ModifiedBy.Email),
			ProfilePicture: ptrIfNotEmpty(resp.ModifiedBy.ProfilePicture),
		}
	}

	return goaResp
}

// Helper function to convert pointer to string value
func ptrToString(ptr *string) string {
	if ptr == nil {
		return ""
	}
	return *ptr
}
