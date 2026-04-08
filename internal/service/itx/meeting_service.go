// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package itx

import (
	"context"
	"log/slog"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/models/itx"
)

// MeetingService handles ITX Zoom meeting operations
type MeetingService struct {
	meetingClient domain.ITXMeetingClient
	idMapper      domain.IDMapper
}

// NewMeetingService creates a new ITX meeting service
func NewMeetingService(meetingClient domain.ITXMeetingClient, idMapper domain.IDMapper) *MeetingService {
	return &MeetingService{
		meetingClient: meetingClient,
		idMapper:      idMapper,
	}
}

// CreateMeeting creates a meeting via ITX proxy
func (s *MeetingService) CreateMeeting(ctx context.Context, req *models.CreateITXMeetingRequest) (*itx.ZoomMeetingResponse, error) {
	if err := validateMeetingRequest(req); err != nil {
		return nil, err
	}

	// Map v2 UIDs to v1 SFIDs before sending to ITX
	if err := s.mapRequestV2ToV1(ctx, req); err != nil {
		return nil, err
	}

	itxReq := s.transformToITXRequest(req)
	resp, err := s.meetingClient.CreateZoomMeeting(ctx, itxReq)
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
	resp, err := s.meetingClient.GetZoomMeeting(ctx, meetingID)
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
	if err := validateMeetingRequest(req); err != nil {
		return err
	}

	// Map v2 UIDs to v1 SFIDs before sending to ITX
	if err := s.mapRequestV2ToV1(ctx, req); err != nil {
		return err
	}

	itxReq := s.transformToITXRequest(req)
	err := s.meetingClient.UpdateZoomMeeting(ctx, meetingID, itxReq)
	if err != nil {
		return err
	}

	return nil
}

// DeleteMeeting deletes a meeting via ITX proxy
func (s *MeetingService) DeleteMeeting(ctx context.Context, meetingID string) error {
	err := s.meetingClient.DeleteZoomMeeting(ctx, meetingID)
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

	resp, err := s.meetingClient.GetMeetingCount(ctx, v1SFID)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// GetMeetingJoinLink retrieves a join link for a meeting via ITX proxy
func (s *MeetingService) GetMeetingJoinLink(ctx context.Context, req *itx.GetJoinLinkRequest) (*itx.ZoomMeetingJoinLink, error) {
	return s.meetingClient.GetMeetingJoinLink(ctx, req)
}

// ResendMeetingInvitations resends meeting invitations to all registrants via ITX proxy
func (s *MeetingService) ResendMeetingInvitations(ctx context.Context, meetingID string, req *itx.ResendMeetingInvitationsRequest) error {
	return s.meetingClient.ResendMeetingInvitations(ctx, meetingID, req)
}

// RegisterCommitteeMembers registers committee members to a meeting asynchronously via ITX proxy
func (s *MeetingService) RegisterCommitteeMembers(ctx context.Context, meetingID string) error {
	return s.meetingClient.RegisterCommitteeMembers(ctx, meetingID)
}

// UpdateOccurrence updates a specific occurrence of a recurring meeting via ITX proxy
func (s *MeetingService) UpdateOccurrence(ctx context.Context, meetingID, occurrenceID string, req *itx.UpdateOccurrenceRequest) error {
	return s.meetingClient.UpdateOccurrence(ctx, meetingID, occurrenceID, req)
}

// DeleteOccurrence deletes a specific occurrence of a recurring meeting via ITX proxy
func (s *MeetingService) DeleteOccurrence(ctx context.Context, meetingID, occurrenceID string) error {
	return s.meetingClient.DeleteOccurrence(ctx, meetingID, occurrenceID)
}

// SubmitMeetingResponse submits a meeting response for a meeting or occurrence via ITX proxy
func (s *MeetingService) SubmitMeetingResponse(ctx context.Context, meetingAndOccurrenceID string, req *itx.MeetingResponseRequest) (*itx.MeetingResponseResult, error) {
	return s.meetingClient.SubmitMeetingResponse(ctx, meetingAndOccurrenceID, req)
}

// validateMeetingRequest validates a meeting create/update request before sending to ITX
func validateMeetingRequest(req *models.CreateITXMeetingRequest) error {
	anyFeatureEnabled := req.RecordingEnabled || req.TranscriptEnabled || req.AISummaryEnabled
	if anyFeatureEnabled && req.ArtifactVisibility == "" {
		return domain.NewValidationError("artifact_visibility is required when recording, transcript, or ai_summary is enabled")
	}
	return nil
}

// transformToITXRequest transforms domain request to ITX request format
func (s *MeetingService) transformToITXRequest(req *models.CreateITXMeetingRequest) *itx.CreateZoomMeetingRequest {
	itxReq := &itx.CreateZoomMeetingRequest{
		ID:                   req.ID, // Only used for updates
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
		ZoomAIEnabled:        req.AISummaryEnabled,
	}

	// Map artifact visibility to access controls only when the respective feature is enabled
	if req.ArtifactVisibility != "" {
		if req.RecordingEnabled {
			itxReq.RecordingAccess = req.ArtifactVisibility
		}
		if req.TranscriptEnabled {
			itxReq.TranscriptAccess = req.ArtifactVisibility
		}
		if req.AISummaryEnabled {
			itxReq.AISummaryAccess = req.ArtifactVisibility
		}
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

	// Map committee SFIDs (v1) to committee UIDs (v2). On any mapping failure, log a warning,
	// leave the committee UID empty, and continue so the caller still receives the full response.
	for i := range resp.Committees {
		if resp.Committees[i].ID != "" {
			v2UID, err := s.idMapper.MapCommitteeV1ToV2(ctx, resp.Committees[i].ID)
			if err != nil {
				slog.WarnContext(ctx, "failed to map committee ID in meeting response; returning empty committee UID",
					"v1_id", resp.Committees[i].ID, "err", err)
				resp.Committees[i].ID = ""
				continue
			}
			resp.Committees[i].ID = v2UID
		}
	}

	return nil
}
