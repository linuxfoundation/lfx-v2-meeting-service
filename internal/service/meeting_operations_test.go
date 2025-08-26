// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"testing"
	"time"

	meetingsvc "github.com/linuxfoundation/lfx-v2-meeting-service/gen/meeting_service"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/mocks"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

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
			expectedErr: domain.ErrServiceUnavailable,
		},
		{
			name: "repository error",
			setupMocks: func(mockRepo *mocks.MockMeetingRepository, mockBuilder *mocks.MockMessageBuilder) {
				mockRepo.On("ListAll", mock.Anything).Return(
					nil, nil, domain.ErrInternal,
				)
			},
			expectedLen: 0,
			wantErr:     true,
			expectedErr: domain.ErrInternal,
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
			service, mockRepo, mockBuilder, mockAuth := setupServiceForTesting()

			if tt.name == "service not ready" {
				service.MeetingRepository = nil
			}

			tt.setupMocks(mockRepo, mockBuilder)

			result, err := service.GetMeetings(context.Background())

			if tt.wantErr {
				assert.Error(t, err)
				if tt.expectedErr != nil {
					assert.Equal(t, tt.expectedErr, err)
				}
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.Len(t, result, tt.expectedLen)
			}

			mockRepo.AssertExpectations(t)
			mockBuilder.AssertExpectations(t)
			mockAuth.AssertExpectations(t)
		})
	}
}

func TestMeetingsService_CreateMeeting(t *testing.T) {
	tests := []struct {
		name        string
		payload     *meetingsvc.CreateMeetingPayload
		setupMocks  func(*mocks.MockMeetingRepository, *mocks.MockMessageBuilder)
		wantErr     bool
		expectedErr error
		validate    func(*testing.T, *meetingsvc.MeetingFull)
	}{
		{
			name: "successful meeting creation",
			payload: &meetingsvc.CreateMeetingPayload{
				Title:     "Test Meeting",
				StartTime: time.Now().Add(time.Hour * 24).Format(time.RFC3339),
			},
			setupMocks: func(mockRepo *mocks.MockMeetingRepository, mockBuilder *mocks.MockMessageBuilder) {
				mockRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.MeetingBase"), mock.AnythingOfType("*models.MeetingSettings")).Return(nil)
				mockBuilder.On("SendIndexMeeting", mock.Anything, models.ActionCreated, mock.Anything).Return(nil)
				mockBuilder.On("SendIndexMeetingSettings", mock.Anything, models.ActionCreated, mock.Anything).Return(nil)
				mockBuilder.On("SendUpdateAccessMeeting", mock.Anything, mock.Anything).Return(nil)
			},
			wantErr: false,
			validate: func(t *testing.T, result *meetingsvc.MeetingFull) {
				assert.NotNil(t, result)
				assert.NotNil(t, result.UID)
				assert.Equal(t, "Test Meeting", *result.Title)
			},
		},
		{
			name: "service not ready",
			payload: &meetingsvc.CreateMeetingPayload{
				Title:     "Test Meeting",
				StartTime: time.Now().Add(time.Hour * 24).Format(time.RFC3339),
			},
			setupMocks: func(mockRepo *mocks.MockMeetingRepository, mockBuilder *mocks.MockMessageBuilder) {
				// Service will not be ready
			},
			wantErr:     true,
			expectedErr: domain.ErrServiceUnavailable,
		},
		{
			name: "repository creation error",
			payload: &meetingsvc.CreateMeetingPayload{
				Title:     "Test Meeting",
				StartTime: time.Now().Add(time.Hour * 24).Format(time.RFC3339),
			},
			setupMocks: func(mockRepo *mocks.MockMeetingRepository, mockBuilder *mocks.MockMessageBuilder) {
				mockRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.MeetingBase"), mock.AnythingOfType("*models.MeetingSettings")).Return(domain.ErrInternal)
			},
			wantErr:     true,
			expectedErr: domain.ErrInternal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, mockRepo, mockBuilder, mockAuth := setupServiceForTesting()

			if tt.name == "service not ready" {
				service.MeetingRepository = nil
			}

			tt.setupMocks(mockRepo, mockBuilder)

			result, err := service.CreateMeeting(context.Background(), tt.payload)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.expectedErr != nil {
					assert.Equal(t, tt.expectedErr, err)
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
			mockAuth.AssertExpectations(t)
		})
	}
}

