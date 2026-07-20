// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// v1 sometimes sends auto_email_reminder_time as a string; unmarshal must coerce it.
func TestMeetingDBRawUnmarshalAutoEmailReminderTime(t *testing.T) {
	tests := []struct {
		name string
		json string
		want int
	}{
		{"string", `{"auto_email_reminder_time":"10"}`, 10},
		{"int", `{"auto_email_reminder_time":10}`, 10},
		{"empty string", `{"auto_email_reminder_time":""}`, 0},
		{"absent", `{}`, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var m MeetingDBRaw
			err := json.Unmarshal([]byte(tt.json), &m)
			require.NoError(t, err)
			assert.Equal(t, tt.want, m.AutoEmailReminderTime)
		})
	}
}

// Non-numeric strings and wrong JSON types must still be rejected.
func TestMeetingDBRawUnmarshalAutoEmailReminderTimeInvalid(t *testing.T) {
	for _, tt := range []struct {
		name string
		json string
	}{
		{"non-numeric string", `{"auto_email_reminder_time":"soon"}`},
		{"bool", `{"auto_email_reminder_time":true}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var m MeetingDBRaw
			require.Error(t, json.Unmarshal([]byte(tt.json), &m))
		})
	}
}

// updated_occurrences duration coercion: unknown JSON types (bool, object) are now
// rejected rather than silently coerced to zero. This is a deliberate tightening over
// the original switch which had no default case.
func TestMeetingDBRawUnmarshalUpdatedOccurrenceDuration(t *testing.T) {
	happyPath := []struct {
		name    string
		json    string
		wantDur int
	}{
		{"int", `{"updated_occurrences":[{"duration":30}]}`, 30},
		{"string", `{"updated_occurrences":[{"duration":"45"}]}`, 45},
		{"float", `{"updated_occurrences":[{"duration":60.0}]}`, 60},
		{"absent", `{"updated_occurrences":[{}]}`, 0},
	}
	for _, tt := range happyPath {
		t.Run(tt.name, func(t *testing.T) {
			var m MeetingDBRaw
			require.NoError(t, json.Unmarshal([]byte(tt.json), &m))
			require.Len(t, m.UpdatedOccurrences, 1)
			assert.Equal(t, tt.wantDur, m.UpdatedOccurrences[0].Duration)
		})
	}

	invalidTypes := []struct {
		name string
		json string
	}{
		{"bool", `{"updated_occurrences":[{"duration":true}]}`},
		{"object", `{"updated_occurrences":[{"duration":{}}]}`},
		{"non-numeric string", `{"updated_occurrences":[{"duration":"soon"}]}`},
	}
	for _, tt := range invalidTypes {
		t.Run(tt.name, func(t *testing.T) {
			var m MeetingDBRaw
			require.Error(t, json.Unmarshal([]byte(tt.json), &m))
		})
	}
}
