// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package itx

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/models/itx"
)

// fakeParticipantClient records the create/update/delete calls made through it so
// tests can pin down created_by / updated_by stamping across both invitee and
// attendee paths.
type fakeParticipantClient struct {
	domain.ITXPastMeetingParticipantClient

	inviteeCreateReq  *itx.CreateInviteeRequest
	inviteeUpdateReq  *itx.UpdateInviteeRequest
	attendeeCreateReq *itx.CreateAttendeeRequest
	attendeeUpdateReq *itx.UpdateAttendeeRequest
}

func (f *fakeParticipantClient) CreateInvitee(_ context.Context, _ string, req *itx.CreateInviteeRequest) (*itx.InviteeResponse, error) {
	f.inviteeCreateReq = req
	return &itx.InviteeResponse{UUID: "invitee-1"}, nil
}

func (f *fakeParticipantClient) UpdateInvitee(_ context.Context, _, _ string, req *itx.UpdateInviteeRequest) (*itx.InviteeResponse, error) {
	f.inviteeUpdateReq = req
	return &itx.InviteeResponse{UUID: "invitee-1"}, nil
}

func (f *fakeParticipantClient) DeleteInvitee(_ context.Context, _, _ string) error { return nil }

func (f *fakeParticipantClient) CreateAttendee(_ context.Context, _ string, req *itx.CreateAttendeeRequest) (*itx.AttendeeResponse, error) {
	f.attendeeCreateReq = req
	return &itx.AttendeeResponse{ID: "attendee-1"}, nil
}

func (f *fakeParticipantClient) UpdateAttendee(_ context.Context, _, _ string, req *itx.UpdateAttendeeRequest) (*itx.AttendeeResponse, error) {
	f.attendeeUpdateReq = req
	return &itx.AttendeeResponse{ID: "attendee-1"}, nil
}

func (f *fakeParticipantClient) DeleteAttendee(_ context.Context, _, _ string) error { return nil }

// participantIDMapper lets tests toggle whether the invitee / attendee mappings
// resolve, which is how the participant service decides between create and update
// paths on an update request.
type participantIDMapper struct {
	noOpIDMapper
	inviteeExists  bool
	attendeeExists bool
}

func (m participantIDMapper) MapParticipantV2ToInviteeID(_ context.Context, v2UID string) (string, error) {
	if !m.inviteeExists {
		return "", nil
	}
	return v2UID, nil
}
func (m participantIDMapper) MapParticipantV2ToAttendeeID(_ context.Context, v2UID string) (string, error) {
	if !m.attendeeExists {
		return "", nil
	}
	return v2UID, nil
}

func TestPastMeetingParticipantService_CreateParticipant_StampsCreatedBy(t *testing.T) {
	// Ticket flagged that CreateInviteeRequest and CreateAttendeeRequest didn't even
	// have created_by fields. These tests pin down that both are populated from the
	// authenticated principal.

	client := &fakeParticipantClient{}
	reader := &fakeUserMetadataReader{profile: &domain.UserProfile{
		Username: "alice", Name: "Alice", Email: "alice@example.com",
	}}
	svc := NewPastMeetingParticipantService(client, noOpIDMapper{}, reader)

	_, err := svc.CreateParticipant(
		ctxWithPrincipal("alice", ""),
		"pm-1",
		true, true,
		&itx.CreateInviteeRequest{PrimaryEmail: "invitee@example.com"},
		&itx.CreateAttendeeRequest{Email: "invitee@example.com"},
	)
	require.NoError(t, err)

	require.NotNil(t, client.inviteeCreateReq.CreatedBy)
	assert.Equal(t, "alice", client.inviteeCreateReq.CreatedBy.Username)
	assert.Equal(t, "Alice", client.inviteeCreateReq.CreatedBy.Name)

	require.NotNil(t, client.attendeeCreateReq.CreatedBy)
	assert.Equal(t, "alice", client.attendeeCreateReq.CreatedBy.Username)
}

func TestPastMeetingParticipantService_UpdateParticipant_StampsUpdatedByOnExistingRecords(t *testing.T) {
	client := &fakeParticipantClient{}
	reader := &fakeUserMetadataReader{profile: &domain.UserProfile{Username: "bob"}}
	svc := NewPastMeetingParticipantService(
		client,
		participantIDMapper{inviteeExists: true, attendeeExists: true},
		reader,
	)

	trueVal := true
	_, err := svc.UpdateParticipant(
		ctxWithPrincipal("bob", ""),
		&models.UpdatePastMeetingParticipant{
			PastMeetingID: "pm-1",
			ParticipantID: "p-1",
			IsInvited:     &trueVal,
			IsAttended:    &trueVal,
		},
		&itx.UpdateInviteeRequest{FirstName: "Bob", LastName: "Test"},
		&itx.UpdateAttendeeRequest{Org: "Corp"},
	)
	require.NoError(t, err)

	require.NotNil(t, client.inviteeUpdateReq, "invitee update should have been called")
	require.NotNil(t, client.inviteeUpdateReq.UpdatedBy)
	assert.Equal(t, "bob", client.inviteeUpdateReq.UpdatedBy.Username)

	require.NotNil(t, client.attendeeUpdateReq, "attendee update should have been called")
	require.NotNil(t, client.attendeeUpdateReq.UpdatedBy)
	assert.Equal(t, "bob", client.attendeeUpdateReq.UpdatedBy.Username)
}

