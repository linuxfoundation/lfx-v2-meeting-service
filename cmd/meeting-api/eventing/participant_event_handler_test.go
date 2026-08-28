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
// Username-change: transient siblingConvert failure → retry, no member_remove
// =============================================================================

// TestHandleInviteeUpdate_SiblingConvert_TransientError verifies that when the username
// changes, a sibling attendee xref exists for the old username, but siblingConvert fails
// transiently (e.g. project-ID mapping unavailable), the handler returns retry=true and
// does NOT fall through to member_remove — which would incorrectly revoke the sibling's
// surviving access.
func TestHandleInviteeUpdate_SiblingConvert_TransientError(t *testing.T) {
	const (
		inviteeUID    = "inv-sc-transient"
		attendeeUID   = "att-sc-transient"
		meetingAndOcc = "meeting-sc_occ-1"
		oldUsername   = "alice"
		newUsername   = "alice-new"
	)
	storedMapping := buildRegistrantMappingValue(inviteeUID, oldUsername, meetingAndOcc)
	// Attendee data is missing proj_id so that siblingConvert (convertMapToAttendeeParticipantData)
	// will attempt a KV fallback and fail transiently when the parent past_meeting is absent.
	attendeeDataNoProject := map[string]interface{}{
		"id":                        attendeeUID,
		"meeting_and_occurrence_id": meetingAndOcc,
		"lf_sso":                    oldUsername,
		// proj_id intentionally omitted to force resolveProjectFields to hit the KV
	}
	attendeeJSON := mustMarshalJSON(t, attendeeDataNoProject)

	mappingsKV := &mockKeyValue{}
	objectsKV := &mockKeyValue{}
	publisher := &mockParticipantPublisher{}

	// Sibling merge check for new username — no attendee xref.
	mappingsKV.On("Get", mock.Anything, "v1_participant_by_meeting_user.attendee."+meetingAndOcc+"."+newUsername).
		Return(nil, jetstream.ErrKeyNotFound)
	// Existing mapping carries old username.
	mappingsKV.On("Get", mock.Anything, "v1_past_meeting_invitees."+inviteeUID).
		Return(mockKeyValueEntry{value: []byte(storedMapping)}, nil)
	// Sibling attendee xref exists for old username.
	mappingsKV.On("Get", mock.Anything, "v1_participant_by_meeting_user.attendee."+meetingAndOcc+"."+oldUsername).
		Return(mockKeyValueEntry{value: []byte(attendeeUID)}, nil)
	// Fetch sibling attendee object — succeeds, but the object has no proj_id.
	objectsKV.On("Get", mock.Anything, "itx-zoom-past-meetings-attendees."+attendeeUID).
		Return(mockKeyValueEntry{value: attendeeJSON}, nil)
	// siblingConvert will call resolveProjectFields which falls back to the past_meeting KV —
	// return ErrKeyNotFound so the error wraps with "(transient)" and triggers the retry path.
	objectsKV.On("Get", mock.Anything, mock.MatchedBy(func(k string) bool {
		return k != "itx-zoom-past-meetings-attendees."+attendeeUID
	})).Return(nil, jetstream.ErrKeyNotFound)

	h := newParticipantHandlers(publisher, mappingsKV, objectsKV)
	retry := h.handlePastMeetingInviteeUpdate(context.Background(),
		"itx-zoom-past-meetings-invitees."+inviteeUID,
		minimalInviteeV1Data(inviteeUID, meetingAndOcc, newUsername))

	// Must retry — must NOT publish member_remove (which would revoke the sibling's access).
	assert.True(t, retry, "transient siblingConvert failure must trigger retry")
	publisher.AssertNotCalled(t, "PublishAccessDelete")
	publisher.AssertNotCalled(t, "PublishPastMeetingParticipantEvent")
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

// TestHandleInviteeUpdate_MergeSibling_TransientKVError verifies that any KV failure
// reading the attendee sibling object causes the handler to retry rather than publish a
// zero-valued invitee record that silently drops IsUnknown / IsAIReconciled / etc.
// Covers both a generic connection error and context.DeadlineExceeded, whose message
// ("context deadline exceeded") was not matched by isTransientError's keyword list.
func TestHandleInviteeUpdate_MergeSibling_TransientKVError(t *testing.T) {
	const (
		meetingAndOcc = "meeting-transient_occ-1"
		username      = "alice"
	)

	tests := []struct {
		name string
		err  error
	}{
		{name: "connection error", err: fmt.Errorf("nats: connection timeout fetching attendee object")},
		{name: "context deadline exceeded", err: context.DeadlineExceeded},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			inviteeUID := "inv-merge-transient-" + tc.name
			attendeeUID := "att-merge-transient-" + tc.name

			mappingsKV := &mockKeyValue{}
			objectsKV := &mockKeyValue{}
			publisher := &mockParticipantPublisher{}

			// Sibling xref exists — the merge closure will be called.
			mappingsKV.On("Get", mock.Anything, "v1_participant_by_meeting_user.attendee."+meetingAndOcc+"."+username).
				Return(mockKeyValueEntry{value: []byte(attendeeUID)}, nil)
			// KV read for the sibling object fails.
			objectsKV.On("Get", mock.Anything, "itx-zoom-past-meetings-attendees."+attendeeUID).
				Return(nil, tc.err)

			h := newParticipantHandlers(publisher, mappingsKV, objectsKV)
			retry := h.handlePastMeetingInviteeUpdate(context.Background(),
				"itx-zoom-past-meetings-invitees."+inviteeUID,
				minimalInviteeV1Data(inviteeUID, meetingAndOcc, username))

			// Must retry — must NOT publish with zero-valued attendee fields.
			assert.True(t, retry, "sibling object read error must trigger retry")
			publisher.AssertNotCalled(t, "PublishPastMeetingParticipantEvent")
			mappingsKV.AssertExpectations(t)
			objectsKV.AssertExpectations(t)
		})
	}
}

