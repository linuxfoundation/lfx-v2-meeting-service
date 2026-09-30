// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package itx

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/models/itx"
)

// PastMeetingService handles ITX past meeting operations
type PastMeetingService struct {
	auditStamper
	pastMeetingClient domain.ITXPastMeetingClient
	meetingClient     domain.ITXMeetingClient
	idMapper          domain.IDMapper
}

// NewPastMeetingService creates a new ITX past meeting service. userMetadata may be nil
// (e.g. when NATS is disabled), in which case created_by / updated_by are limited to the
// JWT-derived username/email rather than blocking the request.
func NewPastMeetingService(pastMeetingClient domain.ITXPastMeetingClient, meetingClient domain.ITXMeetingClient, idMapper domain.IDMapper, userMetadata domain.UserMetadataReader) *PastMeetingService {
	return &PastMeetingService{
		auditStamper:      auditStamper{userMetadata: userMetadata},
		pastMeetingClient: pastMeetingClient,
		meetingClient:     meetingClient,
		idMapper:          idMapper,
	}
}

// CreatePastMeeting creates a past meeting via ITX proxy
func (s *PastMeetingService) CreatePastMeeting(ctx context.Context, req *itx.CreatePastMeetingRequest) (*itx.PastMeetingResponse, error) {
	if err := mapProjectFieldV2ToV1(ctx, s.idMapper, &req.ProjectID); err != nil {
		return nil, err
	}
	if err := mapITXCommitteesV2ToV1(ctx, s.idMapper, req.Committees); err != nil {
		return nil, err
	}

	// Verify the caller's authorized object (project or committee) actually owns this
	// meeting. Both req.ProjectID/req.Committees and meeting.Project/meeting.Committees are
	// in v1 format at this point (after the mapping above).
	//
	// The Heimdall rule uses openfga_or_check: meetings_creator@project OR writer@committee.
	// On the committee path the body's project_uid is not authorized by Heimdall, so we must
	// also bind the supplied committee to the fetched meeting — otherwise a committee writer
	// on project A can name any meeting_id with project_uid=B and bypass the project check.
	meeting, err := s.meetingClient.GetZoomMeeting(ctx, req.MeetingID)
	if err != nil {
		return nil, err
	}
	if meeting.Project != req.ProjectID {
		return nil, domain.NewForbiddenError("meeting does not belong to the authorized project")
	}
	for _, c := range req.Committees {
		if !meetingHasCommittee(meeting, c.ID) {
			return nil, domain.NewForbiddenError("committee is not associated with the meeting")
		}
	}

	// Stamp created_by from the authenticated principal so the past-meeting record's
	// audit trail reflects who created it via the v2 API.
	req.CreatedBy = s.buildRequestingUser(ctx)

	resp, err := s.pastMeetingClient.CreatePastMeeting(ctx, req)
	if err != nil {
		return nil, err
	}

	if err := mapProjectFieldV1ToV2(ctx, s.idMapper, &resp.ProjectID); err != nil {
		return nil, err
	}
	mapMeetingCommitteesV1ToV2Graceful(ctx, s.idMapper, resp.Committees,
		"failed to map committee ID in past meeting response; returning empty committee UID")
	return resp, nil
}

// GetPastMeeting retrieves a past meeting via ITX proxy
func (s *PastMeetingService) GetPastMeeting(ctx context.Context, pastMeetingID string) (*itx.PastMeetingResponse, error) {
	resp, err := s.pastMeetingClient.GetPastMeeting(ctx, pastMeetingID)
	if err != nil {
		return nil, err
	}

	if err := mapProjectFieldV1ToV2(ctx, s.idMapper, &resp.ProjectID); err != nil {
		return nil, err
	}
	mapMeetingCommitteesV1ToV2Graceful(ctx, s.idMapper, resp.Committees,
		"failed to map committee ID in past meeting response; returning empty committee UID")
	return resp, nil
}

// UpdatePastMeeting updates a past meeting via ITX proxy
func (s *PastMeetingService) UpdatePastMeeting(ctx context.Context, pastMeetingID string, req *itx.CreatePastMeetingRequest) (*itx.PastMeetingResponse, error) {
	// Map project_uid v2→v1 before the immutability check so the comparison happens in v1
	// space, matching the stored record's ProjectID as returned by ITX.
	if err := mapProjectFieldV2ToV1(ctx, s.idMapper, &req.ProjectID); err != nil {
		return nil, err
	}

	// Reject re-parenting: meeting_id and project_uid are immutable after creation. Fetch
	// the current record once when either field is supplied, then compare both. Callers
	// echoing an unchanged value on a full PUT are not blocked.
	if req.MeetingID != "" || req.ProjectID != "" {
		current, err := s.pastMeetingClient.GetPastMeeting(ctx, pastMeetingID)
		if err != nil {
			return nil, err
		}
		if req.MeetingID != "" && req.MeetingID != current.MeetingID {
			return nil, domain.NewForbiddenError("meeting_id cannot be changed after a past meeting is created")
		}
		if req.ProjectID != "" && req.ProjectID != current.ProjectID {
			return nil, domain.NewForbiddenError("project_uid cannot be changed after a past meeting is created")
		}
	}

	if err := mapITXCommitteesV2ToV1(ctx, s.idMapper, req.Committees); err != nil {
		return nil, err
	}

	// Stamp updated_by from the authenticated principal so ITX overwrites the stored
	// updated_by / updated_by_list on the past-meeting record instead of preserving
	// stale data.
	req.UpdatedBy = s.buildRequestingUser(ctx)

	_, err := s.pastMeetingClient.UpdatePastMeeting(ctx, pastMeetingID, req)
	if err != nil {
		return nil, err
	}
	return nil, nil
}

// DeletePastMeeting deletes a past meeting via ITX proxy
func (s *PastMeetingService) DeletePastMeeting(ctx context.Context, pastMeetingID string) error {
	return s.pastMeetingClient.DeletePastMeeting(ctx, pastMeetingID)
}

// meetingHasCommittee reports whether the meeting's committee list contains committeeID
// (v1 SFID space). Used to bind the committee-auth path to the actual meeting.
func meetingHasCommittee(meeting *itx.ZoomMeetingResponse, committeeID string) bool {
	for _, c := range meeting.Committees {
		if c.ID == committeeID {
			return true
		}
	}
	return false
}
