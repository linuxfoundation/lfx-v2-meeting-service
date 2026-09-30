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
	pkgitx "github.com/linuxfoundation/lfx-v2-meeting-service/pkg/models/itx"
)

// fakePastMeetingClient captures the past-meeting requests sent to ITX so tests can
// assert on the outbound created_by / updated_by stamping.
type fakePastMeetingClient struct {
	domain.ITXPastMeetingClient
	lastCreateReq *pkgitx.CreatePastMeetingRequest
	lastUpdateReq *pkgitx.CreatePastMeetingRequest
}

func (f *fakePastMeetingClient) CreatePastMeeting(_ context.Context, req *pkgitx.CreatePastMeetingRequest) (*pkgitx.PastMeetingResponse, error) {
	f.lastCreateReq = req
	return &pkgitx.PastMeetingResponse{}, nil
}

func (f *fakePastMeetingClient) UpdatePastMeeting(_ context.Context, _ string, req *pkgitx.CreatePastMeetingRequest) (*pkgitx.PastMeetingResponse, error) {
	f.lastUpdateReq = req
	return &pkgitx.PastMeetingResponse{}, nil
}

// fakePastMeetingMeetingClient returns a canned meeting response for ownership checks
// in past-meeting service tests. project is the v1 project ID the fake meeting reports.
type fakePastMeetingMeetingClient struct {
	domain.ITXMeetingClient
	project string
	getErr  error
}

func (f *fakePastMeetingMeetingClient) GetZoomMeeting(_ context.Context, _ string) (*pkgitx.ZoomMeetingResponse, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	return &pkgitx.ZoomMeetingResponse{Project: f.project}, nil
}

// newPastMeetingSvc builds a PastMeetingService whose fake meeting client reports the
// given project as the owner of every meeting. Uses noOpIDMapper so v2/v1 IDs are equal.
func newPastMeetingSvc(pastClient *fakePastMeetingClient, meetingProject string, reader domain.UserMetadataReader) *PastMeetingService {
	return NewPastMeetingService(pastClient, &fakePastMeetingMeetingClient{project: meetingProject}, noOpIDMapper{}, reader)
}

func TestPastMeetingService_CreatePastMeeting_StampsCreatedBy(t *testing.T) {
	t.Run("stamps full profile on create; leaves updated_by nil", func(t *testing.T) {
		client := &fakePastMeetingClient{}
		reader := &fakeUserMetadataReader{profile: &domain.UserProfile{
			Username: "alice", Name: "Alice", Email: "alice@example.com",
		}}
		svc := newPastMeetingSvc(client, "proj-1", reader)

		_, err := svc.CreatePastMeeting(ctxWithPrincipal("alice", ""), &pkgitx.CreatePastMeetingRequest{
			MeetingID:    "mtg-1",
			OccurrenceID: "1234567890",
			ProjectID:    "proj-1",
		})
		require.NoError(t, err)
		require.NotNil(t, client.lastCreateReq.CreatedBy)
		assert.Equal(t, "alice", client.lastCreateReq.CreatedBy.Username)
		assert.Equal(t, "Alice", client.lastCreateReq.CreatedBy.Name)
		assert.Nil(t, client.lastCreateReq.UpdatedBy)
	})

	t.Run("omits stamp without principal", func(t *testing.T) {
		client := &fakePastMeetingClient{}
		svc := newPastMeetingSvc(client, "proj-1", nil)
		_, err := svc.CreatePastMeeting(context.Background(), &pkgitx.CreatePastMeetingRequest{
			MeetingID:    "mtg-1",
			OccurrenceID: "1234567890",
			ProjectID:    "proj-1",
		})
		require.NoError(t, err)
		assert.Nil(t, client.lastCreateReq.CreatedBy)
	})
}

func TestPastMeetingService_CreatePastMeeting_OwnershipCheck(t *testing.T) {
	t.Run("allows create when meeting belongs to authorized project", func(t *testing.T) {
		client := &fakePastMeetingClient{}
		svc := newPastMeetingSvc(client, "proj-1", nil)

		_, err := svc.CreatePastMeeting(context.Background(), &pkgitx.CreatePastMeetingRequest{
			MeetingID:    "mtg-1",
			OccurrenceID: "1234567890",
			ProjectID:    "proj-1",
		})
		require.NoError(t, err)
		require.NotNil(t, client.lastCreateReq, "ITX create must be called on an owned meeting")
	})

	t.Run("rejects create when meeting belongs to a different project", func(t *testing.T) {
		client := &fakePastMeetingClient{}
		// meeting client reports project B owns the meeting; request claims project A
		svc := newPastMeetingSvc(client, "proj-victim", nil)

		_, err := svc.CreatePastMeeting(context.Background(), &pkgitx.CreatePastMeetingRequest{
			MeetingID:    "foreign-mtg",
			OccurrenceID: "1234567890",
			ProjectID:    "proj-attacker",
		})
		require.Error(t, err)
		assert.Equal(t, domain.ErrorTypeForbidden, domain.GetErrorType(err))
		assert.Nil(t, client.lastCreateReq, "ITX create must not be called for a foreign meeting")
	})

	t.Run("propagates meeting lookup error", func(t *testing.T) {
		client := &fakePastMeetingClient{}
		lookupErr := errors.New("ITX unavailable")
		svc := NewPastMeetingService(
			client,
			&fakePastMeetingMeetingClient{getErr: lookupErr},
			noOpIDMapper{},
			nil,
		)

		_, err := svc.CreatePastMeeting(context.Background(), &pkgitx.CreatePastMeetingRequest{
			MeetingID:    "mtg-1",
			OccurrenceID: "1234567890",
			ProjectID:    "proj-1",
		})
		require.ErrorIs(t, err, lookupErr)
		assert.Nil(t, client.lastCreateReq)
	})
}

func TestPastMeetingService_UpdatePastMeeting_StampsUpdatedByNotCreatedBy(t *testing.T) {
	t.Run("stamps only updated_by on update", func(t *testing.T) {
		client := &fakePastMeetingClient{}
		reader := &fakeUserMetadataReader{profile: &domain.UserProfile{Username: "bob"}}
		svc := newPastMeetingSvc(client, "proj-1", reader)

		_, err := svc.UpdatePastMeeting(ctxWithPrincipal("bob", "bob@example.com"), "pm-1", &pkgitx.CreatePastMeetingRequest{
			ProjectID: "proj-1",
		})
		require.NoError(t, err)
		require.NotNil(t, client.lastUpdateReq.UpdatedBy)
		assert.Equal(t, "bob", client.lastUpdateReq.UpdatedBy.Username)
		assert.Equal(t, "bob@example.com", client.lastUpdateReq.UpdatedBy.Email)
		assert.Nil(t, client.lastUpdateReq.CreatedBy, "update must not overwrite original creator")
	})
}
