// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package zoom

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/infrastructure/zoom/api"
	"github.com/stretchr/testify/mock"
)

// MockClient is a mock implementation of the Zoom client for testing
type MockClient struct {
	mock.Mock
}

// CreateMeeting mocks the CreateMeeting method
func (m *MockClient) CreateMeeting(ctx context.Context, meeting *models.MeetingBase) (string, string, error) {
	args := m.Called(ctx, meeting)
	return args.String(0), args.String(1), args.Error(2)
}

// UpdateMeeting mocks the UpdateMeeting method
func (m *MockClient) UpdateMeeting(ctx context.Context, meetingID string, meeting *models.MeetingBase) error {
	args := m.Called(ctx, meetingID, meeting)
	return args.Error(0)
}

// DeleteMeeting mocks the DeleteMeeting method
func (m *MockClient) DeleteMeeting(ctx context.Context, meetingID string) error {
	args := m.Called(ctx, meetingID)
	return args.Error(0)
}

// GetUsers mocks the GetUsers method
func (m *MockClient) GetUsers(ctx context.Context) ([]api.ZoomUser, error) {
	args := m.Called(ctx)
	return args.Get(0).([]api.ZoomUser), args.Error(1)
}
