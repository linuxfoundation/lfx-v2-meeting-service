// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package main

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-meeting-service/cmd/meeting-api/service"
	meetingsvc "github.com/linuxfoundation/lfx-v2-meeting-service/gen/meeting_service"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
)

// CreateItxPastMeetingParticipant creates a past meeting participant via ITX proxy
func (s *MeetingsAPI) CreateItxPastMeetingParticipant(ctx context.Context, p *meetingsvc.CreateItxPastMeetingParticipantPayload) (*meetingsvc.ITXPastMeetingParticipant, error) {
	inviteeReq, attendeeReq := service.ConvertCreateParticipantPayload(p)
	isInvited := p.IsInvited != nil && *p.IsInvited
	isAttended := p.IsAttended != nil && *p.IsAttended
	resp, err := s.itxPastMeetingParticipantService.CreateParticipant(ctx, p.PastMeetingID, isInvited, isAttended, inviteeReq, attendeeReq)
	if err != nil {
		return nil, handleError(err)
	}
	return service.ConvertParticipantResponseToGoa(resp), nil
}

func (s *MeetingsAPI) UpdateItxPastMeetingParticipant(ctx context.Context, p *meetingsvc.UpdateItxPastMeetingParticipantPayload) (*meetingsvc.ITXPastMeetingParticipant, error) {
	inviteeReq, attendeeReq := service.ConvertUpdateParticipantPayload(p)

	var inviteeID, attendeeID string
	if p.InviteeID != nil {
		inviteeID = *p.InviteeID
	}
	if p.AttendeeID != nil {
		attendeeID = *p.AttendeeID
	}

	resp, err := s.itxPastMeetingParticipantService.UpdateParticipant(ctx, &models.UpdatePastMeetingParticipant{
		PastMeetingID: p.PastMeetingID,
		ParticipantID: p.ParticipantID,
		InviteeID:     inviteeID,
		AttendeeID:    attendeeID,
		IsInvited:     p.IsInvited,
		IsAttended:    p.IsAttended,
	}, inviteeReq, attendeeReq)
	if err != nil {
		return nil, handleError(err)
	}

	return service.ConvertParticipantResponseToGoa(resp), nil
}

// DeleteItxPastMeetingParticipant deletes a past meeting participant via ITX proxy
func (s *MeetingsAPI) DeleteItxPastMeetingParticipant(ctx context.Context, p *meetingsvc.DeleteItxPastMeetingParticipantPayload) error {
	err := s.itxPastMeetingParticipantService.DeleteParticipant(ctx, p.PastMeetingID, p.ParticipantID)
	if err != nil {
		return handleError(err)
	}

	return nil
}
