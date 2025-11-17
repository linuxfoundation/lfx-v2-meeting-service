// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/mocks"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/constants"
)

func setupPastMeetingParticipantServiceForTesting() (*PastMeetingParticipantService, *mocks.MockMeetingRepository, *mocks.MockPastMeetingRepository, *mocks.MockPastMeetingParticipantRepository, *mocks.MockMessageBuilder) {
	mockMeetingRepo := &mocks.MockMeetingRepository{}
	mockPastMeetingRepo := &mocks.MockPastMeetingRepository{}
	mockParticipantRepo := &mocks.MockPastMeetingParticipantRepository{}
	mockBuilder := &mocks.MockMessageBuilder{}
	config := ServiceConfig{
		SkipEtagValidation: false,
		LfxURLGenerator:    constants.NewLfxURLGenerator("dev", ""),
	}

	service := NewPastMeetingParticipantService(
		mockMeetingRepo,
		mockPastMeetingRepo,
		mockParticipantRepo,
		mockBuilder,
		config,
	)

	return service, mockMeetingRepo, mockPastMeetingRepo, mockParticipantRepo, mockBuilder
}

func TestPastMeetingParticipantService_ServiceReady(t *testing.T) {
	tests := []struct {
		name          string
		setupService  func() *PastMeetingParticipantService
		expectedReady bool
	}{
		{
			name: "service ready with all dependencies",
			setupService: func() *PastMeetingParticipantService {
				service, _, _, _, _ := setupPastMeetingParticipantServiceForTesting()
				return service
			},
			expectedReady: true,
		},
		{
			name: "service not ready - missing past meeting repository",
			setupService: func() *PastMeetingParticipantService {
				mockMeetingRepo := &mocks.MockMeetingRepository{}
				mockParticipantRepo := &mocks.MockPastMeetingParticipantRepository{}
				mockBuilder := &mocks.MockMessageBuilder{}
				config := ServiceConfig{
					SkipEtagValidation: false,
					LfxURLGenerator:    constants.NewLfxURLGenerator("dev", ""),
				}
				return NewPastMeetingParticipantService(
					mockMeetingRepo,
					nil, // past meeting repository is nil
					mockParticipantRepo,
					mockBuilder,
					config,
				)
			},
			expectedReady: false,
		},
		{
			name: "service not ready - missing participant repository",
			setupService: func() *PastMeetingParticipantService {
				mockMeetingRepo := &mocks.MockMeetingRepository{}
				mockPastMeetingRepo := &mocks.MockPastMeetingRepository{}
				mockBuilder := &mocks.MockMessageBuilder{}
				config := ServiceConfig{
					SkipEtagValidation: false,
					LfxURLGenerator:    constants.NewLfxURLGenerator("dev", ""),
				}
				return NewPastMeetingParticipantService(
					mockMeetingRepo,
					mockPastMeetingRepo,
					nil, // participant repository is nil
					mockBuilder,
					config,
				)
			},
			expectedReady: false,
		},
		{
			name: "service not ready - missing message builder",
			setupService: func() *PastMeetingParticipantService {
				mockMeetingRepo := &mocks.MockMeetingRepository{}
				mockPastMeetingRepo := &mocks.MockPastMeetingRepository{}
				mockParticipantRepo := &mocks.MockPastMeetingParticipantRepository{}
				config := ServiceConfig{
					SkipEtagValidation: false,
					LfxURLGenerator:    constants.NewLfxURLGenerator("dev", ""),
				}
				return NewPastMeetingParticipantService(
					mockMeetingRepo,
					mockPastMeetingRepo,
					mockParticipantRepo,
					nil, // message builder is nil
					config,
				)
			},
			expectedReady: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := tt.setupService()
			result := service.ServiceReady()
			assert.Equal(t, tt.expectedReady, result)
		})
	}
}

