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

func TestMeetingsService_CreateMeetingRegistrant(t *testing.T) {
	tests := []struct {
		name               string
		payload            *meetingsvc.CreateMeetingRegistrantPayload
		setupMocks         func(*domain.MockMeetingRepository, *domain.MockRegistrantRepository, *domain.MockMessageBuilder)
		expectedEmail      string
		wantErr            bool
		expectedErr        error
		expectedErrContext string
	}{
		{
			name: "successful create registrant",
			payload: &meetingsvc.CreateMeetingRegistrantPayload{
				MeetingUID: "meeting-1",
				Email:      "user@example.com",
				FirstName:  "John",
				LastName:   "Doe",
				Host:       utils.BoolPtr(false),
			},
			setupMocks: func(mockMeetingRepo *domain.MockMeetingRepository, mockRegistrantRepo *domain.MockRegistrantRepository, mockBuilder *domain.MockMessageBuilder) {
				// Meeting exists check
				mockMeetingRepo.On("GetMeeting", mock.Anything, "meeting-1").Return(&models.Meeting{
					UID: "meeting-1",
				}, nil)
				// Create registrant
				mockRegistrantRepo.On("CreateRegistrant", mock.Anything, mock.MatchedBy(func(r *models.Registrant) bool {
					return r.Email == "user@example.com" && r.FirstName == "John" && r.LastName == "Doe" && r.MeetingUID == "meeting-1"
				})).Return(nil)
			},
			expectedEmail: "user@example.com",
			wantErr:       false,
		},
		{
			name: "meeting not found",
			payload: &meetingsvc.CreateMeetingRegistrantPayload{
				MeetingUID: "nonexistent-meeting",
				Email:      "user@example.com",
				FirstName:  "John",
				LastName:   "Doe",
			},
			setupMocks: func(mockMeetingRepo *domain.MockMeetingRepository, mockRegistrantRepo *domain.MockRegistrantRepository, mockBuilder *domain.MockMessageBuilder) {
				mockMeetingRepo.On("GetMeeting", mock.Anything, "nonexistent-meeting").Return(nil, domain.ErrMeetingNotFound)
			},
			wantErr:     true,
			expectedErr: domain.ErrMeetingNotFound,
		},
		{
			name:               "nil payload",
			payload:            nil,
			setupMocks:         func(*domain.MockMeetingRepository, *domain.MockRegistrantRepository, *domain.MockMessageBuilder) {},
			wantErr:            true,
			expectedErr:        domain.ErrValidationFailed,
			expectedErrContext: "payload is required",
		},
		{
			name: "registrant already exists",
			payload: &meetingsvc.CreateMeetingRegistrantPayload{
				MeetingUID: "meeting-1",
				Email:      "existing@example.com",
				FirstName:  "Jane",
				LastName:   "Smith",
			},
			setupMocks: func(mockMeetingRepo *domain.MockMeetingRepository, mockRegistrantRepo *domain.MockRegistrantRepository, mockBuilder *domain.MockMessageBuilder) {
				mockMeetingRepo.On("GetMeeting", mock.Anything, "meeting-1").Return(&models.Meeting{
					UID: "meeting-1",
				}, nil)
				mockRegistrantRepo.On("CreateRegistrant", mock.Anything, mock.AnythingOfType("*models.Registrant")).Return(domain.ErrRegistrantAlreadyExists)
			},
			wantErr:     true,
			expectedErr: domain.ErrRegistrantAlreadyExists,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock repositories
			mockMeetingRepo := &domain.MockMeetingRepository{}
			mockRegistrantRepo := &domain.MockRegistrantRepository{}
			mockBuilder := &domain.MockMessageBuilder{}

			// Setup mocks
			tt.setupMocks(mockMeetingRepo, mockRegistrantRepo, mockBuilder)

			// Create service
			service := &MeetingsService{
				MeetingRepository:    mockMeetingRepo,
				RegistrantRepository: mockRegistrantRepo,
				MessageBuilder:       mockBuilder,
				Config:               ServiceConfig{},
			}

			// Execute
			result, err := service.CreateMeetingRegistrant(context.Background(), tt.payload)

			// Assert error expectations
			if tt.wantErr {
				require.Error(t, err)
				if tt.expectedErr != nil {
					assert.ErrorIs(t, err, tt.expectedErr)
				}
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				if tt.expectedEmail != "" {
					assert.Equal(t, tt.expectedEmail, result.Email)
				}
				assert.NotEmpty(t, result.UID)
				assert.NotNil(t, result.CreatedAt)
				assert.NotNil(t, result.UpdatedAt)
			}

			// Verify all mocks were called as expected
			mockMeetingRepo.AssertExpectations(t)
			mockRegistrantRepo.AssertExpectations(t)
			mockBuilder.AssertExpectations(t)
		})
	}
}

