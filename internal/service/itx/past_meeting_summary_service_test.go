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

type fakePastMeetingSummaryClient struct {
	domain.ITXPastMeetingSummaryClient
	lastUpdateReq *itx.UpdatePastMeetingSummaryRequest
}

func (f *fakePastMeetingSummaryClient) UpdatePastMeetingSummary(_ context.Context, _, _ string, req *itx.UpdatePastMeetingSummaryRequest) (*itx.PastMeetingSummaryResponse, error) {
	f.lastUpdateReq = req
	return &itx.PastMeetingSummaryResponse{}, nil
}

func TestPastMeetingSummaryService_Update_StampsModifiedBy(t *testing.T) {
	// The ticket flagged this call out explicitly because the ITX side uses the
	// non-standard field name modified_by (not updated_by), and the previous
	// converter comment claimed ITX would derive the value from the JWT — which
	// isn't accurate for our M2M-token calls.

	t.Run("stamps full profile", func(t *testing.T) {
		client := &fakePastMeetingSummaryClient{}
		reader := &fakeUserMetadataReader{profile: &domain.UserProfile{
			Username: "alice", Name: "Alice", Email: "alice@example.com",
		}}
		svc := NewPastMeetingSummaryService(client, reader)

		_, err := svc.UpdatePastMeetingSummary(ctxWithPrincipal("alice", ""), "pm-1", "sum-1", &itx.UpdatePastMeetingSummaryRequest{})
		require.NoError(t, err)
		require.NotNil(t, client.lastUpdateReq.ModifiedBy)
		assert.Equal(t, "alice", client.lastUpdateReq.ModifiedBy.Username)
		assert.Equal(t, "Alice", client.lastUpdateReq.ModifiedBy.Name)
	})

	t.Run("omits modified_by without principal", func(t *testing.T) {
		client := &fakePastMeetingSummaryClient{}
		svc := NewPastMeetingSummaryService(client, nil)

		_, err := svc.UpdatePastMeetingSummary(context.Background(), "pm-1", "sum-1", &itx.UpdatePastMeetingSummaryRequest{})
		require.NoError(t, err)
		assert.Nil(t, client.lastUpdateReq.ModifiedBy)
	})
}
