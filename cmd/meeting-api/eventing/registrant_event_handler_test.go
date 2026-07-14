// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"
	"time"

	inviteapi "github.com/linuxfoundation/lfx-v2-invite-service/pkg/api"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	meetingconstants "github.com/linuxfoundation/lfx-v2-meeting-service/pkg/constants"
)

// stubV1UserLookup is a no-op lookup used in tests that do not exercise user enrichment.
type stubV1UserLookup struct{}

func (stubV1UserLookup) LookupUser(_ context.Context, _ string) (*domain.V1User, error) {
	return nil, nil
}

// stubIDMapper is a no-op mapper for tests that don't involve committee mapping.
type stubIDMapper struct{}

func (stubIDMapper) MapProjectV2ToV1(_ context.Context, _ string) (string, error)   { return "", nil }
func (stubIDMapper) MapProjectV1ToV2(_ context.Context, _ string) (string, error)   { return "", nil }
func (stubIDMapper) MapCommitteeV2ToV1(_ context.Context, _ string) (string, error) { return "", nil }
func (stubIDMapper) MapCommitteeV1ToV2(_ context.Context, _ string) (string, error) { return "", nil }
func (stubIDMapper) MapInviteeIDToParticipantV2(_ context.Context, _ string) (string, error) {
	return "", nil
}
func (stubIDMapper) MapAttendeeIDToParticipantV2(_ context.Context, _ string) (string, error) {
	return "", nil
}
func (stubIDMapper) MapParticipantV2ToInviteeID(_ context.Context, _ string) (string, error) {
	return "", nil
}
func (stubIDMapper) MapParticipantV2ToAttendeeID(_ context.Context, _ string) (string, error) {
	return "", nil
}

// mockEventPublisher is a testify mock for domain.EventPublisher.
type mockEventPublisher struct{ mock.Mock }

func (m *mockEventPublisher) PublishMeetingEvent(_ context.Context, _ string, _ *models.MeetingEventData) error {
	return nil
}
func (m *mockEventPublisher) PublishRegistrantEvent(ctx context.Context, action string, r *models.RegistrantEventData) error {
	return m.Called(ctx, action, r).Error(0)
}
func (m *mockEventPublisher) PublishInviteResponseEvent(_ context.Context, _ string, _ *models.InviteResponseEventData) error {
	return nil
}
func (m *mockEventPublisher) PublishPastMeetingEvent(_ context.Context, _ string, _ *models.PastMeetingEventData) error {
	return nil
}
func (m *mockEventPublisher) PublishPastMeetingParticipantEvent(_ context.Context, _ string, _ *models.PastMeetingParticipantEventData) error {
	return nil
}
func (m *mockEventPublisher) PublishPastMeetingRecordingEvent(_ context.Context, _ string, _ *models.RecordingEventData) error {
	return nil
}
func (m *mockEventPublisher) PublishPastMeetingTranscriptEvent(_ context.Context, _ string, _ *models.TranscriptEventData) error {
	return nil
}
func (m *mockEventPublisher) PublishPastMeetingSummaryEvent(_ context.Context, _ string, _ *models.SummaryEventData, _ string) error {
	return nil
}
func (m *mockEventPublisher) PublishMeetingAttachmentEvent(_ context.Context, _ string, _ *models.MeetingAttachmentEventData) error {
	return nil
}
func (m *mockEventPublisher) PublishPastMeetingAttachmentEvent(_ context.Context, _ string, _ *models.PastMeetingAttachmentEventData) error {
	return nil
}
func (m *mockEventPublisher) PublishIndexerDelete(_ context.Context, _, _ string) error { return nil }
func (m *mockEventPublisher) PublishAccessDelete(ctx context.Context, subject string, payload []byte) error {
	return m.Called(ctx, subject, payload).Error(0)
}
func (m *mockEventPublisher) Close() error { return nil }

type stubUserReader struct {
	username string
	err      error
}

func (s stubUserReader) UsernameByEmail(_ context.Context, _ string) (string, error) {
	return s.username, s.err
}

type stubInviteSender struct {
	result *domain.InviteResult
	err    error
	called bool
	last   inviteapi.SendInviteRequest
}

func (s *stubInviteSender) SendInvite(_ context.Context, req inviteapi.SendInviteRequest) (*domain.InviteResult, error) {
	s.called = true
	s.last = req
	if s.err != nil {
		return nil, s.err
	}
	return s.result, nil
}

type stubAcceptanceClient struct {
	called bool
	email  string
	user   string
	err    error
}

func (s *stubAcceptanceClient) AcceptInvite(_ context.Context, email, username string) error {
	s.called = true
	s.email = email
	s.user = username
	return s.err
}

