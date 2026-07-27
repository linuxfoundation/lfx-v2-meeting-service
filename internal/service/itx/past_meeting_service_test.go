// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package itx

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/models/itx"
)

// fakePastMeetingClient captures the past-meeting requests sent to ITX so tests can
// assert on the outbound created_by / updated_by stamping.
type fakePastMeetingClient struct {
	domain.ITXPastMeetingClient
	lastCreateReq *itx.CreatePastMeetingRequest
	lastUpdateReq *itx.CreatePastMeetingRequest
}

func (f *fakePastMeetingClient) CreatePastMeeting(_ context.Context, req *itx.CreatePastMeetingRequest) (*itx.PastMeetingResponse, error) {
	f.lastCreateReq = req
	return &itx.PastMeetingResponse{}, nil
}

func (f *fakePastMeetingClient) UpdatePastMeeting(_ context.Context, _ string, req *itx.CreatePastMeetingRequest) (*itx.PastMeetingResponse, error) {
	f.lastUpdateReq = req
	return &itx.PastMeetingResponse{}, nil
}

func TestPastMeetingService_CreatePastMeeting_StampsCreatedBy(t *testing.T) {
	t.Run("stamps full profile on create; leaves updated_by nil", func(t *testing.T) {
		client := &fakePastMeetingClient{}
		reader := &fakeUserMetadataReader{profile: &domain.UserProfile{
			Username: "alice", Name: "Alice", Email: "alice@example.com",
		}}
		svc := NewPastMeetingService(client, noOpIDMapper{}, reader)

		_, err := svc.CreatePastMeeting(ctxWithPrincipal("alice", ""), &itx.CreatePastMeetingRequest{
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
		svc := NewPastMeetingService(client, noOpIDMapper{}, nil)
		_, err := svc.CreatePastMeeting(context.Background(), &itx.CreatePastMeetingRequest{
			MeetingID:    "mtg-1",
			OccurrenceID: "1234567890",
			ProjectID:    "proj-1",
		})
		require.NoError(t, err)
		assert.Nil(t, client.lastCreateReq.CreatedBy)
	})
}

func TestPastMeetingService_UpdatePastMeeting_StampsUpdatedByNotCreatedBy(t *testing.T) {
	t.Run("stamps only updated_by on update", func(t *testing.T) {
		client := &fakePastMeetingClient{}
		reader := &fakeUserMetadataReader{profile: &domain.UserProfile{Username: "bob"}}
		svc := NewPastMeetingService(client, noOpIDMapper{}, reader)

		_, err := svc.UpdatePastMeeting(ctxWithPrincipal("bob", "bob@example.com"), "pm-1", &itx.CreatePastMeetingRequest{
			ProjectID: "proj-1",
		})
		require.NoError(t, err)
		require.NotNil(t, client.lastUpdateReq.UpdatedBy)
		assert.Equal(t, "bob", client.lastUpdateReq.UpdatedBy.Username)
		assert.Equal(t, "bob@example.com", client.lastUpdateReq.UpdatedBy.Email)
		assert.Nil(t, client.lastUpdateReq.CreatedBy, "update must not overwrite original creator")
	})
}
