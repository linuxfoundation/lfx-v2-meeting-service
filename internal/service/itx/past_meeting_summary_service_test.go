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
	updateCalls   int
	getCalls      int
	getParentID   string
	getSummaryID  string
	getResp       *itx.PastMeetingSummaryResponse
	getErr        error
}

func (f *fakePastMeetingSummaryClient) GetPastMeetingSummary(_ context.Context, pastMeetingID, summaryID string) (*itx.PastMeetingSummaryResponse, error) {
	f.getCalls++
	f.getParentID = pastMeetingID
	f.getSummaryID = summaryID
	return f.getResp, f.getErr
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
	f.updateCalls++
	f.lastUpdateReq = req
	return &itx.PastMeetingSummaryResponse{}, nil
}

func TestPastMeetingSummaryService_UpdatePastMeetingSummary(t *testing.T) {
	// The ticket flagged this call out explicitly because the ITX side uses the
	// non-standard field name modified_by (not updated_by), and the previous
	// converter comment claimed ITX would derive the value from the JWT — which
	// isn't accurate for our M2M-token calls.

	t.Run("stamps full profile", func(t *testing.T) {
		client := &fakePastMeetingSummaryClient{getResp: &itx.PastMeetingSummaryResponse{MeetingAndOccurrenceID: "pm-1"}}
		reader := &fakeUserMetadataReader{profile: &domain.UserProfile{
			Username: "alice", Name: "Alice", Email: "alice@example.com",
		}}
		svc := NewPastMeetingSummaryService(client, reader, nil)

		_, err := svc.UpdatePastMeetingSummary(ctxWithPrincipal("alice", ""), "pm-1", "sum-1", &itx.UpdatePastMeetingSummaryRequest{})
		require.NoError(t, err)
		assert.Equal(t, 1, client.getCalls)
		assert.Equal(t, "pm-1", client.getParentID)
		assert.Equal(t, "sum-1", client.getSummaryID)
		assert.Equal(t, 1, client.updateCalls)
		require.NotNil(t, client.lastUpdateReq.ModifiedBy)
		assert.Equal(t, "alice", client.lastUpdateReq.ModifiedBy.Username)
		assert.Equal(t, "Alice", client.lastUpdateReq.ModifiedBy.Name)
	})

	t.Run("omits modified_by without principal", func(t *testing.T) {
		client := &fakePastMeetingSummaryClient{getResp: &itx.PastMeetingSummaryResponse{MeetingAndOccurrenceID: "pm-1"}}
		svc := NewPastMeetingSummaryService(client, nil, nil)

		_, err := svc.UpdatePastMeetingSummary(context.Background(), "pm-1", "sum-1", &itx.UpdatePastMeetingSummaryRequest{})
		require.NoError(t, err)
		assert.Nil(t, client.lastUpdateReq.ModifiedBy)
	})

	t.Run("rejects a summary belonging to another past meeting before writing", func(t *testing.T) {
		client := &fakePastMeetingSummaryClient{getResp: &itx.PastMeetingSummaryResponse{MeetingAndOccurrenceID: "pm-1"}}
		svc := NewPastMeetingSummaryService(client, nil, nil)
		approved := true

		resp, err := svc.UpdatePastMeetingSummary(ctxWithPrincipal("alice", ""), "pm-2", "sum-1", &itx.UpdatePastMeetingSummaryRequest{Approved: &approved})
		require.Error(t, err)
		assert.Nil(t, resp)
		assert.Equal(t, domain.ErrorTypeForbidden, domain.GetErrorType(err))
		assert.Equal(t, 1, client.getCalls)
		assert.Equal(t, "pm-2", client.getParentID)
		assert.Equal(t, "sum-1", client.getSummaryID)
		assert.Zero(t, client.updateCalls)
	})

	t.Run("rejects a summary with no parent before writing", func(t *testing.T) {
		client := &fakePastMeetingSummaryClient{getResp: &itx.PastMeetingSummaryResponse{}}
		svc := NewPastMeetingSummaryService(client, nil, nil)

		resp, err := svc.UpdatePastMeetingSummary(context.Background(), "pm-1", "sum-1", &itx.UpdatePastMeetingSummaryRequest{})
		require.Error(t, err)
		assert.Nil(t, resp)
		assert.Equal(t, domain.ErrorTypeForbidden, domain.GetErrorType(err))
		assert.Zero(t, client.updateCalls)
	})

	t.Run("does not write when the summary is absent", func(t *testing.T) {
		client := &fakePastMeetingSummaryClient{}
		svc := NewPastMeetingSummaryService(client, nil, nil)

		resp, err := svc.UpdatePastMeetingSummary(context.Background(), "pm-1", "sum-1", &itx.UpdatePastMeetingSummaryRequest{})
		require.Error(t, err)
		assert.Nil(t, resp)
		assert.Equal(t, domain.ErrorTypeNotFound, domain.GetErrorType(err))
		assert.Zero(t, client.updateCalls)
	})

	t.Run("does not write when the lookup fails", func(t *testing.T) {
		client := &fakePastMeetingSummaryClient{getErr: domain.NewUnavailableError("ITX unavailable")}
		svc := NewPastMeetingSummaryService(client, nil, nil)

		resp, err := svc.UpdatePastMeetingSummary(context.Background(), "pm-1", "sum-1", &itx.UpdatePastMeetingSummaryRequest{})
		require.Error(t, err)
		assert.Nil(t, resp)
		assert.ErrorIs(t, err, client.getErr)
		assert.Zero(t, client.updateCalls)
	})
}

