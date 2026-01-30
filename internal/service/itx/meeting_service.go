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
}

// NewMeetingService creates a new ITX meeting service
func NewMeetingService(proxyClient domain.ITXProxyClient) *MeetingService {
	return &MeetingService{
		proxyClient: proxyClient,
	}
}

// CreateMeeting creates a meeting via ITX proxy
func (s *MeetingService) CreateMeeting(ctx context.Context, req *models.CreateITXMeetingRequest) (*itx.ZoomMeetingResponse, error) {
	// Transform to ITX format
	itxReq := s.transformToITXRequest(req)

	// Call ITX proxy
	resp, err := s.proxyClient.CreateZoomMeeting(ctx, itxReq)
	if err != nil {
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

	return resp, nil
}

// UpdateMeeting updates a meeting via ITX proxy
func (s *MeetingService) UpdateMeeting(ctx context.Context, meetingID string, req *models.CreateITXMeetingRequest) error {
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
	// Call ITX proxy
	resp, err := s.proxyClient.GetMeetingCount(ctx, projectID)
	if err != nil {
		return nil, err
	}

	return resp, nil
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
