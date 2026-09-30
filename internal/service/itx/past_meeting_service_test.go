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

// fakePastMeetingClient captures the past-meeting requests sent to ITX so tests can
// assert on the outbound created_by / updated_by stamping. storedMeetingID and
// storedProjectID are returned by GetPastMeeting for the reparenting guard tests.
type fakePastMeetingClient struct {
	domain.ITXPastMeetingClient
	lastCreateReq   *itx.CreatePastMeetingRequest
	lastUpdateReq   *itx.CreatePastMeetingRequest
	storedMeetingID string
	storedProjectID string
}

func (f *fakePastMeetingClient) CreatePastMeeting(_ context.Context, req *itx.CreatePastMeetingRequest) (*itx.PastMeetingResponse, error) {
	f.lastCreateReq = req
	return &itx.PastMeetingResponse{}, nil
}

func (f *fakePastMeetingClient) UpdatePastMeeting(_ context.Context, _ string, req *itx.CreatePastMeetingRequest) (*itx.PastMeetingResponse, error) {
	f.lastUpdateReq = req
	return &itx.PastMeetingResponse{}, nil
}

func (f *fakePastMeetingClient) GetPastMeeting(_ context.Context, _ string) (*itx.PastMeetingResponse, error) {
	return &itx.PastMeetingResponse{MeetingID: f.storedMeetingID, ProjectID: f.storedProjectID}, nil
}

// fakePastMeetingMeetingClient returns a canned meeting response for ownership checks
// in past-meeting service tests. project is the v1 project ID the fake meeting reports.
type fakePastMeetingMeetingClient struct {
	domain.ITXMeetingClient
	project    string
	committees []itx.Committee
	getErr     error
}

func (f *fakePastMeetingMeetingClient) GetZoomMeeting(_ context.Context, _ string) (*itx.ZoomMeetingResponse, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	return &itx.ZoomMeetingResponse{Project: f.project, Committees: f.committees}, nil
}

// newPastMeetingSvc builds a PastMeetingService whose fake meeting client reports the
// given project as the owner of every meeting. Uses noOpIDMapper so v2/v1 IDs are equal.
func newPastMeetingSvc(pastClient *fakePastMeetingClient, meetingProject string, reader domain.UserMetadataReader) *PastMeetingService {
	return NewPastMeetingService(pastClient, &fakePastMeetingMeetingClient{project: meetingProject}, noOpIDMapper{}, reader)
}

func TestPastMeetingService_CreatePastMeeting_StampsCreatedBy(t *testing.T) {
	t.Run("stamps full profile on create; leaves updated_by nil", func(t *testing.T) {
		client := &fakePastMeetingClient{}
		reader := &fakeUserMetadataReader{profile: &domain.UserProfile{
			Username: "alice", Name: "Alice", Email: "alice@example.com",
		}}
		svc := newPastMeetingSvc(client, "proj-1", reader)

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
		svc := newPastMeetingSvc(client, "proj-1", nil)
		_, err := svc.CreatePastMeeting(context.Background(), &itx.CreatePastMeetingRequest{
			MeetingID:    "mtg-1",
			OccurrenceID: "1234567890",
			ProjectID:    "proj-1",
		})
		require.NoError(t, err)
		assert.Nil(t, client.lastCreateReq.CreatedBy)
	})
}

func TestPastMeetingService_CreatePastMeeting_OwnershipCheck(t *testing.T) {
	t.Run("allows create when meeting belongs to authorized project", func(t *testing.T) {
		client := &fakePastMeetingClient{}
		svc := newPastMeetingSvc(client, "proj-1", nil)

		_, err := svc.CreatePastMeeting(context.Background(), &itx.CreatePastMeetingRequest{
			MeetingID:    "mtg-1",
			OccurrenceID: "1234567890",
			ProjectID:    "proj-1",
		})
		require.NoError(t, err)
		require.NotNil(t, client.lastCreateReq, "ITX create must be called on an owned meeting")
	})

	t.Run("rejects create when meeting belongs to a different project", func(t *testing.T) {
		client := &fakePastMeetingClient{}
		// meeting client reports project B owns the meeting; request claims project A
		svc := newPastMeetingSvc(client, "proj-victim", nil)

		_, err := svc.CreatePastMeeting(context.Background(), &itx.CreatePastMeetingRequest{
			MeetingID:    "foreign-mtg",
			OccurrenceID: "1234567890",
			ProjectID:    "proj-attacker",
		})
		require.Error(t, err)
		assert.Equal(t, domain.ErrorTypeForbidden, domain.GetErrorType(err))
		assert.Nil(t, client.lastCreateReq, "ITX create must not be called for a foreign meeting")
	})

	t.Run("propagates meeting lookup error", func(t *testing.T) {
		client := &fakePastMeetingClient{}
		lookupErr := errors.New("ITX unavailable")
		svc := NewPastMeetingService(
			client,
			&fakePastMeetingMeetingClient{getErr: lookupErr},
			noOpIDMapper{},
			nil,
		)

		_, err := svc.CreatePastMeeting(context.Background(), &itx.CreatePastMeetingRequest{
			MeetingID:    "mtg-1",
			OccurrenceID: "1234567890",
			ProjectID:    "proj-1",
		})
		require.ErrorIs(t, err, lookupErr)
		assert.Nil(t, client.lastCreateReq)
	})
}

