// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPastMeetingRecording_Tags(t *testing.T) {
	tests := []struct {
		name      string
		recording *PastMeetingRecording
		expected  []string
	}{
		{
			name:      "nil recording returns nil",
			recording: nil,
			expected:  nil,
		},
		{
			name:      "empty recording returns empty slice",
			recording: &PastMeetingRecording{},
			expected:  []string{},
		},
		{
			name: "recording with UID only",
			recording: &PastMeetingRecording{
				UID: "recording-123",
			},
			expected: []string{
				"recording-123",
				"past_meeting_recording_uid:recording-123",
			},
		},
		{
			name: "recording with PastMeetingUID only",
			recording: &PastMeetingRecording{
				PastMeetingUID: "past-meeting-456",
			},
			expected: []string{
				"past_meeting_uid:past-meeting-456",
			},
		},
		{
			name: "recording with Platform only",
			recording: &PastMeetingRecording{
				Platform: PlatformZoom,
			},
			expected: []string{
				"platform:Zoom",
			},
		},
		{
			name: "recording with PlatformMeetingID only",
			recording: &PastMeetingRecording{
				PlatformMeetingID: "123456789",
			},
			expected: []string{
				"platform_meeting_id:123456789",
			},
		},
		{
			name: "recording with all fields populated",
			recording: &PastMeetingRecording{
				UID:               "recording-123",
				PastMeetingUID:    "past-meeting-456",
				Platform:          PlatformZoom,
				PlatformMeetingID: "123456789",
				TotalSize:         1024000,
				RecordingCount:    3,
				RecordingFiles: []RecordingFileData{
					{
						ID:                "file-1",
						PlatformMeetingID: "123456789",
						RecordingEnd:      time.Now(),
						FileType:          "MP4",
						FileSize:          512000,
					},
				},
				Sessions: []RecordingSession{
					{
						UUID:      "session-1",
						ShareURL:  "https://example.com/recording/session-1",
						TotalSize: 512000,
						StartTime: time.Now(),
					},
				},
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			expected: []string{
				"recording-123",
				"past_meeting_recording_uid:recording-123",
				"past_meeting_uid:past-meeting-456",
				"platform:Zoom",
				"platform_meeting_id:123456789",
			},
		},
		{
			name: "recording with empty string fields are ignored",
			recording: &PastMeetingRecording{
				UID:               "",
				PastMeetingUID:    "",
				Platform:          "",
				PlatformMeetingID: "",
				TotalSize:         1024000,
			},
			expected: []string{},
		},
		{
			name: "recording with some fields empty",
			recording: &PastMeetingRecording{
				UID:               "recording-123",
				PastMeetingUID:    "past-meeting-456",
				Platform:          "",
				PlatformMeetingID: "123456789",
			},
			expected: []string{
				"recording-123",
				"past_meeting_recording_uid:recording-123",
				"past_meeting_uid:past-meeting-456",
				"platform_meeting_id:123456789",
			},
		},
		{
			name: "recording with whitespace-only fields are included",
			recording: &PastMeetingRecording{
				UID:               "recording-123",
				PastMeetingUID:    "   ",
				Platform:          "\t",
				PlatformMeetingID: " \n ",
			},
			expected: []string{
				"recording-123",
				"past_meeting_recording_uid:recording-123",
				"past_meeting_uid:   ",
				"platform:\t",
				"platform_meeting_id: \n ",
			},
		},
		{
			name: "recording with special characters in fields",
			recording: &PastMeetingRecording{
				UID:               "recording-123",
				PastMeetingUID:    "past-meeting-456",
				Platform:          "Zoom™",
				PlatformMeetingID: "meeting-id@special#chars",
			},
			expected: []string{
				"recording-123",
				"past_meeting_recording_uid:recording-123",
				"past_meeting_uid:past-meeting-456",
				"platform:Zoom™",
				"platform_meeting_id:meeting-id@special#chars",
			},
		},
		{
			name: "recording with different platform",
			recording: &PastMeetingRecording{
				UID:               "recording-456",
				PastMeetingUID:    "past-meeting-789",
				Platform:          "Teams",
				PlatformMeetingID: "teams-meeting-456",
			},
			expected: []string{
				"recording-456",
				"past_meeting_recording_uid:recording-456",
				"past_meeting_uid:past-meeting-789",
				"platform:Teams",
				"platform_meeting_id:teams-meeting-456",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.recording.Tags()
			assert.Equal(t, tt.expected, result)
		})
	}
}
