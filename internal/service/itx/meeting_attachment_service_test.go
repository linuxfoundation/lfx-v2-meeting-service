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

type fakeMeetingAttachmentClient struct {
	domain.ITXMeetingAttachmentClient
	lastCreateReq        *itx.CreateMeetingAttachmentRequest
	lastUpdateReq        *itx.UpdateMeetingAttachmentRequest
	lastCreatePresignReq *itx.CreateAttachmentPresignRequest
}

func (f *fakeMeetingAttachmentClient) CreateMeetingAttachment(_ context.Context, _ string, req *itx.CreateMeetingAttachmentRequest) (*itx.MeetingAttachment, error) {
	f.lastCreateReq = req
	return &itx.MeetingAttachment{}, nil
}

func (f *fakeMeetingAttachmentClient) UpdateMeetingAttachment(_ context.Context, _, _ string, req *itx.UpdateMeetingAttachmentRequest) error {
	f.lastUpdateReq = req
	return nil
}

func (f *fakeMeetingAttachmentClient) CreateMeetingAttachmentPresignURL(_ context.Context, _ string, req *itx.CreateAttachmentPresignRequest) (*itx.MeetingAttachmentPresignResponse, error) {
	f.lastCreatePresignReq = req
	return &itx.MeetingAttachmentPresignResponse{}, nil
}

func TestMeetingAttachmentService_StampsAuditFields(t *testing.T) {
	// The ticket called out that attachments were only sending `username` on
	// created_by/updated_by. These tests pin down that the service now enriches
	// with name+email via the UserMetadataReader — matching the meeting endpoint.

	newSvcWithReader := func() (*MeetingAttachmentService, *fakeMeetingAttachmentClient) {
		client := &fakeMeetingAttachmentClient{}
		reader := &fakeUserMetadataReader{profile: &domain.UserProfile{
			Username: "alice", Name: "Alice", Email: "alice@example.com",
		}}
		return NewMeetingAttachmentService(client, reader), client
	}

	t.Run("create stamps enriched CreatedBy", func(t *testing.T) {
		svc, client := newSvcWithReader()
		_, err := svc.CreateMeetingAttachment(ctxWithPrincipal("alice", ""), "mtg-1", &itx.CreateMeetingAttachmentRequest{Name: "notes.pdf"})
		require.NoError(t, err)
		require.NotNil(t, client.lastCreateReq.CreatedBy)
		assert.Equal(t, "alice", client.lastCreateReq.CreatedBy.Username)
		assert.Equal(t, "Alice", client.lastCreateReq.CreatedBy.Name)
		assert.Equal(t, "alice@example.com", client.lastCreateReq.CreatedBy.Email)
	})

	t.Run("update stamps enriched UpdatedBy", func(t *testing.T) {
		svc, client := newSvcWithReader()
		err := svc.UpdateMeetingAttachment(ctxWithPrincipal("alice", ""), "mtg-1", "att-1", &itx.UpdateMeetingAttachmentRequest{Name: "notes.pdf"})
		require.NoError(t, err)
		require.NotNil(t, client.lastUpdateReq.UpdatedBy)
		assert.Equal(t, "Alice", client.lastUpdateReq.UpdatedBy.Name)
		assert.Equal(t, "alice@example.com", client.lastUpdateReq.UpdatedBy.Email)
	})

	t.Run("presign also stamps CreatedBy (ITX persists a record with upload_status=ongoing)", func(t *testing.T) {
		svc, client := newSvcWithReader()
		_, err := svc.CreateMeetingAttachmentPresignURL(ctxWithPrincipal("alice", ""), "mtg-1", &itx.CreateAttachmentPresignRequest{Name: "notes.pdf", FileSize: 1, FileType: "application/pdf"})
		require.NoError(t, err)
		require.NotNil(t, client.lastCreatePresignReq.CreatedBy)
		assert.Equal(t, "Alice", client.lastCreatePresignReq.CreatedBy.Name)
	})

	t.Run("degrades to username-only without reader", func(t *testing.T) {
		client := &fakeMeetingAttachmentClient{}
		svc := NewMeetingAttachmentService(client, nil)

		_, err := svc.CreateMeetingAttachment(ctxWithPrincipal("carol", ""), "mtg-1", &itx.CreateMeetingAttachmentRequest{Name: "notes.pdf"})
		require.NoError(t, err)
		require.NotNil(t, client.lastCreateReq.CreatedBy)
		assert.Equal(t, "carol", client.lastCreateReq.CreatedBy.Username)
		assert.Empty(t, client.lastCreateReq.CreatedBy.Name)
	})

	t.Run("omits stamp without principal", func(t *testing.T) {
		client := &fakeMeetingAttachmentClient{}
		svc := NewMeetingAttachmentService(client, nil)

		_, err := svc.CreateMeetingAttachment(context.Background(), "mtg-1", &itx.CreateMeetingAttachmentRequest{Name: "notes.pdf"})
		require.NoError(t, err)
		assert.Nil(t, client.lastCreateReq.CreatedBy)
	})
}