func TestPastMeetingService_CreatePastMeeting_CommitteeOwnershipCheck(t *testing.T) {
	t.Run("allows create when all committees are associated with the meeting", func(t *testing.T) {
		client := &fakePastMeetingClient{}
		meetingClient := &fakePastMeetingMeetingClient{
			project:    "proj-1",
			committees: []itx.Committee{{ID: "committee-1"}, {ID: "committee-2"}},
		}
		svc := NewPastMeetingService(client, meetingClient, noOpIDMapper{}, nil)

		_, err := svc.CreatePastMeeting(context.Background(), &itx.CreatePastMeetingRequest{
			MeetingID:    "mtg-1",
			OccurrenceID: "1234567890",
			ProjectID:    "proj-1",
			Committees:   []itx.Committee{{ID: "committee-1"}, {ID: "committee-2"}},
		})
		require.NoError(t, err)
		require.NotNil(t, client.lastCreateReq)
	})

	t.Run("rejects create when any committee is not associated with the meeting", func(t *testing.T) {
		// The meeting has committee-1; the request includes committee-1 plus a foreign
		// committee-attacker. Even though index 0 matches, the extra entry must be rejected.
		client := &fakePastMeetingClient{}
		meetingClient := &fakePastMeetingMeetingClient{
			project:    "proj-1",
			committees: []itx.Committee{{ID: "committee-1"}},
		}
		svc := NewPastMeetingService(client, meetingClient, noOpIDMapper{}, nil)

		_, err := svc.CreatePastMeeting(context.Background(), &itx.CreatePastMeetingRequest{
			MeetingID:    "mtg-1",
			OccurrenceID: "1234567890",
			ProjectID:    "proj-1",
			Committees:   []itx.Committee{{ID: "committee-1"}, {ID: "committee-attacker"}},
		})
		require.Error(t, err)
		assert.Equal(t, domain.ErrorTypeForbidden, domain.GetErrorType(err))
		assert.Nil(t, client.lastCreateReq)
	})

	t.Run("rejects create when committee is not associated with the meeting (bypass attempt)", func(t *testing.T) {
		// Simulates the committee-path bypass: attacker is writer on committee-A (project A)
		// but sends project_uid = B and meeting_id = a meeting on B that has committee-B.
		// Heimdall passed via committee-A, but the meeting doesn't have committee-A.
		client := &fakePastMeetingClient{}
		meetingClient := &fakePastMeetingMeetingClient{
			project:    "proj-victim",
			committees: []itx.Committee{{ID: "committee-victim"}},
		}
		svc := NewPastMeetingService(client, meetingClient, noOpIDMapper{}, nil)

		_, err := svc.CreatePastMeeting(context.Background(), &itx.CreatePastMeetingRequest{
			MeetingID:    "foreign-mtg",
			OccurrenceID: "1234567890",
			ProjectID:    "proj-victim",
			Committees:   []itx.Committee{{ID: "committee-attacker"}},
		})
		require.Error(t, err)
		assert.Equal(t, domain.ErrorTypeForbidden, domain.GetErrorType(err))
		assert.Nil(t, client.lastCreateReq)
	})
}

// mappingIDMapper translates a single v2 ID to a fixed v1 SFID so tests can verify
// that the project-ownership comparison happens after v2→v1 mapping.
type mappingIDMapper struct {
	noOpIDMapper
	v2ID string
	v1ID string
}

func (m *mappingIDMapper) MapProjectV2ToV1(_ context.Context, v2UID string) (string, error) {
	if v2UID == m.v2ID {
		return m.v1ID, nil
	}
	return v2UID, nil
}

