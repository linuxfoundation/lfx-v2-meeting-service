// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/linuxfoundation/lfx-v2-meeting-service/cmd/meeting-api/service"
	meetingsvc "github.com/linuxfoundation/lfx-v2-meeting-service/gen/meeting_service"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/models/itx"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/utils"
)

// CreateItxMeeting creates a meeting via ITX proxy
func (s *MeetingsAPI) CreateItxMeeting(ctx context.Context, p *meetingsvc.CreateItxMeetingPayload) (*meetingsvc.ITXZoomMeetingResponse, error) {
	req := service.ConvertCreateITXMeetingPayloadToDomain(p)
	resp, err := s.itxMeetingService.CreateMeeting(ctx, req)
	if err != nil {
		return nil, handleError(ctx, err)
	}
	slog.InfoContext(ctx, "meeting created", "meeting_id", resp.ID)
	return service.ConvertITXMeetingResponseToGoa(resp), nil
}

// GetItxMeeting retrieves a meeting via ITX proxy
func (s *MeetingsAPI) GetItxMeeting(ctx context.Context, p *meetingsvc.GetItxMeetingPayload) (*meetingsvc.ITXZoomMeetingResponse, error) {
	resp, err := s.itxMeetingService.GetMeeting(ctx, p.MeetingID)
	if err != nil {
		return nil, handleError(ctx, err)
	}
	return service.ConvertITXMeetingResponseToGoa(resp), nil
}

// UpdateItxMeeting updates a meeting via ITX proxy
func (s *MeetingsAPI) UpdateItxMeeting(ctx context.Context, p *meetingsvc.UpdateItxMeetingPayload) error {
	req := service.ConvertCreateITXMeetingPayloadToDomain(&meetingsvc.CreateItxMeetingPayload{
		BearerToken:              p.BearerToken,
		Version:                  p.Version,
		XSync:                    p.XSync,
		ProjectUID:               p.ProjectUID,
		Title:                    p.Title,
		StartTime:                p.StartTime,
		Duration:                 p.Duration,
		Timezone:                 p.Timezone,
		Visibility:               p.Visibility,
		Description:              p.Description,
		Restricted:               p.Restricted,
		Committees:               p.Committees,
		MeetingType:              p.MeetingType,
		EarlyJoinTimeMinutes:     p.EarlyJoinTimeMinutes,
		RecordingEnabled:         p.RecordingEnabled,
		TranscriptEnabled:        p.TranscriptEnabled,
		YoutubeUploadEnabled:     p.YoutubeUploadEnabled,
		AiSummaryEnabled:         p.AiSummaryEnabled,
		RequireAiSummaryApproval: p.RequireAiSummaryApproval,
		ArtifactVisibility:       p.ArtifactVisibility,
		Recurrence:               p.Recurrence,
		AutoEmailReminderEnabled: p.AutoEmailReminderEnabled,
		AutoEmailReminderTime:    p.AutoEmailReminderTime,
		Owner:                    p.Owner,
	})

	req.ID = p.MeetingID
	req.UpdateNote = utils.StringValue(p.UpdateNote)
	err := s.itxMeetingService.UpdateMeeting(ctx, p.MeetingID, req)
	if err != nil {
		return handleError(ctx, err)
	}
	slog.InfoContext(ctx, "meeting updated", "meeting_id", p.MeetingID)
	return nil
}

// DeleteItxMeeting deletes a meeting via ITX proxy
func (s *MeetingsAPI) DeleteItxMeeting(ctx context.Context, p *meetingsvc.DeleteItxMeetingPayload) error {
	err := s.itxMeetingService.DeleteMeeting(ctx, p.MeetingID)
	if err != nil {
		return handleError(ctx, err)
	}
	slog.InfoContext(ctx, "meeting deleted", "meeting_id", p.MeetingID)
	return nil
}

// GetItxMeetingCount retrieves the meeting count for a project via ITX proxy
func (s *MeetingsAPI) GetItxMeetingCount(ctx context.Context, p *meetingsvc.GetItxMeetingCountPayload) (*meetingsvc.ITXMeetingCountResponse, error) {
	resp, err := s.itxMeetingService.GetMeetingCount(ctx, p.ProjectUID)
	if err != nil {
		return nil, handleError(ctx, err)
	}
	return &meetingsvc.ITXMeetingCountResponse{MeetingCount: resp.MeetingCount}, nil
}

