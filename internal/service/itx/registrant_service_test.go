// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package itx_test

import (
	"context"
	"errors"
	"testing"
	"time"

	inviteapi "github.com/linuxfoundation/lfx-v2-invite-service/pkg/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/mocks"
	svc "github.com/linuxfoundation/lfx-v2-meeting-service/internal/service/itx"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/models/itx"
)

// newService is a test helper that wires up a RegistrantService with all mocks.
func newService(
	client *mocks.MockITXRegistrantClient,
	idMapper *mocks.MockIDMapper,
	userReader *mocks.MockUserReader,
	inviteSender *mocks.MockInviteSender,
	selfServeURL string,
	inviteEnabled bool,
) *svc.RegistrantService {
	opts := []svc.Option{
		svc.WithUserReader(userReader),
		svc.WithInviteSender(inviteSender),
		svc.WithSelfServeBaseURL(selfServeURL),
		svc.WithInviteFeatureEnabled(inviteEnabled),
	}
	return svc.NewRegistrantService(client, idMapper, opts...)
}

// ---- CreateRegistrant tests ----

// LFID-known path: SubByEmail returns a sub → username set on request before ITX call.
func TestCreateRegistrant_LFIDKnown(t *testing.T) {
	ctx := context.Background()
	client := &mocks.MockITXRegistrantClient{}
	idMapper := &mocks.MockIDMapper{}
	userReader := &mocks.MockUserReader{}
	inviteSender := &mocks.MockInviteSender{}

	req := &itx.ZoomMeetingRegistrant{Email: "alice@example.com", FirstName: "Alice"}
	// userReader returns a sub for the email
	userReader.On("SubByEmail", ctx, "alice@example.com").Return("auth0|alice", nil)

	// ITX receives the request with Username already populated
	resp := &itx.ZoomMeetingRegistrant{ID: "reg-1", Email: "alice@example.com", Username: "auth0|alice"}
	client.On("CreateRegistrant", ctx, "meet-1", req).Return(resp, nil)

	s := newService(client, idMapper, userReader, inviteSender, "", true)
	got, err := s.CreateRegistrant(ctx, "meet-1", req)

	require.NoError(t, err)
	assert.Equal(t, "auth0|alice", got.Username)
	assert.Equal(t, "auth0|alice", req.Username) // mutated before ITX call
	client.AssertExpectations(t)
	userReader.AssertExpectations(t)
	inviteSender.AssertNotCalled(t, "SendInvite")
}

// Invite path: SubByEmail returns ErrUserNotFound → ITX call proceeds without username → invite issued.
func TestCreateRegistrant_InvitePath(t *testing.T) {
	ctx := context.Background()
	client := &mocks.MockITXRegistrantClient{}
	idMapper := &mocks.MockIDMapper{}
	userReader := &mocks.MockUserReader{}
	inviteSender := &mocks.MockInviteSender{}

	req := &itx.ZoomMeetingRegistrant{Email: "new@example.com", FirstName: "New", LastName: "User"}
	userReader.On("SubByEmail", ctx, "new@example.com").Return("", domain.ErrUserNotFound)

	// Response carries names back so issueInvite can build RecipientName.
	resp := &itx.ZoomMeetingRegistrant{ID: "reg-2", Email: "new@example.com", FirstName: "New", LastName: "User"}
	client.On("CreateRegistrant", ctx, "meet-1", req).Return(resp, nil)

	expiresAt := time.Now().Add(30 * 24 * time.Hour)
	inviteSender.On("SendInvite", ctx, inviteapi.SendInviteRequest{
		RecipientEmail: "new@example.com",
		RecipientName:  "New User",
		ResourceUID:    "meet-1",
		ResourceName:   "meet-1",
		ResourceType:   "meeting",
		Role:           "Member",
		ReturnURL:      "",
		ExpirationDays: 30,
	}).Return(domain.InviteResult{InviteUID: "inv-abc", ExpiresAt: expiresAt}, nil)

	client.On("UpdateRegistrantInvite", ctx, "reg-2", domain.ITXRegistrantInviteFields{
		LFIDInviteUID:       "inv-abc",
		LFIDInviteEmail:     "new@example.com",
		LFIDInviteExpiresAt: expiresAt.Format(time.RFC3339),
	}).Return(nil)

	s := newService(client, idMapper, userReader, inviteSender, "", true)
	got, err := s.CreateRegistrant(ctx, "meet-1", req)

	require.NoError(t, err)
	assert.Equal(t, "reg-2", got.ID)
	assert.Empty(t, got.Username)
	client.AssertExpectations(t)
	userReader.AssertExpectations(t)
	inviteSender.AssertExpectations(t)
}