func TestPastMeetingService_CreatePastMeeting_OwnershipCheckAfterMapping(t *testing.T) {
	t.Run("compares project IDs in v1 space after mapper translation", func(t *testing.T) {
		client := &fakePastMeetingClient{}
		// Meeting client speaks v1 SFIDs; the request arrives with a v2 UID.
		meetingClient := &fakePastMeetingMeetingClient{project: "v1-sfid"}
		mapper := &mappingIDMapper{v2ID: "v2-proj", v1ID: "v1-sfid"}
		svc := NewPastMeetingService(client, meetingClient, mapper, nil)

		_, err := svc.CreatePastMeeting(context.Background(), &itx.CreatePastMeetingRequest{
			MeetingID:    "mtg-1",
			OccurrenceID: "1234567890",
			ProjectID:    "v2-proj", // mapped to "v1-sfid" before the ownership check
		})
		require.NoError(t, err)
		require.NotNil(t, client.lastCreateReq)
	})
}

func TestPastMeetingService_UpdatePastMeeting_RejectsMeetingIDChange(t *testing.T) {
	t.Run("returns forbidden when meeting_id differs from stored value", func(t *testing.T) {
		client := &fakePastMeetingClient{storedMeetingID: "original-mtg", storedProjectID: "proj-1"}
		svc := newPastMeetingSvc(client, "proj-1", nil)

		_, err := svc.UpdatePastMeeting(context.Background(), "pm-1", &itx.CreatePastMeetingRequest{
			MeetingID: "different-mtg",
			ProjectID: "proj-1",
		})
		require.Error(t, err)
		assert.Equal(t, domain.ErrorTypeForbidden, domain.GetErrorType(err))
		assert.Nil(t, client.lastUpdateReq, "ITX update must not be called when meeting_id changes")
	})

	t.Run("allows update when meeting_id echoes the stored value", func(t *testing.T) {
		client := &fakePastMeetingClient{storedMeetingID: "original-mtg", storedProjectID: "proj-1"}
		svc := newPastMeetingSvc(client, "proj-1", nil)

		_, err := svc.UpdatePastMeeting(context.Background(), "pm-1", &itx.CreatePastMeetingRequest{
			MeetingID: "original-mtg", // same as stored — full-object echo is safe
			ProjectID: "proj-1",
		})
		require.NoError(t, err)
		require.NotNil(t, client.lastUpdateReq)
	})

	t.Run("allows update when meeting_id is absent", func(t *testing.T) {
		client := &fakePastMeetingClient{storedMeetingID: "original-mtg", storedProjectID: "proj-1"}
		svc := newPastMeetingSvc(client, "proj-1", nil)

		_, err := svc.UpdatePastMeeting(context.Background(), "pm-1", &itx.CreatePastMeetingRequest{
			ProjectID: "proj-1",
		})
		require.NoError(t, err)
		require.NotNil(t, client.lastUpdateReq)
	})
}

func TestPastMeetingService_UpdatePastMeeting_RejectsProjectUIDChange(t *testing.T) {
	t.Run("returns forbidden when project_uid differs from stored value", func(t *testing.T) {
		client := &fakePastMeetingClient{storedProjectID: "proj-original"}
		svc := newPastMeetingSvc(client, "proj-original", nil)

		_, err := svc.UpdatePastMeeting(context.Background(), "pm-1", &itx.CreatePastMeetingRequest{
			ProjectID: "proj-attacker",
		})
		require.Error(t, err)
		assert.Equal(t, domain.ErrorTypeForbidden, domain.GetErrorType(err))
		assert.Nil(t, client.lastUpdateReq, "ITX update must not be called when project_uid changes")
	})

	t.Run("allows update when project_uid echoes the stored value", func(t *testing.T) {
		client := &fakePastMeetingClient{storedProjectID: "proj-1"}
		svc := newPastMeetingSvc(client, "proj-1", nil)

		_, err := svc.UpdatePastMeeting(context.Background(), "pm-1", &itx.CreatePastMeetingRequest{
			ProjectID: "proj-1", // same as stored — full-object echo is safe
		})
		require.NoError(t, err)
		require.NotNil(t, client.lastUpdateReq)
	})

	t.Run("allows update when project_uid is absent", func(t *testing.T) {
		client := &fakePastMeetingClient{storedProjectID: "proj-1"}
		svc := newPastMeetingSvc(client, "proj-1", nil)

		_, err := svc.UpdatePastMeeting(context.Background(), "pm-1", &itx.CreatePastMeetingRequest{})
		require.NoError(t, err)
		require.NotNil(t, client.lastUpdateReq)
	})
}

func TestPastMeetingService_UpdatePastMeeting_StampsUpdatedByNotCreatedBy(t *testing.T) {
	t.Run("stamps only updated_by on update", func(t *testing.T) {
		client := &fakePastMeetingClient{storedProjectID: "proj-1"}
		reader := &fakeUserMetadataReader{profile: &domain.UserProfile{Username: "bob"}}
		svc := newPastMeetingSvc(client, "proj-1", reader)

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
