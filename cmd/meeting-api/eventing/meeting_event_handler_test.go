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