func TestPastMeetingParticipantService_UpdateParticipant_StampsCreatedByOnMissingRecords(t *testing.T) {
	// Update path: when the underlying invitee / attendee doesn't yet exist, the
	// service creates it. Those creation calls must stamp created_by, not
	// updated_by, since ITX treats them as fresh records.

	client := &fakeParticipantClient{}
	reader := &fakeUserMetadataReader{profile: &domain.UserProfile{Username: "carol"}}
	svc := NewPastMeetingParticipantService(
		client,
		participantIDMapper{inviteeExists: false, attendeeExists: false},
		reader,
	)

	trueVal := true
	_, err := svc.UpdateParticipant(
		ctxWithPrincipal("carol", ""),
		&models.UpdatePastMeetingParticipant{
			PastMeetingID: "pm-1",
			ParticipantID: "p-1",
			IsInvited:     &trueVal,
			IsAttended:    &trueVal,
		},
		&itx.UpdateInviteeRequest{FirstName: "Carol", LastName: "Test", PrimaryEmail: "carol@example.com"},
		&itx.UpdateAttendeeRequest{Org: "Corp"},
	)
	require.NoError(t, err)

	require.NotNil(t, client.inviteeCreateReq, "should fall back to create when invitee doesn't exist")
	require.NotNil(t, client.inviteeCreateReq.CreatedBy)
	assert.Equal(t, "carol", client.inviteeCreateReq.CreatedBy.Username)

	require.NotNil(t, client.attendeeCreateReq, "should fall back to create when attendee doesn't exist")
	require.NotNil(t, client.attendeeCreateReq.CreatedBy)
	assert.Equal(t, "carol", client.attendeeCreateReq.CreatedBy.Username)
}

func TestPastMeetingParticipantService_UpdateParticipant_ResolvesRequesterOnce(t *testing.T) {
	// Regression guard for the pattern where each helper independently called
	// buildRequestingUser. ResolveProfile is a NATS request with a 2s timeout, so
	// firing it twice per update (once for invitee, once for attendee) could add up
	// to 4s of latency and could produce inconsistent stamps if one lookup returned
	// a full profile and the other timed out. UpdateParticipant now resolves the
	// requester once and threads it down.

	assertSingleResolve := func(t *testing.T, mapper participantIDMapper, inviteeReq *itx.UpdateInviteeRequest, attendeeReq *itx.UpdateAttendeeRequest) {
		t.Helper()
		client := &fakeParticipantClient{}
		reader := &fakeUserMetadataReader{profile: &domain.UserProfile{Username: "dana", Name: "Dana"}}
		svc := NewPastMeetingParticipantService(client, mapper, reader)

		trueVal := true
		_, err := svc.UpdateParticipant(
			ctxWithPrincipal("dana", ""),
			&models.UpdatePastMeetingParticipant{
				PastMeetingID: "pm-1",
				ParticipantID: "p-1",
				IsInvited:     &trueVal,
				IsAttended:    &trueVal,
			},
			inviteeReq,
			attendeeReq,
		)
		require.NoError(t, err)
		assert.Equal(t, []string{"dana"}, reader.calls,
			"UpdateParticipant must resolve the requester once and share it across the invitee and attendee paths")
	}

	t.Run("both records exist (double update)", func(t *testing.T) {
		assertSingleResolve(t,
			participantIDMapper{inviteeExists: true, attendeeExists: true},
			&itx.UpdateInviteeRequest{FirstName: "D", LastName: "T"},
			&itx.UpdateAttendeeRequest{Org: "Corp"},
		)
	})

	t.Run("neither record exists (double create-from-update fallback)", func(t *testing.T) {
		assertSingleResolve(t,
			participantIDMapper{inviteeExists: false, attendeeExists: false},
			&itx.UpdateInviteeRequest{FirstName: "D", LastName: "T", PrimaryEmail: "d@example.com"},
			&itx.UpdateAttendeeRequest{Org: "Corp"},
		)
	})

	t.Run("mixed (update invitee, create attendee)", func(t *testing.T) {
		assertSingleResolve(t,
			participantIDMapper{inviteeExists: true, attendeeExists: false},
			&itx.UpdateInviteeRequest{FirstName: "D", LastName: "T"},
			&itx.UpdateAttendeeRequest{Org: "Corp"},
		)
	})
}
