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
	// submitResponseResp/submitResponseErr control SubmitMeetingResponse.
	submitResponseResp *itx.MeetingResponseResult
	submitResponseErr  error
	// submitResponseCalled records whether SubmitMeetingResponse was called.
	submitResponseCalled bool
}

func (f *fakeMeetingClient) SubmitMeetingResponse(_ context.Context, _ string, _ *itx.MeetingResponseRequest) (*itx.MeetingResponseResult, error) {
	f.submitResponseCalled = true
	if f.submitResponseErr != nil {
		return nil, f.submitResponseErr
	}
	if f.submitResponseResp != nil {
		return f.submitResponseResp, nil
	}
	return &itx.MeetingResponseResult{}, nil
}

// fakeGetRegistrantClient is a minimal read-side test double for domain.ITXRegistrantClient,
// used by SubmitMeetingResponse ownership-check tests. The write-side fakeRegistrantClient
// is defined in registrant_service_test.go.
type fakeGetRegistrantClient struct {
	domain.ITXRegistrantClient
	registrant *itx.ZoomMeetingRegistrant
	err        error
}

func (f *fakeGetRegistrantClient) GetRegistrant(_ context.Context, _, _ string) (*itx.ZoomMeetingRegistrant, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.registrant != nil {
		return f.registrant, nil
	}
	return &itx.ZoomMeetingRegistrant{}, nil
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

// fakeCommitteeAuthorizer is a test double for domain.CommitteeAuthorizer.
type fakeCommitteeAuthorizer struct {
	// allowedIDs maps committee ID → access granted. The special key "*" grants
	// access to every committee.
	allowedIDs map[string]bool
	// err, when non-nil, is returned for every call instead of a permission result.
	err error
	// calls records each (principal, committeeID) pair received, for assertions.
	calls [][2]string
}

func (f *fakeCommitteeAuthorizer) HasWriteAccess(_ context.Context, principal, committeeID string) (bool, error) {
	f.calls = append(f.calls, [2]string{principal, committeeID})
	if f.err != nil {
		return false, f.err
	}
	if f.allowedIDs["*"] {
		return true, nil
	}
	return f.allowedIDs[committeeID], nil
}

// allowAllCommittees returns an authorizer that grants access to every committee.
func allowAllCommittees() *fakeCommitteeAuthorizer {
	return &fakeCommitteeAuthorizer{allowedIDs: map[string]bool{"*": true}}
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
		svc := NewMeetingService(client, nil, noOpIDMapper{}, reader, nil)

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
		svc := NewMeetingService(client, nil, noOpIDMapper{}, reader, nil)

		_, err := svc.CreateMeeting(ctxWithPrincipal("alice", "alice@heimdall.example.com"), baseReq())
		require.NoError(t, err)
		assert.Equal(t, "alice@heimdall.example.com", client.lastCreateReq.CreatedBy.Email)
	})

	t.Run("degrades to username/email when resolver errors", func(t *testing.T) {
		client := &fakeMeetingClient{}
		reader := &fakeUserMetadataReader{err: errors.New("auth service unavailable")}
		svc := NewMeetingService(client, nil, noOpIDMapper{}, reader, nil)

		_, err := svc.CreateMeeting(ctxWithPrincipal("bob", "bob@heimdall.example.com"), baseReq())
		require.NoError(t, err, "resolver failures must never block meeting creation")
		require.NotNil(t, client.lastCreateReq.CreatedBy)
		assert.Equal(t, "bob", client.lastCreateReq.CreatedBy.Username)
		assert.Equal(t, "bob@heimdall.example.com", client.lastCreateReq.CreatedBy.Email)
		assert.Empty(t, client.lastCreateReq.CreatedBy.Name)
	})

	t.Run("degrades to username/email when reader is nil (NATS disabled)", func(t *testing.T) {
		client := &fakeMeetingClient{}
		svc := NewMeetingService(client, nil, noOpIDMapper{}, nil, nil)

		_, err := svc.CreateMeeting(ctxWithPrincipal("carol", "carol@heimdall.example.com"), baseReq())
		require.NoError(t, err)
		require.NotNil(t, client.lastCreateReq.CreatedBy)
		assert.Equal(t, "carol", client.lastCreateReq.CreatedBy.Username)
	})

	t.Run("omits created_by when there is no principal in context", func(t *testing.T) {
		client := &fakeMeetingClient{}
		reader := &fakeUserMetadataReader{profile: &domain.UserProfile{Username: "alice"}}
		svc := NewMeetingService(client, nil, noOpIDMapper{}, reader, nil)

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
		svc := NewMeetingService(client, nil, noOpIDMapper{}, reader, nil)

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
		svc := NewMeetingService(client, nil, noOpIDMapper{}, reader, nil)

		err := svc.UpdateMeeting(ctxWithPrincipal("alice", "alice@heimdall.example.com"), "meeting-1", baseReq())
		require.NoError(t, err)
		require.NotNil(t, client.lastUpdateReq.UpdatedBy)
		assert.Equal(t, "alice@heimdall.example.com", client.lastUpdateReq.UpdatedBy.Email)
	})

	t.Run("degrades to username/email when resolver errors", func(t *testing.T) {
		client := &fakeMeetingClient{getResp: currentMeeting}
		reader := &fakeUserMetadataReader{err: errors.New("auth service unavailable")}
		svc := NewMeetingService(client, nil, noOpIDMapper{}, reader, nil)

		err := svc.UpdateMeeting(ctxWithPrincipal("bob", "bob@heimdall.example.com"), "meeting-1", baseReq())
		require.NoError(t, err, "resolver failures must never block meeting updates")
		require.NotNil(t, client.lastUpdateReq.UpdatedBy)
		assert.Equal(t, "bob", client.lastUpdateReq.UpdatedBy.Username)
		assert.Equal(t, "bob@heimdall.example.com", client.lastUpdateReq.UpdatedBy.Email)
		assert.Empty(t, client.lastUpdateReq.UpdatedBy.Name)
	})

	t.Run("degrades to username/email when reader is nil (NATS disabled)", func(t *testing.T) {
		client := &fakeMeetingClient{getResp: currentMeeting}
		svc := NewMeetingService(client, nil, noOpIDMapper{}, nil, nil)

		err := svc.UpdateMeeting(ctxWithPrincipal("carol", "carol@heimdall.example.com"), "meeting-1", baseReq())
		require.NoError(t, err)
		require.NotNil(t, client.lastUpdateReq.UpdatedBy)
		assert.Equal(t, "carol", client.lastUpdateReq.UpdatedBy.Username)
	})

	t.Run("omits updated_by when there is no principal in context", func(t *testing.T) {
		client := &fakeMeetingClient{getResp: currentMeeting}
		reader := &fakeUserMetadataReader{profile: &domain.UserProfile{Username: "alice"}}
		svc := NewMeetingService(client, nil, noOpIDMapper{}, reader, nil)

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
		svc := NewMeetingService(client, nil, noOpIDMapper{}, nil, nil)

		_, err := svc.CreateMeeting(context.Background(), baseReq())
		require.NoError(t, err)
		require.NotNil(t, client.lastCreateReq)
		require.NotNil(t, client.lastCreateReq.AutoEmailReminderEnabled)
		assert.True(t, *client.lastCreateReq.AutoEmailReminderEnabled)
		assert.Equal(t, 1440, client.lastCreateReq.AutoEmailReminderTime)
	})

	t.Run("update forwards reminder fields to ITX", func(t *testing.T) {
		client := &fakeMeetingClient{getResp: &itx.ZoomMeetingResponse{Project: "proj-1"}}
		svc := NewMeetingService(client, nil, noOpIDMapper{}, nil, nil)

		err := svc.UpdateMeeting(context.Background(), "meeting-1", baseReq())
		require.NoError(t, err)
		require.NotNil(t, client.lastUpdateReq)
		require.NotNil(t, client.lastUpdateReq.AutoEmailReminderEnabled)
		assert.True(t, *client.lastUpdateReq.AutoEmailReminderEnabled)
		assert.Equal(t, 1440, client.lastUpdateReq.AutoEmailReminderTime)
	})

	t.Run("explicit false serializes on the wire so ITX resets the stored pair", func(t *testing.T) {
		client := &fakeMeetingClient{}
		svc := NewMeetingService(client, nil, noOpIDMapper{}, nil, nil)

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
		svc := NewMeetingService(client, nil, noOpIDMapper{}, nil, nil)

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
		svc := NewMeetingService(client, nil, noOpIDMapper{}, nil, nil)

		_, err := svc.CreateMeeting(context.Background(), baseReq())
		require.NoError(t, err)
		require.NotNil(t, client.lastCreateReq)
		require.NotNil(t, client.lastCreateReq.ShowMeetingAttendees)
		assert.True(t, *client.lastCreateReq.ShowMeetingAttendees)
	})

	t.Run("update forwards the flag to ITX", func(t *testing.T) {
		client := &fakeMeetingClient{getResp: &itx.ZoomMeetingResponse{Project: "proj-1"}}
		svc := NewMeetingService(client, nil, noOpIDMapper{}, nil, nil)

		err := svc.UpdateMeeting(context.Background(), "meeting-1", baseReq())
		require.NoError(t, err)
		require.NotNil(t, client.lastUpdateReq)
		require.NotNil(t, client.lastUpdateReq.ShowMeetingAttendees)
		assert.True(t, *client.lastUpdateReq.ShowMeetingAttendees)
	})

	t.Run("explicit false serializes on the wire", func(t *testing.T) {
		client := &fakeMeetingClient{}
		svc := NewMeetingService(client, nil, noOpIDMapper{}, nil, nil)

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
		svc := NewMeetingService(client, nil, noOpIDMapper{}, nil, nil)

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
		svc := NewMeetingService(client, nil, noOpIDMapper{}, nil, nil)

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
		svc := NewMeetingService(client, nil, noOpIDMapper{}, nil, nil)

		err := svc.UpdateMeeting(context.Background(), "meeting-1", baseReq())
		require.NoError(t, err)
		require.NotNil(t, client.lastUpdateReq)
		require.NotNil(t, client.lastUpdateReq.Owner)
		assert.Equal(t, "oowner", client.lastUpdateReq.Owner.Username)
	})

	t.Run("omitted owner stays off the wire so ITX preserves the stored owner", func(t *testing.T) {
		client := &fakeMeetingClient{getResp: &itx.ZoomMeetingResponse{Project: "proj-1"}}
		svc := NewMeetingService(client, nil, noOpIDMapper{}, nil, nil)

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
		svc := NewMeetingService(client, nil, noOpIDMapper{}, reader, nil)

		err := svc.UpdateOccurrence(ctxWithPrincipal("alice", ""), "meeting-1", "occ-1", &itx.UpdateOccurrenceRequest{Topic: "new topic"})
		require.NoError(t, err)
		require.NotNil(t, client.lastUpdateOccurrence)
		require.NotNil(t, client.lastUpdateOccurrence.UpdatedBy, "occurrence update must stamp updated_by so ITX doesn't preserve stale data")
		assert.Equal(t, "alice", client.lastUpdateOccurrence.UpdatedBy.Username)
		assert.Equal(t, "Alice Example", client.lastUpdateOccurrence.UpdatedBy.Name)
	})

	t.Run("omits updated_by when no principal in context", func(t *testing.T) {
		client := &fakeMeetingClient{}
		svc := NewMeetingService(client, nil, noOpIDMapper{}, nil, nil)

		err := svc.UpdateOccurrence(context.Background(), "meeting-1", "occ-1", &itx.UpdateOccurrenceRequest{})
		require.NoError(t, err)
		require.NotNil(t, client.lastUpdateOccurrence)
		assert.Nil(t, client.lastUpdateOccurrence.UpdatedBy)
	})
}

