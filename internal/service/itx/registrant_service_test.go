// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package itx

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/models/itx"
)

// fakeRegistrantClient captures the ZoomMeetingRegistrant sent to ITX so tests can
// assert on the outbound created_by / updated_by stamping.
type fakeRegistrantClient struct {
	domain.ITXRegistrantClient
	lastCreateReq *itx.ZoomMeetingRegistrant
	lastUpdateReq *itx.ZoomMeetingRegistrant
}

func (f *fakeRegistrantClient) CreateRegistrant(_ context.Context, _ string, req *itx.ZoomMeetingRegistrant) (*itx.ZoomMeetingRegistrant, error) {
	f.lastCreateReq = req
	return &itx.ZoomMeetingRegistrant{}, nil
}

func (f *fakeRegistrantClient) UpdateRegistrant(_ context.Context, _, _ string, req *itx.ZoomMeetingRegistrant) error {
	f.lastUpdateReq = req
	return nil
}

// fakeRegistrantMeetingClient returns a canned meeting response for visibility checks
// in registrant service tests. Kept separate from meeting_service_test.go's fakeMeetingClient.
type fakeRegistrantMeetingClient struct {
	domain.ITXMeetingClient
	visibility itx.MeetingVisibility
	getErr     error
}

func (f *fakeRegistrantMeetingClient) GetZoomMeeting(_ context.Context, _ string) (*itx.ZoomMeetingResponse, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	return &itx.ZoomMeetingResponse{Visibility: f.visibility}, nil
}

func newSvcWithMeeting(registrant *fakeRegistrantClient, visibility itx.MeetingVisibility, reader domain.UserMetadataReader) *RegistrantService {
	return NewRegistrantService(registrant, &fakeRegistrantMeetingClient{visibility: visibility}, noOpIDMapper{}, reader)
}

func TestRegistrantService_CreateRegistrant_StampsCreatedBy(t *testing.T) {
	t.Run("stamps full profile on create", func(t *testing.T) {
		client := &fakeRegistrantClient{}
		reader := &fakeUserMetadataReader{profile: &domain.UserProfile{
			Username: "alice", Name: "Alice", Email: "alice@example.com",
		}}
		svc := NewRegistrantService(client, &fakeRegistrantMeetingClient{}, noOpIDMapper{}, reader)

		_, err := svc.CreateRegistrant(ctxWithPrincipal("alice", ""), "mtg-1", &itx.ZoomMeetingRegistrant{Email: "invitee@example.com"})
		require.NoError(t, err)
		require.NotNil(t, client.lastCreateReq.CreatedBy)
		assert.Equal(t, "alice", client.lastCreateReq.CreatedBy.Username)
		assert.Equal(t, "Alice", client.lastCreateReq.CreatedBy.Name)
		// Create never touches updated_by; leave the record's slot for actual updates.
		assert.Nil(t, client.lastCreateReq.UpdatedBy)
	})

	t.Run("omits created_by without principal", func(t *testing.T) {
		client := &fakeRegistrantClient{}
		svc := NewRegistrantService(client, &fakeRegistrantMeetingClient{}, noOpIDMapper{}, nil)

		_, err := svc.CreateRegistrant(context.Background(), "mtg-1", &itx.ZoomMeetingRegistrant{})
		require.NoError(t, err)
		assert.Nil(t, client.lastCreateReq.CreatedBy)
	})
}

