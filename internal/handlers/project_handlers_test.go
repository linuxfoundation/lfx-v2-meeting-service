// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package handlers

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/mocks"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/infrastructure/platform"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/service"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/constants"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// setupProjectHandlerForTesting creates a ProjectHandlers with all mock dependencies for testing
func setupProjectHandlerForTesting() (*ProjectHandlers, *mocks.MockMeetingRepository, *mocks.MockMessageBuilder) {
	mockMeetingRepo := new(mocks.MockMeetingRepository)
	mockRSVPRepo := new(mocks.MockMeetingRSVPRepository)
	mockRegistrantRepo := new(mocks.MockRegistrantRepository)
	mockAttachmentRepo := new(mocks.MockMeetingAttachmentRepository)
	mockMessageBuilder := new(mocks.MockMessageBuilder)
	mockAttachmentService := service.NewMeetingAttachmentService(mockAttachmentRepo, mockMeetingRepo, mockMessageBuilder, mockMessageBuilder)
	mockEmailService := new(mocks.MockEmailService)
	mockPlatformRegistry := platform.NewRegistry()

	config := service.ServiceConfig{
		SkipEtagValidation: false,
		LfxURLGenerator:    constants.NewLfxURLGenerator("dev", ""),
	}

	occurrenceService := service.NewOccurrenceService()

	// Set up default expectations for RSVP repository
	mockRSVPRepo.On("ListByMeeting", mock.Anything, mock.Anything).Return([]*models.RSVPResponse{}, nil).Maybe()

	// Set up default expectations for attachment repository
	mockAttachmentRepo.On("ListByMeeting", mock.Anything, mock.Anything).Return([]*models.MeetingAttachment{}, nil).Maybe()

	meetingService := service.NewMeetingService(
		mockMeetingRepo,
		mockRegistrantRepo,
		mockRSVPRepo,
		mockMessageBuilder,
		mockMessageBuilder,
		mockPlatformRegistry,
		occurrenceService,
		mockEmailService,
		mockAttachmentService,
		config,
	)

	handler := NewProjectHandlers(meetingService)

	return handler, mockMeetingRepo, mockMessageBuilder
}

