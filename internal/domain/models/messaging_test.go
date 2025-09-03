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
		{
			name:     "MeetingDeletedSubject",
			subject:  MeetingDeletedSubject,
			expected: "lfx.meetings-api.meeting_deleted",
		},
		{
			name:     "MeetingCreatedSubject",
			subject:  MeetingCreatedSubject,
			expected: "lfx.meetings-api.meeting_created",
		},
		{
			name:     "MeetingUpdatedSubject",
			subject:  MeetingUpdatedSubject,
			expected: "lfx.meetings-api.meeting_updated",
		},
		{
			name:     "CommitteeGetNameSubject",
			subject:  CommitteeGetNameSubject,
			expected: "lfx.committee-api.get_name",
		},
		{
			name:     "CommitteeGetMembersSubject",
			subject:  CommitteeGetMembersSubject,
			expected: "lfx.committee-api.get_members",
		},
		{
			name:     "CommitteeMemberCreatedSubject",
			subject:  CommitteeMemberCreatedSubject,
			expected: "lfx.committee-api.member_created",
		},
		{
			name:     "CommitteeMemberDeletedSubject",
			subject:  CommitteeMemberDeletedSubject,
			expected: "lfx.committee-api.member_deleted",
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

func TestPlatformConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant string
		expected string
	}{
		{"PlatformZoom", PlatformZoom, "Zoom"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.constant != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, tt.constant)
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
		UID:        "meeting-456",
		Public:     true,
		ProjectUID: "project-789",
		Organizers: []string{"user-1", "user-2"},
		Committees: []string{"committee-1", "committee-2", "committee-3"},
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
	if unmarshaled.ProjectUID != message.ProjectUID {
		t.Errorf("expected ProjectUID %q, got %q", message.ProjectUID, unmarshaled.ProjectUID)
	}
	if len(unmarshaled.Organizers) != len(message.Organizers) {
		t.Errorf("expected %d organizers, got %d", len(message.Organizers), len(unmarshaled.Organizers))
	}
	for i, organizer := range message.Organizers {
		if unmarshaled.Organizers[i] != organizer {
			t.Errorf("expected organizer[%d] %q, got %q", i, organizer, unmarshaled.Organizers[i])
		}
	}
	if len(unmarshaled.Committees) != len(message.Committees) {
		t.Errorf("expected %d committees, got %d", len(message.Committees), len(unmarshaled.Committees))
	}
	for i, committee := range message.Committees {
		if unmarshaled.Committees[i] != committee {
			t.Errorf("expected committee[%d] %q, got %q", i, committee, unmarshaled.Committees[i])
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
		UID:        "meeting-empty",
		Public:     false,
		ProjectUID: "project-empty",
		Organizers: []string{},
		Committees: []string{},
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

	if len(unmarshaled.Organizers) != 0 {
		t.Errorf("expected empty Organizers slice, got %d items", len(unmarshaled.Organizers))
	}
	if len(unmarshaled.Committees) != 0 {
		t.Errorf("expected empty Committees slice, got %d items", len(unmarshaled.Committees))
	}
}
