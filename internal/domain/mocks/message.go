// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package mocks

import (
	"github.com/stretchr/testify/mock"
)

// MockMessage implements Message for testing
type MockMessage struct {
	mock.Mock
	data    []byte
	subject string
}

func (m *MockMessage) Subject() string {
	return m.subject
}

func (m *MockMessage) Data() []byte {
	return m.data
}

func (m *MockMessage) HasReply() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *MockMessage) Respond(data []byte) error {
	args := m.Called(data)
	return args.Error(0)
}

// NewMockMessage creates a mock message for testing
func NewMockMessage(data []byte, subject string) *MockMessage {
	return &MockMessage{
		data:    data,
		subject: subject,
	}
}
