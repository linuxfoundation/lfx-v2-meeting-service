// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"testing"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

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
func (m *mockParticipantPublisher) PublishIndexerDelete(ctx context.Context, subject, id string) error {
	return m.Called(ctx, subject, id).Error(0)
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
		"last_name":                 "Example",
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
		"name":                      "Bob Fixture",
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
			// Delete the key entirely so this case genuinely differs from
			// "username present and empty" — both shapes must trigger revocation.
			name: "username absent",
			v1Data: func() map[string]interface{} {
				d := minimalInviteeV1Data(inviteeUID, meetingAndOcc, "")
				delete(d, "lf_sso")
				return d
			}(),
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

// =============================================================================
// Current-username sibling merge (mergeSibling called for same username)
// =============================================================================

// TestHandleInviteeUpdate_MergesAttendeeFields verifies the core late-arrival scenario:
// when an invitee update arrives after the attendee record is already in place,
// the handler merges attendee-only fields (isUnknown etc.) into the invitee record
// so they are not lost. This exercises the mergeSibling closure end-to-end.
func TestHandleInviteeUpdate_MergesAttendeeFields(t *testing.T) {
	const (
		inviteeUID    = "inv-merge-1"
		attendeeUID   = "att-merge-1"
		meetingAndOcc = "meeting-merge-1_occ-1"
		username      = "alice"
	)
	// Attendee already processed: has isUnknown=true which the invitee record should inherit.
	attendeeData := map[string]interface{}{
		"id":                        attendeeUID,
		"meeting_and_occurrence_id": meetingAndOcc,
		"lf_sso":                    username,
		"proj_id":                   "proj-sfid",
		"project_slug":              "my-project",
		"name":                      "Alice Example",
		"is_unknown":                true,
		"zoom_user_name":            "Alice Z",
		"mapped_invitee_name":       "Alice Example",
	}
	attendeeJSON := mustMarshalJSON(t, attendeeData)

	mappingsKV := &mockKeyValue{}
	objectsKV := &mockKeyValue{}
	publisher := &mockParticipantPublisher{}

	// Sibling merge step: attendee xref exists for current username.
	mappingsKV.On("Get", mock.Anything, "v1_participant_by_meeting_user.attendee."+meetingAndOcc+"."+username).
		Return(mockKeyValueEntry{value: []byte(attendeeUID)}, nil)
	// Fetch sibling attendee for field merge.
	objectsKV.On("Get", mock.Anything, "itx-zoom-past-meetings-attendees."+attendeeUID).
		Return(mockKeyValueEntry{value: attendeeJSON}, nil)
	// First time create (no prior mapping).
	mappingsKV.On("Get", mock.Anything, "v1_past_meeting_invitees."+inviteeUID).
		Return(nil, jetstream.ErrKeyNotFound)
	// Capture the participant event to assert IsAttended=true and IsUnknown=true.
	var captured *models.PastMeetingParticipantEventData
	publisher.On("PublishPastMeetingParticipantEvent", mock.Anything, mock.Anything, mock.MatchedBy(func(p *models.PastMeetingParticipantEventData) bool {
		captured = p
		return true
	})).Return(nil)
	mappingsKV.On("Put", mock.Anything, "v1_past_meeting_invitees."+inviteeUID, mock.Anything).
		Return(uint64(1), nil)
	mappingsKV.On("Put", mock.Anything, "v1_participant_by_meeting_user.invitee."+meetingAndOcc+"."+username, mock.Anything).
		Return(uint64(1), nil)

	h := newParticipantHandlers(publisher, mappingsKV, objectsKV)
	retry := h.handlePastMeetingInviteeUpdate(context.Background(),
		"itx-zoom-past-meetings-invitees."+inviteeUID,
		minimalInviteeV1Data(inviteeUID, meetingAndOcc, username))

	assert.False(t, retry)
	require.NotNil(t, captured)
	// Invitee-side flags are set, PLUS attendee-only fields are merged in.
	assert.True(t, captured.IsInvited, "invitee record must be IsInvited")
	assert.True(t, captured.IsAttended, "attendee sibling's IsAttended must be merged")
	assert.True(t, captured.IsUnknown, "is_unknown from attendee sibling must be merged")
	assert.Equal(t, "Alice Z", captured.ZoomUserName, "zoom_user_name from sibling must be merged")
	publisher.AssertExpectations(t)
	mappingsKV.AssertExpectations(t)
}

// TestHandleInviteeUpdate_MergeSibling_TransientKVError verifies that a transient KV failure
// reading the attendee sibling object causes the handler to retry rather than publish a
// zero-valued invitee record that silently drops IsUnknown / IsAIReconciled / etc.
func TestHandleInviteeUpdate_MergeSibling_TransientKVError(t *testing.T) {
	const (
		inviteeUID    = "inv-merge-transient"
		attendeeUID   = "att-merge-transient"
		meetingAndOcc = "meeting-transient_occ-1"
		username      = "alice"
	)

	transientErr := fmt.Errorf("nats: connection timeout fetching attendee object")

	mappingsKV := &mockKeyValue{}
	objectsKV := &mockKeyValue{}
	publisher := &mockParticipantPublisher{}

	// Sibling xref exists — the merge closure will be called.
	mappingsKV.On("Get", mock.Anything, "v1_participant_by_meeting_user.attendee."+meetingAndOcc+"."+username).
		Return(mockKeyValueEntry{value: []byte(attendeeUID)}, nil)
	// KV read for the sibling object fails transiently.
	objectsKV.On("Get", mock.Anything, "itx-zoom-past-meetings-attendees."+attendeeUID).
		Return(nil, transientErr)

	h := newParticipantHandlers(publisher, mappingsKV, objectsKV)
	retry := h.handlePastMeetingInviteeUpdate(context.Background(),
		"itx-zoom-past-meetings-invitees."+inviteeUID,
		minimalInviteeV1Data(inviteeUID, meetingAndOcc, username))

	// Must retry — must NOT publish with zero-valued attendee fields.
	assert.True(t, retry, "transient sibling object read must trigger retry")
	publisher.AssertNotCalled(t, "PublishPastMeetingParticipantEvent")
	mappingsKV.AssertExpectations(t)
	objectsKV.AssertExpectations(t)
}

// TestHandleAttendeeUpdate_MergesInviteeFlag verifies that when an attendee update arrives
// after the invitee record is already in place, IsInvited=true is merged into the attendee
// record (attendee-side mergeSibling: just a flag, no field copying).
func TestHandleAttendeeUpdate_MergesInviteeFlag(t *testing.T) {
	const (
		attendeeUID   = "att-merge-2"
		meetingAndOcc = "meeting-merge-2_occ-2"
		username      = "bob"
	)

	mappingsKV := &mockKeyValue{}
	objectsKV := &mockKeyValue{}
	publisher := &mockParticipantPublisher{}

	// Sibling merge step: invitee xref exists for current username.
	mappingsKV.On("Get", mock.Anything, "v1_participant_by_meeting_user.invitee."+meetingAndOcc+"."+username).
		Return(mockKeyValueEntry{value: []byte("inv-merge-2")}, nil)
	// First time create.
	mappingsKV.On("Get", mock.Anything, "v1_past_meeting_attendees."+attendeeUID).
		Return(nil, jetstream.ErrKeyNotFound)
	var captured *models.PastMeetingParticipantEventData
	publisher.On("PublishPastMeetingParticipantEvent", mock.Anything, mock.Anything, mock.MatchedBy(func(p *models.PastMeetingParticipantEventData) bool {
		captured = p
		return true
	})).Return(nil)
	mappingsKV.On("Put", mock.Anything, "v1_past_meeting_attendees."+attendeeUID, mock.Anything).
		Return(uint64(1), nil)
	mappingsKV.On("Put", mock.Anything, "v1_participant_by_meeting_user.attendee."+meetingAndOcc+"."+username, mock.Anything).
		Return(uint64(1), nil)

	h := newParticipantHandlers(publisher, mappingsKV, objectsKV)
	retry := h.handlePastMeetingAttendeeUpdate(context.Background(),
		"itx-zoom-past-meetings-attendees."+attendeeUID,
		minimalAttendeeV1Data(attendeeUID, meetingAndOcc, username))

	assert.False(t, retry)
	require.NotNil(t, captured)
	assert.True(t, captured.IsAttended, "attendee record must be IsAttended")
	assert.True(t, captured.IsInvited, "invitee sibling's flag must be merged into attendee record")
	publisher.AssertExpectations(t)
	mappingsKV.AssertExpectations(t)
}

// TestHandleAttendeeUpdate_SiblingExists verifies the attendee-side username-change path:
// when an attendee's username changes and a sibling invitee xref exists for the old username,
// a partial member_put is published for the invitee with IsInvited=true, IsAttended=false
// (not a member_remove), confirming attendee-side setSiblingFlags is correct.
func TestHandleAttendeeUpdate_SiblingExists(t *testing.T) {
	const (
		attendeeUID   = "att-sib-2"
		inviteeUID    = "inv-sib-2"
		meetingAndOcc = "meeting-7_occ-7"
		oldUsername   = "bob"
		newUsername   = "bob-new"
	)
	storedMapping := buildRegistrantMappingValue(attendeeUID, oldUsername, meetingAndOcc)
	inviteeData := minimalInviteeV1Data(inviteeUID, meetingAndOcc, oldUsername)
	inviteeJSON := mustMarshalJSON(t, inviteeData)

	mappingsKV := &mockKeyValue{}
	objectsKV := &mockKeyValue{}
	publisher := &mockParticipantPublisher{}

	// Sibling merge check for NEW username — no invitee xref yet.
	mappingsKV.On("Get", mock.Anything, "v1_participant_by_meeting_user.invitee."+meetingAndOcc+"."+newUsername).
		Return(nil, jetstream.ErrKeyNotFound)
	// Existing mapping carries old username.
	mappingsKV.On("Get", mock.Anything, "v1_past_meeting_attendees."+attendeeUID).
		Return(mockKeyValueEntry{value: []byte(storedMapping)}, nil)
	// Sibling invitee xref exists for old username.
	mappingsKV.On("Get", mock.Anything, "v1_participant_by_meeting_user.invitee."+meetingAndOcc+"."+oldUsername).
		Return(mockKeyValueEntry{value: []byte(inviteeUID)}, nil)
	// Fetch sibling invitee for partial update.
	objectsKV.On("Get", mock.Anything, "itx-zoom-past-meetings-invitees."+inviteeUID).
		Return(mockKeyValueEntry{value: inviteeJSON}, nil)
	// Capture the first call only — the sibling invitee partial member_put.
	// The second call is the own attendee participant event (IsAttended=true).
	var capturedSibling *models.PastMeetingParticipantEventData
	publisher.On("PublishPastMeetingParticipantEvent", mock.Anything, mock.Anything, mock.MatchedBy(func(p *models.PastMeetingParticipantEventData) bool {
		if capturedSibling == nil {
			capturedSibling = p
		}
		return true
	})).Return(nil)
	// Tombstone old attendee xref for old username.
	mappingsKV.On("Put", mock.Anything, "v1_participant_by_meeting_user.attendee."+meetingAndOcc+"."+oldUsername, []byte(tombstoneMarker)).
		Return(uint64(1), nil)
	mappingsKV.On("Put", mock.Anything, "v1_past_meeting_attendees."+attendeeUID, mock.Anything).
		Return(uint64(2), nil)
	mappingsKV.On("Put", mock.Anything, "v1_participant_by_meeting_user.attendee."+meetingAndOcc+"."+newUsername, mock.Anything).
		Return(uint64(1), nil)

	h := newParticipantHandlers(publisher, mappingsKV, objectsKV)
	retry := h.handlePastMeetingAttendeeUpdate(context.Background(),
		"itx-zoom-past-meetings-attendees."+attendeeUID,
		minimalAttendeeV1Data(attendeeUID, meetingAndOcc, newUsername))

	assert.False(t, retry)
	publisher.AssertNumberOfCalls(t, "PublishAccessDelete", 0)
	publisher.AssertNumberOfCalls(t, "PublishPastMeetingParticipantEvent", 2)
	// Verify attendee-side flag polarity on the sibling (invitee) partial member_put.
	require.NotNil(t, capturedSibling)
	assert.True(t, capturedSibling.IsInvited, "sibling invitee must retain IsInvited=true")
	assert.False(t, capturedSibling.IsAttended, "sibling invitee must have IsAttended=false")
	mappingsKV.AssertExpectations(t)
	publisher.AssertExpectations(t)
}

// =============================================================================
// Delete handlers
// =============================================================================

// TestHandleInviteeDelete_FullDelete verifies that when an invitee is deleted with no
// surviving sibling attendee, the handler publishes an indexer delete + FGA member_remove
// and tombstones the own mapping and xref.
func TestHandleInviteeDelete_FullDelete(t *testing.T) {
	const (
		inviteeUID    = "inv-del-1"
		meetingAndOcc = "meeting-del-1_occ-1"
		username      = "alice"
	)

	mappingsKV := &mockKeyValue{}
	objectsKV := &mockKeyValue{}
	publisher := &mockParticipantPublisher{}

	// isTombstoned check — not tombstoned.
	mappingsKV.On("Get", mock.Anything, "v1_past_meeting_invitees."+inviteeUID).
		Return(mockKeyValueEntry{value: []byte("some-mapping")}, nil)
	// No sibling attendee xref — full delete path.
	mappingsKV.On("Get", mock.Anything, "v1_participant_by_meeting_user.attendee."+meetingAndOcc+"."+username).
		Return(nil, jetstream.ErrKeyNotFound)
	// Indexer delete for own record.
	publisher.On("PublishIndexerDelete", mock.Anything, "lfx.index.v1_past_meeting_participant", inviteeUID).Return(nil)
	// FGA member_remove.
	publisher.On("PublishAccessDelete", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	// Tombstone own mapping key.
	mappingsKV.On("Put", mock.Anything, "v1_past_meeting_invitees."+inviteeUID, []byte(tombstoneMarker)).
		Return(uint64(1), nil)
	// Tombstone own xref.
	mappingsKV.On("Put", mock.Anything, "v1_participant_by_meeting_user.invitee."+meetingAndOcc+"."+username, []byte(tombstoneMarker)).
		Return(uint64(1), nil)

	h := newParticipantHandlers(publisher, mappingsKV, objectsKV)
	v1Data := map[string]interface{}{
		"lf_sso":                    username,
		"meeting_and_occurrence_id": meetingAndOcc,
	}
	retry := h.handlePastMeetingInviteeDelete(context.Background(),
		"itx-zoom-past-meetings-invitees."+inviteeUID, v1Data)

	assert.False(t, retry)
	publisher.AssertNumberOfCalls(t, "PublishIndexerDelete", 1)
	publisher.AssertNumberOfCalls(t, "PublishAccessDelete", 1)
	publisher.AssertNumberOfCalls(t, "PublishPastMeetingParticipantEvent", 0)
	mappingsKV.AssertExpectations(t)
	publisher.AssertExpectations(t)
}

// TestHandleInviteeDelete_AlreadyTombstoned verifies that a re-delivered delete event
// is skipped without publishing anything when the mapping is already tombstoned.
func TestHandleInviteeDelete_AlreadyTombstoned(t *testing.T) {
	const inviteeUID = "inv-del-2"

	mappingsKV := &mockKeyValue{}
	objectsKV := &mockKeyValue{}
	publisher := &mockParticipantPublisher{}

	// isTombstoned check — already tombstoned, handler must skip.
	mappingsKV.On("Get", mock.Anything, "v1_past_meeting_invitees."+inviteeUID).
		Return(mockKeyValueEntry{value: []byte(tombstoneMarker)}, nil)

	h := newParticipantHandlers(publisher, mappingsKV, objectsKV)
	retry := h.handlePastMeetingInviteeDelete(context.Background(),
		"itx-zoom-past-meetings-invitees."+inviteeUID, nil)

	assert.False(t, retry)
	publisher.AssertNumberOfCalls(t, "PublishIndexerDelete", 0)
	publisher.AssertNumberOfCalls(t, "PublishAccessDelete", 0)
	mappingsKV.AssertExpectations(t)
}

// TestHandleInviteeDelete_SiblingExists verifies that when a sibling attendee record
// exists, a partial delete is applied: the own record is deleted from the indexer but
// the sibling's state is published as an update so access is preserved.
func TestHandleInviteeDelete_SiblingExists(t *testing.T) {
	const (
		inviteeUID    = "inv-del-3"
		attendeeUID   = "att-sibling-del"
		meetingAndOcc = "meeting-del-3_occ-3"
		username      = "alice"
	)
	attendeeData := minimalAttendeeV1Data(attendeeUID, meetingAndOcc, username)
	attendeeJSON := mustMarshalJSON(t, attendeeData)

	mappingsKV := &mockKeyValue{}
	objectsKV := &mockKeyValue{}
	publisher := &mockParticipantPublisher{}

	// isTombstoned check — not tombstoned.
	mappingsKV.On("Get", mock.Anything, "v1_past_meeting_invitees."+inviteeUID).
		Return(mockKeyValueEntry{value: []byte("some-mapping")}, nil)
	// Sibling attendee xref exists — partial delete path.
	mappingsKV.On("Get", mock.Anything, "v1_participant_by_meeting_user.attendee."+meetingAndOcc+"."+username).
		Return(mockKeyValueEntry{value: []byte(attendeeUID)}, nil)
	// Fetch surviving sibling from objects KV.
	objectsKV.On("Get", mock.Anything, "itx-zoom-past-meetings-attendees."+attendeeUID).
		Return(mockKeyValueEntry{value: attendeeJSON}, nil)
	// Indexer delete for own (invitee) record.
	publisher.On("PublishIndexerDelete", mock.Anything, "lfx.index.v1_past_meeting_participant", inviteeUID).Return(nil)
	// Sibling attendee published as update with IsInvited=false, IsAttended=true.
	publisher.On("PublishPastMeetingParticipantEvent", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	// Tombstone own mapping and xref; sibling's records are left intact.
	mappingsKV.On("Put", mock.Anything, "v1_past_meeting_invitees."+inviteeUID, []byte(tombstoneMarker)).
		Return(uint64(1), nil)
	mappingsKV.On("Put", mock.Anything, "v1_participant_by_meeting_user.invitee."+meetingAndOcc+"."+username, []byte(tombstoneMarker)).
		Return(uint64(1), nil)

	h := newParticipantHandlers(publisher, mappingsKV, objectsKV)
	v1Data := map[string]interface{}{
		"lf_sso":                    username,
		"meeting_and_occurrence_id": meetingAndOcc,
	}
	retry := h.handlePastMeetingInviteeDelete(context.Background(),
		"itx-zoom-past-meetings-invitees."+inviteeUID, v1Data)

	assert.False(t, retry)
	// Sibling preserves access — member_remove must NOT fire.
	publisher.AssertNumberOfCalls(t, "PublishAccessDelete", 0)
	publisher.AssertNumberOfCalls(t, "PublishIndexerDelete", 1)
	publisher.AssertNumberOfCalls(t, "PublishPastMeetingParticipantEvent", 1)
	mappingsKV.AssertExpectations(t)
	publisher.AssertExpectations(t)
}

// =============================================================================
// Decoder unit tests — field mapping and flag polarity
// =============================================================================

// TestDecodeInviteeRaw verifies that decodeInviteeRaw correctly maps v1 fields to
// rawParticipantData: identity, flags (isInvited=true, isAttended=false, isHost derivation),
// and that attendee-only fields remain zero-valued.
func TestDecodeInviteeRaw(t *testing.T) {
	t.Run("basic field mapping", func(t *testing.T) {
		v1Data := map[string]interface{}{
			"invitee_id":                "inv-field-1",
			"meeting_and_occurrence_id": "mtg_occ-1",
			"meeting_id":                "mtg-1",
			"lf_sso":                    "alice",
			"email":                     "alice@example.com",
			"first_name":                "Alice",
			"last_name":                 "Example",
			"job_title":                 "Engineer",
			"org":                       "Example Foundation",
			"profile_picture":           "https://example.com/pic.jpg",
			"proj_id":                   "proj-sfid",
			"project_slug":              "my-project",
			"created_at":                "2024-01-01T00:00:00Z",
			"modified_at":               "2024-01-02T00:00:00Z",
		}
		objectsKV := &mockKeyValue{}

		raw, err := decodeInviteeRaw(context.Background(), v1Data, objectsKV)

		require.NoError(t, err)
		assert.Equal(t, "inv-field-1", raw.uid)
		assert.Equal(t, "mtg_occ-1", raw.meetingAndOccID)
		assert.Equal(t, "mtg-1", raw.meetingID)
		assert.Equal(t, "alice", raw.username)
		assert.Equal(t, "alice@example.com", raw.email)
		assert.Equal(t, "Alice", raw.firstName)
		assert.Equal(t, "Example", raw.lastName)
		assert.Equal(t, "Engineer", raw.jobTitle)
		assert.Equal(t, "Example Foundation", raw.org)
		assert.Equal(t, "proj-sfid", raw.projectSFID)
		assert.Equal(t, "my-project", raw.projectSlug)
		// Invitee-side flag polarity.
		assert.True(t, raw.isInvited, "invitee must have isInvited=true")
		assert.False(t, raw.isAttended, "invitee must have isAttended=false")
		assert.False(t, raw.isHost, "no registrant_id → isHost=false")
		// Attendee-only fields must be zero.
		assert.False(t, raw.isUnknown)
		assert.False(t, raw.isAIReconciled)
		assert.False(t, raw.isAutoMatched)
		assert.Empty(t, raw.zoomUserName)
		assert.Empty(t, raw.mappedInviteeName)
		assert.Empty(t, raw.sessions)
	})

	t.Run("isHost set when registrant KV marks host=true", func(t *testing.T) {
		v1Data := map[string]interface{}{
			"invitee_id":                "inv-host-1",
			"meeting_and_occurrence_id": "mtg_occ-2",
			"lf_sso":                    "alice",
			"proj_id":                   "proj-sfid",
			"project_slug":              "my-project",
			"registrant_id":             "reg-1",
		}
		registrantJSON := mustMarshalJSON(t, map[string]interface{}{"host": true})
		objectsKV := &mockKeyValue{}
		objectsKV.On("Get", mock.Anything, "itx-zoom-meetings-registrants-v2.reg-1").
			Return(mockKeyValueEntry{value: registrantJSON}, nil)

		raw, err := decodeInviteeRaw(context.Background(), v1Data, objectsKV)

		require.NoError(t, err)
		assert.True(t, raw.isHost, "registrant host=true must set isHost")
		objectsKV.AssertExpectations(t)
	})

	t.Run("returns error when required fields are absent", func(t *testing.T) {
		v1Data := map[string]interface{}{"lf_sso": "alice"} // no invitee_id or meeting_and_occurrence_id
		objectsKV := &mockKeyValue{}

		_, err := decodeInviteeRaw(context.Background(), v1Data, objectsKV)

		require.Error(t, err)
	})
}

// TestDecodeAttendeeRaw verifies that decodeAttendeeRaw correctly maps v1 fields:
// identity, flag polarity (isAttended=true, isInvited from registrant_id presence),
// attendee-only fields, name parsing, and session mapping.
func TestDecodeAttendeeRaw(t *testing.T) {
	t.Run("basic field mapping with attendee-only fields", func(t *testing.T) {
		v1Data := map[string]interface{}{
			"id":                        "att-field-1",
			"meeting_and_occurrence_id": "mtg_occ-3",
			"meeting_id":                "mtg-1",
			"lf_sso":                    "bob",
			"email":                     "bob@example.com",
			"name":                      "Bob Fixture",
			"job_title":                 "Engineer",
			"org":                       "Test Org",
			"proj_id":                   "proj-sfid",
			"project_slug":              "my-project",
			"is_unknown":                true,
			"is_ai_reconciled":          true,
			"is_auto_matched":           true,
			"zoom_user_name":            "Bob Z",
			"mapped_invitee_name":       "Bob Fixture",
			"sessions": []interface{}{
				map[string]interface{}{
					"participant_uuid": "uuid-1",
					"join_time":        "2024-01-01T10:00:00Z",
					"leave_time":       "2024-01-01T11:00:00Z",
					"leave_reason":     "left",
				},
			},
		}

		raw, err := decodeAttendeeRaw(v1Data)

		require.NoError(t, err)
		assert.Equal(t, "att-field-1", raw.uid)
		assert.Equal(t, "mtg_occ-3", raw.meetingAndOccID)
		assert.Equal(t, "bob", raw.username)
		assert.Equal(t, "bob@example.com", raw.email)
		assert.Equal(t, "Bob", raw.firstName, "parseName should split first")
		assert.Equal(t, "Fixture", raw.lastName, "parseName should split last")
		// Attendee-side flag polarity.
		assert.True(t, raw.isAttended, "attendee must have isAttended=true")
		assert.False(t, raw.isInvited, "no registrant_id → isInvited=false")
		assert.False(t, raw.isHost, "attendees always have isHost=false")
		// Attendee-only fields must be populated.
		assert.True(t, raw.isUnknown)
		assert.True(t, raw.isAIReconciled)
		assert.True(t, raw.isAutoMatched)
		assert.Equal(t, "Bob Z", raw.zoomUserName)
		assert.Equal(t, "Bob Fixture", raw.mappedInviteeName)
		require.Len(t, raw.sessions, 1)
		assert.Equal(t, "uuid-1", raw.sessions[0].ParticipantUUID)
		assert.Equal(t, "left", raw.sessions[0].LeaveReason)
	})

	t.Run("isInvited=true when registrant_id present", func(t *testing.T) {
		v1Data := map[string]interface{}{
			"id":                        "att-inv-1",
			"meeting_and_occurrence_id": "mtg_occ-4",
			"registrant_id":             "reg-2",
		}

		raw, err := decodeAttendeeRaw(v1Data)

		require.NoError(t, err)
		assert.True(t, raw.isInvited, "registrant_id present → isInvited=true")
		assert.True(t, raw.isAttended)
	})

	t.Run("returns error when required fields are absent", func(t *testing.T) {
		v1Data := map[string]interface{}{"lf_sso": "bob"} // no id or meeting_and_occurrence_id

		_, err := decodeAttendeeRaw(v1Data)

		require.Error(t, err)
	})
}

// TestHandleAttendeeDelete_FullDelete mirrors TestHandleInviteeDelete_FullDelete for the
// attendee side, confirming the unified delete path uses the correct prefixes for each side.
func TestHandleAttendeeDelete_FullDelete(t *testing.T) {
	const (
		attendeeUID   = "att-del-1"
		meetingAndOcc = "meeting-del-4_occ-4"
		username      = "bob"
	)

	mappingsKV := &mockKeyValue{}
	objectsKV := &mockKeyValue{}
	publisher := &mockParticipantPublisher{}

	// isTombstoned check — not tombstoned.
	mappingsKV.On("Get", mock.Anything, "v1_past_meeting_attendees."+attendeeUID).
		Return(mockKeyValueEntry{value: []byte("some-mapping")}, nil)
	// No sibling invitee xref — full delete path.
	mappingsKV.On("Get", mock.Anything, "v1_participant_by_meeting_user.invitee."+meetingAndOcc+"."+username).
		Return(nil, jetstream.ErrKeyNotFound)
	publisher.On("PublishIndexerDelete", mock.Anything, "lfx.index.v1_past_meeting_participant", attendeeUID).Return(nil)
	publisher.On("PublishAccessDelete", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mappingsKV.On("Put", mock.Anything, "v1_past_meeting_attendees."+attendeeUID, []byte(tombstoneMarker)).
		Return(uint64(1), nil)
	mappingsKV.On("Put", mock.Anything, "v1_participant_by_meeting_user.attendee."+meetingAndOcc+"."+username, []byte(tombstoneMarker)).
		Return(uint64(1), nil)

	h := newParticipantHandlers(publisher, mappingsKV, objectsKV)
	v1Data := map[string]interface{}{
		"lf_sso":                    username,
		"meeting_and_occurrence_id": meetingAndOcc,
	}
	retry := h.handlePastMeetingAttendeeDelete(context.Background(),
		"itx-zoom-past-meetings-attendees."+attendeeUID, v1Data)

	assert.False(t, retry)
	publisher.AssertNumberOfCalls(t, "PublishIndexerDelete", 1)
	publisher.AssertNumberOfCalls(t, "PublishAccessDelete", 1)
	publisher.AssertNumberOfCalls(t, "PublishPastMeetingParticipantEvent", 0)
	mappingsKV.AssertExpectations(t)
	publisher.AssertExpectations(t)
}

// TestHandleAttendeeDelete_SiblingExists mirrors TestHandleInviteeDelete_SiblingExists
// for the attendee side, verifying that the surviving invitee sibling is published with
// IsInvited=true, IsAttended=false (attendee-side setSiblingFlags polarity).
func TestHandleAttendeeDelete_SiblingExists(t *testing.T) {
	const (
		attendeeUID   = "att-del-2"
		inviteeUID    = "inv-sibling-del"
		meetingAndOcc = "meeting-del-5_occ-5"
		username      = "bob"
	)
	inviteeData := minimalInviteeV1Data(inviteeUID, meetingAndOcc, username)
	inviteeJSON := mustMarshalJSON(t, inviteeData)

	mappingsKV := &mockKeyValue{}
	objectsKV := &mockKeyValue{}
	publisher := &mockParticipantPublisher{}

	// isTombstoned check — not tombstoned.
	mappingsKV.On("Get", mock.Anything, "v1_past_meeting_attendees."+attendeeUID).
		Return(mockKeyValueEntry{value: []byte("some-mapping")}, nil)
	// Sibling invitee xref exists — partial delete path.
	mappingsKV.On("Get", mock.Anything, "v1_participant_by_meeting_user.invitee."+meetingAndOcc+"."+username).
		Return(mockKeyValueEntry{value: []byte(inviteeUID)}, nil)
	// Fetch surviving sibling invitee from objects KV.
	objectsKV.On("Get", mock.Anything, "itx-zoom-past-meetings-invitees."+inviteeUID).
		Return(mockKeyValueEntry{value: inviteeJSON}, nil)
	// Indexer delete for own (attendee) record.
	publisher.On("PublishIndexerDelete", mock.Anything, "lfx.index.v1_past_meeting_participant", attendeeUID).Return(nil)
	// Capture sibling update to assert attendee-side flag polarity.
	var capturedSibling *models.PastMeetingParticipantEventData
	publisher.On("PublishPastMeetingParticipantEvent", mock.Anything, mock.Anything, mock.MatchedBy(func(p *models.PastMeetingParticipantEventData) bool {
		capturedSibling = p
		return true
	})).Return(nil)
	mappingsKV.On("Put", mock.Anything, "v1_past_meeting_attendees."+attendeeUID, []byte(tombstoneMarker)).
		Return(uint64(1), nil)
	mappingsKV.On("Put", mock.Anything, "v1_participant_by_meeting_user.attendee."+meetingAndOcc+"."+username, []byte(tombstoneMarker)).
		Return(uint64(1), nil)

	h := newParticipantHandlers(publisher, mappingsKV, objectsKV)
	v1Data := map[string]interface{}{
		"lf_sso":                    username,
		"meeting_and_occurrence_id": meetingAndOcc,
	}
	retry := h.handlePastMeetingAttendeeDelete(context.Background(),
		"itx-zoom-past-meetings-attendees."+attendeeUID, v1Data)

	assert.False(t, retry)
	// Sibling preserves access — member_remove must NOT fire.
	publisher.AssertNumberOfCalls(t, "PublishAccessDelete", 0)
	publisher.AssertNumberOfCalls(t, "PublishIndexerDelete", 1)
	publisher.AssertNumberOfCalls(t, "PublishPastMeetingParticipantEvent", 1)
	// Confirm attendee-side setSiblingFlags polarity: invitee stays invited but not attended.
	require.NotNil(t, capturedSibling)
	assert.True(t, capturedSibling.IsInvited, "surviving invitee must retain IsInvited=true")
	assert.False(t, capturedSibling.IsAttended, "surviving invitee must have IsAttended=false")
	mappingsKV.AssertExpectations(t)
	publisher.AssertExpectations(t)
}

// TestHandleInviteeDelete_HardDelete verifies the nil-v1Data (hard NATS delete) path:
// username and meetingAndOccurrenceID are recovered from the stored mapping, and the
// full delete proceeds correctly without v1Data being provided.
func TestHandleInviteeDelete_HardDelete(t *testing.T) {
	const (
		inviteeUID    = "inv-hard-del-1"
		meetingAndOcc = "meeting-hard-1_occ-1"
		username      = "alice"
	)
	storedMapping := buildRegistrantMappingValue(inviteeUID, username, meetingAndOcc)

	mappingsKV := &mockKeyValue{}
	objectsKV := &mockKeyValue{}
	publisher := &mockParticipantPublisher{}

	// isTombstoned check — not tombstoned; the same key is then read again to
	// recover username/meetingAndOcc since v1Data is nil (hard-delete shape).
	mappingsKV.On("Get", mock.Anything, "v1_past_meeting_invitees."+inviteeUID).
		Return(mockKeyValueEntry{value: []byte(storedMapping)}, nil)
	// No sibling attendee xref — full delete.
	mappingsKV.On("Get", mock.Anything, "v1_participant_by_meeting_user.attendee."+meetingAndOcc+"."+username).
		Return(nil, jetstream.ErrKeyNotFound)
	publisher.On("PublishIndexerDelete", mock.Anything, "lfx.index.v1_past_meeting_participant", inviteeUID).Return(nil)
	publisher.On("PublishAccessDelete", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mappingsKV.On("Put", mock.Anything, "v1_past_meeting_invitees."+inviteeUID, []byte(tombstoneMarker)).
		Return(uint64(1), nil)
	mappingsKV.On("Put", mock.Anything, "v1_participant_by_meeting_user.invitee."+meetingAndOcc+"."+username, []byte(tombstoneMarker)).
		Return(uint64(1), nil)

	h := newParticipantHandlers(publisher, mappingsKV, objectsKV)
	// v1Data is nil — hard NATS delete shape.
	retry := h.handlePastMeetingInviteeDelete(context.Background(),
		"itx-zoom-past-meetings-invitees."+inviteeUID, nil)

	assert.False(t, retry)
	publisher.AssertNumberOfCalls(t, "PublishIndexerDelete", 1)
	publisher.AssertNumberOfCalls(t, "PublishAccessDelete", 1)
	mappingsKV.AssertExpectations(t)
	publisher.AssertExpectations(t)
}
