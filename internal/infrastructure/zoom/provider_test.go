// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package zoom

import (
	"context"
	"errors"
	"testing"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/infrastructure/zoom/api"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/infrastructure/zoom/api/mocks"
)

func TestNewZoomProvider(t *testing.T) {
	tests := []struct {
		name string
	}{
		{
			name: "creates provider successfully",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := mocks.NewMockClient()
			provider := NewZoomProvider(client)

			if provider == nil {
				t.Fatal("NewZoomProvider returned nil")
			}
			if provider.client == nil {
				t.Error("provider client is nil")
			}
			if provider.cachedUsers == nil {
				t.Error("provider cachedUsers is nil")
			}
		})
	}
}

func TestZoomProvider_CreateMeeting(t *testing.T) {
	tests := []struct {
		name          string
		meeting       *models.MeetingBase
		setupMock     func(*mocks.MockClient)
		expectedError bool
		expectedID    string
	}{
		{
			name: "successful creation",
			meeting: &models.MeetingBase{
				Title:            "Test Meeting",
				Description:      "Test Description",
				Duration:         60,
				Timezone:         "UTC",
				RecordingEnabled: true,
			},
			setupMock: func(client *mocks.MockClient) {
				client.CreateMeetingFunc = func(ctx context.Context, userID string, request *api.CreateMeetingRequest) (*api.CreateMeetingResponse, error) {
					return &api.CreateMeetingResponse{
						ID:       987654321,
						JoinURL:  "https://zoom.us/j/987654321",
						Password: "pass123",
					}, nil
				}
			},
			expectedError: false,
			expectedID:    "987654321",
		},
		{
			name: "creation with AI companion enabled",
			meeting: &models.MeetingBase{
				Title:       "AI Meeting",
				Description: "AI Description",
				Duration:    60,
				Timezone:    "UTC",
				ZoomConfig: &models.ZoomConfig{
					AICompanionEnabled: true,
				},
			},
			setupMock:     func(client *mocks.MockClient) {},
			expectedError: false,
			expectedID:    "123456789",
		},
		{
			name: "no users available",
			meeting: &models.MeetingBase{
				Title:    "Test Meeting",
				Duration: 60,
				Timezone: "UTC",
			},
			setupMock: func(client *mocks.MockClient) {
				client.GetUsersFunc = func(ctx context.Context) ([]api.ZoomUser, error) {
					return []api.ZoomUser{}, nil
				}
			},
			expectedError: true,
		},
		{
			name: "API error",
			meeting: &models.MeetingBase{
				Title:    "Test Meeting",
				Duration: 60,
				Timezone: "UTC",
			},
			setupMock: func(client *mocks.MockClient) {
				client.CreateMeetingFunc = func(ctx context.Context, userID string, request *api.CreateMeetingRequest) (*api.CreateMeetingResponse, error) {
					return nil, errors.New("API error")
				}
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := mocks.NewMockClient()
			tt.setupMock(client)
			provider := NewZoomProvider(client)
			ctx := context.Background()

			result, err := provider.CreateMeeting(ctx, tt.meeting)

			if tt.expectedError {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if result == nil {
				t.Fatal("expected result but got nil")
			}

			if result.PlatformMeetingID != tt.expectedID {
				t.Errorf("expected meeting ID %s, got %s", tt.expectedID, result.PlatformMeetingID)
			}
		})
	}
}

func TestZoomProvider_UpdateMeeting(t *testing.T) {
	tests := []struct {
		name          string
		meetingID     string
		meeting       *models.MeetingBase
		setupMock     func(*mocks.MockClient)
		expectedError bool
	}{
		{
			name:      "successful update",
			meetingID: "123456789",
			meeting: &models.MeetingBase{
				Title:       "Updated Meeting",
				Description: "Updated Description",
				Duration:    90,
				Timezone:    "UTC",
			},
			setupMock:     func(client *mocks.MockClient) {},
			expectedError: false,
		},
		{
			name:      "API error",
			meetingID: "123456789",
			meeting: &models.MeetingBase{
				Title:    "Test Meeting",
				Duration: 60,
				Timezone: "UTC",
			},
			setupMock: func(client *mocks.MockClient) {
				client.UpdateMeetingFunc = func(ctx context.Context, meetingID string, request *api.UpdateMeetingRequest) error {
					return errors.New("API error")
				}
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := mocks.NewMockClient()
			tt.setupMock(client)
			provider := NewZoomProvider(client)
			ctx := context.Background()

			err := provider.UpdateMeeting(ctx, tt.meetingID, tt.meeting)

			if tt.expectedError {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestZoomProvider_DeleteMeeting(t *testing.T) {
	tests := []struct {
		name          string
		meetingID     string
		setupMock     func(*mocks.MockClient)
		expectedError bool
	}{
		{
			name:          "successful deletion",
			meetingID:     "123456789",
			setupMock:     func(client *mocks.MockClient) {},
			expectedError: false,
		},
		{
			name:      "API error",
			meetingID: "123456789",
			setupMock: func(client *mocks.MockClient) {
				client.DeleteMeetingFunc = func(ctx context.Context, meetingID string) error {
					return errors.New("API error")
				}
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := mocks.NewMockClient()
			tt.setupMock(client)
			provider := NewZoomProvider(client)
			ctx := context.Background()

			err := provider.DeleteMeeting(ctx, tt.meetingID)

			if tt.expectedError {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestZoomProvider_StorePlatformData(t *testing.T) {
	tests := []struct {
		name            string
		meeting         *models.MeetingBase
		result          *domain.CreateMeetingResult
		expectedConfig  *models.ZoomConfig
		expectedJoinURL string
	}{
		{
			name: "store data with nil zoom config",
			meeting: &models.MeetingBase{
				UID: "meeting-123",
			},
			result: &domain.CreateMeetingResult{
				PlatformMeetingID: "987654321",
				JoinURL:           "https://zoom.us/j/987654321",
				Passcode:          "test123",
			},
			expectedConfig: &models.ZoomConfig{
				MeetingID: "987654321",
				Passcode:  "test123",
			},
			expectedJoinURL: "https://zoom.us/j/987654321",
		},
		{
			name: "store data with existing zoom config",
			meeting: &models.MeetingBase{
				UID: "meeting-456",
				ZoomConfig: &models.ZoomConfig{
					AICompanionEnabled: true,
				},
			},
			result: &domain.CreateMeetingResult{
				PlatformMeetingID: "111222333",
				JoinURL:           "https://zoom.us/j/111222333",
				Passcode:          "pass456",
			},
			expectedConfig: &models.ZoomConfig{
				MeetingID:          "111222333",
				Passcode:           "pass456",
				AICompanionEnabled: true,
			},
			expectedJoinURL: "https://zoom.us/j/111222333",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := mocks.NewMockClient()
			provider := NewZoomProvider(client)
			provider.StorePlatformData(tt.meeting, tt.result)

			if tt.meeting.ZoomConfig == nil {
				t.Fatal("expected ZoomConfig to be set")
			}

			if tt.meeting.ZoomConfig.MeetingID != tt.expectedConfig.MeetingID {
				t.Errorf("expected MeetingID %s, got %s", tt.expectedConfig.MeetingID, tt.meeting.ZoomConfig.MeetingID)
			}

			if tt.meeting.ZoomConfig.Passcode != tt.expectedConfig.Passcode {
				t.Errorf("expected Passcode %s, got %s", tt.expectedConfig.Passcode, tt.meeting.ZoomConfig.Passcode)
			}

			if tt.meeting.ZoomConfig.AICompanionEnabled != tt.expectedConfig.AICompanionEnabled {
				t.Errorf("expected AICompanionEnabled %v, got %v", tt.expectedConfig.AICompanionEnabled, tt.meeting.ZoomConfig.AICompanionEnabled)
			}

			if tt.meeting.JoinURL != tt.expectedJoinURL {
				t.Errorf("expected JoinURL %s, got %s", tt.expectedJoinURL, tt.meeting.JoinURL)
			}
		})
	}
}

func TestZoomProvider_GetPlatformMeetingID(t *testing.T) {
	tests := []struct {
		name       string
		meeting    *models.MeetingBase
		expectedID string
	}{
		{
			name: "get ID with zoom config",
			meeting: &models.MeetingBase{
				UID: "meeting-123",
				ZoomConfig: &models.ZoomConfig{
					MeetingID: "987654321",
				},
			},
			expectedID: "987654321",
		},
		{
			name: "get ID with nil zoom config",
			meeting: &models.MeetingBase{
				UID: "meeting-456",
			},
			expectedID: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := mocks.NewMockClient()
			provider := NewZoomProvider(client)
			id := provider.GetPlatformMeetingID(tt.meeting)

			if id != tt.expectedID {
				t.Errorf("expected ID %s, got %s", tt.expectedID, id)
			}
		})
	}
}