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

// stubEventPublisher implements EventPublisher for testing handleRegistrantUpdate.
type stubEventPublisher struct {
	registrantCalls   []registrantPublishCall
	accessDeleteCalls []accessDeleteCall
	registrantErr     error
	accessDeleteErr   error
}

type registrantPublishCall struct {
	action     string
	registrant *models.RegistrantEventData
}

type accessDeleteCall struct {
	subject string
	payload []byte
}

func (s *stubEventPublisher) PublishRegistrantEvent(_ context.Context, action string, r *models.RegistrantEventData) error {
	s.registrantCalls = append(s.registrantCalls, registrantPublishCall{action: action, registrant: r})
	return s.registrantErr
}

func (s *stubEventPublisher) PublishAccessDelete(_ context.Context, subject string, payload []byte) error {
	s.accessDeleteCalls = append(s.accessDeleteCalls, accessDeleteCall{subject: subject, payload: payload})
	return s.accessDeleteErr
}

// Stub out unused interface methods
func (s *stubEventPublisher) PublishMeetingEvent(_ context.Context, _ string, _ *models.MeetingEventData) error {
	return nil
}

func (s *stubEventPublisher) PublishInviteResponseEvent(_ context.Context, _ string, _ *models.InviteResponseEventData) error {
	return nil
}

func (s *stubEventPublisher) PublishPastMeetingEvent(_ context.Context, _ string, _ *models.PastMeetingEventData) error {
	return nil
}

func (s *stubEventPublisher) PublishPastMeetingParticipantEvent(_ context.Context, _ string, _ *models.PastMeetingParticipantEventData) error {
	return nil
}

func (s *stubEventPublisher) PublishPastMeetingRecordingEvent(_ context.Context, _ string, _ *models.RecordingEventData) error {
	return nil
}

func (s *stubEventPublisher) PublishPastMeetingTranscriptEvent(_ context.Context, _ string, _ *models.TranscriptEventData) error {
	return nil
}

func (s *stubEventPublisher) PublishPastMeetingSummaryEvent(_ context.Context, _ string, _ *models.SummaryEventData, _ string) error {
	return nil
}

func (s *stubEventPublisher) PublishMeetingAttachmentEvent(_ context.Context, _ string, _ *models.MeetingAttachmentEventData) error {
	return nil
}

func (s *stubEventPublisher) PublishPastMeetingAttachmentEvent(_ context.Context, _ string, _ *models.PastMeetingAttachmentEventData) error {
	return nil
}

func (s *stubEventPublisher) PublishIndexerDelete(_ context.Context, _, _ string) error {
	return nil
}

func (s *stubEventPublisher) Close() error {
	return nil
}

func TestHandleRegistrantUpdate_Create_NoRemove(t *testing.T) {
	const (
		uid       = "reg-1"
		meetingID = "mtg-1"
		username  = "alice"
	)

	v1Data := map[string]interface{}{
		"registrant_id": uid,
		"meeting_id":    meetingID,
		"username":      username,
		"created_at":    "2024-01-01T00:00:00Z",
	}

	meetingMappingKey := "v1_meetings." + meetingID
	registrantMappingKey := "v1_meeting_registrants." + uid

	mappingsKV := &mockKeyValue{}
	mappingsKV.On("Get", mock.Anything, meetingMappingKey).Return(mockKeyValueEntry{key: meetingMappingKey, value: []byte("1")}, nil)
	mappingsKV.On("Get", mock.Anything, registrantMappingKey).Return(nil, jetstream.ErrKeyNotFound)
	mappingsKV.On("Put", mock.Anything, registrantMappingKey, mock.MatchedBy(func(v []byte) bool {
		// Verify it's valid JSON
		var d registrantMappingData
		return json.Unmarshal(v, &d) == nil && d.UID == uid && d.Username == username && d.MeetingID == meetingID
	})).Return(uint64(1), nil)

	publisher := &stubEventPublisher{}

	h := &EventHandlers{
		v1MappingsKV: mappingsKV,
		publisher:    publisher,
		logger:       slog.Default(),
		userLookup:   nil,
		idMapper:     nil,
	}

	retry := h.handleRegistrantUpdate(context.Background(), "test-key", v1Data)
	require.False(t, retry)
	require.Len(t, publisher.accessDeleteCalls, 0)
	require.Len(t, publisher.registrantCalls, 1)
	assert.Equal(t, "created", publisher.registrantCalls[0].action)
	assert.Equal(t, username, publisher.registrantCalls[0].registrant.Username)
	mappingsKV.AssertExpectations(t)
}

