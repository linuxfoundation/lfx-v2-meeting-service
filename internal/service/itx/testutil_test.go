// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package itx

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/constants"
)

// Shared test helpers for ITX service package tests. Keeping these in one file lets
// each per-service test file focus on the stamping behavior specific to it.

// noOpIDMapper passes IDs through unchanged. Individual test files may override
// specific methods by declaring a local mapper type that embeds this.
type noOpIDMapper struct{ domain.IDMapper }

func (noOpIDMapper) MapProjectV2ToV1(_ context.Context, v2UID string) (string, error) {
	return v2UID, nil
}
func (noOpIDMapper) MapProjectV1ToV2(_ context.Context, v1SFID string) (string, error) {
	return v1SFID, nil
}
func (noOpIDMapper) MapCommitteeV2ToV1(_ context.Context, v2UID string) (string, error) {
	return v2UID, nil
}
func (noOpIDMapper) MapCommitteeV1ToV2(_ context.Context, v1SFID string) (string, error) {
	return v1SFID, nil
}
func (noOpIDMapper) MapParticipantV2ToInviteeID(_ context.Context, v2UID string) (string, error) {
	return v2UID, nil
}
func (noOpIDMapper) MapParticipantV2ToAttendeeID(_ context.Context, v2UID string) (string, error) {
	return v2UID, nil
}
func (noOpIDMapper) MapInviteeIDToParticipantV2(_ context.Context, inviteeID string) (string, error) {
	return inviteeID, nil
}
func (noOpIDMapper) MapAttendeeIDToParticipantV2(_ context.Context, attendeeID string) (string, error) {
	return attendeeID, nil
}

// fakeUserMetadataReader returns a canned profile or error for ResolveProfile.
type fakeUserMetadataReader struct {
	profile *domain.UserProfile
	err     error
	calls   []string
}

func (f *fakeUserMetadataReader) ResolveProfile(_ context.Context, username string) (*domain.UserProfile, error) {
	f.calls = append(f.calls, username)
	if f.err != nil {
		return nil, f.err
	}
	return f.profile, nil
}

// ctxWithPrincipal builds a context matching what the JWT auth middleware installs
// on a real request.
func ctxWithPrincipal(principal, email string) context.Context {
	ctx := context.WithValue(context.Background(), constants.PrincipalContextID, principal)
	if email != "" {
		ctx = context.WithValue(ctx, constants.EmailContextID, email)
	}
	return ctx
}
