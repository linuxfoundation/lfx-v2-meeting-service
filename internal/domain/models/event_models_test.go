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
				FirstName: "Paul",
				LastName:  "Hinz",
				Username:  "phinz",
				Email:     "phinz@example.com",
			},
			expected: []string{"phinz", "phinz@example.com", "Paul", "Hinz", "Paul Hinz"},
		},
		{
			name: "omits combined name when first or last name is missing",
			data: RegistrantEventData{
				FirstName: "Paul",
				Username:  "phinz",
			},
			expected: []string{"phinz", "Paul"},
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
				FirstName: "Paul",
				LastName:  "Hinz",
				Username:  "phinz",
			},
			expected: []string{"Paul", "Hinz", "phinz", "Paul Hinz"},
		},
		{
			name: "omits combined name when last name is missing",
			data: PastMeetingParticipantEventData{
				FirstName: "Paul",
				Username:  "phinz",
			},
			expected: []string{"Paul", "phinz"},
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
