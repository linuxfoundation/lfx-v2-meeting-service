// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package itx

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/models/itx"
)

// fakeMeetingClient captures the CreateZoomMeetingRequest / UpdateZoomMeetingRequest /
// UpdateOccurrenceRequest it receives so tests can assert on the outbound audit fields.
type fakeMeetingClient struct {
	domain.ITXMeetingClient
	lastCreateReq          *itx.CreateZoomMeetingRequest
	lastUpdateReq          *itx.CreateZoomMeetingRequest
	lastUpdateOccurrenceID string
	lastUpdateOccurrence   *itx.UpdateOccurrenceRequest
	createResp             *itx.ZoomMeetingResponse
	createErr              error
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

func (f *fakeMeetingClient) UpdateOccurrence(_ context.Context, _, occurrenceID string, req *itx.UpdateOccurrenceRequest) error {
	f.lastUpdateOccurrenceID = occurrenceID
	f.lastUpdateOccurrence = req
	return nil
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

func TestMeetingService_UpdateMeeting_StampsUpdatedByNotCreatedBy(t *testing.T) {
	baseReq := func() *models.CreateITXMeetingRequest {
		return &models.CreateITXMeetingRequest{
			ID:         "meeting-1",
			ProjectUID: "proj-1",
			Title:      "Test Meeting",
			StartTime:  "2026-01-01T00:00:00Z",
			Duration:   30,
			Visibility: itx.MeetingVisibilityPublic,
		}
	}

	t.Run("stamps updated_by from resolved profile and never touches created_by", func(t *testing.T) {
		client := &fakeMeetingClient{}
		reader := &fakeUserMetadataReader{
			profile: &domain.UserProfile{Username: "alice", Name: "Alice Example", AvatarURL: "https://example.com/a.jpg", Email: "alice@example.com"},
		}
		svc := NewMeetingService(client, noOpIDMapper{}, reader)

		err := svc.UpdateMeeting(ctxWithPrincipal("alice", "alice@heimdall.example.com"), "meeting-1", baseReq())
		require.NoError(t, err)
		require.NotNil(t, client.lastUpdateReq)
		assert.Nil(t, client.lastUpdateReq.CreatedBy, "update must never stamp created_by, to avoid overwriting the original creator")

		require.NotNil(t, client.lastUpdateReq.UpdatedBy, "update must stamp updated_by so ITX overwrites the stored value instead of preserving stale data")
		got := client.lastUpdateReq.UpdatedBy
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

		err := svc.UpdateMeeting(ctxWithPrincipal("alice", "alice@heimdall.example.com"), "meeting-1", baseReq())
		require.NoError(t, err)
		require.NotNil(t, client.lastUpdateReq.UpdatedBy)
		assert.Equal(t, "alice@heimdall.example.com", client.lastUpdateReq.UpdatedBy.Email)
	})

	t.Run("degrades to username/email when resolver errors", func(t *testing.T) {
		client := &fakeMeetingClient{}
		reader := &fakeUserMetadataReader{err: errors.New("auth service unavailable")}
		svc := NewMeetingService(client, noOpIDMapper{}, reader)

		err := svc.UpdateMeeting(ctxWithPrincipal("bob", "bob@heimdall.example.com"), "meeting-1", baseReq())
		require.NoError(t, err, "resolver failures must never block meeting updates")
		require.NotNil(t, client.lastUpdateReq.UpdatedBy)
		assert.Equal(t, "bob", client.lastUpdateReq.UpdatedBy.Username)
		assert.Equal(t, "bob@heimdall.example.com", client.lastUpdateReq.UpdatedBy.Email)
		assert.Empty(t, client.lastUpdateReq.UpdatedBy.Name)
	})

	t.Run("degrades to username/email when reader is nil (NATS disabled)", func(t *testing.T) {
		client := &fakeMeetingClient{}
		svc := NewMeetingService(client, noOpIDMapper{}, nil)

		err := svc.UpdateMeeting(ctxWithPrincipal("carol", "carol@heimdall.example.com"), "meeting-1", baseReq())
		require.NoError(t, err)
		require.NotNil(t, client.lastUpdateReq.UpdatedBy)
		assert.Equal(t, "carol", client.lastUpdateReq.UpdatedBy.Username)
	})

	t.Run("omits updated_by when there is no principal in context", func(t *testing.T) {
		client := &fakeMeetingClient{}
		reader := &fakeUserMetadataReader{profile: &domain.UserProfile{Username: "alice"}}
		svc := NewMeetingService(client, noOpIDMapper{}, reader)

		err := svc.UpdateMeeting(context.Background(), "meeting-1", baseReq())
		require.NoError(t, err)
		assert.Nil(t, client.lastUpdateReq.UpdatedBy)
		assert.Nil(t, client.lastUpdateReq.CreatedBy)
		assert.Empty(t, reader.calls, "resolver should not be called without a principal")
	})
}

func TestMeetingService_AutoEmailReminderFieldsForwardedToITX(t *testing.T) {
	baseReq := func() *models.CreateITXMeetingRequest {
		return &models.CreateITXMeetingRequest{
			ID:                       "meeting-1",
			ProjectUID:               "proj-1",
			Title:                    "Test Meeting",
			StartTime:                "2026-01-01T00:00:00Z",
			Duration:                 30,
			Visibility:               itx.MeetingVisibilityPublic,
			AutoEmailReminderEnabled: true,
			AutoEmailReminderTime:    1440,
		}
	}

	t.Run("create forwards reminder fields to ITX", func(t *testing.T) {
		client := &fakeMeetingClient{}
		svc := NewMeetingService(client, noOpIDMapper{}, nil)

		_, err := svc.CreateMeeting(context.Background(), baseReq())
		require.NoError(t, err)
		require.NotNil(t, client.lastCreateReq)
		assert.True(t, client.lastCreateReq.AutoEmailReminderEnabled)
		assert.Equal(t, 1440, client.lastCreateReq.AutoEmailReminderTime)
	})

	t.Run("update forwards reminder fields to ITX", func(t *testing.T) {
		client := &fakeMeetingClient{}
		svc := NewMeetingService(client, noOpIDMapper{}, nil)

		err := svc.UpdateMeeting(context.Background(), "meeting-1", baseReq())
		require.NoError(t, err)
		require.NotNil(t, client.lastUpdateReq)
		assert.True(t, client.lastUpdateReq.AutoEmailReminderEnabled)
		assert.Equal(t, 1440, client.lastUpdateReq.AutoEmailReminderTime)
	})

	t.Run("disabled reminder serializes an explicit false so ITX resets the stored pair", func(t *testing.T) {
		client := &fakeMeetingClient{}
		svc := NewMeetingService(client, noOpIDMapper{}, nil)

		req := baseReq()
		req.AutoEmailReminderEnabled = false
		req.AutoEmailReminderTime = 0
		_, err := svc.CreateMeeting(context.Background(), req)
		require.NoError(t, err)
		require.NotNil(t, client.lastCreateReq)
		assert.False(t, client.lastCreateReq.AutoEmailReminderEnabled)
		assert.Equal(t, 0, client.lastCreateReq.AutoEmailReminderTime)

		// ITX preserves the stored reminder pair when the enabled field is absent, so the wire
		// format must carry an explicit false (no omitempty) while the zero time is omitted.
		body, err := json.Marshal(client.lastCreateReq)
		require.NoError(t, err)
		assert.Contains(t, string(body), `"auto_email_reminder_enabled":false`)
		assert.NotContains(t, string(body), `"auto_email_reminder_time"`)
	})
}

func TestMeetingService_UpdateOccurrence_StampsUpdatedBy(t *testing.T) {
	t.Run("stamps updated_by from resolved profile", func(t *testing.T) {
		client := &fakeMeetingClient{}
		reader := &fakeUserMetadataReader{
			profile: &domain.UserProfile{Username: "alice", Name: "Alice Example", Email: "alice@example.com"},
		}
		svc := NewMeetingService(client, noOpIDMapper{}, reader)

		err := svc.UpdateOccurrence(ctxWithPrincipal("alice", ""), "meeting-1", "occ-1", &itx.UpdateOccurrenceRequest{Topic: "new topic"})
		require.NoError(t, err)
		require.NotNil(t, client.lastUpdateOccurrence)
		require.NotNil(t, client.lastUpdateOccurrence.UpdatedBy, "occurrence update must stamp updated_by so ITX doesn't preserve stale data")
		assert.Equal(t, "alice", client.lastUpdateOccurrence.UpdatedBy.Username)
		assert.Equal(t, "Alice Example", client.lastUpdateOccurrence.UpdatedBy.Name)
	})

	t.Run("omits updated_by when no principal in context", func(t *testing.T) {
		client := &fakeMeetingClient{}
		svc := NewMeetingService(client, noOpIDMapper{}, nil)

		err := svc.UpdateOccurrence(context.Background(), "meeting-1", "occ-1", &itx.UpdateOccurrenceRequest{})
		require.NoError(t, err)
		require.NotNil(t, client.lastUpdateOccurrence)
		assert.Nil(t, client.lastUpdateOccurrence.UpdatedBy)
	})
}
