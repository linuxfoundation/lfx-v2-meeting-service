// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/redaction"
)

// primaryEmailSentinel is the special email/email_id value that clears the override so the
// user's primary email is used (mirrors the myprofile "use primary" behavior).
const primaryEmailSentinel = "primary"

// PreferredEmailService orchestrates reading and writing a user's preferred
// meeting-invite email. It acts AS the user via their bearer token (forwarded by
// self-serve): it resolves the user from the token and delegates storage to the v1
// user-service (Phase 1).
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
func (s *PreferredEmailService) GetPreferredEmail(ctx context.Context, token string) (*domain.PreferredEmail, error) {
	self, err := s.getSelf(ctx, token)
	if err != nil {
		return nil, err
	}
	pref, err := s.userClient.GetMeetingEmailPreference(ctx, token, self.SFID)
	if err != nil {
		s.logger.WarnContext(ctx, "failed to get preferred meeting email",
			"user", redaction.Redact(self.SFID), logging.ErrKey, err)
		return nil, err
	}
	return pref, nil
}

// SetPreferredEmail sets the user's preferred meeting-invite email. The selection may be
// given as a verified email address (email) or an SFDC email-record ID (emailID); email
// takes precedence when non-empty. An empty selection, or the "primary" sentinel, clears
// the override, in which case a nil PreferredEmail is returned. An address is resolved to
// its (auth0→SFDC synced) email-record ID and must be an active, verified email.
func (s *PreferredEmailService) SetPreferredEmail(ctx context.Context, token, email, emailID string) (*domain.PreferredEmail, error) {
	self, err := s.getSelf(ctx, token)
	if err != nil {
		return nil, err
	}

	email = strings.TrimSpace(email)
	emailID = strings.TrimSpace(emailID)

	// A provided address fully determines the selection (takes precedence over email_id):
	// "primary" clears the override, any other address resolves to its SFDC email-record ID.
	if email != "" {
		if strings.EqualFold(email, primaryEmailSentinel) {
			emailID = ""
		} else {
			resolvedID, err := resolveVerifiedEmailID(self, email)
			if err != nil {
				s.logger.WarnContext(ctx, "failed to resolve email address to a verified record",
					"user", redaction.Redact(self.SFID), logging.ErrKey, err)
				return nil, err
			}
			emailID = resolvedID
		}
	}

	if emailID == "" || strings.EqualFold(emailID, primaryEmailSentinel) {
		if err := s.userClient.ClearMeetingEmailPreference(ctx, token, self.SFID); err != nil {
			s.logger.WarnContext(ctx, "failed to clear preferred meeting email",
				"user", redaction.Redact(self.SFID), logging.ErrKey, err)
			return nil, err
		}
		s.logger.InfoContext(ctx, "cleared preferred meeting email override", "user", redaction.Redact(self.SFID))
		return nil, nil
	}

	pref, err := s.userClient.SetMeetingEmailPreference(ctx, token, self.SFID, emailID)
	if err != nil {
		s.logger.WarnContext(ctx, "failed to set preferred meeting email",
			"user", redaction.Redact(self.SFID), logging.ErrKey, err)
		return nil, err
	}
	s.logger.InfoContext(ctx, "set preferred meeting email", "user", redaction.Redact(self.SFID))
	return pref, nil
}

// getSelf resolves the calling user from their bearer token.
func (s *PreferredEmailService) getSelf(ctx context.Context, token string) (*domain.Self, error) {
	if strings.TrimSpace(token) == "" {
		return nil, domain.NewValidationError("user token is required")
	}
	return s.userClient.GetSelf(ctx, token)
}

// resolveVerifiedEmailID finds the SFDC email-record ID for the given address among the
// user's emails. The address must be an active, verified record (invites must only route to
// a verified address): an address that doesn't match any record, or matches a record that
// already has an SFDC ID but isn't active/verified, is a ValidationError. A match with no SFDC
// ID yet returns a retryable UnavailableError wrapping domain.ErrEmailNotSynced regardless of
// its Active/Verified flags -- those can lag the auth0->SFDC sync just as much as the ID does,
// so a freshly-added address auth0 already reports as verified can briefly appear here with
// both flags still false. Treating that as a permanent validation failure (rather than
// retryable) was the root cause of lfx-self-serve-ops#1125: a verified secondary email was
// rejected outright instead of "try again shortly". Once a record has an ID, its Active/Verified
// flags are the final, synced word on whether the address can be selected.
func resolveVerifiedEmailID(self *domain.Self, email string) (string, error) {
	email = strings.TrimSpace(email)
	matched := false
	pendingSync := false
	for _, e := range self.Emails {
		if !strings.EqualFold(strings.TrimSpace(e.Address), email) {
			continue
		}
		matched = true
		if e.ID == "" {
			pendingSync = true
			continue
		}
		if e.Active && e.Verified {
			return e.ID, nil
		}
	}

	// Redact the address in the returned error — it propagates into logs via ErrKey.
	redactedEmail := redaction.RedactEmail(email)
	if pendingSync {
		return "", domain.NewUnavailableError(
			fmt.Sprintf("email %q not yet available in user-service; retry", redactedEmail),
			domain.ErrEmailNotSynced)
	}
	if matched {
		return "", domain.NewValidationError(
			fmt.Sprintf("email %q is not an active, verified address on this account", redactedEmail))
	}
	return "", domain.NewValidationError(
		fmt.Sprintf("email %q is not a recognized address on this account", redactedEmail))
}
