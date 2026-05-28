// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package itx

import (
	"context"
	"log/slog"
	"strings"
	"time"

	inviteapi "github.com/linuxfoundation/lfx-v2-invite-service/pkg/api"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/constants"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/models/itx"
)

// RegistrantService handles ITX Zoom registrant operations.
type RegistrantService struct {
	registrantClient     domain.ITXRegistrantClient
	idMapper             domain.IDMapper
	userReader           domain.UserReader
	inviteSender         domain.InviteSender
	selfServeBaseURL     string
	inviteFeatureEnabled bool
}

// Option configures optional invite-related dependencies on RegistrantService.
type Option func(*RegistrantService)

// WithUserReader sets the NATS-backed user-reader for email → Auth0-sub lookups.
func WithUserReader(ur domain.UserReader) Option {
	return func(s *RegistrantService) { s.userReader = ur }
}

// WithInviteSender sets the NATS-backed invite sender.
func WithInviteSender(is domain.InviteSender) Option {
	return func(s *RegistrantService) { s.inviteSender = is }
}

// WithSelfServeBaseURL sets the LFX self-serve base URL included in invite emails.
func WithSelfServeBaseURL(u string) Option {
	return func(s *RegistrantService) { s.selfServeBaseURL = u }
}

// WithInviteFeatureEnabled controls whether the invite flow is active (default false).
func WithInviteFeatureEnabled(enabled bool) Option {
	return func(s *RegistrantService) { s.inviteFeatureEnabled = enabled }
}

