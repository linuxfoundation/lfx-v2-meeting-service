// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
)

// =============================================================================
// participantTestIDMapper
// =============================================================================

// participantTestIDMapper returns a fixed non-empty projectUID so participant conversion
// doesn't short-circuit at the "parent project not found" skip.
type participantTestIDMapper struct{}

func (participantTestIDMapper) MapProjectV1ToV2(_ context.Context, _ string) (string, error) {
	return "project-uid-v2", nil
}
func (participantTestIDMapper) MapProjectV2ToV1(_ context.Context, _ string) (string, error) {
	return "", nil
}
func (participantTestIDMapper) MapCommitteeV1ToV2(_ context.Context, _ string) (string, error) {
	return "", nil
}
func (participantTestIDMapper) MapCommitteeV2ToV1(_ context.Context, _ string) (string, error) {
	return "", nil
}
func (participantTestIDMapper) MapInviteeIDToParticipantV2(_ context.Context, _ string) (string, error) {
	return "", nil
}
func (participantTestIDMapper) MapAttendeeIDToParticipantV2(_ context.Context, _ string) (string, error) {
	return "", nil
}
func (participantTestIDMapper) MapParticipantV2ToInviteeID(_ context.Context, _ string) (string, error) {
	return "", nil
}
func (participantTestIDMapper) MapParticipantV2ToAttendeeID(_ context.Context, _ string) (string, error) {
	return "", nil
}

// =============================================================================
// mockParticipantPublisher — tracks the two publish methods used in participant tests
// =============================================================================

// mockParticipantPublisher is a focused mock for participant handler tests.
// It tracks PublishPastMeetingParticipantEvent and PublishAccessDelete via testify expectations,
// and silently no-ops all other EventPublisher methods.
type mockParticipantPublisher struct{ mock.Mock }

func (m *mockParticipantPublisher) PublishPastMeetingParticipantEvent(ctx context.Context, action string, data *models.PastMeetingParticipantEventData) error {
	return m.Called(ctx, action, data).Error(0)
}
func (m *mockParticipantPublisher) PublishAccessDelete(ctx context.Context, subject string, payload []byte) error {
	return m.Called(ctx, subject, payload).Error(0)
}
func (m *mockParticipantPublisher) PublishIndexerDelete(_ context.Context, _, _ string) error {
	return nil
}
func (m *mockParticipantPublisher) PublishMeetingEvent(_ context.Context, _ string, _ *models.MeetingEventData) error {
	return nil
}
func (m *mockParticipantPublisher) PublishMeetingHostCredentialsEvent(_ context.Context, _ string, _ *models.MeetingHostCredentialsEventData) error {
	return nil
}
func (m *mockParticipantPublisher) PublishRegistrantEvent(_ context.Context, _ string, _ *models.RegistrantEventData) error {
	return nil
}
func (m *mockParticipantPublisher) PublishInviteResponseEvent(_ context.Context, _ string, _ *models.InviteResponseEventData) error {
	return nil
}
func (m *mockParticipantPublisher) PublishPastMeetingEvent(_ context.Context, _ string, _ *models.PastMeetingEventData) error {
	return nil
}
func (m *mockParticipantPublisher) PublishPastMeetingRecordingEvent(_ context.Context, _ string, _ *models.RecordingEventData) error {
	return nil
}
func (m *mockParticipantPublisher) PublishPastMeetingTranscriptEvent(_ context.Context, _ string, _ *models.TranscriptEventData) error {
	return nil
}
func (m *mockParticipantPublisher) PublishPastMeetingSummaryEvent(_ context.Context, _ string, _ *models.SummaryEventData, _ string) error {
	return nil
}
func (m *mockParticipantPublisher) PublishMeetingAttachmentEvent(_ context.Context, _ string, _ *models.MeetingAttachmentEventData) error {
	return nil
}
func (m *mockParticipantPublisher) PublishPastMeetingAttachmentEvent(_ context.Context, _ string, _ *models.PastMeetingAttachmentEventData) error {
	return nil
}
func (m *mockParticipantPublisher) Close() error { return nil }

// =============================================================================
// v1Data helpers
// =============================================================================

