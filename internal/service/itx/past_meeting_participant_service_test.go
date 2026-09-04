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

func TestMergeParticipantResponses_CarriesAttendeeReconciliationFields(t *testing.T) {
	attendee := &itx.AttendeeResponse{
		ID:                "attendee-1",
		IsVerified:        true,
		IsUnknown:         false,
		IsAIReconciled:    true,
		IsAutoMatched:     true,
		ZoomUserName:      "Alice Example (Zoom)",
		MappedInviteeName: "Alice Example",
	}

	unified := mergeParticipantResponses("mtg-1-occ-1", nil, attendee, false, true)

	assert.True(t, unified.IsVerified)
	assert.True(t, unified.IsAIReconciled)
	assert.True(t, unified.IsAutoMatched)
	assert.Equal(t, "Alice Example (Zoom)", unified.ZoomUserName)
	assert.Equal(t, "Alice Example", unified.MappedInviteeName)
}

func TestPastMeetingParticipantService_UpdateParticipant_CarriesReconciliationFieldsOnAttendeeCreate(t *testing.T) {
	// When an update targets an attendee that doesn't exist yet, the service falls
	// back to creating one. Reconciliation fields set on the update request must
	// survive that create fallback, not just a straight update.

	client := &fakeParticipantClient{}
	reader := &fakeUserMetadataReader{profile: &domain.UserProfile{Username: "erin"}}
	svc := NewPastMeetingParticipantService(
		client,
		participantIDMapper{inviteeExists: true, attendeeExists: false},
		reader,
	)

	trueVal := true
	zoomUserName := "Erin (Zoom)"
	mappedInviteeName := "Erin Test"
	_, err := svc.UpdateParticipant(
		ctxWithPrincipal("erin", ""),
		&models.UpdatePastMeetingParticipant{
			PastMeetingID: "pm-1",
			ParticipantID: "p-1",
			IsInvited:     &trueVal,
			IsAttended:    &trueVal,
		},
		&itx.UpdateInviteeRequest{FirstName: "Erin", LastName: "Test"},
		&itx.UpdateAttendeeRequest{
			IsAIReconciled:    &trueVal,
			IsAutoMatched:     &trueVal,
			ZoomUserName:      &zoomUserName,
			MappedInviteeName: &mappedInviteeName,
		},
	)
	require.NoError(t, err)

	require.NotNil(t, client.attendeeCreateReq, "should fall back to create when attendee doesn't exist")
	assert.True(t, client.attendeeCreateReq.IsAIReconciled)
	assert.True(t, client.attendeeCreateReq.IsAutoMatched)
	assert.Equal(t, "Erin (Zoom)", client.attendeeCreateReq.ZoomUserName)
	assert.Equal(t, "Erin Test", client.attendeeCreateReq.MappedInviteeName)
}

func TestPastMeetingParticipantService_UpdateParticipant_CarriesIdentityFieldsOnAttendeeCreate(t *testing.T) {
	// Identity fields set on the update request (attaching a matched identity to
	// an attendee that doesn't exist yet) must survive the create fallback.

	client := &fakeParticipantClient{}
	reader := &fakeUserMetadataReader{profile: &domain.UserProfile{Username: "iris"}}
	svc := NewPastMeetingParticipantService(
		client,
		participantIDMapper{inviteeExists: true, attendeeExists: false},
		reader,
	)

	trueVal := true
	falseVal := false
	_, err := svc.UpdateParticipant(
		ctxWithPrincipal("iris", ""),
		&models.UpdatePastMeetingParticipant{
			PastMeetingID: "pm-1",
			ParticipantID: "p-1",
			IsInvited:     &trueVal,
			IsAttended:    &trueVal,
		},
		&itx.UpdateInviteeRequest{FirstName: "Iris", LastName: "Test"},
		&itx.UpdateAttendeeRequest{
			Name:      "Iris Test",
			Email:     "iris@example.com",
			LFSSO:     "iris",
			LFUserID:  "sf-002",
			IsUnknown: &falseVal,
		},
	)
	require.NoError(t, err)

	require.NotNil(t, client.attendeeCreateReq, "should fall back to create when attendee doesn't exist")
	assert.Equal(t, "Iris Test", client.attendeeCreateReq.Name)
	assert.Equal(t, "iris@example.com", client.attendeeCreateReq.Email)
	assert.Equal(t, "iris", client.attendeeCreateReq.LFSSO)
	assert.Equal(t, "sf-002", client.attendeeCreateReq.LFUserID)
	assert.False(t, client.attendeeCreateReq.IsUnknown)
}

