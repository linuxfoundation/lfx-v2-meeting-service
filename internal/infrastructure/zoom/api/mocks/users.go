// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package mocks

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/infrastructure/zoom/api"
)

// MockUsersAPI is a mock implementation of Zoom user API operations for testing
type MockUsersAPI struct {
	GetUsersFunc func(ctx context.Context) ([]api.ZoomUser, error)
}

// GetUsers mocks the GetUsers API call
func (m *MockUsersAPI) GetUsers(ctx context.Context) ([]api.ZoomUser, error) {
	if m.GetUsersFunc != nil {
		return m.GetUsersFunc(ctx)
	}
	// Default mock response with various user types and statuses for testing
	return []api.ZoomUser{
		{
			ID:        "user1",
			Email:     "user1@example.com",
			FirstName: "John",
			LastName:  "Doe",
			Type:      api.UserTypeLicensed,
			Status:    api.UserStatusActive,
		},
		{
			ID:        "user2",
			Email:     "user2@example.com",
			FirstName: "Jane",
			LastName:  "Smith",
			Type:      api.UserTypeBasic,
			Status:    api.UserStatusActive,
		},
		{
			ID:        "user3",
			Email:     "user3@example.com",
			FirstName: "Bob",
			LastName:  "Johnson",
			Type:      api.UserTypeLicensed,
			Status:    api.UserStatusInactive,
		},
	}, nil
}