// minimalInviteeV1Data returns the smallest v1Data map that passes invitee conversion.
// Setting both proj_id and project_slug avoids a v1ObjectsKV parent-meeting lookup.
// Omitting committee_id, lf_user_id, and registrant_id avoids those optional KV lookups.
func minimalInviteeV1Data(inviteeID, meetingAndOccID, username string) map[string]interface{} {
	return map[string]interface{}{
		"invitee_id":                inviteeID,
		"meeting_and_occurrence_id": meetingAndOccID,
		"lf_sso":                    username,
		"proj_id":                   "proj-sfid",
		"project_slug":              "my-project",
		"first_name":                "Alice",
		"last_name":                 "Smith",
	}
}

// minimalAttendeeV1Data returns the smallest v1Data map that passes attendee conversion.
func minimalAttendeeV1Data(attendeeID, meetingAndOccID, username string) map[string]interface{} {
	return map[string]interface{}{
		"id":                        attendeeID,
		"meeting_and_occurrence_id": meetingAndOccID,
		"lf_sso":                    username,
		"proj_id":                   "proj-sfid",
		"project_slug":              "my-project",
		"name":                      "Bob Jones",
	}
}

// mustMarshalJSON marshals v to JSON bytes or fails the test.
func mustMarshalJSON(t *testing.T, v interface{}) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("mustMarshalJSON: %v", err)
	}
	return b
}

// newParticipantHandlers wires an EventHandlers with the participantTestIDMapper and
// the supplied mocks, ready for participant handler tests.
func newParticipantHandlers(publisher *mockParticipantPublisher, mappingsKV, objectsKV *mockKeyValue) *EventHandlers {
	return &EventHandlers{
		publisher:    publisher,
		userLookup:   stubV1UserLookup{},
		idMapper:     participantTestIDMapper{},
		v1MappingsKV: mappingsKV,
		v1ObjectsKV:  objectsKV,
		logger:       slog.Default(),
	}
}

// =============================================================================
// Invitee update — username cleared → FGA member_remove
// =============================================================================

