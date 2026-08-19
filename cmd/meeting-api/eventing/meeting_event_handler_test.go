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

// v1 sends updated_occurrences recurrence int fields as strings; RecurrenceDBRaw.UnmarshalJSON must coerce them.
func TestMeetingDBRawUnmarshalUpdatedOccurrenceRecurrence(t *testing.T) {
	input := `{"updated_occurrences":[{"recurrence":{"type":"2","repeat_interval":"1","monthly_week":"3","monthly_week_day":"5","end_times":"10"}}]}`
	var m MeetingDBRaw
	require.NoError(t, json.Unmarshal([]byte(input), &m))
	require.Len(t, m.UpdatedOccurrences, 1)
	rec := m.UpdatedOccurrences[0].Recurrence
	require.NotNil(t, rec)
	assert.Equal(t, 2, rec.Type)
	assert.Equal(t, 1, rec.RepeatInterval)
	assert.Equal(t, 3, rec.MonthlyWeek)
	assert.Equal(t, 5, rec.MonthlyWeekDay)
	assert.Equal(t, 10, rec.EndTimes)
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

// The owner field from v1 KV meeting data must decode into MeetingDBRaw so it
// flows through to the indexer event payload.
func TestMeetingDBRawUnmarshalOwner(t *testing.T) {
	input := `{"owner":{"user_id":"user-999","username":"oowner","name":"Olive Owner","email":"olive@example.com"}}`
	var m MeetingDBRaw
	require.NoError(t, json.Unmarshal([]byte(input), &m))
	assert.Equal(t, "user-999", m.Owner.UserID)
	assert.Equal(t, "oowner", m.Owner.Username)
	assert.Equal(t, "Olive Owner", m.Owner.Name)
	assert.Equal(t, "olive@example.com", m.Owner.Email)

	var empty MeetingDBRaw
	require.NoError(t, json.Unmarshal([]byte(`{}`), &empty))
	assert.Empty(t, empty.Owner.UserID)
}