// TestHandleInviteeUpdate_MergeSibling_StaleXref_DoesNotSetIsAttended verifies the primary
// FGA-flag regression guard for the mergeSibling fix: when a sibling attendee xref exists
// but the attendee object itself returns ErrKeyNotFound (stale xref), IsAttended must NOT
// be set and the invitee update must still be published normally with IsAttended=false.
func TestHandleInviteeUpdate_MergeSibling_StaleXref_DoesNotSetIsAttended(t *testing.T) {
	const (
		inviteeUID    = "inv-stale-xref"
		attendeeUID   = "att-gone"
		meetingAndOcc = "meeting-stale_occ-1"
		username      = "alice"
	)

	mappingsKV := &mockKeyValue{}
	objectsKV := &mockKeyValue{}
	publisher := &mockParticipantPublisher{}

	// Sibling xref exists — merge closure will be called.
	mappingsKV.On("Get", mock.Anything, "v1_participant_by_meeting_user.attendee."+meetingAndOcc+"."+username).
		Return(mockKeyValueEntry{value: []byte(attendeeUID)}, nil)
	// Attendee object is gone (stale xref).
	objectsKV.On("Get", mock.Anything, "itx-zoom-past-meetings-attendees."+attendeeUID).
		Return(nil, jetstream.ErrKeyNotFound)
	// First-time create.
	mappingsKV.On("Get", mock.Anything, "v1_past_meeting_invitees."+inviteeUID).
		Return(nil, jetstream.ErrKeyNotFound)
	// Invitee event must still be published.
	var captured *models.PastMeetingParticipantEventData
	publisher.On("PublishPastMeetingParticipantEvent", mock.Anything, mock.Anything, mock.MatchedBy(func(p *models.PastMeetingParticipantEventData) bool {
		captured = p
		return true
	})).Return(nil)
	mappingsKV.On("Put", mock.Anything, mock.Anything, mock.Anything).Return(uint64(1), nil)

	h := newParticipantHandlers(publisher, mappingsKV, objectsKV)
	retry := h.handlePastMeetingInviteeUpdate(context.Background(),
		"itx-zoom-past-meetings-invitees."+inviteeUID,
		minimalInviteeV1Data(inviteeUID, meetingAndOcc, username))

	assert.False(t, retry)
	require.NotNil(t, captured)
	// Core assertion: stale xref must NOT cause IsAttended=true.
	assert.False(t, captured.IsAttended, "stale xref with absent attendee object must not set IsAttended")
	assert.True(t, captured.IsInvited, "invitee own flag must be set")
	publisher.AssertExpectations(t)
}

