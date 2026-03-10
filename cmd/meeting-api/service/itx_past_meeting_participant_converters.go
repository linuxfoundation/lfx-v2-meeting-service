// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"fmt"
	"strings"

	meetingservice "github.com/linuxfoundation/lfx-v2-meeting-service/gen/meeting_service"
	itxservice "github.com/linuxfoundation/lfx-v2-meeting-service/internal/service/itx"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/models/itx"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/utils"
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
		switch {
		case payload.FirstName != nil && payload.LastName != nil:
			name = strings.TrimSpace(fmt.Sprintf("%s %s", *payload.FirstName, *payload.LastName))
		case payload.FirstName != nil:
			name = *payload.FirstName
		case payload.LastName != nil:
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
					ParticipantUUID: utils.StringValue(s.ParticipantUUID),
					JoinTime:        utils.StringValue(s.JoinTime),
					LeaveTime:       utils.StringValue(s.LeaveTime),
					LeaveReason:     utils.StringValue(s.LeaveReason),
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
		InviteeID:     utils.StringPtrOmitEmpty(resp.InviteeID),
		AttendeeID:    utils.StringPtrOmitEmpty(resp.AttendeeID),
		PastMeetingID: utils.StringPtrOmitEmpty(resp.PastMeetingID),
		MeetingID:     utils.StringPtrOmitEmpty(resp.MeetingID),

		// Flags
		IsInvited:  utils.BoolPtr(resp.IsInvited),
		IsAttended: utils.BoolPtr(resp.IsAttended),

		// User data
		FirstName:          utils.StringPtrOmitEmpty(resp.FirstName),
		LastName:           utils.StringPtrOmitEmpty(resp.LastName),
		Email:              utils.StringPtrOmitEmpty(resp.Email),
		Username:           utils.StringPtrOmitEmpty(resp.Username),
		LfUserID:           utils.StringPtrOmitEmpty(resp.LFUserID),
		OrgName:            utils.StringPtrOmitEmpty(resp.OrgName),
		JobTitle:           utils.StringPtrOmitEmpty(resp.JobTitle),
		AvatarURL:          utils.StringPtrOmitEmpty(resp.AvatarURL),
		OrgIsMember:        utils.BoolPtr(resp.OrgIsMember),
		OrgIsProjectMember: utils.BoolPtr(resp.OrgIsProjectMember),

		// Committee data
		CommitteeID:           utils.StringPtrOmitEmpty(resp.CommitteeID),
		CommitteeRole:         utils.StringPtrOmitEmpty(resp.CommitteeRole),
		IsCommitteeMember:     utils.BoolPtr(resp.IsCommitteeMember),
		CommitteeVotingStatus: utils.StringPtrOmitEmpty(resp.CommitteeVotingStatus),

		// Attendee-specific fields
		IsVerified: utils.BoolPtr(resp.IsVerified),
		IsUnknown:  utils.BoolPtr(resp.IsUnknown),

		// Audit fields
		CreatedAt:  utils.StringPtrOmitEmpty(resp.CreatedAt),
		ModifiedAt: utils.StringPtrOmitEmpty(resp.ModifiedAt),
	}

	// Add ID (use invitee_id if present, otherwise attendee_id)
	if resp.InviteeID != "" {
		goaResp.ID = utils.StringPtrOmitEmpty(resp.InviteeID)
	} else if resp.AttendeeID != "" {
		goaResp.ID = utils.StringPtrOmitEmpty(resp.AttendeeID)
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
				ParticipantUUID: utils.StringPtrOmitEmpty(s.ParticipantUUID),
				JoinTime:        utils.StringPtrOmitEmpty(s.JoinTime),
				LeaveTime:       utils.StringPtrOmitEmpty(s.LeaveTime),
				LeaveReason:     utils.StringPtrOmitEmpty(s.LeaveReason),
			}
		}
	}

	// Convert created_by
	if resp.CreatedBy != nil {
		goaResp.CreatedBy = &meetingservice.ITXUser{
			Username:       utils.StringPtrOmitEmpty(resp.CreatedBy.Username),
			Name:           utils.StringPtrOmitEmpty(resp.CreatedBy.Name),
			Email:          utils.StringPtrOmitEmpty(resp.CreatedBy.Email),
			ProfilePicture: utils.StringPtrOmitEmpty(resp.CreatedBy.ProfilePicture),
		}
	}

	// Convert modified_by
	if resp.ModifiedBy != nil {
		goaResp.ModifiedBy = &meetingservice.ITXUser{
			Username:       utils.StringPtrOmitEmpty(resp.ModifiedBy.Username),
			Name:           utils.StringPtrOmitEmpty(resp.ModifiedBy.Name),
			Email:          utils.StringPtrOmitEmpty(resp.ModifiedBy.Email),
			ProfilePicture: utils.StringPtrOmitEmpty(resp.ModifiedBy.ProfilePicture),
		}
	}

	return goaResp
}

