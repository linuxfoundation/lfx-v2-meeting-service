// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
)

// --- Stubs ---

type stubParticipantPublisher struct {
	participantCalls  []participantPublishCall
	accessDeleteCalls []accessDeleteCall // reuse type from registrant tests
	participantErr    error
	accessDeleteErr   error
}

type participantPublishCall struct {
	action      string
	participant *models.PastMeetingParticipantEventData
}

func (s *stubParticipantPublisher) PublishPastMeetingParticipantEvent(_ context.Context, action string, p *models.PastMeetingParticipantEventData) error {
	s.participantCalls = append(s.participantCalls, participantPublishCall{action: action, participant: p})
	return s.participantErr
}
func (s *stubParticipantPublisher) PublishAccessDelete(_ context.Context, subject string, payload []byte) error {
	s.accessDeleteCalls = append(s.accessDeleteCalls, accessDeleteCall{subject: subject, payload: payload})
	return s.accessDeleteErr
}
func (s *stubParticipantPublisher) PublishRegistrantEvent(_ context.Context, _ string, _ *models.RegistrantEventData) error {
	return nil
}
func (s *stubParticipantPublisher) PublishMeetingEvent(_ context.Context, _ string, _ *models.MeetingEventData) error {
	return nil
}
func (s *stubParticipantPublisher) PublishInviteResponseEvent(_ context.Context, _ string, _ *models.InviteResponseEventData) error {
	return nil
}
func (s *stubParticipantPublisher) PublishPastMeetingEvent(_ context.Context, _ string, _ *models.PastMeetingEventData) error {
	return nil
}
func (s *stubParticipantPublisher) PublishPastMeetingRecordingEvent(_ context.Context, _ string, _ *models.RecordingEventData) error {
	return nil
}
func (s *stubParticipantPublisher) PublishPastMeetingTranscriptEvent(_ context.Context, _ string, _ *models.TranscriptEventData) error {
	return nil
}
func (s *stubParticipantPublisher) PublishPastMeetingSummaryEvent(_ context.Context, _ string, _ *models.SummaryEventData, _ string) error {
	return nil
}
func (s *stubParticipantPublisher) PublishMeetingAttachmentEvent(_ context.Context, _ string, _ *models.MeetingAttachmentEventData) error {
	return nil
}
func (s *stubParticipantPublisher) PublishPastMeetingAttachmentEvent(_ context.Context, _ string, _ *models.PastMeetingAttachmentEventData) error {
	return nil
}
func (s *stubParticipantPublisher) PublishIndexerDelete(_ context.Context, _, _ string) error {
	return nil
}
func (s *stubParticipantPublisher) Close() error { return nil }

// stubIDMapper returns a fixed v2 UID for any v1 ID.
type stubIDMapper struct{ projectUID string }

func (s *stubIDMapper) MapProjectV1ToV2(_ context.Context, _ string) (string, error) {
	if s.projectUID == "" {
		return "", domain.NewValidationError("project not found")
	}
	return s.projectUID, nil
}
func (s *stubIDMapper) MapProjectV2ToV1(_ context.Context, _ string) (string, error) {
	return "", domain.NewValidationError("not found")
}
func (s *stubIDMapper) MapCommitteeV2ToV1(_ context.Context, _ string) (string, error) {
	return "", domain.NewValidationError("not found")
}
func (s *stubIDMapper) MapCommitteeV1ToV2(_ context.Context, _ string) (string, error) {
	return "", domain.NewValidationError("not found")
}
func (s *stubIDMapper) MapInviteeIDToParticipantV2(_ context.Context, _ string) (string, error) {
	return "", domain.NewValidationError("not found")
}
func (s *stubIDMapper) MapAttendeeIDToParticipantV2(_ context.Context, _ string) (string, error) {
	return "", domain.NewValidationError("not found")
}
func (s *stubIDMapper) MapParticipantV2ToInviteeID(_ context.Context, _ string) (string, error) {
	return "", domain.NewValidationError("not found")
}
func (s *stubIDMapper) MapParticipantV2ToAttendeeID(_ context.Context, _ string) (string, error) {
	return "", domain.NewValidationError("not found")
}

