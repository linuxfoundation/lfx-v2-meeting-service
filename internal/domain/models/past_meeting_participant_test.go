// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPastMeetingParticipant_Tags(t *testing.T) {
	tests := []struct {
		name        string
		participant *PastMeetingParticipant
		expected    []string
	}{
		{
			name:        "nil participant returns nil",
			participant: nil,
			expected:    nil,
		},
		{
			name:        "empty participant returns empty slice",
			participant: &PastMeetingParticipant{},
			expected:    []string{},
		},
		{
			name: "participant with UID only",
			participant: &PastMeetingParticipant{
				UID: "participant-123",
			},
			expected: []string{
				"participant-123",
				"past_meeting_participant_uid:participant-123",
			},
		},
		{
			name: "participant with PastMeetingUID only",
			participant: &PastMeetingParticipant{
				PastMeetingUID: "past-meeting-456",
			},
			expected: []string{
				"past_meeting_uid:past-meeting-456",
			},
		},
		{
			name: "participant with MeetingUID only",
			participant: &PastMeetingParticipant{
				MeetingUID: "meeting-789",
			},
			expected: []string{
				"meeting_uid:meeting-789",
			},
		},
		{
			name: "participant with FirstName only",
			participant: &PastMeetingParticipant{
				FirstName: "John",
			},
			expected: []string{
				"first_name:John",
			},
		},
		{
			name: "participant with LastName only",
			participant: &PastMeetingParticipant{
				LastName: "Doe",
			},
			expected: []string{
				"last_name:Doe",
			},
		},
		{
			name: "participant with Username only",
			participant: &PastMeetingParticipant{
				Username: "johndoe",
			},
			expected: []string{
				"username:johndoe",
			},
		},
		{
			name: "participant with Email only",
			participant: &PastMeetingParticipant{
				Email: "john.doe@example.com",
			},
			expected: []string{
				"email:john.doe@example.com",
			},
		},
		{
			name: "participant with all fields populated",
			participant: &PastMeetingParticipant{
				UID:                "participant-123",
				PastMeetingUID:     "past-meeting-456",
				MeetingUID:         "meeting-789",
				Email:              "john.doe@example.com",
				FirstName:          "John",
				LastName:           "Doe",
				Username:           "johndoe",
				Host:               true,
				JobTitle:           "Developer",
				OrgName:            "Example Corp",
				OrgIsMember:        true,
				OrgIsProjectMember: false,
				AvatarURL:          "https://example.com/avatar.jpg",
				IsInvited:          true,
				IsAttended:         true,
				CreatedAt:          &time.Time{},
				UpdatedAt:          &time.Time{},
			},
			expected: []string{
				"participant-123",
				"past_meeting_participant_uid:participant-123",
				"past_meeting_uid:past-meeting-456",
				"meeting_uid:meeting-789",
				"first_name:John",
				"last_name:Doe",
				"username:johndoe",
				"email:john.doe@example.com",
			},
		},
		{
			name: "participant with empty string fields are ignored",
			participant: &PastMeetingParticipant{
				UID:            "",
				PastMeetingUID: "",
				MeetingUID:     "",
				Email:          "",
				FirstName:      "",
				LastName:       "",
				Username:       "",
				Host:           true,
			},
			expected: []string{},
		},
		{
			name: "participant with some fields empty",
			participant: &PastMeetingParticipant{
				UID:            "participant-123",
				PastMeetingUID: "past-meeting-456",
				MeetingUID:     "",
				Email:          "john.doe@example.com",
				FirstName:      "John",
				LastName:       "",
				Username:       "johndoe",
			},
			expected: []string{
				"participant-123",
				"past_meeting_participant_uid:participant-123",
				"past_meeting_uid:past-meeting-456",
				"first_name:John",
				"username:johndoe",
				"email:john.doe@example.com",
			},
		},
		{
			name: "participant with whitespace-only fields are included",
			participant: &PastMeetingParticipant{
				UID:       "participant-123",
				FirstName: "   ",
				LastName:  "\t",
				Username:  " \n ",
				Email:     "  test@example.com  ",
			},
			expected: []string{
				"participant-123",
				"past_meeting_participant_uid:participant-123",
				"first_name:   ",
				"last_name:\t",
				"username: \n ",
				"email:  test@example.com  ",
			},
		},
		{
			name: "participant with special characters in fields",
			participant: &PastMeetingParticipant{
				UID:       "participant-123",
				FirstName: "José",
				LastName:  "García-López",
				Username:  "jose.garcia@123",
				Email:     "josé.garcía+test@example.com",
			},
			expected: []string{
				"participant-123",
				"past_meeting_participant_uid:participant-123",
				"first_name:José",
				"last_name:García-López",
				"username:jose.garcia@123",
				"email:josé.garcía+test@example.com",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.participant.Tags()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPastMeetingParticipant_GetFullName(t *testing.T) {
	tests := []struct {
		name        string
		participant *PastMeetingParticipant
		want        string
	}{
		{
			name: "both names present",
			participant: &PastMeetingParticipant{
				FirstName: "John",
				LastName:  "Doe",
			},
			want: "John Doe",
		},
		{
			name: "only first name",
			participant: &PastMeetingParticipant{
				FirstName: "John",
				LastName:  "",
			},
			want: "John",
		},
		{
			name: "only last name",
			participant: &PastMeetingParticipant{
				FirstName: "",
				LastName:  "Doe",
			},
			want: "Doe",
		},
		{
			name: "both empty",
			participant: &PastMeetingParticipant{
				FirstName: "",
				LastName:  "",
			},
			want: "",
		},
		{
			name: "whitespace only",
			participant: &PastMeetingParticipant{
				FirstName: "  ",
				LastName:  "  ",
			},
			want: "",
		},
		{
			name:        "nil participant",
			participant: nil,
			want:        "",
		},
		{
			name: "with surrounding whitespace",
			participant: &PastMeetingParticipant{
				FirstName: "  John",
				LastName:  "Doe  ",
			},
			want: "John Doe",
		},
		{
			name: "special characters",
			participant: &PastMeetingParticipant{
				FirstName: "José",
				LastName:  "García-López",
			},
			want: "José García-López",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.participant.GetFullName()
			assert.Equal(t, tt.want, got)
		})
	}
}