func TestHandleRegistrantUpdate_Update_UnchangedUsername(t *testing.T) {
	const (
		uid       = "reg-1"
		meetingID = "mtg-1"
		username  = "alice"
	)

	v1Data := map[string]interface{}{
		"registrant_id": uid,
		"meeting_id":    meetingID,
		"username":      username,
		"created_at":    "2024-01-01T00:00:00Z",
	}

	meetingMappingKey := "v1_meetings." + meetingID
	registrantMappingKey := "v1_meeting_registrants." + uid

	// Existing mapping with alice
	oldMapping := buildRegistrantMappingValue(uid, username, meetingID)

	mappingsKV := &mockKeyValue{}
	mappingsKV.On("Get", mock.Anything, meetingMappingKey).Return(mockKeyValueEntry{key: meetingMappingKey, value: []byte("1")}, nil)
	mappingsKV.On("Get", mock.Anything, registrantMappingKey).Return(mockKeyValueEntry{key: registrantMappingKey, value: []byte(oldMapping)}, nil)
	mappingsKV.On("Put", mock.Anything, registrantMappingKey, mock.MatchedBy(func(v []byte) bool {
		var d registrantMappingData
		return json.Unmarshal(v, &d) == nil && d.Username == username
	})).Return(uint64(1), nil)

	publisher := &stubEventPublisher{}

	h := &EventHandlers{
		v1MappingsKV: mappingsKV,
		publisher:    publisher,
		logger:       slog.Default(),
		userLookup:   nil,
		idMapper:     nil,
	}

	retry := h.handleRegistrantUpdate(context.Background(), "test-key", v1Data)
	require.False(t, retry)
	require.Len(t, publisher.accessDeleteCalls, 0)
	require.Len(t, publisher.registrantCalls, 1)
	assert.Equal(t, "updated", publisher.registrantCalls[0].action)
	assert.Equal(t, username, publisher.registrantCalls[0].registrant.Username)
	mappingsKV.AssertExpectations(t)
}

func TestHandleRegistrantUpdate_Update_UsernameChanged(t *testing.T) {
	const (
		uid       = "reg-1"
		meetingID = "mtg-1"
		oldUser   = "alice"
		newUser   = "bob"
	)

	v1Data := map[string]interface{}{
		"registrant_id": uid,
		"meeting_id":    meetingID,
		"username":      newUser,
		"created_at":    "2024-01-01T00:00:00Z",
	}

	meetingMappingKey := "v1_meetings." + meetingID
	registrantMappingKey := "v1_meeting_registrants." + uid

	// Existing mapping with alice
	oldMapping := buildRegistrantMappingValue(uid, oldUser, meetingID)

	mappingsKV := &mockKeyValue{}
	mappingsKV.On("Get", mock.Anything, meetingMappingKey).Return(mockKeyValueEntry{key: meetingMappingKey, value: []byte("1")}, nil)
	mappingsKV.On("Get", mock.Anything, registrantMappingKey).Return(mockKeyValueEntry{key: registrantMappingKey, value: []byte(oldMapping)}, nil)
	mappingsKV.On("Put", mock.Anything, registrantMappingKey, mock.MatchedBy(func(v []byte) bool {
		var d registrantMappingData
		return json.Unmarshal(v, &d) == nil && d.Username == newUser
	})).Return(uint64(1), nil)

	publisher := &stubEventPublisher{}

	h := &EventHandlers{
		v1MappingsKV: mappingsKV,
		publisher:    publisher,
		logger:       slog.Default(),
		userLookup:   nil,
		idMapper:     nil,
	}

	retry := h.handleRegistrantUpdate(context.Background(), "test-key", v1Data)
	require.False(t, retry)
	require.Len(t, publisher.accessDeleteCalls, 1)
	require.Len(t, publisher.registrantCalls, 1)
	assert.Equal(t, "updated", publisher.registrantCalls[0].action)
	assert.Equal(t, newUser, publisher.registrantCalls[0].registrant.Username)
	mappingsKV.AssertExpectations(t)
}