func TestPastMeetingParticipantService_UpdateParticipant_EchoesIdentityFieldsOn204(t *testing.T) {
	// ITX returns 204 No Content on a successful attendee update, so the ITX client
	// returns (nil, nil). The synchronous API response must still reflect the
	// identity fields that were just persisted (e.g. a "Confirm Match" action)
	// instead of silently reporting zero values.

	client := &fake204ParticipantClient{}
	reader := &fakeUserMetadataReader{profile: &domain.UserProfile{Username: "jack"}}
	svc := NewPastMeetingParticipantService(
		client,
		participantIDMapper{inviteeExists: false, attendeeExists: true},
		reader,
	)

	trueVal := true
	falseVal := false
	resp, err := svc.UpdateParticipant(
		ctxWithPrincipal("jack", ""),
		&models.UpdatePastMeetingParticipant{
			PastMeetingID: "pm-1",
			ParticipantID: "p-1",
			IsAttended:    &trueVal,
		},
		nil,
		&itx.UpdateAttendeeRequest{
			Name:      "Jack Test",
			Email:     "jack@example.com",
			LFSSO:     "jack",
			LFUserID:  "sf-003",
			IsUnknown: &falseVal,
		},
	)
	require.NoError(t, err)
	require.NotNil(t, resp)

	assert.Equal(t, "Jack Test", resp.FirstName)
	assert.Equal(t, "jack@example.com", resp.Email)
	assert.Equal(t, "jack", resp.Username)
	assert.Equal(t, "sf-003", resp.LFUserID)
	assert.False(t, resp.IsUnknown)
}

func TestPastMeetingParticipantService_UpdateParticipant_RefetchesIsUnknownOn204(t *testing.T) {
	// Regression test: an identity-only update that omits is_unknown must not
	// falsely collapse a previously-unknown attendee's is_unknown flag to false.
	// The 204 fallback must refetch ground truth via GetAttendee rather than
	// echoing the request, since a nil IsUnknown pointer means "don't change it",
	// not "set it to false".

	client := &fake204ParticipantClient{
		getAttendeeResp: &itx.AttendeeResponse{
			ID:        "attendee-1",
			Name:      "Karen Test",
			Email:     "karen@example.com",
			IsUnknown: true,
		},
	}
	reader := &fakeUserMetadataReader{profile: &domain.UserProfile{Username: "karen"}}
	svc := NewPastMeetingParticipantService(
		client,
		participantIDMapper{inviteeExists: false, attendeeExists: true},
		reader,
	)

	trueVal := true
	resp, err := svc.UpdateParticipant(
		ctxWithPrincipal("karen", ""),
		&models.UpdatePastMeetingParticipant{
			PastMeetingID: "pm-1",
			ParticipantID: "p-1",
			IsAttended:    &trueVal,
		},
		nil,
		&itx.UpdateAttendeeRequest{
			Name:  "Karen Test",
			Email: "karen@example.com",
			// IsUnknown intentionally omitted (nil): identity-only update.
		},
	)
	require.NoError(t, err)
	require.NotNil(t, resp)

	assert.True(t, resp.IsUnknown, "must reflect the persisted is_unknown value from the refetch, not falsely collapse the omitted field to false")
}

func TestPastMeetingParticipantService_UpdateParticipant_ReconciliationOnlyDoesNotCreateAttendee(t *testing.T) {
	// A reconciliation-only update (isAttended == nil, i.e. no attendance status
	// change requested) against a participant with no existing attendee record
	// must not implicitly create one - there's nothing to reconcile yet.

	client := &fakeParticipantClient{}
	reader := &fakeUserMetadataReader{profile: &domain.UserProfile{Username: "frank"}}
	svc := NewPastMeetingParticipantService(
		client,
		participantIDMapper{inviteeExists: true, attendeeExists: false},
		reader,
	)

	trueVal := true
	_, err := svc.UpdateParticipant(
		ctxWithPrincipal("frank", ""),
		&models.UpdatePastMeetingParticipant{
			PastMeetingID: "pm-1",
			ParticipantID: "p-1",
			IsInvited:     &trueVal,
			// IsAttended intentionally omitted (nil): reconciliation-only update.
		},
		&itx.UpdateInviteeRequest{FirstName: "Frank", LastName: "Test"},
		&itx.UpdateAttendeeRequest{IsAIReconciled: &trueVal},
	)
	require.NoError(t, err)

	assert.Nil(t, client.attendeeCreateReq, "should not create an attendee for a reconciliation-only update")
	assert.Nil(t, client.attendeeUpdateReq, "should not update a non-existent attendee either")
}

