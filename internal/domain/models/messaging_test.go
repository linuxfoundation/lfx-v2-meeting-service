// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package models

import (
	"encoding/json"
	"testing"
)

func TestMessageActionConstants(t *testing.T) {
	tests := []struct {
		name     string
		action   MessageAction
		expected string
	}{
		{
			name:     "ActionCreated",
			action:   ActionCreated,
			expected: "created",
		},
		{
			name:     "ActionUpdated",
			action:   ActionUpdated,
			expected: "updated",
		},
		{
			name:     "ActionDeleted",
			action:   ActionDeleted,
			expected: "deleted",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.action) != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, string(tt.action))
			}
		})
	}
}

func TestMessagingSubjects(t *testing.T) {
	tests := []struct {
		name     string
		subject  string
		expected string
	}{
		{
			name:     "IndexMeetingSubject",
			subject:  IndexMeetingSubject,
			expected: "lfx.index.meeting",
		},
		{
			name:     "UpdateAccessMeetingSubject",
			subject:  UpdateAccessMeetingSubject,
			expected: "lfx.update_access.meeting",
		},
		{
			name:     "DeleteAllAccessMeetingSubject",
			subject:  DeleteAllAccessMeetingSubject,
			expected: "lfx.delete_all_access.meeting",
		},
		{
			name:     "MeetingsAPIQueue",
			subject:  MeetingsAPIQueue,
			expected: "lfx.meetings-api.queue",
		},
		{
			name:     "MeetingGetTitleSubject",
			subject:  MeetingGetTitleSubject,
			expected: "lfx.meetings-api.get_title",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.subject != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, tt.subject)
			}
		})
	}
}

func TestMeetingIndexerMessage_JSONSerialization(t *testing.T) {
	message := MeetingIndexerMessage{
		Action: ActionCreated,
		Headers: map[string]string{
			"Content-Type": "application/json",
			"User-Agent":   "meeting-service",
		},
		Data: map[string]interface{}{
			"uid":   "meeting-123",
			"title": "Test Meeting",
		},
		Tags: []string{"meeting", "project-123", "public"},
	}

	// Test JSON marshaling
	data, err := json.Marshal(message)
	if err != nil {
		t.Errorf("failed to marshal MeetingIndexerMessage: %v", err)
	}

	// Test JSON unmarshaling
	var unmarshaled MeetingIndexerMessage
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Errorf("failed to unmarshal MeetingIndexerMessage: %v", err)
	}

	// Compare fields
	if unmarshaled.Action != message.Action {
		t.Errorf("expected Action %q, got %q", message.Action, unmarshaled.Action)
	}
	if len(unmarshaled.Headers) != len(message.Headers) {
		t.Errorf("expected %d headers, got %d", len(message.Headers), len(unmarshaled.Headers))
	}
	for key, value := range message.Headers {
		if unmarshaled.Headers[key] != value {
			t.Errorf("expected header %q to be %q, got %q", key, value, unmarshaled.Headers[key])
		}
	}
	if len(unmarshaled.Tags) != len(message.Tags) {
		t.Errorf("expected %d tags, got %d", len(message.Tags), len(unmarshaled.Tags))
	}
	for i, tag := range message.Tags {
		if unmarshaled.Tags[i] != tag {
			t.Errorf("expected tag[%d] %q, got %q", i, tag, unmarshaled.Tags[i])
		}
	}
}

func TestMeetingAccessMessage_JSONSerialization(t *testing.T) {
	message := MeetingAccessMessage{
		UID:       "meeting-456",
		Public:    true,
		ParentUID: "project-789",
		Writers:   []string{"user-1", "user-2"},
		Auditors:  []string{"auditor-1", "auditor-2", "auditor-3"},
	}

	// Test JSON marshaling
	data, err := json.Marshal(message)
	if err != nil {
		t.Errorf("failed to marshal MeetingAccessMessage: %v", err)
	}

	// Test JSON unmarshaling
	var unmarshaled MeetingAccessMessage
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Errorf("failed to unmarshal MeetingAccessMessage: %v", err)
	}

	// Compare fields
	if unmarshaled.UID != message.UID {
		t.Errorf("expected UID %q, got %q", message.UID, unmarshaled.UID)
	}
	if unmarshaled.Public != message.Public {
		t.Errorf("expected Public %t, got %t", message.Public, unmarshaled.Public)
	}
	if unmarshaled.ParentUID != message.ParentUID {
		t.Errorf("expected ParentUID %q, got %q", message.ParentUID, unmarshaled.ParentUID)
	}
	if len(unmarshaled.Writers) != len(message.Writers) {
		t.Errorf("expected %d writers, got %d", len(message.Writers), len(unmarshaled.Writers))
	}
	for i, writer := range message.Writers {
		if unmarshaled.Writers[i] != writer {
			t.Errorf("expected writer[%d] %q, got %q", i, writer, unmarshaled.Writers[i])
		}
	}
	if len(unmarshaled.Auditors) != len(message.Auditors) {
		t.Errorf("expected %d auditors, got %d", len(message.Auditors), len(unmarshaled.Auditors))
	}
	for i, auditor := range message.Auditors {
		if unmarshaled.Auditors[i] != auditor {
			t.Errorf("expected auditor[%d] %q, got %q", i, auditor, unmarshaled.Auditors[i])
		}
	}
}

func TestMeetingIndexerMessage_WithDifferentDataTypes(t *testing.T) {
	tests := []struct {
		name string
		data interface{}
	}{
		{
			name: "string data",
			data: "simple string data",
		},
		{
			name: "map data",
			data: map[string]interface{}{
				"id":     123,
				"name":   "test",
				"active": true,
			},
		},
		{
			name: "array data",
			data: []string{"item1", "item2", "item3"},
		},
		{
			name: "nil data",
			data: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			message := MeetingIndexerMessage{
				Action: ActionUpdated,
				Headers: map[string]string{
					"test": "header",
				},
				Data: tt.data,
				Tags: []string{"test"},
			}

			data, err := json.Marshal(message)
			if err != nil {
				t.Errorf("failed to marshal message with %s: %v", tt.name, err)
			}

			var unmarshaled MeetingIndexerMessage
			err = json.Unmarshal(data, &unmarshaled)
			if err != nil {
				t.Errorf("failed to unmarshal message with %s: %v", tt.name, err)
			}

			if unmarshaled.Action != message.Action {
				t.Errorf("Action mismatch for %s", tt.name)
			}
		})
	}
}

func TestMeetingAccessMessage_EmptySlices(t *testing.T) {
	message := MeetingAccessMessage{
		UID:       "meeting-empty",
		Public:    false,
		ParentUID: "parent-empty",
		Writers:   []string{},
		Auditors:  []string{},
	}

	data, err := json.Marshal(message)
	if err != nil {
		t.Errorf("failed to marshal message with empty slices: %v", err)
	}

	var unmarshaled MeetingAccessMessage
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Errorf("failed to unmarshal message with empty slices: %v", err)
	}

	if len(unmarshaled.Writers) != 0 {
		t.Errorf("expected empty Writers slice, got %d items", len(unmarshaled.Writers))
	}
	if len(unmarshaled.Auditors) != 0 {
		t.Errorf("expected empty Auditors slice, got %d items", len(unmarshaled.Auditors))
	}
}
