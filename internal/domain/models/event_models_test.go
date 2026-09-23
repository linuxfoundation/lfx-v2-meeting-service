// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package models

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestSummaryEventData_AwaitingApproval(t *testing.T) {
	tests := []struct {
		name             string
		requiresApproval bool
		approved         bool
		expected         bool
	}{
		{name: "requires approval and not approved", requiresApproval: true, approved: false, expected: true},
		{name: "requires approval and approved", requiresApproval: true, approved: true, expected: false},
		{name: "approval not required", requiresApproval: false, approved: false, expected: false},
		{name: "approval not required but approved", requiresApproval: false, approved: true, expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := SummaryEventData{RequiresApproval: tt.requiresApproval, Approved: tt.approved}
			assert.Equal(t, tt.expected, s.AwaitingApproval())
		})
	}
}

func TestSummaryEventData_JSON(t *testing.T) {
	b, err := json.Marshal(SummaryEventData{ID: "00000000-0000-0000-0000-000000000001"})
	require.NoError(t, err)

	var doc map[string]any
	require.NoError(t, json.Unmarshal(b, &doc))

	// The provider webhook payload is not part of the index document.
	assert.NotContains(t, doc, "zoom_webhook_event")

	// Readers rely on requires_approval and approved being present even when false.
	assert.Equal(t, false, doc["requires_approval"])
	assert.Equal(t, false, doc["approved"])
}
