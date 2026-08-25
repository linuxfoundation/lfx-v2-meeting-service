// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package itx

import (
	"context"
	"log/slog"
	"strings"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/constants"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/models/itx"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/redaction"
)

// RegistrantService handles ITX Zoom registrant operations
type RegistrantService struct {
	auditStamper
	registrantClient domain.ITXRegistrantClient
	meetingClient    domain.ITXMeetingClient
	idMapper         domain.IDMapper
}

// NewRegistrantService creates a new ITX registrant service. userMetadata may be nil (e.g.
// when NATS is disabled), in which case created_by / updated_by are limited to the
// JWT-derived username/email rather than blocking the request.
func NewRegistrantService(registrantClient domain.ITXRegistrantClient, meetingClient domain.ITXMeetingClient, idMapper domain.IDMapper, userMetadata domain.UserMetadataReader) *RegistrantService {
	return &RegistrantService{
		auditStamper:     auditStamper{userMetadata: userMetadata},
		registrantClient: registrantClient,
		meetingClient:    meetingClient,
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
			slog.WarnContext(ctx, "failed to map committee ID in registrant response; returning empty committee UID",
				"v1_id", redaction.Redact(resp.CommitteeID), logging.ErrKey, err)
			resp.CommitteeID = ""
		} else {
			resp.CommitteeID = v2UID
		}
	}

	return resp, nil
}

// SelfRegisterForMeeting registers the authenticated user as a meeting registrant.
// The caller's email is sourced from the JWT claim on ctx (EmailContextID) first; if the JWT
// omits the email claim (e.g. use_oidc_contextualizer is disabled), it falls back to the
// auth-service profile resolved via NATS. Email is never accepted from req.
// All other fields in req (first_name, last_name, org, job_title, occurrence) are used as-is.
// Returns an error if the user is already registered (ITX returns 409 Conflict).
func (s *RegistrantService) SelfRegisterForMeeting(ctx context.Context, meetingID string, req *itx.ZoomMeetingRegistrant) (*itx.ZoomMeetingRegistrant, error) {
	// Derive email from an authoritative source (JWT claim or auth-service profile) rather
	// than accepting it from the request body — prevents a caller from self-registering
	// under a different identity.
	meeting, err := s.meetingClient.GetZoomMeeting(ctx, meetingID)
	if err != nil {
		return nil, err
	}
	if meeting.Visibility != itx.MeetingVisibilityPublic {
		return nil, domain.NewForbiddenError("self-registration is only available for public meetings")
	}

	username, _ := ctx.Value(constants.PrincipalContextID).(string)
	// M2M client principals carry an @clients suffix (e.g. "<id>@clients") and have no
	// associated LFX user account. Self-registration requires a human identity.
	if strings.HasSuffix(username, "@clients") {
		return nil, domain.NewValidationError("self-registration requires a user token, not an M2M client token")
	}

	// Resolve the user profile once and reuse it for both field enrichment and the
	// audit stamp. Each ResolveProfile call is a NATS request with a 2s timeout;
	// calling it twice would double the latency and could yield an inconsistent stamp
	// (enrichment fields from profile A, CreatedBy username/email only from a
	// timed-out profile B).
	var resolvedProfile *domain.UserProfile
	if s.userMetadata != nil {
		if profile, err := s.userMetadata.ResolveProfile(ctx, username); err != nil {
			slog.WarnContext(ctx, "failed to resolve user profile for self-registration enrichment; using request payload",
				"username", redaction.Redact(username), logging.ErrKey, err)
		} else {
			resolvedProfile = profile
		}
	}

	// Email must come from an authoritative source — JWT claim or auth-service profile —
	// not from the request body, to prevent a caller from self-registering under a
	// different identity. The JWT email claim is omitempty; fall back to the profile
	// email when the token doesn't carry the claim (e.g. local dev, older tokens).
	email, _ := ctx.Value(constants.EmailContextID).(string)
	if email == "" && resolvedProfile != nil {
		email = resolvedProfile.Email
	}
	if email == "" {
		return nil, domain.NewValidationError("authenticated user email is required for self-registration")
	}
	req.Email = email
	req.Username = username

	// Auth service data is authoritative over what the client sent; the request
	// payload serves as fallback when a field is absent from the profile or the
	// lookup failed entirely.
	if resolvedProfile != nil {
		if resolvedProfile.FirstName != "" {
			req.FirstName = resolvedProfile.FirstName
		}
		if resolvedProfile.LastName != "" {
			req.LastName = resolvedProfile.LastName
		}
		if resolvedProfile.JobTitle != "" {
			req.JobTitle = resolvedProfile.JobTitle
		}
		if resolvedProfile.Organization != "" {
			req.Org = resolvedProfile.Organization
		}
	}

	req.CreatedBy = s.buildRequestingUserFromProfile(ctx, resolvedProfile)

	return s.registrantClient.CreateRegistrant(ctx, meetingID, req)
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
			slog.WarnContext(ctx, "failed to map committee ID in registrant response; returning empty committee UID",
				"v1_id", redaction.Redact(resp.CommitteeID), logging.ErrKey, err)
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
