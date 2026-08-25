// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package itx

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
)

// The audit stamper's contract is exercised in depth by the MeetingService tests
// (create/update/occurrence). These tests pin down the shape the shared helper
// itself returns — in particular the *itx.CreatedUpdatedBy conversion used by
// the attachment services — so a regression there doesn't have to be found through
// an attachment-level test.

func TestAuditStamper_BuildRequestingUser(t *testing.T) {
	t.Run("returns nil when no principal", func(t *testing.T) {
		a := auditStamper{}
		assert.Nil(t, a.buildRequestingUser(context.Background()))
	})

	t.Run("username-only fallback when reader is nil", func(t *testing.T) {
		a := auditStamper{}
		got := a.buildRequestingUser(ctxWithPrincipal("alice", "alice@example.com"))
		if assert.NotNil(t, got) {
			assert.Equal(t, "alice", got.Username)
			assert.Equal(t, "alice@example.com", got.Email)
			assert.Empty(t, got.Name)
			assert.Empty(t, got.ProfilePicture)
		}
	})

	t.Run("full profile when reader resolves", func(t *testing.T) {
		reader := &fakeUserMetadataReader{profile: &domain.UserProfile{
			Username:  "alice",
			Name:      "Alice Example",
			Email:     "alice@example.com",
			AvatarURL: "https://example.com/a.jpg",
		}}
		a := auditStamper{userMetadata: reader}
		got := a.buildRequestingUser(ctxWithPrincipal("alice", "alice@heimdall.example.com"))
		if assert.NotNil(t, got) {
			assert.Equal(t, "alice", got.Username)
			assert.Equal(t, "Alice Example", got.Name)
			// Resolved-profile email wins over JWT-claimed email (which may be stale).
			assert.Equal(t, "alice@example.com", got.Email)
			assert.Equal(t, "https://example.com/a.jpg", got.ProfilePicture)
		}
	})

	t.Run("degrades to username/email when resolver errors", func(t *testing.T) {
		reader := &fakeUserMetadataReader{err: errors.New("boom")}
		a := auditStamper{userMetadata: reader}
		got := a.buildRequestingUser(ctxWithPrincipal("bob", "bob@example.com"))
		if assert.NotNil(t, got) {
			assert.Equal(t, "bob", got.Username)
			assert.Equal(t, "bob@example.com", got.Email)
			assert.Empty(t, got.Name)
		}
	})

	t.Run("degrades to username/email when resolver returns (nil, nil)", func(t *testing.T) {
		// Defensive path: the domain.UserMetadataReader interface doesn't forbid
		// returning a nil profile with a nil error. Dereferencing profile.Name
		// would panic and take down the outbound ITX write, so we treat this
		// exactly like a resolution failure.
		reader := &fakeUserMetadataReader{} // profile=nil, err=nil
		a := auditStamper{userMetadata: reader}
		got := a.buildRequestingUser(ctxWithPrincipal("bob", "bob@example.com"))
		if assert.NotNil(t, got) {
			assert.Equal(t, "bob", got.Username)
			assert.Equal(t, "bob@example.com", got.Email)
			assert.Empty(t, got.Name)
			assert.Empty(t, got.ProfilePicture)
		}
	})
}

func TestAuditStamper_BuildRequestingCreatedUpdatedBy(t *testing.T) {
	t.Run("returns nil when no principal", func(t *testing.T) {
		a := auditStamper{}
		assert.Nil(t, a.buildRequestingCreatedUpdatedBy(context.Background()))
	})

	t.Run("mirrors full profile onto CreatedUpdatedBy shape", func(t *testing.T) {
		reader := &fakeUserMetadataReader{profile: &domain.UserProfile{
			Username:  "alice",
			Name:      "Alice Example",
			Email:     "alice@example.com",
			AvatarURL: "https://example.com/a.jpg", // intentionally dropped by CreatedUpdatedBy shape
		}}
		a := auditStamper{userMetadata: reader}
		got := a.buildRequestingCreatedUpdatedBy(ctxWithPrincipal("alice", ""))
		if assert.NotNil(t, got) {
			assert.Equal(t, "alice", got.Username)
			assert.Equal(t, "Alice Example", got.Name)
			assert.Equal(t, "alice@example.com", got.Email)
		}
	})
}