func TestPastMeetingParticipantService_GetPastMeetingParticipants(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name            string
		pastMeetingUID  string
		setupMocks      func(*mocks.MockPastMeetingRepository, *mocks.MockPastMeetingParticipantRepository)
		wantErr         bool
		expectedErrType domain.ErrorType
		expectedLen     int
	}{
		{
			name:           "successful get participants",
			pastMeetingUID: "past-meeting-123",
			setupMocks: func(mockPastMeetingRepo *mocks.MockPastMeetingRepository, mockParticipantRepo *mocks.MockPastMeetingParticipantRepository) {
				mockPastMeetingRepo.On("Exists", mock.Anything, "past-meeting-123").Return(true, nil)
				mockParticipantRepo.On("ListByPastMeeting", mock.Anything, "past-meeting-123").Return([]*models.PastMeetingParticipant{
					{
						UID:            "participant-1",
						PastMeetingUID: "past-meeting-123",
						MeetingUID:     "meeting-123",
						Email:          "user1@example.com",
						FirstName:      "John",
						LastName:       "Doe",
						CreatedAt:      &[]time.Time{time.Now()}[0],
					},
					{
						UID:            "participant-2",
						PastMeetingUID: "past-meeting-123",
						MeetingUID:     "meeting-123",
						Email:          "user2@example.com",
						FirstName:      "Jane",
						LastName:       "Smith",
						CreatedAt:      &[]time.Time{time.Now()}[0],
					},
				}, nil)
			},
			wantErr:     false,
			expectedLen: 2,
		},
		{
			name:           "service not ready",
			pastMeetingUID: "past-meeting-123",
			setupMocks: func(mockPastMeetingRepo *mocks.MockPastMeetingRepository, mockParticipantRepo *mocks.MockPastMeetingParticipantRepository) {
			},
			wantErr:         true,
			expectedErrType: domain.ErrorTypeUnavailable,
		},
		{
			name:           "empty past meeting UID",
			pastMeetingUID: "",
			setupMocks: func(mockPastMeetingRepo *mocks.MockPastMeetingRepository, mockParticipantRepo *mocks.MockPastMeetingParticipantRepository) {
			},
			wantErr:         true,
			expectedErrType: domain.ErrorTypeValidation,
		},
		{
			name:           "past meeting not found",
			pastMeetingUID: "past-meeting-123",
			setupMocks: func(mockPastMeetingRepo *mocks.MockPastMeetingRepository, mockParticipantRepo *mocks.MockPastMeetingParticipantRepository) {
				mockPastMeetingRepo.On("Exists", mock.Anything, "past-meeting-123").Return(false, nil)
			},
			wantErr:         true,
			expectedErrType: domain.ErrorTypeNotFound,
		},
		{
			name:           "past meeting exists check error",
			pastMeetingUID: "past-meeting-123",
			setupMocks: func(mockPastMeetingRepo *mocks.MockPastMeetingRepository, mockParticipantRepo *mocks.MockPastMeetingParticipantRepository) {
				mockPastMeetingRepo.On("Exists", mock.Anything, "past-meeting-123").Return(false, domain.NewInternalError("database error"))
			},
			wantErr:         true,
			expectedErrType: domain.ErrorTypeInternal,
		},
		{
			name:           "repository list error",
			pastMeetingUID: "past-meeting-123",
			setupMocks: func(mockPastMeetingRepo *mocks.MockPastMeetingRepository, mockParticipantRepo *mocks.MockPastMeetingParticipantRepository) {
				mockPastMeetingRepo.On("Exists", mock.Anything, "past-meeting-123").Return(true, nil)
				mockParticipantRepo.On("ListByPastMeeting", mock.Anything, "past-meeting-123").Return(nil, domain.NewInternalError("database error"))
			},
			wantErr:         true,
			expectedErrType: domain.ErrorTypeInternal,
		},
		{
			name:           "empty participants list",
			pastMeetingUID: "past-meeting-123",
			setupMocks: func(mockPastMeetingRepo *mocks.MockPastMeetingRepository, mockParticipantRepo *mocks.MockPastMeetingParticipantRepository) {
				mockPastMeetingRepo.On("Exists", mock.Anything, "past-meeting-123").Return(true, nil)
				mockParticipantRepo.On("ListByPastMeeting", mock.Anything, "past-meeting-123").Return([]*models.PastMeetingParticipant{}, nil)
			},
			wantErr:     false,
			expectedLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, _, mockPastMeetingRepo, mockParticipantRepo, _ := setupPastMeetingParticipantServiceForTesting()

			// Set service as not ready for specific test
			if tt.name == "service not ready" {
				// Create a service with nil repository for this test
				service = NewPastMeetingParticipantService(nil, nil, mockParticipantRepo, nil, ServiceConfig{})
			}

			if tt.setupMocks != nil {
				tt.setupMocks(mockPastMeetingRepo, mockParticipantRepo)
			}

			result, err := service.ListPastMeetingParticipants(ctx, tt.pastMeetingUID)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedErrType, domain.GetErrorType(err))
			} else {
				assert.NoError(t, err)
				assert.Len(t, result, tt.expectedLen)
			}

			mockPastMeetingRepo.AssertExpectations(t)
			mockParticipantRepo.AssertExpectations(t)
		})
	}
}