func TestPastMeetingSummaryService_GetPastMeetingSummary(t *testing.T) {
	awaiting := func() *itx.PastMeetingSummaryResponse {
		return &itx.PastMeetingSummaryResponse{ID: "sum-1", MeetingAndOccurrenceID: "pm-1", SummaryOverview: "draft", RequiresApproval: true, Approved: false}
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

	t.Run("awaiting approval is forbidden when the summary belongs to another past meeting", func(t *testing.T) {
		// An organizer of pm-2 must not satisfy the gate for pm-1's pending summary.
		client := &fakePastMeetingSummaryClient{getResp: awaiting()}
		authz := &fakePastMeetingAuthorizer{organizer: true}
		svc := NewPastMeetingSummaryService(client, nil, authz)

		resp, err := svc.GetPastMeetingSummary(ctxWithPrincipal("alice", ""), "pm-2", "sum-1")
		require.Error(t, err)
		assert.Nil(t, resp)
		assert.Equal(t, domain.ErrorTypeForbidden, domain.GetErrorType(err))
		assert.Zero(t, authz.calls)
	})

	t.Run("summary is forbidden under a different past meeting regardless of approval", func(t *testing.T) {
		for _, tc := range []struct {
			name             string
			requiresApproval bool
			approved         bool
			parentID         string
		}{
			{name: "approved", requiresApproval: true, approved: true, parentID: "pm-1"},
			{name: "approval not required", parentID: "pm-1"},
			{name: "missing parent", requiresApproval: true, approved: true},
		} {
			t.Run(tc.name, func(t *testing.T) {
				client := &fakePastMeetingSummaryClient{getResp: &itx.PastMeetingSummaryResponse{
					MeetingAndOccurrenceID: tc.parentID,
					SummaryOverview:        "private summary",
					RequiresApproval:       tc.requiresApproval,
					Approved:               tc.approved,
				}}
				authz := &fakePastMeetingAuthorizer{organizer: true}
				svc := NewPastMeetingSummaryService(client, nil, authz)

				resp, err := svc.GetPastMeetingSummary(ctxWithPrincipal("alice", ""), "pm-2", "sum-1")
				require.Error(t, err)
				assert.Nil(t, resp, "the mismatched ITX response must not reach the converter")
				assert.Equal(t, domain.ErrorTypeForbidden, domain.GetErrorType(err))
				assert.NotContains(t, err.Error(), "private summary")
				assert.Zero(t, authz.calls)
			})
		}
	})

	t.Run("ITX error is returned without an organizer check", func(t *testing.T) {
		client := &fakePastMeetingSummaryClient{getErr: domain.NewNotFoundError("summary not found")}
		authz := &fakePastMeetingAuthorizer{organizer: true}
		svc := NewPastMeetingSummaryService(client, nil, authz)

		resp, err := svc.GetPastMeetingSummary(ctxWithPrincipal("alice", ""), "pm-1", "sum-1")
		require.Error(t, err)
		assert.Nil(t, resp)
		assert.Equal(t, domain.ErrorTypeNotFound, domain.GetErrorType(err))
		assert.Zero(t, authz.calls)
	})

	t.Run("approved summary is returned without an organizer check", func(t *testing.T) {
		client := &fakePastMeetingSummaryClient{getResp: &itx.PastMeetingSummaryResponse{MeetingAndOccurrenceID: "pm-1", RequiresApproval: true, Approved: true}}
		authz := &fakePastMeetingAuthorizer{organizer: false}
		svc := NewPastMeetingSummaryService(client, nil, authz)

		resp, err := svc.GetPastMeetingSummary(ctxWithPrincipal("bob", ""), "pm-1", "sum-1")
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Zero(t, authz.calls)
	})

	t.Run("summary not requiring approval is returned without an organizer check", func(t *testing.T) {
		client := &fakePastMeetingSummaryClient{getResp: &itx.PastMeetingSummaryResponse{MeetingAndOccurrenceID: "pm-1", RequiresApproval: false}}
		authz := &fakePastMeetingAuthorizer{organizer: false}
		svc := NewPastMeetingSummaryService(client, nil, authz)

		resp, err := svc.GetPastMeetingSummary(context.Background(), "pm-1", "sum-1")
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Zero(t, authz.calls)
	})
}
