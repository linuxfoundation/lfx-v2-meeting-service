// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"testing"
	"time"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/mocks"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// setupServiceForTesting creates a MeetingService with all mock dependencies for testing
func setupServiceForTesting() (*MeetingService, *mocks.MockMeetingRepository, *mocks.MockMessageBuilder) {
	mockRepo := new(mocks.MockMeetingRepository)
	mockRegistrantRepo := new(mocks.MockRegistrantRepository)
	mockBuilder := new(mocks.MockMessageBuilder)
	mockPlatformRegistry := new(mocks.MockPlatformRegistry)
	mockEmailService := new(mocks.MockEmailService)
	occurrenceService := NewOccurrenceService()

	config := ServiceConfig{
		SkipEtagValidation: false,
	}

	service := NewMeetingService(
		mockRepo,
		mockRegistrantRepo,
		mockBuilder,
		mockPlatformRegistry,
		occurrenceService,
		mockEmailService,
		config,
	)

	return service, mockRepo, mockBuilder
}

func TestMeetingsService_GetMeetings(t *testing.T) {
	tests := []struct {
		name        string
		setupMocks  func(*mocks.MockMeetingRepository, *mocks.MockMessageBuilder)
		expectedLen int
		wantErr     bool
		expectedErr error
	}{
		{
			name: "successful get all meetings",
			setupMocks: func(mockRepo *mocks.MockMeetingRepository, mockBuilder *mocks.MockMessageBuilder) {
				now := time.Now()
				startTime := now.Add(time.Hour * 24)
				mockRepo.On("ListAll", mock.Anything).Return(
					[]*models.MeetingBase{
						{
							UID:         "meeting-1",
							Title:       "Test Meeting 1",
							StartTime:   startTime,
							Description: "Description 1",
							CreatedAt:   &now,
							UpdatedAt:   &now,
						},
						{
							UID:         "meeting-2",
							Title:       "Test Meeting 2",
							StartTime:   startTime,
							Description: "Description 2",
							CreatedAt:   &now,
							UpdatedAt:   &now,
						},
					},
					[]*models.MeetingSettings{
						{
							UID:        "meeting-1",
							Organizers: []string{"org1"},
							CreatedAt:  &now,
							UpdatedAt:  &now,
						},
						{
							UID:        "meeting-2",
							Organizers: []string{"org2"},
							CreatedAt:  &now,
							UpdatedAt:  &now,
						},
					},
					nil,
				)
			},
			expectedLen: 2,
			wantErr:     false,
		},
		{
			name: "service not ready",
			setupMocks: func(mockRepo *mocks.MockMeetingRepository, mockBuilder *mocks.MockMessageBuilder) {
				// Don't set up repository - will make service not ready
			},
			expectedLen: 0,
			wantErr:     true,
			expectedErr: domain.NewUnavailableError("test"),
		},
		{
			name: "repository error",
			setupMocks: func(mockRepo *mocks.MockMeetingRepository, mockBuilder *mocks.MockMessageBuilder) {
				mockRepo.On("ListAll", mock.Anything).Return(
					nil, nil, domain.NewInternalError("database error"),
				)
			},
			expectedLen: 0,
			wantErr:     true,
			expectedErr: domain.NewInternalError("database error"),
		},
		{
			name: "empty meetings list",
			setupMocks: func(mockRepo *mocks.MockMeetingRepository, mockBuilder *mocks.MockMessageBuilder) {
				mockRepo.On("ListAll", mock.Anything).Return(
					[]*models.MeetingBase{},
					[]*models.MeetingSettings{},
					nil,
				)
			},
			expectedLen: 0,
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, mockRepo, mockBuilder := setupServiceForTesting()

			if tt.name == "service not ready" {
				service.meetingRepository = nil
			}

			tt.setupMocks(mockRepo, mockBuilder)

			result, err := service.ListMeetings(context.Background(), false)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.expectedErr != nil {
					expectedType := domain.GetErrorType(tt.expectedErr)
					actualType := domain.GetErrorType(err)
					assert.Equal(t, expectedType, actualType, "Error types should match: expected %v, got %v", expectedType, actualType)
				}
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.Len(t, result, tt.expectedLen)
			}

			mockRepo.AssertExpectations(t)
			mockBuilder.AssertExpectations(t)
		})
	}
}