func TestProjectHandlers_HandleProjectSettingsUpdated(t *testing.T) {
	ctx := context.Background()
	now := time.Now()

	tests := []struct {
		name        string
		payload     models.ProjectSettingsUpdatedPayload
		setupMocks  func(*mocks.MockMeetingRepository, *mocks.MockMessageBuilder)
		expectError bool
	}{
		{
			name: "user removed from writers - removes from meeting organizers",
			payload: models.ProjectSettingsUpdatedPayload{
				ProjectUID: "project-123",
				OldSettings: &models.ProjectSettings{
					UID: "settings-123",
					Writers: []models.ProjectUserInfo{
						{UID: "user-1", Username: "writer1", Email: "writer1@example.com"},
						{UID: "user-2", Username: "writer2", Email: "writer2@example.com"},
					},
					MeetingCoordinators: []models.ProjectUserInfo{},
				},
				NewSettings: &models.ProjectSettings{
					UID: "settings-123",
					Writers: []models.ProjectUserInfo{
						{UID: "user-1", Username: "writer1", Email: "writer1@example.com"},
					},
					MeetingCoordinators: []models.ProjectUserInfo{},
				},
			},
			setupMocks: func(mockMeetingRepo *mocks.MockMeetingRepository, mockBuilder *mocks.MockMessageBuilder) {
				// Return meetings for the project
				meetings := []*models.MeetingBase{
					{UID: "meeting-1", ProjectUID: "project-123", Title: "Meeting 1", StartTime: now},
				}
				mockMeetingRepo.On("ListByProject", mock.Anything, "project-123").Return(meetings, nil)

				// GetMeetingSettings (called by GetMeetingSettings service method)
				mockMeetingRepo.On("GetSettingsWithRevision", mock.Anything, "meeting-1").Return(
					&models.MeetingSettings{UID: "meeting-1", Organizers: []string{"writer1", "writer2", "other-user"}},
					uint64(1),
					nil,
				)

				// UpdateSettings
				mockMeetingRepo.On("UpdateSettings", mock.Anything, mock.MatchedBy(func(s *models.MeetingSettings) bool {
					return s.UID == "meeting-1" && len(s.Organizers) == 2 && !contains(s.Organizers, "writer2")
				}), uint64(1)).Return(nil)

				// GetSettings for existing settings in UpdateMeetingSettings
				mockMeetingRepo.On("GetSettings", mock.Anything, "meeting-1").Return(
					&models.MeetingSettings{UID: "meeting-1", Organizers: []string{"writer1", "writer2", "other-user"}},
					nil,
				)

				// GetBase for access message
				mockMeetingRepo.On("GetBase", mock.Anything, "meeting-1").Return(
					&models.MeetingBase{UID: "meeting-1", ProjectUID: "project-123"},
					nil,
				)

				// Send index and access messages
				mockBuilder.On("SendIndexMeetingSettings", mock.Anything, models.ActionUpdated, mock.Anything, false).Return(nil)
				mockBuilder.On("SendUpdateAccessMeeting", mock.Anything, mock.Anything, false).Return(nil)
			},
			expectError: false,
		},
		{
			name: "user removed from meeting coordinators - removes from meeting organizers",
			payload: models.ProjectSettingsUpdatedPayload{
				ProjectUID: "project-456",
				OldSettings: &models.ProjectSettings{
					UID:     "settings-456",
					Writers: []models.ProjectUserInfo{},
					MeetingCoordinators: []models.ProjectUserInfo{
						{UID: "user-1", Username: "coordinator1", Email: "coord1@example.com"},
					},
				},
				NewSettings: &models.ProjectSettings{
					UID:                 "settings-456",
					Writers:             []models.ProjectUserInfo{},
					MeetingCoordinators: []models.ProjectUserInfo{},
				},
			},
			setupMocks: func(mockMeetingRepo *mocks.MockMeetingRepository, mockBuilder *mocks.MockMessageBuilder) {
				meetings := []*models.MeetingBase{
					{UID: "meeting-2", ProjectUID: "project-456", Title: "Meeting 2", StartTime: now},
				}
				mockMeetingRepo.On("ListByProject", mock.Anything, "project-456").Return(meetings, nil)

				mockMeetingRepo.On("GetSettingsWithRevision", mock.Anything, "meeting-2").Return(
					&models.MeetingSettings{UID: "meeting-2", Organizers: []string{"coordinator1", "other-user"}},
					uint64(2),
					nil,
				)

				mockMeetingRepo.On("UpdateSettings", mock.Anything, mock.MatchedBy(func(s *models.MeetingSettings) bool {
					return s.UID == "meeting-2" && len(s.Organizers) == 1 && !contains(s.Organizers, "coordinator1")
				}), uint64(2)).Return(nil)

				mockMeetingRepo.On("GetSettings", mock.Anything, "meeting-2").Return(
					&models.MeetingSettings{UID: "meeting-2", Organizers: []string{"coordinator1", "other-user"}},
					nil,
				)

				mockMeetingRepo.On("GetBase", mock.Anything, "meeting-2").Return(
					&models.MeetingBase{UID: "meeting-2", ProjectUID: "project-456"},
					nil,
				)

				mockBuilder.On("SendIndexMeetingSettings", mock.Anything, models.ActionUpdated, mock.Anything, false).Return(nil)
				mockBuilder.On("SendUpdateAccessMeeting", mock.Anything, mock.Anything, false).Return(nil)
			},
			expectError: false,
		},
		{
			name: "no users removed - no updates",
			payload: models.ProjectSettingsUpdatedPayload{
				ProjectUID: "project-789",
				OldSettings: &models.ProjectSettings{
					UID: "settings-789",
					Writers: []models.ProjectUserInfo{
						{UID: "user-1", Username: "writer1", Email: "writer1@example.com"},
					},
					MeetingCoordinators: []models.ProjectUserInfo{},
				},
				NewSettings: &models.ProjectSettings{
					UID: "settings-789",
					Writers: []models.ProjectUserInfo{
						{UID: "user-1", Username: "writer1", Email: "writer1@example.com"},
						{UID: "user-2", Username: "writer2", Email: "writer2@example.com"}, // Added, not removed
					},
					MeetingCoordinators: []models.ProjectUserInfo{},
				},
			},
			setupMocks: func(mockMeetingRepo *mocks.MockMeetingRepository, mockBuilder *mocks.MockMessageBuilder) {
				// No meetings should be fetched since no users were removed
			},
			expectError: false,
		},
		{
			name: "project with no meetings - success",
			payload: models.ProjectSettingsUpdatedPayload{
				ProjectUID: "project-empty",
				OldSettings: &models.ProjectSettings{
					UID: "settings-empty",
					Writers: []models.ProjectUserInfo{
						{UID: "user-1", Username: "writer1", Email: "writer1@example.com"},
					},
					MeetingCoordinators: []models.ProjectUserInfo{},
				},
				NewSettings: &models.ProjectSettings{
					UID:                 "settings-empty",
					Writers:             []models.ProjectUserInfo{},
					MeetingCoordinators: []models.ProjectUserInfo{},
				},
			},
			setupMocks: func(mockMeetingRepo *mocks.MockMeetingRepository, mockBuilder *mocks.MockMessageBuilder) {
				mockMeetingRepo.On("ListByProject", mock.Anything, "project-empty").Return(
					[]*models.MeetingBase{},
					nil,
				)
			},
			expectError: false,
		},
		{
			name: "removed user not in organizers - no update needed",
			payload: models.ProjectSettingsUpdatedPayload{
				ProjectUID: "project-no-match",
				OldSettings: &models.ProjectSettings{
					UID: "settings-no-match",
					Writers: []models.ProjectUserInfo{
						{UID: "user-1", Username: "writer1", Email: "writer1@example.com"},
					},
					MeetingCoordinators: []models.ProjectUserInfo{},
				},
				NewSettings: &models.ProjectSettings{
					UID:                 "settings-no-match",
					Writers:             []models.ProjectUserInfo{},
					MeetingCoordinators: []models.ProjectUserInfo{},
				},
			},
			setupMocks: func(mockMeetingRepo *mocks.MockMeetingRepository, mockBuilder *mocks.MockMessageBuilder) {
				meetings := []*models.MeetingBase{
					{UID: "meeting-3", ProjectUID: "project-no-match", Title: "Meeting 3", StartTime: now},
				}
				mockMeetingRepo.On("ListByProject", mock.Anything, "project-no-match").Return(meetings, nil)

				// GetMeetingSettings called to check organizers - writer1 not in organizers
				mockMeetingRepo.On("GetSettingsWithRevision", mock.Anything, "meeting-3").Return(
					&models.MeetingSettings{UID: "meeting-3", Organizers: []string{"other-user1", "other-user2"}},
					uint64(1),
					nil,
				)
				// No update calls expected since writer1 is not in organizers
			},
			expectError: false,
		},
		{
			name: "missing project UID - error",
			payload: models.ProjectSettingsUpdatedPayload{
				ProjectUID:  "",
				OldSettings: &models.ProjectSettings{},
				NewSettings: &models.ProjectSettings{},
			},
			setupMocks:  func(mockMeetingRepo *mocks.MockMeetingRepository, mockBuilder *mocks.MockMessageBuilder) {},
			expectError: true,
		},
		{
			name: "missing old settings - error",
			payload: models.ProjectSettingsUpdatedPayload{
				ProjectUID:  "project-123",
				OldSettings: nil,
				NewSettings: &models.ProjectSettings{},
			},
			setupMocks:  func(mockMeetingRepo *mocks.MockMeetingRepository, mockBuilder *mocks.MockMessageBuilder) {},
			expectError: true,
		},
		{
			name: "missing new settings - error",
			payload: models.ProjectSettingsUpdatedPayload{
				ProjectUID:  "project-123",
				OldSettings: &models.ProjectSettings{},
				NewSettings: nil,
			},
			setupMocks:  func(mockMeetingRepo *mocks.MockMeetingRepository, mockBuilder *mocks.MockMessageBuilder) {},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, mockMeetingRepo, mockBuilder := setupProjectHandlerForTesting()

			tt.setupMocks(mockMeetingRepo, mockBuilder)

			// Create message payload
			payloadBytes, err := json.Marshal(tt.payload)
			assert.NoError(t, err)

			mockMsg := mocks.NewMockMessageWithReply(payloadBytes, models.ProjectSettingsUpdatedSubject, false)

			// Call HandleMessage
			handler.HandleMessage(ctx, mockMsg)

			// Verify mock expectations
			mockMeetingRepo.AssertExpectations(t)
			mockBuilder.AssertExpectations(t)
		})
	}
}