// Transport error from SubByEmail → fail the request with UnavailableError (503).
func TestCreateRegistrant_LookupTransportError(t *testing.T) {
	ctx := context.Background()
	client := &mocks.MockITXRegistrantClient{}
	idMapper := &mocks.MockIDMapper{}
	userReader := &mocks.MockUserReader{}
	inviteSender := &mocks.MockInviteSender{}

	req := &itx.ZoomMeetingRegistrant{Email: "bob@example.com"}
	transportErr := errors.New("nats: timeout")
	userReader.On("SubByEmail", ctx, "bob@example.com").Return("", transportErr)

	s := newService(client, idMapper, userReader, inviteSender, "", true)
	_, err := s.CreateRegistrant(ctx, "meet-1", req)

	require.Error(t, err)
	assert.Equal(t, domain.ErrorTypeUnavailable, domain.GetErrorType(err))
	client.AssertNotCalled(t, "CreateRegistrant")
	inviteSender.AssertNotCalled(t, "SendInvite")
}

// SendInvite failure → warn-log but return 201 (registrant already created in ITX).
func TestCreateRegistrant_SendInviteFailure(t *testing.T) {
	ctx := context.Background()
	client := &mocks.MockITXRegistrantClient{}
	idMapper := &mocks.MockIDMapper{}
	userReader := &mocks.MockUserReader{}
	inviteSender := &mocks.MockInviteSender{}

	req := &itx.ZoomMeetingRegistrant{Email: "carol@example.com"}
	userReader.On("SubByEmail", ctx, "carol@example.com").Return("", domain.ErrUserNotFound)

	resp := &itx.ZoomMeetingRegistrant{ID: "reg-3", Email: "carol@example.com"}
	client.On("CreateRegistrant", ctx, "meet-1", req).Return(resp, nil)

	inviteSender.On("SendInvite", ctx, mock.AnythingOfType("api.SendInviteRequest")).
		Return(domain.InviteResult{}, errors.New("invite service unavailable"))

	s := newService(client, idMapper, userReader, inviteSender, "", true)
	got, err := s.CreateRegistrant(ctx, "meet-1", req)

	// The registrant is returned despite the invite failure
	require.NoError(t, err)
	assert.Equal(t, "reg-3", got.ID)
	client.AssertNotCalled(t, "UpdateRegistrantInvite")
}

// UpdateRegistrantInvite failure → warn-log but return 201 (registrant + invite exist, metadata missing).
func TestCreateRegistrant_UpdateInviteFailure(t *testing.T) {
	ctx := context.Background()
	client := &mocks.MockITXRegistrantClient{}
	idMapper := &mocks.MockIDMapper{}
	userReader := &mocks.MockUserReader{}
	inviteSender := &mocks.MockInviteSender{}

	req := &itx.ZoomMeetingRegistrant{Email: "dave@example.com"}
	userReader.On("SubByEmail", ctx, "dave@example.com").Return("", domain.ErrUserNotFound)

	resp := &itx.ZoomMeetingRegistrant{ID: "reg-4", Email: "dave@example.com"}
	client.On("CreateRegistrant", ctx, "meet-1", req).Return(resp, nil)

	expiresAt := time.Now().Add(30 * 24 * time.Hour)
	inviteSender.On("SendInvite", ctx, mock.AnythingOfType("api.SendInviteRequest")).
		Return(domain.InviteResult{InviteUID: "inv-xyz", ExpiresAt: expiresAt}, nil)

	client.On("UpdateRegistrantInvite", ctx, "reg-4", mock.AnythingOfType("domain.ITXRegistrantInviteFields")).
		Return(errors.New("itx: 500 internal server error"))

	s := newService(client, idMapper, userReader, inviteSender, "", true)
	got, err := s.CreateRegistrant(ctx, "meet-1", req)

	require.NoError(t, err)
	assert.Equal(t, "reg-4", got.ID)
}