func TestMeetingsService_CreateMeeting(t *testing.T) {
	tests := []struct {
		name        string
		payload     *models.MeetingFull
		setupMocks  func(*MeetingService, *mocks.MockMeetingRepository, *mocks.MockMessageBuilder)
		wantErr     bool
		expectedErr error
		validate    func(*testing.T, *models.MeetingFull)
	}{
		{
			name: "successful meeting creation",
			payload: &models.MeetingFull{
				Base: &models.MeetingBase{
					Title:       "Test Meeting",
					StartTime:   time.Now().Add(time.Hour * 24),
					ProjectUID:  "project-123",
					Description: "Test Description",
					Platform:    models.PlatformZoom,
				},
				Settings: &models.MeetingSettings{
					Organizers: []string{"org1"},
				},
			},
			setupMocks: func(service *MeetingService, mockRepo *mocks.MockMeetingRepository, mockBuilder *mocks.MockMessageBuilder) {
				// Set up platform registry mock
				mockPlatformRegistry := service.platformRegistry.(*mocks.MockPlatformRegistry)
				mockProvider := &mocks.MockPlatformProvider{}
				mockProvider.On("CreateMeeting", mock.Anything, mock.Anything).Return(&domain.CreateMeetingResult{
					PlatformMeetingID: "zoom-meeting-123",
					JoinURL:           "https://zoom.us/j/123456789",
					Passcode:          "test-pass",
				}, nil)
				mockProvider.On("StorePlatformData", mock.Anything, mock.Anything)
				mockPlatformRegistry.On("GetProvider", models.PlatformZoom).Return(mockProvider, nil)

				// Set up repository and message builder mocks
				mockRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.MeetingBase"), mock.AnythingOfType("*models.MeetingSettings")).Return(nil)
				mockBuilder.On("GetProjectName", mock.Anything, "project-123").Return("Test Project", nil)
				mockBuilder.On("SendIndexMeeting", mock.Anything, models.ActionCreated, mock.Anything).Return(nil)
				mockBuilder.On("SendIndexMeetingSettings", mock.Anything, models.ActionCreated, mock.Anything).Return(nil)
				mockBuilder.On("SendUpdateAccessMeeting", mock.Anything, mock.Anything).Return(nil)
				mockBuilder.On("SendMeetingCreated", mock.Anything, mock.Anything).Return(nil)
			},
			wantErr: false,
			validate: func(t *testing.T, result *models.MeetingFull) {
				assert.NotNil(t, result)
				assert.NotNil(t, result.Base.UID)
				assert.Equal(t, "Test Meeting", result.Base.Title)
			},
		},
		{
			name: "service not ready",
			payload: &models.MeetingFull{
				Base: &models.MeetingBase{
					Title:       "Test Meeting",
					StartTime:   time.Now().Add(time.Hour * 24),
					ProjectUID:  "project-123",
					Description: "Test Description",
					Platform:    models.PlatformZoom,
				},
				Settings: &models.MeetingSettings{
					Organizers: []string{"org1"},
				},
			},
			setupMocks: func(service *MeetingService, mockRepo *mocks.MockMeetingRepository, mockBuilder *mocks.MockMessageBuilder) {
				// Service will not be ready - no mocks needed
			},
			wantErr:     true,
			expectedErr: domain.NewUnavailableError("test"),
		},
		{
			name: "repository creation error",
			payload: &models.MeetingFull{
				Base: &models.MeetingBase{
					Title:       "Test Meeting",
					StartTime:   time.Now().Add(time.Hour * 24),
					ProjectUID:  "project-123",
					Description: "Test Description",
					Platform:    models.PlatformZoom,
				},
				Settings: &models.MeetingSettings{
					Organizers: []string{"org1"},
				},
			},
			setupMocks: func(service *MeetingService, mockRepo *mocks.MockMeetingRepository, mockBuilder *mocks.MockMessageBuilder) {
				// Set up platform registry mock for cleanup on error
				mockPlatformRegistry := service.platformRegistry.(*mocks.MockPlatformRegistry)
				mockProvider := &mocks.MockPlatformProvider{}
				mockProvider.On("CreateMeeting", mock.Anything, mock.Anything).Return(&domain.CreateMeetingResult{
					PlatformMeetingID: "zoom-meeting-123",
					JoinURL:           "https://zoom.us/j/123456789",
					Passcode:          "test-pass",
				}, nil)
				mockProvider.On("StorePlatformData", mock.Anything, mock.Anything)
				mockProvider.On("GetPlatformMeetingID", mock.Anything).Return("zoom-meeting-123")
				mockProvider.On("DeleteMeeting", mock.Anything, mock.Anything).Return(nil)
				mockPlatformRegistry.On("GetProvider", models.PlatformZoom).Return(mockProvider, nil)

				// Repository returns error to test cleanup
				mockBuilder.On("GetProjectName", mock.Anything, "project-123").Return("Test Project", nil)
				mockRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.MeetingBase"), mock.AnythingOfType("*models.MeetingSettings")).Return(domain.NewInternalError("database error"))
			},
			wantErr:     true,
			expectedErr: domain.NewInternalError("database error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, mockRepo, mockBuilder := setupServiceForTesting()

			if tt.name == "service not ready" {
				service.meetingRepository = nil
			}

			tt.setupMocks(service, mockRepo, mockBuilder)

			result, err := service.CreateMeeting(context.Background(), tt.payload)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.expectedErr != nil {
					expectedType := domain.GetErrorType(tt.expectedErr)
					actualType := domain.GetErrorType(err)
					assert.Equal(t, expectedType, actualType, "Error types should match: expected %v, got %v", expectedType, actualType)
				}
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				require.NotNil(t, result)
				if tt.validate != nil {
					tt.validate(t, result)
				}
			}

			mockRepo.AssertExpectations(t)
			mockBuilder.AssertExpectations(t)
			if tt.name != "service not ready" {
				mockPlatformRegistry := service.platformRegistry.(*mocks.MockPlatformRegistry)
				mockPlatformRegistry.AssertExpectations(t)
			}
		})
	}
}

