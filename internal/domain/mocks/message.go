// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package mocks

import (
	"github.com/stretchr/testify/mock"
)

// MockMessage implements Message for testing with full mock capabilities
type MockMessage struct {
	mock.Mock
	data      []byte
	subject   string
	hasReply  bool
	responded bool
}

func (m *MockMessage) Subject() string {
	// Use field value if set
	if m.subject != "" {
		return m.subject
	}
	// Check if there are specific expectations for Subject
	for _, call := range m.ExpectedCalls {
		if call.Method == "Subject" {
			args := m.Called()
			return args.String(0)
		}
	}
	// Default to empty string
	return ""
}

func (m *MockMessage) Data() []byte {
	// Use field value if set
	if m.data != nil {
		return m.data
	}
	// Check if there are specific expectations for Data
	for _, call := range m.ExpectedCalls {
		if call.Method == "Data" {
			args := m.Called()
			return args.Get(0).([]byte)
		}
	}
	// Default to nil
	return nil
}

func (m *MockMessage) HasReply() bool {
	// Check if there are specific expectations for HasReply
	for _, call := range m.ExpectedCalls {
		if call.Method == "HasReply" {
			args := m.Called()
			return args.Bool(0)
		}
	}
	// Default to field value if no expectations set
	return m.hasReply
}

func (m *MockMessage) Respond(data []byte) error {
	m.responded = true
	// Check if there are specific expectations for Respond
	for _, call := range m.ExpectedCalls {
		if call.Method == "Respond" {
			args := m.Called(data)
			return args.Error(0)
		}
	}
	// Default to no error
	return nil
}

// NewMockMessage creates a mock message for testing
func NewMockMessage(data []byte, subject string) *MockMessage {
	return &MockMessage{
		data:    data,
		subject: subject,
	}
}

// NewMockMessageWithReply creates a mock message with reply capability
func NewMockMessageWithReply(data []byte, subject string, hasReply bool) *MockMessage {
	return &MockMessage{
		data:     data,
		subject:  subject,
		hasReply: hasReply,
	}
}