func TestMeetingsService_GetMeetingRegistrants(t *testing.T) {
	tests := []struct {
		name        string
		payload     *meetingsvc.GetMeetingRegistrantsPayload
		setupMocks  func(*domain.MockMeetingRepository, *domain.MockRegistrantRepository, *domain.MockMessageBuilder)
		expectedLen int
		wantErr     bool
		expectedErr error
	}{
		{
			name: "successful get meeting registrants",
			payload: &meetingsvc.GetMeetingRegistrantsPayload{
				UID: utils.StringPtr("meeting-1"),
			},
			setupMocks: func(mockMeetingRepo *domain.MockMeetingRepository, mockRegistrantRepo *domain.MockRegistrantRepository, mockBuilder *domain.MockMessageBuilder) {
				// Meeting exists check
				mockMeetingRepo.On("GetMeeting", mock.Anything, "meeting-1").Return(&models.Meeting{
					UID: "meeting-1",
				}, nil)
				// Get registrants
				now := time.Now()
				mockRegistrantRepo.On("ListMeetingRegistrants", mock.Anything, "meeting-1").Return([]*models.Registrant{
					{
						UID:        "registrant-1",
						MeetingUID: "meeting-1",
						Email:      "user1@example.com",
						FirstName:  "John",
						LastName:   "Doe",
						CreatedAt:  &now,
						UpdatedAt:  &now,
					},
					{
						UID:        "registrant-2",
						MeetingUID: "meeting-1",
						Email:      "user2@example.com",
						FirstName:  "Jane",
						LastName:   "Smith",
						CreatedAt:  &now,
						UpdatedAt:  &now,
					},
				}, nil)
			},
			expectedLen: 2,
			wantErr:     false,
		},
		{
			name: "meeting not found",
			payload: &meetingsvc.GetMeetingRegistrantsPayload{
				UID: utils.StringPtr("nonexistent-meeting"),
			},
			setupMocks: func(mockMeetingRepo *domain.MockMeetingRepository, mockRegistrantRepo *domain.MockRegistrantRepository, mockBuilder *domain.MockMessageBuilder) {
				mockMeetingRepo.On("GetMeeting", mock.Anything, "nonexistent-meeting").Return(nil, domain.ErrMeetingNotFound)
			},
			wantErr:     true,
			expectedErr: domain.ErrMeetingNotFound,
		},
		{
			name:        "nil payload",
			payload:     nil,
			setupMocks:  func(*domain.MockMeetingRepository, *domain.MockRegistrantRepository, *domain.MockMessageBuilder) {},
			wantErr:     true,
			expectedErr: domain.ErrValidationFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock repositories
			mockMeetingRepo := &domain.MockMeetingRepository{}
			mockRegistrantRepo := &domain.MockRegistrantRepository{}
			mockBuilder := &domain.MockMessageBuilder{}

			// Setup mocks
			tt.setupMocks(mockMeetingRepo, mockRegistrantRepo, mockBuilder)

			// Create service
			service := &MeetingsService{
				MeetingRepository:    mockMeetingRepo,
				RegistrantRepository: mockRegistrantRepo,
				MessageBuilder:       mockBuilder,
				Config:               ServiceConfig{},
			}

			// Execute
			result, err := service.GetMeetingRegistrants(context.Background(), tt.payload)

			// Assert error expectations
			if tt.wantErr {
				require.Error(t, err)
				if tt.expectedErr != nil {
					assert.ErrorIs(t, err, tt.expectedErr)
				}
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				assert.Len(t, result.Registrants, tt.expectedLen)
			}

			// Verify all mocks were called as expected
			mockMeetingRepo.AssertExpectations(t)
			mockRegistrantRepo.AssertExpectations(t)
			mockBuilder.AssertExpectations(t)
		})
	}
}