// TestHandleInviteeUpdate_UsernameCleared verifies that when a stored username is cleared
// (or changed) and no sibling attendee record exists, a FGA member_remove is published and
// the old invitee xref is tombstoned.
func TestHandleInviteeUpdate_UsernameCleared(t *testing.T) {
	const (
		inviteeUID    = "inv-1"
		meetingAndOcc = "meeting-1_occ-1"
		oldUsername   = "alice"
	)
	storedMapping := buildRegistrantMappingValue(inviteeUID, oldUsername, meetingAndOcc)

	tests := []struct {
		name   string
		v1Data map[string]interface{}
	}{
		{
			name:   "username absent",
			v1Data: minimalInviteeV1Data(inviteeUID, meetingAndOcc, ""),
		},
		{
			name: "username present and empty",
			v1Data: func() map[string]interface{} {
				d := minimalInviteeV1Data(inviteeUID, meetingAndOcc, "")
				d["lf_sso"] = ""
				return d
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mappingsKV := &mockKeyValue{}
			objectsKV := &mockKeyValue{}
			publisher := &mockParticipantPublisher{}

			// Existing mapping carries the old username.
			mappingsKV.On("Get", mock.Anything, "v1_past_meeting_invitees."+inviteeUID).
				Return(mockKeyValueEntry{value: []byte(storedMapping)}, nil)
			// No sibling attendee xref for old username — full revoke path.
			mappingsKV.On("Get", mock.Anything, "v1_participant_by_meeting_user.attendee."+meetingAndOcc+"."+oldUsername).
				Return(nil, jetstream.ErrKeyNotFound)
			// FGA member_remove for old username.
			publisher.On("PublishAccessDelete", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			// Tombstone the stale invitee xref.
			mappingsKV.On("Put", mock.Anything, "v1_participant_by_meeting_user.invitee."+meetingAndOcc+"."+oldUsername, []byte(tombstoneMarker)).
				Return(uint64(1), nil)
			// Publish participant event (new state, empty username) and store updated mapping.
			publisher.On("PublishPastMeetingParticipantEvent", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			mappingsKV.On("Put", mock.Anything, "v1_past_meeting_invitees."+inviteeUID, mock.Anything).
				Return(uint64(2), nil)

			h := newParticipantHandlers(publisher, mappingsKV, objectsKV)
			retry := h.handlePastMeetingInviteeUpdate(context.Background(),
				"itx-zoom-past-meetings-invitees."+inviteeUID, tt.v1Data)

			assert.False(t, retry)
			publisher.AssertNumberOfCalls(t, "PublishAccessDelete", 1)
			publisher.AssertNumberOfCalls(t, "PublishPastMeetingParticipantEvent", 1)
			mappingsKV.AssertExpectations(t)
			publisher.AssertExpectations(t)
		})
	}
}

// =============================================================================
// Invitee update — username unchanged → no FGA revocation
// =============================================================================

// TestHandleInviteeUpdate_UsernameUnchanged verifies that no member_remove is published
// when the username in the update matches the stored mapping.
func TestHandleInviteeUpdate_UsernameUnchanged(t *testing.T) {
	const (
		inviteeUID    = "inv-2"
		meetingAndOcc = "meeting-2_occ-2"
		username      = "alice"
	)
	storedMapping := buildRegistrantMappingValue(inviteeUID, username, meetingAndOcc)

	mappingsKV := &mockKeyValue{}
	objectsKV := &mockKeyValue{}
	publisher := &mockParticipantPublisher{}

	// Sibling merge check — no attendee xref for current username.
	mappingsKV.On("Get", mock.Anything, "v1_participant_by_meeting_user.attendee."+meetingAndOcc+"."+username).
		Return(nil, jetstream.ErrKeyNotFound)
	// Existing mapping with same username — no revocation.
	mappingsKV.On("Get", mock.Anything, "v1_past_meeting_invitees."+inviteeUID).
		Return(mockKeyValueEntry{value: []byte(storedMapping)}, nil)
	publisher.On("PublishPastMeetingParticipantEvent", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mappingsKV.On("Put", mock.Anything, "v1_past_meeting_invitees."+inviteeUID, mock.Anything).
		Return(uint64(2), nil)
	mappingsKV.On("Put", mock.Anything, "v1_participant_by_meeting_user.invitee."+meetingAndOcc+"."+username, mock.Anything).
		Return(uint64(1), nil)

	h := newParticipantHandlers(publisher, mappingsKV, objectsKV)
	retry := h.handlePastMeetingInviteeUpdate(context.Background(),
		"itx-zoom-past-meetings-invitees."+inviteeUID,
		minimalInviteeV1Data(inviteeUID, meetingAndOcc, username))

	assert.False(t, retry)
	publisher.AssertNumberOfCalls(t, "PublishAccessDelete", 0)
	publisher.AssertNumberOfCalls(t, "PublishPastMeetingParticipantEvent", 1)
	mappingsKV.AssertExpectations(t)
	publisher.AssertExpectations(t)
}

// =============================================================================
// Attendee update — username cleared → FGA member_remove
// =============================================================================

// TestHandleAttendeeUpdate_UsernameCleared mirrors TestHandleInviteeUpdate_UsernameCleared
// for the attendee side, verifying the same revocation path fires through the unified method.
func TestHandleAttendeeUpdate_UsernameCleared(t *testing.T) {
	const (
		attendeeUID   = "att-1"
		meetingAndOcc = "meeting-3_occ-3"
		oldUsername   = "bob"
	)
	storedMapping := buildRegistrantMappingValue(attendeeUID, oldUsername, meetingAndOcc)

	mappingsKV := &mockKeyValue{}
	objectsKV := &mockKeyValue{}
	publisher := &mockParticipantPublisher{}

	// Existing mapping carries the old username.
	mappingsKV.On("Get", mock.Anything, "v1_past_meeting_attendees."+attendeeUID).
		Return(mockKeyValueEntry{value: []byte(storedMapping)}, nil)
	// No sibling invitee xref for old username — full revoke.
	mappingsKV.On("Get", mock.Anything, "v1_participant_by_meeting_user.invitee."+meetingAndOcc+"."+oldUsername).
		Return(nil, jetstream.ErrKeyNotFound)
	publisher.On("PublishAccessDelete", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	// Tombstone old attendee xref.
	mappingsKV.On("Put", mock.Anything, "v1_participant_by_meeting_user.attendee."+meetingAndOcc+"."+oldUsername, []byte(tombstoneMarker)).
		Return(uint64(1), nil)
	publisher.On("PublishPastMeetingParticipantEvent", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mappingsKV.On("Put", mock.Anything, "v1_past_meeting_attendees."+attendeeUID, mock.Anything).
		Return(uint64(2), nil)

	h := newParticipantHandlers(publisher, mappingsKV, objectsKV)
	retry := h.handlePastMeetingAttendeeUpdate(context.Background(),
		"itx-zoom-past-meetings-attendees."+attendeeUID,
		minimalAttendeeV1Data(attendeeUID, meetingAndOcc, ""))

	assert.False(t, retry)
	publisher.AssertNumberOfCalls(t, "PublishAccessDelete", 1)
	publisher.AssertNumberOfCalls(t, "PublishPastMeetingParticipantEvent", 1)
	mappingsKV.AssertExpectations(t)
	publisher.AssertExpectations(t)
}

// =============================================================================
// Attendee update — username unchanged → no FGA revocation
// =============================================================================

func TestHandleAttendeeUpdate_UsernameUnchanged(t *testing.T) {
	const (
		attendeeUID   = "att-2"
		meetingAndOcc = "meeting-4_occ-4"
		username      = "bob"
	)
	storedMapping := buildRegistrantMappingValue(attendeeUID, username, meetingAndOcc)

	mappingsKV := &mockKeyValue{}
	objectsKV := &mockKeyValue{}
	publisher := &mockParticipantPublisher{}

	// Sibling merge check — no invitee xref for current username.
	mappingsKV.On("Get", mock.Anything, "v1_participant_by_meeting_user.invitee."+meetingAndOcc+"."+username).
		Return(nil, jetstream.ErrKeyNotFound)
	// Existing mapping with same username.
	mappingsKV.On("Get", mock.Anything, "v1_past_meeting_attendees."+attendeeUID).
		Return(mockKeyValueEntry{value: []byte(storedMapping)}, nil)
	publisher.On("PublishPastMeetingParticipantEvent", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mappingsKV.On("Put", mock.Anything, "v1_past_meeting_attendees."+attendeeUID, mock.Anything).
		Return(uint64(2), nil)
	mappingsKV.On("Put", mock.Anything, "v1_participant_by_meeting_user.attendee."+meetingAndOcc+"."+username, mock.Anything).
		Return(uint64(1), nil)

	h := newParticipantHandlers(publisher, mappingsKV, objectsKV)
	retry := h.handlePastMeetingAttendeeUpdate(context.Background(),
		"itx-zoom-past-meetings-attendees."+attendeeUID,
		minimalAttendeeV1Data(attendeeUID, meetingAndOcc, username))

	assert.False(t, retry)
	publisher.AssertNumberOfCalls(t, "PublishAccessDelete", 0)
	publisher.AssertNumberOfCalls(t, "PublishPastMeetingParticipantEvent", 1)
	mappingsKV.AssertExpectations(t)
	publisher.AssertExpectations(t)
}

// =============================================================================
// Invitee update — username changed, sibling attendee exists → partial update
// =============================================================================

// TestHandleInviteeUpdate_SiblingExists verifies that when the invitee's username changes
// and a sibling attendee xref exists for the old username, a partial member_put is published
// (not a member_remove), and the old invitee xref is tombstoned.
func TestHandleInviteeUpdate_SiblingExists(t *testing.T) {
	const (
		inviteeUID    = "inv-3"
		attendeeUID   = "att-sib"
		meetingAndOcc = "meeting-5_occ-5"
		oldUsername   = "alice"
		newUsername   = "alice-new"
	)
	storedMapping := buildRegistrantMappingValue(inviteeUID, oldUsername, meetingAndOcc)
	attendeeData := minimalAttendeeV1Data(attendeeUID, meetingAndOcc, oldUsername)
	attendeeJSON := mustMarshalJSON(t, attendeeData)

	mappingsKV := &mockKeyValue{}
	objectsKV := &mockKeyValue{}
	publisher := &mockParticipantPublisher{}

	// Sibling merge check for NEW username — no attendee xref yet.
	mappingsKV.On("Get", mock.Anything, "v1_participant_by_meeting_user.attendee."+meetingAndOcc+"."+newUsername).
		Return(nil, jetstream.ErrKeyNotFound)
	// Existing mapping carries old username.
	mappingsKV.On("Get", mock.Anything, "v1_past_meeting_invitees."+inviteeUID).
		Return(mockKeyValueEntry{value: []byte(storedMapping)}, nil)
	// Sibling attendee xref exists for old username.
	mappingsKV.On("Get", mock.Anything, "v1_participant_by_meeting_user.attendee."+meetingAndOcc+"."+oldUsername).
		Return(mockKeyValueEntry{value: []byte(attendeeUID)}, nil)
	// Fetch sibling attendee object for partial update.
	objectsKV.On("Get", mock.Anything, "itx-zoom-past-meetings-attendees."+attendeeUID).
		Return(mockKeyValueEntry{value: attendeeJSON}, nil)
	// Two participant events: sibling partial member_put + own event.
	publisher.On("PublishPastMeetingParticipantEvent", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	// Tombstone stale invitee xref for old username.
	mappingsKV.On("Put", mock.Anything, "v1_participant_by_meeting_user.invitee."+meetingAndOcc+"."+oldUsername, []byte(tombstoneMarker)).
		Return(uint64(1), nil)
	// Store updated mapping and new xref.
	mappingsKV.On("Put", mock.Anything, "v1_past_meeting_invitees."+inviteeUID, mock.Anything).
		Return(uint64(2), nil)
	mappingsKV.On("Put", mock.Anything, "v1_participant_by_meeting_user.invitee."+meetingAndOcc+"."+newUsername, mock.Anything).
		Return(uint64(1), nil)

	h := newParticipantHandlers(publisher, mappingsKV, objectsKV)
	retry := h.handlePastMeetingInviteeUpdate(context.Background(),
		"itx-zoom-past-meetings-invitees."+inviteeUID,
		minimalInviteeV1Data(inviteeUID, meetingAndOcc, newUsername))

	assert.False(t, retry)
	// Sibling partial update used — member_remove must NOT fire.
	publisher.AssertNumberOfCalls(t, "PublishAccessDelete", 0)
	// Partial member_put (sibling) + own participant event.
	publisher.AssertNumberOfCalls(t, "PublishPastMeetingParticipantEvent", 2)
	mappingsKV.AssertExpectations(t)
	publisher.AssertExpectations(t)
}

// =============================================================================
// Skip when parent project not in mappings
// =============================================================================

// TestHandleInviteeUpdate_ProjectNotFound verifies that when the id mapper cannot resolve
// the project UID (returns ""), the handler skips without publishing anything.
func TestHandleInviteeUpdate_ProjectNotFound(t *testing.T) {
	const (
		inviteeUID    = "inv-4"
		meetingAndOcc = "meeting-6_occ-6"
		username      = "carol"
	)

	mappingsKV := &mockKeyValue{}
	objectsKV := &mockKeyValue{}
	publisher := &mockParticipantPublisher{}

	// stubIDMapper returns ("", nil) for MapProjectV1ToV2 → handler skips.
	h := &EventHandlers{
		publisher:    publisher,
		userLookup:   stubV1UserLookup{},
		idMapper:     stubIDMapper{},
		v1MappingsKV: mappingsKV,
		v1ObjectsKV:  objectsKV,
		logger:       slog.Default(),
	}

	retry := h.handlePastMeetingInviteeUpdate(context.Background(),
		"itx-zoom-past-meetings-invitees."+inviteeUID,
		minimalInviteeV1Data(inviteeUID, meetingAndOcc, username))

	assert.False(t, retry)
	publisher.AssertNumberOfCalls(t, "PublishPastMeetingParticipantEvent", 0)
	publisher.AssertNumberOfCalls(t, "PublishAccessDelete", 0)
}
