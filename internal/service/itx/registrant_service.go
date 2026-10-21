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
	userReader       domain.UserReader
}

// NewRegistrantService creates a new ITX registrant service. userMetadata may be nil (e.g.
// when NATS is disabled), in which case created_by / updated_by are limited to the
// JWT-derived username/email rather than blocking the request. userReader may also be nil,
// in which case self-registration falls back to the raw JWT principal as the username.
func NewRegistrantService(registrantClient domain.ITXRegistrantClient, meetingClient domain.ITXMeetingClient, idMapper domain.IDMapper, userMetadata domain.UserMetadataReader, userReader domain.UserReader) *RegistrantService {
	return &RegistrantService{
		auditStamper:     auditStamper{userMetadata: userMetadata},
		registrantClient: registrantClient,
		meetingClient:    meetingClient,
		idMapper:         idMapper,
		userReader:       userReader,
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

	// The JWT principal is an auth identifier (e.g. an Auth0 "sub" claim like
	// "auth0|<id>"), not necessarily the user's LFX/LFID username — and
	// ResolveProfile's username is only an enrichment lookup key, echoed back
	// unchanged, never a resolved value. ITX looks up registrants by LFX
	// username, so resolve the real one from the verified email via the
	// auth-service's email_to_username lookup, falling back to the raw
	// principal when no reader is wired or the lookup fails, to avoid ITX
	// rejecting the registration with "invalid LFX username"
	// (linuxfoundation/lfx-self-serve#3134).
	if s.userReader != nil && email != "" {
		if resolvedUsername, err := s.userReader.UsernameByEmail(ctx, email); err != nil {
			slog.WarnContext(ctx, "failed to resolve LFX username from email for self-registration; using JWT principal",
				"username", redaction.Redact(username), logging.ErrKey, err)
		} else if resolvedUsername != "" {
			username = resolvedUsername
		}
	}

	if err := enrichRegistrantFromProfile(req, resolvedProfile, email, username); err != nil {
		return nil, err
	}

	req.CreatedBy = s.buildRequestingUserFromProfile(ctx, resolvedProfile)
	if req.CreatedBy != nil {
		req.CreatedBy.Username = username
	}

	return s.registrantClient.CreateRegistrant(ctx, meetingID, req)
}

// enrichRegistrantFromProfile applies auth-service profile data to a self-registration
// request following the canonical precedence rules:
//
//   - Email always comes from an authoritative source (JWT claim or profile); the
//     request body value is unconditionally overwritten to prevent identity spoofing.
//   - Username is the LFX username resolved from the verified email via
//     UserReader.UsernameByEmail when available, falling back to the raw JWT
//     principal when no reader is wired or the lookup failed (caller resolves this
//     precedence before calling in).
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

// SelfUnregisterFromMeeting removes the authenticated user's own registrant record from a meeting.
// The caller supplies their registrant_id; the service fetches the record, verifies the identity
// matches the JWT principal (email or username), and only then deletes it. M2M tokens are rejected.
// The delete is flagged as a self removal, so ITX also removes a committee registrant and records
// an exclusion that keeps committee syncs from adding the user back.
func (s *RegistrantService) SelfUnregisterFromMeeting(ctx context.Context, meetingID, registrantID string) error {
	username, email, err := s.verifyOwnRegistrant(ctx, meetingID, registrantID, selfUnregisterMessages)
	if err != nil {
		return err
	}

	// Tell ITX this is the registrant removing themselves. ITX checks updated_by against the
	// registrant again, so send the same username and email the ownership check above used rather
	// than a re-resolved profile that could differ from it.
	return s.registrantClient.DeleteRegistrant(ctx, meetingID, registrantID, &itx.DeleteRegistrantRequest{
		SelfRemoval: true,
		UpdatedBy:   &itx.User{Username: username, Email: email},
	})
}

// SelfResendRegistrantInvitation resends the meeting invitation to the authenticated user's own
// registrant record. Like SelfUnregisterFromMeeting, the caller supplies their registrant_id and the
// service verifies it belongs to the JWT principal (email or username) before acting; M2M tokens are
// rejected. The ownership check is what makes it safe behind a viewer-level gateway rule, unlike the
// organizer-only ResendRegistrantInvitation: the invitation can only go to a registrant record that
// belongs to the caller, so the endpoint cannot be used to email anyone else.
func (s *RegistrantService) SelfResendRegistrantInvitation(ctx context.Context, meetingID, registrantID string) error {
	if _, _, err := s.verifyOwnRegistrant(ctx, meetingID, registrantID, selfResendMessages); err != nil {
		return err
	}

	return s.registrantClient.ResendRegistrantInvitation(ctx, meetingID, registrantID)
}

// selfActionMessages words the errors and the log line of verifyOwnRegistrant for the self-service
// action that calls it, so each action keeps its own client-facing message.
type selfActionMessages struct {
	// unauthenticated is the 403 message when the request carries no principal.
	unauthenticated string
	// m2mToken is the 400 message when the principal is an M2M client rather than a user.
	m2mToken string
	// ownershipFailed is the log line written when the registrant belongs to someone else.
	ownershipFailed string
}

var (
	selfUnregisterMessages = selfActionMessages{
		unauthenticated: "must be authenticated to unregister from a meeting",
		m2mToken:        "self-unregistration requires a user token, not an M2M client token",
		ownershipFailed: "self-unregister ownership check failed: registrant does not belong to principal",
	}
	selfResendMessages = selfActionMessages{
		unauthenticated: "must be authenticated to resend a meeting invitation",
		m2mToken:        "self-resend requires a user token, not an M2M client token",
		ownershipFailed: "self-resend ownership check failed: registrant does not belong to principal",
	}
)

// verifyOwnRegistrant checks that the registrant record belongs to the authenticated user and returns
// the username and email the check used. It rejects a request with no principal (403) and an M2M client
// principal (400) before reading anything, then loads the registrant and requires its email
// (case-insensitive) or username to match the JWT principal, returning 403 when neither does. The email
// comes from the JWT claim, falling back to the NATS user profile when the token carries none.
func (s *RegistrantService) verifyOwnRegistrant(ctx context.Context, meetingID, registrantID string, msgs selfActionMessages) (username, email string, err error) {
	username, _ = ctx.Value(constants.PrincipalContextID).(string)
	if username == "" {
		return "", "", domain.NewForbiddenError(msgs.unauthenticated)
	}
	if strings.HasSuffix(username, "@clients") {
		return "", "", domain.NewValidationError(msgs.m2mToken)
	}

	registrant, err := s.registrantClient.GetRegistrant(ctx, meetingID, registrantID)
	if err != nil {
		return "", "", err
	}

	email, _ = ctx.Value(constants.EmailContextID).(string)
	if email == "" && s.userMetadata != nil {
		if profile, resolveErr := s.userMetadata.ResolveProfile(ctx, username); resolveErr == nil && profile != nil && profile.Email != "" {
			email = profile.Email
		}
	}

	emailMatch := email != "" && strings.EqualFold(registrant.Email, email)
	usernameMatch := registrant.Username == username
	if !emailMatch && !usernameMatch {
		slog.WarnContext(ctx, msgs.ownershipFailed,
			"principal", redaction.Redact(username),
			"registrant_id", registrantID)
		return "", "", domain.NewForbiddenError("registrant does not belong to the authenticated user")
	}

	return username, email, nil
}

// DeleteRegistrant deletes a meeting registrant via ITX proxy
func (s *RegistrantService) DeleteRegistrant(ctx context.Context, meetingID, registrantID string) error {
	return s.registrantClient.DeleteRegistrant(ctx, meetingID, registrantID, nil)
}

// GetRegistrantICS retrieves an ICS calendar file for a meeting registrant via ITX proxy
func (s *RegistrantService) GetRegistrantICS(ctx context.Context, meetingID, registrantID string) (*itx.RegistrantICS, error) {
	return s.registrantClient.GetRegistrantICS(ctx, meetingID, registrantID)
}

// ResendRegistrantInvitation resends a meeting invitation to a registrant via ITX proxy
func (s *RegistrantService) ResendRegistrantInvitation(ctx context.Context, meetingID, registrantID string) error {
	return s.registrantClient.ResendRegistrantInvitation(ctx, meetingID, registrantID)
}