func TestRegistrantService_SelfRegisterForMeeting(t *testing.T) {
	t.Run("sets email and username from context for public meeting", func(t *testing.T) {
		client := &fakeRegistrantClient{}
		reader := &fakeUserMetadataReader{profile: &domain.UserProfile{
			Username: "alice", Name: "Alice", Email: "alice@example.com",
		}}
		svc := newSvcWithMeeting(client, itx.MeetingVisibilityPublic, reader)

		ctx := ctxWithPrincipal("alice", "alice@example.com")
		_, err := svc.SelfRegisterForMeeting(ctx, "mtg-1", &itx.ZoomMeetingRegistrant{
			FirstName: "Alice",
			LastName:  "Liddell",
		})
		require.NoError(t, err)
		assert.Equal(t, "alice@example.com", client.lastCreateReq.Email)
		assert.Equal(t, "alice", client.lastCreateReq.Username)
		// Type is read-only on ITX create — must not be set on the outbound request.
		assert.Empty(t, client.lastCreateReq.Type)
		require.NotNil(t, client.lastCreateReq.CreatedBy)
		assert.Equal(t, "alice", client.lastCreateReq.CreatedBy.Username)
	})

	t.Run("propagates GetZoomMeeting error before any registrant call", func(t *testing.T) {
		client := &fakeRegistrantClient{}
		svc := newSvcWithMeeting(client, itx.MeetingVisibilityPublic, nil)
		// Swap to a client that returns an error on GetZoomMeeting.
		svc.meetingClient = &fakeRegistrantMeetingClient{getErr: domain.NewNotFoundError("meeting not found")}

		ctx := ctxWithPrincipal("alice", "alice@example.com")
		_, err := svc.SelfRegisterForMeeting(ctx, "mtg-1", &itx.ZoomMeetingRegistrant{})
		require.Error(t, err)
		var de *domain.DomainError
		require.ErrorAs(t, err, &de)
		assert.Equal(t, domain.ErrorTypeNotFound, de.Type)
		assert.Nil(t, client.lastCreateReq, "ITX client must not be called when meeting lookup fails")
	})

	t.Run("returns forbidden error for private meeting", func(t *testing.T) {
		client := &fakeRegistrantClient{}
		svc := newSvcWithMeeting(client, itx.MeetingVisibilityPrivate, nil)

		ctx := ctxWithPrincipal("alice", "alice@example.com")
		_, err := svc.SelfRegisterForMeeting(ctx, "mtg-1", &itx.ZoomMeetingRegistrant{
			FirstName: "Alice", LastName: "Liddell",
		})
		require.Error(t, err)
		var de *domain.DomainError
		require.ErrorAs(t, err, &de)
		assert.Equal(t, domain.ErrorTypeForbidden, de.Type)
		assert.Nil(t, client.lastCreateReq, "ITX registrant client must not be called for private meetings")
	})

	t.Run("returns validation error when email absent from JWT and profile", func(t *testing.T) {
		client := &fakeRegistrantClient{}
		// No userMetadata reader and no JWT email — both sources are empty.
		svc := newSvcWithMeeting(client, itx.MeetingVisibilityPublic, nil)

		ctx := ctxWithPrincipal("svc-account", "")
		_, err := svc.SelfRegisterForMeeting(ctx, "mtg-1", &itx.ZoomMeetingRegistrant{})
		require.Error(t, err)
		var de *domain.DomainError
		require.ErrorAs(t, err, &de)
		assert.Equal(t, domain.ErrorTypeValidation, de.Type)
		assert.Nil(t, client.lastCreateReq, "ITX client must not be called when email is missing")
	})

	t.Run("uses profile email when JWT email is absent", func(t *testing.T) {
		client := &fakeRegistrantClient{}
		reader := &fakeUserMetadataReader{profile: &domain.UserProfile{
			Username: "alice", Name: "Alice", Email: "alice@example.com",
		}}
		svc := newSvcWithMeeting(client, itx.MeetingVisibilityPublic, reader)

		// No JWT email — should fall back to profile.Email.
		ctx := ctxWithPrincipal("alice", "")
		_, err := svc.SelfRegisterForMeeting(ctx, "mtg-1", &itx.ZoomMeetingRegistrant{})
		require.NoError(t, err)
		assert.Equal(t, "alice@example.com", client.lastCreateReq.Email)
	})

	t.Run("returns validation error for M2M client token", func(t *testing.T) {
		client := &fakeRegistrantClient{}
		svc := newSvcWithMeeting(client, itx.MeetingVisibilityPublic, nil)

		// M2M principals carry the "@clients" suffix — self-registration requires a human identity.
		ctx := ctxWithPrincipal("6cjgEeimLcnqcHtqmRYqmOSt6s5spXNP@clients", "svc@example.com")
		_, err := svc.SelfRegisterForMeeting(ctx, "mtg-1", &itx.ZoomMeetingRegistrant{})
		require.Error(t, err)
		var de *domain.DomainError
		require.ErrorAs(t, err, &de)
		assert.Equal(t, domain.ErrorTypeValidation, de.Type)
		assert.Nil(t, client.lastCreateReq, "ITX client must not be called for M2M tokens")
	})

	t.Run("does not accept email from request body", func(t *testing.T) {
		client := &fakeRegistrantClient{}
		svc := newSvcWithMeeting(client, itx.MeetingVisibilityPublic, nil)

		ctx := ctxWithPrincipal("bob", "bob@example.com")
		_, err := svc.SelfRegisterForMeeting(ctx, "mtg-1", &itx.ZoomMeetingRegistrant{
			Email: "attacker@evil.com",
		})
		require.NoError(t, err)
		// The context email must win; the body email is overwritten.
		assert.Equal(t, "bob@example.com", client.lastCreateReq.Email)
	})

	t.Run("auth service profile overrides request payload fields", func(t *testing.T) {
		client := &fakeRegistrantClient{}
		reader := &fakeUserMetadataReader{profile: &domain.UserProfile{
			Username:     "alice",
			FirstName:    "Alice",
			LastName:     "Smith",
			JobTitle:     "Engineer",
			Organization: "Linux Foundation",
		}}
		svc := newSvcWithMeeting(client, itx.MeetingVisibilityPublic, reader)

		ctx := ctxWithPrincipal("alice", "alice@example.com")
		_, err := svc.SelfRegisterForMeeting(ctx, "mtg-1", &itx.ZoomMeetingRegistrant{
			FirstName: "Al",
			LastName:  "S",
			JobTitle:  "Dev",
			Org:       "ACME",
		})
		require.NoError(t, err)
		assert.Equal(t, "Alice", client.lastCreateReq.FirstName)
		assert.Equal(t, "Smith", client.lastCreateReq.LastName)
		assert.Equal(t, "Engineer", client.lastCreateReq.JobTitle)
		assert.Equal(t, "Linux Foundation", client.lastCreateReq.Org)
	})

	t.Run("request payload used as fallback when auth service fields are empty", func(t *testing.T) {
		client := &fakeRegistrantClient{}
		// Profile has name/email for audit stamp but no enrichment fields.
		reader := &fakeUserMetadataReader{profile: &domain.UserProfile{
			Username: "alice", Name: "Alice", Email: "alice@example.com",
		}}
		svc := newSvcWithMeeting(client, itx.MeetingVisibilityPublic, reader)

		ctx := ctxWithPrincipal("alice", "alice@example.com")
		_, err := svc.SelfRegisterForMeeting(ctx, "mtg-1", &itx.ZoomMeetingRegistrant{
			FirstName: "Alice",
			LastName:  "Liddell",
			JobTitle:  "Engineer",
			Org:       "ACME",
		})
		require.NoError(t, err)
		assert.Equal(t, "Alice", client.lastCreateReq.FirstName)
		assert.Equal(t, "Liddell", client.lastCreateReq.LastName)
		assert.Equal(t, "Engineer", client.lastCreateReq.JobTitle)
		assert.Equal(t, "ACME", client.lastCreateReq.Org)
	})

	t.Run("proceeds with request payload when auth service lookup fails", func(t *testing.T) {
		client := &fakeRegistrantClient{}
		reader := &fakeUserMetadataReader{err: fmt.Errorf("nats timeout")}
		svc := newSvcWithMeeting(client, itx.MeetingVisibilityPublic, reader)

		ctx := ctxWithPrincipal("alice", "alice@example.com")
		_, err := svc.SelfRegisterForMeeting(ctx, "mtg-1", &itx.ZoomMeetingRegistrant{
			FirstName: "Alice",
			LastName:  "Liddell",
		})
		require.NoError(t, err)
		assert.Equal(t, "Alice", client.lastCreateReq.FirstName)
		assert.Equal(t, "Liddell", client.lastCreateReq.LastName)
	})
}

