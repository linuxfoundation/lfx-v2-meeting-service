// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package itx

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/models/itx"
)

// MeetingService handles ITX Zoom meeting operations
type MeetingService struct {
	proxyClient domain.ITXProxyClient
	idMapper    domain.IDMapper
}

// NewMeetingService creates a new ITX meeting service
func NewMeetingService(proxyClient domain.ITXProxyClient, idMapper domain.IDMapper) *MeetingService {
	return &MeetingService{
		proxyClient: proxyClient,
		idMapper:    idMapper,
	}
}

// CreateMeeting creates a meeting via ITX proxy
func (s *MeetingService) CreateMeeting(ctx context.Context, req *models.CreateITXMeetingRequest) (*itx.ZoomMeetingResponse, error) {
	// Map v2 UIDs to v1 SFIDs before sending to ITX
	if err := s.mapRequestV2ToV1(ctx, req); err != nil {
		return nil, err
	}

	// Transform to ITX format
	itxReq := s.transformToITXRequest(req)

	// Call ITX proxy
	resp, err := s.proxyClient.CreateZoomMeeting(ctx, itxReq)
	if err != nil {
		return nil, err
	}

	// Map v1 SFIDs back to v2 UIDs in response
	if err := s.mapResponseV1ToV2(ctx, resp); err != nil {
		return nil, err
	}

	return resp, nil
}

// GetMeeting retrieves a meeting via ITX proxy
func (s *MeetingService) GetMeeting(ctx context.Context, meetingID string) (*itx.ZoomMeetingResponse, error) {
	// Call ITX proxy
	resp, err := s.proxyClient.GetZoomMeeting(ctx, meetingID)
	if err != nil {
		return nil, err
	}

	// Map v1 SFIDs back to v2 UIDs in response
	if err := s.mapResponseV1ToV2(ctx, resp); err != nil {
		return nil, err
	}

	return resp, nil
}

// UpdateMeeting updates a meeting via ITX proxy
func (s *MeetingService) UpdateMeeting(ctx context.Context, meetingID string, req *models.CreateITXMeetingRequest) error {
	// Map v2 UIDs to v1 SFIDs before sending to ITX
	if err := s.mapRequestV2ToV1(ctx, req); err != nil {
		return err
	}

	// Transform to ITX format
	itxReq := s.transformToITXRequest(req)

	// Call ITX proxy
	err := s.proxyClient.UpdateZoomMeeting(ctx, meetingID, itxReq)
	if err != nil {
		return err
	}

	return nil
}

// DeleteMeeting deletes a meeting via ITX proxy
func (s *MeetingService) DeleteMeeting(ctx context.Context, meetingID string) error {
	// Call ITX proxy
	err := s.proxyClient.DeleteZoomMeeting(ctx, meetingID)
	if err != nil {
		return err
	}

	return nil
}

