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
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/models/itx"
)

type fakePastMeetingSummaryClient struct {
	domain.ITXPastMeetingSummaryClient
	lastUpdateReq *itx.UpdatePastMeetingSummaryRequest
	getResp       *itx.PastMeetingSummaryResponse
}

func (f *fakePastMeetingSummaryClient) GetPastMeetingSummary(_ context.Context, _, _ string) (*itx.PastMeetingSummaryResponse, error) {
	return f.getResp, nil
}

type fakePastMeetingAuthorizer struct {
	organizer     bool
	err           error
	calls         int
	lastPrincipal string
	lastPastMtgID string
}

func (f *fakePastMeetingAuthorizer) IsOrganizer(_ context.Context, principal, pastMeetingID string) (bool, error) {
	f.calls++
	f.lastPrincipal = principal
	f.lastPastMtgID = pastMeetingID
	return f.organizer, f.err
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
		svc := NewPastMeetingSummaryService(client, reader, nil)

		_, err := svc.UpdatePastMeetingSummary(ctxWithPrincipal("alice", ""), "pm-1", "sum-1", &itx.UpdatePastMeetingSummaryRequest{})
		require.NoError(t, err)
		require.NotNil(t, client.lastUpdateReq.ModifiedBy)
		assert.Equal(t, "alice", client.lastUpdateReq.ModifiedBy.Username)
		assert.Equal(t, "Alice", client.lastUpdateReq.ModifiedBy.Name)
	})

	t.Run("omits modified_by without principal", func(t *testing.T) {
		client := &fakePastMeetingSummaryClient{}
		svc := NewPastMeetingSummaryService(client, nil, nil)

		_, err := svc.UpdatePastMeetingSummary(context.Background(), "pm-1", "sum-1", &itx.UpdatePastMeetingSummaryRequest{})
		require.NoError(t, err)
		assert.Nil(t, client.lastUpdateReq.ModifiedBy)
	})
}

func TestPastMeetingSummaryService_GetPastMeetingSummary(t *testing.T) {
	awaiting := func() *itx.PastMeetingSummaryResponse {
		return &itx.PastMeetingSummaryResponse{ID: "sum-1", SummaryOverview: "draft", RequiresApproval: true, Approved: false}
	}

	t.Run("awaiting approval is returned to an organizer", func(t *testing.T) {
		client := &fakePastMeetingSummaryClient{getResp: awaiting()}
		authz := &fakePastMeetingAuthorizer{organizer: true}
		svc := NewPastMeetingSummaryService(client, nil, authz)

		resp, err := svc.GetPastMeetingSummary(ctxWithPrincipal("alice", ""), "pm-1", "sum-1")
		require.NoError(t, err)
		assert.Equal(t, "draft", resp.SummaryOverview)
		assert.Equal(t, "alice", authz.lastPrincipal)
		assert.Equal(t, "pm-1", authz.lastPastMtgID)
	})

	t.Run("awaiting approval is forbidden to a non-organizer", func(t *testing.T) {
		client := &fakePastMeetingSummaryClient{getResp: awaiting()}
		svc := NewPastMeetingSummaryService(client, nil, &fakePastMeetingAuthorizer{organizer: false})

		resp, err := svc.GetPastMeetingSummary(ctxWithPrincipal("bob", ""), "pm-1", "sum-1")
		require.Error(t, err)
		assert.Nil(t, resp)
		assert.Equal(t, domain.ErrorTypeForbidden, domain.GetErrorType(err))
	})

	t.Run("awaiting approval is forbidden without a principal", func(t *testing.T) {
		client := &fakePastMeetingSummaryClient{getResp: awaiting()}
		authz := &fakePastMeetingAuthorizer{organizer: true}
		svc := NewPastMeetingSummaryService(client, nil, authz)

		resp, err := svc.GetPastMeetingSummary(context.Background(), "pm-1", "sum-1")
		require.Error(t, err)
		assert.Nil(t, resp)
		assert.Equal(t, domain.ErrorTypeForbidden, domain.GetErrorType(err))
		assert.Zero(t, authz.calls)
	})

	t.Run("awaiting approval is forbidden when no authorizer is configured", func(t *testing.T) {
		client := &fakePastMeetingSummaryClient{getResp: awaiting()}
		svc := NewPastMeetingSummaryService(client, nil, nil)

		resp, err := svc.GetPastMeetingSummary(ctxWithPrincipal("alice", ""), "pm-1", "sum-1")
		require.Error(t, err)
		assert.Nil(t, resp)
		assert.Equal(t, domain.ErrorTypeForbidden, domain.GetErrorType(err))
	})

	t.Run("awaiting approval fails closed when the organizer check errors", func(t *testing.T) {
		client := &fakePastMeetingSummaryClient{getResp: awaiting()}
		svc := NewPastMeetingSummaryService(client, nil, &fakePastMeetingAuthorizer{organizer: true, err: errors.New("nats: timeout")})

		resp, err := svc.GetPastMeetingSummary(ctxWithPrincipal("alice", ""), "pm-1", "sum-1")
		require.Error(t, err)
		assert.Nil(t, resp)
		assert.Equal(t, domain.ErrorTypeUnavailable, domain.GetErrorType(err))
		assert.NotContains(t, err.Error(), "nats")
	})

	t.Run("approved summary is returned without an organizer check", func(t *testing.T) {
		client := &fakePastMeetingSummaryClient{getResp: &itx.PastMeetingSummaryResponse{RequiresApproval: true, Approved: true}}
		authz := &fakePastMeetingAuthorizer{organizer: false}
		svc := NewPastMeetingSummaryService(client, nil, authz)

		resp, err := svc.GetPastMeetingSummary(ctxWithPrincipal("bob", ""), "pm-1", "sum-1")
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Zero(t, authz.calls)
	})

	t.Run("summary not requiring approval is returned without an organizer check", func(t *testing.T) {
		client := &fakePastMeetingSummaryClient{getResp: &itx.PastMeetingSummaryResponse{RequiresApproval: false}}
		authz := &fakePastMeetingAuthorizer{organizer: false}
		svc := NewPastMeetingSummaryService(client, nil, authz)

		resp, err := svc.GetPastMeetingSummary(context.Background(), "pm-1", "sum-1")
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Zero(t, authz.calls)
	})
}
