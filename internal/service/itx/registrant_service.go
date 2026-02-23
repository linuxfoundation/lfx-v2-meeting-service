// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package itx

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/models/itx"
)

// RegistrantService handles ITX Zoom registrant operations
type RegistrantService struct {
	registrantClient domain.ITXRegistrantClient
	idMapper         domain.IDMapper
}

// NewRegistrantService creates a new ITX registrant service
func NewRegistrantService(registrantClient domain.ITXRegistrantClient, idMapper domain.IDMapper) *RegistrantService {
	return &RegistrantService{
		registrantClient: registrantClient,
		idMapper:         idMapper,
	}
}

// CreateRegistrant creates a meeting registrant via ITX proxy
func (s *RegistrantService) CreateRegistrant(ctx context.Context, meetingID string, req *itx.ZoomMeetingRegistrant) (*itx.ZoomMeetingRegistrant, error) {
	// Map committee UID to committee SFID if present
	if req.CommitteeID != "" {
		v1SFID, err := s.idMapper.MapCommitteeV2ToV1(ctx, req.CommitteeID)
		if err != nil {
			return nil, err
		}
		req.CommitteeID = v1SFID
	}

	// Call ITX proxy
	resp, err := s.registrantClient.CreateRegistrant(ctx, meetingID, req)
	if err != nil {
		return nil, err
	}

	// Map committee SFID back to committee UID if present
	if resp.CommitteeID != "" {
		v2UID, err := s.idMapper.MapCommitteeV1ToV2(ctx, resp.CommitteeID)
		if err != nil {
			return nil, err
		}
		resp.CommitteeID = v2UID
	}

	return resp, nil
}

// GetRegistrant retrieves a meeting registrant via ITX proxy
func (s *RegistrantService) GetRegistrant(ctx context.Context, meetingID, registrantID string) (*itx.ZoomMeetingRegistrant, error) {
	// Call ITX proxy
	resp, err := s.registrantClient.GetRegistrant(ctx, meetingID, registrantID)
	if err != nil {
		return nil, err
	}

	// Map committee SFID back to committee UID if present
	if resp.CommitteeID != "" {
		v2UID, err := s.idMapper.MapCommitteeV1ToV2(ctx, resp.CommitteeID)
		if err != nil {
			return nil, err
		}
		resp.CommitteeID = v2UID
	}

	return resp, nil
}

// UpdateRegistrant updates a meeting registrant via ITX proxy
func (s *RegistrantService) UpdateRegistrant(ctx context.Context, meetingID, registrantID string, req *itx.ZoomMeetingRegistrant) error {
	// Map committee UID to committee SFID if present
	if req.CommitteeID != "" {
		v1SFID, err := s.idMapper.MapCommitteeV2ToV1(ctx, req.CommitteeID)
		if err != nil {
			return err
		}
		req.CommitteeID = v1SFID
	}

	// Call ITX proxy
	return s.registrantClient.UpdateRegistrant(ctx, meetingID, registrantID, req)
}

// DeleteRegistrant deletes a meeting registrant via ITX proxy
func (s *RegistrantService) DeleteRegistrant(ctx context.Context, meetingID, registrantID string) error {
	// Call ITX proxy
	return s.registrantClient.DeleteRegistrant(ctx, meetingID, registrantID)
}

// GetRegistrantICS retrieves an ICS calendar file for a meeting registrant via ITX proxy
func (s *RegistrantService) GetRegistrantICS(ctx context.Context, meetingID, registrantID string) (*itx.RegistrantICS, error) {
	// Call ITX proxy
	return s.registrantClient.GetRegistrantICS(ctx, meetingID, registrantID)
}

// ResendRegistrantInvitation resends a meeting invitation to a registrant via ITX proxy
func (s *RegistrantService) ResendRegistrantInvitation(ctx context.Context, meetingID, registrantID string) error {
	// Call ITX proxy
	return s.registrantClient.ResendRegistrantInvitation(ctx, meetingID, registrantID)
}
