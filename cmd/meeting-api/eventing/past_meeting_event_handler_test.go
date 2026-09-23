// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"testing"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// The past meeting index document no longer carries the obsolete recording_password or a
// zoom passcode, while the LFX meeting_password and the zoom_config AI flags are kept.
func TestConvertMapToPastMeetingDataOmitsRecordingPassword(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	kv := &mockKeyValue{}
	kv.On("Get", mock.Anything, mock.Anything).Return(nil, jetstream.ErrKeyNotFound)

	const meetingPassword = "00000000-0000-0000-0000-000000000000"

	baseData := func() map[string]interface{} {
		return map[string]interface{}{
			"meeting_and_occurrence_id": "meeting-1-occurrence-1",
			"meeting_id":                "meeting-1",
			"proj_id":                   "proj-1",
			"scheduled_start_time":      "2026-01-01T00:00:00Z",
			"meeting_password":          meetingPassword,
			"zoom_ai_enabled":           true,
		}
	}

	tests := []struct {
		name   string
		extras map[string]interface{}
	}{
		{
			name: "recording_password and passcode in input are dropped",
			extras: map[string]interface{}{
				"recording_password": "placeholder-recording-password",
				"passcode":           "placeholder-passcode",
			},
		},
		{
			name: "no recording_password in input",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := baseData()
			for k, v := range tt.extras {
				data[k] = v
			}

			pastMeeting, err := convertMapToPastMeetingData(context.Background(), data, stubIDMapper{}, kv, kv, logger)
			require.NoError(t, err)
			require.NotNil(t, pastMeeting)

			raw, err := json.Marshal(pastMeeting)
			require.NoError(t, err)
			var doc map[string]any
			require.NoError(t, json.Unmarshal(raw, &doc))

			assert.NotContains(t, doc, "recording_password")
			assert.NotContains(t, doc, "passcode")
			assert.Equal(t, meetingPassword, doc["meeting_password"])

			zoomConfig, ok := doc["zoom_config"].(map[string]any)
			require.True(t, ok, "zoom_config must be present")
			assert.NotContains(t, zoomConfig, "passcode")
			assert.Equal(t, true, zoomConfig["ai_companion_enabled"])
		})
	}
}