func TestMeetingsService_GetMeetingBase(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name        string
		uid         string
		setupMocks  func(*mocks.MockMeetingRepository, *mocks.MockMessageBuilder)
		wantErr     bool
		expectedErr error
		validate    func(*testing.T, *models.MeetingBase, string)
	}{
		{
			name: "successful get meeting",
			uid:  "test-meeting-uid",
			setupMocks: func(mockRepo *mocks.MockMeetingRepository, mockBuilder *mocks.MockMessageBuilder) {
				mockRepo.On("GetBaseWithRevision", mock.Anything, "test-meeting-uid").Return(
					&models.MeetingBase{
						UID:         "test-meeting-uid",
						Title:       "Test Meeting",
						StartTime:   now.Add(time.Hour * 24),
						Description: "Test Description",
						CreatedAt:   &now,
						UpdatedAt:   &now,
					},
					uint64(123),
					nil,
				)
			},
			wantErr: false,
			validate: func(t *testing.T, result *models.MeetingBase, etag string) {
				assert.NotNil(t, result)
				assert.Equal(t, "test-meeting-uid", result.UID)
				assert.Equal(t, "Test Meeting", result.Title)
				assert.Equal(t, "123", etag)
			},
		},
		{
			name: "meeting not found",
			uid:  "non-existent-uid",
			setupMocks: func(mockRepo *mocks.MockMeetingRepository, mockBuilder *mocks.MockMessageBuilder) {
				mockRepo.On("GetBaseWithRevision", mock.Anything, "non-existent-uid").Return(
					nil, uint64(0), domain.NewNotFoundError("meeting not found"),
				)
			},
			wantErr:     true,
			expectedErr: domain.NewNotFoundError("meeting not found"),
		},
		{
			name: "empty UID",
			uid:  "",
			setupMocks: func(mockRepo *mocks.MockMeetingRepository, mockBuilder *mocks.MockMessageBuilder) {
				// Repository is called even with empty UID, but returns error
				mockRepo.On("GetBaseWithRevision", mock.Anything, "").Return(
					nil, uint64(0), domain.NewValidationError("meeting UID cannot be empty"),
				)
			},
			wantErr:     true,
			expectedErr: domain.NewValidationError("meeting UID cannot be empty"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, mockRepo, mockBuilder := setupServiceForTesting()
			tt.setupMocks(mockRepo, mockBuilder)

			result, etag, err := service.GetMeetingBase(context.Background(), tt.uid, GetMeetingBaseOptions{})

			if tt.wantErr {
				assert.Error(t, err)
				if tt.expectedErr != nil {
					expectedType := domain.GetErrorType(tt.expectedErr)
					actualType := domain.GetErrorType(err)
					assert.Equal(t, expectedType, actualType, "Error types should match: expected %v, got %v", expectedType, actualType)
				}
				assert.Nil(t, result)
				assert.Empty(t, etag)
			} else {
				assert.NoError(t, err)
				require.NotNil(t, result)
				if tt.validate != nil {
					tt.validate(t, result, etag)
				}
			}

			mockRepo.AssertExpectations(t)
			mockBuilder.AssertExpectations(t)
		})
	}
}

