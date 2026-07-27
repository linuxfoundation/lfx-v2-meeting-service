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

func TestRegistrantService_CreateRegistrant_StampsCreatedBy(t *testing.T) {
	t.Run("stamps full profile on create", func(t *testing.T) {
		client := &fakeRegistrantClient{}
		reader := &fakeUserMetadataReader{profile: &domain.UserProfile{
			Username: "alice", Name: "Alice", Email: "alice@example.com",
		}}
		svc := NewRegistrantService(client, noOpIDMapper{}, reader)

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
		svc := NewRegistrantService(client, noOpIDMapper{}, nil)

		_, err := svc.CreateRegistrant(context.Background(), "mtg-1", &itx.ZoomMeetingRegistrant{})
		require.NoError(t, err)
		assert.Nil(t, client.lastCreateReq.CreatedBy)
	})
}

func TestRegistrantService_UpdateRegistrant_StampsUpdatedByNotCreatedBy(t *testing.T) {
	t.Run("stamps only updated_by on update", func(t *testing.T) {
		client := &fakeRegistrantClient{}
		reader := &fakeUserMetadataReader{profile: &domain.UserProfile{Username: "bob", Email: "bob@example.com"}}
		svc := NewRegistrantService(client, noOpIDMapper{}, reader)

		err := svc.UpdateRegistrant(ctxWithPrincipal("bob", ""), "mtg-1", "reg-1", &itx.ZoomMeetingRegistrant{})
		require.NoError(t, err)
		require.NotNil(t, client.lastUpdateReq.UpdatedBy)
		assert.Equal(t, "bob", client.lastUpdateReq.UpdatedBy.Username)
		// Never overwrite the original creator on update.
		assert.Nil(t, client.lastUpdateReq.CreatedBy)
	})
}