func TestPastMeetingParticipantService_CreatePastMeetingParticipant(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name            string
		participant     *models.PastMeetingParticipant
		setupMocks      func(*mocks.MockPastMeetingRepository, *mocks.MockPastMeetingParticipantRepository, *mocks.MockMessageBuilder)
		wantErr         bool
		expectedErrType domain.ErrorType
		validate        func(t *testing.T, result *models.PastMeetingParticipant)
	}{
		{
			name: "successful creation",
			participant: &models.PastMeetingParticipant{
				PastMeetingUID: "past-meeting-123",
				Email:          "user@example.com",
				FirstName:      "John",
				LastName:       "Doe",
				Username:       "johndoe",
				Host:           false,
				IsInvited:      true,
				IsAttended:     true,
				Sessions: []models.ParticipantSession{
					{
						UID:       "session-1",
						JoinTime:  mustParseTimeForTest("2023-12-01T10:00:00Z"),
						LeaveTime: &[]time.Time{mustParseTimeForTest("2023-12-01T11:00:00Z")}[0],
					},
				},
			},
			setupMocks: func(mockPastMeetingRepo *mocks.MockPastMeetingRepository, mockParticipantRepo *mocks.MockPastMeetingParticipantRepository, mockBuilder *mocks.MockMessageBuilder) {
				// Validate past meeting exists
				mockPastMeetingRepo.On("Exists", mock.Anything, "past-meeting-123").Return(true, nil)

				// Check for existing participant
				mockParticipantRepo.On("GetByPastMeetingAndEmail", mock.Anything, "past-meeting-123", "user@example.com").Return(nil, domain.NewNotFoundError("past meeting participant not found"))

				// Get past meeting to populate MeetingUID
				mockPastMeetingRepo.On("Get", mock.Anything, "past-meeting-123").Return(&models.PastMeeting{
					UID:        "past-meeting-123",
					MeetingUID: "meeting-123",
				}, nil)

				// Create participant
				mockParticipantRepo.On("Create", mock.Anything, mock.MatchedBy(func(p *models.PastMeetingParticipant) bool {
					return p.PastMeetingUID == "past-meeting-123" &&
						p.Email == "user@example.com" &&
						p.MeetingUID == "meeting-123" &&
						p.UID != "" && // UUID should be generated
						len(p.Sessions) == 1
				})).Return(nil)

				// Send messages
				mockBuilder.On("SendIndexPastMeetingParticipant", mock.Anything, models.ActionCreated, mock.Anything, mock.Anything).Return(nil)
				mockBuilder.On("SendPutPastMeetingParticipantAccess", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			},
			wantErr: false,
			validate: func(t *testing.T, result *models.PastMeetingParticipant) {
				assert.NotEmpty(t, result.UID)
				assert.Equal(t, "meeting-123", result.MeetingUID)
				assert.Len(t, result.Sessions, 1)
			},
		},
		{
			name:        "service not ready",
			participant: &models.PastMeetingParticipant{},
			setupMocks: func(mockPastMeetingRepo *mocks.MockPastMeetingRepository, mockParticipantRepo *mocks.MockPastMeetingParticipantRepository, mockBuilder *mocks.MockMessageBuilder) {
			},
			wantErr:         true,
			expectedErrType: domain.ErrorTypeUnavailable,
		},
		{
			name:        "nil participant",
			participant: nil,
			setupMocks: func(mockPastMeetingRepo *mocks.MockPastMeetingRepository, mockParticipantRepo *mocks.MockPastMeetingParticipantRepository, mockBuilder *mocks.MockMessageBuilder) {
			},
			wantErr:         true,
			expectedErrType: domain.ErrorTypeValidation,
		},
		{
			name: "past meeting not found",
			participant: &models.PastMeetingParticipant{
				PastMeetingUID: "past-meeting-123",
				Email:          "user@example.com",
			},
			setupMocks: func(mockPastMeetingRepo *mocks.MockPastMeetingRepository, mockParticipantRepo *mocks.MockPastMeetingParticipantRepository, mockBuilder *mocks.MockMessageBuilder) {
				mockPastMeetingRepo.On("Exists", mock.Anything, "past-meeting-123").Return(false, nil)
			},
			wantErr:         true,
			expectedErrType: domain.ErrorTypeNotFound,
		},
		{
			name: "participant already exists",
			participant: &models.PastMeetingParticipant{
				PastMeetingUID: "past-meeting-123",
				Email:          "user@example.com",
			},
			setupMocks: func(mockPastMeetingRepo *mocks.MockPastMeetingRepository, mockParticipantRepo *mocks.MockPastMeetingParticipantRepository, mockBuilder *mocks.MockMessageBuilder) {
				mockPastMeetingRepo.On("Exists", mock.Anything, "past-meeting-123").Return(true, nil)
				mockParticipantRepo.On("GetByPastMeetingAndEmail", mock.Anything, "past-meeting-123", "user@example.com").Return(&models.PastMeetingParticipant{
					UID:            "existing-participant",
					PastMeetingUID: "past-meeting-123",
					Email:          "user@example.com",
				}, nil)
			},
			wantErr:         true,
			expectedErrType: domain.ErrorTypeConflict,
		},
		{
			name: "repository create error",
			participant: &models.PastMeetingParticipant{
				PastMeetingUID: "past-meeting-123",
				Email:          "user@example.com",
			},
			setupMocks: func(mockPastMeetingRepo *mocks.MockPastMeetingRepository, mockParticipantRepo *mocks.MockPastMeetingParticipantRepository, mockBuilder *mocks.MockMessageBuilder) {
				mockPastMeetingRepo.On("Exists", mock.Anything, "past-meeting-123").Return(true, nil)
				mockParticipantRepo.On("GetByPastMeetingAndEmail", mock.Anything, "past-meeting-123", "user@example.com").Return(nil, domain.NewNotFoundError("past meeting participant not found"))
				mockPastMeetingRepo.On("Get", mock.Anything, "past-meeting-123").Return(&models.PastMeeting{
					UID:        "past-meeting-123",
					MeetingUID: "meeting-123",
				}, nil)
				mockParticipantRepo.On("Create", mock.Anything, mock.Anything).Return(domain.NewInternalError("database error"))
			},
			wantErr:         true,
			expectedErrType: domain.ErrorTypeInternal,
		},
		{
			name: "messaging failure doesn't fail operation",
			participant: &models.PastMeetingParticipant{
				PastMeetingUID: "past-meeting-123",
				Email:          "user@example.com",
			},
			setupMocks: func(mockPastMeetingRepo *mocks.MockPastMeetingRepository, mockParticipantRepo *mocks.MockPastMeetingParticipantRepository, mockBuilder *mocks.MockMessageBuilder) {
				mockPastMeetingRepo.On("Exists", mock.Anything, "past-meeting-123").Return(true, nil)
				mockParticipantRepo.On("GetByPastMeetingAndEmail", mock.Anything, "past-meeting-123", "user@example.com").Return(nil, domain.NewNotFoundError("past meeting participant not found"))
				mockPastMeetingRepo.On("Get", mock.Anything, "past-meeting-123").Return(&models.PastMeeting{
					UID:        "past-meeting-123",
					MeetingUID: "meeting-123",
				}, nil)
				mockParticipantRepo.On("Create", mock.Anything, mock.Anything).Return(nil)

				// Messaging fails but operation continues
				mockBuilder.On("SendIndexPastMeetingParticipant", mock.Anything, models.ActionCreated, mock.Anything, mock.Anything).Maybe().Return(errors.New("messaging error"))
				mockBuilder.On("SendPutPastMeetingParticipantAccess", mock.Anything, mock.Anything, mock.Anything).Maybe().Return(errors.New("messaging error"))
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, _, mockPastMeetingRepo, mockParticipantRepo, mockBuilder := setupPastMeetingParticipantServiceForTesting()

			// Set service as not ready for specific test
			if tt.name == "service not ready" {
				// Create a service with nil participant repository for this test
				service = NewPastMeetingParticipantService(nil, mockPastMeetingRepo, nil, mockBuilder, ServiceConfig{})
			}

			if tt.setupMocks != nil {
				tt.setupMocks(mockPastMeetingRepo, mockParticipantRepo, mockBuilder)
			}

			result, err := service.CreatePastMeetingParticipant(ctx, tt.participant)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedErrType, domain.GetErrorType(err))
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				if tt.validate != nil {
					tt.validate(t, result)
				}
			}

			mockPastMeetingRepo.AssertExpectations(t)
			mockParticipantRepo.AssertExpectations(t)
			mockBuilder.AssertExpectations(t)
		})
	}
}