func TestHandleRegistrantUpdate_Update_UsernameCleared(t *testing.T) {
	const (
		uid       = "reg-1"
		meetingID = "mtg-1"
		oldUser   = "alice"
	)

	v1Data := map[string]interface{}{
		"registrant_id": uid,
		"meeting_id":    meetingID,
		"username":      "", // Cleared
		"created_at":    "2024-01-01T00:00:00Z",
	}

	meetingMappingKey := "v1_meetings." + meetingID
	registrantMappingKey := "v1_meeting_registrants." + uid

	// Existing mapping with alice
	oldMapping := buildRegistrantMappingValue(uid, oldUser, meetingID)

	mappingsKV := &mockKeyValue{}
	mappingsKV.On("Get", mock.Anything, meetingMappingKey).Return(mockKeyValueEntry{key: meetingMappingKey, value: []byte("1")}, nil)
	mappingsKV.On("Get", mock.Anything, registrantMappingKey).Return(mockKeyValueEntry{key: registrantMappingKey, value: []byte(oldMapping)}, nil)
	mappingsKV.On("Put", mock.Anything, registrantMappingKey, mock.MatchedBy(func(v []byte) bool {
		var d registrantMappingData
		return json.Unmarshal(v, &d) == nil && d.Username == ""
	})).Return(uint64(1), nil)

	publisher := &stubEventPublisher{}

	h := &EventHandlers{
		v1MappingsKV: mappingsKV,
		publisher:    publisher,
		logger:       slog.Default(),
		userLookup:   nil,
		idMapper:     nil,
	}

	retry := h.handleRegistrantUpdate(context.Background(), "test-key", v1Data)
	require.False(t, retry)
	require.Len(t, publisher.accessDeleteCalls, 1)
	require.Len(t, publisher.registrantCalls, 1)
	assert.Equal(t, "updated", publisher.registrantCalls[0].action)
	assert.Equal(t, "", publisher.registrantCalls[0].registrant.Username)
	mappingsKV.AssertExpectations(t)
}

func TestHandleRegistrantUpdate_LegacySentinel(t *testing.T) {
	const (
		uid       = "reg-1"
		meetingID = "mtg-1"
		username  = "alice"
	)

	v1Data := map[string]interface{}{
		"registrant_id": uid,
		"meeting_id":    meetingID,
		"username":      username,
		"created_at":    "2024-01-01T00:00:00Z",
	}

	meetingMappingKey := "v1_meetings." + meetingID
	registrantMappingKey := "v1_meeting_registrants." + uid

	// Existing mapping with legacy "1" sentinel
	mappingsKV := &mockKeyValue{}
	mappingsKV.On("Get", mock.Anything, meetingMappingKey).Return(mockKeyValueEntry{key: meetingMappingKey, value: []byte("1")}, nil)
	mappingsKV.On("Get", mock.Anything, registrantMappingKey).Return(mockKeyValueEntry{key: registrantMappingKey, value: []byte("1")}, nil)
	mappingsKV.On("Put", mock.Anything, registrantMappingKey, mock.MatchedBy(func(v []byte) bool {
		var d registrantMappingData
		return json.Unmarshal(v, &d) == nil && d.Username == username
	})).Return(uint64(1), nil)

	publisher := &stubEventPublisher{}

	h := &EventHandlers{
		v1MappingsKV: mappingsKV,
		publisher:    publisher,
		logger:       slog.Default(),
		userLookup:   nil,
		idMapper:     nil,
	}

	retry := h.handleRegistrantUpdate(context.Background(), "test-key", v1Data)
	require.False(t, retry)
	require.Len(t, publisher.accessDeleteCalls, 0, "legacy sentinel should not trigger remove")
	require.Len(t, publisher.registrantCalls, 1)
	assert.Equal(t, "updated", publisher.registrantCalls[0].action)
	mappingsKV.AssertExpectations(t)
}

func TestHandleRegistrantUpdate_RemovePublishFailure_Transient_NAK(t *testing.T) {
	const (
		uid       = "reg-1"
		meetingID = "mtg-1"
		oldUser   = "alice"
		newUser   = "bob"
	)

	v1Data := map[string]interface{}{
		"registrant_id": uid,
		"meeting_id":    meetingID,
		"username":      newUser,
		"created_at":    "2024-01-01T00:00:00Z",
	}

	meetingMappingKey := "v1_meetings." + meetingID
	registrantMappingKey := "v1_meeting_registrants." + uid

	oldMapping := buildRegistrantMappingValue(uid, oldUser, meetingID)

	mappingsKV := &mockKeyValue{}
	mappingsKV.On("Get", mock.Anything, meetingMappingKey).Return(mockKeyValueEntry{key: meetingMappingKey, value: []byte("1")}, nil)
	mappingsKV.On("Get", mock.Anything, registrantMappingKey).Return(mockKeyValueEntry{key: registrantMappingKey, value: []byte(oldMapping)}, nil)

	publisher := &stubEventPublisher{
		accessDeleteErr: errors.New("transient publish failure"),
	}

	h := &EventHandlers{
		v1MappingsKV: mappingsKV,
		publisher:    publisher,
		logger:       slog.Default(),
		userLookup:   nil,
		idMapper:     nil,
	}

	retry := h.handleRegistrantUpdate(context.Background(), "test-key", v1Data)
	require.True(t, retry, "should retry on transient publish failure")
	require.Len(t, publisher.accessDeleteCalls, 1)
	require.Len(t, publisher.registrantCalls, 0, "should not publish registrant event if remove fails")
	mappingsKV.AssertNotCalled(t, "Put")
}

