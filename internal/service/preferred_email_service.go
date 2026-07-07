// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"log/slog"
	"strings"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
)

// primaryEmailSentinel is the special email_id value that clears the override so the
// user's primary email is used (mirrors the myprofile "use primary" behavior).
const primaryEmailSentinel = "primary"

// PreferredEmailService orchestrates reading and writing a user's preferred
// meeting-invite email: it resolves the LFID/username to a Salesforce ID and then
// delegates storage to the v1 user-service (Phase 1).
type PreferredEmailService struct {
	userClient domain.UserServiceClient
	logger     *slog.Logger
}

// NewPreferredEmailService creates a new PreferredEmailService.
func NewPreferredEmailService(userClient domain.UserServiceClient, logger *slog.Logger) *PreferredEmailService {
	return &PreferredEmailService{userClient: userClient, logger: logger}
}

// GetPreferredEmail returns the user's preferred meeting-invite email, or nil when
// no override is set (use primary).
func (s *PreferredEmailService) GetPreferredEmail(ctx context.Context, username string) (*domain.PreferredEmail, error) {
	sfid, err := s.resolveSFID(ctx, username)
	if err != nil {
		return nil, err
	}
	return s.userClient.GetMeetingEmailPreference(ctx, sfid)
}

// SetPreferredEmail sets the user's preferred meeting-invite email.
//
// The selection may be given as a verified email address (email) or as an SFDC email
// record ID (emailID); email takes precedence when non-empty. An empty selection, or the
// "primary" sentinel, clears the override, in which case a nil PreferredEmail is returned.
// When an address is given it is resolved to its (auth0→SFDC synced) email-record ID; a
// not-yet-synced address surfaces the client's retryable error.
func (s *PreferredEmailService) SetPreferredEmail(ctx context.Context, username, email, emailID string) (*domain.PreferredEmail, error) {
	sfid, err := s.resolveSFID(ctx, username)
	if err != nil {
		return nil, err
	}

	email = strings.TrimSpace(email)
	emailID = strings.TrimSpace(emailID)

	// An address takes precedence and is resolved to its SFDC email-record ID.
	if email != "" && !strings.EqualFold(email, primaryEmailSentinel) {
		resolvedID, err := s.userClient.ResolveEmailID(ctx, sfid, email)
		if err != nil {
			return nil, err
		}
		emailID = resolvedID
	}

	if emailID == "" || strings.EqualFold(emailID, primaryEmailSentinel) {
		if err := s.userClient.ClearMeetingEmailPreference(ctx, sfid); err != nil {
			return nil, err
		}
		return nil, nil
	}

	return s.userClient.SetMeetingEmailPreference(ctx, sfid, emailID)
}

// resolveSFID resolves an LFID/username to a Salesforce ID.
func (s *PreferredEmailService) resolveSFID(ctx context.Context, username string) (string, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return "", domain.NewValidationError("user is required")
	}
	return s.userClient.ResolveSFIDByUsername(ctx, username)
}