// stubUserLookup always returns nil user with no error.
type stubParticipantUserLookup struct{}

func (s *stubParticipantUserLookup) LookupUser(_ context.Context, _ string) (*domain.V1User, error) {
	return nil, nil
}

// inviteeV1Data returns minimal valid v1Data for an invitee with the given fields.
func inviteeV1Data(uid, meetingAndOccurrenceID, projectID, projectSlug, username string) map[string]interface{} {
	return map[string]interface{}{
		"invitee_id":                uid,
		"meeting_and_occurrence_id": meetingAndOccurrenceID,
		"proj_id":                   projectID,
		"project_slug":              projectSlug,
		"lf_sso":                    username,
	}
}

// attendeeV1Data returns minimal valid v1Data for an attendee with the given fields.
func attendeeV1Data(uid, meetingAndOccurrenceID, projectID, projectSlug, username string) map[string]interface{} {
	return map[string]interface{}{
		"id":                        uid,
		"meeting_and_occurrence_id": meetingAndOccurrenceID,
		"proj_id":                   projectID,
		"project_slug":              projectSlug,
		"lf_sso":                    username,
	}
}

// newParticipantHandler builds an EventHandlers wired with the supplied stubs.
func newParticipantHandler(mappingsKV *mockKeyValue, objectsKV *mockKeyValue, pub *stubParticipantPublisher) *EventHandlers {
	return &EventHandlers{
		v1MappingsKV: mappingsKV,
		v1ObjectsKV:  objectsKV,
		publisher:    pub,
		logger:       slog.Default(),
		userLookup:   &stubParticipantUserLookup{},
		idMapper:     &stubIDMapper{projectUID: "proj-uid-1"},
	}
}

// --- Invitee tests ---

func TestHandleInviteeUpdate_Create_NoRemove(t *testing.T) {
	const (
		uid     = "inv-1"
		meeting = "meet-occ-1"
		projID  = "sfid-1"
		slug    = "my-project"
		user    = "alice"
	)

	inviteeMappingKey := "v1_past_meeting_invitees." + uid
	attendeeXrefKey := "v1_participant_by_meeting_user.attendee." + meeting + "." + user
	inviteeXrefKey := "v1_participant_by_meeting_user.invitee." + meeting + "." + user

	objectsKV := &mockKeyValue{}
	mappingsKV := &mockKeyValue{}
	// No existing invitee mapping (first time)
	mappingsKV.On("Get", mock.Anything, inviteeMappingKey).Return(nil, jetstream.ErrKeyNotFound)
	// No cross-reference from attendee side
	mappingsKV.On("Get", mock.Anything, attendeeXrefKey).Return(nil, jetstream.ErrKeyNotFound)
	// Store new JSON mapping value
	mappingsKV.On("Put", mock.Anything, inviteeMappingKey, mock.MatchedBy(func(v []byte) bool {
		var d registrantMappingData
		return json.Unmarshal(v, &d) == nil && d.UID == uid && d.Username == user && d.MeetingID == meeting
	})).Return(uint64(1), nil)
	// Store xref
	mappingsKV.On("Put", mock.Anything, inviteeXrefKey, []byte(uid)).Return(uint64(1), nil)

	pub := &stubParticipantPublisher{}
	h := newParticipantHandler(mappingsKV, objectsKV, pub)

	retry := h.handlePastMeetingInviteeUpdate(context.Background(), "test-key", inviteeV1Data(uid, meeting, projID, slug, user))

	require.False(t, retry)
	require.Len(t, pub.accessDeleteCalls, 0, "create should not emit member_remove")
	require.Len(t, pub.participantCalls, 1)
	assert.Equal(t, "created", pub.participantCalls[0].action)
	mappingsKV.AssertExpectations(t)
}