func TestHandleRegistrantUpdate_MappingWriteFailure_Transient_NAK(t *testing.T) {
	const (
		uid       = "reg-1"
		meetingID = "mtg-1"
		username  = "alice"
	)

	v1Data := map[string]interface{}{
		"registrant_id": uid,
		"meeting_id":    meetingID,
		"username":      username,
		"created_at":    "2024-01-01T00:00:00Z",
	}

	meetingMappingKey := "v1_meetings." + meetingID
	registrantMappingKey := "v1_meeting_registrants." + uid

	oldMapping := buildRegistrantMappingValue(uid, username, meetingID)

	mappingsKV := &mockKeyValue{}
	mappingsKV.On("Get", mock.Anything, meetingMappingKey).Return(mockKeyValueEntry{key: meetingMappingKey, value: []byte("1")}, nil)
	mappingsKV.On("Get", mock.Anything, registrantMappingKey).Return(mockKeyValueEntry{key: registrantMappingKey, value: []byte(oldMapping)}, nil)
	mappingsKV.On("Put", mock.Anything, registrantMappingKey, mock.Anything).Return(uint64(0), errors.New("transient write failure"))

	publisher := &stubEventPublisher{}

	h := &EventHandlers{
		v1MappingsKV: mappingsKV,
		publisher:    publisher,
		logger:       slog.Default(),
		userLookup:   nil,
		idMapper:     nil,
	}

	retry := h.handleRegistrantUpdate(context.Background(), "test-key", v1Data)
	require.True(t, retry, "should retry on transient write failure")
	require.Len(t, publisher.registrantCalls, 1)
	mappingsKV.AssertExpectations(t)
}

func TestHandleRegistrantUpdate_MappingReadTransientError_NAK(t *testing.T) {
	const (
		uid       = "reg-1"
		meetingID = "mtg-1"
		username  = "alice"
	)

	v1Data := map[string]interface{}{
		"registrant_id": uid,
		"meeting_id":    meetingID,
		"username":      username,
		"created_at":    "2024-01-01T00:00:00Z",
	}

	meetingMappingKey := "v1_meetings." + meetingID
	registrantMappingKey := "v1_meeting_registrants." + uid

	mappingsKV := &mockKeyValue{}
	mappingsKV.On("Get", mock.Anything, meetingMappingKey).Return(mockKeyValueEntry{key: meetingMappingKey, value: []byte("1")}, nil)
	mappingsKV.On("Get", mock.Anything, registrantMappingKey).Return(nil, errors.New("transient connection error"))

	publisher := &stubEventPublisher{}

	h := &EventHandlers{
		v1MappingsKV: mappingsKV,
		publisher:    publisher,
		logger:       slog.Default(),
		userLookup:   nil,
		idMapper:     nil,
	}

	retry := h.handleRegistrantUpdate(context.Background(), "test-key", v1Data)
	require.True(t, retry, "should retry on transient read error")
	require.Len(t, publisher.registrantCalls, 0)
	mappingsKV.AssertNotCalled(t, "Put")
}

// Tests for handleRegistrantDelete

func TestHandleRegistrantDelete_ValidMapping_SendsRemove(t *testing.T) {
	const (
		uid       = "reg-1"
		meetingID = "mtg-1"
		username  = "alice"
	)

	mappingKey := "v1_meeting_registrants." + uid
	mapping := buildRegistrantMappingValue(uid, username, meetingID)

	mappingsKV := &mockKeyValue{}
	mappingsKV.On("Get", mock.Anything, mappingKey).Return(mockKeyValueEntry{key: mappingKey, value: []byte(mapping)}, nil)
	mappingsKV.On("Put", mock.Anything, mappingKey, []byte("!del")).Return(uint64(1), nil)

	publisher := &stubEventPublisher{}

	h := &EventHandlers{
		v1MappingsKV: mappingsKV,
		publisher:    publisher,
		logger:       slog.Default(),
	}

	key := "itx-zoom-meetings-registrants-v2." + uid
	retry := h.handleRegistrantDelete(context.Background(), key, nil)
	require.False(t, retry)
	require.Len(t, publisher.accessDeleteCalls, 1)
	assert.Equal(t, "lfx.fga-sync.member_remove", publisher.accessDeleteCalls[0].subject)
	mappingsKV.AssertExpectations(t)
}