func TestMeetingsService_GetMeetingBase(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name        string
		payload     *meetingsvc.GetMeetingBasePayload
		setupMocks  func(*mocks.MockMeetingRepository, *mocks.MockMessageBuilder)
		wantErr     bool
		expectedErr error
		validate    func(*testing.T, *meetingsvc.GetMeetingBaseResult)
	}{
		{
			name: "successful get meeting",
			payload: &meetingsvc.GetMeetingBasePayload{
				UID: utils.StringPtr("test-meeting-uid"),
			},
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
			validate: func(t *testing.T, result *meetingsvc.GetMeetingBaseResult) {
				assert.NotNil(t, result)
				assert.NotNil(t, result.Meeting)
				assert.Equal(t, "test-meeting-uid", *result.Meeting.UID)
				assert.Equal(t, "Test Meeting", *result.Meeting.Title)
			},
		},
		{
			name: "meeting not found",
			payload: &meetingsvc.GetMeetingBasePayload{
				UID: utils.StringPtr("non-existent-uid"),
			},
			setupMocks: func(mockRepo *mocks.MockMeetingRepository, mockBuilder *mocks.MockMessageBuilder) {
				mockRepo.On("GetBaseWithRevision", mock.Anything, "non-existent-uid").Return(
					nil, uint64(0), domain.ErrMeetingNotFound,
				)
			},
			wantErr:     true,
			expectedErr: domain.ErrMeetingNotFound,
		},
		{
			name:    "nil payload",
			payload: nil,
			setupMocks: func(mockRepo *mocks.MockMeetingRepository, mockBuilder *mocks.MockMessageBuilder) {
				// No repo calls expected
			},
			wantErr:     true,
			expectedErr: domain.ErrValidationFailed,
		},
		{
			name: "empty UID",
			payload: &meetingsvc.GetMeetingBasePayload{
				UID: utils.StringPtr(""),
			},
			setupMocks: func(mockRepo *mocks.MockMeetingRepository, mockBuilder *mocks.MockMessageBuilder) {
				// Repository is called even with empty UID, but returns error
				mockRepo.On("GetBaseWithRevision", mock.Anything, "").Return(
					nil, uint64(0), domain.ErrValidationFailed,
				)
			},
			wantErr:     true,
			expectedErr: domain.ErrInternal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, mockRepo, mockBuilder, mockAuth := setupServiceForTesting()
			tt.setupMocks(mockRepo, mockBuilder)

			result, etag, err := service.GetMeetingBase(context.Background(), tt.payload)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.expectedErr != nil {
					assert.Equal(t, tt.expectedErr, err)
				}
				assert.Nil(t, result)
				assert.Empty(t, etag)
			} else {
				assert.NoError(t, err)
				require.NotNil(t, result)
				if tt.validate != nil {
					tt.validate(t, &meetingsvc.GetMeetingBaseResult{
						Meeting: result,
						Etag:    utils.StringPtr(etag),
					})
				}
			}

			mockRepo.AssertExpectations(t)
			mockBuilder.AssertExpectations(t)
			mockAuth.AssertExpectations(t)
		})
	}
}