func TestHandleInviteeUpdate_Update_UnchangedUsername(t *testing.T) {
	const (
		uid     = "inv-1"
		meeting = "meet-occ-1"
		projID  = "sfid-1"
		slug    = "my-project"
		user    = "alice"
	)

	inviteeMappingKey := "v1_past_meeting_invitees." + uid
	attendeeXrefKey := "v1_participant_by_meeting_user.attendee." + meeting + "." + user
	inviteeXrefKey := "v1_participant_by_meeting_user.invitee." + meeting + "." + user

	oldMapping := buildRegistrantMappingValue(uid, user, meeting)

	objectsKV := &mockKeyValue{}
	mappingsKV := &mockKeyValue{}
	mappingsKV.On("Get", mock.Anything, inviteeMappingKey).Return(mockKeyValueEntry{key: inviteeMappingKey, value: []byte(oldMapping)}, nil)
	mappingsKV.On("Get", mock.Anything, attendeeXrefKey).Return(nil, jetstream.ErrKeyNotFound)
	mappingsKV.On("Put", mock.Anything, inviteeMappingKey, mock.MatchedBy(func(v []byte) bool {
		var d registrantMappingData
		return json.Unmarshal(v, &d) == nil && d.Username == user
	})).Return(uint64(1), nil)
	mappingsKV.On("Put", mock.Anything, inviteeXrefKey, []byte(uid)).Return(uint64(1), nil)

	pub := &stubParticipantPublisher{}
	h := newParticipantHandler(mappingsKV, objectsKV, pub)

	retry := h.handlePastMeetingInviteeUpdate(context.Background(), "test-key", inviteeV1Data(uid, meeting, projID, slug, user))

	require.False(t, retry)
	require.Len(t, pub.accessDeleteCalls, 0, "unchanged username should not emit member_remove")
	require.Len(t, pub.participantCalls, 1)
	assert.Equal(t, "updated", pub.participantCalls[0].action)
	mappingsKV.AssertExpectations(t)
}

func TestHandleInviteeUpdate_Update_UsernameCleared(t *testing.T) {
	const (
		uid     = "inv-1"
		meeting = "meet-occ-1"
		projID  = "sfid-1"
		slug    = "my-project"
		oldUser = "alice"
	)

	inviteeMappingKey := "v1_past_meeting_invitees." + uid
	oldXrefKey := "v1_participant_by_meeting_user.invitee." + meeting + "." + oldUser
	attendeeXrefKey := "v1_participant_by_meeting_user.attendee." + meeting + "." + oldUser

	oldMapping := buildRegistrantMappingValue(uid, oldUser, meeting)

	objectsKV := &mockKeyValue{}
	mappingsKV := &mockKeyValue{}
	mappingsKV.On("Get", mock.Anything, inviteeMappingKey).Return(mockKeyValueEntry{key: inviteeMappingKey, value: []byte(oldMapping)}, nil)
	mappingsKV.On("Get", mock.Anything, attendeeXrefKey).Return(nil, jetstream.ErrKeyNotFound)
	// Tombstone old xref
	mappingsKV.On("Put", mock.Anything, oldXrefKey, []byte(tombstoneMarker)).Return(uint64(1), nil)
	// Store new mapping (username now empty)
	mappingsKV.On("Put", mock.Anything, inviteeMappingKey, mock.MatchedBy(func(v []byte) bool {
		var d registrantMappingData
		return json.Unmarshal(v, &d) == nil && d.Username == ""
	})).Return(uint64(1), nil)
	// No new xref written (username is empty)

	pub := &stubParticipantPublisher{}
	h := newParticipantHandler(mappingsKV, objectsKV, pub)

	retry := h.handlePastMeetingInviteeUpdate(context.Background(), "test-key", inviteeV1Data(uid, meeting, projID, slug, ""))

	require.False(t, retry)
	require.Len(t, pub.accessDeleteCalls, 1, "cleared username should emit member_remove")
	require.Len(t, pub.participantCalls, 1)
	assert.Equal(t, "updated", pub.participantCalls[0].action)
	assert.Equal(t, "", pub.participantCalls[0].participant.Username)
	mappingsKV.AssertExpectations(t)
}

