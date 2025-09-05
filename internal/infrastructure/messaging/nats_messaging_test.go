// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package messaging

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/constants"
	"github.com/nats-io/nats.go"
)

// mockNatsConn implements INatsConn for testing
type mockNatsConn struct {
	connected     bool
	publishedMsgs []publishedMessage
	publishError  error
}

type publishedMessage struct {
	subject string
	data    []byte
}

func (m *mockNatsConn) IsConnected() bool {
	return m.connected
}

func (m *mockNatsConn) Publish(subj string, data []byte) error {
	m.publishedMsgs = append(m.publishedMsgs, publishedMessage{
		subject: subj,
		data:    data,
	})
	if m.publishError != nil {
		return m.publishError
	}
	return nil
}

func (m *mockNatsConn) Request(subj string, data []byte, timeout time.Duration) (*nats.Msg, error) {
	// For testing purposes, return a simple mock response
	m.publishedMsgs = append(m.publishedMsgs, publishedMessage{
		subject: subj,
		data:    data,
	})
	if m.publishError != nil {
		return nil, m.publishError
	}
	// Return a mock response message
	return &nats.Msg{
		Subject: subj,
		Data:    []byte("mock response"),
	}, nil
}

func TestMessageBuilder_publish(t *testing.T) {
	tests := []struct {
		name          string
		connected     bool
		publishError  error
		subject       string
		data          []byte
		expectError   bool
		expectedCalls int
	}{
		{
			name:          "successful send",
			connected:     true,
			publishError:  nil,
			subject:       "test.subject",
			data:          []byte("test data"),
			expectError:   false,
			expectedCalls: 1,
		},
		{
			name:          "publish error",
			connected:     true,
			publishError:  errors.New("publish failed"),
			subject:       "test.subject",
			data:          []byte("test data"),
			expectError:   true,
			expectedCalls: 1,
		},
		{
			name:          "disconnected",
			connected:     false,
			publishError:  nil,
			subject:       "test.subject",
			data:          []byte("test data"),
			expectError:   false,
			expectedCalls: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockConn := &mockNatsConn{
				connected:    tt.connected,
				publishError: tt.publishError,
			}
			builder := &MessageBuilder{
				NatsConn: mockConn,
			}

			ctx := context.Background()
			err := builder.publish(ctx, tt.subject, tt.data)

			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("expected no error but got: %v", err)
			}
			if len(mockConn.publishedMsgs) != tt.expectedCalls {
				t.Errorf("expected %d publish calls, got %d", tt.expectedCalls, len(mockConn.publishedMsgs))
			}
			if len(mockConn.publishedMsgs) > 0 {
				msg := mockConn.publishedMsgs[0]
				if msg.subject != tt.subject {
					t.Errorf("expected subject %q, got %q", tt.subject, msg.subject)
				}
				if string(msg.data) != string(tt.data) {
					t.Errorf("expected data %q, got %q", string(tt.data), string(msg.data))
				}
			}
		})
	}
}

func TestMessageBuilder_SendIndexMeeting(t *testing.T) {
	mockConn := &mockNatsConn{
		connected: true,
	}
	builder := &MessageBuilder{
		NatsConn: mockConn,
	}

	ctx := context.Background()
	meeting := models.MeetingBase{
		UID:         "test-meeting-uid",
		Title:       "Test Meeting",
		ProjectUID:  "project-123",
		Description: "Test Description",
	}

	err := builder.SendIndexMeeting(ctx, models.ActionCreated, meeting)
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}

	if len(mockConn.publishedMsgs) != 1 {
		t.Errorf("expected 1 published message, got %d", len(mockConn.publishedMsgs))
		return
	}

	msg := mockConn.publishedMsgs[0]
	if msg.subject != models.IndexMeetingSubject {
		t.Errorf("expected subject %q, got %q", models.IndexMeetingSubject, msg.subject)
	}

	// Parse the message to verify structure
	var indexerMsg models.MeetingIndexerMessage
	err = json.Unmarshal(msg.data, &indexerMsg)
	if err != nil {
		t.Errorf("failed to unmarshal message: %v", err)
		return
	}

	if indexerMsg.Action != models.ActionCreated {
		t.Errorf("expected action %q, got %q", models.ActionCreated, indexerMsg.Action)
	}
}