func TestMeetingsService_GetMeetingRegistrant(t *testing.T) {
	tests := []struct {
		name          string
		payload       *meetingsvc.GetMeetingRegistrantPayload
		setupMocks    func(*domain.MockMeetingRepository, *domain.MockRegistrantRepository, *domain.MockMessageBuilder)
		expectedEmail string
		wantErr       bool
		expectedErr   error
	}{
		{
			name: "successful get meeting registrant",
			payload: &meetingsvc.GetMeetingRegistrantPayload{
				MeetingUID: utils.StringPtr("meeting-1"),
				UID:        utils.StringPtr("registrant-1"),
			},
			setupMocks: func(mockMeetingRepo *domain.MockMeetingRepository, mockRegistrantRepo *domain.MockRegistrantRepository, mockBuilder *domain.MockMessageBuilder) {
				now := time.Now()
				mockRegistrantRepo.On("GetRegistrantWithRevision", mock.Anything, "registrant-1").Return(&models.Registrant{
					UID:        "registrant-1",
					MeetingUID: "meeting-1",
					Email:      "user@example.com",
					FirstName:  "John",
					LastName:   "Doe",
					CreatedAt:  &now,
					UpdatedAt:  &now,
				}, uint64(1), nil)
			},
			expectedEmail: "user@example.com",
			wantErr:       false,
		},
		{
			name: "registrant not found",
			payload: &meetingsvc.GetMeetingRegistrantPayload{
				MeetingUID: utils.StringPtr("meeting-1"),
				UID:        utils.StringPtr("nonexistent-registrant"),
			},
			setupMocks: func(mockMeetingRepo *domain.MockMeetingRepository, mockRegistrantRepo *domain.MockRegistrantRepository, mockBuilder *domain.MockMessageBuilder) {
				mockRegistrantRepo.On("GetRegistrantWithRevision", mock.Anything, "nonexistent-registrant").Return(nil, uint64(0), domain.ErrRegistrantNotFound)
			},
			wantErr:     true,
			expectedErr: domain.ErrRegistrantNotFound,
		},
		{
			name:        "nil payload",
			payload:     nil,
			setupMocks:  func(*domain.MockMeetingRepository, *domain.MockRegistrantRepository, *domain.MockMessageBuilder) {},
			wantErr:     true,
			expectedErr: domain.ErrValidationFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock repositories
			mockMeetingRepo := &domain.MockMeetingRepository{}
			mockRegistrantRepo := &domain.MockRegistrantRepository{}
			mockBuilder := &domain.MockMessageBuilder{}

			// Setup mocks
			tt.setupMocks(mockMeetingRepo, mockRegistrantRepo, mockBuilder)

			// Create service
			service := &MeetingsService{
				MeetingRepository:    mockMeetingRepo,
				RegistrantRepository: mockRegistrantRepo,
				MessageBuilder:       mockBuilder,
				Config:               ServiceConfig{},
			}

			// Execute
			result, err := service.GetMeetingRegistrant(context.Background(), tt.payload)

			// Assert error expectations
			if tt.wantErr {
				require.Error(t, err)
				if tt.expectedErr != nil {
					assert.ErrorIs(t, err, tt.expectedErr)
				}
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.NotNil(t, result.Registrant)
				if tt.expectedEmail != "" {
					assert.Equal(t, tt.expectedEmail, result.Registrant.Email)
				}
				assert.NotNil(t, result.Etag)
			}

			// Verify all mocks were called as expected
			mockMeetingRepo.AssertExpectations(t)
			mockRegistrantRepo.AssertExpectations(t)
			mockBuilder.AssertExpectations(t)
		})
	}
}

func TestMeetingsService_UpdateMeetingRegistrant(t *testing.T) {
	tests := []struct {
		name        string
		payload     *meetingsvc.UpdateMeetingRegistrantPayload
		setupMocks  func(*domain.MockMeetingRepository, *domain.MockRegistrantRepository, *domain.MockMessageBuilder)
		wantErr     bool
		expectedErr error
	}{
		{
			name: "successful update registrant",
			payload: &meetingsvc.UpdateMeetingRegistrantPayload{
				MeetingUID: "meeting-1",
				UID:        utils.StringPtr("registrant-1"),
				Email:      "updated@example.com",
				FirstName:  "John",
				LastName:   "Doe",
				Etag:       utils.StringPtr("1"),
			},
			setupMocks: func(mockMeetingRepo *domain.MockMeetingRepository, mockRegistrantRepo *domain.MockRegistrantRepository, mockBuilder *domain.MockMessageBuilder) {
				now := time.Now()
				existingRegistrant := &models.Registrant{
					UID:        "registrant-1",
					MeetingUID: "meeting-1",
					Email:      "old@example.com",
					FirstName:  "John",
					LastName:   "Doe",
					CreatedAt:  &now,
					UpdatedAt:  &now,
				}
				mockRegistrantRepo.On("GetRegistrant", mock.Anything, "registrant-1").Return(existingRegistrant, nil)
				mockRegistrantRepo.On("UpdateRegistrant", mock.Anything, mock.MatchedBy(func(r *models.Registrant) bool {
					return r.Email == "updated@example.com" && r.UID == "registrant-1"
				}), uint64(1)).Return(nil)
			},
			wantErr: false,
		},
		{
			name:        "nil payload",
			payload:     nil,
			setupMocks:  func(*domain.MockMeetingRepository, *domain.MockRegistrantRepository, *domain.MockMessageBuilder) {},
			wantErr:     true,
			expectedErr: domain.ErrValidationFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock repositories
			mockMeetingRepo := &domain.MockMeetingRepository{}
			mockRegistrantRepo := &domain.MockRegistrantRepository{}
			mockBuilder := &domain.MockMessageBuilder{}

			// Setup mocks
			tt.setupMocks(mockMeetingRepo, mockRegistrantRepo, mockBuilder)

			// Create service
			service := &MeetingsService{
				MeetingRepository:    mockMeetingRepo,
				RegistrantRepository: mockRegistrantRepo,
				MessageBuilder:       mockBuilder,
				Config:               ServiceConfig{},
			}

			// Execute
			result, err := service.UpdateMeetingRegistrant(context.Background(), tt.payload)

			// Assert error expectations
			if tt.wantErr {
				require.Error(t, err)
				if tt.expectedErr != nil {
					assert.ErrorIs(t, err, tt.expectedErr)
				}
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
			}

			// Verify all mocks were called as expected
			mockMeetingRepo.AssertExpectations(t)
			mockRegistrantRepo.AssertExpectations(t)
			mockBuilder.AssertExpectations(t)
		})
	}
}

