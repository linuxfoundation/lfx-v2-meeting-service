// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMeetingBase_Tags(t *testing.T) {
	tests := []struct {
		name     string
		meeting  *MeetingBase
		expected []string
	}{
		{
			name:     "nil meeting returns nil",
			meeting:  nil,
			expected: nil,
		},
		{
			name:     "empty meeting returns empty slice",
			meeting:  &MeetingBase{},
			expected: []string{},
		},
		{
			name: "meeting with UID only",
			meeting: &MeetingBase{
				UID: "meeting-123",
			},
			expected: []string{
				"meeting-123",
				"meeting_uid:meeting-123",
			},
		},
		{
			name: "meeting with ProjectUID only",
			meeting: &MeetingBase{
				ProjectUID: "project-456",
			},
			expected: []string{
				"project_uid:project-456",
			},
		},
		{
			name: "meeting with Title only",
			meeting: &MeetingBase{
				Title: "Weekly Standup",
			},
			expected: []string{
				"Weekly Standup",
			},
		},
		{
			name: "meeting with Description only",
			meeting: &MeetingBase{
				Description: "Team sync meeting",
			},
			expected: []string{
				"Team sync meeting",
			},
		},
		{
			name: "meeting with single committee",
			meeting: &MeetingBase{
				Committees: []Committee{
					{UID: "committee-789"},
				},
			},
			expected: []string{
				"committee_uid:committee-789",
			},
		},
		{
			name: "meeting with multiple committees",
			meeting: &MeetingBase{
				Committees: []Committee{
					{UID: "committee-789"},
					{UID: "committee-101"},
				},
			},
			expected: []string{
				"committee_uid:committee-789",
				"committee_uid:committee-101",
			},
		},
		{
			name: "meeting with all fields populated",
			meeting: &MeetingBase{
				UID:         "meeting-123",
				ProjectUID:  "project-456",
				Title:       "Weekly Standup",
				Description: "Team sync meeting",
				Committees: []Committee{
					{UID: "committee-789"},
					{UID: "committee-101"},
				},
				Platform:  "Zoom",
				StartTime: time.Now(),
				Duration:  60,
				Timezone:  "UTC",
			},
			expected: []string{
				"meeting-123",
				"meeting_uid:meeting-123",
				"project_uid:project-456",
				"committee_uid:committee-789",
				"committee_uid:committee-101",
				"Weekly Standup",
				"Team sync meeting",
			},
		},
		{
			name: "meeting with empty string fields are ignored",
			meeting: &MeetingBase{
				UID:         "",
				ProjectUID:  "",
				Title:       "",
				Description: "",
				Platform:    "Zoom",
			},
			expected: []string{},
		},
		{
			name: "meeting with committee with empty UID is ignored",
			meeting: &MeetingBase{
				UID: "meeting-123",
				Committees: []Committee{
					{UID: ""},
				},
			},
			expected: []string{
				"meeting-123",
				"meeting_uid:meeting-123",
				"committee_uid:",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.meeting.Tags()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMeetingSettings_Tags(t *testing.T) {
	tests := []struct {
		name     string
		settings *MeetingSettings
		expected []string
	}{
		{
			name:     "nil settings returns nil",
			settings: nil,
			expected: nil,
		},
		{
			name:     "empty settings returns empty slice",
			settings: &MeetingSettings{},
			expected: []string{},
		},
		{
			name: "settings with UID only",
			settings: &MeetingSettings{
				UID: "meeting-123",
			},
			expected: []string{
				"meeting-123",
				"meeting_uid:meeting-123",
			},
		},
		{
			name: "settings with all fields populated",
			settings: &MeetingSettings{
				UID:        "meeting-123",
				Organizers: []string{"user1", "user2"},
				CreatedAt:  &time.Time{},
				UpdatedAt:  &time.Time{},
			},
			expected: []string{
				"meeting-123",
				"meeting_uid:meeting-123",
			},
		},
		{
			name: "settings with empty UID is ignored",
			settings: &MeetingSettings{
				UID:        "",
				Organizers: []string{"user1"},
			},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.settings.Tags()
			assert.Equal(t, tt.expected, result)
		})
	}
}