// Invite feature disabled → no lookup, no invite, pass-through.
func TestCreateRegistrant_FeatureDisabled(t *testing.T) {
	ctx := context.Background()
	client := &mocks.MockITXRegistrantClient{}
	idMapper := &mocks.MockIDMapper{}
	userReader := &mocks.MockUserReader{}
	inviteSender := &mocks.MockInviteSender{}

	req := &itx.ZoomMeetingRegistrant{Email: "eve@example.com"}
	resp := &itx.ZoomMeetingRegistrant{ID: "reg-5", Email: "eve@example.com"}
	client.On("CreateRegistrant", ctx, "meet-1", req).Return(resp, nil)

	s := newService(client, idMapper, userReader, inviteSender, "", false)
	got, err := s.CreateRegistrant(ctx, "meet-1", req)

	require.NoError(t, err)
	assert.Equal(t, "reg-5", got.ID)
	userReader.AssertNotCalled(t, "SubByEmail")
	inviteSender.AssertNotCalled(t, "SendInvite")
}

// Username already set on request → no lookup, no invite.
func TestCreateRegistrant_UsernamePrePopulated(t *testing.T) {
	ctx := context.Background()
	client := &mocks.MockITXRegistrantClient{}
	idMapper := &mocks.MockIDMapper{}
	userReader := &mocks.MockUserReader{}
	inviteSender := &mocks.MockInviteSender{}

	req := &itx.ZoomMeetingRegistrant{Email: "frank@example.com", Username: "frank-lfid"}
	resp := &itx.ZoomMeetingRegistrant{ID: "reg-6", Email: "frank@example.com", Username: "frank-lfid"}
	client.On("CreateRegistrant", ctx, "meet-1", req).Return(resp, nil)

	s := newService(client, idMapper, userReader, inviteSender, "", true)
	got, err := s.CreateRegistrant(ctx, "meet-1", req)

	require.NoError(t, err)
	assert.Equal(t, "frank-lfid", got.Username)
	userReader.AssertNotCalled(t, "SubByEmail")
	inviteSender.AssertNotCalled(t, "SendInvite")
}

// ---- ReconcileAcceptedInvite tests ----

func TestReconcileAcceptedInvite_Success(t *testing.T) {
	ctx := context.Background()
	client := &mocks.MockITXRegistrantClient{}
	idMapper := &mocks.MockIDMapper{}

	updated := []*itx.ZoomMeetingRegistrant{
		{ID: "reg-1", MeetingID: "meet-1", Email: "grace@example.com", Username: "grace-lfid"},
		{ID: "reg-2", MeetingID: "meet-2", Email: "grace@example.com", Username: "grace-lfid"},
	}
	client.On("AcceptInvite", ctx, "grace@example.com", "grace-lfid").
		Return(&domain.ITXAcceptInviteResult{Updated: updated}, nil)

	s := svc.NewRegistrantService(client, idMapper)
	got, err := s.ReconcileAcceptedInvite(ctx, "grace@example.com", "grace-lfid")

	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "grace-lfid", got[0].Username)
	client.AssertExpectations(t)
}

// ITX returns empty updated list → already reconciled; no error returned.
func TestReconcileAcceptedInvite_AlreadyReconciled(t *testing.T) {
	ctx := context.Background()
	client := &mocks.MockITXRegistrantClient{}
	idMapper := &mocks.MockIDMapper{}

	client.On("AcceptInvite", ctx, "grace@example.com", "grace-lfid").
		Return(&domain.ITXAcceptInviteResult{Updated: nil}, nil)

	s := svc.NewRegistrantService(client, idMapper)
	got, err := s.ReconcileAcceptedInvite(ctx, "grace@example.com", "grace-lfid")

	require.NoError(t, err)
	assert.Nil(t, got)
}

// ITX returns an error → propagated to caller.
func TestReconcileAcceptedInvite_ITXError(t *testing.T) {
	ctx := context.Background()
	client := &mocks.MockITXRegistrantClient{}
	idMapper := &mocks.MockIDMapper{}

	client.On("AcceptInvite", ctx, "grace@example.com", "grace-lfid").
		Return((*domain.ITXAcceptInviteResult)(nil), errors.New("itx: 503"))

	s := svc.NewRegistrantService(client, idMapper)
	_, err := s.ReconcileAcceptedInvite(ctx, "grace@example.com", "grace-lfid")

	require.Error(t, err)
}