// failingAttendeeUpdateClient simulates ITX rejecting an attendee update (e.g. a
// 4xx/5xx response) instead of the 204-no-content success case.
type failingAttendeeUpdateClient struct {
	fakeParticipantClient
}

func (f *failingAttendeeUpdateClient) UpdateAttendee(_ context.Context, _, _ string, _ *itx.UpdateAttendeeRequest) (*itx.AttendeeResponse, error) {
	return nil, errors.New("itx: attendee update rejected")
}

func TestPastMeetingParticipantService_UpdateParticipant_PropagatesAttendeeUpdateError(t *testing.T) {
	// If ITX fails to persist the attendee update, UpdateParticipant must
	// return an error instead of silently reporting a fake-successful response.

	client := &failingAttendeeUpdateClient{}
	reader := &fakeUserMetadataReader{profile: &domain.UserProfile{Username: "henry"}}
	svc := NewPastMeetingParticipantService(
		client,
		participantIDMapper{inviteeExists: false, attendeeExists: true},
		reader,
	)

	trueVal := true
	resp, err := svc.UpdateParticipant(
		ctxWithPrincipal("henry", ""),
		&models.UpdatePastMeetingParticipant{
			PastMeetingID: "pm-1",
			ParticipantID: "p-1",
			IsAttended:    &trueVal,
		},
		nil,
		&itx.UpdateAttendeeRequest{IsAIReconciled: &trueVal},
	)

	require.Error(t, err)
	assert.Nil(t, resp)
}

// failingInviteeUpdateClient simulates ITX rejecting an invitee update. It also
// tracks whether the attendee side was ever invoked, so tests can assert that
// UpdateParticipant aborts before issuing the attendee operation.
type failingInviteeUpdateClient struct {
	fakeParticipantClient
}

func (f *failingInviteeUpdateClient) UpdateInvitee(_ context.Context, _, _ string, _ *itx.UpdateInviteeRequest) (*itx.InviteeResponse, error) {
	return nil, errors.New("itx: invitee update rejected")
}

func TestPastMeetingParticipantService_UpdateParticipant_AbortsAttendeeOperationOnInviteeError(t *testing.T) {
	// If the invitee operation fails, UpdateParticipant must return before
	// attempting the attendee operation at all - running it anyway would
	// persist partial state and encourage a retry of an already-applied write.

	client := &failingInviteeUpdateClient{}
	reader := &fakeUserMetadataReader{profile: &domain.UserProfile{Username: "jack"}}
	svc := NewPastMeetingParticipantService(
		client,
		participantIDMapper{inviteeExists: true, attendeeExists: false},
		reader,
	)

	trueVal := true
	resp, err := svc.UpdateParticipant(
		ctxWithPrincipal("jack", ""),
		&models.UpdatePastMeetingParticipant{
			PastMeetingID: "pm-1",
			ParticipantID: "p-1",
			IsInvited:     &trueVal,
			IsAttended:    &trueVal,
		},
		&itx.UpdateInviteeRequest{FirstName: "Jack", LastName: "Test"},
		&itx.UpdateAttendeeRequest{IsAIReconciled: &trueVal},
	)

	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Nil(t, client.attendeeCreateReq, "attendee operation must not run once the invitee operation has failed")
	assert.Nil(t, client.attendeeUpdateReq, "attendee operation must not run once the invitee operation has failed")
}

// failingAttendeeDeleteClient simulates ITX rejecting an attendee delete.
type failingAttendeeDeleteClient struct {
	fakeParticipantClient
}

func (f *failingAttendeeDeleteClient) DeleteAttendee(_ context.Context, _, _ string) error {
	return errors.New("itx: attendee delete rejected")
}

