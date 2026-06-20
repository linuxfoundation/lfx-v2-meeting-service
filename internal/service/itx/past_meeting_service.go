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
	if err := mapProjectFieldV2ToV1(ctx, s.idMapper, &req.ProjectID); err != nil {
		return nil, err
	}
	if err := mapITXCommitteesV2ToV1(ctx, s.idMapper, req.Committees); err != nil {
		return nil, err
	}

	resp, err := s.pastMeetingClient.CreatePastMeeting(ctx, req)
	if err != nil {
		return nil, err
	}

	if err := mapProjectFieldV1ToV2(ctx, s.idMapper, &resp.ProjectID); err != nil {
		return nil, err
	}
	mapITXCommitteesV1ToV2Graceful(ctx, s.idMapper, resp.Committees,
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
	mapITXCommitteesV1ToV2Graceful(ctx, s.idMapper, resp.Committees,
		"failed to map committee ID in past meeting response; returning empty committee UID")
	return resp, nil
}

// UpdatePastMeeting updates a past meeting via ITX proxy
func (s *PastMeetingService) UpdatePastMeeting(ctx context.Context, pastMeetingID string, req *itx.CreatePastMeetingRequest) (*itx.PastMeetingResponse, error) {
	if err := mapProjectFieldV2ToV1(ctx, s.idMapper, &req.ProjectID); err != nil {
		return nil, err
	}
	if err := mapITXCommitteesV2ToV1(ctx, s.idMapper, req.Committees); err != nil {
		return nil, err
	}

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