func TestMeetingsService_GetMeetingSettings(t *testing.T) {
	tests := []struct {
		name        string
		payload     *meetingsvc.GetMeetingSettingsPayload
		setupMocks  func(*mocks.MockMeetingRepository, *mocks.MockMessageBuilder)
		wantErr     bool
		expectedErr error
		validate    func(*testing.T, *meetingsvc.GetMeetingSettingsResult)
	}{
		{
			name: "successful get meeting settings",
			payload: &meetingsvc.GetMeetingSettingsPayload{
				UID: utils.StringPtr("meeting-123"),
			},
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
			validate: func(t *testing.T, result *meetingsvc.GetMeetingSettingsResult) {
				assert.NotNil(t, result)
				assert.NotNil(t, result.MeetingSettings)
				assert.Equal(t, "meeting-123", *result.MeetingSettings.UID)
				assert.Len(t, result.MeetingSettings.Organizers, 2)
				assert.Equal(t, "1", *result.Etag)
			},
		},
		{
			name: "meeting settings not found",
			payload: &meetingsvc.GetMeetingSettingsPayload{
				UID: utils.StringPtr("nonexistent-meeting"),
			},
			setupMocks: func(mockRepo *mocks.MockMeetingRepository, mockBuilder *mocks.MockMessageBuilder) {
				mockRepo.On("GetSettingsWithRevision", mock.Anything, "nonexistent-meeting").Return(nil, uint64(0), domain.ErrMeetingNotFound)
			},
			wantErr:     true,
			expectedErr: domain.ErrMeetingNotFound,
		},
		{
			name:        "nil payload",
			payload:     nil,
			setupMocks:  func(*mocks.MockMeetingRepository, *mocks.MockMessageBuilder) {},
			wantErr:     true,
			expectedErr: domain.ErrValidationFailed,
		},
		{
			name: "service not ready",
			payload: &meetingsvc.GetMeetingSettingsPayload{
				UID: utils.StringPtr("meeting-123"),
			},
			setupMocks:  func(*mocks.MockMeetingRepository, *mocks.MockMessageBuilder) {},
			wantErr:     true,
			expectedErr: domain.ErrServiceUnavailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, mockRepo, mockBuilder, mockAuth := setupServiceForTesting()

			if tt.name == "service not ready" {
				service.MeetingRepository = nil
			}

			tt.setupMocks(mockRepo, mockBuilder)

			result, etag, err := service.GetMeetingSettings(context.Background(), tt.payload)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.expectedErr != nil {
					assert.Equal(t, tt.expectedErr, err)
				}
				assert.Nil(t, result)
				assert.Empty(t, etag)
			} else {
				assert.NoError(t, err)
				require.NotNil(t, result)
				if tt.validate != nil {
					tt.validate(t, &meetingsvc.GetMeetingSettingsResult{
						MeetingSettings: result,
						Etag:            utils.StringPtr(etag),
					})
				}
			}

			mockRepo.AssertExpectations(t)
			mockBuilder.AssertExpectations(t)
			mockAuth.AssertExpectations(t)
		})
	}
}