func TestMeetingsService_GetMeetingSettings(t *testing.T) {
	tests := []struct {
		name        string
		uid         string
		setupMocks  func(*mocks.MockMeetingRepository, *mocks.MockMessageBuilder)
		wantErr     bool
		expectedErr error
		validate    func(*testing.T, *models.MeetingSettings, string)
	}{
		{
			name: "successful get meeting settings",
			uid:  "meeting-123",
			setupMocks: func(mockRepo *mocks.MockMeetingRepository, mockBuilder *mocks.MockMessageBuilder) {
				now := time.Now()
				mockRepo.On("GetSettingsWithRevision", mock.Anything, "meeting-123").Return(&models.MeetingSettings{
					UID:        "meeting-123",
					Organizers: []string{"org1", "org2"},
					CreatedAt:  &now,
					UpdatedAt:  &now,
				}, uint64(1), nil)
			},
			wantErr: false,
			validate: func(t *testing.T, result *models.MeetingSettings, etag string) {
				assert.NotNil(t, result)
				assert.Equal(t, "meeting-123", result.UID)
				assert.Len(t, result.Organizers, 2)
				assert.Equal(t, "1", etag)
			},
		},
		{
			name: "meeting settings not found",
			uid:  "nonexistent-meeting",
			setupMocks: func(mockRepo *mocks.MockMeetingRepository, mockBuilder *mocks.MockMessageBuilder) {
				mockRepo.On("GetSettingsWithRevision", mock.Anything, "nonexistent-meeting").Return(nil, uint64(0), domain.NewNotFoundError("meeting not found"))
			},
			wantErr:     true,
			expectedErr: domain.NewNotFoundError("meeting not found"),
		},
		{
			name: "empty UID",
			uid:  "",
			setupMocks: func(mockRepo *mocks.MockMeetingRepository, mockBuilder *mocks.MockMessageBuilder) {
				// Repository is called with empty UID but should return validation error
				mockRepo.On("GetSettingsWithRevision", mock.Anything, "").Return(nil, uint64(0), domain.NewValidationError("meeting UID cannot be empty"))
			},
			wantErr:     true,
			expectedErr: domain.NewValidationError("meeting UID cannot be empty"),
		},
		{
			name:        "service not ready",
			uid:         "meeting-123",
			setupMocks:  func(*mocks.MockMeetingRepository, *mocks.MockMessageBuilder) {},
			wantErr:     true,
			expectedErr: domain.NewUnavailableError("test"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, mockRepo, mockBuilder := setupServiceForTesting()

			if tt.name == "service not ready" {
				service.meetingRepository = nil
			}

			tt.setupMocks(mockRepo, mockBuilder)

			result, etag, err := service.GetMeetingSettings(context.Background(), tt.uid)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.expectedErr != nil {
					expectedType := domain.GetErrorType(tt.expectedErr)
					actualType := domain.GetErrorType(err)
					assert.Equal(t, expectedType, actualType, "Error types should match: expected %v, got %v", expectedType, actualType)
				}
				assert.Nil(t, result)
				assert.Empty(t, etag)
			} else {
				assert.NoError(t, err)
				require.NotNil(t, result)
				if tt.validate != nil {
					tt.validate(t, result, etag)
				}
			}

			mockRepo.AssertExpectations(t)
			mockBuilder.AssertExpectations(t)
		})
	}
}