func TestHandleInviteeUpdate_Update_UsernameChanged(t *testing.T) {
	const (
		uid     = "inv-1"
		meeting = "meet-occ-1"
		projID  = "sfid-1"
		slug    = "my-project"
		oldUser = "alice"
		newUser = "bob"
	)

	inviteeMappingKey := "v1_past_meeting_invitees." + uid
	oldXrefKey := "v1_participant_by_meeting_user.invitee." + meeting + "." + oldUser
	newXrefKey := "v1_participant_by_meeting_user.invitee." + meeting + "." + newUser
	attendeeXrefKeyNewUser := "v1_participant_by_meeting_user.attendee." + meeting + "." + newUser
	attendeeXrefKeyOldUser := "v1_participant_by_meeting_user.attendee." + meeting + "." + oldUser

	oldMapping := buildRegistrantMappingValue(uid, oldUser, meeting)

	objectsKV := &mockKeyValue{}
	mappingsKV := &mockKeyValue{}
	mappingsKV.On("Get", mock.Anything, inviteeMappingKey).Return(mockKeyValueEntry{key: inviteeMappingKey, value: []byte(oldMapping)}, nil)
	mappingsKV.On("Get", mock.Anything, attendeeXrefKeyNewUser).Return(nil, jetstream.ErrKeyNotFound)
	mappingsKV.On("Get", mock.Anything, attendeeXrefKeyOldUser).Return(nil, jetstream.ErrKeyNotFound)
	mappingsKV.On("Put", mock.Anything, oldXrefKey, []byte(tombstoneMarker)).Return(uint64(1), nil)
	mappingsKV.On("Put", mock.Anything, inviteeMappingKey, mock.MatchedBy(func(v []byte) bool {
		var d registrantMappingData
		return json.Unmarshal(v, &d) == nil && d.Username == newUser
	})).Return(uint64(1), nil)
	mappingsKV.On("Put", mock.Anything, newXrefKey, []byte(uid)).Return(uint64(1), nil)

	pub := &stubParticipantPublisher{}
	h := newParticipantHandler(mappingsKV, objectsKV, pub)

	retry := h.handlePastMeetingInviteeUpdate(context.Background(), "test-key", inviteeV1Data(uid, meeting, projID, slug, newUser))

	require.False(t, retry)
	require.Len(t, pub.accessDeleteCalls, 1, "changed username should emit member_remove for old username")
	require.Len(t, pub.participantCalls, 1)
	assert.Equal(t, "updated", pub.participantCalls[0].action)
	assert.Equal(t, newUser, pub.participantCalls[0].participant.Username)
	mappingsKV.AssertExpectations(t)
}

func TestHandleInviteeUpdate_LegacySentinel_NoRemove(t *testing.T) {
	const (
		uid     = "inv-1"
		meeting = "meet-occ-1"
		projID  = "sfid-1"
		slug    = "my-project"
		user    = "alice"
	)

	inviteeMappingKey := "v1_past_meeting_invitees." + uid
	attendeeXrefKey := "v1_participant_by_meeting_user.attendee." + meeting + "." + user
	inviteeXrefKey := "v1_participant_by_meeting_user.invitee." + meeting + "." + user

	objectsKV := &mockKeyValue{}
	mappingsKV := &mockKeyValue{}
	// Old "1" sentinel — unknown old username
	mappingsKV.On("Get", mock.Anything, inviteeMappingKey).Return(mockKeyValueEntry{key: inviteeMappingKey, value: []byte("1")}, nil)
	mappingsKV.On("Get", mock.Anything, attendeeXrefKey).Return(nil, jetstream.ErrKeyNotFound)
	mappingsKV.On("Put", mock.Anything, inviteeMappingKey, mock.MatchedBy(func(v []byte) bool {
		var d registrantMappingData
		return json.Unmarshal(v, &d) == nil && d.Username == user
	})).Return(uint64(1), nil)
	mappingsKV.On("Put", mock.Anything, inviteeXrefKey, []byte(uid)).Return(uint64(1), nil)

	pub := &stubParticipantPublisher{}
	h := newParticipantHandler(mappingsKV, objectsKV, pub)

	retry := h.handlePastMeetingInviteeUpdate(context.Background(), "test-key", inviteeV1Data(uid, meeting, projID, slug, user))

	require.False(t, retry)
	require.Len(t, pub.accessDeleteCalls, 0, "legacy sentinel cannot trigger remove — old username unknown")
	mappingsKV.AssertExpectations(t)
}