func TestMeetingsService_UpdateMeetingSettings(t *testing.T) {
	tests := []struct {
		name        string
		payload     *meetingsvc.UpdateMeetingSettingsPayload
		setupMocks  func(*mocks.MockMeetingRepository, *mocks.MockMessageBuilder)
		wantErr     bool
		expectedErr error
		validate    func(*testing.T, *meetingsvc.MeetingSettings)
	}{
		{
			name: "successful update meeting settings",
			payload: &meetingsvc.UpdateMeetingSettingsPayload{
				UID:        utils.StringPtr("meeting-123"),
				IfMatch:    utils.StringPtr("1"),
				Organizers: []string{"org3", "org4"},
			},
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
					Visibility: "public",
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
			validate: func(t *testing.T, result *meetingsvc.MeetingSettings) {
				assert.NotNil(t, result)
				assert.Equal(t, "meeting-123", *result.UID)
				assert.Len(t, result.Organizers, 2)
				assert.Equal(t, "org3", result.Organizers[0])
				assert.Equal(t, "org4", result.Organizers[1])
			},
		},
		{
			name: "meeting settings not found",
			payload: &meetingsvc.UpdateMeetingSettingsPayload{
				UID:        utils.StringPtr("nonexistent-meeting"),
				IfMatch:    utils.StringPtr("1"),
				Organizers: []string{"org1"},
			},
			setupMocks: func(mockRepo *mocks.MockMeetingRepository, mockBuilder *mocks.MockMessageBuilder) {
				mockRepo.On("GetSettings", mock.Anything, "nonexistent-meeting").Return(nil, domain.ErrMeetingNotFound)
			},
			wantErr:     true,
			expectedErr: domain.ErrMeetingNotFound,
		},
		{
			name:        "nil payload",
			payload:     nil,
			setupMocks:  func(*mocks.MockMeetingRepository, *mocks.MockMessageBuilder) {},
			wantErr:     true,
			expectedErr: domain.ErrValidationFailed,
		},
		{
			name: "service not ready",
			payload: &meetingsvc.UpdateMeetingSettingsPayload{
				UID:        utils.StringPtr("meeting-123"),
				Organizers: []string{"org1"},
			},
			setupMocks:  func(*mocks.MockMeetingRepository, *mocks.MockMessageBuilder) {},
			wantErr:     true,
			expectedErr: domain.ErrServiceUnavailable,
		},
		{
			name: "nil UID",
			payload: &meetingsvc.UpdateMeetingSettingsPayload{
				UID:        nil,
				IfMatch:    utils.StringPtr("1"),
				Organizers: []string{"org1"},
			},
			setupMocks:  func(*mocks.MockMeetingRepository, *mocks.MockMessageBuilder) {},
			wantErr:     true,
			expectedErr: domain.ErrValidationFailed,
		},
		{
			name: "missing If-Match header",
			payload: &meetingsvc.UpdateMeetingSettingsPayload{
				UID:        utils.StringPtr("meeting-123"),
				IfMatch:    nil,
				Organizers: []string{"org1"},
			},
			setupMocks:  func(*mocks.MockMeetingRepository, *mocks.MockMessageBuilder) {},
			wantErr:     true,
			expectedErr: domain.ErrValidationFailed,
		},
		{
			name: "skip etag validation mode",
			payload: &meetingsvc.UpdateMeetingSettingsPayload{
				UID:        utils.StringPtr("meeting-789"),
				IfMatch:    nil,
				Organizers: []string{"org5", "org6"},
			},
			setupMocks: func(mockRepo *mocks.MockMeetingRepository, mockBuilder *mocks.MockMessageBuilder) {
				now := time.Now()
				existingSettings := &models.MeetingSettings{
					UID:        "meeting-789",
					Organizers: []string{"old-org"},
					CreatedAt:  &now,
					UpdatedAt:  &now,
				}

				// This call happens when ETag validation is skipped
				mockRepo.On("GetSettingsWithRevision", mock.Anything, "meeting-789").Return(existingSettings, uint64(456), nil)
				// This call happens to get the existing settings for the update
				mockRepo.On("GetSettings", mock.Anything, "meeting-789").Return(existingSettings, nil)

				mockRepo.On("UpdateSettings", mock.Anything, mock.MatchedBy(func(settings *models.MeetingSettings) bool {
					return settings.UID == "meeting-789" &&
						len(settings.Organizers) == 2 &&
						settings.Organizers[0] == "org5" &&
						settings.Organizers[1] == "org6"
				}), uint64(456)).Return(nil)

				mockBuilder.On("SendIndexMeetingSettings", mock.Anything, models.ActionUpdated, mock.Anything).Return(nil)

				mockRepo.On("GetBase", mock.Anything, "meeting-789").Return(&models.MeetingBase{
					UID:        "meeting-789",
					ProjectUID: "project-789",
					Visibility: "public",
					Committees: []models.Committee{},
				}, nil)

				mockBuilder.On("SendUpdateAccessMeeting", mock.Anything, mock.Anything).Return(nil)
			},
			wantErr: false,
			validate: func(t *testing.T, result *meetingsvc.MeetingSettings) {
				assert.NotNil(t, result)
				assert.Equal(t, "meeting-789", *result.UID)
				assert.Len(t, result.Organizers, 2)
				assert.Equal(t, "org5", result.Organizers[0])
				assert.Equal(t, "org6", result.Organizers[1])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, mockRepo, mockBuilder, mockAuth := setupServiceForTesting()

			if tt.name == "service not ready" {
				service.MeetingRepository = nil
			}

			if tt.name == "skip etag validation mode" {
				service.Config.SkipEtagValidation = true
			}

			tt.setupMocks(mockRepo, mockBuilder)

			result, err := service.UpdateMeetingSettings(context.Background(), tt.payload)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.expectedErr != nil {
					assert.Equal(t, tt.expectedErr, err)
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
			mockAuth.AssertExpectations(t)
		})
	}
}