func TestPastMeetingParticipantService_GetPastMeetingParticipant(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name             string
		pastMeetingUID   string
		participantUID   string
		setupMocks       func(*mocks.MockPastMeetingRepository, *mocks.MockPastMeetingParticipantRepository)
		wantErr          bool
		expectedErrType  domain.ErrorType
		expectedRevision string
	}{
		{
			name:           "successful get",
			pastMeetingUID: "past-meeting-123",
			participantUID: "participant-123",
			setupMocks: func(mockPastMeetingRepo *mocks.MockPastMeetingRepository, mockParticipantRepo *mocks.MockPastMeetingParticipantRepository) {
				mockPastMeetingRepo.On("Exists", mock.Anything, "past-meeting-123").Return(true, nil)
				mockParticipantRepo.On("GetWithRevision", mock.Anything, "participant-123").Return(&models.PastMeetingParticipant{
					UID:            "participant-123",
					PastMeetingUID: "past-meeting-123",
					MeetingUID:     "meeting-123",
					Email:          "user@example.com",
					FirstName:      "John",
					LastName:       "Doe",
					CreatedAt:      &[]time.Time{time.Now()}[0],
				}, uint64(42), nil)
			},
			wantErr:          false,
			expectedRevision: "42",
		},
		{
			name:           "service not ready",
			pastMeetingUID: "past-meeting-123",
			participantUID: "participant-123",
			setupMocks: func(mockPastMeetingRepo *mocks.MockPastMeetingRepository, mockParticipantRepo *mocks.MockPastMeetingParticipantRepository) {
			},
			wantErr:         true,
			expectedErrType: domain.ErrorTypeUnavailable,
		},
		{
			name:           "empty UIDs",
			pastMeetingUID: "",
			participantUID: "",
			setupMocks: func(mockPastMeetingRepo *mocks.MockPastMeetingRepository, mockParticipantRepo *mocks.MockPastMeetingParticipantRepository) {
			},
			wantErr:         true,
			expectedErrType: domain.ErrorTypeValidation,
		},
		{
			name:           "past meeting not found",
			pastMeetingUID: "past-meeting-123",
			participantUID: "participant-123",
			setupMocks: func(mockPastMeetingRepo *mocks.MockPastMeetingRepository, mockParticipantRepo *mocks.MockPastMeetingParticipantRepository) {
				mockPastMeetingRepo.On("Exists", mock.Anything, "past-meeting-123").Return(false, nil)
			},
			wantErr:         true,
			expectedErrType: domain.ErrorTypeNotFound,
		},
		{
			name:           "participant not found",
			pastMeetingUID: "past-meeting-123",
			participantUID: "participant-123",
			setupMocks: func(mockPastMeetingRepo *mocks.MockPastMeetingRepository, mockParticipantRepo *mocks.MockPastMeetingParticipantRepository) {
				mockPastMeetingRepo.On("Exists", mock.Anything, "past-meeting-123").Return(true, nil)
				mockParticipantRepo.On("GetWithRevision", mock.Anything, "participant-123").Return(nil, uint64(0), domain.NewNotFoundError("past meeting participant not found"))
			},
			wantErr:         true,
			expectedErrType: domain.ErrorTypeNotFound,
		},
		{
			name:           "participant belongs to different past meeting",
			pastMeetingUID: "past-meeting-123",
			participantUID: "participant-123",
			setupMocks: func(mockPastMeetingRepo *mocks.MockPastMeetingRepository, mockParticipantRepo *mocks.MockPastMeetingParticipantRepository) {
				mockPastMeetingRepo.On("Exists", mock.Anything, "past-meeting-123").Return(true, nil)
				mockParticipantRepo.On("GetWithRevision", mock.Anything, "participant-123").Return(&models.PastMeetingParticipant{
					UID:            "participant-123",
					PastMeetingUID: "different-past-meeting",
					Email:          "user@example.com",
				}, uint64(42), nil)
			},
			wantErr:         true,
			expectedErrType: domain.ErrorTypeNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, _, mockPastMeetingRepo, mockParticipantRepo, mockBuilder := setupPastMeetingParticipantServiceForTesting()

			// Set service as not ready for specific test
			if tt.name == "service not ready" {
				// Create a service with nil participant repository for this test
				service = NewPastMeetingParticipantService(nil, mockPastMeetingRepo, nil, mockBuilder, ServiceConfig{})
			}

			if tt.setupMocks != nil {
				tt.setupMocks(mockPastMeetingRepo, mockParticipantRepo)
			}

			result, revision, err := service.GetPastMeetingParticipant(ctx, tt.pastMeetingUID, tt.participantUID)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedErrType, domain.GetErrorType(err))
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.expectedRevision, revision)
			}

			mockPastMeetingRepo.AssertExpectations(t)
			mockParticipantRepo.AssertExpectations(t)
		})
	}
}