func TestPastMeetingParticipantService_UpdateParticipant_PropagatesAttendeeDeleteError(t *testing.T) {
	// If ITX fails to delete an attendee record on isAttended=false, the record
	// is still present in ITX and the response must not claim it was removed.

	client := &failingAttendeeDeleteClient{}
	reader := &fakeUserMetadataReader{profile: &domain.UserProfile{Username: "ivy"}}
	svc := NewPastMeetingParticipantService(
		client,
		participantIDMapper{attendeeExists: true},
		reader,
	)

	falseVal := false
	resp, err := svc.UpdateParticipant(
		ctxWithPrincipal("ivy", ""),
		&models.UpdatePastMeetingParticipant{
			PastMeetingID: "pm-1",
			ParticipantID: "p-1",
			IsAttended:    &falseVal,
		},
		nil,
		nil,
	)

	require.Error(t, err)
	assert.Nil(t, resp)
}

// mappingErrorIDMapper simulates a transient ID-mapper failure (e.g. NATS
// timeout) that must be distinguished from a genuine "not found".
type mappingErrorIDMapper struct {
	noOpIDMapper
}

func (m mappingErrorIDMapper) MapParticipantV2ToAttendeeID(_ context.Context, _ string) (string, error) {
	return "", domain.NewUnavailableError("v1-sync-helper lookup timed out")
}

func TestPastMeetingParticipantService_UpdateParticipant_PropagatesAttendeeMappingError(t *testing.T) {
	// A reconciliation-only update (isAttended == nil) must not silently succeed
	// when the ID mapper fails transiently - that failure must not be
	// indistinguishable from "attendee genuinely doesn't exist yet".

	client := &fakeParticipantClient{}
	reader := &fakeUserMetadataReader{profile: &domain.UserProfile{Username: "jack"}}
	svc := NewPastMeetingParticipantService(
		client,
		mappingErrorIDMapper{},
		reader,
	)

	trueVal := true
	resp, err := svc.UpdateParticipant(
		ctxWithPrincipal("jack", ""),
		&models.UpdatePastMeetingParticipant{
			PastMeetingID: "pm-1",
			ParticipantID: "p-1",
		},
		nil,
		&itx.UpdateAttendeeRequest{IsAIReconciled: &trueVal},
	)

	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Nil(t, client.attendeeCreateReq, "must not create an attendee off a mapping error")
	assert.Nil(t, client.attendeeUpdateReq, "must not update off a mapping error")
}

// reverseMappingErrorIDMapper simulates a transient failure on the reverse
// (attendee ID -> participant ID) lookup used when the caller already knows
// the attendee ID.
type reverseMappingErrorIDMapper struct {
	noOpIDMapper
}

func (m reverseMappingErrorIDMapper) MapAttendeeIDToParticipantV2(_ context.Context, _ string) (string, error) {
	return "", domain.NewUnavailableError("v1-sync-helper reverse lookup timed out")
}

func TestPastMeetingParticipantService_UpdateParticipant_PropagatesAttendeeReverseMappingError(t *testing.T) {
	// A reconciliation-only update (isAttended == nil) supplying an already-known
	// attendee ID must not silently succeed when the reverse ID-mapper lookup
	// fails transiently - collapsing that failure to "does not exist" would drop
	// the write entirely since neither the create nor delete branch fires when
	// isAttended is nil.

	client := &fakeParticipantClient{}
	reader := &fakeUserMetadataReader{profile: &domain.UserProfile{Username: "kate"}}
	svc := NewPastMeetingParticipantService(
		client,
		reverseMappingErrorIDMapper{},
		reader,
	)

	trueVal := true
	resp, err := svc.UpdateParticipant(
		ctxWithPrincipal("kate", ""),
		&models.UpdatePastMeetingParticipant{
			PastMeetingID: "pm-1",
			ParticipantID: "p-1",
			AttendeeID:    "att-1",
		},
		nil,
		&itx.UpdateAttendeeRequest{IsAIReconciled: &trueVal},
	)

	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Nil(t, client.attendeeCreateReq, "must not create an attendee off a reverse mapping error")
	assert.Nil(t, client.attendeeUpdateReq, "must not update off a reverse mapping error")
}