func TestMessageBuilder_SendIndexMeeting_WithContext(t *testing.T) {
	mockConn := &mockNatsConn{
		connected: true,
	}
	builder := &MessageBuilder{
		NatsConn: mockConn,
	}

	// Create context with authorization and principal
	ctx := context.Background()
	ctx = context.WithValue(ctx, constants.AuthorizationContextID, "Bearer token123")
	ctx = context.WithValue(ctx, constants.PrincipalContextID, "user123")

	meeting := models.MeetingBase{
		UID:   "test-meeting-uid",
		Title: "Test Meeting",
	}

	err := builder.SendIndexMeeting(ctx, models.ActionUpdated, meeting)
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}

	if len(mockConn.publishedMsgs) != 1 {
		t.Errorf("expected 1 published message, got %d", len(mockConn.publishedMsgs))
		return
	}

	msg := mockConn.publishedMsgs[0]
	var indexerMsg models.MeetingIndexerMessage
	err = json.Unmarshal(msg.data, &indexerMsg)
	if err != nil {
		t.Errorf("failed to unmarshal message: %v", err)
		return
	}

	// Check headers
	if indexerMsg.Headers[constants.AuthorizationHeader] != "Bearer token123" {
		t.Errorf("expected authorization header %q, got %q", "Bearer token123", indexerMsg.Headers[constants.AuthorizationHeader])
	}
	if indexerMsg.Headers[constants.XOnBehalfOfHeader] != "user123" {
		t.Errorf("expected x-on-behalf-of header %q, got %q", "user123", indexerMsg.Headers[constants.XOnBehalfOfHeader])
	}
}

func TestMessageBuilder_SendDeleteIndexMeeting(t *testing.T) {
	mockConn := &mockNatsConn{
		connected: true,
	}
	builder := &MessageBuilder{
		NatsConn: mockConn,
	}

	ctx := context.Background()
	meetingUID := "delete-meeting-uid"

	err := builder.SendDeleteIndexMeeting(ctx, meetingUID)
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}

	if len(mockConn.publishedMsgs) != 1 {
		t.Errorf("expected 1 published message, got %d", len(mockConn.publishedMsgs))
		return
	}

	msg := mockConn.publishedMsgs[0]
	if msg.subject != models.IndexMeetingSubject {
		t.Errorf("expected subject %q, got %q", models.IndexMeetingSubject, msg.subject)
	}

	var indexerMsg models.MeetingIndexerMessage
	err = json.Unmarshal(msg.data, &indexerMsg)
	if err != nil {
		t.Errorf("failed to unmarshal message: %v", err)
		return
	}

	if indexerMsg.Action != models.ActionDeleted {
		t.Errorf("expected action %q, got %q", models.ActionDeleted, indexerMsg.Action)
	}

	// Check that data contains the meeting UID
	// The data might be base64 encoded, so we need to decode it
	if dataStr, ok := indexerMsg.Data.(string); ok {
		// Try to decode from base64 first
		if decoded, err := base64.StdEncoding.DecodeString(dataStr); err == nil {
			decodedStr := string(decoded)
			if decodedStr != meetingUID {
				t.Errorf("expected decoded data %q, got %q", meetingUID, decodedStr)
			}
		} else if dataStr != meetingUID {
			// If not base64, compare directly
			t.Errorf("expected data %q, got %q", meetingUID, dataStr)
		}
	} else {
		t.Errorf("expected data to be string, got %T", indexerMsg.Data)
	}
}