func TestPastMeetingParticipantService_UpdatePastMeetingParticipant(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name            string
		participant     *models.PastMeetingParticipant
		revision        uint64
		setupMocks      func(*mocks.MockPastMeetingRepository, *mocks.MockPastMeetingParticipantRepository, *mocks.MockMessageBuilder)
		wantErr         bool
		expectedErrType domain.ErrorType
	}{
		{
			name: "successful update",
			participant: &models.PastMeetingParticipant{
				UID:            "participant-123",
				PastMeetingUID: "past-meeting-123",
				Email:          "updated@example.com",
				FirstName:      "Updated",
				LastName:       "User",
			},
			revision: 42,
			setupMocks: func(mockPastMeetingRepo *mocks.MockPastMeetingRepository, mockParticipantRepo *mocks.MockPastMeetingParticipantRepository, mockBuilder *mocks.MockMessageBuilder) {
				// Get past meeting for artifact visibility
				mockPastMeetingRepo.On("Get", mock.Anything, "past-meeting-123").Return(&models.PastMeeting{
					UID:                "past-meeting-123",
					ArtifactVisibility: "meeting_participants",
				}, nil)

				// Get existing participant
				mockParticipantRepo.On("Get", mock.Anything, "participant-123").Return(&models.PastMeetingParticipant{
					UID:            "participant-123",
					PastMeetingUID: "past-meeting-123",
					MeetingUID:     "meeting-123",
					Email:          "old@example.com",
					CreatedAt:      &[]time.Time{time.Now()}[0],
				}, nil)

				// Validate past meeting exists
				mockPastMeetingRepo.On("Exists", mock.Anything, "past-meeting-123").Return(true, nil)

				// Check for duplicate email
				mockParticipantRepo.On("GetByPastMeetingAndEmail", mock.Anything, "past-meeting-123", "updated@example.com").Return(nil, domain.NewNotFoundError("past meeting participant not found"))

				// Update participant
				mockParticipantRepo.On("Update", mock.Anything, mock.MatchedBy(func(p *models.PastMeetingParticipant) bool {
					return p.UID == "participant-123" &&
						p.Email == "updated@example.com" &&
						p.MeetingUID == "meeting-123" && // Should be preserved
						p.CreatedAt != nil // Should be preserved
				}), uint64(42)).Return(nil)

				// Send messages
				mockBuilder.On("SendIndexPastMeetingParticipant", mock.Anything, models.ActionUpdated, mock.Anything, mock.Anything).Return(nil)
				mockBuilder.On("SendPutPastMeetingParticipantAccess", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "service not ready",
			participant: &models.PastMeetingParticipant{
				UID: "participant-123",
			},
			revision: 42,
			setupMocks: func(mockPastMeetingRepo *mocks.MockPastMeetingRepository, mockParticipantRepo *mocks.MockPastMeetingParticipantRepository, mockBuilder *mocks.MockMessageBuilder) {
			},
			wantErr:         true,
			expectedErrType: domain.ErrorTypeUnavailable,
		},
		{
			name:        "nil participant",
			participant: nil,
			revision:    42,
			setupMocks: func(mockPastMeetingRepo *mocks.MockPastMeetingRepository, mockParticipantRepo *mocks.MockPastMeetingParticipantRepository, mockBuilder *mocks.MockMessageBuilder) {
			},
			wantErr:         true,
			expectedErrType: domain.ErrorTypeValidation,
		},
		{
			name: "participant not found",
			participant: &models.PastMeetingParticipant{
				UID:            "participant-123",
				PastMeetingUID: "past-meeting-123",
			},
			revision: 42,
			setupMocks: func(mockPastMeetingRepo *mocks.MockPastMeetingRepository, mockParticipantRepo *mocks.MockPastMeetingParticipantRepository, mockBuilder *mocks.MockMessageBuilder) {
				// Mock past meeting get call (which happens before participant check)
				mockPastMeetingRepo.On("Get", mock.Anything, "past-meeting-123").Return(&models.PastMeeting{
					UID:                "past-meeting-123",
					ArtifactVisibility: "meeting_participants",
				}, nil)
				mockParticipantRepo.On("Get", mock.Anything, "participant-123").Return(nil, domain.NewNotFoundError("past meeting participant not found"))
			},
			wantErr:         true,
			expectedErrType: domain.ErrorTypeNotFound,
		},
		{
			name: "revision mismatch",
			participant: &models.PastMeetingParticipant{
				UID:            "participant-123",
				PastMeetingUID: "past-meeting-123",
				Email:          "updated@example.com",
			},
			revision: 42,
			setupMocks: func(mockPastMeetingRepo *mocks.MockPastMeetingRepository, mockParticipantRepo *mocks.MockPastMeetingParticipantRepository, mockBuilder *mocks.MockMessageBuilder) {
				// Mock past meeting get call (which happens before participant check)
				mockPastMeetingRepo.On("Get", mock.Anything, "past-meeting-123").Return(&models.PastMeeting{
					UID:                "past-meeting-123",
					ArtifactVisibility: "meeting_participants",
				}, nil)
				mockParticipantRepo.On("Get", mock.Anything, "participant-123").Return(&models.PastMeetingParticipant{
					UID:            "participant-123",
					PastMeetingUID: "past-meeting-123",
					MeetingUID:     "meeting-123",
					Email:          "old@example.com",
				}, nil)
				mockPastMeetingRepo.On("Exists", mock.Anything, "past-meeting-123").Return(true, nil)
				mockParticipantRepo.On("GetByPastMeetingAndEmail", mock.Anything, "past-meeting-123", "updated@example.com").Return(nil, domain.NewNotFoundError("past meeting participant not found"))
				mockParticipantRepo.On("Update", mock.Anything, mock.Anything, uint64(42)).Return(domain.NewConflictError("revision mismatch"))
			},
			wantErr:         true,
			expectedErrType: domain.ErrorTypeConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, _, mockPastMeetingRepo, mockParticipantRepo, mockBuilder := setupPastMeetingParticipantServiceForTesting()

			// Set service as not ready for specific test
			if tt.name == "service not ready" {
				// Create a service with nil participant repository for this test
				service = NewPastMeetingParticipantService(nil, mockPastMeetingRepo, nil, mockBuilder, ServiceConfig{})
			}

			if tt.setupMocks != nil {
				tt.setupMocks(mockPastMeetingRepo, mockParticipantRepo, mockBuilder)
			}

			result, err := service.UpdatePastMeetingParticipant(ctx, tt.participant, tt.revision)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedErrType, domain.GetErrorType(err))
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}

			mockPastMeetingRepo.AssertExpectations(t)
			mockParticipantRepo.AssertExpectations(t)
			mockBuilder.AssertExpectations(t)
		})
	}
}