func TestMeetingService_UpdateMeeting_ProjectImmutability(t *testing.T) {
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

	t.Run("allows update when project is unchanged", func(t *testing.T) {
		client := &fakeMeetingClient{getResp: currentMeeting}
		svc := NewMeetingService(client, nil, noOpIDMapper{}, nil, nil)

		err := svc.UpdateMeeting(context.Background(), "meeting-1", baseReq())
		require.NoError(t, err)
		require.NotNil(t, client.lastUpdateReq)
	})

	t.Run("rejects update that changes project_uid", func(t *testing.T) {
		client := &fakeMeetingClient{getResp: currentMeeting}
		svc := NewMeetingService(client, nil, noOpIDMapper{}, nil, nil)

		req := baseReq()
		req.ProjectUID = "proj-other"
		err := svc.UpdateMeeting(context.Background(), "meeting-1", req)
		require.Error(t, err)
		assert.Equal(t, domain.ErrorTypeForbidden, domain.GetErrorType(err))
		assert.Nil(t, client.lastUpdateReq, "ITX must not be called when re-parenting is rejected")
	})

	// Committee changes are permitted when the caller has FGA write access on any new committee.

	t.Run("allows update that adds a committee when FGA grants access", func(t *testing.T) {
		client := &fakeMeetingClient{getResp: currentMeeting}
		authz := allowAllCommittees()
		svc := NewMeetingService(client, nil, noOpIDMapper{}, nil, authz)

		req := baseReq()
		req.Committees = append(req.Committees, models.Committee{UID: "00000000-0000-0000-0000-000000000002"})
		err := svc.UpdateMeeting(ctxWithPrincipal("alice", ""), "meeting-1", req)
		require.NoError(t, err)
		require.NotNil(t, client.lastUpdateReq)
		// Only the new committee should have been checked; the existing one is skipped.
		require.Len(t, authz.calls, 1)
		assert.Equal(t, "00000000-0000-0000-0000-000000000002", authz.calls[0][1])
	})

	t.Run("allows update that swaps a committee when FGA grants access", func(t *testing.T) {
		client := &fakeMeetingClient{getResp: currentMeeting}
		authz := allowAllCommittees()
		svc := NewMeetingService(client, nil, noOpIDMapper{}, nil, authz)

		req := baseReq()
		req.Committees = []models.Committee{{UID: "00000000-0000-0000-0000-000000000099"}}
		err := svc.UpdateMeeting(ctxWithPrincipal("alice", ""), "meeting-1", req)
		require.NoError(t, err)
		require.NotNil(t, client.lastUpdateReq)
		require.Len(t, authz.calls, 1, "swapped-in committee must trigger a FGA check")
		assert.Equal(t, "00000000-0000-0000-0000-000000000099", authz.calls[0][1])
	})

	t.Run("rejects update that swaps to a committee the principal cannot write", func(t *testing.T) {
		client := &fakeMeetingClient{getResp: currentMeeting}
		authz := &fakeCommitteeAuthorizer{allowedIDs: map[string]bool{}} // denies everything
		svc := NewMeetingService(client, nil, noOpIDMapper{}, nil, authz)

		req := baseReq()
		req.Committees = []models.Committee{{UID: "00000000-0000-0000-0000-000000000099"}}
		err := svc.UpdateMeeting(ctxWithPrincipal("alice", ""), "meeting-1", req)
		require.Error(t, err)
		assert.Equal(t, domain.ErrorTypeForbidden, domain.GetErrorType(err))
		assert.Nil(t, client.lastUpdateReq, "ITX must not be called when swapped committee access is denied")
	})

	t.Run("allows update that removes all committees (no FGA check needed)", func(t *testing.T) {
		client := &fakeMeetingClient{getResp: currentMeeting}
		authz := allowAllCommittees()
		svc := NewMeetingService(client, nil, noOpIDMapper{}, nil, authz)

		req := baseReq()
		req.Committees = nil
		err := svc.UpdateMeeting(ctxWithPrincipal("alice", ""), "meeting-1", req)
		require.NoError(t, err)
		require.NotNil(t, client.lastUpdateReq)
		assert.Empty(t, authz.calls, "removing committees requires no FGA check")
	})
}