func TestMessageBuilder_SendUpdateAccessMeeting(t *testing.T) {
	mockConn := &mockNatsConn{
		connected: true,
	}
	builder := &MessageBuilder{
		NatsConn: mockConn,
	}

	ctx := context.Background()
	accessMsg := models.MeetingAccessMessage{
		UID:        "access-meeting-uid",
		Public:     true,
		ProjectUID: "project-123",
		Organizers: []string{"organizer1", "organizer2"},
		Committees: []string{"committee1", "committee2"},
	}

	err := builder.SendUpdateAccessMeeting(ctx, accessMsg)
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}

	if len(mockConn.publishedMsgs) != 1 {
		t.Errorf("expected 1 published message, got %d", len(mockConn.publishedMsgs))
		return
	}

	msg := mockConn.publishedMsgs[0]
	if msg.subject != models.UpdateAccessMeetingSubject {
		t.Errorf("expected subject %q, got %q", models.UpdateAccessMeetingSubject, msg.subject)
	}

	// Parse and verify the access message
	var receivedMsg models.MeetingAccessMessage
	err = json.Unmarshal(msg.data, &receivedMsg)
	if err != nil {
		t.Errorf("failed to unmarshal access message: %v", err)
		return
	}

	if receivedMsg.UID != accessMsg.UID {
		t.Errorf("expected UID %q, got %q", accessMsg.UID, receivedMsg.UID)
	}
	if receivedMsg.Public != accessMsg.Public {
		t.Errorf("expected Public %t, got %t", accessMsg.Public, receivedMsg.Public)
	}
	if receivedMsg.ProjectUID != accessMsg.ProjectUID {
		t.Errorf("expected ProjectUID %q, got %q", accessMsg.ProjectUID, receivedMsg.ProjectUID)
	}
	if len(receivedMsg.Organizers) != len(accessMsg.Organizers) {
		t.Errorf("expected %d organizers, got %d", len(accessMsg.Organizers), len(receivedMsg.Organizers))
	}
	for i, organizer := range receivedMsg.Organizers {
		if organizer != accessMsg.Organizers[i] {
			t.Errorf("expected organizer %q, got %q", accessMsg.Organizers[i], organizer)
		}
	}
	if len(receivedMsg.Committees) != len(accessMsg.Committees) {
		t.Errorf("expected %d committees, got %d", len(accessMsg.Committees), len(receivedMsg.Committees))
	}
	for i, committee := range receivedMsg.Committees {
		if committee != accessMsg.Committees[i] {
			t.Errorf("expected committee %q, got %q", accessMsg.Committees[i], committee)
		}
	}
}

func TestMessageBuilder_SendDeleteAllAccessMeeting(t *testing.T) {
	mockConn := &mockNatsConn{
		connected: true,
	}
	builder := &MessageBuilder{
		NatsConn: mockConn,
	}

	ctx := context.Background()
	meetingUID := "delete-access-meeting-uid"

	err := builder.SendDeleteAllAccessMeeting(ctx, meetingUID)
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}

	if len(mockConn.publishedMsgs) != 1 {
		t.Errorf("expected 1 published message, got %d", len(mockConn.publishedMsgs))
		return
	}

	msg := mockConn.publishedMsgs[0]
	if msg.subject != models.DeleteAllAccessMeetingSubject {
		t.Errorf("expected subject %q, got %q", models.DeleteAllAccessMeetingSubject, msg.subject)
	}

	// Check that data contains the meeting UID
	if string(msg.data) != meetingUID {
		t.Errorf("expected data %q, got %q", meetingUID, string(msg.data))
	}
}

