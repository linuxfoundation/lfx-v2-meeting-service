// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package mocks

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/infrastructure/zoom/api"
)

// MockMeetingsAPI is a mock implementation of Zoom meeting API operations for testing
type MockMeetingsAPI struct {
	CreateMeetingFunc func(ctx context.Context, userID string, request *api.CreateMeetingRequest) (*api.CreateMeetingResponse, error)
	UpdateMeetingFunc func(ctx context.Context, meetingID string, request *api.UpdateMeetingRequest) error
	DeleteMeetingFunc func(ctx context.Context, meetingID string) error
}

// CreateMeeting mocks the CreateMeeting API call
func (m *MockMeetingsAPI) CreateMeeting(ctx context.Context, userID string, request *api.CreateMeetingRequest) (*api.CreateMeetingResponse, error) {
	if m.CreateMeetingFunc != nil {
		return m.CreateMeetingFunc(ctx, userID, request)
	}
	// Default mock response
	return &api.CreateMeetingResponse{
		ID:       123456789,
		UUID:     "test-uuid-123",
		HostID:   userID,
		Topic:    request.Topic,
		Type:     request.Type,
		Status:   "waiting",
		Duration: request.Duration,
		Timezone: request.Timezone,
		JoinURL:  "https://zoom.us/j/123456789",
		Password: "test123",
	}, nil
}

// UpdateMeeting mocks the UpdateMeeting API call
func (m *MockMeetingsAPI) UpdateMeeting(ctx context.Context, meetingID string, request *api.UpdateMeetingRequest) error {
	if m.UpdateMeetingFunc != nil {
		return m.UpdateMeetingFunc(ctx, meetingID, request)
	}
	return nil
}

// DeleteMeeting mocks the DeleteMeeting API call
func (m *MockMeetingsAPI) DeleteMeeting(ctx context.Context, meetingID string) error {
	if m.DeleteMeetingFunc != nil {
		return m.DeleteMeetingFunc(ctx, meetingID)
	}
	return nil
}