func TestHandleInviteeUpdate_TransientMappingRead_NAK(t *testing.T) {
	const (
		uid     = "inv-1"
		meeting = "meet-occ-1"
		projID  = "sfid-1"
		slug    = "my-project"
		user    = "alice"
	)

	inviteeMappingKey := "v1_past_meeting_invitees." + uid
	// The handler checks the attendee xref before reading the invitee mapping key.
	attendeeXrefKey := "v1_participant_by_meeting_user.attendee." + meeting + "." + user

	objectsKV := &mockKeyValue{}
	mappingsKV := &mockKeyValue{}
	// Attendee xref lookup returns not-found (no cross-reference to preserve)
	mappingsKV.On("Get", mock.Anything, attendeeXrefKey).Return(nil, jetstream.ErrKeyNotFound)
	// Transient KV error (not ErrKeyNotFound) on the invitee mapping key
	mappingsKV.On("Get", mock.Anything, inviteeMappingKey).Return(nil, errors.New("connection timeout"))

	pub := &stubParticipantPublisher{}
	h := newParticipantHandler(mappingsKV, objectsKV, pub)

	retry := h.handlePastMeetingInviteeUpdate(context.Background(), "test-key", inviteeV1Data(uid, meeting, projID, slug, user))

	require.True(t, retry, "transient KV read error should NAK for retry")
	require.Len(t, pub.participantCalls, 0, "should not publish when mapping is unreadable")
	require.Len(t, pub.accessDeleteCalls, 0)
}

func TestHandleInviteeUpdate_RemovePublishFailure_Transient_NAK(t *testing.T) {
	const (
		uid     = "inv-1"
		meeting = "meet-occ-1"
		projID  = "sfid-1"
		slug    = "my-project"
		oldUser = "alice"
	)

	inviteeMappingKey := "v1_past_meeting_invitees." + uid
	oldMapping := buildRegistrantMappingValue(uid, oldUser, meeting)
	attendeeXrefKey := "v1_participant_by_meeting_user.attendee." + meeting + "." + oldUser

	objectsKV := &mockKeyValue{}
	mappingsKV := &mockKeyValue{}
	mappingsKV.On("Get", mock.Anything, inviteeMappingKey).Return(mockKeyValueEntry{key: inviteeMappingKey, value: []byte(oldMapping)}, nil)
	mappingsKV.On("Get", mock.Anything, attendeeXrefKey).Return(nil, jetstream.ErrKeyNotFound)

	pub := &stubParticipantPublisher{accessDeleteErr: errors.New("connection refused")}
	h := newParticipantHandler(mappingsKV, objectsKV, pub)

	retry := h.handlePastMeetingInviteeUpdate(context.Background(), "test-key", inviteeV1Data(uid, meeting, projID, slug, ""))

	require.True(t, retry, "transient member_remove failure should NAK")
	require.Len(t, pub.accessDeleteCalls, 1)
	require.Len(t, pub.participantCalls, 0, "should not publish when remove fails")
	mappingsKV.AssertNotCalled(t, "Put")
}

// --- Attendee tests ---

func TestHandleAttendeeUpdate_Create_NoRemove(t *testing.T) {
	const (
		uid     = "att-1"
		meeting = "meet-occ-1"
		projID  = "sfid-1"
		slug    = "my-project"
		user    = "alice"
	)

	attendeeMappingKey := "v1_past_meeting_attendees." + uid
	inviteeXrefKey := "v1_participant_by_meeting_user.invitee." + meeting + "." + user
	attendeeXrefKey := "v1_participant_by_meeting_user.attendee." + meeting + "." + user

	objectsKV := &mockKeyValue{}
	mappingsKV := &mockKeyValue{}
	mappingsKV.On("Get", mock.Anything, attendeeMappingKey).Return(nil, jetstream.ErrKeyNotFound)
	mappingsKV.On("Get", mock.Anything, inviteeXrefKey).Return(nil, jetstream.ErrKeyNotFound)
	mappingsKV.On("Put", mock.Anything, attendeeMappingKey, mock.MatchedBy(func(v []byte) bool {
		var d registrantMappingData
		return json.Unmarshal(v, &d) == nil && d.UID == uid && d.Username == user && d.MeetingID == meeting
	})).Return(uint64(1), nil)
	mappingsKV.On("Put", mock.Anything, attendeeXrefKey, []byte(uid)).Return(uint64(1), nil)

	pub := &stubParticipantPublisher{}
	h := newParticipantHandler(mappingsKV, objectsKV, pub)

	retry := h.handlePastMeetingAttendeeUpdate(context.Background(), "test-key", attendeeV1Data(uid, meeting, projID, slug, user))

	require.False(t, retry)
	require.Len(t, pub.accessDeleteCalls, 0, "create should not emit member_remove")
	require.Len(t, pub.participantCalls, 1)
	assert.Equal(t, "created", pub.participantCalls[0].action)
	mappingsKV.AssertExpectations(t)
}

