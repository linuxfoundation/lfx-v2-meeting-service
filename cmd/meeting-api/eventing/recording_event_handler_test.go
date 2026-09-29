// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/vmihailenco/msgpack/v5"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
)

// =============================================================================
// recordingTestPublisher — captures recording and transcript publish calls
// =============================================================================

type recordingTestPublisher struct {
	recordingAction string
	recordingData   *models.RecordingEventData
	transcriptData  *models.TranscriptEventData
}

func (p *recordingTestPublisher) PublishPastMeetingRecordingEvent(_ context.Context, action string, data *models.RecordingEventData) error {
	p.recordingAction = action
	p.recordingData = data
	return nil
}
func (p *recordingTestPublisher) PublishPastMeetingTranscriptEvent(_ context.Context, _ string, data *models.TranscriptEventData) error {
	p.transcriptData = data
	return nil
}
func (p *recordingTestPublisher) PublishMeetingEvent(_ context.Context, _ string, _ *models.MeetingEventData) error {
	return nil
}
func (p *recordingTestPublisher) PublishMeetingHostCredentialsEvent(_ context.Context, _ string, _ *models.MeetingHostCredentialsEventData) error {
	return nil
}
func (p *recordingTestPublisher) PublishRegistrantEvent(_ context.Context, _ string, _ *models.RegistrantEventData) error {
	return nil
}
func (p *recordingTestPublisher) PublishInviteResponseEvent(_ context.Context, _ string, _ *models.InviteResponseEventData) error {
	return nil
}
func (p *recordingTestPublisher) PublishPastMeetingEvent(_ context.Context, _ string, _ *models.PastMeetingEventData) error {
	return nil
}
func (p *recordingTestPublisher) PublishPastMeetingSummaryEvent(_ context.Context, _ string, _ *models.SummaryEventData, _ string) error {
	return nil
}
func (p *recordingTestPublisher) PublishPastMeetingParticipantEvent(_ context.Context, _ string, _ *models.PastMeetingParticipantEventData) error {
	return nil
}
func (p *recordingTestPublisher) PublishMeetingAttachmentEvent(_ context.Context, _ string, _ *models.MeetingAttachmentEventData) error {
	return nil
}
func (p *recordingTestPublisher) PublishPastMeetingAttachmentEvent(_ context.Context, _ string, _ *models.PastMeetingAttachmentEventData) error {
	return nil
}
func (p *recordingTestPublisher) PublishAccessDelete(_ context.Context, _ string, _ []byte) error {
	return nil
}
func (p *recordingTestPublisher) PublishIndexerDelete(_ context.Context, _ string, _ string) error {
	return nil
}
func (p *recordingTestPublisher) Close() error { return nil }

// =============================================================================
// recordingTestIDMapper — returns a fixed project UID so the handler proceeds
// =============================================================================

type recordingTestIDMapper struct{}

func (recordingTestIDMapper) MapProjectV1ToV2(_ context.Context, _ string) (string, error) {
	return "00000000-0000-0000-0000-000000000001", nil
}
func (recordingTestIDMapper) MapProjectV2ToV1(_ context.Context, _ string) (string, error) {
	return "proj-sfid", nil
}
func (recordingTestIDMapper) MapCommitteeV1ToV2(_ context.Context, _ string) (string, error) {
	return "", nil
}
func (recordingTestIDMapper) MapCommitteeV2ToV1(_ context.Context, _ string) (string, error) {
	return "", nil
}
func (recordingTestIDMapper) MapInviteeIDToParticipantV2(_ context.Context, _ string) (string, error) {
	return "", nil
}
func (recordingTestIDMapper) MapAttendeeIDToParticipantV2(_ context.Context, _ string) (string, error) {
	return "", nil
}
func (recordingTestIDMapper) MapParticipantV2ToInviteeID(_ context.Context, _ string) (string, error) {
	return "", nil
}
func (recordingTestIDMapper) MapParticipantV2ToAttendeeID(_ context.Context, _ string) (string, error) {
	return "", nil
}

// =============================================================================
// helpers
// =============================================================================

// mustMsgpackKV encodes v as msgpack for use in mockKeyValueEntry (parent KV lookups).
func mustMsgpackKV(t *testing.T, v any) []byte {
	t.Helper()
	b, err := msgpack.Marshal(v)
	require.NoError(t, err)
	return b
}