func TestPastMeetingParticipantService_DeletePastMeetingParticipant(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name            string
		pastMeetingUID  string
		participantUID  string
		revision        uint64
		setupMocks      func(*mocks.MockPastMeetingRepository, *mocks.MockPastMeetingParticipantRepository, *mocks.MockMessageBuilder)
		wantErr         bool
		expectedErrType domain.ErrorType
	}{
		{
			name:           "successful delete",
			pastMeetingUID: "past-meeting-123",
			participantUID: "participant-123",
			revision:       42,
			setupMocks: func(mockPastMeetingRepo *mocks.MockPastMeetingRepository, mockParticipantRepo *mocks.MockPastMeetingParticipantRepository, mockBuilder *mocks.MockMessageBuilder) {
				// Mock past meeting get call (which happens before participant check)
				mockPastMeetingRepo.On("Get", mock.Anything, "past-meeting-123").Return(&models.PastMeeting{
					UID:                "past-meeting-123",
					ArtifactVisibility: "meeting_participants",
				}, nil)
				mockParticipantRepo.On("Get", mock.Anything, "participant-123").Return(&models.PastMeetingParticipant{
					UID:            "participant-123",
					PastMeetingUID: "past-meeting-123",
					Username:       "johndoe",
					Host:           false,
				}, nil)
				mockParticipantRepo.On("Delete", mock.Anything, "participant-123", uint64(42)).Return(nil)

				// Send messages
				mockBuilder.On("SendDeleteIndexPastMeetingParticipant", mock.Anything, "participant-123", mock.Anything).Return(nil)
				mockBuilder.On("SendRemovePastMeetingParticipantAccess", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			},
			wantErr: false,
		},
		{
			name:           "service not ready",
			pastMeetingUID: "past-meeting-123",
			participantUID: "participant-123",
			revision:       42,
			setupMocks: func(mockPastMeetingRepo *mocks.MockPastMeetingRepository, mockParticipantRepo *mocks.MockPastMeetingParticipantRepository, mockBuilder *mocks.MockMessageBuilder) {
			},
			wantErr:         true,
			expectedErrType: domain.ErrorTypeUnavailable,
		},
		{
			name:           "empty UIDs",
			pastMeetingUID: "",
			participantUID: "",
			revision:       42,
			setupMocks: func(mockPastMeetingRepo *mocks.MockPastMeetingRepository, mockParticipantRepo *mocks.MockPastMeetingParticipantRepository, mockBuilder *mocks.MockMessageBuilder) {
			},
			wantErr:         true,
			expectedErrType: domain.ErrorTypeValidation,
		},
		{
			name:           "past meeting not found",
			pastMeetingUID: "past-meeting-123",
			participantUID: "participant-123",
			revision:       42,
			setupMocks: func(mockPastMeetingRepo *mocks.MockPastMeetingRepository, mockParticipantRepo *mocks.MockPastMeetingParticipantRepository, mockBuilder *mocks.MockMessageBuilder) {
				mockPastMeetingRepo.On("Get", mock.Anything, "past-meeting-123").Return(nil, domain.NewNotFoundError("past meeting not found"))
			},
			wantErr:         true,
			expectedErrType: domain.ErrorTypeNotFound,
		},
		{
			name:           "participant not found",
			pastMeetingUID: "past-meeting-123",
			participantUID: "participant-123",
			revision:       42,
			setupMocks: func(mockPastMeetingRepo *mocks.MockPastMeetingRepository, mockParticipantRepo *mocks.MockPastMeetingParticipantRepository, mockBuilder *mocks.MockMessageBuilder) {
				mockPastMeetingRepo.On("Get", mock.Anything, "past-meeting-123").Return(&models.PastMeeting{
					UID:                "past-meeting-123",
					ArtifactVisibility: "meeting_participants",
				}, nil)
				mockParticipantRepo.On("Get", mock.Anything, "participant-123").Return(nil, domain.NewNotFoundError("past meeting participant not found"))
			},
			wantErr:         true,
			expectedErrType: domain.ErrorTypeNotFound,
		},
		{
			name:           "participant belongs to different past meeting",
			pastMeetingUID: "past-meeting-123",
			participantUID: "participant-123",
			revision:       42,
			setupMocks: func(mockPastMeetingRepo *mocks.MockPastMeetingRepository, mockParticipantRepo *mocks.MockPastMeetingParticipantRepository, mockBuilder *mocks.MockMessageBuilder) {
				mockPastMeetingRepo.On("Get", mock.Anything, "past-meeting-123").Return(&models.PastMeeting{
					UID:                "past-meeting-123",
					ArtifactVisibility: "meeting_participants",
				}, nil)
				mockParticipantRepo.On("Get", mock.Anything, "participant-123").Return(&models.PastMeetingParticipant{
					UID:            "participant-123",
					PastMeetingUID: "different-past-meeting",
				}, nil)
			},
			wantErr:         true,
			expectedErrType: domain.ErrorTypeNotFound,
		},
		{
			name:           "revision mismatch",
			pastMeetingUID: "past-meeting-123",
			participantUID: "participant-123",
			revision:       42,
			setupMocks: func(mockPastMeetingRepo *mocks.MockPastMeetingRepository, mockParticipantRepo *mocks.MockPastMeetingParticipantRepository, mockBuilder *mocks.MockMessageBuilder) {
				mockPastMeetingRepo.On("Get", mock.Anything, "past-meeting-123").Return(&models.PastMeeting{
					UID:                "past-meeting-123",
					ArtifactVisibility: "meeting_participants",
				}, nil)
				mockParticipantRepo.On("Get", mock.Anything, "participant-123").Return(&models.PastMeetingParticipant{
					UID:            "participant-123",
					PastMeetingUID: "past-meeting-123",
				}, nil)
				mockParticipantRepo.On("Delete", mock.Anything, "participant-123", uint64(42)).Return(domain.NewConflictError("revision mismatch"))
			},
			wantErr:         true,
			expectedErrType: domain.ErrorTypeConflict,
		},
		{
			name:           "messaging failure doesn't fail operation",
			pastMeetingUID: "past-meeting-123",
			participantUID: "participant-123",
			revision:       42,
			setupMocks: func(mockPastMeetingRepo *mocks.MockPastMeetingRepository, mockParticipantRepo *mocks.MockPastMeetingParticipantRepository, mockBuilder *mocks.MockMessageBuilder) {
				mockPastMeetingRepo.On("Get", mock.Anything, "past-meeting-123").Return(&models.PastMeeting{
					UID:                "past-meeting-123",
					ArtifactVisibility: "meeting_participants",
				}, nil)
				mockParticipantRepo.On("Get", mock.Anything, "participant-123").Return(&models.PastMeetingParticipant{
					UID:            "participant-123",
					PastMeetingUID: "past-meeting-123",
					Username:       "johndoe",
				}, nil)
				mockParticipantRepo.On("Delete", mock.Anything, "participant-123", uint64(42)).Return(nil)

				// Messaging fails but operation continues
				mockBuilder.On("SendDeleteIndexPastMeetingParticipant", mock.Anything, "participant-123", mock.Anything).Maybe().Return(errors.New("messaging error"))
				mockBuilder.On("SendRemovePastMeetingParticipantAccess", mock.Anything, mock.Anything, mock.Anything).Maybe().Return(errors.New("messaging error"))
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, _, mockPastMeetingRepo, mockParticipantRepo, mockBuilder := setupPastMeetingParticipantServiceForTesting()

			// Set service as not ready for specific test
			if tt.name == "service not ready" {
				// Create a service with nil participant repository for this test
				service = NewPastMeetingParticipantService(nil, mockPastMeetingRepo, nil, mockBuilder, ServiceConfig{})
			}

			if tt.setupMocks != nil {
				tt.setupMocks(mockPastMeetingRepo, mockParticipantRepo, mockBuilder)
			}

			err := service.DeletePastMeetingParticipant(ctx, tt.pastMeetingUID, tt.participantUID, tt.revision)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedErrType, domain.GetErrorType(err))
			} else {
				assert.NoError(t, err)
			}

			mockPastMeetingRepo.AssertExpectations(t)
			mockParticipantRepo.AssertExpectations(t)
			mockBuilder.AssertExpectations(t)
		})
	}
}