func TestMessageBuilder_PublishErrors(t *testing.T) {
	publishError := errors.New("publish failed")
	mockConn := &mockNatsConn{
		connected:    true,
		publishError: publishError,
	}
	builder := &MessageBuilder{
		NatsConn: mockConn,
	}

	ctx := context.Background()
	meeting := models.MeetingBase{UID: "test-uid", Title: "Test"}

	// Test SendIndexMeeting error
	err := builder.SendIndexMeeting(ctx, models.ActionCreated, meeting)
	if err == nil {
		t.Error("expected error from SendIndexMeeting but got none")
	}

	// Test SendDeleteIndexMeeting error
	err = builder.SendDeleteIndexMeeting(ctx, "test-uid")
	if err == nil {
		t.Error("expected error from SendDeleteIndexMeeting but got none")
	}

	// Test SendUpdateAccessMeeting error
	accessMsg := models.MeetingAccessMessage{UID: "test-uid"}
	err = builder.SendUpdateAccessMeeting(ctx, accessMsg)
	if err == nil {
		t.Error("expected error from SendUpdateAccessMeeting but got none")
	}

	// Test SendDeleteAllAccessMeeting error
	err = builder.SendDeleteAllAccessMeeting(ctx, "test-uid")
	if err == nil {
		t.Error("expected error from SendDeleteAllAccessMeeting but got none")
	}

	// Test SendIndexMeetingSettings error
	settings := models.MeetingSettings{UID: "test-uid", Organizers: []string{"org1"}}
	err = builder.SendIndexMeetingSettings(ctx, models.ActionCreated, settings)
	if err == nil {
		t.Error("expected error from SendIndexMeetingSettings but got none")
	}

	// Test SendDeleteIndexMeetingSettings error
	err = builder.SendDeleteIndexMeetingSettings(ctx, "test-uid")
	if err == nil {
		t.Error("expected error from SendDeleteIndexMeetingSettings but got none")
	}

	// Test SendPutMeetingRegistrantAccess error
	registrantMsg := models.MeetingRegistrantAccessMessage{MeetingUID: "meeting-uid", UID: "registrant-uid"}
	err = builder.SendPutMeetingRegistrantAccess(ctx, registrantMsg)
	if err == nil {
		t.Error("expected error from SendPutMeetingRegistrantAccess but got none")
	}

	// Test SendRemoveMeetingRegistrantAccess error
	err = builder.SendRemoveMeetingRegistrantAccess(ctx, registrantMsg)
	if err == nil {
		t.Error("expected error from SendRemoveMeetingRegistrantAccess but got none")
	}
}

func TestMessageBuilder_SendIndexMeetingSettings(t *testing.T) {
	mockConn := &mockNatsConn{
		connected: true,
	}
	builder := &MessageBuilder{
		NatsConn: mockConn,
	}

	ctx := context.Background()
	settings := models.MeetingSettings{
		UID:        "test-settings-uid",
		Organizers: []string{"organizer1", "organizer2"},
	}

	err := builder.SendIndexMeetingSettings(ctx, models.ActionCreated, settings)
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}

	if len(mockConn.publishedMsgs) != 1 {
		t.Errorf("expected 1 published message, got %d", len(mockConn.publishedMsgs))
		return
	}

	msg := mockConn.publishedMsgs[0]
	if msg.subject != models.IndexMeetingSettingsSubject {
		t.Errorf("expected subject %q, got %q", models.IndexMeetingSettingsSubject, msg.subject)
	}

	// Parse the message to verify structure
	var indexerMsg models.MeetingIndexerMessage
	err = json.Unmarshal(msg.data, &indexerMsg)
	if err != nil {
		t.Errorf("failed to unmarshal message: %v", err)
		return
	}

	if indexerMsg.Action != models.ActionCreated {
		t.Errorf("expected action %q, got %q", models.ActionCreated, indexerMsg.Action)
	}
}

func TestMessageBuilder_SendDeleteIndexMeetingSettings(t *testing.T) {
	mockConn := &mockNatsConn{
		connected: true,
	}
	builder := &MessageBuilder{
		NatsConn: mockConn,
	}

	ctx := context.Background()
	settingsUID := "delete-settings-uid"

	err := builder.SendDeleteIndexMeetingSettings(ctx, settingsUID)
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}

	if len(mockConn.publishedMsgs) != 1 {
		t.Errorf("expected 1 published message, got %d", len(mockConn.publishedMsgs))
		return
	}

	msg := mockConn.publishedMsgs[0]
	if msg.subject != models.IndexMeetingSettingsSubject {
		t.Errorf("expected subject %q, got %q", models.IndexMeetingSettingsSubject, msg.subject)
	}

	var indexerMsg models.MeetingIndexerMessage
	err = json.Unmarshal(msg.data, &indexerMsg)
	if err != nil {
		t.Errorf("failed to unmarshal message: %v", err)
		return
	}

	if indexerMsg.Action != models.ActionDeleted {
		t.Errorf("expected action %q, got %q", models.ActionDeleted, indexerMsg.Action)
	}

	// Check that data contains the settings UID
	if dataStr, ok := indexerMsg.Data.(string); ok {
		if dataStr != settingsUID {
			t.Errorf("expected data %q, got %q", settingsUID, dataStr)
		}
	} else {
		t.Errorf("expected data to be string, got %T", indexerMsg.Data)
	}
}