// TestHandleInviteeUpdate_MergeSibling_CorruptPayload_PublishesWithIsAttended verifies that
// when the attendee sibling object exists but its payload cannot be decoded (corrupt KV
// value), the invitee update is still published with IsAttended=true rather than retrying
// until max-delivery and dropping the event. Attendee-only enrichment fields are zeroed,
// but the FGA-critical flag is preserved.
func TestHandleInviteeUpdate_MergeSibling_CorruptPayload_PublishesWithIsAttended(t *testing.T) {
	const (
		inviteeUID    = "inv-corrupt-sibling"
		attendeeUID   = "att-corrupt-sibling"
		meetingAndOcc = "meeting-corrupt_occ-1"
		username      = "alice"
	)

	mappingsKV := &mockKeyValue{}
	objectsKV := &mockKeyValue{}
	publisher := &mockParticipantPublisher{}

	// Sibling xref exists — merge closure will be called.
	mappingsKV.On("Get", mock.Anything, "v1_participant_by_meeting_user.attendee."+meetingAndOcc+"."+username).
		Return(mockKeyValueEntry{value: []byte(attendeeUID)}, nil)
	// Sibling object present but corrupt (not valid JSON map).
	objectsKV.On("Get", mock.Anything, "itx-zoom-past-meetings-attendees."+attendeeUID).
		Return(mockKeyValueEntry{value: []byte("not-valid-json")}, nil)
	// First-time create.
	mappingsKV.On("Get", mock.Anything, "v1_past_meeting_invitees."+inviteeUID).
		Return(nil, jetstream.ErrKeyNotFound)
	// Invitee update MUST still be published.
	var captured *models.PastMeetingParticipantEventData
	publisher.On("PublishPastMeetingParticipantEvent", mock.Anything, mock.Anything, mock.MatchedBy(func(p *models.PastMeetingParticipantEventData) bool {
		captured = p
		return true
	})).Return(nil)
	mappingsKV.On("Put", mock.Anything, mock.Anything, mock.Anything).Return(uint64(1), nil)

	h := newParticipantHandlers(publisher, mappingsKV, objectsKV)
	retry := h.handlePastMeetingInviteeUpdate(context.Background(),
		"itx-zoom-past-meetings-invitees."+inviteeUID,
		minimalInviteeV1Data(inviteeUID, meetingAndOcc, username))

	// Corrupt payload is permanent — must NOT retry (would exhaust max-delivery).
	assert.False(t, retry, "corrupt sibling payload must not trigger retry")
	// Event must still be published with IsAttended=true (attendee object existence confirmed).
	require.NotNil(t, captured)
	assert.True(t, captured.IsAttended, "IsAttended must be true even when sibling decode fails")
	assert.True(t, captured.IsInvited, "invitee own flag must be set")
	// Attendee-only enrichment fields are zeroed — acceptable degraded mode.
	assert.False(t, captured.IsUnknown, "enrichment field zeroed on decode failure")
	assert.Empty(t, captured.ZoomUserName, "enrichment field zeroed on decode failure")
	publisher.AssertExpectations(t)
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
// ErrInvalidKey treated as absence (not as retry) at all three xref-read sites
// =============================================================================

// TestHandleInviteeUpdate_InvalidKeyUsername_MergeSkipped verifies that when the current
// username contains characters that make an invalid NATS KV key (e.g. spaces), the sibling
// xref Get returns ErrInvalidKey and the handler proceeds without retrying — treating the
// xref as absent and publishing the invitee record normally.
func TestHandleInviteeUpdate_InvalidKeyUsername_MergeSkipped(t *testing.T) {
	const (
		inviteeUID    = "inv-invalid-key"
		meetingAndOcc = "meeting-ik_occ-1"
		username      = "alice with spaces" // spaces → ErrInvalidKey from NATS
	)

	mappingsKV := &mockKeyValue{}
	objectsKV := &mockKeyValue{}
	publisher := &mockParticipantPublisher{}

	// Sibling xref Get fails with ErrInvalidKey because the username contains spaces.
	mappingsKV.On("Get", mock.Anything, "v1_participant_by_meeting_user.attendee."+meetingAndOcc+"."+username).
		Return(nil, jetstream.ErrInvalidKey)
	// First-time create.
	mappingsKV.On("Get", mock.Anything, "v1_past_meeting_invitees."+inviteeUID).
		Return(nil, jetstream.ErrKeyNotFound)
	publisher.On("PublishPastMeetingParticipantEvent", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mappingsKV.On("Put", mock.Anything, mock.Anything, mock.Anything).Return(uint64(1), nil)

	h := newParticipantHandlers(publisher, mappingsKV, objectsKV)
	retry := h.handlePastMeetingInviteeUpdate(context.Background(),
		"itx-zoom-past-meetings-invitees."+inviteeUID,
		minimalInviteeV1Data(inviteeUID, meetingAndOcc, username))

	// ErrInvalidKey must not cause a retry — the event should be processed normally.
	assert.False(t, retry)
	publisher.AssertNumberOfCalls(t, "PublishPastMeetingParticipantEvent", 1)
}

// TestHandleInviteeUpdate_InvalidKeyOldUsername_NoRetry verifies that when the old username
// (recovered from the stored mapping on a username-change event) contains invalid KV key
// characters, the sibling xref Get for the old username returns ErrInvalidKey and is treated
// as absent — the handler falls through to member_remove rather than retrying forever.
func TestHandleInviteeUpdate_InvalidKeyOldUsername_NoRetry(t *testing.T) {
	const (
		inviteeUID    = "inv-invalid-old"
		meetingAndOcc = "meeting-iko_occ-1"
		oldUsername   = "old user with spaces"
		newUsername   = "alice-new"
	)
	storedMapping := buildRegistrantMappingValue(inviteeUID, oldUsername, meetingAndOcc)

	mappingsKV := &mockKeyValue{}
	objectsKV := &mockKeyValue{}
	publisher := &mockParticipantPublisher{}

	// Sibling merge check for new username — no sibling xref.
	mappingsKV.On("Get", mock.Anything, "v1_participant_by_meeting_user.attendee."+meetingAndOcc+"."+newUsername).
		Return(nil, jetstream.ErrKeyNotFound)
	// Existing mapping carries the old (invalid) username.
	mappingsKV.On("Get", mock.Anything, "v1_past_meeting_invitees."+inviteeUID).
		Return(mockKeyValueEntry{value: []byte(storedMapping)}, nil)
	// Old-username sibling xref Get fails with ErrInvalidKey — treated as absent.
	mappingsKV.On("Get", mock.Anything, "v1_participant_by_meeting_user.attendee."+meetingAndOcc+"."+oldUsername).
		Return(nil, jetstream.ErrInvalidKey)
	// Falls through to member_remove (no sibling survives).
	publisher.On("PublishAccessDelete", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	// Tombstone old own-side xref (may also get ErrInvalidKey for the old username, which is tolerated).
	mappingsKV.On("Put", mock.Anything, mock.Anything, mock.Anything).Return(uint64(1), nil)
	publisher.On("PublishPastMeetingParticipantEvent", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	h := newParticipantHandlers(publisher, mappingsKV, objectsKV)
	retry := h.handlePastMeetingInviteeUpdate(context.Background(),
		"itx-zoom-past-meetings-invitees."+inviteeUID,
		minimalInviteeV1Data(inviteeUID, meetingAndOcc, newUsername))

	// ErrInvalidKey on the old-username xref must not retry — must fall through to member_remove.
	assert.False(t, retry)
	publisher.AssertCalled(t, "PublishAccessDelete", mock.Anything, mock.Anything, mock.Anything)
}

// TestHandleInviteeDelete_InvalidKeyUsername_FullDelete verifies that when the username
// contains invalid KV key characters, the sibling xref Get on the delete path returns
// ErrInvalidKey and is treated as absent — the handler proceeds to fullParticipantDelete
// rather than retrying forever.
func TestHandleInviteeDelete_InvalidKeyUsername_FullDelete(t *testing.T) {
	const (
		inviteeUID    = "inv-invalid-del"
		meetingAndOcc = "meeting-ikd_occ-1"
		username      = "del user spaces"
	)
	storedMapping := buildRegistrantMappingValue(inviteeUID, username, meetingAndOcc)

	mappingsKV := &mockKeyValue{}
	objectsKV := &mockKeyValue{}
	publisher := &mockParticipantPublisher{}

	// Own mapping lookup succeeds.
	mappingsKV.On("Get", mock.Anything, "v1_past_meeting_invitees."+inviteeUID).
		Return(mockKeyValueEntry{value: []byte(storedMapping)}, nil)
	// Sibling xref Get on the delete path returns ErrInvalidKey — treated as absent.
	mappingsKV.On("Get", mock.Anything, "v1_participant_by_meeting_user.attendee."+meetingAndOcc+"."+username).
		Return(nil, jetstream.ErrInvalidKey)
	// Proceeds to fullParticipantDelete.
	publisher.On("PublishIndexerDelete", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	publisher.On("PublishAccessDelete", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mappingsKV.On("Put", mock.Anything, mock.Anything, mock.Anything).Return(uint64(1), nil)

	h := newParticipantHandlers(publisher, mappingsKV, objectsKV)
	v1Data := minimalInviteeV1Data(inviteeUID, meetingAndOcc, username)
	retry := h.handlePastMeetingInviteeDelete(context.Background(),
		"itx-zoom-past-meetings-invitees."+inviteeUID, v1Data)

	// ErrInvalidKey on delete-path sibling xref must not retry — full delete must proceed.
	assert.False(t, retry)
	publisher.AssertCalled(t, "PublishIndexerDelete", mock.Anything, mock.Anything, mock.Anything)
	publisher.AssertCalled(t, "PublishAccessDelete", mock.Anything, mock.Anything, mock.Anything)
}

// TestHandleInviteeUpdate_InvalidKeyOldUsername_TombstonePut_ContinuesProcessing verifies
// the data-loss path fixed by this PR: when the old username contains invalid KV key
// characters, the tombstone Put for the old own-side xref returns ErrInvalidKey. Previously
// this flowed through isTransientError → false → ACK, which exited the handler before the
// new participant event was published. The fixed path must skip the tombstone (no-op) and
// continue to publish the new participant event.
func TestHandleInviteeUpdate_InvalidKeyOldUsername_TombstonePut_ContinuesProcessing(t *testing.T) {
	const (
		inviteeUID    = "inv-tombstone-ik"
		attendeeUID   = "att-tombstone-ik"
		meetingAndOcc = "meeting-tik_occ-1"
		oldUsername   = "old user spaces" // invalid KV key characters
		newUsername   = "alice-clean"
	)
	storedMapping := buildRegistrantMappingValue(inviteeUID, oldUsername, meetingAndOcc)
	attendeeData := minimalAttendeeV1Data(attendeeUID, meetingAndOcc, oldUsername)
	attendeeJSON := mustMarshalJSON(t, attendeeData)

	mappingsKV := &mockKeyValue{}
	objectsKV := &mockKeyValue{}
	publisher := &mockParticipantPublisher{}

	// Sibling merge for new username — no sibling xref.
	mappingsKV.On("Get", mock.Anything, "v1_participant_by_meeting_user.attendee."+meetingAndOcc+"."+newUsername).
		Return(nil, jetstream.ErrKeyNotFound)
	// Existing mapping carries the old (invalid) username.
	mappingsKV.On("Get", mock.Anything, "v1_past_meeting_invitees."+inviteeUID).
		Return(mockKeyValueEntry{value: []byte(storedMapping)}, nil)
	// Old-username sibling xref — no sibling exists.
	mappingsKV.On("Get", mock.Anything, "v1_participant_by_meeting_user.attendee."+meetingAndOcc+"."+oldUsername).
		Return(nil, jetstream.ErrKeyNotFound)
	// member_remove published (no sibling survives).
	publisher.On("PublishAccessDelete", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	// Tombstone Put for old own-side xref returns ErrInvalidKey — must be treated as no-op.
	mappingsKV.On("Put", mock.Anything, "v1_participant_by_meeting_user.invitee."+meetingAndOcc+"."+oldUsername, mock.Anything).
		Return(uint64(0), jetstream.ErrInvalidKey)
	// New participant event must still be published.
	publisher.On("PublishPastMeetingParticipantEvent", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	// New mapping and xref writes succeed.
	mappingsKV.On("Put", mock.Anything, "v1_past_meeting_invitees."+inviteeUID, mock.Anything).Return(uint64(1), nil)
	mappingsKV.On("Put", mock.Anything, "v1_participant_by_meeting_user.invitee."+meetingAndOcc+"."+newUsername, mock.Anything).Return(uint64(1), nil)

	_ = attendeeUID
	_ = attendeeJSON
	h := newParticipantHandlers(publisher, mappingsKV, objectsKV)
	retry := h.handlePastMeetingInviteeUpdate(context.Background(),
		"itx-zoom-past-meetings-invitees."+inviteeUID,
		minimalInviteeV1Data(inviteeUID, meetingAndOcc, newUsername))

	// ErrInvalidKey on tombstone Put must NOT stop processing — new event must be published.
	assert.False(t, retry)
	publisher.AssertCalled(t, "PublishPastMeetingParticipantEvent", mock.Anything, mock.Anything, mock.Anything)
}

// TestHandleInviteeUpdate_MappingPut_DeadlineExceeded_Retries verifies that a
// context.DeadlineExceeded failure on the durable mapping Put triggers retry=true.
// isTransientError does not match "context deadline exceeded", so without the fix the
// handler would ACK after publishing without storing the mapping that future
// username-change events and hard deletes depend on.
func TestHandleInviteeUpdate_MappingPut_DeadlineExceeded_Retries(t *testing.T) {
	const (
		inviteeUID    = "inv-mapping-deadline"
		meetingAndOcc = "meeting-md_occ-1"
		username      = "alice"
	)

	mappingsKV := &mockKeyValue{}
	objectsKV := &mockKeyValue{}
	publisher := &mockParticipantPublisher{}

	// No sibling xref (first-time create path).
	mappingsKV.On("Get", mock.Anything, "v1_participant_by_meeting_user.attendee."+meetingAndOcc+"."+username).
		Return(nil, jetstream.ErrKeyNotFound)
	mappingsKV.On("Get", mock.Anything, "v1_past_meeting_invitees."+inviteeUID).
		Return(nil, jetstream.ErrKeyNotFound)
	// Participant event published successfully.
	publisher.On("PublishPastMeetingParticipantEvent", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	// Durable mapping Put fails with context.DeadlineExceeded.
	mappingsKV.On("Put", mock.Anything, "v1_past_meeting_invitees."+inviteeUID, mock.Anything).
		Return(uint64(0), context.DeadlineExceeded)

	h := newParticipantHandlers(publisher, mappingsKV, objectsKV)
	retry := h.handlePastMeetingInviteeUpdate(context.Background(),
		"itx-zoom-past-meetings-invitees."+inviteeUID,
		minimalInviteeV1Data(inviteeUID, meetingAndOcc, username))

	// Must retry — mapping is required for future username-change and hard-delete events.
	assert.True(t, retry, "context.DeadlineExceeded on mapping Put must trigger retry")
}

// TestHandleInviteeUpdate_NewXrefPut_TransientError_Retries verifies that a transient
// failure writing the new-username own-side xref triggers retry=true rather than silently
// dropping the xref (which would cause future sibling merges to fail for this participant).
func TestHandleInviteeUpdate_NewXrefPut_TransientError_Retries(t *testing.T) {
	const (
		inviteeUID    = "inv-xref-transient"
		meetingAndOcc = "meeting-xrt_occ-1"
		username      = "alice"
	)
	transientErr := fmt.Errorf("nats: connection timeout writing xref")

	mappingsKV := &mockKeyValue{}
	objectsKV := &mockKeyValue{}
	publisher := &mockParticipantPublisher{}

	// No sibling xref (first-time create path).
	mappingsKV.On("Get", mock.Anything, "v1_participant_by_meeting_user.attendee."+meetingAndOcc+"."+username).
		Return(nil, jetstream.ErrKeyNotFound)
	mappingsKV.On("Get", mock.Anything, "v1_past_meeting_invitees."+inviteeUID).
		Return(nil, jetstream.ErrKeyNotFound)
	// Participant event and main mapping write succeed.
	publisher.On("PublishPastMeetingParticipantEvent", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mappingsKV.On("Put", mock.Anything, "v1_past_meeting_invitees."+inviteeUID, mock.Anything).Return(uint64(1), nil)
	// New-username xref Put fails transiently.
	mappingsKV.On("Put", mock.Anything, "v1_participant_by_meeting_user.invitee."+meetingAndOcc+"."+username, mock.Anything).
		Return(uint64(0), transientErr)

	h := newParticipantHandlers(publisher, mappingsKV, objectsKV)
	retry := h.handlePastMeetingInviteeUpdate(context.Background(),
		"itx-zoom-past-meetings-invitees."+inviteeUID,
		minimalInviteeV1Data(inviteeUID, meetingAndOcc, username))

	// Transient xref Put must trigger retry so the xref is not silently lost.
	assert.True(t, retry, "transient new-username xref Put must trigger retry")
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
	// Capture sibling update to assert invitee-side flag polarity.
	var capturedSibling *models.PastMeetingParticipantEventData
	publisher.On("PublishPastMeetingParticipantEvent", mock.Anything, mock.Anything, mock.MatchedBy(func(p *models.PastMeetingParticipantEventData) bool {
		capturedSibling = p
		return true
	})).Return(nil)
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
	// Confirm invitee-side setSiblingFlags polarity: attendee stays attended but not invited.
	require.NotNil(t, capturedSibling)
	assert.False(t, capturedSibling.IsInvited, "surviving attendee must have IsInvited=false")
	assert.True(t, capturedSibling.IsAttended, "surviving attendee must retain IsAttended=true")
	mappingsKV.AssertExpectations(t)
	publisher.AssertExpectations(t)
}

// TestHandleInviteeDelete_SiblingObjectAbsent_FallsBackToFullDelete verifies that when the
// sibling attendee xref exists but the attendee object itself is not found (stale xref),
// the handler falls back to fullParticipantDelete rather than skipping the delete entirely.
func TestHandleInviteeDelete_SiblingObjectAbsent_FallsBackToFullDelete(t *testing.T) {
	const (
		inviteeUID    = "inv-del-fallback"
		attendeeUID   = "att-gone"
		meetingAndOcc = "meeting-del-fb_occ-1"
		username      = "alice"
	)

	mappingsKV := &mockKeyValue{}
	objectsKV := &mockKeyValue{}
	publisher := &mockParticipantPublisher{}

	// Not tombstoned.
	mappingsKV.On("Get", mock.Anything, "v1_past_meeting_invitees."+inviteeUID).
		Return(mockKeyValueEntry{value: []byte("some-mapping")}, nil)
	// Sibling xref exists.
	mappingsKV.On("Get", mock.Anything, "v1_participant_by_meeting_user.attendee."+meetingAndOcc+"."+username).
		Return(mockKeyValueEntry{value: []byte(attendeeUID)}, nil)
	// Sibling object is gone (stale xref).
	objectsKV.On("Get", mock.Anything, "itx-zoom-past-meetings-attendees."+attendeeUID).
		Return(nil, jetstream.ErrKeyNotFound)
	// Falls back to fullParticipantDelete.
	publisher.On("PublishIndexerDelete", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	publisher.On("PublishAccessDelete", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mappingsKV.On("Put", mock.Anything, mock.Anything, mock.Anything).Return(uint64(1), nil)

	h := newParticipantHandlers(publisher, mappingsKV, objectsKV)
	v1Data := map[string]interface{}{
		"lf_sso":                    username,
		"meeting_and_occurrence_id": meetingAndOcc,
	}
	retry := h.handlePastMeetingInviteeDelete(context.Background(),
		"itx-zoom-past-meetings-invitees."+inviteeUID, v1Data)

	assert.False(t, retry)
	// Full delete must fire — indexer delete + member_remove.
	publisher.AssertCalled(t, "PublishIndexerDelete", mock.Anything, mock.Anything, mock.Anything)
	publisher.AssertCalled(t, "PublishAccessDelete", mock.Anything, mock.Anything, mock.Anything)
	// No sibling update published.
	publisher.AssertNumberOfCalls(t, "PublishPastMeetingParticipantEvent", 0)
}

// TestHandleInviteeDelete_SiblingDecodeFailure_FallsBackToFullDelete verifies that when the
// sibling attendee object is present but cannot be decoded, the handler falls back to
// fullParticipantDelete (rather than ACKing without doing any cleanup).
func TestHandleInviteeDelete_SiblingDecodeFailure_FallsBackToFullDelete(t *testing.T) {
	const (
		inviteeUID    = "inv-del-decode-fail"
		attendeeUID   = "att-corrupt"
		meetingAndOcc = "meeting-del-df_occ-1"
		username      = "alice"
	)

	mappingsKV := &mockKeyValue{}
	objectsKV := &mockKeyValue{}
	publisher := &mockParticipantPublisher{}

	// Not tombstoned.
	mappingsKV.On("Get", mock.Anything, "v1_past_meeting_invitees."+inviteeUID).
		Return(mockKeyValueEntry{value: []byte("some-mapping")}, nil)
	// Sibling xref exists.
	mappingsKV.On("Get", mock.Anything, "v1_participant_by_meeting_user.attendee."+meetingAndOcc+"."+username).
		Return(mockKeyValueEntry{value: []byte(attendeeUID)}, nil)
	// Sibling object present but corrupt (not valid JSON map).
	objectsKV.On("Get", mock.Anything, "itx-zoom-past-meetings-attendees."+attendeeUID).
		Return(mockKeyValueEntry{value: []byte("not-json")}, nil)
	// Falls back to fullParticipantDelete.
	publisher.On("PublishIndexerDelete", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	publisher.On("PublishAccessDelete", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mappingsKV.On("Put", mock.Anything, mock.Anything, mock.Anything).Return(uint64(1), nil)

	h := newParticipantHandlers(publisher, mappingsKV, objectsKV)
	v1Data := map[string]interface{}{
		"lf_sso":                    username,
		"meeting_and_occurrence_id": meetingAndOcc,
	}
	retry := h.handlePastMeetingInviteeDelete(context.Background(),
		"itx-zoom-past-meetings-invitees."+inviteeUID, v1Data)

	assert.False(t, retry)
	// Full delete must fire — own state cleaned up despite corrupt sibling.
	publisher.AssertCalled(t, "PublishIndexerDelete", mock.Anything, mock.Anything, mock.Anything)
	publisher.AssertCalled(t, "PublishAccessDelete", mock.Anything, mock.Anything, mock.Anything)
	// No sibling update published.
	publisher.AssertNumberOfCalls(t, "PublishPastMeetingParticipantEvent", 0)
}

// TestHandleInviteeDelete_SiblingConvertFailure_FallsBackToFullDelete verifies that when
// the sibling attendee object is valid JSON but fails siblingConvert (e.g. missing required
// fields like meeting_and_occurrence_id), a permanent conversion error falls back to
// fullParticipantDelete rather than ACKing without any cleanup.
func TestHandleInviteeDelete_SiblingConvertFailure_FallsBackToFullDelete(t *testing.T) {
	const (
		inviteeUID    = "inv-del-convert-fail"
		attendeeUID   = "att-incomplete"
		meetingAndOcc = "meeting-del-cf_occ-1"
		username      = "alice"
	)

	// Valid JSON map but missing required "id" and "meeting_and_occurrence_id" fields
	// so decodeAttendeeRaw returns an error (permanent conversion failure).
	incompleteAttendee := map[string]interface{}{
		"lf_sso": username,
		// "id" and "meeting_and_occurrence_id" intentionally omitted
	}
	incompleteJSON := mustMarshalJSON(t, incompleteAttendee)

	mappingsKV := &mockKeyValue{}
	objectsKV := &mockKeyValue{}
	publisher := &mockParticipantPublisher{}

	// Not tombstoned.
	mappingsKV.On("Get", mock.Anything, "v1_past_meeting_invitees."+inviteeUID).
		Return(mockKeyValueEntry{value: []byte("some-mapping")}, nil)
	// Sibling xref exists.
	mappingsKV.On("Get", mock.Anything, "v1_participant_by_meeting_user.attendee."+meetingAndOcc+"."+username).
		Return(mockKeyValueEntry{value: []byte(attendeeUID)}, nil)
	// Sibling object present but missing required fields.
	objectsKV.On("Get", mock.Anything, "itx-zoom-past-meetings-attendees."+attendeeUID).
		Return(mockKeyValueEntry{value: incompleteJSON}, nil)
	// Falls back to fullParticipantDelete.
	publisher.On("PublishIndexerDelete", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	publisher.On("PublishAccessDelete", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mappingsKV.On("Put", mock.Anything, mock.Anything, mock.Anything).Return(uint64(1), nil)

	h := newParticipantHandlers(publisher, mappingsKV, objectsKV)
	v1Data := map[string]interface{}{
		"lf_sso":                    username,
		"meeting_and_occurrence_id": meetingAndOcc,
	}
	retry := h.handlePastMeetingInviteeDelete(context.Background(),
		"itx-zoom-past-meetings-invitees."+inviteeUID, v1Data)

	// Permanent conversion failure must fall back to full delete — not ACK silently.
	assert.False(t, retry)
	publisher.AssertCalled(t, "PublishIndexerDelete", mock.Anything, mock.Anything, mock.Anything)
	publisher.AssertCalled(t, "PublishAccessDelete", mock.Anything, mock.Anything, mock.Anything)
	publisher.AssertNumberOfCalls(t, "PublishPastMeetingParticipantEvent", 0)
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

// TestHandleAttendeeDelete_SiblingObjectAbsent_FallsBackToFullDelete mirrors the invitee
// equivalent: sibling invitee xref exists but the invitee object is absent (stale xref) —
// falls back to fullParticipantDelete.
func TestHandleAttendeeDelete_SiblingObjectAbsent_FallsBackToFullDelete(t *testing.T) {
	const (
		attendeeUID   = "att-del-fallback"
		inviteeUID    = "inv-gone"
		meetingAndOcc = "meeting-del-afb_occ-1"
		username      = "bob"
	)

	mappingsKV := &mockKeyValue{}
	objectsKV := &mockKeyValue{}
	publisher := &mockParticipantPublisher{}

	// Not tombstoned.
	mappingsKV.On("Get", mock.Anything, "v1_past_meeting_attendees."+attendeeUID).
		Return(mockKeyValueEntry{value: []byte("some-mapping")}, nil)
	// Sibling invitee xref exists.
	mappingsKV.On("Get", mock.Anything, "v1_participant_by_meeting_user.invitee."+meetingAndOcc+"."+username).
		Return(mockKeyValueEntry{value: []byte(inviteeUID)}, nil)
	// Sibling invitee object is gone (stale xref).
	objectsKV.On("Get", mock.Anything, "itx-zoom-past-meetings-invitees."+inviteeUID).
		Return(nil, jetstream.ErrKeyNotFound)
	// Falls back to fullParticipantDelete.
	publisher.On("PublishIndexerDelete", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	publisher.On("PublishAccessDelete", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mappingsKV.On("Put", mock.Anything, mock.Anything, mock.Anything).Return(uint64(1), nil)

	h := newParticipantHandlers(publisher, mappingsKV, objectsKV)
	v1Data := map[string]interface{}{
		"lf_sso":                    username,
		"meeting_and_occurrence_id": meetingAndOcc,
	}
	retry := h.handlePastMeetingAttendeeDelete(context.Background(),
		"itx-zoom-past-meetings-attendees."+attendeeUID, v1Data)

	assert.False(t, retry)
	publisher.AssertCalled(t, "PublishIndexerDelete", mock.Anything, mock.Anything, mock.Anything)
	publisher.AssertCalled(t, "PublishAccessDelete", mock.Anything, mock.Anything, mock.Anything)
	publisher.AssertNumberOfCalls(t, "PublishPastMeetingParticipantEvent", 0)
}

// TestHandleAttendeeDelete_SiblingDecodeFailure_FallsBackToFullDelete mirrors the invitee
// equivalent: sibling invitee object present but corrupt JSON — falls back to fullParticipantDelete.
func TestHandleAttendeeDelete_SiblingDecodeFailure_FallsBackToFullDelete(t *testing.T) {
	const (
		attendeeUID   = "att-del-decode-fail"
		inviteeUID    = "inv-corrupt"
		meetingAndOcc = "meeting-del-adf_occ-1"
		username      = "bob"
	)

	mappingsKV := &mockKeyValue{}
	objectsKV := &mockKeyValue{}
	publisher := &mockParticipantPublisher{}

	mappingsKV.On("Get", mock.Anything, "v1_past_meeting_attendees."+attendeeUID).
		Return(mockKeyValueEntry{value: []byte("some-mapping")}, nil)
	mappingsKV.On("Get", mock.Anything, "v1_participant_by_meeting_user.invitee."+meetingAndOcc+"."+username).
		Return(mockKeyValueEntry{value: []byte(inviteeUID)}, nil)
	// Sibling object present but not valid JSON.
	objectsKV.On("Get", mock.Anything, "itx-zoom-past-meetings-invitees."+inviteeUID).
		Return(mockKeyValueEntry{value: []byte("not-json")}, nil)
	publisher.On("PublishIndexerDelete", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	publisher.On("PublishAccessDelete", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mappingsKV.On("Put", mock.Anything, mock.Anything, mock.Anything).Return(uint64(1), nil)

	h := newParticipantHandlers(publisher, mappingsKV, objectsKV)
	v1Data := map[string]interface{}{
		"lf_sso":                    username,
		"meeting_and_occurrence_id": meetingAndOcc,
	}
	retry := h.handlePastMeetingAttendeeDelete(context.Background(),
		"itx-zoom-past-meetings-attendees."+attendeeUID, v1Data)

	assert.False(t, retry)
	publisher.AssertCalled(t, "PublishIndexerDelete", mock.Anything, mock.Anything, mock.Anything)
	publisher.AssertCalled(t, "PublishAccessDelete", mock.Anything, mock.Anything, mock.Anything)
	publisher.AssertNumberOfCalls(t, "PublishPastMeetingParticipantEvent", 0)
}

// TestHandleAttendeeDelete_SiblingConvertFailure_FallsBackToFullDelete mirrors the invitee
// equivalent: valid JSON sibling missing required fields (permanent convert error) —
// falls back to fullParticipantDelete.
func TestHandleAttendeeDelete_SiblingConvertFailure_FallsBackToFullDelete(t *testing.T) {
	const (
		attendeeUID   = "att-del-convert-fail"
		inviteeUID    = "inv-incomplete"
		meetingAndOcc = "meeting-del-acf_occ-1"
		username      = "bob"
	)
	// Valid JSON but missing required invitee fields.
	incomplete := map[string]interface{}{"lf_sso": username}
	incompleteJSON := mustMarshalJSON(t, incomplete)

	mappingsKV := &mockKeyValue{}
	objectsKV := &mockKeyValue{}
	publisher := &mockParticipantPublisher{}

	mappingsKV.On("Get", mock.Anything, "v1_past_meeting_attendees."+attendeeUID).
		Return(mockKeyValueEntry{value: []byte("some-mapping")}, nil)
	mappingsKV.On("Get", mock.Anything, "v1_participant_by_meeting_user.invitee."+meetingAndOcc+"."+username).
		Return(mockKeyValueEntry{value: []byte(inviteeUID)}, nil)
	objectsKV.On("Get", mock.Anything, "itx-zoom-past-meetings-invitees."+inviteeUID).
		Return(mockKeyValueEntry{value: incompleteJSON}, nil)
	publisher.On("PublishIndexerDelete", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	publisher.On("PublishAccessDelete", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mappingsKV.On("Put", mock.Anything, mock.Anything, mock.Anything).Return(uint64(1), nil)

	h := newParticipantHandlers(publisher, mappingsKV, objectsKV)
	v1Data := map[string]interface{}{
		"lf_sso":                    username,
		"meeting_and_occurrence_id": meetingAndOcc,
	}
	retry := h.handlePastMeetingAttendeeDelete(context.Background(),
		"itx-zoom-past-meetings-attendees."+attendeeUID, v1Data)

	assert.False(t, retry)
	publisher.AssertCalled(t, "PublishIndexerDelete", mock.Anything, mock.Anything, mock.Anything)
	publisher.AssertCalled(t, "PublishAccessDelete", mock.Anything, mock.Anything, mock.Anything)
	publisher.AssertNumberOfCalls(t, "PublishPastMeetingParticipantEvent", 0)
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
