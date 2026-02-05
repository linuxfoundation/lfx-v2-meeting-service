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
	pastMeetingClient domain.ITXPastMeetingClient
	idMapper          domain.IDMapper
}

// NewPastMeetingService creates a new ITX past meeting service
func NewPastMeetingService(pastMeetingClient domain.ITXPastMeetingClient, idMapper domain.IDMapper) *PastMeetingService {
	return &PastMeetingService{
		pastMeetingClient: pastMeetingClient,
		idMapper:          idMapper,
	}
}

// CreatePastMeeting creates a past meeting via ITX proxy
func (s *PastMeetingService) CreatePastMeeting(ctx context.Context, req *itx.CreatePastMeetingRequest) (*itx.PastMeetingResponse, error) {
	// Map v2 project UID to v1 SFID before sending to ITX
	if req.ProjectID != "" {
		v1ID, err := s.idMapper.MapProjectV2ToV1(ctx, req.ProjectID)
		if err != nil {
			return nil, err
		}
		req.ProjectID = v1ID
	}

	// Map committee UIDs to v1 IDs
	for i := range req.Committees {
		if req.Committees[i].ID != "" {
			v1ID, err := s.idMapper.MapCommitteeV2ToV1(ctx, req.Committees[i].ID)
			if err != nil {
				return nil, err
			}
			req.Committees[i].ID = v1ID
		}
	}

	// Call ITX proxy
	resp, err := s.pastMeetingClient.CreatePastMeeting(ctx, req)
	if err != nil {
		return nil, err
	}

	// Map v1 project ID back to v2 UID in response
	if resp.ProjectID != "" {
		v2UID, err := s.idMapper.MapProjectV1ToV2(ctx, resp.ProjectID)
		if err != nil {
			return nil, err
		}
		resp.ProjectID = v2UID
	}

	// Map committee IDs back to v2 UIDs
	for i := range resp.Committees {
		if resp.Committees[i].ID != "" {
			v2UID, err := s.idMapper.MapCommitteeV1ToV2(ctx, resp.Committees[i].ID)
			if err != nil {
				return nil, err
			}
			resp.Committees[i].ID = v2UID
		}
	}

	return resp, nil
}

// GetPastMeeting retrieves a past meeting via ITX proxy
func (s *PastMeetingService) GetPastMeeting(ctx context.Context, pastMeetingID string) (*itx.PastMeetingResponse, error) {
	// Call ITX proxy
	resp, err := s.pastMeetingClient.GetPastMeeting(ctx, pastMeetingID)
	if err != nil {
		return nil, err
	}

	// Map v1 project ID back to v2 UID in response
	if resp.ProjectID != "" {
		v2UID, err := s.idMapper.MapProjectV1ToV2(ctx, resp.ProjectID)
		if err != nil {
			return nil, err
		}
		resp.ProjectID = v2UID
	}

	// Map committee IDs back to v2 UIDs
	for i := range resp.Committees {
		if resp.Committees[i].ID != "" {
			v2UID, err := s.idMapper.MapCommitteeV1ToV2(ctx, resp.Committees[i].ID)
			if err != nil {
				return nil, err
			}
			resp.Committees[i].ID = v2UID
		}
	}

	return resp, nil
}

// UpdatePastMeeting updates a past meeting via ITX proxy
func (s *PastMeetingService) UpdatePastMeeting(ctx context.Context, pastMeetingID string, req *itx.CreatePastMeetingRequest) (*itx.PastMeetingResponse, error) {
	// Map v2 project UID to v1 SFID before sending to ITX
	if req.ProjectID != "" {
		v1ID, err := s.idMapper.MapProjectV2ToV1(ctx, req.ProjectID)
		if err != nil {
			return nil, err
		}
		req.ProjectID = v1ID
	}

	// Map committee UIDs to v1 IDs
	for i := range req.Committees {
		if req.Committees[i].ID != "" {
			v1ID, err := s.idMapper.MapCommitteeV2ToV1(ctx, req.Committees[i].ID)
			if err != nil {
				return nil, err
			}
			req.Committees[i].ID = v1ID
		}
	}

	// Call ITX proxy
	resp, err := s.pastMeetingClient.UpdatePastMeeting(ctx, pastMeetingID, req)
	if err != nil {
		return nil, err
	}

	// Map v1 project ID back to v2 UID in response
	if resp.ProjectID != "" {
		v2UID, err := s.idMapper.MapProjectV1ToV2(ctx, resp.ProjectID)
		if err != nil {
			return nil, err
		}
		resp.ProjectID = v2UID
	}

	// Map committee IDs back to v2 UIDs
	for i := range resp.Committees {
		if resp.Committees[i].ID != "" {
			v2UID, err := s.idMapper.MapCommitteeV1ToV2(ctx, resp.Committees[i].ID)
			if err != nil {
				return nil, err
			}
			resp.Committees[i].ID = v2UID
		}
	}

	return resp, nil
}

// DeletePastMeeting deletes a past meeting via ITX proxy
func (s *PastMeetingService) DeletePastMeeting(ctx context.Context, pastMeetingID string) error {
	// Call ITX proxy
	return s.pastMeetingClient.DeletePastMeeting(ctx, pastMeetingID)
}
