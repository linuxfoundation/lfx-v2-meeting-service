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

type fakePastMeetingAttachmentClient struct {
	domain.ITXPastMeetingAttachmentClient
	lastCreateReq        *itx.CreatePastMeetingAttachmentRequest
	lastUpdateReq        *itx.UpdatePastMeetingAttachmentRequest
	lastCreatePresignReq *itx.CreateAttachmentPresignRequest
}

func (f *fakePastMeetingAttachmentClient) CreatePastMeetingAttachment(_ context.Context, _ string, req *itx.CreatePastMeetingAttachmentRequest) (*itx.PastMeetingAttachment, error) {
	f.lastCreateReq = req
	return &itx.PastMeetingAttachment{}, nil
}

func (f *fakePastMeetingAttachmentClient) UpdatePastMeetingAttachment(_ context.Context, _, _ string, req *itx.UpdatePastMeetingAttachmentRequest) error {
	f.lastUpdateReq = req
	return nil
}

func (f *fakePastMeetingAttachmentClient) CreatePastMeetingAttachmentPresignURL(_ context.Context, _ string, req *itx.CreateAttachmentPresignRequest) (*itx.PastMeetingAttachmentPresignResponse, error) {
	f.lastCreatePresignReq = req
	return &itx.PastMeetingAttachmentPresignResponse{}, nil
}

func TestPastMeetingAttachmentService_StampsAuditFields(t *testing.T) {
	newSvcWithReader := func() (*PastMeetingAttachmentService, *fakePastMeetingAttachmentClient) {
		client := &fakePastMeetingAttachmentClient{}
		reader := &fakeUserMetadataReader{profile: &domain.UserProfile{
			Username: "alice", Name: "Alice", Email: "alice@example.com",
		}}
		return NewPastMeetingAttachmentService(client, reader), client
	}

	t.Run("create stamps enriched CreatedBy", func(t *testing.T) {
		svc, client := newSvcWithReader()
		_, err := svc.CreatePastMeetingAttachment(ctxWithPrincipal("alice", ""), "pm-1", &itx.CreatePastMeetingAttachmentRequest{Name: "recording-notes.pdf"})
		require.NoError(t, err)
		require.NotNil(t, client.lastCreateReq.CreatedBy)
		assert.Equal(t, "Alice", client.lastCreateReq.CreatedBy.Name)
		assert.Equal(t, "alice@example.com", client.lastCreateReq.CreatedBy.Email)
	})

	t.Run("update stamps enriched UpdatedBy", func(t *testing.T) {
		svc, client := newSvcWithReader()
		err := svc.UpdatePastMeetingAttachment(ctxWithPrincipal("alice", ""), "pm-1", "att-1", &itx.UpdatePastMeetingAttachmentRequest{Name: "notes.pdf"})
		require.NoError(t, err)
		require.NotNil(t, client.lastUpdateReq.UpdatedBy)
		assert.Equal(t, "Alice", client.lastUpdateReq.UpdatedBy.Name)
	})

	t.Run("presign stamps CreatedBy", func(t *testing.T) {
		svc, client := newSvcWithReader()
		_, err := svc.CreatePastMeetingAttachmentPresignURL(ctxWithPrincipal("alice", ""), "pm-1", &itx.CreateAttachmentPresignRequest{Name: "notes.pdf", FileSize: 1, FileType: "application/pdf"})
		require.NoError(t, err)
		require.NotNil(t, client.lastCreatePresignReq.CreatedBy)
		assert.Equal(t, "Alice", client.lastCreatePresignReq.CreatedBy.Name)
	})

	t.Run("omits stamp without principal", func(t *testing.T) {
		client := &fakePastMeetingAttachmentClient{}
		svc := NewPastMeetingAttachmentService(client, nil)

		_, err := svc.CreatePastMeetingAttachment(context.Background(), "pm-1", &itx.CreatePastMeetingAttachmentRequest{Name: "notes.pdf"})
		require.NoError(t, err)
		assert.Nil(t, client.lastCreateReq.CreatedBy)
	})
}
