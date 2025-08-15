// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package mocks

import (
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/infrastructure/zoom/api"
)

// MockClient is a complete mock implementation of the Zoom API client
// It embeds both meeting and user mocks to provide full API coverage
type MockClient struct {
	*MockMeetingsAPI
	*MockUsersAPI
}

// NewMockClient creates a new mock client with default implementations
func NewMockClient() *MockClient {
	return &MockClient{
		MockMeetingsAPI: &MockMeetingsAPI{},
		MockUsersAPI:    &MockUsersAPI{},
	}
}

// Ensure MockClient implements ClientAPI interface
var _ api.ClientAPI = (*MockClient)(nil)