func TestHandleRegistrantDelete_LegacySentinel_FallsBackToPayload(t *testing.T) {
	const (
		uid       = "reg-1"
		meetingID = "mtg-1"
		username  = "alice"
	)

	mappingKey := "v1_meeting_registrants." + uid

	mappingsKV := &mockKeyValue{}
	mappingsKV.On("Get", mock.Anything, mappingKey).Return(mockKeyValueEntry{key: mappingKey, value: []byte("1")}, nil)
	mappingsKV.On("Put", mock.Anything, mappingKey, []byte("!del")).Return(uint64(1), nil)

	publisher := &stubEventPublisher{}

	h := &EventHandlers{
		v1MappingsKV: mappingsKV,
		publisher:    publisher,
		logger:       slog.Default(),
	}

	v1Data := map[string]interface{}{
		"meeting_id": meetingID,
		"username":   username,
	}

	key := "itx-zoom-meetings-registrants-v2." + uid
	retry := h.handleRegistrantDelete(context.Background(), key, v1Data)
	require.False(t, retry)
	require.Len(t, publisher.accessDeleteCalls, 1)
	assert.Equal(t, "lfx.fga-sync.member_remove", publisher.accessDeleteCalls[0].subject)
	mappingsKV.AssertExpectations(t)
}

func TestHandleRegistrantDelete_Tombstoned_SkipsRemove(t *testing.T) {
	const (
		uid = "reg-1"
	)

	mappingKey := "v1_meeting_registrants." + uid
	tombstone := "!del"

	mappingsKV := &mockKeyValue{}
	mappingsKV.On("Get", mock.Anything, mappingKey).Return(mockKeyValueEntry{key: mappingKey, value: []byte(tombstone)}, nil)

	publisher := &stubEventPublisher{}

	h := &EventHandlers{
		v1MappingsKV: mappingsKV,
		publisher:    publisher,
		logger:       slog.Default(),
	}

	key := "itx-zoom-meetings-registrants-v2." + uid
	retry := h.handleRegistrantDelete(context.Background(), key, nil)
	require.False(t, retry)
	require.Len(t, publisher.accessDeleteCalls, 0, "should not publish remove for tombstoned mapping")
	mappingsKV.AssertNotCalled(t, "Delete")
}

func TestHandleRegistrantDelete_MappingNotFound_UsesPayload(t *testing.T) {
	const (
		uid       = "reg-1"
		meetingID = "mtg-1"
		username  = "alice"
	)

	mappingKey := "v1_meeting_registrants." + uid

	mappingsKV := &mockKeyValue{}
	mappingsKV.On("Get", mock.Anything, mappingKey).Return(nil, jetstream.ErrKeyNotFound)
	mappingsKV.On("Put", mock.Anything, mappingKey, []byte("!del")).Return(uint64(1), nil)

	publisher := &stubEventPublisher{}

	h := &EventHandlers{
		v1MappingsKV: mappingsKV,
		publisher:    publisher,
		logger:       slog.Default(),
	}

	v1Data := map[string]interface{}{
		"meeting_id": meetingID,
		"username":   username,
	}

	key := "itx-zoom-meetings-registrants-v2." + uid
	retry := h.handleRegistrantDelete(context.Background(), key, v1Data)
	require.False(t, retry)
	require.Len(t, publisher.accessDeleteCalls, 1)
	assert.Equal(t, "lfx.fga-sync.member_remove", publisher.accessDeleteCalls[0].subject)
	mappingsKV.AssertExpectations(t)
}

func TestHandleRegistrantDelete_TransientMappingError_NAK(t *testing.T) {
	const (
		uid = "reg-1"
	)

	mappingKey := "v1_meeting_registrants." + uid

	mappingsKV := &mockKeyValue{}
	mappingsKV.On("Get", mock.Anything, mappingKey).Return(nil, errors.New("transient connection error"))

	publisher := &stubEventPublisher{}

	h := &EventHandlers{
		v1MappingsKV: mappingsKV,
		publisher:    publisher,
		logger:       slog.Default(),
	}

	key := "itx-zoom-meetings-registrants-v2." + uid
	retry := h.handleRegistrantDelete(context.Background(), key, nil)
	require.True(t, retry, "should retry on transient mapping error")
	require.Len(t, publisher.accessDeleteCalls, 0)
	mappingsKV.AssertNotCalled(t, "Delete")
}