func TestProjectHandlers_FindRemovedUsernamesByRoles(t *testing.T) {
	handler := &ProjectHandlers{}

	tests := []struct {
		name        string
		oldSettings *models.ProjectSettings
		newSettings *models.ProjectSettings
		roles       []ProjectRole
		expected    []string
	}{
		{
			name: "user removed from writers",
			oldSettings: &models.ProjectSettings{
				Writers: []models.ProjectUserInfo{
					{Username: "user1"},
					{Username: "user2"},
				},
			},
			newSettings: &models.ProjectSettings{
				Writers: []models.ProjectUserInfo{
					{Username: "user1"},
				},
			},
			roles:    []ProjectRole{ProjectRoleWriter},
			expected: []string{"user2"},
		},
		{
			name: "user removed from meeting coordinators",
			oldSettings: &models.ProjectSettings{
				MeetingCoordinators: []models.ProjectUserInfo{
					{Username: "coord1"},
					{Username: "coord2"},
				},
			},
			newSettings: &models.ProjectSettings{
				MeetingCoordinators: []models.ProjectUserInfo{
					{Username: "coord1"},
				},
			},
			roles:    []ProjectRole{ProjectRoleMeetingCoordinator},
			expected: []string{"coord2"},
		},
		{
			name: "user removed from both roles - deduplicates",
			oldSettings: &models.ProjectSettings{
				Writers: []models.ProjectUserInfo{
					{Username: "user1"},
				},
				MeetingCoordinators: []models.ProjectUserInfo{
					{Username: "user1"},
				},
			},
			newSettings: &models.ProjectSettings{
				Writers:             []models.ProjectUserInfo{},
				MeetingCoordinators: []models.ProjectUserInfo{},
			},
			roles:    []ProjectRole{ProjectRoleWriter, ProjectRoleMeetingCoordinator},
			expected: []string{"user1"},
		},
		{
			name: "no users removed",
			oldSettings: &models.ProjectSettings{
				Writers: []models.ProjectUserInfo{
					{Username: "user1"},
				},
			},
			newSettings: &models.ProjectSettings{
				Writers: []models.ProjectUserInfo{
					{Username: "user1"},
					{Username: "user2"}, // Added
				},
			},
			roles:    []ProjectRole{ProjectRoleWriter},
			expected: []string{},
		},
		{
			name: "check auditor role",
			oldSettings: &models.ProjectSettings{
				Auditors: []models.ProjectUserInfo{
					{Username: "auditor1"},
					{Username: "auditor2"},
				},
			},
			newSettings: &models.ProjectSettings{
				Auditors: []models.ProjectUserInfo{
					{Username: "auditor1"},
				},
			},
			roles:    []ProjectRole{ProjectRoleAuditor},
			expected: []string{"auditor2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.findRemovedUsernamesByRoles(tt.oldSettings, tt.newSettings, tt.roles)

			// Check length matches
			assert.Equal(t, len(tt.expected), len(result), "expected %d removed users, got %d", len(tt.expected), len(result))

			// Check all expected users are in result
			for _, expected := range tt.expected {
				assert.True(t, contains(result, expected), "expected %s to be in result", expected)
			}
		})
	}
}

// Helper function to check if slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