func TestMeetingsService_UpdateMeetingSettings(t *testing.T) {
	tests := []struct {
		name        string
		settings    *models.MeetingSettings
		revision    uint64
		setupMocks  func(*mocks.MockMeetingRepository, *mocks.MockMessageBuilder)
		wantErr     bool
		expectedErr error
		validate    func(*testing.T, *models.MeetingSettings)
	}{
		{
			name: "successful update meeting settings",
			settings: &models.MeetingSettings{
				UID:        "meeting-123",
				Organizers: []string{"org3", "org4"},
			},
			revision: 1,
			setupMocks: func(mockRepo *mocks.MockMeetingRepository, mockBuilder *mocks.MockMessageBuilder) {
				now := time.Now()
				existingSettings := &models.MeetingSettings{
					UID:        "meeting-123",
					Organizers: []string{"org1", "org2"},
					CreatedAt:  &now,
					UpdatedAt:  &now,
				}

				mockRepo.On("GetSettings", mock.Anything, "meeting-123").Return(existingSettings, nil)
				mockRepo.On("UpdateSettings", mock.Anything, mock.MatchedBy(func(settings *models.MeetingSettings) bool {
					return settings.UID == "meeting-123" &&
						len(settings.Organizers) == 2 &&
						settings.Organizers[0] == "org3" &&
						settings.Organizers[1] == "org4" &&
						settings.CreatedAt.Equal(now)
				}), uint64(1)).Return(nil)

				mockBuilder.On("SendIndexMeetingSettings", mock.Anything, models.ActionUpdated, mock.Anything).Return(nil)

				mockRepo.On("GetBase", mock.Anything, "meeting-123").Return(&models.MeetingBase{
					UID:        "meeting-123",
					Title:      "Test Meeting",
					ProjectUID: "project-123",
					Visibility: models.VisibilityPublic,
					Committees: []models.Committee{{UID: "committee-123"}},
				}, nil)

				mockBuilder.On("SendUpdateAccessMeeting", mock.Anything, mock.MatchedBy(func(msg models.MeetingAccessMessage) bool {
					return msg.UID == "meeting-123" &&
						msg.ProjectUID == "project-123" &&
						msg.Public == true &&
						len(msg.Organizers) == 2 &&
						msg.Organizers[0] == "org3" &&
						msg.Organizers[1] == "org4"
				})).Return(nil)
			},
			wantErr: false,
			validate: func(t *testing.T, result *models.MeetingSettings) {
				assert.NotNil(t, result)
				assert.Equal(t, "meeting-123", result.UID)
				assert.Len(t, result.Organizers, 2)
				assert.Equal(t, "org3", result.Organizers[0])
				assert.Equal(t, "org4", result.Organizers[1])
			},
		},
		{
			name: "meeting settings not found",
			settings: &models.MeetingSettings{
				UID:        "nonexistent-meeting",
				Organizers: []string{"org1"},
			},
			revision: 1,
			setupMocks: func(mockRepo *mocks.MockMeetingRepository, mockBuilder *mocks.MockMessageBuilder) {
				mockRepo.On("GetSettings", mock.Anything, "nonexistent-meeting").Return(nil, domain.NewNotFoundError("meeting not found"))
			},
			wantErr:     true,
			expectedErr: domain.NewNotFoundError("meeting not found"),
		},
		{
			name:        "nil settings",
			settings:    nil,
			revision:    1,
			setupMocks:  func(*mocks.MockMeetingRepository, *mocks.MockMessageBuilder) {},
			wantErr:     true,
			expectedErr: domain.NewValidationError("test"),
		},
		{
			name: "service not ready",
			settings: &models.MeetingSettings{
				UID:        "meeting-123",
				Organizers: []string{"org1"},
			},
			revision:    1,
			setupMocks:  func(*mocks.MockMeetingRepository, *mocks.MockMessageBuilder) {},
			wantErr:     true,
			expectedErr: domain.NewUnavailableError("test"),
		},
		{
			name: "empty UID",
			settings: &models.MeetingSettings{
				UID:        "",
				Organizers: []string{"org1"},
			},
			revision:    1,
			setupMocks:  func(*mocks.MockMeetingRepository, *mocks.MockMessageBuilder) {},
			wantErr:     true,
			expectedErr: domain.NewValidationError("test"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, mockRepo, mockBuilder := setupServiceForTesting()

			if tt.name == "service not ready" {
				service.meetingRepository = nil
			}

			if tt.name == "skip etag validation mode" {
				service.config.SkipEtagValidation = true
			}

			tt.setupMocks(mockRepo, mockBuilder)

			result, err := service.UpdateMeetingSettings(context.Background(), tt.settings, tt.revision)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.expectedErr != nil {
					expectedType := domain.GetErrorType(tt.expectedErr)
					actualType := domain.GetErrorType(err)
					assert.Equal(t, expectedType, actualType, "Error types should match: expected %v, got %v", expectedType, actualType)
				}
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				require.NotNil(t, result)
				if tt.validate != nil {
					tt.validate(t, result)
				}
			}

			mockRepo.AssertExpectations(t)
			mockBuilder.AssertExpectations(t)
		})
	}
}