func TestMaybeSendInvite(t *testing.T) {
	const (
		registrantUID = "reg-123"
		meetingID     = "meeting-456"
		email         = "guest@example.com"
	)

	meetingKey := "itx-zoom-meetings-v2." + meetingID
	meetingPayload, err := json.Marshal(map[string]any{
		"topic":    "Weekly Sync",
		"password": "secret",
	})
	require.NoError(t, err)

	inviteSentKey := registrantLFIDInviteSentKey(registrantUID)

	tests := []struct {
		name         string
		userReader   stubUserReader
		setupObjects func(*mockKeyValue)
		setupMaps    func(*mockKeyValue)
		wantCalled   bool
		wantRole     string
	}{
		{
			name: "skips when invite already sent marker exists",
			setupMaps: func(kv *mockKeyValue) {
				kv.On("Get", mock.Anything, inviteSentKey).
					Return(mockKeyValueEntry{key: inviteSentKey, value: []byte("invite-old")}, nil)
			},
		},
		{
			name:       "skips when user already has LFID",
			userReader: stubUserReader{username: "existing"},
			setupMaps: func(kv *mockKeyValue) {
				kv.On("Get", mock.Anything, inviteSentKey).Return(nil, jetstream.ErrKeyNotFound)
			},
		},
		{
			name:       "skips on transient auth lookup failure",
			userReader: stubUserReader{err: errors.New("auth unavailable")},
			setupMaps: func(kv *mockKeyValue) {
				kv.On("Get", mock.Anything, inviteSentKey).Return(nil, jetstream.ErrKeyNotFound)
			},
		},
		{
			name:       "skips on transient sent-marker lookup failure",
			userReader: stubUserReader{err: domain.ErrUserNotFound},
			setupMaps: func(kv *mockKeyValue) {
				kv.On("Get", mock.Anything, inviteSentKey).Return(nil, errors.New("kv unavailable"))
			},
		},
		{
			name:       "skips when meeting title cannot be resolved",
			userReader: stubUserReader{err: domain.ErrUserNotFound},
			setupMaps: func(kv *mockKeyValue) {
				kv.On("Get", mock.Anything, inviteSentKey).Return(nil, jetstream.ErrKeyNotFound)
			},
			setupObjects: func(kv *mockKeyValue) {
				kv.On("Get", mock.Anything, meetingKey).Return(nil, jetstream.ErrKeyNotFound)
			},
		},
		{
			name:       "sends invite and stores sent marker on success",
			userReader: stubUserReader{err: domain.ErrUserNotFound},
			setupMaps: func(kv *mockKeyValue) {
				kv.On("Get", mock.Anything, inviteSentKey).Return(nil, jetstream.ErrKeyNotFound)
				kv.On("Put", mock.Anything, inviteSentKey, []byte("invite-new")).Return(uint64(1), nil)
			},
			setupObjects: func(kv *mockKeyValue) {
				kv.On("Get", mock.Anything, meetingKey).
					Return(mockKeyValueEntry{key: meetingKey, value: meetingPayload}, nil)
			},
			wantCalled: true,
			wantRole:   meetingconstants.InviteRoleRegistrant,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objectsKV := &mockKeyValue{}
			mappingsKV := &mockKeyValue{}
			if tt.setupObjects != nil {
				tt.setupObjects(objectsKV)
			}
			if tt.setupMaps != nil {
				tt.setupMaps(mappingsKV)
			}

			sender := &stubInviteSender{
				result: &domain.InviteResult{
					InviteUID:      "invite-new",
					RecipientEmail: email,
					ExpiresAt:      time.Now().Add(24 * time.Hour),
				},
			}

			h := &EventHandlers{
				v1ObjectsKV:      objectsKV,
				v1MappingsKV:     mappingsKV,
				userReader:       tt.userReader,
				inviteSender:     sender,
				selfServeBaseURL: "https://app.dev.lfx.dev",
				logger:           slog.Default(),
			}

			h.maybeSendInvite(context.Background(), slog.Default(), registrantUID, email, "Guest", meetingID, models.CreatedBy{Name: "Host"})

			assert.Equal(t, tt.wantCalled, sender.called)
			if tt.wantCalled {
				assert.Equal(t, tt.wantRole, sender.last.Role)
				assert.Equal(t, meetingID, sender.last.Resource.UID)
			}

			objectsKV.AssertExpectations(t)
			mappingsKV.AssertExpectations(t)
		})
	}
}

