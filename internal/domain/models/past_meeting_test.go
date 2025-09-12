// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPastMeeting_Tags(t *testing.T) {
	tests := []struct {
		name        string
		pastMeeting *PastMeeting
		expected    []string
	}{
		{
			name:        "nil past meeting returns nil",
			pastMeeting: nil,
			expected:    nil,
		},
		{
			name:        "empty past meeting returns empty slice",
			pastMeeting: &PastMeeting{},
			expected:    []string{},
		},
		{
			name: "past meeting with UID only",
			pastMeeting: &PastMeeting{
				UID: "past-meeting-123",
			},
			expected: []string{
				"past-meeting-123",
				"past_meeting_uid:past-meeting-123",
			},
		},
		{
			name: "past meeting with MeetingUID only",
			pastMeeting: &PastMeeting{
				MeetingUID: "meeting-456",
			},
			expected: []string{
				"meeting_uid:meeting-456",
			},
		},
		{
			name: "past meeting with ProjectUID only",
			pastMeeting: &PastMeeting{
				ProjectUID: "project-789",
			},
			expected: []string{
				"project_uid:project-789",
			},
		},
		{
			name: "past meeting with OccurrenceID only",
			pastMeeting: &PastMeeting{
				OccurrenceID: "occurrence-101",
			},
			expected: []string{
				"occurrence_id:occurrence-101",
			},
		},
		{
			name: "past meeting with Title only",
			pastMeeting: &PastMeeting{
				Title: "Weekly Standup - Past",
			},
			expected: []string{
				"title:Weekly Standup - Past",
			},
		},
		{
			name: "past meeting with Description only",
			pastMeeting: &PastMeeting{
				Description: "Past team sync meeting",
			},
			expected: []string{
				"description:Past team sync meeting",
			},
		},
		{
			name: "past meeting with all fields populated",
			pastMeeting: &PastMeeting{
				UID:                "past-meeting-123",
				MeetingUID:         "meeting-456",
				OccurrenceID:       "occurrence-101",
				ProjectUID:         "project-789",
				Title:              "Weekly Standup - Past",
				Description:        "Past team sync meeting",
				ScheduledStartTime: time.Now().Add(-time.Hour),
				ScheduledEndTime:   time.Now(),
				Duration:           60,
				Timezone:           "UTC",
				Platform:           PlatformZoom,
				Committees: []Committee{
					{UID: "committee-111"},
				},
			},
			expected: []string{
				"past-meeting-123",
				"past_meeting_uid:past-meeting-123",
				"meeting_uid:meeting-456",
				"project_uid:project-789",
				"occurrence_id:occurrence-101",
				"committee_uid:committee-111",
				"title:Weekly Standup - Past",
				"description:Past team sync meeting",
			},
		},
		{
			name: "past meeting with empty string fields are ignored",
			pastMeeting: &PastMeeting{
				UID:          "",
				MeetingUID:   "",
				OccurrenceID: "",
				ProjectUID:   "",
				Title:        "",
				Description:  "",
				Platform:     PlatformZoom,
			},
			expected: []string{},
		},
		{
			name: "past meeting with some fields empty",
			pastMeeting: &PastMeeting{
				UID:         "past-meeting-123",
				MeetingUID:  "meeting-456",
				ProjectUID:  "",
				Title:       "Weekly Standup",
				Description: "",
			},
			expected: []string{
				"past-meeting-123",
				"past_meeting_uid:past-meeting-123",
				"meeting_uid:meeting-456",
				"title:Weekly Standup",
			},
		},
		{
			name: "past meeting with whitespace-only fields are included",
			pastMeeting: &PastMeeting{
				UID:         "past-meeting-123",
				Title:       "   ",
				Description: "\t\n",
			},
			expected: []string{
				"past-meeting-123",
				"past_meeting_uid:past-meeting-123",
				"title:   ",
				"description:\t\n",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.pastMeeting.Tags()
			assert.Equal(t, tt.expected, result)
		})
	}
}
