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
	if err := mapCommitteeFieldV2ToV1(ctx, s.idMapper, &req.CommitteeID); err != nil {
		return nil, err
	}

	// Stamp created_by from the authenticated principal so the registrant's audit
	// trail reflects who added them via the v2 API (M2M token to ITX would otherwise
	// leave this blank or attribute it to the service identity).
	req.CreatedBy = s.buildRequestingUser(ctx)

	resp, err := s.registrantClient.CreateRegistrant(ctx, meetingID, req)
	if err != nil {
		return nil, err
	}

	resp.CommitteeID = mapCommitteeFieldV1ToV2Graceful(ctx, s.idMapper, resp.CommitteeID,
		"failed to map committee ID in registrant response; returning empty committee UID")
	return resp, nil
}

// SelfRegisterForMeeting registers the authenticated user as a meeting registrant.
// Email is always sourced from the JWT claim (EmailContextID) or the auth-service profile;
// it is never accepted from req. Field-precedence rules for all other fields are documented
// on enrichRegistrantFromProfile.
// Returns an error if the user is already registered (ITX returns 409 Conflict).
func (s *RegistrantService) SelfRegisterForMeeting(ctx context.Context, meetingID string, req *itx.ZoomMeetingRegistrant) (*itx.ZoomMeetingRegistrant, error) {
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
	// calling it twice would double the latency and could yield an inconsistent stamp.
	var resolvedProfile *domain.UserProfile
	if s.userMetadata != nil {
		if profile, err := s.userMetadata.ResolveProfile(ctx, username); err != nil {
			slog.WarnContext(ctx, "failed to resolve user profile for self-registration enrichment; using request payload",
				"username", redaction.Redact(username), logging.ErrKey, err)
		} else {
			resolvedProfile = profile
		}
	}

	// Derive email from the JWT claim first; fall back to the profile email when the
	// token doesn't carry the claim (e.g. local dev, older tokens).
	email, _ := ctx.Value(constants.EmailContextID).(string)
	if email == "" && resolvedProfile != nil {
		email = resolvedProfile.Email
	}

	if err := enrichRegistrantFromProfile(req, resolvedProfile, email, username); err != nil {
		return nil, err
	}

	req.CreatedBy = s.buildRequestingUserFromProfile(ctx, resolvedProfile)

	return s.registrantClient.CreateRegistrant(ctx, meetingID, req)
}

// enrichRegistrantFromProfile applies auth-service profile data to a self-registration
// request following the canonical precedence rules:
//
//   - Email always comes from an authoritative source (JWT claim or profile); the
//     request body value is unconditionally overwritten to prevent identity spoofing.
//   - Username is always set from the JWT principal.
//   - FirstName, LastName, JobTitle, and Org: profile value wins when non-empty;
//     the request payload serves as fallback when the profile field is absent or
//     the lookup failed entirely (profile == nil).
//
// Returns a validation error when email is empty after resolution — the caller must
// not proceed without a verified identity.
func enrichRegistrantFromProfile(req *itx.ZoomMeetingRegistrant, profile *domain.UserProfile, email, username string) error {
	if email == "" {
		return domain.NewValidationError("authenticated user email is required for self-registration")
	}
	req.Email = email
	req.Username = username
	if profile != nil {
		if profile.FirstName != "" {
			req.FirstName = profile.FirstName
		}
		if profile.LastName != "" {
			req.LastName = profile.LastName
		}
		if profile.JobTitle != "" {
			req.JobTitle = profile.JobTitle
		}
		if profile.Organization != "" {
			req.Org = profile.Organization
		}
	}
	return nil
}

// GetRegistrant retrieves a meeting registrant via ITX proxy
func (s *RegistrantService) GetRegistrant(ctx context.Context, meetingID, registrantID string) (*itx.ZoomMeetingRegistrant, error) {
	resp, err := s.registrantClient.GetRegistrant(ctx, meetingID, registrantID)
	if err != nil {
		return nil, err
	}

	resp.CommitteeID = mapCommitteeFieldV1ToV2Graceful(ctx, s.idMapper, resp.CommitteeID,
		"failed to map committee ID in registrant response; returning empty committee UID")
	return resp, nil
}

// UpdateRegistrant updates a meeting registrant via ITX proxy
func (s *RegistrantService) UpdateRegistrant(ctx context.Context, meetingID, registrantID string, req *itx.ZoomMeetingRegistrant) error {
	if err := mapCommitteeFieldV2ToV1(ctx, s.idMapper, &req.CommitteeID); err != nil {
		return err
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