// NewRegistrantService creates a new ITX registrant service.
// The invite feature is disabled by default when no options are provided;
// supply WithInviteFeatureEnabled(true) along with WithUserReader and WithInviteSender to activate it.
func NewRegistrantService(registrantClient domain.ITXRegistrantClient, idMapper domain.IDMapper, opts ...Option) *RegistrantService {
	s := &RegistrantService{
		registrantClient:     registrantClient,
		idMapper:             idMapper,
		inviteFeatureEnabled: false,
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

// CreateRegistrant creates a meeting registrant via ITX proxy.
// If the request has no Username and the invite feature is enabled, the service
// attempts to resolve the email via auth-service (NATS). On hit it sets the
// username directly. On miss it creates the registrant and then issues an LFID
// invite, storing invite metadata on the ITX record.
func (s *RegistrantService) CreateRegistrant(ctx context.Context, meetingID string, req *itx.ZoomMeetingRegistrant) (*itx.ZoomMeetingRegistrant, error) {
	// Map committee UID to committee SFID if present.
	if req.CommitteeID != "" {
		v1SFID, err := s.idMapper.MapCommitteeV2ToV1(ctx, req.CommitteeID)
		if err != nil {
			return nil, err
		}
		req.CommitteeID = v1SFID
	}

	// --- LFID invite flow ---
	if s.inviteFeatureEnabled && req.Username == "" && req.Email != "" {
		sub, err := s.lookupAuthSub(ctx, req.Email)
		switch {
		case err == nil:
			req.Username = sub
		case domain.GetErrorType(err) == domain.ErrorTypeNotFound:
			// No LFID for this email — proceed; invite will be issued after ITX create.
		default:
			// Transport / timeout — fail the request (same policy as project-service).
			return nil, domain.NewUnavailableError("failed to resolve LFID for email", err)
		}
	}

	resp, err := s.registrantClient.CreateRegistrant(ctx, meetingID, req)
	if err != nil {
		return nil, err
	}

	// Map committee SFID back to committee UID if present.
	if resp.CommitteeID != "" {
		v2UID, err := s.idMapper.MapCommitteeV1ToV2(ctx, resp.CommitteeID)
		if err != nil {
			slog.WarnContext(ctx, "failed to map committee ID in registrant response; returning empty committee UID",
				"v1_id", resp.CommitteeID, "err", err)
			resp.CommitteeID = ""
		} else {
			resp.CommitteeID = v2UID
		}
	}

	// Issue LFID invite if the registrant still has no username after the ITX round-trip.
	if s.inviteFeatureEnabled && s.inviteSender != nil && resp.Username == "" && resp.Email != "" {
		s.issueInvite(ctx, meetingID, resp)
	}

	return resp, nil
}

// lookupAuthSub resolves the Auth0 sub for an email, returning ErrUserNotFound when
// no LFID exists or a transport error otherwise.
func (s *RegistrantService) lookupAuthSub(ctx context.Context, email string) (string, error) {
	if s.userReader == nil {
		return "", domain.ErrUserNotFound
	}
	return s.userReader.SubByEmail(ctx, email)
}

// issueInvite calls the invite service and, on success, writes the invite metadata
// back to the ITX registrant. Errors are logged but never surfaced to the caller —
// the registrant already exists in ITX; the invite lifecycle is best-effort.
func (s *RegistrantService) issueInvite(ctx context.Context, meetingID string, reg *itx.ZoomMeetingRegistrant) {
	name := strings.TrimSpace(reg.FirstName + " " + reg.LastName)
	result, err := s.inviteSender.SendInvite(ctx, inviteapi.SendInviteRequest{
		RecipientEmail: reg.Email,
		RecipientName:  name,
		ResourceUID:    meetingID,
		ResourceName:   meetingID,
		ResourceType:   constants.ResourceTypeMeeting,
		Role:           "Member",
		ReturnURL:      s.selfServeBaseURL,
		ExpirationDays: 30,
	})
	if err != nil {
		slog.WarnContext(ctx, "failed to send LFID invite for registrant",
			"registrant_id", reg.ID, "email", reg.Email, "meeting_id", meetingID, "err", err)
		return
	}

	expiresAt := ""
	if !result.ExpiresAt.IsZero() {
		expiresAt = result.ExpiresAt.Format(time.RFC3339)
	}
	if itxErr := s.registrantClient.UpdateRegistrantInvite(ctx, reg.ID, domain.ITXRegistrantInviteFields{
		LFIDInviteUID:       result.InviteUID,
		LFIDInviteEmail:     reg.Email,
		LFIDInviteExpiresAt: expiresAt,
	}); itxErr != nil {
		slog.WarnContext(ctx, "failed to store invite fields on ITX registrant",
			"registrant_id", reg.ID, "invite_uid", result.InviteUID, "err", itxErr)
		return
	}

	slog.DebugContext(ctx, "LFID invite issued for registrant",
		"registrant_id", reg.ID, "invite_uid", result.InviteUID, "meeting_id", meetingID)
}

// ReconcileAcceptedInvite is called by the InviteAcceptedSubscriber when invite-service
// publishes an acceptance event. It calls ITX to fan out the username update across all
// registrants with the given invite email, then returns the updated registrants so the
// caller can publish FGA + indexer events.
func (s *RegistrantService) ReconcileAcceptedInvite(ctx context.Context, email, username string) ([]*itx.ZoomMeetingRegistrant, error) {
	result, err := s.registrantClient.AcceptInvite(ctx, email, username)
	if err != nil {
		return nil, err
	}
	if len(result.Updated) == 0 {
		slog.DebugContext(ctx, "accept-invite: no registrants matched; already reconciled or not ours",
			"email", email)
		return nil, nil
	}
	slog.InfoContext(ctx, "accept-invite: reconciled registrants",
		"count", len(result.Updated), "email", email, "username", username)
	return result.Updated, nil
}

// GetRegistrant retrieves a meeting registrant via ITX proxy.
func (s *RegistrantService) GetRegistrant(ctx context.Context, meetingID, registrantID string) (*itx.ZoomMeetingRegistrant, error) {
	resp, err := s.registrantClient.GetRegistrant(ctx, meetingID, registrantID)
	if err != nil {
		return nil, err
	}

	if resp.CommitteeID != "" {
		v2UID, err := s.idMapper.MapCommitteeV1ToV2(ctx, resp.CommitteeID)
		if err != nil {
			slog.WarnContext(ctx, "failed to map committee ID in registrant response; returning empty committee UID",
				"v1_id", resp.CommitteeID, "err", err)
			resp.CommitteeID = ""
		} else {
			resp.CommitteeID = v2UID
		}
	}

	return resp, nil
}

// UpdateRegistrant updates a meeting registrant via ITX proxy.
func (s *RegistrantService) UpdateRegistrant(ctx context.Context, meetingID, registrantID string, req *itx.ZoomMeetingRegistrant) error {
	if req.CommitteeID != "" {
		v1SFID, err := s.idMapper.MapCommitteeV2ToV1(ctx, req.CommitteeID)
		if err != nil {
			return err
		}
		req.CommitteeID = v1SFID
	}
	return s.registrantClient.UpdateRegistrant(ctx, meetingID, registrantID, req)
}

// DeleteRegistrant deletes a meeting registrant via ITX proxy.
func (s *RegistrantService) DeleteRegistrant(ctx context.Context, meetingID, registrantID string) error {
	return s.registrantClient.DeleteRegistrant(ctx, meetingID, registrantID)
}

// GetRegistrantICS retrieves an ICS calendar file for a meeting registrant via ITX proxy.
func (s *RegistrantService) GetRegistrantICS(ctx context.Context, meetingID, registrantID string) (*itx.RegistrantICS, error) {
	return s.registrantClient.GetRegistrantICS(ctx, meetingID, registrantID)
}

// ResendRegistrantInvitation resends a meeting invitation to a registrant via ITX proxy.
func (s *RegistrantService) ResendRegistrantInvitation(ctx context.Context, meetingID, registrantID string) error {
	return s.registrantClient.ResendRegistrantInvitation(ctx, meetingID, registrantID)
}
