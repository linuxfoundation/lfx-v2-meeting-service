// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package itx

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/constants"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/models/itx"
)

// fakeMeetingClient captures the CreateZoomMeetingRequest/UpdateZoomMeetingRequest it
// receives so tests can assert on the outbound created_by field.
type fakeMeetingClient struct {
	domain.ITXMeetingClient
	lastCreateReq *itx.CreateZoomMeetingRequest
	lastUpdateReq *itx.CreateZoomMeetingRequest
	createResp    *itx.ZoomMeetingResponse
	createErr     error
}

func (f *fakeMeetingClient) CreateZoomMeeting(_ context.Context, req *itx.CreateZoomMeetingRequest) (*itx.ZoomMeetingResponse, error) {
	f.lastCreateReq = req
	if f.createErr != nil {
		return nil, f.createErr
	}
	if f.createResp != nil {
		return f.createResp, nil
	}
	return &itx.ZoomMeetingResponse{}, nil
}

func (f *fakeMeetingClient) UpdateZoomMeeting(_ context.Context, _ string, req *itx.CreateZoomMeetingRequest) error {
	f.lastUpdateReq = req
	return nil
}

// noOpIDMapper passes IDs through unchanged.
type noOpIDMapper struct{ domain.IDMapper }

func (noOpIDMapper) MapProjectV2ToV1(_ context.Context, v2UID string) (string, error) {
	return v2UID, nil
}
func (noOpIDMapper) MapProjectV1ToV2(_ context.Context, v1SFID string) (string, error) {
	return v1SFID, nil
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

func ctxWithPrincipal(principal, email string) context.Context {
	ctx := context.WithValue(context.Background(), constants.PrincipalContextID, principal)
	if email != "" {
		ctx = context.WithValue(ctx, constants.EmailContextID, email)
	}
	return ctx
}

func TestMeetingService_CreateMeeting_CreatedBy(t *testing.T) {
	baseReq := func() *models.CreateITXMeetingRequest {
		return &models.CreateITXMeetingRequest{
			ProjectUID: "proj-1",
			Title:      "Test Meeting",
			StartTime:  "2026-01-01T00:00:00Z",
			Duration:   30,
			Visibility: itx.MeetingVisibilityPublic,
		}
	}

	t.Run("resolves full profile via user metadata reader", func(t *testing.T) {
		client := &fakeMeetingClient{}
		reader := &fakeUserMetadataReader{
			profile: &domain.UserProfile{Username: "alice", Name: "Alice Example", AvatarURL: "https://example.com/a.jpg", Email: "alice@example.com"},
		}
		svc := NewMeetingService(client, noOpIDMapper{}, reader)

		_, err := svc.CreateMeeting(ctxWithPrincipal("alice", "alice@heimdall.example.com"), baseReq())
		require.NoError(t, err)
		require.NotNil(t, client.lastCreateReq.CreatedBy)

		got := client.lastCreateReq.CreatedBy
		assert.Equal(t, "alice", got.Username)
		assert.Equal(t, "Alice Example", got.Name)
		assert.Equal(t, "https://example.com/a.jpg", got.ProfilePicture)
		// The resolved profile email (fresh from the auth service) takes precedence over the
		// JWT-claimed email, which may be stale on a long-lived token.
		assert.Equal(t, "alice@example.com", got.Email)
		assert.Equal(t, []string{"alice"}, reader.calls)
	})

	t.Run("falls back to JWT email when profile has none", func(t *testing.T) {
		client := &fakeMeetingClient{}
		reader := &fakeUserMetadataReader{
			profile: &domain.UserProfile{Username: "alice", Name: "Alice Example"},
		}
		svc := NewMeetingService(client, noOpIDMapper{}, reader)

		_, err := svc.CreateMeeting(ctxWithPrincipal("alice", "alice@heimdall.example.com"), baseReq())
		require.NoError(t, err)
		assert.Equal(t, "alice@heimdall.example.com", client.lastCreateReq.CreatedBy.Email)
	})

	t.Run("degrades to username/email when resolver errors", func(t *testing.T) {
		client := &fakeMeetingClient{}
		reader := &fakeUserMetadataReader{err: errors.New("auth service unavailable")}
		svc := NewMeetingService(client, noOpIDMapper{}, reader)

		_, err := svc.CreateMeeting(ctxWithPrincipal("bob", "bob@heimdall.example.com"), baseReq())
		require.NoError(t, err, "resolver failures must never block meeting creation")
		require.NotNil(t, client.lastCreateReq.CreatedBy)
		assert.Equal(t, "bob", client.lastCreateReq.CreatedBy.Username)
		assert.Equal(t, "bob@heimdall.example.com", client.lastCreateReq.CreatedBy.Email)
		assert.Empty(t, client.lastCreateReq.CreatedBy.Name)
	})

	t.Run("degrades to username/email when reader is nil (NATS disabled)", func(t *testing.T) {
		client := &fakeMeetingClient{}
		svc := NewMeetingService(client, noOpIDMapper{}, nil)

		_, err := svc.CreateMeeting(ctxWithPrincipal("carol", "carol@heimdall.example.com"), baseReq())
		require.NoError(t, err)
		require.NotNil(t, client.lastCreateReq.CreatedBy)
		assert.Equal(t, "carol", client.lastCreateReq.CreatedBy.Username)
	})

	t.Run("omits created_by when there is no principal in context", func(t *testing.T) {
		client := &fakeMeetingClient{}
		reader := &fakeUserMetadataReader{profile: &domain.UserProfile{Username: "alice"}}
		svc := NewMeetingService(client, noOpIDMapper{}, reader)

		_, err := svc.CreateMeeting(context.Background(), baseReq())
		require.NoError(t, err)
		assert.Nil(t, client.lastCreateReq.CreatedBy)
		assert.Empty(t, reader.calls, "resolver should not be called without a principal")
	})
}

func TestMeetingService_UpdateMeeting_NeverSetsCreatedBy(t *testing.T) {
	client := &fakeMeetingClient{}
	reader := &fakeUserMetadataReader{
		profile: &domain.UserProfile{Username: "alice", Name: "Alice Example", Email: "alice@example.com"},
	}
	svc := NewMeetingService(client, noOpIDMapper{}, reader)

	req := &models.CreateITXMeetingRequest{
		ID:         "meeting-1",
		ProjectUID: "proj-1",
		Title:      "Test Meeting",
		StartTime:  "2026-01-01T00:00:00Z",
		Duration:   30,
		Visibility: itx.MeetingVisibilityPublic,
	}

	err := svc.UpdateMeeting(ctxWithPrincipal("alice", "alice@example.com"), "meeting-1", req)
	require.NoError(t, err)
	require.NotNil(t, client.lastUpdateReq)
	assert.Nil(t, client.lastUpdateReq.CreatedBy, "update must never stamp created_by, to avoid overwriting the original creator")
	assert.Empty(t, reader.calls, "resolver should not be invoked on update")
}