func TestMergeParticipantResponses_OmitsAttendeeFieldsWhenInviteeOnly(t *testing.T) {
	invitee := &itx.InviteeResponse{UUID: "invitee-1", FirstName: "Bob"}

	unified := mergeParticipantResponses("mtg-1-occ-1", invitee, nil, true, false)

	assert.False(t, unified.IsAIReconciled)
	assert.False(t, unified.IsAutoMatched)
	assert.Empty(t, unified.ZoomUserName)
	assert.Empty(t, unified.MappedInviteeName)
}

// fake204ParticipantClient simulates ITX's real behavior on an attendee update:
// a 204 No Content response, surfaced here as (nil, nil). By default GetAttendee
// is unconfigured and returns an error, exercising the graceful-degradation
// fallback; set getAttendeeResp to simulate a successful refetch instead.
type fake204ParticipantClient struct {
	fakeParticipantClient

	getAttendeeResp *itx.AttendeeResponse
	getAttendeeErr  error
}

func (f *fake204ParticipantClient) UpdateAttendee(_ context.Context, _, _ string, req *itx.UpdateAttendeeRequest) (*itx.AttendeeResponse, error) {
	f.attendeeUpdateReq = req
	return nil, nil
}

func (f *fake204ParticipantClient) GetAttendee(_ context.Context, _, _ string) (*itx.AttendeeResponse, error) {
	if f.getAttendeeResp != nil {
		return f.getAttendeeResp, nil
	}
	if f.getAttendeeErr != nil {
		return nil, f.getAttendeeErr
	}
	return nil, errors.New("fake204ParticipantClient: GetAttendee not configured")
}

func TestPastMeetingParticipantService_UpdateParticipant_EchoesReconciliationFieldsOn204(t *testing.T) {
	// ITX returns 204 No Content on a successful attendee update, so the ITX client
	// returns (nil, nil). The synchronous API response must still reflect the
	// values that were just persisted instead of silently reporting zero values.

	client := &fake204ParticipantClient{}
	reader := &fakeUserMetadataReader{profile: &domain.UserProfile{Username: "gina"}}
	svc := NewPastMeetingParticipantService(
		client,
		participantIDMapper{inviteeExists: false, attendeeExists: true},
		reader,
	)

	trueVal := true
	zoomUserName := "Gina (Zoom)"
	mappedInviteeName := "Gina Test"
	resp, err := svc.UpdateParticipant(
		ctxWithPrincipal("gina", ""),
		&models.UpdatePastMeetingParticipant{
			PastMeetingID: "pm-1",
			ParticipantID: "p-1",
			IsAttended:    &trueVal,
		},
		nil,
		&itx.UpdateAttendeeRequest{
			IsAIReconciled:    &trueVal,
			IsAutoMatched:     &trueVal,
			ZoomUserName:      &zoomUserName,
			MappedInviteeName: &mappedInviteeName,
		},
	)
	require.NoError(t, err)
	require.NotNil(t, resp)

	assert.True(t, resp.IsAIReconciled)
	assert.True(t, resp.IsAutoMatched)
	assert.Equal(t, "Gina (Zoom)", resp.ZoomUserName)
	assert.Equal(t, "Gina Test", resp.MappedInviteeName)
}

func TestPastMeetingParticipantService_UpdateParticipant_ReconciliationOnlyUpdateWithoutIsAttended(t *testing.T) {
	// A reconciliation-only update (e.g. re-running AI matching) carries no
	// is_attended flag at all. It must still reach ITX rather than being silently
	// dropped by the isAttended==nil early return.

	client := &fakeParticipantClient{}
	reader := &fakeUserMetadataReader{profile: &domain.UserProfile{Username: "hank"}}
	svc := NewPastMeetingParticipantService(
		client,
		participantIDMapper{inviteeExists: false, attendeeExists: true},
		reader,
	)

	trueVal := true
	_, err := svc.UpdateParticipant(
		ctxWithPrincipal("hank", ""),
		&models.UpdatePastMeetingParticipant{
			PastMeetingID: "pm-1",
			ParticipantID: "p-1",
		},
		nil,
		&itx.UpdateAttendeeRequest{IsAIReconciled: &trueVal},
	)
	require.NoError(t, err)

	require.NotNil(t, client.attendeeUpdateReq, "reconciliation-only update must still call ITX")
	require.NotNil(t, client.attendeeUpdateReq.IsAIReconciled)
	assert.True(t, *client.attendeeUpdateReq.IsAIReconciled)
}
