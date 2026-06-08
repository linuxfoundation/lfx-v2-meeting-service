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
	sub string
	err error
}

func (s stubUserReader) SubByEmail(_ context.Context, _ string) (string, error) {
	return s.sub, s.err
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
			userReader: stubUserReader{sub: "auth0|existing"},
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
