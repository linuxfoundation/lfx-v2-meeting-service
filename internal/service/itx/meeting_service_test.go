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
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/utils"
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
	// getResp/getErr control GetZoomMeeting; used by update tests to set the
	// current stored meeting for the re-parenting guard.
	getResp *itx.ZoomMeetingResponse
	getErr  error
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

func (f *fakeMeetingClient) GetZoomMeeting(_ context.Context, _ string) (*itx.ZoomMeetingResponse, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	if f.getResp != nil {
		return f.getResp, nil
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
	// currentMeeting matches baseReq so the re-parenting guard passes.
	currentMeeting := &itx.ZoomMeetingResponse{Project: "proj-1"}

	t.Run("stamps updated_by from resolved profile and never touches created_by", func(t *testing.T) {
		client := &fakeMeetingClient{getResp: currentMeeting}
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
		client := &fakeMeetingClient{getResp: currentMeeting}
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
		client := &fakeMeetingClient{getResp: currentMeeting}
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
		client := &fakeMeetingClient{getResp: currentMeeting}
		svc := NewMeetingService(client, noOpIDMapper{}, nil)

		err := svc.UpdateMeeting(ctxWithPrincipal("carol", "carol@heimdall.example.com"), "meeting-1", baseReq())
		require.NoError(t, err)
		require.NotNil(t, client.lastUpdateReq.UpdatedBy)
		assert.Equal(t, "carol", client.lastUpdateReq.UpdatedBy.Username)
	})

	t.Run("omits updated_by when there is no principal in context", func(t *testing.T) {
		client := &fakeMeetingClient{getResp: currentMeeting}
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
			AutoEmailReminderEnabled: utils.BoolPtr(true),
			AutoEmailReminderTime:    1440,
		}
	}

	t.Run("create forwards reminder fields to ITX", func(t *testing.T) {
		client := &fakeMeetingClient{}
		svc := NewMeetingService(client, noOpIDMapper{}, nil)

		_, err := svc.CreateMeeting(context.Background(), baseReq())
		require.NoError(t, err)
		require.NotNil(t, client.lastCreateReq)
		require.NotNil(t, client.lastCreateReq.AutoEmailReminderEnabled)
		assert.True(t, *client.lastCreateReq.AutoEmailReminderEnabled)
		assert.Equal(t, 1440, client.lastCreateReq.AutoEmailReminderTime)
	})

	t.Run("update forwards reminder fields to ITX", func(t *testing.T) {
		client := &fakeMeetingClient{getResp: &itx.ZoomMeetingResponse{Project: "proj-1"}}
		svc := NewMeetingService(client, noOpIDMapper{}, nil)

		err := svc.UpdateMeeting(context.Background(), "meeting-1", baseReq())
		require.NoError(t, err)
		require.NotNil(t, client.lastUpdateReq)
		require.NotNil(t, client.lastUpdateReq.AutoEmailReminderEnabled)
		assert.True(t, *client.lastUpdateReq.AutoEmailReminderEnabled)
		assert.Equal(t, 1440, client.lastUpdateReq.AutoEmailReminderTime)
	})

	t.Run("explicit false serializes on the wire so ITX resets the stored pair", func(t *testing.T) {
		client := &fakeMeetingClient{}
		svc := NewMeetingService(client, noOpIDMapper{}, nil)

		req := baseReq()
		req.AutoEmailReminderEnabled = utils.BoolPtr(false)
		req.AutoEmailReminderTime = 0
		_, err := svc.CreateMeeting(context.Background(), req)
		require.NoError(t, err)
		require.NotNil(t, client.lastCreateReq)

		// A non-nil false must survive omitempty so an explicit disable reaches ITX,
		// while the zero time is omitted.
		body, err := json.Marshal(client.lastCreateReq)
		require.NoError(t, err)
		assert.Contains(t, string(body), `"auto_email_reminder_enabled":false`)
		assert.NotContains(t, string(body), `"auto_email_reminder_time"`)
	})

	t.Run("omitted reminder field stays off the wire so ITX preserves the stored pair", func(t *testing.T) {
		client := &fakeMeetingClient{getResp: &itx.ZoomMeetingResponse{Project: "proj-1"}}
		svc := NewMeetingService(client, noOpIDMapper{}, nil)

		req := baseReq()
		req.AutoEmailReminderEnabled = nil
		req.AutoEmailReminderTime = 0
		err := svc.UpdateMeeting(context.Background(), "meeting-1", req)
		require.NoError(t, err)
		require.NotNil(t, client.lastUpdateReq)

		// An update from a client that never sends the reminder fields must not disable an
		// existing reminder: nil serializes as absent and ITX leaves the stored pair untouched.
		body, err := json.Marshal(client.lastUpdateReq)
		require.NoError(t, err)
		assert.NotContains(t, string(body), `"auto_email_reminder_enabled"`)
		assert.NotContains(t, string(body), `"auto_email_reminder_time"`)
	})
}

func TestMeetingService_ShowMeetingAttendeesForwardedToITX(t *testing.T) {
	baseReq := func() *models.CreateITXMeetingRequest {
		return &models.CreateITXMeetingRequest{
			ID:                   "meeting-1",
			ProjectUID:           "proj-1",
			Title:                "Test Meeting",
			StartTime:            "2026-01-01T00:00:00Z",
			Duration:             30,
			Visibility:           itx.MeetingVisibilityPublic,
			ShowMeetingAttendees: utils.BoolPtr(true),
		}
	}

	t.Run("create forwards the flag to ITX", func(t *testing.T) {
		client := &fakeMeetingClient{}
		svc := NewMeetingService(client, noOpIDMapper{}, nil)

		_, err := svc.CreateMeeting(context.Background(), baseReq())
		require.NoError(t, err)
		require.NotNil(t, client.lastCreateReq)
		require.NotNil(t, client.lastCreateReq.ShowMeetingAttendees)
		assert.True(t, *client.lastCreateReq.ShowMeetingAttendees)
	})

	t.Run("update forwards the flag to ITX", func(t *testing.T) {
		client := &fakeMeetingClient{getResp: &itx.ZoomMeetingResponse{Project: "proj-1"}}
		svc := NewMeetingService(client, noOpIDMapper{}, nil)

		err := svc.UpdateMeeting(context.Background(), "meeting-1", baseReq())
		require.NoError(t, err)
		require.NotNil(t, client.lastUpdateReq)
		require.NotNil(t, client.lastUpdateReq.ShowMeetingAttendees)
		assert.True(t, *client.lastUpdateReq.ShowMeetingAttendees)
	})

	t.Run("explicit false serializes on the wire", func(t *testing.T) {
		client := &fakeMeetingClient{}
		svc := NewMeetingService(client, noOpIDMapper{}, nil)

		req := baseReq()
		req.ShowMeetingAttendees = utils.BoolPtr(false)
		_, err := svc.CreateMeeting(context.Background(), req)
		require.NoError(t, err)
		require.NotNil(t, client.lastCreateReq)

		body, err := json.Marshal(client.lastCreateReq)
		require.NoError(t, err)
		assert.Contains(t, string(body), `"show_meeting_attendees":false`)
	})

	t.Run("omitted field stays off the wire so ITX preserves the stored value", func(t *testing.T) {
		client := &fakeMeetingClient{getResp: &itx.ZoomMeetingResponse{Project: "proj-1"}}
		svc := NewMeetingService(client, noOpIDMapper{}, nil)

		req := baseReq()
		req.ShowMeetingAttendees = nil
		err := svc.UpdateMeeting(context.Background(), "meeting-1", req)
		require.NoError(t, err)
		require.NotNil(t, client.lastUpdateReq)

		body, err := json.Marshal(client.lastUpdateReq)
		require.NoError(t, err)
		assert.NotContains(t, string(body), `"show_meeting_attendees"`)
	})
}

func TestMeetingService_OwnerForwardedToITX(t *testing.T) {
	baseReq := func() *models.CreateITXMeetingRequest {
		return &models.CreateITXMeetingRequest{
			ID:         "meeting-1",
			ProjectUID: "proj-1",
			Title:      "Test Meeting",
			StartTime:  "2026-01-01T00:00:00Z",
			Duration:   30,
			Visibility: itx.MeetingVisibilityPublic,
			Owner: &itx.User{
				Username: "oowner",
				Name:     "Olive Owner",
				Email:    "olive@example.com",
			},
		}
	}

	t.Run("create forwards owner and serializes it on the wire", func(t *testing.T) {
		client := &fakeMeetingClient{}
		svc := NewMeetingService(client, noOpIDMapper{}, nil)

		_, err := svc.CreateMeeting(context.Background(), baseReq())
		require.NoError(t, err)
		require.NotNil(t, client.lastCreateReq)
		require.NotNil(t, client.lastCreateReq.Owner)
		assert.Equal(t, "oowner", client.lastCreateReq.Owner.Username)
		assert.Equal(t, "olive@example.com", client.lastCreateReq.Owner.Email)

		body, err := json.Marshal(client.lastCreateReq)
		require.NoError(t, err)
		assert.Contains(t, string(body), `"owner":{`)
		assert.Contains(t, string(body), `"olive@example.com"`)
	})

	t.Run("update forwards owner to ITX", func(t *testing.T) {
		client := &fakeMeetingClient{getResp: &itx.ZoomMeetingResponse{Project: "proj-1"}}
		svc := NewMeetingService(client, noOpIDMapper{}, nil)

		err := svc.UpdateMeeting(context.Background(), "meeting-1", baseReq())
		require.NoError(t, err)
		require.NotNil(t, client.lastUpdateReq)
		require.NotNil(t, client.lastUpdateReq.Owner)
		assert.Equal(t, "oowner", client.lastUpdateReq.Owner.Username)
	})

	t.Run("omitted owner stays off the wire so ITX preserves the stored owner", func(t *testing.T) {
		client := &fakeMeetingClient{getResp: &itx.ZoomMeetingResponse{Project: "proj-1"}}
		svc := NewMeetingService(client, noOpIDMapper{}, nil)

		req := baseReq()
		req.Owner = nil
		err := svc.UpdateMeeting(context.Background(), "meeting-1", req)
		require.NoError(t, err)
		require.NotNil(t, client.lastUpdateReq)

		// An update from a client that never sends owner must not clear it: nil
		// serializes as absent and ITX leaves the stored owner untouched.
		body, err := json.Marshal(client.lastUpdateReq)
		require.NoError(t, err)
		assert.NotContains(t, string(body), `"owner"`)
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

func TestMeetingService_UpdateMeeting_NoReparenting(t *testing.T) {
	baseReq := func() *models.CreateITXMeetingRequest {
		return &models.CreateITXMeetingRequest{
			ID:         "meeting-1",
			ProjectUID: "proj-1",
			Title:      "Test Meeting",
			StartTime:  "2026-01-01T00:00:00Z",
			Duration:   30,
			Visibility: itx.MeetingVisibilityPublic,
			Committees: []models.Committee{
				{UID: "00000000-0000-0000-0000-000000000001"},
			},
		}
	}
	currentMeeting := &itx.ZoomMeetingResponse{
		Project: "proj-1",
		Committees: []itx.Committee{
			{ID: "00000000-0000-0000-0000-000000000001"},
		},
	}

	t.Run("allows update when project and committees are unchanged", func(t *testing.T) {
		client := &fakeMeetingClient{getResp: currentMeeting}
		svc := NewMeetingService(client, noOpIDMapper{}, nil)

		err := svc.UpdateMeeting(context.Background(), "meeting-1", baseReq())
		require.NoError(t, err)
		require.NotNil(t, client.lastUpdateReq)
	})

	t.Run("rejects update that changes project_uid", func(t *testing.T) {
		client := &fakeMeetingClient{getResp: currentMeeting}
		svc := NewMeetingService(client, noOpIDMapper{}, nil)

		req := baseReq()
		req.ProjectUID = "proj-other"
		err := svc.UpdateMeeting(context.Background(), "meeting-1", req)
		require.Error(t, err)
		assert.Equal(t, domain.ErrorTypeForbidden, domain.GetErrorType(err))
		assert.Nil(t, client.lastUpdateReq, "ITX must not be called when re-parenting is rejected")
	})

	t.Run("rejects update that adds a new committee", func(t *testing.T) {
		client := &fakeMeetingClient{getResp: currentMeeting}
		svc := NewMeetingService(client, noOpIDMapper{}, nil)

		req := baseReq()
		req.Committees = append(req.Committees, models.Committee{UID: "00000000-0000-0000-0000-000000000002"})
		err := svc.UpdateMeeting(context.Background(), "meeting-1", req)
		require.Error(t, err)
		assert.Equal(t, domain.ErrorTypeForbidden, domain.GetErrorType(err))
		assert.Nil(t, client.lastUpdateReq)
	})

	t.Run("rejects update that swaps a committee", func(t *testing.T) {
		client := &fakeMeetingClient{getResp: currentMeeting}
		svc := NewMeetingService(client, noOpIDMapper{}, nil)

		req := baseReq()
		req.Committees = []models.Committee{{UID: "00000000-0000-0000-0000-000000000099"}}
		err := svc.UpdateMeeting(context.Background(), "meeting-1", req)
		require.Error(t, err)
		assert.Equal(t, domain.ErrorTypeForbidden, domain.GetErrorType(err))
		assert.Nil(t, client.lastUpdateReq)
	})

	t.Run("rejects update that removes a committee", func(t *testing.T) {
		client := &fakeMeetingClient{getResp: currentMeeting}
		svc := NewMeetingService(client, noOpIDMapper{}, nil)

		req := baseReq()
		req.Committees = nil
		err := svc.UpdateMeeting(context.Background(), "meeting-1", req)
		require.Error(t, err)
		assert.Equal(t, domain.ErrorTypeForbidden, domain.GetErrorType(err))
		assert.Nil(t, client.lastUpdateReq)
	})

	t.Run("rejects update with duplicate committee IDs that would detach a current committee", func(t *testing.T) {
		stored := &itx.ZoomMeetingResponse{
			Project: "proj-1",
			Committees: []itx.Committee{
				{ID: "00000000-0000-0000-0000-000000000001"},
				{ID: "00000000-0000-0000-0000-000000000002"},
			},
		}
		client := &fakeMeetingClient{getResp: stored}
		svc := NewMeetingService(client, noOpIDMapper{}, nil)

		// [A, A] has length 2 matching stored [A, B], but would drop B.
		req := baseReq()
		req.Committees = []models.Committee{
			{UID: "00000000-0000-0000-0000-000000000001"},
			{UID: "00000000-0000-0000-0000-000000000001"},
		}
		err := svc.UpdateMeeting(context.Background(), "meeting-1", req)
		require.Error(t, err)
		assert.Equal(t, domain.ErrorTypeForbidden, domain.GetErrorType(err))
		assert.Nil(t, client.lastUpdateReq)
	})
}