// =============================================================================
// enrichRegistrantFromProfile — direct unit tests
// =============================================================================
// These tests drive the enricher through its own seam without any service,
// mock, or NATS dependency. Each test case covers one precedence rule.

func TestEnrichRegistrantFromProfile(t *testing.T) {
	t.Run("returns validation error when email is empty", func(t *testing.T) {
		req := &itx.ZoomMeetingRegistrant{}
		err := enrichRegistrantFromProfile(req, nil, "", "alice")
		require.Error(t, err)
		var de *domain.DomainError
		require.ErrorAs(t, err, &de)
		assert.Equal(t, domain.ErrorTypeValidation, de.Type)
	})

	t.Run("sets email and username from arguments; overwrites any request body value", func(t *testing.T) {
		req := &itx.ZoomMeetingRegistrant{Email: "attacker@example.com"}
		err := enrichRegistrantFromProfile(req, nil, "alice@example.com", "alice")
		require.NoError(t, err)
		assert.Equal(t, "alice@example.com", req.Email)
		assert.Equal(t, "alice", req.Username)
	})

	t.Run("profile wins over request when profile field is non-empty", func(t *testing.T) {
		req := &itx.ZoomMeetingRegistrant{
			FirstName: "Al",
			LastName:  "S",
			JobTitle:  "Dev",
			Org:       "Test Org",
		}
		profile := &domain.UserProfile{
			FirstName:    "Alice",
			LastName:     "Example",
			JobTitle:     "Engineer",
			Organization: "Example Foundation",
		}
		err := enrichRegistrantFromProfile(req, profile, "alice@example.com", "alice")
		require.NoError(t, err)
		assert.Equal(t, "Alice", req.FirstName)
		assert.Equal(t, "Example", req.LastName)
		assert.Equal(t, "Engineer", req.JobTitle)
		assert.Equal(t, "Example Foundation", req.Org)
	})

	t.Run("request payload used as fallback when profile fields are empty", func(t *testing.T) {
		req := &itx.ZoomMeetingRegistrant{
			FirstName: "Alice",
			LastName:  "Example",
			JobTitle:  "Engineer",
			Org:       "Test Org",
		}
		profile := &domain.UserProfile{} // all enrichment fields empty
		err := enrichRegistrantFromProfile(req, profile, "alice@example.com", "alice")
		require.NoError(t, err)
		assert.Equal(t, "Alice", req.FirstName)
		assert.Equal(t, "Example", req.LastName)
		assert.Equal(t, "Engineer", req.JobTitle)
		assert.Equal(t, "Test Org", req.Org)
	})

	t.Run("nil profile leaves request payload fields intact", func(t *testing.T) {
		req := &itx.ZoomMeetingRegistrant{
			FirstName: "Alice",
			LastName:  "Example",
		}
		err := enrichRegistrantFromProfile(req, nil, "alice@example.com", "alice")
		require.NoError(t, err)
		assert.Equal(t, "Alice", req.FirstName)
		assert.Equal(t, "Example", req.LastName)
	})

	t.Run("profile only partially overrides — empty profile fields leave request values", func(t *testing.T) {
		req := &itx.ZoomMeetingRegistrant{
			FirstName: "Al",
			LastName:  "S",
			JobTitle:  "Dev",      // profile has no JobTitle → stays
			Org:       "Test Org", // profile has no Organization → stays
		}
		profile := &domain.UserProfile{
			FirstName: "Alice",   // overrides
			LastName:  "Example", // overrides
			// JobTitle and Organization intentionally empty
		}
		err := enrichRegistrantFromProfile(req, profile, "alice@example.com", "alice")
		require.NoError(t, err)
		assert.Equal(t, "Alice", req.FirstName, "profile first name overrides request")
		assert.Equal(t, "Example", req.LastName, "profile last name overrides request")
		assert.Equal(t, "Dev", req.JobTitle, "empty profile job title leaves request value")
		assert.Equal(t, "Test Org", req.Org, "empty profile org leaves request value")
	})
}

func TestRegistrantService_UpdateRegistrant_StampsUpdatedByNotCreatedBy(t *testing.T) {
	t.Run("stamps only updated_by on update", func(t *testing.T) {
		client := &fakeRegistrantClient{}
		reader := &fakeUserMetadataReader{profile: &domain.UserProfile{Username: "bob", Email: "bob@example.com"}}
		svc := NewRegistrantService(client, &fakeRegistrantMeetingClient{}, noOpIDMapper{}, reader)

		err := svc.UpdateRegistrant(ctxWithPrincipal("bob", ""), "mtg-1", "reg-1", &itx.ZoomMeetingRegistrant{})
		require.NoError(t, err)
		require.NotNil(t, client.lastUpdateReq.UpdatedBy)
		assert.Equal(t, "bob", client.lastUpdateReq.UpdatedBy.Username)
		// Never overwrite the original creator on update.
		assert.Nil(t, client.lastUpdateReq.CreatedBy)
	})
}