func TestHandleAttendeeUpdate_Update_UsernameCleared(t *testing.T) {
	const (
		uid     = "att-1"
		meeting = "meet-occ-1"
		projID  = "sfid-1"
		slug    = "my-project"
		oldUser = "alice"
	)

	attendeeMappingKey := "v1_past_meeting_attendees." + uid
	oldXrefKey := "v1_participant_by_meeting_user.attendee." + meeting + "." + oldUser
	inviteeXrefKey := "v1_participant_by_meeting_user.invitee." + meeting + "." + oldUser

	oldMapping := buildRegistrantMappingValue(uid, oldUser, meeting)

	objectsKV := &mockKeyValue{}
	mappingsKV := &mockKeyValue{}
	mappingsKV.On("Get", mock.Anything, attendeeMappingKey).Return(mockKeyValueEntry{key: attendeeMappingKey, value: []byte(oldMapping)}, nil)
	mappingsKV.On("Get", mock.Anything, inviteeXrefKey).Return(nil, jetstream.ErrKeyNotFound)
	mappingsKV.On("Put", mock.Anything, oldXrefKey, []byte(tombstoneMarker)).Return(uint64(1), nil)
	mappingsKV.On("Put", mock.Anything, attendeeMappingKey, mock.MatchedBy(func(v []byte) bool {
		var d registrantMappingData
		return json.Unmarshal(v, &d) == nil && d.Username == ""
	})).Return(uint64(1), nil)

	pub := &stubParticipantPublisher{}
	h := newParticipantHandler(mappingsKV, objectsKV, pub)

	retry := h.handlePastMeetingAttendeeUpdate(context.Background(), "test-key", attendeeV1Data(uid, meeting, projID, slug, ""))

	require.False(t, retry)
	require.Len(t, pub.accessDeleteCalls, 1, "cleared username should emit member_remove")
	require.Len(t, pub.participantCalls, 1)
	assert.Equal(t, "updated", pub.participantCalls[0].action)
	assert.Equal(t, "", pub.participantCalls[0].participant.Username)
	mappingsKV.AssertExpectations(t)
}

func TestHandleAttendeeUpdate_TransientMappingRead_NAK(t *testing.T) {
	const (
		uid     = "att-1"
		meeting = "meet-occ-1"
		projID  = "sfid-1"
		slug    = "my-project"
		user    = "alice"
	)

	attendeeMappingKey := "v1_past_meeting_attendees." + uid
	// The handler checks the invitee xref before reading the attendee mapping key.
	inviteeXrefKey := "v1_participant_by_meeting_user.invitee." + meeting + "." + user

	objectsKV := &mockKeyValue{}
	mappingsKV := &mockKeyValue{}
	// Invitee xref lookup returns not-found (no cross-reference to preserve)
	mappingsKV.On("Get", mock.Anything, inviteeXrefKey).Return(nil, jetstream.ErrKeyNotFound)
	// Transient KV error (not ErrKeyNotFound) on the attendee mapping key
	mappingsKV.On("Get", mock.Anything, attendeeMappingKey).Return(nil, errors.New("connection timeout"))

	pub := &stubParticipantPublisher{}
	h := newParticipantHandler(mappingsKV, objectsKV, pub)

	retry := h.handlePastMeetingAttendeeUpdate(context.Background(), "test-key", attendeeV1Data(uid, meeting, projID, slug, user))

	require.True(t, retry, "transient KV read error should NAK for retry")
	require.Len(t, pub.participantCalls, 0)
	require.Len(t, pub.accessDeleteCalls, 0)
}
