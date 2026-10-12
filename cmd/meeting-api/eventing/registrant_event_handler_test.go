// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"testing"
	"time"

	fgaconstants "github.com/linuxfoundation/lfx-v2-fga-sync/pkg/constants"
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
func (m *mockEventPublisher) PublishMeetingHostCredentialsEvent(_ context.Context, _ string, _ *models.MeetingHostCredentialsEventData) error {
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

// TestConvertMapToInviteResponseData_MailerDaemon verifies that mailer-daemon bounce emails
// are silently filtered (nil, nil) before required-field validation or any KV lookup occurs.
// Addresses where "mailer-daemon" appears only in the domain or as a subaddress suffix are
// not filtered; those cases require a real KV stub and are covered by integration tests.
func TestConvertMapToInviteResponseData_MailerDaemon(t *testing.T) {
	cases := []struct {
		name  string
		email string
	}{
		{"standard lowercase", "mailer-daemon@example.com"},
		{"uppercase", "MAILER-DAEMON@example.com"},
		{"mixed case", "Mailer-Daemon@example.com"},
		// Payload missing required id/meeting_id — still filtered before validation runs.
		{"missing required fields", "mailer-daemon@bounce.example.org"},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			v1Data := map[string]interface{}{
				"id":         "resp-1",
				"meeting_id": "mtg-1",
				"email":      tt.email,
				"response":   "accepted",
			}
			// v1ObjectsKV is nil — the filter must return before any KV call is made.
			result, err := convertMapToInviteResponseData(context.Background(), v1Data, stubV1UserLookup{}, stubIDMapper{}, nil, slog.Default())
			require.NoError(t, err)
			assert.Nil(t, result)
		})
	}
}