// GetItxJoinLink retrieves a join link for a meeting via ITX proxy
func (s *MeetingsAPI) GetItxJoinLink(ctx context.Context, p *meetingsvc.GetItxJoinLinkPayload) (*meetingsvc.ITXZoomMeetingJoinLink, error) {
	req := service.ConvertGetJoinLinkPayloadToITX(p)
	resp, err := s.itxMeetingService.GetMeetingJoinLink(ctx, req)
	if err != nil {
		return nil, handleError(ctx, err)
	}
	return service.ConvertITXJoinLinkResponseToGoa(resp), nil
}

// ResendItxMeetingInvitations resends meeting invitations to all registrants via ITX proxy
func (s *MeetingsAPI) ResendItxMeetingInvitations(ctx context.Context, p *meetingsvc.ResendItxMeetingInvitationsPayload) error {
	req := &itx.ResendMeetingInvitationsRequest{
		ExcludeRegistrantIDs: p.ExcludeRegistrantIds,
	}
	err := s.itxMeetingService.ResendMeetingInvitations(ctx, p.MeetingID, req)
	if err != nil {
		return handleError(ctx, err)
	}
	slog.InfoContext(ctx, "meeting invitations resent", "meeting_id", p.MeetingID)
	return nil
}

// RegisterItxCommitteeMembers registers committee members to a meeting asynchronously via ITX proxy
func (s *MeetingsAPI) RegisterItxCommitteeMembers(ctx context.Context, p *meetingsvc.RegisterItxCommitteeMembersPayload) error {
	err := s.itxMeetingService.RegisterCommitteeMembers(ctx, p.MeetingID)
	if err != nil {
		return handleError(ctx, err)
	}
	slog.InfoContext(ctx, "committee member registration triggered", "meeting_id", p.MeetingID)
	return nil
}

// UpdateItxOccurrence updates a specific occurrence of a recurring meeting via ITX proxy
func (s *MeetingsAPI) UpdateItxOccurrence(ctx context.Context, p *meetingsvc.UpdateItxOccurrencePayload) error {
	req := service.ConvertUpdateOccurrencePayloadToITX(p)
	err := s.itxMeetingService.UpdateOccurrence(ctx, p.MeetingID, p.OccurrenceID, req)
	if err != nil {
		return handleError(ctx, err)
	}
	slog.InfoContext(ctx, "meeting occurrence updated", "meeting_id", p.MeetingID, "occurrence_id", p.OccurrenceID)
	return nil
}

// DeleteItxOccurrence deletes a specific occurrence of a recurring meeting via ITX proxy
func (s *MeetingsAPI) DeleteItxOccurrence(ctx context.Context, p *meetingsvc.DeleteItxOccurrencePayload) error {
	err := s.itxMeetingService.DeleteOccurrence(ctx, p.MeetingID, p.OccurrenceID)
	if err != nil {
		return handleError(ctx, err)
	}
	slog.InfoContext(ctx, "meeting occurrence deleted", "meeting_id", p.MeetingID, "occurrence_id", p.OccurrenceID)
	return nil
}

// SubmitItxMeetingResponse submits a meeting response for a meeting or occurrence via ITX proxy
func (s *MeetingsAPI) SubmitItxMeetingResponse(ctx context.Context, p *meetingsvc.SubmitItxMeetingResponsePayload) (*meetingsvc.ITXMeetingResponseResult, error) {
	meetingAndOccurrenceID := p.MeetingID
	if p.OccurrenceID != nil && *p.OccurrenceID != "" {
		meetingAndOccurrenceID = fmt.Sprintf("%s-%s", p.MeetingID, *p.OccurrenceID)
	}

	req := service.ConvertSubmitITXMeetingResponsePayloadToITX(p)

	result, err := s.itxMeetingService.SubmitMeetingResponse(ctx, meetingAndOccurrenceID, req)
	if err != nil {
		return nil, handleError(ctx, err)
	}
	result.MeetingID = p.MeetingID
	slog.InfoContext(ctx, "meeting response submitted", "meeting_id", p.MeetingID)
	return service.ConvertITXMeetingResponseResultToGoa(result), nil
}