func TestMessageBuilder_SendPutMeetingRegistrantAccess(t *testing.T) {
	mockConn := &mockNatsConn{
		connected: true,
	}
	builder := &MessageBuilder{
		NatsConn: mockConn,
	}

	ctx := context.Background()
	registrantMsg := models.MeetingRegistrantAccessMessage{
		MeetingUID: "meeting-uid-123",
		UID:        "registrant-uid-456",
		Username:   "john.doe",
		Host:       false,
	}

	err := builder.SendPutMeetingRegistrantAccess(ctx, registrantMsg)
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}

	if len(mockConn.publishedMsgs) != 1 {
		t.Errorf("expected 1 published message, got %d", len(mockConn.publishedMsgs))
		return
	}

	msg := mockConn.publishedMsgs[0]
	if msg.subject != models.PutRegistrantMeetingSubject {
		t.Errorf("expected subject %q, got %q", models.PutRegistrantMeetingSubject, msg.subject)
	}

	// Parse and verify the registrant message
	var receivedMsg models.MeetingRegistrantAccessMessage
	err = json.Unmarshal(msg.data, &receivedMsg)
	if err != nil {
		t.Errorf("failed to unmarshal registrant message: %v", err)
		return
	}

	if receivedMsg.MeetingUID != registrantMsg.MeetingUID {
		t.Errorf("expected MeetingUID %q, got %q", registrantMsg.MeetingUID, receivedMsg.MeetingUID)
	}
	if receivedMsg.UID != registrantMsg.UID {
		t.Errorf("expected UID %q, got %q", registrantMsg.UID, receivedMsg.UID)
	}
	if receivedMsg.Username != registrantMsg.Username {
		t.Errorf("expected Username %q, got %q", registrantMsg.Username, receivedMsg.Username)
	}
	if receivedMsg.Host != registrantMsg.Host {
		t.Errorf("expected Host %t, got %t", registrantMsg.Host, receivedMsg.Host)
	}
}

func TestMessageBuilder_SendRemoveMeetingRegistrantAccess(t *testing.T) {
	mockConn := &mockNatsConn{
		connected: true,
	}
	builder := &MessageBuilder{
		NatsConn: mockConn,
	}

	ctx := context.Background()
	registrantMsg := models.MeetingRegistrantAccessMessage{
		MeetingUID: "meeting-uid-789",
		UID:        "registrant-uid-012",
		Username:   "jane.smith",
		Host:       true,
	}

	err := builder.SendRemoveMeetingRegistrantAccess(ctx, registrantMsg)
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}

	if len(mockConn.publishedMsgs) != 1 {
		t.Errorf("expected 1 published message, got %d", len(mockConn.publishedMsgs))
		return
	}

	msg := mockConn.publishedMsgs[0]
	if msg.subject != models.RemoveRegistrantMeetingSubject {
		t.Errorf("expected subject %q, got %q", models.RemoveRegistrantMeetingSubject, msg.subject)
	}

	// Parse and verify the registrant message
	var receivedMsg models.MeetingRegistrantAccessMessage
	err = json.Unmarshal(msg.data, &receivedMsg)
	if err != nil {
		t.Errorf("failed to unmarshal registrant message: %v", err)
		return
	}

	if receivedMsg.MeetingUID != registrantMsg.MeetingUID {
		t.Errorf("expected MeetingUID %q, got %q", registrantMsg.MeetingUID, receivedMsg.MeetingUID)
	}
	if receivedMsg.UID != registrantMsg.UID {
		t.Errorf("expected UID %q, got %q", registrantMsg.UID, receivedMsg.UID)
	}
	if receivedMsg.Username != registrantMsg.Username {
		t.Errorf("expected Username %q, got %q", registrantMsg.Username, receivedMsg.Username)
	}
	if receivedMsg.Host != registrantMsg.Host {
		t.Errorf("expected Host %t, got %t", registrantMsg.Host, receivedMsg.Host)
	}
}
