// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package domain

import (
	"context"
	"testing"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
)

// mockMessage implements the Message interface for testing
type mockMessage struct {
	subject   string
	data      []byte
	responded bool
}

func (m *mockMessage) Subject() string {
	return m.subject
}

func (m *mockMessage) Data() []byte {
	return m.data
}

func (m *mockMessage) Respond(data []byte) error {
	m.responded = true
	return nil
}

// mockMessageHandler implements the MessageHandler interface for testing
type mockMessageHandler struct {
	handledMessages []Message
}

func (m *mockMessageHandler) HandleMessage(ctx context.Context, msg Message) {
	m.handledMessages = append(m.handledMessages, msg)
}

// mockMessageBuilder implements the MessageBuilder interface for testing
type mockMessageBuilder struct {
	indexMeetingCalls        []models.MeetingBase
	deleteIndexMeetingCalls  []string
	updateAccessMeetingCalls []models.MeetingAccessMessage
	deleteAllAccessCalls     []string
}

func (m *mockMessageBuilder) SendIndexMeeting(ctx context.Context, action models.MessageAction, data models.MeetingBase) error {
	m.indexMeetingCalls = append(m.indexMeetingCalls, data)
	return nil
}

func (m *mockMessageBuilder) SendDeleteIndexMeeting(ctx context.Context, data string) error {
	m.deleteIndexMeetingCalls = append(m.deleteIndexMeetingCalls, data)
	return nil
}

func (m *mockMessageBuilder) SendUpdateAccessMeeting(ctx context.Context, data models.MeetingAccessMessage) error {
	m.updateAccessMeetingCalls = append(m.updateAccessMeetingCalls, data)
	return nil
}

func (m *mockMessageBuilder) SendDeleteAllAccessMeeting(ctx context.Context, data string) error {
	m.deleteAllAccessCalls = append(m.deleteAllAccessCalls, data)
	return nil
}

func TestMessage_Interface(t *testing.T) {
	msg := &mockMessage{
		subject: "test.subject",
		data:    []byte("test data"),
	}

	if msg.Subject() != "test.subject" {
		t.Errorf("expected subject 'test.subject', got %q", msg.Subject())
	}

	if string(msg.Data()) != "test data" {
		t.Errorf("expected data 'test data', got %q", string(msg.Data()))
	}

	if err := msg.Respond([]byte("response")); err != nil {
		t.Errorf("expected no error on respond, got %v", err)
	}

	if !msg.responded {
		t.Error("expected message to be marked as responded")
	}
}

func TestMessageHandler_Interface(t *testing.T) {
	handler := &mockMessageHandler{}
	msg := &mockMessage{subject: "test", data: []byte("data")}

	handler.HandleMessage(context.Background(), msg)

	if len(handler.handledMessages) != 1 {
		t.Errorf("expected 1 handled message, got %d", len(handler.handledMessages))
	}

	if handler.handledMessages[0] != msg {
		t.Error("expected handled message to be the same as input message")
	}
}

func TestMessageBuilder_Interface(t *testing.T) {
	ctx := context.Background()
	builder := &mockMessageBuilder{}

	// Test SendIndexMeeting
	meeting := models.MeetingBase{UID: "test-uid"}
	err := builder.SendIndexMeeting(ctx, models.ActionCreated, meeting)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if len(builder.indexMeetingCalls) != 1 {
		t.Errorf("expected 1 index meeting call, got %d", len(builder.indexMeetingCalls))
	}

	// Test SendDeleteIndexMeeting
	err = builder.SendDeleteIndexMeeting(ctx, "delete-uid")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if len(builder.deleteIndexMeetingCalls) != 1 {
		t.Errorf("expected 1 delete index call, got %d", len(builder.deleteIndexMeetingCalls))
	}

	// Test SendUpdateAccessMeeting
	accessMsg := models.MeetingAccessMessage{UID: "access-uid"}
	err = builder.SendUpdateAccessMeeting(ctx, accessMsg)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if len(builder.updateAccessMeetingCalls) != 1 {
		t.Errorf("expected 1 update access call, got %d", len(builder.updateAccessMeetingCalls))
	}

	// Test SendDeleteAllAccessMeeting
	err = builder.SendDeleteAllAccessMeeting(ctx, "delete-all-uid")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if len(builder.deleteAllAccessCalls) != 1 {
		t.Errorf("expected 1 delete all access call, got %d", len(builder.deleteAllAccessCalls))
	}
}
