// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package itx

import (
	"context"
	"log/slog"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/constants"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/models/itx"
)

// MeetingService handles ITX Zoom meeting operations
type MeetingService struct {
	auditStamper
	meetingClient  domain.ITXMeetingClient
	idMapper       domain.IDMapper
	committeeAuthz domain.CommitteeAuthorizer // nil when NATS is unavailable
}

// NewMeetingService creates a new ITX meeting service.
//
// userMetadata may be nil (e.g. when NATS is disabled), in which case
// created_by / updated_by are limited to the JWT-derived username/email.
//
// committeeAuthz may be nil (e.g. when NATS is disabled), in which case the
// per-committee FGA write-access check is skipped on UpdateMeeting. Heimdall's
// existing organizer check still applies; the FGA check is defense-in-depth.
func NewMeetingService(meetingClient domain.ITXMeetingClient, idMapper domain.IDMapper, userMetadata domain.UserMetadataReader, committeeAuthz domain.CommitteeAuthorizer) *MeetingService {
	return &MeetingService{
		auditStamper:   auditStamper{userMetadata: userMetadata},
		meetingClient:  meetingClient,
		idMapper:       idMapper,
		committeeAuthz: committeeAuthz,
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
	itxReq.CreatedBy = s.buildRequestingUser(ctx)
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

	// Capture v2 committee UIDs before mapping overwrites them with v1 SFIDs.
	// FGA object IDs use v2 UIDs (docs/fga-contract.md), so we need to preserve
	// the originals for the HasWriteAccess call below.
	v2CommitteeUIDs := make([]string, len(req.Committees))
	for i, c := range req.Committees {
		v2CommitteeUIDs[i] = c.UID
	}

	// Map v2 UIDs to v1 SFIDs before sending to ITX
	if err := s.mapRequestV2ToV1(ctx, req); err != nil {
		return err
	}

	// Build v1SFID→v2UID lookup now that mapping is complete.
	// req.Committees[i].UID is the v1 SFID; v2CommitteeUIDs[i] is the original v2 UID.
	v1ToV2Committee := make(map[string]string, len(req.Committees))
	for i, c := range req.Committees {
		v1ToV2Committee[c.UID] = v2CommitteeUIDs[i]
	}

	// Fetch the live meeting so we can (a) block project re-parenting and (b)
	// compute the committee delta for the FGA write-access check below.
	// Both req and current are in v1 SFID space at this point.
	current, err := s.meetingClient.GetZoomMeeting(ctx, meetingID)
	if err != nil {
		return err
	}
	if err := validateProjectNotReparented(req, current); err != nil {
		return err
	}
	if err := s.authorizeNewCommittees(ctx, req, current, v1ToV2Committee); err != nil {
		return err
	}

	itxReq := s.transformToITXRequest(req)
	// Stamp updated_by from the authenticated principal. ITX only overwrites the stored
	// updated_by / updated_by_list when this field is non-zero, so omitting it leaves a
	// stale value on the record (typically the original creator or last PIS updater).
	itxReq.UpdatedBy = s.buildRequestingUser(ctx)
	return s.meetingClient.UpdateZoomMeeting(ctx, meetingID, itxReq)
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
	// Stamp updated_by from the authenticated principal so ITX overwrites the stored
	// value on the occurrence record instead of preserving stale data.
	req.UpdatedBy = s.buildRequestingUser(ctx)
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

// validateProjectNotReparented rejects updates that attempt to move a meeting to a
// different project. req must already be in v1 SFID space (post-mapping); current is
// the live ITX record, also in v1 SFID space.
func validateProjectNotReparented(req *models.CreateITXMeetingRequest, current *itx.ZoomMeetingResponse) error {
	if req.ProjectUID != current.Project {
		return domain.NewForbiddenError("project_uid cannot be changed after a meeting is created")
	}
	return nil
}

// authorizeNewCommittees checks that the requesting principal has FGA "writer"
// access on every committee that is present in req but absent from the live
// ITX record (i.e. committees being newly added). Committees already on the
// meeting are unchanged from the caller's perspective so no new permission is
// required for them.
//
// v1ToV2Committee maps each committee's v1 SFID (as stored in req.Committees after
// ID mapping) to its original v2 UID, which is the form FGA uses for object IDs.
//
// If committeeAuthz is nil (NATS not configured at all), the check is skipped.
// When committeeAuthz is non-nil but HasWriteAccess returns an error (e.g. fga-sync
// temporarily unavailable), the request is rejected so that outages cannot be used
// to bypass the committee authorization check.
func (s *MeetingService) authorizeNewCommittees(ctx context.Context, req *models.CreateITXMeetingRequest, current *itx.ZoomMeetingResponse, v1ToV2Committee map[string]string) error {
	if s.committeeAuthz == nil || len(req.Committees) == 0 {
		return nil
	}

	principal, _ := ctx.Value(constants.PrincipalContextID).(string)
	if principal == "" {
		// No authenticated principal — Heimdall already blocked unauthenticated
		// requests; nothing to check here.
		return nil
	}

	// Build the set of committee IDs already on the meeting (v1 SFID space).
	existing := make(map[string]struct{}, len(current.Committees))
	for _, c := range current.Committees {
		existing[c.ID] = struct{}{}
	}

	for _, c := range req.Committees {
		if _, alreadyOn := existing[c.UID]; alreadyOn {
			continue // no new permission needed for an existing committee
		}
		// Use the v2 UID for the FGA check; FGA object IDs are v2 UIDs.
		v2UID := v1ToV2Committee[c.UID]
		if v2UID == "" {
			v2UID = c.UID // no mapping available; best-effort with the SFID
		}
		ok, err := s.committeeAuthz.HasWriteAccess(ctx, principal, v2UID)
		if err != nil {
			// fga-sync is unreachable — fail closed to prevent the authorization
			// bypass window that fail-open would create during outages. Use
			// Unavailable (503) rather than Forbidden (403) so callers know the
			// denial is transient and can retry.
			slog.WarnContext(ctx, "committee FGA write-access check failed; rejecting update",
				"committee_id", v2UID, "error", err)
			return domain.NewUnavailableError("cannot verify committee write access; please retry")
		}
		if !ok {
			return domain.NewForbiddenError("not authorized to add committee to this meeting")
		}
	}
	return nil
}

// validateMeetingRequest validates a meeting create/update request before sending to ITX
func validateMeetingRequest(req *models.CreateITXMeetingRequest) error {
	anyFeatureEnabled := req.RecordingEnabled || req.TranscriptEnabled || req.AISummaryEnabled
	if anyFeatureEnabled && req.ArtifactVisibility == "" {
		return domain.NewValidationError("artifact_visibility is required when recording, transcript, or ai_summary is enabled")
	}
	return nil
}

// buildRequestingUser is provided by the embedded auditStamper; see audit.go.

// transformToITXRequest transforms domain request to ITX request format
func (s *MeetingService) transformToITXRequest(req *models.CreateITXMeetingRequest) *itx.CreateZoomMeetingRequest {
	itxReq := &itx.CreateZoomMeetingRequest{
		ID:                       req.ID, // Only used for updates
		Project:                  req.ProjectUID,
		Topic:                    req.Title,
		StartTime:                req.StartTime,
		Duration:                 req.Duration,
		Timezone:                 req.Timezone,
		Visibility:               req.Visibility,
		Agenda:                   req.Description,
		Restricted:               req.Restricted,
		MeetingType:              req.MeetingType,
		EarlyJoinTime:            req.EarlyJoinTimeMinutes,
		RecordingEnabled:         req.RecordingEnabled,
		TranscriptEnabled:        req.TranscriptEnabled,
		YoutubeUploadEnabled:     req.YoutubeUploadEnabled,
		ZoomAIEnabled:            req.AISummaryEnabled,
		RequireAISummaryApproval: req.RequireAISummaryApproval,
		AutoEmailReminderEnabled: req.AutoEmailReminderEnabled, // nil = omitted, ITX preserves the stored reminder
		AutoEmailReminderTime:    req.AutoEmailReminderTime,
		ShowMeetingAttendees:     req.ShowMeetingAttendees, // nil = omitted, ITX preserves the stored value
		Note:                     req.UpdateNote,
		Owner:                    req.Owner, // nil = omitted, ITX preserves the stored owner
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
	if err := mapProjectFieldV2ToV1(ctx, s.idMapper, &req.ProjectUID); err != nil {
		return err
	}
	return mapMeetingCommitteesV2ToV1(ctx, s.idMapper, req.Committees)
}

// mapResponseV1ToV2 maps v1 SFIDs to v2 UIDs in the response
func (s *MeetingService) mapResponseV1ToV2(ctx context.Context, resp *itx.ZoomMeetingResponse) error {
	if err := mapProjectFieldV1ToV2(ctx, s.idMapper, &resp.Project); err != nil {
		return err
	}
	mapMeetingCommitteesV1ToV2Graceful(ctx, s.idMapper, resp.Committees,
		"failed to map committee ID in meeting response; returning empty committee UID")
	return nil
}
