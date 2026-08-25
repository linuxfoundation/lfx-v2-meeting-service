// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package itx

import (
	"context"
	"log/slog"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/constants"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/models/itx"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/redaction"
)

// auditStamper resolves the requesting principal (stashed on ctx by the JWT auth
// middleware) into an *itx.User suitable for stamping created_by / updated_by /
// modified_by on outbound ITX requests. It is embedded by each ITX service so the
// stamping logic (and its graceful-degradation guarantees) lives in one place.
//
// Concretely, callers get one of:
//   - full profile ({username, name, email, profile_picture}) — happy path, resolved
//     via UserMetadataReader (NATS -> auth-service).
//   - {username, email} only — when NATS isn't wired (userMetadata == nil) or the
//     resolver returns an error. Never blocks the caller's request.
//   - nil — when there is no principal on ctx (e.g. unauthenticated internal call).
//     Downstream stamping is a no-op in that case.
//
// The resolved-profile email is preferred over the JWT-claimed email because JWT
// claims can be stale on long-lived tokens, while the auth-service view is authoritative.
type auditStamper struct {
	userMetadata domain.UserMetadataReader
}

// buildRequestingUser resolves the requesting user's identity from ctx into an
// *itx.User. See auditStamper docs for the full contract.
func (a auditStamper) buildRequestingUser(ctx context.Context) *itx.User {
	principal, _ := ctx.Value(constants.PrincipalContextID).(string)
	if principal == "" {
		return nil
	}
	email, _ := ctx.Value(constants.EmailContextID).(string)

	if a.userMetadata == nil {
		return &itx.User{Username: principal, Email: email}
	}

	profile, err := a.userMetadata.ResolveProfile(ctx, principal)
	if err != nil {
		slog.WarnContext(ctx, "failed to resolve user profile for audit stamp; stamping username/email only",
			"username", redaction.Redact(principal), logging.ErrKey, err)
		return &itx.User{Username: principal, Email: email}
	}
	// Defensive: the current NATSUserMetadataReader always returns either a
	// populated profile or a non-nil error, but the domain.UserMetadataReader
	// interface doesn't forbid (nil, nil). Dereferencing a nil profile would panic
	// and take down the outbound ITX write — degrade to username/email instead so
	// the write still succeeds with a minimal audit stamp.
	if profile == nil {
		slog.WarnContext(ctx, "user profile lookup returned no profile; stamping username/email only",
			"username", redaction.Redact(principal))
		return &itx.User{Username: principal, Email: email}
	}

	user := &itx.User{
		Username:       principal,
		Name:           profile.Name,
		Email:          profile.Email,
		ProfilePicture: profile.AvatarURL,
	}
	if user.Email == "" {
		user.Email = email
	}
	return user
}

// buildRequestingUserFromProfile constructs an *itx.User from an already-resolved
// *domain.UserProfile, skipping the NATS round-trip. Use this when the caller has
// already called ResolveProfile (e.g. for field enrichment) and wants to reuse the
// result for the audit stamp without a second lookup.
func (a auditStamper) buildRequestingUserFromProfile(ctx context.Context, profile *domain.UserProfile) *itx.User {
	principal, _ := ctx.Value(constants.PrincipalContextID).(string)
	if principal == "" {
		return nil
	}
	email, _ := ctx.Value(constants.EmailContextID).(string)
	if profile == nil {
		return &itx.User{Username: principal, Email: email}
	}
	user := &itx.User{
		Username:       principal,
		Name:           profile.Name,
		Email:          profile.Email,
		ProfilePicture: profile.AvatarURL,
	}
	if user.Email == "" {
		user.Email = email
	}
	return user
}

// buildRequestingCreatedUpdatedBy is the same as buildRequestingUser but returns an
// *itx.CreatedUpdatedBy — the audit-user shape used by attachment endpoints, which
// omit id and profile_picture. Returns nil when there is no principal on ctx.
func (a auditStamper) buildRequestingCreatedUpdatedBy(ctx context.Context) *itx.CreatedUpdatedBy {
	u := a.buildRequestingUser(ctx)
	if u == nil {
		return nil
	}
	return &itx.CreatedUpdatedBy{
		Username: u.Username,
		Email:    u.Email,
		Name:     u.Name,
	}
}
