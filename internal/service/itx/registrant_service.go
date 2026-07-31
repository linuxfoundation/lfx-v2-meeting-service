// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package itx

import (
	"context"
	"log/slog"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/models/itx"
)

// RegistrantService handles ITX Zoom registrant operations
type RegistrantService struct {
	auditStamper
	registrantClient domain.ITXRegistrantClient
	idMapper         domain.IDMapper
}

// NewRegistrantService creates a new ITX registrant service. userMetadata may be nil (e.g.
// when NATS is disabled), in which case created_by / updated_by are limited to the
// JWT-derived username/email rather than blocking the request.
func NewRegistrantService(registrantClient domain.ITXRegistrantClient, idMapper domain.IDMapper, userMetadata domain.UserMetadataReader) *RegistrantService {
	return &RegistrantService{
		auditStamper:     auditStamper{userMetadata: userMetadata},
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

	// Stamp created_by from the authenticated principal so the registrant's audit
	// trail reflects who added them via the v2 API (M2M token to ITX would otherwise
	// leave this blank or attribute it to the service identity).
	req.CreatedBy = s.buildRequestingUser(ctx)

	resp, err := s.registrantClient.CreateRegistrant(ctx, meetingID, req)
	if err != nil {
		return nil, err
	}

	// Map committee SFID back to committee UID if present. On any mapping failure, log a warning
	// and leave the committee UID empty so the caller still receives the full registrant response.
	if resp.CommitteeID != "" {
		v2UID, err := s.idMapper.MapCommitteeV1ToV2(ctx, resp.CommitteeID)
		if err != nil {
			slog.InfoContext(ctx, "failed to map committee ID in registrant response; returning empty committee UID",
				"v1_id", resp.CommitteeID, "err", err)
			resp.CommitteeID = ""
		} else {
			resp.CommitteeID = v2UID
		}
	}

	return resp, nil
}

// GetRegistrant retrieves a meeting registrant via ITX proxy
func (s *RegistrantService) GetRegistrant(ctx context.Context, meetingID, registrantID string) (*itx.ZoomMeetingRegistrant, error) {
	resp, err := s.registrantClient.GetRegistrant(ctx, meetingID, registrantID)
	if err != nil {
		return nil, err
	}

	// Map committee SFID back to committee UID if present. On any mapping failure, log a warning
	// and leave the committee UID empty so the caller still receives the full registrant response.
	if resp.CommitteeID != "" {
		v2UID, err := s.idMapper.MapCommitteeV1ToV2(ctx, resp.CommitteeID)
		if err != nil {
			slog.InfoContext(ctx, "failed to map committee ID in registrant response; returning empty committee UID",
				"v1_id", resp.CommitteeID, "err", err)
			resp.CommitteeID = ""
		} else {
			resp.CommitteeID = v2UID
		}
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

	// Stamp updated_by from the authenticated principal so ITX overwrites the stored
	// updated_by on the registrant record instead of preserving stale data.
	req.UpdatedBy = s.buildRequestingUser(ctx)

	return s.registrantClient.UpdateRegistrant(ctx, meetingID, registrantID, req)
}

// DeleteRegistrant deletes a meeting registrant via ITX proxy
func (s *RegistrantService) DeleteRegistrant(ctx context.Context, meetingID, registrantID string) error {
	return s.registrantClient.DeleteRegistrant(ctx, meetingID, registrantID)
}

// GetRegistrantICS retrieves an ICS calendar file for a meeting registrant via ITX proxy
func (s *RegistrantService) GetRegistrantICS(ctx context.Context, meetingID, registrantID string) (*itx.RegistrantICS, error) {
	return s.registrantClient.GetRegistrantICS(ctx, meetingID, registrantID)
}

// ResendRegistrantInvitation resends a meeting invitation to a registrant via ITX proxy
func (s *RegistrantService) ResendRegistrantInvitation(ctx context.Context, meetingID, registrantID string) error {
	return s.registrantClient.ResendRegistrantInvitation(ctx, meetingID, registrantID)
}