func TestProcessInviteAcceptedEvent(t *testing.T) {
	client := &stubAcceptanceClient{}
	evt := inviteapi.InviteServiceAcceptedEvent{
		Invite: inviteapi.Invite{
			Recipient:  inviteapi.Recipient{Email: "guest@example.com"},
			AcceptedBy: "auth0|guest",
			Resource:   inviteapi.Resource{Type: meetingconstants.ResourceTypeMeeting},
		},
	}

	err := processInviteAcceptedEvent(context.Background(), evt, client, slog.Default())
	require.NoError(t, err)
	assert.True(t, client.called)
	assert.Equal(t, "guest@example.com", client.email)
	assert.Equal(t, "auth0|guest", client.user)
}

func TestProcessInviteAcceptedEvent_missingFields(t *testing.T) {
	client := &stubAcceptanceClient{}
	evt := inviteapi.InviteServiceAcceptedEvent{
		Invite: inviteapi.Invite{AcceptedBy: "auth0|guest"},
	}

	err := processInviteAcceptedEvent(context.Background(), evt, client, slog.Default())
	require.NoError(t, err)
	assert.False(t, client.called)
}

func TestProcessInviteAcceptedEvent_clientError(t *testing.T) {
	client := &stubAcceptanceClient{err: errors.New("itx unavailable")}
	evt := inviteapi.InviteServiceAcceptedEvent{
		Invite: inviteapi.Invite{
			Recipient:  inviteapi.Recipient{Email: "guest@example.com"},
			AcceptedBy: "auth0|guest",
		},
	}

	err := processInviteAcceptedEvent(context.Background(), evt, client, slog.Default())
	require.Error(t, err)
}

// TestHandleRegistrantUpdate_SparseUpdate_NoSpuriousRevocation verifies that a sparse CDC
// update payload — one that omits the "username" key entirely — does not trigger a
// member_remove even when the stored mapping contains an existing username.
// This guards against the bug where utils.GetString(v1Data["username"]) returns ""
// for both an absent key (sparse update) and an explicit clear, making them
// indistinguishable without the key-presence check.
func TestHandleRegistrantUpdate_SparseUpdate_NoSpuriousRevocation(t *testing.T) {
	const (
		registrantUID = "reg-1"
		meetingID     = "meeting-1"
		oldUsername   = "alice"
	)

	storedMapping := buildRegistrantMappingValue(registrantUID, oldUsername, meetingID)

	tests := []struct {
		name                  string
		v1Data                map[string]interface{}
		wantAccessDeleteCalls int
	}{
		{
			name: "sparse update (username key absent) does not revoke",
			v1Data: map[string]interface{}{
				"registrant_id": registrantUID,
				"meeting_id":    meetingID,
				// "username" key intentionally absent — simulates sparse CDC payload
			},
			wantAccessDeleteCalls: 0,
		},
		{
			name: "explicit clear (username key present and empty) revokes",
			v1Data: map[string]interface{}{
				"registrant_id": registrantUID,
				"meeting_id":    meetingID,
				"username":      "", // explicit clear — must revoke
			},
			wantAccessDeleteCalls: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mappingsKV := &mockKeyValue{}
			publisher := &mockEventPublisher{}

			// Parent meeting exists
			mappingsKV.On("Get", mock.Anything, "v1_meetings."+meetingID).
				Return(mockKeyValueEntry{key: "v1_meetings." + meetingID, value: []byte("1")}, nil)

			// Existing registrant mapping with a username
			mappingsKV.On("Get", mock.Anything, "v1_meeting_registrants."+registrantUID).
				Return(mockKeyValueEntry{key: "v1_meeting_registrants." + registrantUID, value: []byte(storedMapping)}, nil)

			// Mapping write after update
			mappingsKV.On("Put", mock.Anything, "v1_meeting_registrants."+registrantUID, mock.Anything).
				Return(uint64(2), nil)

			publisher.On("PublishRegistrantEvent", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			if tt.wantAccessDeleteCalls > 0 {
				publisher.On("PublishAccessDelete", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			}

			h := &EventHandlers{
				publisher:    publisher,
				userLookup:   stubV1UserLookup{},
				idMapper:     stubIDMapper{},
				v1MappingsKV: mappingsKV,
				logger:       slog.Default(),
			}

			retry := h.handleRegistrantUpdate(context.Background(), "itx-zoom-meetings-registrants-v2."+registrantUID, tt.v1Data)

			assert.False(t, retry)
			publisher.AssertNumberOfCalls(t, "PublishAccessDelete", tt.wantAccessDeleteCalls)
			mappingsKV.AssertExpectations(t)
			publisher.AssertExpectations(t)
		})
	}
}
