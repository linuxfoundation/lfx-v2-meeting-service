// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package models

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// v1 NATS data sends ZoomMeetingRecurrence int fields as JSON strings; UnmarshalJSON must coerce them.
func TestZoomMeetingRecurrenceUnmarshalCoercion(t *testing.T) {
	happyPath := []struct {
		name string
		json string
		want ZoomMeetingRecurrence
	}{
		{
			"all ints as strings",
			`{"type":"2","repeat_interval":"1","monthly_day":"0","monthly_week":"0","monthly_week_day":"0","end_times":"5"}`,
			ZoomMeetingRecurrence{Type: 2, RepeatInterval: 1, EndTimes: 5},
		},
		{
			"all ints as numbers",
			`{"type":2,"repeat_interval":1,"monthly_week":2,"monthly_week_day":3,"end_times":5}`,
			ZoomMeetingRecurrence{Type: 2, RepeatInterval: 1, MonthlyWeek: 2, MonthlyWeekDay: 3, EndTimes: 5},
		},
		{
			"all ints as floats",
			`{"type":2.0,"repeat_interval":1.0,"end_times":5.0}`,
			ZoomMeetingRecurrence{Type: 2, RepeatInterval: 1, EndTimes: 5},
		},
		{
			"empty strings treated as zero",
			`{"type":"","repeat_interval":"","monthly_week":""}`,
			ZoomMeetingRecurrence{},
		},
		{
			"string fields preserved",
			`{"type":2,"repeat_interval":1,"weekly_days":"1,2","end_date_time":"2026-12-31T00:00:00Z"}`,
			ZoomMeetingRecurrence{Type: 2, RepeatInterval: 1, WeeklyDays: "1,2", EndDateTime: "2026-12-31T00:00:00Z"},
		},
	}
	for _, tt := range happyPath {
		t.Run(tt.name, func(t *testing.T) {
			var r ZoomMeetingRecurrence
			require.NoError(t, json.Unmarshal([]byte(tt.json), &r))
			assert.Equal(t, tt.want, r)
		})
	}

	invalidTypes := []struct {
		name string
		json string
	}{
		{"bool for repeat_interval", `{"repeat_interval":true}`},
		{"non-numeric string for repeat_interval", `{"repeat_interval":"weekly"}`},
		{"object for monthly_week", `{"monthly_week":{}}`},
	}
	for _, tt := range invalidTypes {
		t.Run(tt.name, func(t *testing.T) {
			var r ZoomMeetingRecurrence
			require.Error(t, json.Unmarshal([]byte(tt.json), &r))
		})
	}
}

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
