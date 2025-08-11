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
			setupMocks: func(mockRepo *domain.MockMeetingRepository, mockBuilder *domain.MockMessageBuilder) {
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
		setupMocks  func(*domain.MockMeetingRepository, *domain.MockMessageBuilder)
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
			setupMocks: func(mockRepo *domain.MockMeetingRepository, mockBuilder *domain.MockMessageBuilder) {
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
			setupMocks: func(mockRepo *domain.MockMeetingRepository, mockBuilder *domain.MockMessageBuilder) {
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
			setupMocks: func(mockRepo *domain.MockMeetingRepository, mockBuilder *domain.MockMessageBuilder) {
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
		setupMocks  func(*domain.MockMeetingRepository, *domain.MockMessageBuilder)
		wantErr     bool
		expectedErr error
		validate    func(*testing.T, *meetingsvc.GetMeetingBaseResult)
	}{
		{
			name: "successful get meeting",
			payload: &meetingsvc.GetMeetingBasePayload{
				UID: utils.StringPtr("test-meeting-uid"),
			},
			setupMocks: func(mockRepo *domain.MockMeetingRepository, mockBuilder *domain.MockMessageBuilder) {
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
			setupMocks: func(mockRepo *domain.MockMeetingRepository, mockBuilder *domain.MockMessageBuilder) {
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
			setupMocks: func(mockRepo *domain.MockMeetingRepository, mockBuilder *domain.MockMessageBuilder) {
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
			setupMocks: func(mockRepo *domain.MockMeetingRepository, mockBuilder *domain.MockMessageBuilder) {
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
