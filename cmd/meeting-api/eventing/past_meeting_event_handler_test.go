// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/infrastructure/eventing"
)

// The past meeting index document no longer carries the obsolete recording_password,
// while the LFX meeting_password and the zoom_config AI flags are kept.
func TestConvertMapToPastMeetingDataOmitsRecordingPassword(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	kv := &mockKeyValue{}
	kv.On("Get", mock.Anything, mock.Anything).Return(nil, jetstream.ErrKeyNotFound)

	const meetingPassword = "00000000-0000-0000-0000-000000000000"
	data := map[string]interface{}{
		"meeting_and_occurrence_id": "meeting-1-occurrence-1",
		"meeting_id":                "meeting-1",
		"proj_id":                   "proj-1",
		"scheduled_start_time":      "2026-01-01T00:00:00Z",
		"meeting_password":          meetingPassword,
		"recording_password":        "placeholder-recording-password",
		"zoom_ai_enabled":           true,
	}

	pastMeeting, err := convertMapToPastMeetingData(context.Background(), data, stubIDMapper{}, kv, kv, logger)
	require.NoError(t, err)
	require.NotNil(t, pastMeeting)

	raw, err := json.Marshal(pastMeeting)
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(raw, &doc))

	assert.NotContains(t, doc, "recording_password")
	assert.Equal(t, meetingPassword, doc["meeting_password"])

	zoomConfig, ok := doc["zoom_config"].(map[string]any)
	require.True(t, ok, "zoom_config must be present")
	assert.Equal(t, true, zoomConfig["ai_companion_enabled"])
}

// artifactRepublishPublisher records past meeting, recording and summary publishes and
// no-ops everything else.
type artifactRepublishPublisher struct {
	mockParticipantPublisher
	pastMeetings []*models.PastMeetingEventData
	recordings   []*models.RecordingEventData
	summaries    []publishedSummary
	deletes      []string // "subject:id"
}

func (p *artifactRepublishPublisher) PublishIndexerDelete(_ context.Context, subject, id string) error {
	p.deletes = append(p.deletes, subject+":"+id)
	return nil
}

func (p *artifactRepublishPublisher) PublishPastMeetingEvent(_ context.Context, _ string, data *models.PastMeetingEventData) error {
	p.pastMeetings = append(p.pastMeetings, data)
	return nil
}

func (p *artifactRepublishPublisher) PublishPastMeetingRecordingEvent(_ context.Context, _ string, data *models.RecordingEventData) error {
	p.recordings = append(p.recordings, data)
	return nil
}

func (p *artifactRepublishPublisher) PublishPastMeetingSummaryEvent(_ context.Context, action string, data *models.SummaryEventData, access string) error {
	p.summaries = append(p.summaries, publishedSummary{action: action, data: data, access: access})
	return nil
}

// TestHandlePastMeetingUpdate_RepublishesArtifacts verifies that a past meeting update
// republishes the recording and summary documents from the access settings now on the past
// meeting, so tightening a setting revokes their public flag in the index.
func TestHandlePastMeetingUpdate_RepublishesArtifacts(t *testing.T) {
	const recordingKey = "itx-zoom-past-meetings-recordings." + testSummaryMeetOcc

	// The past meeting as v1 now stores it, after the host restricted every artifact.
	parent := map[string]any{
		"meeting_and_occurrence_id": testSummaryMeetOcc,
		"meeting_id":                "12345678901",
		"proj_id":                   "proj-sfid",
		"recording_access":          "meeting_hosts",
		"transcript_access":         "meeting_hosts",
		"ai_summary_access":         "meeting_hosts",
	}

	newHandlers := func(t *testing.T, summaryIndex jetstream.KeyValueEntry, summaryIndexErr error) (*EventHandlers, *artifactRepublishPublisher) {
		t.Helper()
		objectsKV := &mockKeyValue{}
		objectsKV.On("Get", mock.Anything, testPastMeetingKey).Return(mockKeyValueEntry{key: testPastMeetingKey, value: mustMarshalJSON(t, parent)}, nil)
		// The recording record still carries the public snapshot from when it was indexed.
		objectsKV.On("Get", mock.Anything, recordingKey).Return(mockKeyValueEntry{key: recordingKey, value: mustMarshalJSON(t, map[string]any{
			"meeting_and_occurrence_id": testSummaryMeetOcc,
			"proj_id":                   "proj-sfid",
			"recording_access":          "public",
		})}, nil)
		objectsKV.On("Get", mock.Anything, testSummaryKVKey).Return(mockKeyValueEntry{key: testSummaryKVKey, value: mustMarshalJSON(t, map[string]any{
			"id":                        testSummaryID,
			"meeting_and_occurrence_id": testSummaryMeetOcc,
			"approved":                  true,
		})}, nil)

		mappingsKV := newMemKV()
		if summaryIndexErr != nil {
			mappingsKV.getErrs[testSummaryIndexKey] = summaryIndexErr
		} else if summaryIndex != nil {
			mappingsKV.data[testSummaryIndexKey] = summaryIndex.Value()
		}

		publisher := &artifactRepublishPublisher{}
		return &EventHandlers{
			publisher:    publisher,
			idMapper:     participantTestIDMapper{},
			v1ObjectsKV:  objectsKV,
			v1MappingsKV: mappingsKV,
			logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		}, publisher
	}

	t.Run("tightened access republishes recording and summary as non-public", func(t *testing.T) {
		h, publisher := newHandlers(t, mockKeyValueEntry{key: testSummaryIndexKey, value: []byte(`["` + testSummaryID + `"]`)}, nil)

		retry := handleKVPut(context.Background(), testPastMeetingKey, parent, h)
		require.False(t, retry)
		require.Len(t, publisher.pastMeetings, 1)

		require.Len(t, publisher.recordings, 1)
		assert.Equal(t, "meeting_hosts", publisher.recordings[0].RecordingAccess)
		// The recording record has no transcript files, so any old transcript document is removed.
		assert.Equal(t, []string{eventing.IndexV1PastMeetingTranscriptSubject + ":" + testSummaryMeetOcc}, publisher.deletes)

		require.Len(t, publisher.summaries, 1)
		got := publisher.summaries[0]
		assert.Equal(t, testSummaryID, got.data.ID)
		assert.Equal(t, "meeting_hosts", got.access)
		cfg := eventing.PastMeetingSummaryIndexingConfig(got.data, got.access)
		require.NotNil(t, cfg.Public)
		assert.False(t, *cfg.Public, "summary must be republished as non-public")
	})

	t.Run("invalid summary list key is not retried", func(t *testing.T) {
		h, publisher := newHandlers(t, nil, jetstream.ErrInvalidKey)

		retry := handleKVPut(context.Background(), testPastMeetingKey, parent, h)
		assert.False(t, retry, "an invalid key is permanent; retrying the past meeting event cannot help")
		assert.Empty(t, publisher.summaries)
		require.Len(t, publisher.recordings, 1, "the recording is still republished")
	})

	t.Run("summary listing failure retries the past meeting event", func(t *testing.T) {
		h, publisher := newHandlers(t, nil, errors.New("kv unavailable"))

		retry := handleKVPut(context.Background(), testPastMeetingKey, parent, h)
		assert.True(t, retry)
		assert.Empty(t, publisher.summaries)
	})
}