// TestConvertMapToInviteResponseData_NoneMeetingID verifies that invite responses with
// meeting_id "NONE" (a v1 sentinel for unassociated responses) are silently skipped,
// even when other required fields (id) are absent — confirming the guard fires before
// required-field validation.
func TestConvertMapToInviteResponseData_NoneMeetingID(t *testing.T) {
	v1Data := map[string]interface{}{
		"id":         "",
		"meeting_id": "NONE",
		"email":      "user@example.com",
		"response":   "accepted",
	}
	// v1ObjectsKV is nil — the sentinel check must return before any KV call is made.
	// An empty id would trigger the required-field error if the guard were after validation.
	result, err := convertMapToInviteResponseData(context.Background(), v1Data, stubV1UserLookup{}, stubIDMapper{}, nil, slog.Default())
	require.NoError(t, err)
	assert.Nil(t, result)
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

// processInviteAcceptedEvent is exercised in invite_accepted_subscriber_test.go, next to
// the subscriber that owns it.

// TestHandleRegistrantUpdate_UsernameCleared verifies that a CDC update that results in an
// empty username — whether the "username" key is absent (v1 removes the field on clear) or
// explicitly present as "" — revokes the previously-stored FGA access.
//
// Both shapes are treated identically: utils.GetString(v1Data["username"]) returns "" in
// either case, which correctly triggers a member_remove against the stored oldUsername.
func TestHandleRegistrantUpdate_UsernameCleared(t *testing.T) {
	const (
		registrantUID = "reg-1"
		meetingID     = "meeting-1"
		oldUsername   = "alice"
	)

	storedMapping := buildRegistrantMappingValue(registrantUID, oldUsername, meetingID)

	tests := []struct {
		name   string
		v1Data map[string]interface{}
	}{
		{
			name: "username key absent (v1 clear CDC shape) revokes",
			v1Data: map[string]interface{}{
				"registrant_id": registrantUID,
				"meeting_id":    meetingID,
				// "username" key absent — shape v1 sends when username is removed
			},
		},
		{
			name: "username key present and empty revokes",
			v1Data: map[string]interface{}{
				"registrant_id": registrantUID,
				"meeting_id":    meetingID,
				"username":      "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mappingsKV := &mockKeyValue{}
			publisher := &mockEventPublisher{}

			mappingsKV.On("Get", mock.Anything, "v1_meetings."+meetingID).
				Return(mockKeyValueEntry{key: "v1_meetings." + meetingID, value: []byte("1")}, nil)
			mappingsKV.On("Get", mock.Anything, "v1_meeting_registrants."+registrantUID).
				Return(mockKeyValueEntry{key: "v1_meeting_registrants." + registrantUID, value: []byte(storedMapping)}, nil)
			mappingsKV.On("Put", mock.Anything, "v1_meeting_registrants."+registrantUID, mock.Anything).
				Return(uint64(2), nil)

			publisher.On("PublishRegistrantEvent", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			publisher.On("PublishAccessDelete", mock.Anything, mock.Anything, mock.Anything).Return(nil)

			h := &EventHandlers{
				publisher:    publisher,
				userLookup:   stubV1UserLookup{},
				idMapper:     stubIDMapper{},
				v1MappingsKV: mappingsKV,
				logger:       slog.Default(),
			}

			retry := h.handleRegistrantUpdate(context.Background(), "itx-zoom-meetings-registrants-v2."+registrantUID, tt.v1Data)

			assert.False(t, retry)
			publisher.AssertNumberOfCalls(t, "PublishAccessDelete", 1)
			mappingsKV.AssertExpectations(t)
			publisher.AssertExpectations(t)
		})
	}
}

// TestHandleRegistrantUpdate_UsernameUnchanged verifies that an update where the username
// is the same as the stored value does not trigger a member_remove.
func TestHandleRegistrantUpdate_UsernameUnchanged(t *testing.T) {
	const (
		registrantUID = "reg-2"
		meetingID     = "meeting-2"
		username      = "bob"
	)

	storedMapping := buildRegistrantMappingValue(registrantUID, username, meetingID)

	mappingsKV := &mockKeyValue{}
	publisher := &mockEventPublisher{}

	mappingsKV.On("Get", mock.Anything, "v1_meetings."+meetingID).
		Return(mockKeyValueEntry{key: "v1_meetings." + meetingID, value: []byte("1")}, nil)
	mappingsKV.On("Get", mock.Anything, "v1_meeting_registrants."+registrantUID).
		Return(mockKeyValueEntry{key: "v1_meeting_registrants." + registrantUID, value: []byte(storedMapping)}, nil)
	mappingsKV.On("Put", mock.Anything, "v1_meeting_registrants."+registrantUID, mock.Anything).
		Return(uint64(2), nil)

	publisher.On("PublishRegistrantEvent", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	h := &EventHandlers{
		publisher:    publisher,
		userLookup:   stubV1UserLookup{},
		idMapper:     stubIDMapper{},
		v1MappingsKV: mappingsKV,
		logger:       slog.Default(),
	}

	v1Data := map[string]interface{}{
		"registrant_id": registrantUID,
		"meeting_id":    meetingID,
		"username":      username,
	}

	retry := h.handleRegistrantUpdate(context.Background(), "itx-zoom-meetings-registrants-v2."+registrantUID, v1Data)

	assert.False(t, retry)
	publisher.AssertNumberOfCalls(t, "PublishAccessDelete", 0)
	mappingsKV.AssertExpectations(t)
	publisher.AssertExpectations(t)
}

// TestHandleRegistrantUpdate_ParentMeetingTombstoned verifies that a registrant update is
// dropped without publishing when the parent meeting mapping holds the tombstone marker,
// i.e. the meeting was soft-deleted after the delete handler ran.
func TestHandleRegistrantUpdate_ParentMeetingTombstoned(t *testing.T) {
	const (
		registrantUID = "reg-tomb"
		meetingID     = "meeting-tomb"
	)

	mappingsKV := &mockKeyValue{}
	publisher := &mockEventPublisher{}

	mappingsKV.On("Get", mock.Anything, "v1_meetings."+meetingID).
		Return(mockKeyValueEntry{key: "v1_meetings." + meetingID, value: []byte(tombstoneMarker)}, nil)

	h := &EventHandlers{
		publisher:    publisher,
		userLookup:   stubV1UserLookup{},
		idMapper:     stubIDMapper{},
		v1MappingsKV: mappingsKV,
		logger:       slog.Default(),
	}

	retry := h.handleRegistrantUpdate(context.Background(), "itx-zoom-meetings-registrants-v2."+registrantUID,
		map[string]interface{}{
			"registrant_id": registrantUID,
			"meeting_id":    meetingID,
			"username":      "alice",
		})

	assert.False(t, retry)
	publisher.AssertNumberOfCalls(t, "PublishRegistrantEvent", 0)
	publisher.AssertNumberOfCalls(t, "PublishAccessDelete", 0)
	mappingsKV.AssertExpectations(t)
	publisher.AssertExpectations(t)
}

// fakeV1UserLookup resolves platform IDs from a fixed map; unknown IDs return err
// (or jetstream.ErrKeyNotFound when err is nil), mirroring NATSUserLookup.
type fakeV1UserLookup struct {
	users map[string]string
	err   error
}

func (f fakeV1UserLookup) LookupUser(_ context.Context, platformID string) (*domain.V1User, error) {
	if f.err != nil {
		return nil, f.err
	}
	if username, ok := f.users[platformID]; ok {
		return &domain.V1User{Username: username}, nil
	}
	return nil, fmt.Errorf("failed to lookup v1 user: %w", jetstream.ErrKeyNotFound)
}

// historyEntry is a KV entry with a configurable operation, for v1-objects History results.
type historyEntry struct {
	mockKeyValueEntry
	op jetstream.KeyValueOp
}

func (e historyEntry) Operation() jetstream.KeyValueOp { return e.op }

// removedUsernames decodes the usernames of every member_remove published on publisher.
func removedUsernames(t *testing.T, publisher *mockEventPublisher, meetingID string) []string {
	t.Helper()
	var usernames []string
	for _, call := range publisher.Calls {
		if call.Method != "PublishAccessDelete" {
			continue
		}
		require.Equal(t, fgaconstants.GenericMemberRemoveSubject, call.Arguments.String(1))
		var msg struct {
			Data struct {
				UID      string `json:"uid"`
				Username string `json:"username"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(call.Arguments.Get(2).([]byte), &msg))
		assert.Equal(t, meetingID, msg.Data.UID)
		usernames = append(usernames, msg.Data.Username)
	}
	return usernames
}

// TestHandleRegistrantDelete verifies that every registrant removal revokes the principals
// the create path could have granted — including the user_id-enriched username behind a
// legacy "1" mapping — and that the mapping is not tombstoned while a principal is unknown.
func TestHandleRegistrantDelete(t *testing.T) {
	const (
		registrantUID = "reg-del"
		meetingID     = "meeting-del"
		userID        = "user-1"
		enriched      = "alice"
	)
	key := "itx-zoom-meetings-registrants-v2." + registrantUID
	mappingKey := "v1_meeting_registrants." + registrantUID
	record := func(fields map[string]any) []byte {
		fields["registrant_id"] = registrantUID
		fields["meeting_id"] = meetingID
		b, err := json.Marshal(fields)
		require.NoError(t, err)
		return b
	}
	put := func(value []byte) jetstream.KeyValueEntry {
		return historyEntry{mockKeyValueEntry{key: key, value: value}, jetstream.KeyValuePut}
	}
	noHistory := func(kv *mockKeyValue) {
		kv.On("History", mock.Anything, key).Return(nil, jetstream.ErrKeyNotFound)
	}
	userIDOnly := map[string]interface{}{"registrant_id": registrantUID, "meeting_id": meetingID, "user_id": userID}

	tests := []struct {
		name          string
		mapping       string // "" means no mapping exists
		v1Data        map[string]interface{}
		lookup        fakeV1UserLookup
		setupObjects  func(*mockKeyValue)
		wantRetry     bool
		wantRemoved   []string
		wantTombstone bool
	}{
		{
			name:    "hard delete of legacy mapping resolves user_id from v1 history",
			mapping: "1",
			lookup:  fakeV1UserLookup{users: map[string]string{userID: enriched}},
			setupObjects: func(kv *mockKeyValue) {
				kv.On("History", mock.Anything, key).Return([]jetstream.KeyValueEntry{
					put(record(map[string]any{"user_id": userID})),
					historyEntry{mockKeyValueEntry{key: key}, jetstream.KeyValueDelete},
				}, nil)
			},
			wantRemoved:   []string{enriched},
			wantTombstone: true,
		},
		{
			name:          "soft delete of legacy mapping resolves user_id from payload",
			mapping:       "1",
			v1Data:        userIDOnly,
			lookup:        fakeV1UserLookup{users: map[string]string{userID: enriched}},
			setupObjects:  noHistory,
			wantRemoved:   []string{enriched},
			wantTombstone: true,
		},
		{
			name:    "legacy mapping revokes principals of earlier retained revisions",
			mapping: "1",
			v1Data:  map[string]interface{}{"registrant_id": registrantUID, "meeting_id": meetingID, "user_id": "user-2"},
			lookup:  fakeV1UserLookup{users: map[string]string{userID: enriched, "user-2": "bob"}},
			setupObjects: func(kv *mockKeyValue) {
				kv.On("History", mock.Anything, key).Return([]jetstream.KeyValueEntry{
					put(record(map[string]any{"user_id": userID})),
					put(record(map[string]any{"user_id": "user-2"})),
				}, nil)
			},
			wantRemoved:   []string{"bob", enriched},
			wantTombstone: true,
		},
		{
			name:          "soft delete of pre-authoritative JSON mapping with empty username resolves user_id",
			mapping:       buildRegistrantMappingValue(registrantUID, "", meetingID),
			v1Data:        userIDOnly,
			lookup:        fakeV1UserLookup{users: map[string]string{userID: enriched}},
			setupObjects:  noHistory,
			wantRemoved:   []string{enriched},
			wantTombstone: true,
		},
		{
			name:         "hard delete of legacy mapping without v1 history leaves mapping untombstoned",
			mapping:      "1",
			setupObjects: noHistory,
		},
		{
			name:         "legacy mapping whose user_id does not resolve leaves mapping untombstoned",
			mapping:      "1",
			v1Data:       userIDOnly,
			lookup:       fakeV1UserLookup{},
			setupObjects: noHistory,
		},
		{
			name:         "transient user lookup failure retries",
			mapping:      "1",
			v1Data:       userIDOnly,
			lookup:       fakeV1UserLookup{err: errors.New("nats: timeout")},
			setupObjects: noHistory,
			wantRetry:    true,
		},
		{
			name:    "transient v1 history failure retries",
			mapping: "1",
			setupObjects: func(kv *mockKeyValue) {
				kv.On("History", mock.Anything, key).Return(nil, errors.New("nats: timeout"))
			},
			wantRetry: true,
		},
		{
			name:          "authoritative mapping with no principal tombstones without lookup",
			mapping:       buildAuthoritativeRegistrantMappingValue(registrantUID, "", meetingID),
			wantTombstone: true,
		},
		{
			name:          "authoritative mapping revokes stored username",
			mapping:       buildAuthoritativeRegistrantMappingValue(registrantUID, enriched, meetingID),
			wantRemoved:   []string{enriched},
			wantTombstone: true,
		},
		{
			name:          "no mapping revokes only the payload username, not user_id",
			v1Data:        userIDOnly,
			lookup:        fakeV1UserLookup{users: map[string]string{userID: enriched}},
			wantTombstone: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objectsKV := &mockKeyValue{}
			mappingsKV := &mockKeyValue{}
			publisher := &mockEventPublisher{}
			if tt.setupObjects != nil {
				tt.setupObjects(objectsKV)
			}
			if tt.mapping == "" {
				mappingsKV.On("Get", mock.Anything, mappingKey).Return(nil, jetstream.ErrKeyNotFound)
			} else {
				mappingsKV.On("Get", mock.Anything, mappingKey).
					Return(mockKeyValueEntry{key: mappingKey, value: []byte(tt.mapping)}, nil)
			}
			if tt.wantTombstone {
				mappingsKV.On("Put", mock.Anything, mappingKey, []byte(tombstoneMarker)).Return(uint64(2), nil)
			}
			publisher.On("PublishAccessDelete", mock.Anything, mock.Anything, mock.Anything).Return(nil)

			h := &EventHandlers{
				publisher:    publisher,
				userLookup:   tt.lookup,
				idMapper:     stubIDMapper{},
				v1ObjectsKV:  objectsKV,
				v1MappingsKV: mappingsKV,
				logger:       slog.Default(),
			}

			retry := h.handleRegistrantDelete(context.Background(), key, tt.v1Data)

			assert.Equal(t, tt.wantRetry, retry)
			assert.Equal(t, tt.wantRemoved, removedUsernames(t, publisher, meetingID))
			if !tt.wantTombstone {
				mappingsKV.AssertNotCalled(t, "Put", mock.Anything, mock.Anything, mock.Anything)
			}
			objectsKV.AssertExpectations(t)
			mappingsKV.AssertExpectations(t)
		})
	}
}

// TestHandleRegistrantUpdate_LegacyMapping verifies that an update over a legacy mapping
// revokes every principal the earlier revisions could have been granted — not just the one
// the new payload resolves to — and never marks the mapping authoritative.
func TestHandleRegistrantUpdate_LegacyMapping(t *testing.T) {
	const (
		registrantUID = "reg-legacy"
		meetingID     = "meeting-legacy"
		userID        = "user-1"
		enriched      = "alice"
	)
	key := "itx-zoom-meetings-registrants-v2." + registrantUID
	mappingKey := "v1_meeting_registrants." + registrantUID
	priorRecord, err := json.Marshal(map[string]any{
		"registrant_id": registrantUID,
		"meeting_id":    meetingID,
		"user_id":       userID,
	})
	require.NoError(t, err)
	priorHistory := func(kv *mockKeyValue) {
		kv.On("History", mock.Anything, key).Return([]jetstream.KeyValueEntry{
			historyEntry{mockKeyValueEntry{key: key, value: priorRecord}, jetstream.KeyValuePut},
		}, nil)
	}

	tests := []struct {
		name        string
		v1Data      map[string]interface{}
		lookup      fakeV1UserLookup
		history     func(*mockKeyValue)
		wantRemoved []string
		wantMapping string // "" means the legacy mapping must be left untouched
	}{
		{
			name:        "user_id-only update revokes enriched principal and keeps legacy mapping",
			v1Data:      map[string]interface{}{"registrant_id": registrantUID, "meeting_id": meetingID, "user_id": userID},
			lookup:      fakeV1UserLookup{users: map[string]string{userID: enriched}},
			history:     priorHistory,
			wantRemoved: []string{enriched},
		},
		{
			name:        "changed user_id revokes the principal granted from the earlier revision too",
			v1Data:      map[string]interface{}{"registrant_id": registrantUID, "meeting_id": meetingID, "user_id": "user-2"},
			lookup:      fakeV1UserLookup{users: map[string]string{userID: enriched, "user-2": "bob"}},
			history:     priorHistory,
			wantRemoved: []string{"bob", enriched},
		},
		{
			name:        "explicit username records a non-authoritative mapping",
			v1Data:      map[string]interface{}{"registrant_id": registrantUID, "meeting_id": meetingID, "user_id": "user-2", "username": "bob"},
			lookup:      fakeV1UserLookup{users: map[string]string{userID: enriched}},
			history:     priorHistory,
			wantRemoved: []string{enriched},
			wantMapping: buildRegistrantMappingValue(registrantUID, "bob", meetingID),
		},
		{
			name:   "unresolvable principal keeps legacy mapping",
			v1Data: map[string]interface{}{"registrant_id": registrantUID, "meeting_id": meetingID, "user_id": userID},
			lookup: fakeV1UserLookup{},
			history: func(kv *mockKeyValue) {
				kv.On("History", mock.Anything, key).Return(nil, jetstream.ErrKeyNotFound)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objectsKV := &mockKeyValue{}
			mappingsKV := &mockKeyValue{}
			publisher := &mockEventPublisher{}
			tt.history(objectsKV)

			mappingsKV.On("Get", mock.Anything, "v1_meetings."+meetingID).
				Return(mockKeyValueEntry{key: "v1_meetings." + meetingID, value: []byte("1")}, nil)
			mappingsKV.On("Get", mock.Anything, mappingKey).
				Return(mockKeyValueEntry{key: mappingKey, value: []byte("1")}, nil)
			if tt.wantMapping != "" {
				mappingsKV.On("Put", mock.Anything, mappingKey, []byte(tt.wantMapping)).Return(uint64(2), nil)
			}
			publisher.On("PublishRegistrantEvent", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			publisher.On("PublishAccessDelete", mock.Anything, mock.Anything, mock.Anything).Return(nil)

			h := &EventHandlers{
				publisher:    publisher,
				userLookup:   tt.lookup,
				idMapper:     stubIDMapper{},
				v1ObjectsKV:  objectsKV,
				v1MappingsKV: mappingsKV,
				logger:       slog.Default(),
			}

			retry := h.handleRegistrantUpdate(context.Background(), key, tt.v1Data)

			assert.False(t, retry)
			assert.Equal(t, tt.wantRemoved, removedUsernames(t, publisher, meetingID))
			if tt.wantMapping == "" {
				mappingsKV.AssertNotCalled(t, "Put", mock.Anything, mock.Anything, mock.Anything)
			}
			objectsKV.AssertExpectations(t)
			mappingsKV.AssertExpectations(t)
		})
	}
}