func TestMeetingService_UpdateMeeting_CommitteeAuthorization(t *testing.T) {
	existing := &itx.ZoomMeetingResponse{
		Project: "proj-1",
		Committees: []itx.Committee{
			{ID: "00000000-0000-0000-0000-000000000001"},
		},
	}
	baseReq := func() *models.CreateITXMeetingRequest {
		return &models.CreateITXMeetingRequest{
			ID:         "meeting-1",
			ProjectUID: "proj-1",
			Title:      "Test Meeting",
			StartTime:  "2026-01-01T00:00:00Z",
			Duration:   30,
			Visibility: itx.MeetingVisibilityPublic,
			Committees: []models.Committee{
				{UID: "00000000-0000-0000-0000-000000000001"}, // already on the meeting
			},
		}
	}

	t.Run("rejects adding a committee the principal cannot write", func(t *testing.T) {
		client := &fakeMeetingClient{getResp: existing}
		authz := &fakeCommitteeAuthorizer{allowedIDs: map[string]bool{}} // denies everything
		svc := NewMeetingService(client, nil, noOpIDMapper{}, nil, authz)

		req := baseReq()
		req.Committees = append(req.Committees, models.Committee{UID: "00000000-0000-0000-0000-000000000002"})
		err := svc.UpdateMeeting(ctxWithPrincipal("alice", ""), "meeting-1", req)
		require.Error(t, err)
		assert.Equal(t, domain.ErrorTypeForbidden, domain.GetErrorType(err))
		assert.Nil(t, client.lastUpdateReq, "ITX must not be called when committee access is denied")
	})

	t.Run("skips FGA check for committees already on the meeting", func(t *testing.T) {
		client := &fakeMeetingClient{getResp: existing}
		authz := &fakeCommitteeAuthorizer{allowedIDs: map[string]bool{}} // denies everything
		svc := NewMeetingService(client, nil, noOpIDMapper{}, nil, authz)

		// Request contains only the existing committee — no delta, so no FGA call expected.
		err := svc.UpdateMeeting(ctxWithPrincipal("alice", ""), "meeting-1", baseReq())
		require.NoError(t, err)
		assert.Empty(t, authz.calls, "existing committees must not trigger a new FGA check")
	})

	t.Run("skips FGA check when no committees in request", func(t *testing.T) {
		client := &fakeMeetingClient{getResp: existing}
		authz := &fakeCommitteeAuthorizer{allowedIDs: map[string]bool{}}
		svc := NewMeetingService(client, nil, noOpIDMapper{}, nil, authz)

		req := baseReq()
		req.Committees = nil
		err := svc.UpdateMeeting(ctxWithPrincipal("alice", ""), "meeting-1", req)
		require.NoError(t, err)
		assert.Empty(t, authz.calls)
	})

	t.Run("rejects update when FGA is unavailable (fail closed)", func(t *testing.T) {
		client := &fakeMeetingClient{getResp: existing}
		authz := &fakeCommitteeAuthorizer{err: errors.New("nats: no servers available")}
		svc := NewMeetingService(client, nil, noOpIDMapper{}, nil, authz)

		req := baseReq()
		req.Committees = append(req.Committees, models.Committee{UID: "00000000-0000-0000-0000-000000000002"})
		// When the authorizer is configured but returns an error, the service must fail
		// closed — the ITX PUT must not be sent — and return Unavailable (503) so
		// callers know the denial is transient and can retry.
		err := svc.UpdateMeeting(ctxWithPrincipal("alice", ""), "meeting-1", req)
		require.Error(t, err)
		assert.Equal(t, domain.ErrorTypeUnavailable, domain.GetErrorType(err))
		assert.Nil(t, client.lastUpdateReq)
	})

	t.Run("skips FGA check when committeeAuthz is nil (NATS disabled)", func(t *testing.T) {
		client := &fakeMeetingClient{getResp: existing}
		svc := NewMeetingService(client, nil, noOpIDMapper{}, nil, nil)

		req := baseReq()
		req.Committees = append(req.Committees, models.Committee{UID: "00000000-0000-0000-0000-000000000002"})
		err := svc.UpdateMeeting(ctxWithPrincipal("alice", ""), "meeting-1", req)
		require.NoError(t, err)
	})

	t.Run("passes v2 UID (not mapped v1 SFID) to HasWriteAccess", func(t *testing.T) {
		// Use a mapper that returns a distinct v1 SFID for the new committee so
		// any regression that passes the SFID instead of the v2 UID is caught.
		const v2UID = "00000000-0000-0000-0000-000000000002"
		const v1SFID = "a0B000000SFID0001EAC"

		mapper := committeeV2ToV1Mapper{v2UID: v2UID, v1SFID: v1SFID}
		authz := allowAllCommittees()
		client := &fakeMeetingClient{getResp: existing}
		svc := NewMeetingService(client, nil, mapper, nil, authz)

		req := baseReq()
		req.Committees = append(req.Committees, models.Committee{UID: v2UID})
		err := svc.UpdateMeeting(ctxWithPrincipal("alice", ""), "meeting-1", req)
		require.NoError(t, err)
		require.Len(t, authz.calls, 1, "expected exactly one FGA call for the new committee")
		assert.Equal(t, v2UID, authz.calls[0][1], "HasWriteAccess must receive the v2 UID, not the mapped v1 SFID")
	})

	t.Run("existing committee skipped and new committee checked with v2 UID under non-identity project+committee mapper", func(t *testing.T) {
		// Verify that project-immutability and existing-committee comparisons both
		// happen in v1 SFID space when a non-identity mapper is active.
		const projV2 = "proj-v2-uid-1"
		const projV1 = "a0P000000PROJSFID1EAC"
		const existCommV2 = "00000000-0000-0000-0000-000000000001"
		const existCommV1 = "a0C000000EXISTSFID1EAC"
		const newCommV2 = "00000000-0000-0000-0000-000000000003"
		const newCommV1 = "a0C000000NEWSFID00003AC"

		liveRecord := &itx.ZoomMeetingResponse{
			Project:    projV1,
			Committees: []itx.Committee{{ID: existCommV1}},
		}
		mapper := fullIDMapper{
			projectV2: projV2, projectV1: projV1,
			committeeV2ToV1: map[string]string{
				existCommV2: existCommV1,
				newCommV2:   newCommV1,
			},
		}
		authz := allowAllCommittees()
		client := &fakeMeetingClient{getResp: liveRecord}
		svc := NewMeetingService(client, nil, mapper, nil, authz)

		req := baseReq()
		req.ProjectUID = projV2
		req.Committees = []models.Committee{{UID: existCommV2}, {UID: newCommV2}}
		err := svc.UpdateMeeting(ctxWithPrincipal("alice", ""), "meeting-1", req)
		require.NoError(t, err)
		require.Len(t, authz.calls, 1, "only the new committee should trigger an FGA check")
		assert.Equal(t, newCommV2, authz.calls[0][1], "FGA check must use v2 UID")
	})

	t.Run("rejects re-parenting when project differs after v2→v1 mapping", func(t *testing.T) {
		const projV2 = "proj-v2-uid-1"
		const projV1 = "a0P000000PROJSFID1EAC"
		const diffProjV1 = "a0P000000DIFFERENT1EAC"

		liveRecord := &itx.ZoomMeetingResponse{
			Project:    diffProjV1,
			Committees: []itx.Committee{{ID: "00000000-0000-0000-0000-000000000001"}},
		}
		mapper := fullIDMapper{projectV2: projV2, projectV1: projV1}
		client := &fakeMeetingClient{getResp: liveRecord}
		svc := NewMeetingService(client, nil, mapper, nil, allowAllCommittees())

		req := baseReq()
		req.ProjectUID = projV2
		err := svc.UpdateMeeting(ctxWithPrincipal("alice", ""), "meeting-1", req)
		require.Error(t, err)
		assert.Equal(t, domain.ErrorTypeForbidden, domain.GetErrorType(err))
		assert.Nil(t, client.lastUpdateReq)
	})
}

