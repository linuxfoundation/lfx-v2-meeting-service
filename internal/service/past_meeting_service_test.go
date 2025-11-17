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

// mustParseTime is a helper function for tests
func mustParseTimeForTest(timeStr string) time.Time {
	t, err := time.Parse(time.RFC3339, timeStr)
	if err != nil {
		panic(err)
	}
	return t
}

// setupPastMeetingServiceForTesting creates a PastMeetingService with all mock dependencies for testing
func setupPastMeetingServiceForTesting() (*PastMeetingService, *mocks.MockMeetingRepository, *mocks.MockPastMeetingRepository, *mocks.MockMeetingAttachmentRepository, *mocks.MockPastMeetingAttachmentRepository, *mocks.MockMessageBuilder) {
	mockMeetingRepo := new(mocks.MockMeetingRepository)
	mockPastMeetingRepo := new(mocks.MockPastMeetingRepository)
	mockMeetingAttachmentRepo := new(mocks.MockMeetingAttachmentRepository)
	mockPastMeetingAttachmentRepo := new(mocks.MockPastMeetingAttachmentRepository)
	mockBuilder := new(mocks.MockMessageBuilder)

	config := ServiceConfig{
		SkipEtagValidation: false,
		LfxURLGenerator:    constants.NewLfxURLGenerator("dev", ""),
	}

	service := NewPastMeetingService(
		mockMeetingRepo,
		mockPastMeetingRepo,
		mockMeetingAttachmentRepo,
		mockPastMeetingAttachmentRepo,
		mockBuilder,
		config,
	)

	return service, mockMeetingRepo, mockPastMeetingRepo, mockMeetingAttachmentRepo, mockPastMeetingAttachmentRepo, mockBuilder
}

func TestPastMeetingService_ServiceReady(t *testing.T) {
	tests := []struct {
		name     string
		setup    func() *PastMeetingService
		expected bool
	}{
		{
			name: "service ready with all dependencies",
			setup: func() *PastMeetingService {
				service, _, _, _, _, _ := setupPastMeetingServiceForTesting()
				return service
			},
			expected: true,
		},
		{
			name: "service not ready - missing past meeting repository",
			setup: func() *PastMeetingService {
				config := ServiceConfig{SkipEtagValidation: false}
				mockMeetingRepo := new(mocks.MockMeetingRepository)
				mockMeetingAttachmentRepo := new(mocks.MockMeetingAttachmentRepository)
				mockPastMeetingAttachmentRepo := new(mocks.MockPastMeetingAttachmentRepository)
				mockBuilder := new(mocks.MockMessageBuilder)
				return NewPastMeetingService(
					mockMeetingRepo,
					nil, // past meeting repository is nil
					mockMeetingAttachmentRepo,
					mockPastMeetingAttachmentRepo,
					mockBuilder,
					config,
				)
			},
			expected: false,
		},
		{
			name: "service not ready - missing meeting repository",
			setup: func() *PastMeetingService {
				config := ServiceConfig{SkipEtagValidation: false}
				mockPastMeetingRepo := new(mocks.MockPastMeetingRepository)
				mockMeetingAttachmentRepo := new(mocks.MockMeetingAttachmentRepository)
				mockPastMeetingAttachmentRepo := new(mocks.MockPastMeetingAttachmentRepository)
				mockBuilder := new(mocks.MockMessageBuilder)
				return NewPastMeetingService(
					nil, // meeting repository is nil
					mockPastMeetingRepo,
					mockMeetingAttachmentRepo,
					mockPastMeetingAttachmentRepo,
					mockBuilder,
					config,
				)
			},
			expected: false,
		},
		{
			name: "service not ready - missing message builder",
			setup: func() *PastMeetingService {
				config := ServiceConfig{SkipEtagValidation: false}
				mockMeetingRepo := new(mocks.MockMeetingRepository)
				mockPastMeetingRepo := new(mocks.MockPastMeetingRepository)
				mockMeetingAttachmentRepo := new(mocks.MockMeetingAttachmentRepository)
				mockPastMeetingAttachmentRepo := new(mocks.MockPastMeetingAttachmentRepository)
				return NewPastMeetingService(
					mockMeetingRepo,
					mockPastMeetingRepo,
					mockMeetingAttachmentRepo,
					mockPastMeetingAttachmentRepo,
					nil, // message builder is nil
					config,
				)
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := tt.setup()
			assert.Equal(t, tt.expected, service.ServiceReady())
		})
	}
}