// buildRecordingHandler wires up an EventHandlers with the supplied KVs and publisher.
func buildRecordingHandler(pub *recordingTestPublisher, v1ObjectsKV, v1MappingsKV jetstream.KeyValue) *EventHandlers {
	return &EventHandlers{
		publisher:    pub,
		idMapper:     recordingTestIDMapper{},
		v1ObjectsKV:  v1ObjectsKV,
		v1MappingsKV: v1MappingsKV,
		logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

// =============================================================================
// TestHandlePastMeetingRecordingUpdate_AccessOverride
// =============================================================================

// TestHandlePastMeetingRecordingUpdate_AccessOverride verifies that the recording handler
// always uses the parent past meeting record as the authoritative source for
// recording_access and transcript_access, overriding the stale snapshot on the
// recording KV record.
func TestHandlePastMeetingRecordingUpdate_AccessOverride(t *testing.T) {
	const (
		meetingAndOccID = "99000000001:1700000000"
		projSFID        = "proj-sfid"
		recordingKey    = "itx-zoom-past-meetings-recordings." + meetingAndOccID
		parentKey       = "itx-zoom-past-meetings." + meetingAndOccID
		mappingKey      = "v1_past_meeting_recordings." + meetingAndOccID
	)

	// baseRecordingData returns a minimal valid recording map with the given access fields.
	baseRecordingData := func(recordingAccess, transcriptAccess string) map[string]any {
		return map[string]any{
			"meeting_and_occurrence_id": meetingAndOccID,
			"meeting_id":                "99000000001",
			"occurrence_id":             "1700000000",
			"proj_id":                   projSFID,
			"project_slug":              "test-project",
			"recording_access":          recordingAccess,
			"transcript_access":         transcriptAccess,
			"topic":                     "Test Meeting",
			"recording_files":           []any{},
			"sessions":                  []any{},
		}
	}

	// baseParentData returns a minimal parent past meeting map with the given access fields.
	baseParentData := func(recordingAccess, transcriptAccess string) map[string]any {
		return map[string]any{
			"meeting_and_occurrence_id": meetingAndOccID,
			"proj_id":                   projSFID,
			"project_slug":              "test-project",
			"recording_access":          recordingAccess,
			"transcript_access":         transcriptAccess,
		}
	}

	setupKVs := func(t *testing.T, recordingData, parentData map[string]any) (v1Objects, v1Mappings *mockKeyValue) {
		t.Helper()
		v1Objects = &mockKeyValue{}
		v1Mappings = &mockKeyValue{}

		v1Objects.On("Get", mock.Anything, parentKey).
			Return(mockKeyValueEntry{key: parentKey, value: mustMsgpackKV(t, parentData)}, nil)
		// Committee mapping KV: no committee entry — returns not found, which is fine.
		v1Mappings.On("Get", mock.Anything, mock.MatchedBy(func(k string) bool {
			return k != mappingKey
		})).Return(nil, jetstream.ErrKeyNotFound)
		// Mapping key presence check (created vs updated): return not found → "created"
		v1Mappings.On("Get", mock.Anything, mappingKey).Return(nil, jetstream.ErrKeyNotFound)
		v1Mappings.On("Put", mock.Anything, mappingKey, mock.Anything).Return(uint64(1), nil)

		return v1Objects, v1Mappings
	}

	t.Run("parent public overrides snapshot meeting_hosts on recording", func(t *testing.T) {
		pub := &recordingTestPublisher{}
		recordingMap := baseRecordingData("meeting_hosts", "")
		v1Objects, v1Mappings := setupKVs(t, recordingMap, baseParentData("public", "public"))
		h := buildRecordingHandler(pub, v1Objects, v1Mappings)

		retry := h.handlePastMeetingRecordingUpdate(context.Background(), recordingKey, recordingMap)
		require.False(t, retry)
		require.NotNil(t, pub.recordingData)
		assert.Equal(t, "public", pub.recordingData.RecordingAccess, "parent public must override snapshot meeting_hosts")
	})

	t.Run("parent meeting_hosts overrides snapshot public on recording", func(t *testing.T) {
		pub := &recordingTestPublisher{}
		recordingMap := baseRecordingData("public", "public")
		v1Objects, v1Mappings := setupKVs(t, recordingMap, baseParentData("meeting_hosts", "meeting_hosts"))
		h := buildRecordingHandler(pub, v1Objects, v1Mappings)

		retry := h.handlePastMeetingRecordingUpdate(context.Background(), recordingKey, recordingMap)
		require.False(t, retry)
		require.NotNil(t, pub.recordingData)
		assert.Equal(t, "meeting_hosts", pub.recordingData.RecordingAccess, "parent meeting_hosts must override snapshot public")
	})

	t.Run("parent not found falls back to meeting_hosts (fail-closed)", func(t *testing.T) {
		pub := &recordingTestPublisher{}
		v1Objects := &mockKeyValue{}
		v1Mappings := &mockKeyValue{}
		v1Objects.On("Get", mock.Anything, parentKey).Return(nil, jetstream.ErrKeyNotFound)
		v1Mappings.On("Get", mock.Anything, mock.Anything).Return(nil, jetstream.ErrKeyNotFound)
		v1Mappings.On("Put", mock.Anything, mappingKey, mock.Anything).Return(uint64(1), nil)
		h := buildRecordingHandler(pub, v1Objects, v1Mappings)

		recordingMap := baseRecordingData("public", "public")
		retry := h.handlePastMeetingRecordingUpdate(context.Background(), recordingKey, recordingMap)
		require.False(t, retry)
		require.NotNil(t, pub.recordingData)
		assert.Equal(t, defaultArtifactAccess, pub.recordingData.RecordingAccess, "parent not found must fail closed to meeting_hosts")
	})

	t.Run("parent empty access falls back to meeting_hosts for recording", func(t *testing.T) {
		// When the parent carries no access values, recording_access defaults to "meeting_hosts"
		// (fail-closed). transcript_access is NOT defaulted here because baseRecordingData has
		// no transcript files (TranscriptEnabled == false), so the field stays empty (omitempty).
		pub := &recordingTestPublisher{}
		recordingMap := baseRecordingData("public", "public")
		v1Objects, v1Mappings := setupKVs(t, recordingMap, baseParentData("", ""))
		h := buildRecordingHandler(pub, v1Objects, v1Mappings)

		retry := h.handlePastMeetingRecordingUpdate(context.Background(), recordingKey, recordingMap)
		require.False(t, retry)
		require.NotNil(t, pub.recordingData)
		assert.Equal(t, defaultArtifactAccess, pub.recordingData.RecordingAccess, "empty parent recording_access must fall back to meeting_hosts")
		assert.Empty(t, pub.recordingData.TranscriptAccess, "no transcript files: transcript_access must stay empty (omitempty), not defaulted to meeting_hosts")
	})

	t.Run("recordingData.TranscriptAccess is set from parent when transcript files exist", func(t *testing.T) {
		// When a TRANSCRIPT file is present (TranscriptEnabled == true), the recording document's
		// embedded transcript_access must come from the parent, not the stale snapshot.
		pub := &recordingTestPublisher{}
		recordingMapWithTranscript := baseRecordingData("meeting_hosts", "public") // snapshot: transcript public
		recordingMapWithTranscript["recording_files"] = []any{
			map[string]any{"file_type": "TRANSCRIPT"},
		}
		v1Objects, v1Mappings := setupKVs(t, recordingMapWithTranscript, baseParentData("meeting_hosts", "meeting_hosts"))
		h := buildRecordingHandler(pub, v1Objects, v1Mappings)

		retry := h.handlePastMeetingRecordingUpdate(context.Background(), recordingKey, recordingMapWithTranscript)
		require.False(t, retry)
		require.NotNil(t, pub.recordingData)
		assert.Equal(t, "meeting_hosts", pub.recordingData.TranscriptAccess,
			"recording doc TranscriptAccess must use parent value, not stale snapshot")
		// With a transcript file, a separate transcript document is published.
		require.NotNil(t, pub.transcriptData, "transcript file present means transcript document is published")
		assert.Equal(t, "meeting_hosts", pub.transcriptData.TranscriptAccess,
			"transcript doc TranscriptAccess must also use parent value")
	})
}