// committeeV2ToV1Mapper translates one specific v2 UID to a v1 SFID; all other
// IDs pass through unchanged. Used to verify that HasWriteAccess receives the
// original v2 UID regardless of ID mapping.
type committeeV2ToV1Mapper struct {
	noOpIDMapper
	v2UID  string
	v1SFID string
}

func (m committeeV2ToV1Mapper) MapCommitteeV2ToV1(_ context.Context, v2UID string) (string, error) {
	if v2UID == m.v2UID {
		return m.v1SFID, nil
	}
	return v2UID, nil
}

// fullIDMapper maps both project and committee UIDs for tests that require a
// non-identity mapper on multiple ID types simultaneously.
type fullIDMapper struct {
	noOpIDMapper
	projectV2       string
	projectV1       string
	committeeV2ToV1 map[string]string
}

func (m fullIDMapper) MapProjectV2ToV1(_ context.Context, v2UID string) (string, error) {
	if v2UID == m.projectV2 {
		return m.projectV1, nil
	}
	return v2UID, nil
}

func (m fullIDMapper) MapCommitteeV2ToV1(_ context.Context, v2UID string) (string, error) {
	if sfid, ok := m.committeeV2ToV1[v2UID]; ok {
		return sfid, nil
	}
	return v2UID, nil
}

