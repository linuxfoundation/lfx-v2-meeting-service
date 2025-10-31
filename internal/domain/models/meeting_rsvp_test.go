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
