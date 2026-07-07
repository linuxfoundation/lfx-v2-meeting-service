// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRegistrantEventData_NameAndAliases(t *testing.T) {
	tests := []struct {
		name     string
		data     RegistrantEventData
		expected []string
	}{
		{
			name: "includes combined full name alongside individual tokens",
			data: RegistrantEventData{
				FirstName: "Jane",
				LastName:  "Smith",
				Username:  "jsmith",
				Email:     "jsmith@example.com",
			},
			expected: []string{"jsmith", "jsmith@example.com", "Jane", "Smith", "Jane Smith"},
		},
		{
			name: "omits combined name when first or last name is missing",
			data: RegistrantEventData{
				FirstName: "Jane",
				Username:  "jsmith",
			},
			expected: []string{"jsmith", "Jane"},
		},
		{
			name:     "empty when no fields set",
			data:     RegistrantEventData{},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.data.NameAndAliases())
		})
	}
}

func TestPastMeetingParticipantEventData_NameAndAliases(t *testing.T) {
	tests := []struct {
		name     string
		data     PastMeetingParticipantEventData
		expected []string
	}{
		{
			name: "includes combined full name alongside individual tokens",
			data: PastMeetingParticipantEventData{
				FirstName: "Jane",
				LastName:  "Smith",
				Username:  "jsmith",
			},
			expected: []string{"Jane", "Smith", "jsmith", "Jane Smith"},
		},
		{
			name: "omits combined name when last name is missing",
			data: PastMeetingParticipantEventData{
				FirstName: "Jane",
				Username:  "jsmith",
			},
			expected: []string{"Jane", "jsmith"},
		},
		{
			name:     "empty when no fields set",
			data:     PastMeetingParticipantEventData{},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.data.NameAndAliases())
		})
	}
}
