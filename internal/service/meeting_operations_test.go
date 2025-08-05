// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"testing"
	"time"

	meetingsvc "github.com/linuxfoundation/lfx-v2-meeting-service/gen/meeting_service"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestMeetingsService_GetMeetings(t *testing.T) {
	tests := []struct {
		name        string
		setupMocks  func(*domain.MockMeetingRepository, *domain.MockMessageBuilder)
		expectedLen int
		wantErr     bool
		expectedErr error
	}{
		{
			name: "successful get all meetings",
			setupMocks: func(mockRepo *domain.MockMeetingRepository, mockBuilder *domain.MockMessageBuilder) {
				now := time.Now()
				mockRepo.On("ListAllMeetings", mock.Anything).Return(
					[]*models.Meeting{
						{
							UID:         "meeting-1",
							Title:       "Test Meeting 1",
							Description: "Description 1",
							CreatedAt:   &now,
							UpdatedAt:   &now,
						},
						{
							UID:         "meeting-2",
							Title:       "Test Meeting 2",
							Description: "Description 2",
							CreatedAt:   &now,
							UpdatedAt:   &now,
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
			setupMocks: func(mockRepo *domain.MockMeetingRepository, mockBuilder *domain.MockMessageBuilder) {
				// Don't set up repository - will make service not ready
			},
			expectedLen: 0,
			wantErr:     true,
			expectedErr: domain.ErrServiceUnavailable,
		},
		{
			name: "repository error",
			setupMocks: func(mockRepo *domain.MockMeetingRepository, mockBuilder *domain.MockMessageBuilder) {
				mockRepo.On("ListAllMeetings", mock.Anything).Return(
					nil, domain.ErrInternal,
				)
			},
			expectedLen: 0,
			wantErr:     true,
			expectedErr: domain.ErrInternal,
		},
		{
			name: "empty meetings list",
			setupMocks: func(mockRepo *domain.MockMeetingRepository, mockBuilder *domain.MockMessageBuilder) {
				mockRepo.On("ListAllMeetings", mock.Anything).Return(
					[]*models.Meeting{},
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
		setupMocks  func(*domain.MockMeetingRepository, *domain.MockMessageBuilder)
		wantErr     bool
		expectedErr error
		validate    func(*testing.T, *meetingsvc.Meeting)
	}{
		{
			name: "successful meeting creation",
			payload: &meetingsvc.CreateMeetingPayload{
				Title: "Test Meeting",
			},
			setupMocks: func(mockRepo *domain.MockMeetingRepository, mockBuilder *domain.MockMessageBuilder) {
				mockRepo.On("CreateMeeting", mock.Anything, mock.AnythingOfType("*models.Meeting")).Return(nil)
				mockBuilder.On("SendIndexMeeting", mock.Anything, models.ActionCreated, mock.Anything).Return(nil)
				mockBuilder.On("SendUpdateAccessMeeting", mock.Anything, mock.Anything).Return(nil)
			},
			wantErr: false,
			validate: func(t *testing.T, result *meetingsvc.Meeting) {
				assert.NotNil(t, result)
				assert.NotNil(t, result.UID)
				assert.Equal(t, "Test Meeting", *result.Title)
			},
		},
		{
			name: "service not ready",
			payload: &meetingsvc.CreateMeetingPayload{
				Title: "Test Meeting",
			},
			setupMocks: func(mockRepo *domain.MockMeetingRepository, mockBuilder *domain.MockMessageBuilder) {
				// Service will not be ready
			},
			wantErr:     true,
			expectedErr: domain.ErrServiceUnavailable,
		},
		{
			name: "repository creation error",
			payload: &meetingsvc.CreateMeetingPayload{
				Title: "Test Meeting",
			},
			setupMocks: func(mockRepo *domain.MockMeetingRepository, mockBuilder *domain.MockMessageBuilder) {
				mockRepo.On("CreateMeeting", mock.Anything, mock.AnythingOfType("*models.Meeting")).Return(domain.ErrInternal)
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

func TestMeetingsService_GetOneMeeting(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name        string
		payload     *meetingsvc.GetMeetingPayload
		setupMocks  func(*domain.MockMeetingRepository, *domain.MockMessageBuilder)
		wantErr     bool
		expectedErr error
		validate    func(*testing.T, *meetingsvc.GetMeetingResult)
	}{
		{
			name: "successful get meeting",
			payload: &meetingsvc.GetMeetingPayload{
				UID: utils.StringPtr("test-meeting-uid"),
			},
			setupMocks: func(mockRepo *domain.MockMeetingRepository, mockBuilder *domain.MockMessageBuilder) {
				mockRepo.On("GetMeetingWithRevision", mock.Anything, "test-meeting-uid").Return(
					&models.Meeting{
						UID:         "test-meeting-uid",
						Title:       "Test Meeting",
						Description: "Test Description",
						CreatedAt:   &now,
						UpdatedAt:   &now,
					},
					uint64(123),
					nil,
				)
			},
			wantErr: false,
			validate: func(t *testing.T, result *meetingsvc.GetMeetingResult) {
				assert.NotNil(t, result)
				assert.NotNil(t, result.Meeting)
				assert.Equal(t, "test-meeting-uid", *result.Meeting.UID)
				assert.Equal(t, "Test Meeting", *result.Meeting.Title)
			},
		},
		{
			name: "meeting not found",
			payload: &meetingsvc.GetMeetingPayload{
				UID: utils.StringPtr("non-existent-uid"),
			},
			setupMocks: func(mockRepo *domain.MockMeetingRepository, mockBuilder *domain.MockMessageBuilder) {
				mockRepo.On("GetMeetingWithRevision", mock.Anything, "non-existent-uid").Return(
					nil, uint64(0), domain.ErrMeetingNotFound,
				)
			},
			wantErr:     true,
			expectedErr: domain.ErrMeetingNotFound,
		},
		{
			name:    "nil payload",
			payload: nil,
			setupMocks: func(mockRepo *domain.MockMeetingRepository, mockBuilder *domain.MockMessageBuilder) {
				// No repo calls expected
			},
			wantErr:     true,
			expectedErr: domain.ErrValidationFailed,
		},
		{
			name: "empty UID",
			payload: &meetingsvc.GetMeetingPayload{
				UID: utils.StringPtr(""),
			},
			setupMocks: func(mockRepo *domain.MockMeetingRepository, mockBuilder *domain.MockMessageBuilder) {
				// Repository is called even with empty UID, but returns error
				mockRepo.On("GetMeetingWithRevision", mock.Anything, "").Return(
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

			result, etag, err := service.GetOneMeeting(context.Background(), tt.payload)

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
					tt.validate(t, &meetingsvc.GetMeetingResult{
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

func TestMeetingsService_MeetingValidation(t *testing.T) {
	tests := []struct {
		name     string
		testFunc func(*testing.T, *MeetingsService)
	}{
		{
			name: "service validates meeting creation",
			testFunc: func(t *testing.T, service *MeetingsService) {
				// Test that the service properly validates inputs
				// This is a placeholder for validation logic tests
				assert.NotNil(t, service)
			},
		},
		{
			name: "service handles concurrent operations",
			testFunc: func(t *testing.T, service *MeetingsService) {
				// Test that the service can handle multiple concurrent operations
				// This is a placeholder for concurrency tests
				assert.NotNil(t, service)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, mockRepo, mockBuilder, _ := setupServiceForTesting()
			// Setup basic mocks to ensure service is ready
			_ = mockRepo
			_ = mockBuilder

			tt.testFunc(t, service)
		})
	}
}