func TestPastMeetingService_CreatePastMeeting(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	scheduledStartTime := now.Add(-2 * time.Hour).Format(time.RFC3339)
	scheduledEndTime := now.Add(-time.Hour).Format(time.RFC3339)

	tests := []struct {
		name            string
		payload         *models.PastMeeting
		setupMocks      func(*mocks.MockMeetingRepository, *mocks.MockPastMeetingRepository, *mocks.MockMeetingAttachmentRepository, *mocks.MockPastMeetingAttachmentRepository, *mocks.MockMessageBuilder)
		wantErr         bool
		expectedErrType domain.ErrorType
		validate        func(*testing.T, *models.PastMeeting)
	}{
		{
			name: "successful creation",
			payload: &models.PastMeeting{
				MeetingUID:         "meeting-123",
				ProjectUID:         "project-123",
				ScheduledStartTime: mustParseTimeForTest(scheduledStartTime),
				ScheduledEndTime:   mustParseTimeForTest(scheduledEndTime),
				Title:              "Test Past Meeting",
				Description:        "Test Description",
				Platform:           models.PlatformZoom,
				Visibility:         models.VisibilityPublic,
				Committees: []models.Committee{
					{
						UID:                   "committee-1",
						AllowedVotingStatuses: []string{"member"},
					},
				},
				ZoomConfig: &models.ZoomConfig{
					MeetingID:                "123456789",
					Passcode:                 "pass123",
					AICompanionEnabled:       true,
					AISummaryRequireApproval: false,
				},
				Sessions: []models.Session{
					{
						UID:       "session-1",
						StartTime: mustParseTimeForTest(scheduledStartTime),
						EndTime:   &[]time.Time{mustParseTimeForTest(scheduledEndTime)}[0],
					},
				},
			},
			setupMocks: func(mockMeetingRepo *mocks.MockMeetingRepository, mockPastMeetingRepo *mocks.MockPastMeetingRepository, mockMeetingAttachmentRepo *mocks.MockMeetingAttachmentRepository, mockPastMeetingAttachmentRepo *mocks.MockPastMeetingAttachmentRepository, mockBuilder *mocks.MockMessageBuilder) {
				// Check if meeting exists
				mockMeetingRepo.On("Exists", mock.Anything, "meeting-123").Return(true, nil)

				// Create past meeting
				mockPastMeetingRepo.On("Create", mock.Anything, mock.MatchedBy(func(pm *models.PastMeeting) bool {
					return pm.MeetingUID == "meeting-123" &&
						pm.ProjectUID == "project-123" &&
						pm.Title == "Test Past Meeting" &&
						pm.Description == "Test Description" &&
						pm.Platform == models.PlatformZoom &&
						pm.Visibility == models.VisibilityPublic &&
						len(pm.Committees) == 1 &&
						pm.ZoomConfig != nil &&
						len(pm.Sessions) == 1
				})).Return(nil)

				// List meeting attachments (returns empty list)
				mockMeetingAttachmentRepo.On("ListByMeeting", mock.Anything, "meeting-123").Return([]*models.MeetingAttachment{}, nil)

				// Send indexer and access messages
				mockBuilder.On("SendIndexPastMeeting", mock.Anything, models.ActionCreated, mock.Anything, mock.Anything).Return(nil)
				mockBuilder.On("SendUpdateAccessPastMeeting", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			},
			wantErr: false,
			validate: func(t *testing.T, pm *models.PastMeeting) {
				assert.NotEmpty(t, pm.UID)
				assert.Equal(t, "meeting-123", pm.MeetingUID)
				assert.Equal(t, "project-123", pm.ProjectUID)
				assert.Equal(t, "Test Past Meeting", pm.Title)
				assert.Equal(t, "Test Description", pm.Description)
				assert.Equal(t, models.PlatformZoom, pm.Platform)
				assert.Equal(t, models.VisibilityPublic, pm.Visibility)
				assert.Len(t, pm.Committees, 1)
				assert.NotNil(t, pm.ZoomConfig)
				assert.Len(t, pm.Sessions, 1)
			},
		},
		{
			name: "service not ready",
			payload: &models.PastMeeting{
				MeetingUID:         "meeting-123",
				ProjectUID:         "project-123",
				ScheduledStartTime: mustParseTimeForTest(scheduledStartTime),
				ScheduledEndTime:   mustParseTimeForTest(scheduledEndTime),
				Title:              "Test Past Meeting",
				Description:        "Test Description",
				Platform:           models.PlatformZoom,
			},
			setupMocks: func(mockMeetingRepo *mocks.MockMeetingRepository, mockPastMeetingRepo *mocks.MockPastMeetingRepository, mockMeetingAttachmentRepo *mocks.MockMeetingAttachmentRepository, mockPastMeetingAttachmentRepo *mocks.MockPastMeetingAttachmentRepository, mockBuilder *mocks.MockMessageBuilder) {
				// List meeting attachments (returns empty list by default)
				mockMeetingAttachmentRepo.On("ListByMeeting", mock.Anything, mock.Anything).Return([]*models.MeetingAttachment{}, nil).Maybe()
				// Make service not ready by not setting up mocks
			},
			wantErr:         true,
			expectedErrType: domain.ErrorTypeUnavailable,
		},
		{
			name:    "nil payload",
			payload: nil,
			setupMocks: func(*mocks.MockMeetingRepository, *mocks.MockPastMeetingRepository, *mocks.MockMeetingAttachmentRepository, *mocks.MockPastMeetingAttachmentRepository, *mocks.MockMessageBuilder) {
			},
			wantErr:         true,
			expectedErrType: domain.ErrorTypeValidation,
		},
		{
			name: "missing required fields",
			payload: &models.PastMeeting{
				MeetingUID: "",
				ProjectUID: "project-123",
			},
			setupMocks: func(*mocks.MockMeetingRepository, *mocks.MockPastMeetingRepository, *mocks.MockMeetingAttachmentRepository, *mocks.MockPastMeetingAttachmentRepository, *mocks.MockMessageBuilder) {
			},
			wantErr:         true,
			expectedErrType: domain.ErrorTypeValidation,
		},
		{
			name: "end time before start time",
			payload: &models.PastMeeting{
				MeetingUID:         "meeting-123",
				ProjectUID:         "project-123",
				ScheduledStartTime: mustParseTimeForTest(scheduledEndTime), // Swapped
				ScheduledEndTime:   mustParseTimeForTest(scheduledStartTime),
				Title:              "Test Past Meeting",
				Description:        "Test Description",
				Platform:           models.PlatformZoom,
			},
			setupMocks: func(*mocks.MockMeetingRepository, *mocks.MockPastMeetingRepository, *mocks.MockMeetingAttachmentRepository, *mocks.MockPastMeetingAttachmentRepository, *mocks.MockMessageBuilder) {
			},
			wantErr:         true,
			expectedErrType: domain.ErrorTypeValidation,
		},
		{
			name: "repository create error",
			payload: &models.PastMeeting{
				MeetingUID:         "meeting-123",
				ProjectUID:         "project-123",
				ScheduledStartTime: mustParseTimeForTest(scheduledStartTime),
				ScheduledEndTime:   mustParseTimeForTest(scheduledEndTime),
				Title:              "Test Past Meeting",
				Description:        "Test Description",
				Platform:           models.PlatformZoom,
			},
			setupMocks: func(mockMeetingRepo *mocks.MockMeetingRepository, mockPastMeetingRepo *mocks.MockPastMeetingRepository, mockMeetingAttachmentRepo *mocks.MockMeetingAttachmentRepository, mockPastMeetingAttachmentRepo *mocks.MockPastMeetingAttachmentRepository, mockBuilder *mocks.MockMessageBuilder) {
				// List meeting attachments (returns empty list by default)
				mockMeetingAttachmentRepo.On("ListByMeeting", mock.Anything, mock.Anything).Return([]*models.MeetingAttachment{}, nil).Maybe()
				mockMeetingRepo.On("Exists", mock.Anything, "meeting-123").Return(true, nil)
				mockPastMeetingRepo.On("Create", mock.Anything, mock.Anything).Return(domain.NewInternalError("database error"))
			},
			wantErr:         true,
			expectedErrType: domain.ErrorTypeInternal,
		},
		{
			name: "meeting doesn't exist but creation continues",
			payload: &models.PastMeeting{
				MeetingUID:         "meeting-123",
				ProjectUID:         "project-123",
				ScheduledStartTime: mustParseTimeForTest(scheduledStartTime),
				ScheduledEndTime:   mustParseTimeForTest(scheduledEndTime),
				Title:              "Test Past Meeting",
				Description:        "Test Description",
				Platform:           models.PlatformZoom,
			},
			setupMocks: func(mockMeetingRepo *mocks.MockMeetingRepository, mockPastMeetingRepo *mocks.MockPastMeetingRepository, mockMeetingAttachmentRepo *mocks.MockMeetingAttachmentRepository, mockPastMeetingAttachmentRepo *mocks.MockPastMeetingAttachmentRepository, mockBuilder *mocks.MockMessageBuilder) {
				// List meeting attachments (returns empty list by default)
				mockMeetingAttachmentRepo.On("ListByMeeting", mock.Anything, mock.Anything).Return([]*models.MeetingAttachment{}, nil).Maybe()
				// Meeting doesn't exist
				mockMeetingRepo.On("Exists", mock.Anything, "meeting-123").Return(false, nil)

				// But creation continues
				mockPastMeetingRepo.On("Create", mock.Anything, mock.Anything).Return(nil)
				mockBuilder.On("SendIndexPastMeeting", mock.Anything, models.ActionCreated, mock.Anything, mock.Anything).Return(nil)
				mockBuilder.On("SendUpdateAccessPastMeeting", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "with recurrence",
			payload: func() *models.PastMeeting {
				endDateTime := now.Add(30 * 24 * time.Hour)
				return &models.PastMeeting{
					MeetingUID:         "meeting-123",
					ProjectUID:         "project-123",
					ScheduledStartTime: mustParseTimeForTest(scheduledStartTime),
					ScheduledEndTime:   mustParseTimeForTest(scheduledEndTime),
					Title:              "Recurring Meeting",
					Description:        "Test Description",
					Platform:           models.PlatformZoom,
					Recurrence: &models.Recurrence{
						Type:           2, // weekly
						RepeatInterval: 1,
						WeeklyDays:     "1,3", // monday, wednesday
						EndDateTime:    &endDateTime,
					},
				}
			}(),
			setupMocks: func(mockMeetingRepo *mocks.MockMeetingRepository, mockPastMeetingRepo *mocks.MockPastMeetingRepository, mockMeetingAttachmentRepo *mocks.MockMeetingAttachmentRepository, mockPastMeetingAttachmentRepo *mocks.MockPastMeetingAttachmentRepository, mockBuilder *mocks.MockMessageBuilder) {
				// List meeting attachments (returns empty list by default)
				mockMeetingAttachmentRepo.On("ListByMeeting", mock.Anything, mock.Anything).Return([]*models.MeetingAttachment{}, nil).Maybe()
				mockMeetingRepo.On("Exists", mock.Anything, "meeting-123").Return(true, nil)
				mockPastMeetingRepo.On("Create", mock.Anything, mock.MatchedBy(func(pm *models.PastMeeting) bool {
					return pm.Recurrence != nil &&
						pm.Recurrence.Type == 2 &&
						pm.Recurrence.RepeatInterval == 1 &&
						pm.Recurrence.WeeklyDays == "1,3"
				})).Return(nil)
				mockBuilder.On("SendIndexPastMeeting", mock.Anything, models.ActionCreated, mock.Anything, mock.Anything).Return(nil)
				mockBuilder.On("SendUpdateAccessPastMeeting", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			},
			wantErr: false,
			validate: func(t *testing.T, result *models.PastMeeting) {
				assert.NotNil(t, result.Recurrence)
				assert.Equal(t, 2, result.Recurrence.Type)
			},
		},
		{
			name: "with multiple committees",
			payload: &models.PastMeeting{
				MeetingUID:         "meeting-123",
				ProjectUID:         "project-123",
				ScheduledStartTime: mustParseTimeForTest(scheduledStartTime),
				ScheduledEndTime:   mustParseTimeForTest(scheduledEndTime),
				Title:              "Committee Meeting",
				Description:        "Test Description",
				Platform:           models.PlatformZoom,
				Committees: []models.Committee{
					{
						UID:                   "committee-1",
						AllowedVotingStatuses: []string{"member", "admin"},
					},
					{
						UID:                   "committee-2",
						AllowedVotingStatuses: []string{"member"},
					},
				},
			},
			setupMocks: func(mockMeetingRepo *mocks.MockMeetingRepository, mockPastMeetingRepo *mocks.MockPastMeetingRepository, mockMeetingAttachmentRepo *mocks.MockMeetingAttachmentRepository, mockPastMeetingAttachmentRepo *mocks.MockPastMeetingAttachmentRepository, mockBuilder *mocks.MockMessageBuilder) {
				// List meeting attachments (returns empty list by default)
				mockMeetingAttachmentRepo.On("ListByMeeting", mock.Anything, mock.Anything).Return([]*models.MeetingAttachment{}, nil).Maybe()
				mockMeetingRepo.On("Exists", mock.Anything, "meeting-123").Return(true, nil)
				mockPastMeetingRepo.On("Create", mock.Anything, mock.MatchedBy(func(pm *models.PastMeeting) bool {
					return len(pm.Committees) == 2 &&
						pm.Committees[0].UID == "committee-1" &&
						len(pm.Committees[0].AllowedVotingStatuses) == 2
				})).Return(nil)
				mockBuilder.On("SendIndexPastMeeting", mock.Anything, models.ActionCreated, mock.Anything, mock.Anything).Return(nil)
				mockBuilder.On("SendUpdateAccessPastMeeting", mock.Anything, mock.MatchedBy(func(msg models.PastMeetingAccessMessage) bool {
					return len(msg.Committees) == 2
				}), mock.Anything).Return(nil)
			},
			wantErr: false,
			validate: func(t *testing.T, result *models.PastMeeting) {
				assert.Len(t, result.Committees, 2)
			},
		},
		{
			name: "with multiple sessions",
			payload: func() *models.PastMeeting {
				session2StartTime := now.Add(3 * time.Hour).Format(time.RFC3339)
				session2EndTime := now.Add(4 * time.Hour).Format(time.RFC3339)
				return &models.PastMeeting{
					MeetingUID:         "meeting-123",
					ProjectUID:         "project-123",
					ScheduledStartTime: mustParseTimeForTest(scheduledStartTime),
					ScheduledEndTime:   mustParseTimeForTest(scheduledEndTime),
					Title:              "Multi-session Meeting",
					Description:        "Test Description",
					Platform:           models.PlatformZoom,
					Sessions: []models.Session{
						{
							UID:       "session-1",
							StartTime: mustParseTimeForTest(scheduledStartTime),
							EndTime:   &[]time.Time{mustParseTimeForTest(scheduledEndTime)}[0],
						},
						{
							UID:       "session-2",
							StartTime: mustParseTimeForTest(session2StartTime),
							EndTime:   &[]time.Time{mustParseTimeForTest(session2EndTime)}[0],
						},
					},
				}
			}(),
			setupMocks: func(mockMeetingRepo *mocks.MockMeetingRepository, mockPastMeetingRepo *mocks.MockPastMeetingRepository, mockMeetingAttachmentRepo *mocks.MockMeetingAttachmentRepository, mockPastMeetingAttachmentRepo *mocks.MockPastMeetingAttachmentRepository, mockBuilder *mocks.MockMessageBuilder) {
				// List meeting attachments (returns empty list by default)
				mockMeetingAttachmentRepo.On("ListByMeeting", mock.Anything, mock.Anything).Return([]*models.MeetingAttachment{}, nil).Maybe()
				mockMeetingRepo.On("Exists", mock.Anything, "meeting-123").Return(true, nil)
				mockPastMeetingRepo.On("Create", mock.Anything, mock.MatchedBy(func(pm *models.PastMeeting) bool {
					return len(pm.Sessions) == 2 &&
						pm.Sessions[0].UID == "session-1" &&
						pm.Sessions[1].UID == "session-2"
				})).Return(nil)
				mockBuilder.On("SendIndexPastMeeting", mock.Anything, models.ActionCreated, mock.Anything, mock.Anything).Return(nil)
				mockBuilder.On("SendUpdateAccessPastMeeting", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			},
			wantErr: false,
			validate: func(t *testing.T, result *models.PastMeeting) {
				assert.Len(t, result.Sessions, 2)
			},
		},
		{
			name: "messaging failure doesn't fail operation",
			payload: &models.PastMeeting{
				MeetingUID:         "meeting-123",
				ProjectUID:         "project-123",
				ScheduledStartTime: mustParseTimeForTest(scheduledStartTime),
				ScheduledEndTime:   mustParseTimeForTest(scheduledEndTime),
				Title:              "Test Past Meeting",
				Description:        "Test Description",
				Platform:           models.PlatformZoom,
			},
			setupMocks: func(mockMeetingRepo *mocks.MockMeetingRepository, mockPastMeetingRepo *mocks.MockPastMeetingRepository, mockMeetingAttachmentRepo *mocks.MockMeetingAttachmentRepository, mockPastMeetingAttachmentRepo *mocks.MockPastMeetingAttachmentRepository, mockBuilder *mocks.MockMessageBuilder) {
				// List meeting attachments (returns empty list by default)
				mockMeetingAttachmentRepo.On("ListByMeeting", mock.Anything, mock.Anything).Return([]*models.MeetingAttachment{}, nil).Maybe()
				mockMeetingRepo.On("Exists", mock.Anything, "meeting-123").Return(true, nil)
				mockPastMeetingRepo.On("Create", mock.Anything, mock.Anything).Return(nil)

				// Messaging fails but operation continues
				// Due to errgroup behavior, either one or both calls might be made depending on timing
				mockBuilder.On("SendIndexPastMeeting", mock.Anything, models.ActionCreated, mock.Anything, mock.Anything).Return(errors.New("messaging error")).Maybe()
				mockBuilder.On("SendUpdateAccessPastMeeting", mock.Anything, mock.Anything, mock.Anything).Return(errors.New("messaging error")).Maybe()
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, mockMeetingRepo, mockPastMeetingRepo, mockMeetingAttachmentRepo, mockPastMeetingAttachmentRepo, mockBuilder := setupPastMeetingServiceForTesting()

			// Remove repositories to test service not ready case
			if tt.name == "service not ready" {
				// Create a service with nil repository for this test
				service = NewPastMeetingService(nil, nil, nil, nil, mockBuilder, ServiceConfig{})
			}

			if tt.setupMocks != nil {
				tt.setupMocks(mockMeetingRepo, mockPastMeetingRepo, mockMeetingAttachmentRepo, mockPastMeetingAttachmentRepo, mockBuilder)
			}

			result, err := service.CreatePastMeeting(ctx, tt.payload)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.expectedErrType != 0 {
					assert.Equal(t, tt.expectedErrType, domain.GetErrorType(err))
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				if tt.validate != nil {
					tt.validate(t, result)
				}
			}

			mockMeetingRepo.AssertExpectations(t)
			mockPastMeetingRepo.AssertExpectations(t)
			mockBuilder.AssertExpectations(t)
		})
	}
}

func TestPastMeetingService_GetPastMeetings(t *testing.T) {
	ctx := context.Background()
	now := time.Now()

	tests := []struct {
		name            string
		setupMocks      func(*mocks.MockPastMeetingRepository)
		wantErr         bool
		expectedErrType domain.ErrorType
		expectedLen     int
	}{
		{
			name: "successful get all past meetings",
			setupMocks: func(mockRepo *mocks.MockPastMeetingRepository) {
				mockRepo.On("ListAll", mock.Anything).Return([]*models.PastMeeting{
					{
						UID:        "past-meeting-1",
						MeetingUID: "meeting-1",
						Title:      "Past Meeting 1",
						CreatedAt:  &now,
					},
					{
						UID:        "past-meeting-2",
						MeetingUID: "meeting-2",
						Title:      "Past Meeting 2",
						CreatedAt:  &now,
					},
				}, nil)
			},
			wantErr:     false,
			expectedLen: 2,
		},
		{
			name: "service not ready",
			setupMocks: func(mockRepo *mocks.MockPastMeetingRepository) {
				// Don't set up mocks
			},
			wantErr:         true,
			expectedErrType: domain.ErrorTypeUnavailable,
		},
		{
			name: "repository error",
			setupMocks: func(mockRepo *mocks.MockPastMeetingRepository) {
				mockRepo.On("ListAll", mock.Anything).Return(nil, domain.NewInternalError("database error"))
			},
			wantErr:         true,
			expectedErrType: domain.ErrorTypeInternal,
		},
		{
			name: "empty list",
			setupMocks: func(mockRepo *mocks.MockPastMeetingRepository) {
				mockRepo.On("ListAll", mock.Anything).Return([]*models.PastMeeting{}, nil)
			},
			wantErr:     false,
			expectedLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, _, mockPastMeetingRepo, _, _, _ := setupPastMeetingServiceForTesting()

			// Test service not ready case
			if tt.name == "service not ready" {
				// Create a service with nil repository for this test
				service = NewPastMeetingService(nil, nil, nil, nil, nil, ServiceConfig{})
			}

			if tt.setupMocks != nil {
				tt.setupMocks(mockPastMeetingRepo)
			}

			result, err := service.ListPastMeetings(ctx)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.expectedErrType != 0 {
					assert.Equal(t, tt.expectedErrType, domain.GetErrorType(err))
				}
			} else {
				assert.NoError(t, err)
				assert.Len(t, result, tt.expectedLen)
			}

			mockPastMeetingRepo.AssertExpectations(t)
		})
	}
}

func TestPastMeetingService_GetPastMeeting(t *testing.T) {
	ctx := context.Background()
	now := time.Now()

	tests := []struct {
		name            string
		uid             string
		setupMocks      func(*mocks.MockPastMeetingRepository)
		wantErr         bool
		expectedErrType domain.ErrorType
		expectedUID     string
		expectedETag    string
	}{
		{
			name: "successful get",
			uid:  "past-meeting-123",
			setupMocks: func(mockRepo *mocks.MockPastMeetingRepository) {
				mockRepo.On("GetWithRevision", mock.Anything, "past-meeting-123").Return(&models.PastMeeting{
					UID:        "past-meeting-123",
					MeetingUID: "meeting-123",
					Title:      "Test Past Meeting",
					CreatedAt:  &now,
				}, uint64(42), nil)
			},
			wantErr:      false,
			expectedUID:  "past-meeting-123",
			expectedETag: "42",
		},
		{
			name: "service not ready",
			uid:  "past-meeting-123",
			setupMocks: func(mockRepo *mocks.MockPastMeetingRepository) {
				// Don't set up mocks
			},
			wantErr:         true,
			expectedErrType: domain.ErrorTypeUnavailable,
		},
		{
			name: "empty UID",
			uid:  "",
			setupMocks: func(mockRepo *mocks.MockPastMeetingRepository) {
				mockRepo.On("GetWithRevision", mock.Anything, "").Return(nil, uint64(0), domain.NewNotFoundError("past meeting not found"))
			},
			wantErr:         true,
			expectedErrType: domain.ErrorTypeNotFound,
		},
		{
			name: "past meeting not found",
			uid:  "past-meeting-123",
			setupMocks: func(mockRepo *mocks.MockPastMeetingRepository) {
				mockRepo.On("GetWithRevision", mock.Anything, "past-meeting-123").Return(nil, uint64(0), domain.NewNotFoundError("past meeting not found"))
			},
			wantErr:         true,
			expectedErrType: domain.ErrorTypeNotFound,
		},
		{
			name: "repository error",
			uid:  "past-meeting-123",
			setupMocks: func(mockRepo *mocks.MockPastMeetingRepository) {
				mockRepo.On("GetWithRevision", mock.Anything, "past-meeting-123").Return(nil, uint64(0), domain.NewInternalError("database error"))
			},
			wantErr:         true,
			expectedErrType: domain.ErrorTypeInternal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, _, mockPastMeetingRepo, _, _, _ := setupPastMeetingServiceForTesting()

			// Test service not ready case
			if tt.name == "service not ready" {
				// Create a service with nil repository for this test
				service = NewPastMeetingService(nil, nil, nil, nil, nil, ServiceConfig{})
			}

			if tt.setupMocks != nil {
				tt.setupMocks(mockPastMeetingRepo)
			}

			result, etag, err := service.GetPastMeeting(ctx, tt.uid)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.expectedErrType != 0 {
					assert.Equal(t, tt.expectedErrType, domain.GetErrorType(err))
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.expectedUID, result.UID)
				assert.Equal(t, tt.expectedETag, etag)
			}

			mockPastMeetingRepo.AssertExpectations(t)
		})
	}
}

func TestPastMeetingService_DeletePastMeeting(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name            string
		uid             string
		revision        uint64
		setupMocks      func(*mocks.MockPastMeetingRepository, *mocks.MockMessageBuilder, bool)
		skipEtag        bool
		wantErr         bool
		expectedErrType domain.ErrorType
	}{
		{
			name:     "successful delete",
			uid:      "past-meeting-123",
			revision: 42,
			setupMocks: func(mockRepo *mocks.MockPastMeetingRepository, mockBuilder *mocks.MockMessageBuilder, skipEtag bool) {
				mockRepo.On("Exists", mock.Anything, "past-meeting-123").Return(true, nil)
				mockRepo.On("Delete", mock.Anything, "past-meeting-123", uint64(42)).Return(nil)

				// Messages sent after successful deletion
				mockBuilder.On("SendDeleteIndexPastMeeting", mock.Anything, "past-meeting-123", mock.Anything).Return(nil)
				mockBuilder.On("SendDeleteAllAccessPastMeeting", mock.Anything, "past-meeting-123", mock.Anything).Return(nil)
			},
			wantErr: false,
		},
		{
			name:     "service not ready",
			uid:      "past-meeting-123",
			revision: 42,
			setupMocks: func(mockRepo *mocks.MockPastMeetingRepository, mockBuilder *mocks.MockMessageBuilder, skipEtag bool) {
				// Don't set up mocks
			},
			wantErr:         true,
			expectedErrType: domain.ErrorTypeUnavailable,
		},
		{
			name:     "empty UID",
			uid:      "",
			revision: 42,
			setupMocks: func(mockRepo *mocks.MockPastMeetingRepository, mockBuilder *mocks.MockMessageBuilder, skipEtag bool) {
				mockRepo.On("Exists", mock.Anything, "").Return(false, nil)
			},
			wantErr:         true,
			expectedErrType: domain.ErrorTypeNotFound,
		},
		{
			name:     "past meeting not found",
			uid:      "past-meeting-123",
			revision: 42,
			setupMocks: func(mockRepo *mocks.MockPastMeetingRepository, mockBuilder *mocks.MockMessageBuilder, skipEtag bool) {
				mockRepo.On("Exists", mock.Anything, "past-meeting-123").Return(false, nil)
			},
			wantErr:         true,
			expectedErrType: domain.ErrorTypeNotFound,
		},
		{
			name:     "revision mismatch",
			uid:      "past-meeting-123",
			revision: 42,
			setupMocks: func(mockRepo *mocks.MockPastMeetingRepository, mockBuilder *mocks.MockMessageBuilder, skipEtag bool) {
				mockRepo.On("Exists", mock.Anything, "past-meeting-123").Return(true, nil)
				mockRepo.On("Delete", mock.Anything, "past-meeting-123", uint64(42)).Return(domain.NewConflictError("revision mismatch"))
			},
			wantErr:         true,
			expectedErrType: domain.ErrorTypeConflict,
		},
		{
			name:     "skip etag validation",
			uid:      "past-meeting-123",
			revision: 0, // Will be ignored
			setupMocks: func(mockRepo *mocks.MockPastMeetingRepository, mockBuilder *mocks.MockMessageBuilder, skipEtag bool) {
				// When skipping etag, get revision first
				mockRepo.On("GetWithRevision", mock.Anything, "past-meeting-123").Return(&models.PastMeeting{
					UID: "past-meeting-123",
				}, uint64(42), nil)
				mockRepo.On("Exists", mock.Anything, "past-meeting-123").Return(true, nil)
				mockRepo.On("Delete", mock.Anything, "past-meeting-123", uint64(42)).Return(nil)

				mockBuilder.On("SendDeleteIndexPastMeeting", mock.Anything, "past-meeting-123", mock.Anything).Return(nil)
				mockBuilder.On("SendDeleteAllAccessPastMeeting", mock.Anything, "past-meeting-123", mock.Anything).Return(nil)
			},
			skipEtag: true,
			wantErr:  false,
		},
		{
			name:     "repository delete error",
			uid:      "past-meeting-123",
			revision: 42,
			setupMocks: func(mockRepo *mocks.MockPastMeetingRepository, mockBuilder *mocks.MockMessageBuilder, skipEtag bool) {
				mockRepo.On("Exists", mock.Anything, "past-meeting-123").Return(true, nil)
				mockRepo.On("Delete", mock.Anything, "past-meeting-123", uint64(42)).Return(domain.NewInternalError("database error"))
			},
			wantErr:         true,
			expectedErrType: domain.ErrorTypeInternal,
		},
		{
			name:     "messaging failure doesn't fail operation",
			uid:      "past-meeting-123",
			revision: 42,
			setupMocks: func(mockRepo *mocks.MockPastMeetingRepository, mockBuilder *mocks.MockMessageBuilder, skipEtag bool) {
				mockRepo.On("Exists", mock.Anything, "past-meeting-123").Return(true, nil)
				mockRepo.On("Delete", mock.Anything, "past-meeting-123", uint64(42)).Return(nil)

				// Messaging fails but operation succeeds
				mockBuilder.On("SendDeleteIndexPastMeeting", mock.Anything, "past-meeting-123", mock.Anything).Return(errors.New("messaging error")).Maybe()
				mockBuilder.On("SendDeleteAllAccessPastMeeting", mock.Anything, "past-meeting-123", mock.Anything).Return(errors.New("messaging error")).Maybe()
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, _, mockPastMeetingRepo, _, _, mockBuilder := setupPastMeetingServiceForTesting()

			if tt.skipEtag {
				// Create a service with SkipEtagValidation enabled
				config := ServiceConfig{SkipEtagValidation: true}
				_, mockMeetingRepo, _, _, _, _ := setupPastMeetingServiceForTesting()
				mockMeetingAttachmentRepo := new(mocks.MockMeetingAttachmentRepository)
				mockPastMeetingAttachmentRepo := new(mocks.MockPastMeetingAttachmentRepository)
				service = NewPastMeetingService(
					mockMeetingRepo,
					mockPastMeetingRepo,
					mockMeetingAttachmentRepo,
					mockPastMeetingAttachmentRepo,
					mockBuilder,
					config,
				)
			}

			// Test service not ready case
			if tt.name == "service not ready" {
				// Create a service with nil repository for this test
				service = NewPastMeetingService(nil, nil, nil, nil, nil, ServiceConfig{})
			}

			if tt.setupMocks != nil {
				tt.setupMocks(mockPastMeetingRepo, mockBuilder, tt.skipEtag)
			}

			err := service.DeletePastMeeting(ctx, tt.uid, tt.revision)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.expectedErrType != 0 {
					assert.Equal(t, tt.expectedErrType, domain.GetErrorType(err))
				}
			} else {
				assert.NoError(t, err)
			}

			mockPastMeetingRepo.AssertExpectations(t)
			mockBuilder.AssertExpectations(t)
		})
	}
}

