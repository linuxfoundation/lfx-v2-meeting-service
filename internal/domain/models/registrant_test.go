// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRegistrant_Tags(t *testing.T) {
	tests := []struct {
		name       string
		registrant *Registrant
		expected   []string
	}{
		{
			name:       "nil registrant returns nil",
			registrant: nil,
			expected:   nil,
		},
		{
			name:       "empty registrant returns empty slice",
			registrant: &Registrant{},
			expected:   []string{},
		},
		{
			name: "registrant with UID only",
			registrant: &Registrant{
				UID: "registrant-123",
			},
			expected: []string{
				"registrant-123",
				"registrant_uid:registrant-123",
			},
		},
		{
			name: "registrant with MeetingUID only",
			registrant: &Registrant{
				MeetingUID: "meeting-456",
			},
			expected: []string{
				"meeting_uid:meeting-456",
			},
		},
		{
			name: "registrant with FirstName only",
			registrant: &Registrant{
				FirstName: "Jane",
			},
			expected: []string{
				"first_name:Jane",
			},
		},
		{
			name: "registrant with LastName only",
			registrant: &Registrant{
				LastName: "Smith",
			},
			expected: []string{
				"last_name:Smith",
			},
		},
		{
			name: "registrant with Email only",
			registrant: &Registrant{
				Email: "jane.smith@example.com",
			},
			expected: []string{
				"email:jane.smith@example.com",
			},
		},
		{
			name: "registrant with Username only",
			registrant: &Registrant{
				Username: "janesmith",
			},
			expected: []string{
				"username:janesmith",
			},
		},
		{
			name: "registrant with all fields populated",
			registrant: &Registrant{
				UID:                "registrant-123",
				MeetingUID:         "meeting-456",
				Email:              "jane.smith@example.com",
				FirstName:          "Jane",
				LastName:           "Smith",
				Username:           "janesmith",
				Host:               false,
				JobTitle:           "Product Manager",
				OccurrenceID:       "occurrence-789",
				OrgName:            "Example Inc",
				OrgIsMember:        true,
				OrgIsProjectMember: true,
				AvatarURL:          "https://example.com/jane.jpg",
				CreatedAt:          &time.Time{},
				UpdatedAt:          &time.Time{},
			},
			expected: []string{
				"registrant-123",
				"registrant_uid:registrant-123",
				"meeting_uid:meeting-456",
				"first_name:Jane",
				"last_name:Smith",
				"email:jane.smith@example.com",
				"username:janesmith",
			},
		},
		{
			name: "registrant with empty string fields are ignored",
			registrant: &Registrant{
				UID:        "",
				MeetingUID: "",
				Email:      "",
				FirstName:  "",
				LastName:   "",
				Username:   "",
				Host:       true,
			},
			expected: []string{},
		},
		{
			name: "registrant with some fields empty",
			registrant: &Registrant{
				UID:        "registrant-123",
				MeetingUID: "meeting-456",
				Email:      "",
				FirstName:  "Jane",
				LastName:   "Smith",
				Username:   "",
			},
			expected: []string{
				"registrant-123",
				"registrant_uid:registrant-123",
				"meeting_uid:meeting-456",
				"first_name:Jane",
				"last_name:Smith",
			},
		},
		{
			name: "registrant with whitespace-only fields are included",
			registrant: &Registrant{
				UID:       "registrant-123",
				FirstName: "   ",
				LastName:  "\t\n",
				Email:     " test@example.com ",
				Username:  "  user123  ",
			},
			expected: []string{
				"registrant-123",
				"registrant_uid:registrant-123",
				"first_name:   ",
				"last_name:\t\n",
				"email: test@example.com ",
				"username:  user123  ",
			},
		},
		{
			name: "registrant with special characters in fields",
			registrant: &Registrant{
				UID:       "registrant-123",
				FirstName: "María",
				LastName:  "González-Hernández",
				Email:     "maría.gonzález+work@example.com",
				Username:  "maria.gonzalez@2024",
			},
			expected: []string{
				"registrant-123",
				"registrant_uid:registrant-123",
				"first_name:María",
				"last_name:González-Hernández",
				"email:maría.gonzález+work@example.com",
				"username:maria.gonzalez@2024",
			},
		},
		{
			name: "registrant with host flag variations",
			registrant: &Registrant{
				UID:       "registrant-123",
				FirstName: "Host",
				Host:      true, // This should not affect tags output
			},
			expected: []string{
				"registrant-123",
				"registrant_uid:registrant-123",
				"first_name:Host",
			},
		},
		{
			name: "registrant with long field values",
			registrant: &Registrant{
				UID:       "registrant-123",
				FirstName: "VeryLongFirstNameThatExceedsNormalLength",
				LastName:  "EquallyLongLastNameWithManyCharacters",
				Email:     "very.long.email.address.with.many.dots@very-long-domain-name.example.com",
				Username:  "very_long_username_with_underscores_and_numbers_123456789",
			},
			expected: []string{
				"registrant-123",
				"registrant_uid:registrant-123",
				"first_name:VeryLongFirstNameThatExceedsNormalLength",
				"last_name:EquallyLongLastNameWithManyCharacters",
				"email:very.long.email.address.with.many.dots@very-long-domain-name.example.com",
				"username:very_long_username_with_underscores_and_numbers_123456789",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.registrant.Tags()
			assert.Equal(t, tt.expected, result)
		})
	}
}