func TestMeetingsService_DeleteMeetingRegistrant(t *testing.T) {
	tests := []struct {
		name        string
		payload     *meetingsvc.DeleteMeetingRegistrantPayload
		setupMocks  func(*domain.MockMeetingRepository, *domain.MockRegistrantRepository, *domain.MockMessageBuilder)
		wantErr     bool
		expectedErr error
	}{
		{
			name: "successful delete registrant",
			payload: &meetingsvc.DeleteMeetingRegistrantPayload{
				MeetingUID: utils.StringPtr("meeting-1"),
				UID:        utils.StringPtr("registrant-1"),
				Etag:       utils.StringPtr("1"),
			},
			setupMocks: func(mockMeetingRepo *domain.MockMeetingRepository, mockRegistrantRepo *domain.MockRegistrantRepository, mockBuilder *domain.MockMessageBuilder) {
				mockRegistrantRepo.On("DeleteRegistrant", mock.Anything, "registrant-1", uint64(1)).Return(nil)
			},
			wantErr: false,
		},
		{
			name:        "nil payload",
			payload:     nil,
			setupMocks:  func(*domain.MockMeetingRepository, *domain.MockRegistrantRepository, *domain.MockMessageBuilder) {},
			wantErr:     true,
			expectedErr: domain.ErrValidationFailed,
		},
		{
			name: "registrant not found",
			payload: &meetingsvc.DeleteMeetingRegistrantPayload{
				MeetingUID: utils.StringPtr("meeting-1"),
				UID:        utils.StringPtr("nonexistent-registrant"),
				Etag:       utils.StringPtr("1"),
			},
			setupMocks: func(mockMeetingRepo *domain.MockMeetingRepository, mockRegistrantRepo *domain.MockRegistrantRepository, mockBuilder *domain.MockMessageBuilder) {
				mockRegistrantRepo.On("DeleteRegistrant", mock.Anything, "nonexistent-registrant", uint64(1)).Return(domain.ErrRegistrantNotFound)
			},
			wantErr:     true,
			expectedErr: domain.ErrRegistrantNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock repositories
			mockMeetingRepo := &domain.MockMeetingRepository{}
			mockRegistrantRepo := &domain.MockRegistrantRepository{}
			mockBuilder := &domain.MockMessageBuilder{}

			// Setup mocks
			tt.setupMocks(mockMeetingRepo, mockRegistrantRepo, mockBuilder)

			// Create service
			service := &MeetingsService{
				MeetingRepository:    mockMeetingRepo,
				RegistrantRepository: mockRegistrantRepo,
				MessageBuilder:       mockBuilder,
				Config:               ServiceConfig{},
			}

			// Execute
			err := service.DeleteMeetingRegistrant(context.Background(), tt.payload)

			// Assert error expectations
			if tt.wantErr {
				require.Error(t, err)
				if tt.expectedErr != nil {
					assert.ErrorIs(t, err, tt.expectedErr)
				}
			} else {
				require.NoError(t, err)
			}

			// Verify all mocks were called as expected
			mockMeetingRepo.AssertExpectations(t)
			mockRegistrantRepo.AssertExpectations(t)
			mockBuilder.AssertExpectations(t)
		})
	}
}