// GetMeetingCount retrieves the count of meetings for a project via ITX proxy
func (s *MeetingService) GetMeetingCount(ctx context.Context, projectID string) (*itx.MeetingCountResponse, error) {
	// Map v2 project UID to v1 SFID
	v1SFID, err := s.idMapper.MapProjectV2ToV1(ctx, projectID)
	if err != nil {
		return nil, err
	}

	// Call ITX proxy with v1 SFID
	resp, err := s.proxyClient.GetMeetingCount(ctx, v1SFID)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// GetMeetingJoinLink retrieves a join link for a meeting via ITX proxy
func (s *MeetingService) GetMeetingJoinLink(ctx context.Context, req *itx.GetJoinLinkRequest) (*itx.ZoomMeetingJoinLink, error) {
	// Call ITX proxy
	resp, err := s.proxyClient.GetMeetingJoinLink(ctx, req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// ResendMeetingInvitations resends meeting invitations to all registrants via ITX proxy
func (s *MeetingService) ResendMeetingInvitations(ctx context.Context, meetingID string, req *itx.ResendMeetingInvitationsRequest) error {
	// Call ITX proxy
	return s.proxyClient.ResendMeetingInvitations(ctx, meetingID, req)
}

// transformToITXRequest transforms domain request to ITX request format
func (s *MeetingService) transformToITXRequest(req *models.CreateITXMeetingRequest) *itx.CreateZoomMeetingRequest {
	itxReq := &itx.CreateZoomMeetingRequest{
		Project:              req.ProjectUID,
		Topic:                req.Title,
		StartTime:            req.StartTime,
		Duration:             req.Duration,
		Timezone:             req.Timezone,
		Visibility:           req.Visibility,
		Agenda:               req.Description,
		Restricted:           req.Restricted,
		MeetingType:          req.MeetingType,
		EarlyJoinTime:        req.EarlyJoinTimeMinutes,
		RecordingEnabled:     req.RecordingEnabled,
		TranscriptEnabled:    req.TranscriptEnabled,
		YoutubeUploadEnabled: req.YoutubeUploadEnabled,
	}

	// Map artifact visibility to access controls
	if req.ArtifactVisibility != "" {
		itxReq.RecordingAccess = req.ArtifactVisibility
		itxReq.TranscriptAccess = req.ArtifactVisibility
		itxReq.AISummaryAccess = req.ArtifactVisibility
	}

	// Map committees
	if len(req.Committees) > 0 {
		itxReq.Committees = make([]itx.Committee, len(req.Committees))
		for i, c := range req.Committees {
			itxReq.Committees[i] = itx.Committee{
				ID:      c.UID,
				Filters: c.AllowedVotingStatuses,
			}
		}
	}

	// Map recurrence if present
	if req.Recurrence != nil {
		itxReq.Recurrence = &itx.Recurrence{
			Type:           req.Recurrence.Type,
			RepeatInterval: req.Recurrence.RepeatInterval,
			WeeklyDays:     req.Recurrence.WeeklyDays,
			MonthlyDay:     req.Recurrence.MonthlyDay,
			MonthlyWeek:    req.Recurrence.MonthlyWeek,
			MonthlyWeekDay: req.Recurrence.MonthlyWeekDay,
			EndTimes:       req.Recurrence.EndTimes,
			EndDateTime:    req.Recurrence.EndDateTime,
		}
	}

	return itxReq
}

// mapRequestV2ToV1 maps v2 UIDs to v1 SFIDs in the request
func (s *MeetingService) mapRequestV2ToV1(ctx context.Context, req *models.CreateITXMeetingRequest) error {
	// Map project UID (v2) to project SFID (v1)
	if req.ProjectUID != "" {
		v1SFID, err := s.idMapper.MapProjectV2ToV1(ctx, req.ProjectUID)
		if err != nil {
			return err
		}
		req.ProjectUID = v1SFID
	}

	// Map committee UIDs (v2) to committee SFIDs (v1)
	for i := range req.Committees {
		if req.Committees[i].UID != "" {
			v1SFID, err := s.idMapper.MapCommitteeV2ToV1(ctx, req.Committees[i].UID)
			if err != nil {
				return err
			}
			req.Committees[i].UID = v1SFID
		}
	}

	return nil
}

// mapResponseV1ToV2 maps v1 SFIDs to v2 UIDs in the response
func (s *MeetingService) mapResponseV1ToV2(ctx context.Context, resp *itx.ZoomMeetingResponse) error {
	// Map project SFID (v1) to project UID (v2)
	if resp.Project != "" {
		v2UID, err := s.idMapper.MapProjectV1ToV2(ctx, resp.Project)
		if err != nil {
			return err
		}
		resp.Project = v2UID
	}

	// Map committee SFIDs (v1) to committee UIDs (v2)
	for i := range resp.Committees {
		if resp.Committees[i].ID != "" {
			v2UID, err := s.idMapper.MapCommitteeV1ToV2(ctx, resp.Committees[i].ID)
			if err != nil {
				return err
			}
			resp.Committees[i].ID = v2UID
		}
	}

	return nil
}