func TestMeetingService_SubmitMeetingResponse(t *testing.T) {
	const meetingID = "meeting-abc"
	const compoundID = "meeting-abc-occ-1"
	const registrantID = "00000000-0000-0000-0000-000000000001"

	baseReq := func() *itx.MeetingResponseRequest {
		return &itx.MeetingResponseRequest{
			Response:     "accepted",
			Scope:        "all",
			RegistrantID: registrantID,
		}
	}

	t.Run("forwards response when registrant email matches principal", func(t *testing.T) {
		meetingClient := &fakeMeetingClient{}
		registrantClient := &fakeGetRegistrantClient{
			registrant: &itx.ZoomMeetingRegistrant{
				ID:       registrantID,
				Email:    "alice@example.com",
				Username: "other-user",
			},
		}
		svc := NewMeetingService(meetingClient, registrantClient, noOpIDMapper{}, nil, nil)

		_, err := svc.SubmitMeetingResponse(ctxWithPrincipal("alice", "alice@example.com"), meetingID, compoundID, baseReq())
		require.NoError(t, err)
		assert.True(t, meetingClient.submitResponseCalled, "ITX must be called when ownership is verified")
	})

	t.Run("forwards response when registrant username matches principal (email blank)", func(t *testing.T) {
		meetingClient := &fakeMeetingClient{}
		registrantClient := &fakeGetRegistrantClient{
			registrant: &itx.ZoomMeetingRegistrant{
				ID:       registrantID,
				Username: "alice",
				Email:    "other@example.com",
			},
		}
		svc := NewMeetingService(meetingClient, registrantClient, noOpIDMapper{}, nil, nil)

		// No email in ctx — match must fall back to username.
		_, err := svc.SubmitMeetingResponse(ctxWithPrincipal("alice", ""), meetingID, compoundID, baseReq())
		require.NoError(t, err)
		assert.True(t, meetingClient.submitResponseCalled)
	})

	t.Run("forwards response when email matches case-insensitively", func(t *testing.T) {
		meetingClient := &fakeMeetingClient{}
		registrantClient := &fakeGetRegistrantClient{
			registrant: &itx.ZoomMeetingRegistrant{
				ID:       registrantID,
				Email:    "Alice@Example.COM",
				Username: "other-user",
			},
		}
		svc := NewMeetingService(meetingClient, registrantClient, noOpIDMapper{}, nil, nil)

		_, err := svc.SubmitMeetingResponse(ctxWithPrincipal("alice", "alice@example.com"), meetingID, compoundID, baseReq())
		require.NoError(t, err)
		assert.True(t, meetingClient.submitResponseCalled, "case-differing email must still match")
	})

	t.Run("forwards response when JWT email absent but profile email matches registrant", func(t *testing.T) {
		meetingClient := &fakeMeetingClient{}
		registrantClient := &fakeGetRegistrantClient{
			registrant: &itx.ZoomMeetingRegistrant{
				ID:       registrantID,
				Email:    "alice@example.com",
				Username: "other-user",
			},
		}
		reader := &fakeUserMetadataReader{
			profile: &domain.UserProfile{Username: "alice", Email: "alice@example.com"},
		}
		svc := NewMeetingService(meetingClient, registrantClient, noOpIDMapper{}, reader, nil)

		// JWT email is empty — service must resolve the profile and use its email.
		_, err := svc.SubmitMeetingResponse(ctxWithPrincipal("alice", ""), meetingID, compoundID, baseReq())
		require.NoError(t, err)
		assert.True(t, meetingClient.submitResponseCalled, "profile email fallback must allow the registrant's owner")
	})

	t.Run("rejects when neither email nor username matches", func(t *testing.T) {
		meetingClient := &fakeMeetingClient{}
		registrantClient := &fakeGetRegistrantClient{
			registrant: &itx.ZoomMeetingRegistrant{
				ID:       registrantID,
				Email:    "other@example.com",
				Username: "other-user",
			},
		}
		svc := NewMeetingService(meetingClient, registrantClient, noOpIDMapper{}, nil, nil)

		_, err := svc.SubmitMeetingResponse(ctxWithPrincipal("alice", "alice@example.com"), meetingID, compoundID, baseReq())
		require.Error(t, err)
		assert.Equal(t, domain.ErrorTypeForbidden, domain.GetErrorType(err))
		assert.False(t, meetingClient.submitResponseCalled, "ITX must not be called when ownership check fails")
	})

	t.Run("rejects anonymous caller (no principal in context)", func(t *testing.T) {
		meetingClient := &fakeMeetingClient{}
		registrantClient := &fakeGetRegistrantClient{
			registrant: &itx.ZoomMeetingRegistrant{ID: registrantID, Email: "alice@example.com"},
		}
		svc := NewMeetingService(meetingClient, registrantClient, noOpIDMapper{}, nil, nil)

		_, err := svc.SubmitMeetingResponse(context.Background(), meetingID, compoundID, baseReq())
		require.Error(t, err)
		assert.Equal(t, domain.ErrorTypeForbidden, domain.GetErrorType(err))
		assert.False(t, meetingClient.submitResponseCalled)
	})

	t.Run("rejects M2M token (@clients suffix)", func(t *testing.T) {
		meetingClient := &fakeMeetingClient{}
		registrantClient := &fakeGetRegistrantClient{
			registrant: &itx.ZoomMeetingRegistrant{ID: registrantID, Email: "svc@example.com"},
		}
		svc := NewMeetingService(meetingClient, registrantClient, noOpIDMapper{}, nil, nil)

		_, err := svc.SubmitMeetingResponse(ctxWithPrincipal("clientid@clients", ""), meetingID, compoundID, baseReq())
		require.Error(t, err)
		assert.Equal(t, domain.ErrorTypeForbidden, domain.GetErrorType(err))
		assert.False(t, meetingClient.submitResponseCalled)
	})

	t.Run("propagates registrant lookup error and does not call ITX", func(t *testing.T) {
		meetingClient := &fakeMeetingClient{}
		registrantClient := &fakeGetRegistrantClient{
			err: errors.New("itx: upstream error"),
		}
		svc := NewMeetingService(meetingClient, registrantClient, noOpIDMapper{}, nil, nil)

		_, err := svc.SubmitMeetingResponse(ctxWithPrincipal("alice", "alice@example.com"), meetingID, compoundID, baseReq())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "itx: upstream error")
		assert.False(t, meetingClient.submitResponseCalled)
	})
}
