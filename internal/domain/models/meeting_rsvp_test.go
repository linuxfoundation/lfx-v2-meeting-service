// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRSVPResponse_Tags(t *testing.T) {
	occurrenceID := "occ-123"
	tests := []struct {
		name     string
		rsvp     *RSVPResponse
		expected []string
	}{
		{
			name:     "nil RSVP returns nil",
			rsvp:     nil,
			expected: nil,
		},
		{
			name: "complete RSVP with all fields",
			rsvp: &RSVPResponse{
				ID:           "rsvp-123",
				MeetingUID:   "meeting-456",
				RegistrantID: "reg-789",
				Username:     "jdoe",
				Email:        "john.doe@example.com",
				Response:     RSVPResponseAccepted,
				Scope:        RSVPScopeSingle,
				OccurrenceID: &occurrenceID,
			},
			expected: []string{
				"rsvp-123",
				"rsvp_id:rsvp-123",
				"meeting_uid:meeting-456",
				"registrant_id:reg-789",
				"username:jdoe",
				"email:john.doe@example.com",
				"response:accepted",
				"scope:single",
				"occurrence_id:occ-123",
			},
		},
		{
			name: "RSVP with scope all (no occurrence ID)",
			rsvp: &RSVPResponse{
				ID:           "rsvp-123",
				MeetingUID:   "meeting-456",
				RegistrantID: "reg-789",
				Response:     RSVPResponseMaybe,
				Scope:        RSVPScopeAll,
			},
			expected: []string{
				"rsvp-123",
				"rsvp_id:rsvp-123",
				"meeting_uid:meeting-456",
				"registrant_id:reg-789",
				"response:maybe",
				"scope:all",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.rsvp.Tags()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRSVPResponse_AppliesToOccurrence(t *testing.T) {
	occ1 := "occ-1"
	occ2 := "occ-2"

	tests := []struct {
		name         string
		rsvp         *RSVPResponse
		occurrenceID string
		expected     bool
	}{
		{
			name:         "nil RSVP returns false",
			rsvp:         nil,
			occurrenceID: "occ-1",
			expected:     false,
		},
		{
			name: "scope all applies to any occurrence",
			rsvp: &RSVPResponse{
				Scope: RSVPScopeAll,
			},
			occurrenceID: "occ-1",
			expected:     true,
		},
		{
			name: "scope single applies to matching occurrence",
			rsvp: &RSVPResponse{
				Scope:        RSVPScopeSingle,
				OccurrenceID: &occ1,
			},
			occurrenceID: "occ-1",
			expected:     true,
		},
		{
			name: "scope single does not apply to different occurrence",
			rsvp: &RSVPResponse{
				Scope:        RSVPScopeSingle,
				OccurrenceID: &occ1,
			},
			occurrenceID: "occ-2",
			expected:     false,
		},
		{
			name: "scope this_and_following returns true when occurrence ID is set",
			rsvp: &RSVPResponse{
				Scope:        RSVPScopeThisAndFollowing,
				OccurrenceID: &occ2,
			},
			occurrenceID: "occ-2",
			expected:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.rsvp.AppliesToOccurrence(tt.occurrenceID)
			assert.Equal(t, tt.expected, result)
		})
	}
}