func TestPastMeetingService_validateCreatePastMeetingPayload(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	scheduledStartTime := now.Add(-2 * time.Hour).Format(time.RFC3339)
	scheduledEndTime := now.Add(-time.Hour).Format(time.RFC3339)

	tests := []struct {
		name            string
		payload         *models.PastMeeting
		wantErr         bool
		expectedErrType domain.ErrorType
	}{
		{
			name: "valid payload",
			payload: &models.PastMeeting{
				MeetingUID:         "meeting-123",
				ProjectUID:         "project-123",
				ScheduledStartTime: mustParseTimeForTest(scheduledStartTime),
				ScheduledEndTime:   mustParseTimeForTest(scheduledEndTime),
				Title:              "Test Meeting",
				Description:        "Test Description",
				Platform:           models.PlatformZoom,
			},
			wantErr: false,
		},
		{
			name:            "nil payload",
			payload:         nil,
			wantErr:         true,
			expectedErrType: domain.ErrorTypeValidation,
		},
		{
			name: "empty meeting UID",
			payload: &models.PastMeeting{
				MeetingUID:         "",
				ProjectUID:         "project-123",
				ScheduledStartTime: mustParseTimeForTest(scheduledStartTime),
				ScheduledEndTime:   mustParseTimeForTest(scheduledEndTime),
				Title:              "Test Meeting",
				Description:        "Test Description",
				Platform:           models.PlatformZoom,
			},
			wantErr:         true,
			expectedErrType: domain.ErrorTypeValidation,
		},
		{
			name: "empty project UID",
			payload: &models.PastMeeting{
				MeetingUID:         "meeting-123",
				ProjectUID:         "",
				ScheduledStartTime: mustParseTimeForTest(scheduledStartTime),
				ScheduledEndTime:   mustParseTimeForTest(scheduledEndTime),
				Title:              "Test Meeting",
				Description:        "Test Description",
				Platform:           models.PlatformZoom,
			},
			wantErr:         true,
			expectedErrType: domain.ErrorTypeValidation,
		},
		{
			name: "empty title",
			payload: &models.PastMeeting{
				MeetingUID:         "meeting-123",
				ProjectUID:         "project-123",
				ScheduledStartTime: mustParseTimeForTest(scheduledStartTime),
				ScheduledEndTime:   mustParseTimeForTest(scheduledEndTime),
				Title:              "",
				Description:        "Test Description",
				Platform:           models.PlatformZoom,
			},
			wantErr:         true,
			expectedErrType: domain.ErrorTypeValidation,
		},
		{
			name: "empty description",
			payload: &models.PastMeeting{
				MeetingUID:         "meeting-123",
				ProjectUID:         "project-123",
				ScheduledStartTime: mustParseTimeForTest(scheduledStartTime),
				ScheduledEndTime:   mustParseTimeForTest(scheduledEndTime),
				Title:              "Test Meeting",
				Description:        "",
				Platform:           models.PlatformZoom,
			},
			wantErr:         true,
			expectedErrType: domain.ErrorTypeValidation,
		},
		{
			name: "empty platform",
			payload: &models.PastMeeting{
				MeetingUID:         "meeting-123",
				ProjectUID:         "project-123",
				ScheduledStartTime: mustParseTimeForTest(scheduledStartTime),
				ScheduledEndTime:   mustParseTimeForTest(scheduledEndTime),
				Title:              "Test Meeting",
				Description:        "Test Description",
				Platform:           "",
			},
			wantErr:         true,
			expectedErrType: domain.ErrorTypeValidation,
		},
		{
			name: "end time before start time",
			payload: &models.PastMeeting{
				MeetingUID:         "meeting-123",
				ProjectUID:         "project-123",
				ScheduledStartTime: mustParseTimeForTest(scheduledEndTime),
				ScheduledEndTime:   mustParseTimeForTest(scheduledStartTime),
				Title:              "Test Meeting",
				Description:        "Test Description",
				Platform:           models.PlatformZoom,
			},
			wantErr:         true,
			expectedErrType: domain.ErrorTypeValidation,
		},
		{
			name: "scheduled start time too far in future (61 minutes)",
			payload: &models.PastMeeting{
				MeetingUID:         "meeting-123",
				ProjectUID:         "project-123",
				ScheduledStartTime: now.Add(61 * time.Minute),
				ScheduledEndTime:   now.Add(121 * time.Minute),
				Title:              "Test Meeting",
				Description:        "Test Description",
				Platform:           models.PlatformZoom,
			},
			wantErr:         true,
			expectedErrType: domain.ErrorTypeValidation,
		},
		{
			name: "scheduled start time at max early join limit (60 minutes)",
			payload: &models.PastMeeting{
				MeetingUID:         "meeting-123",
				ProjectUID:         "project-123",
				ScheduledStartTime: now.Add(60 * time.Minute),
				ScheduledEndTime:   now.Add(120 * time.Minute),
				Title:              "Test Meeting",
				Description:        "Test Description",
				Platform:           models.PlatformZoom,
			},
			wantErr: false,
		},
		{
			name: "scheduled start time slightly in future (30 minutes)",
			payload: &models.PastMeeting{
				MeetingUID:         "meeting-123",
				ProjectUID:         "project-123",
				ScheduledStartTime: now.Add(30 * time.Minute),
				ScheduledEndTime:   now.Add(90 * time.Minute),
				Title:              "Test Meeting",
				Description:        "Test Description",
				Platform:           models.PlatformZoom,
			},
			wantErr: false,
		},
		{
			name: "scheduled end time too far from start time (601 minutes)",
			payload: &models.PastMeeting{
				MeetingUID:         "meeting-123",
				ProjectUID:         "project-123",
				ScheduledStartTime: now.Add(-2 * time.Hour),
				ScheduledEndTime:   now.Add(-2*time.Hour + 601*time.Minute),
				Title:              "Test Meeting",
				Description:        "Test Description",
				Platform:           models.PlatformZoom,
			},
			wantErr:         true,
			expectedErrType: domain.ErrorTypeValidation,
		},
		{
			name: "scheduled end time at max duration limit (600 minutes)",
			payload: &models.PastMeeting{
				MeetingUID:         "meeting-123",
				ProjectUID:         "project-123",
				ScheduledStartTime: now.Add(-2 * time.Hour),
				ScheduledEndTime:   now.Add(-2*time.Hour + 600*time.Minute),
				Title:              "Test Meeting",
				Description:        "Test Description",
				Platform:           models.PlatformZoom,
			},
			wantErr: false,
		},
		{
			name: "scheduled end time within duration limit (120 minutes)",
			payload: &models.PastMeeting{
				MeetingUID:         "meeting-123",
				ProjectUID:         "project-123",
				ScheduledStartTime: now.Add(-2 * time.Hour),
				ScheduledEndTime:   now.Add(-2*time.Hour + 120*time.Minute),
				Title:              "Test Meeting",
				Description:        "Test Description",
				Platform:           models.PlatformZoom,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, _, _, _, _, _ := setupPastMeetingServiceForTesting()

			err := service.validateCreatePastMeetingPayload(ctx, tt.payload)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.expectedErrType != 0 {
					assert.Equal(t, tt.expectedErrType, domain.GetErrorType(err))
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
