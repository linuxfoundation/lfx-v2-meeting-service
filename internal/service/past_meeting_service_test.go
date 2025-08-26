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
)

// mustParseTime is a helper function for tests
func mustParseTime(timeStr string) time.Time {
	t, err := time.Parse(time.RFC3339, timeStr)
	if err != nil {
		panic(err)
	}
	return t
}

// setupPastMeetingServiceForTesting creates a PastMeetingService with all mock dependencies for testing
func setupPastMeetingServiceForTesting() (*PastMeetingService, *mocks.MockMeetingRepository, *mocks.MockPastMeetingRepository, *mocks.MockMessageBuilder) {
	mockMeetingRepo := new(mocks.MockMeetingRepository)
	mockPastMeetingRepo := new(mocks.MockPastMeetingRepository)
	mockBuilder := new(mocks.MockMessageBuilder)

	config := ServiceConfig{
		SkipEtagValidation: false,
	}

	service := &PastMeetingService{
		MeetingRepository:     mockMeetingRepo,
		PastMeetingRepository: mockPastMeetingRepo,
		MessageBuilder:        mockBuilder,
		Config:                config,
	}

	return service, mockMeetingRepo, mockPastMeetingRepo, mockBuilder
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
				service, _, _, _ := setupPastMeetingServiceForTesting()
				return service
			},
			expected: true,
		},
		{
			name: "service not ready - missing past meeting repository",
			setup: func() *PastMeetingService {
				service, _, _, _ := setupPastMeetingServiceForTesting()
				service.PastMeetingRepository = nil
				return service
			},
			expected: false,
		},
		{
			name: "service not ready - missing meeting repository",
			setup: func() *PastMeetingService {
				service, _, _, _ := setupPastMeetingServiceForTesting()
				service.MeetingRepository = nil
				return service
			},
			expected: false,
		},
		{
			name: "service not ready - missing message builder",
			setup: func() *PastMeetingService {
				service, _, _, _ := setupPastMeetingServiceForTesting()
				service.MessageBuilder = nil
				return service
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
	scheduledStartTime := now.Add(time.Hour).Format(time.RFC3339)
	scheduledEndTime := now.Add(2 * time.Hour).Format(time.RFC3339)

	tests := []struct {
		name        string
		payload     *models.PastMeeting
		setupMocks  func(*mocks.MockMeetingRepository, *mocks.MockPastMeetingRepository, *mocks.MockMessageBuilder)
		wantErr     bool
		expectedErr error
		validate    func(*testing.T, *models.PastMeeting)
	}{
		{
			name: "successful creation",
			payload: &models.PastMeeting{
				MeetingUID:         "meeting-123",
				ProjectUID:         "project-123",
				ScheduledStartTime: mustParseTime(scheduledStartTime),
				ScheduledEndTime:   mustParseTime(scheduledEndTime),
				Title:              "Test Past Meeting",
				Description:        "Test Description",
				Platform:           "Zoom",
				Visibility:         "public",
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
						StartTime: mustParseTime(scheduledStartTime),
						EndTime:   &[]time.Time{mustParseTime(scheduledEndTime)}[0],
					},
				},
			},
			setupMocks: func(mockMeetingRepo *mocks.MockMeetingRepository, mockPastMeetingRepo *mocks.MockPastMeetingRepository, mockBuilder *mocks.MockMessageBuilder) {
				// Check if meeting exists
				mockMeetingRepo.On("Exists", mock.Anything, "meeting-123").Return(true, nil)

				// Create past meeting
				mockPastMeetingRepo.On("Create", mock.Anything, mock.MatchedBy(func(pm *models.PastMeeting) bool {
					return pm.MeetingUID == "meeting-123" &&
						pm.ProjectUID == "project-123" &&
						pm.Title == "Test Past Meeting" &&
						pm.Description == "Test Description" &&
						pm.Platform == "Zoom" &&
						pm.Visibility == "public" &&
						len(pm.Committees) == 1 &&
						pm.ZoomConfig != nil &&
						len(pm.Sessions) == 1
				})).Return(nil)

				// Send indexer and access messages
				mockBuilder.On("SendIndexPastMeeting", mock.Anything, models.ActionCreated, mock.Anything).Return(nil)
				mockBuilder.On("SendUpdateAccessPastMeeting", mock.Anything, mock.Anything).Return(nil)
			},
			wantErr: false,
			validate: func(t *testing.T, pm *models.PastMeeting) {
				assert.NotEmpty(t, pm.UID)
				assert.Equal(t, "meeting-123", pm.MeetingUID)
				assert.Equal(t, "project-123", pm.ProjectUID)
				assert.Equal(t, "Test Past Meeting", pm.Title)
				assert.Equal(t, "Test Description", pm.Description)
				assert.Equal(t, "Zoom", pm.Platform)
				assert.Equal(t, "public", pm.Visibility)
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
				ScheduledStartTime: mustParseTime(scheduledStartTime),
				ScheduledEndTime:   mustParseTime(scheduledEndTime),
				Title:              "Test Past Meeting",
				Description:        "Test Description",
				Platform:           "Zoom",
			},
			setupMocks: func(mockMeetingRepo *mocks.MockMeetingRepository, mockPastMeetingRepo *mocks.MockPastMeetingRepository, mockBuilder *mocks.MockMessageBuilder) {
				// Make service not ready by not setting up mocks
			},
			wantErr:     true,
			expectedErr: domain.ErrServiceUnavailable,
		},
		{
			name:        "nil payload",
			payload:     nil,
			setupMocks:  func(*mocks.MockMeetingRepository, *mocks.MockPastMeetingRepository, *mocks.MockMessageBuilder) {},
			wantErr:     true,
			expectedErr: domain.ErrValidationFailed,
		},
		{
			name: "missing required fields",
			payload: &models.PastMeeting{
				MeetingUID: "",
				ProjectUID: "project-123",
			},
			setupMocks:  func(*mocks.MockMeetingRepository, *mocks.MockPastMeetingRepository, *mocks.MockMessageBuilder) {},
			wantErr:     true,
			expectedErr: domain.ErrValidationFailed,
		},
		{
			name: "end time before start time",
			payload: &models.PastMeeting{
				MeetingUID:         "meeting-123",
				ProjectUID:         "project-123",
				ScheduledStartTime: mustParseTime(scheduledEndTime), // Swapped
				ScheduledEndTime:   mustParseTime(scheduledStartTime),
				Title:              "Test Past Meeting",
				Description:        "Test Description",
				Platform:           "Zoom",
			},
			setupMocks:  func(*mocks.MockMeetingRepository, *mocks.MockPastMeetingRepository, *mocks.MockMessageBuilder) {},
			wantErr:     true,
			expectedErr: domain.ErrValidationFailed,
		},
		{
			name: "repository create error",
			payload: &models.PastMeeting{
				MeetingUID:         "meeting-123",
				ProjectUID:         "project-123",
				ScheduledStartTime: mustParseTime(scheduledStartTime),
				ScheduledEndTime:   mustParseTime(scheduledEndTime),
				Title:              "Test Past Meeting",
				Description:        "Test Description",
				Platform:           "Zoom",
			},
			setupMocks: func(mockMeetingRepo *mocks.MockMeetingRepository, mockPastMeetingRepo *mocks.MockPastMeetingRepository, mockBuilder *mocks.MockMessageBuilder) {
				mockMeetingRepo.On("Exists", mock.Anything, "meeting-123").Return(true, nil)
				mockPastMeetingRepo.On("Create", mock.Anything, mock.Anything).Return(errors.New("database error"))
			},
			wantErr:     true,
			expectedErr: domain.ErrInternal,
		},
		{
			name: "meeting doesn't exist but creation continues",
			payload: &models.PastMeeting{
				MeetingUID:         "meeting-123",
				ProjectUID:         "project-123",
				ScheduledStartTime: mustParseTime(scheduledStartTime),
				ScheduledEndTime:   mustParseTime(scheduledEndTime),
				Title:              "Test Past Meeting",
				Description:        "Test Description",
				Platform:           "Zoom",
			},
			setupMocks: func(mockMeetingRepo *mocks.MockMeetingRepository, mockPastMeetingRepo *mocks.MockPastMeetingRepository, mockBuilder *mocks.MockMessageBuilder) {
				// Meeting doesn't exist
				mockMeetingRepo.On("Exists", mock.Anything, "meeting-123").Return(false, nil)

				// But creation continues
				mockPastMeetingRepo.On("Create", mock.Anything, mock.Anything).Return(nil)
				mockBuilder.On("SendIndexPastMeeting", mock.Anything, models.ActionCreated, mock.Anything).Return(nil)
				mockBuilder.On("SendUpdateAccessPastMeeting", mock.Anything, mock.Anything).Return(nil)
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
					ScheduledStartTime: mustParseTime(scheduledStartTime),
					ScheduledEndTime:   mustParseTime(scheduledEndTime),
					Title:              "Recurring Meeting",
					Description:        "Test Description",
					Platform:           "Zoom",
					Recurrence: &models.Recurrence{
						Type:           2, // weekly
						RepeatInterval: 1,
						WeeklyDays:     "1,3", // monday, wednesday
						EndDateTime:    &endDateTime,
					},
				}
			}(),
			setupMocks: func(mockMeetingRepo *mocks.MockMeetingRepository, mockPastMeetingRepo *mocks.MockPastMeetingRepository, mockBuilder *mocks.MockMessageBuilder) {
				mockMeetingRepo.On("Exists", mock.Anything, "meeting-123").Return(true, nil)
				mockPastMeetingRepo.On("Create", mock.Anything, mock.MatchedBy(func(pm *models.PastMeeting) bool {
					return pm.Recurrence != nil &&
						pm.Recurrence.Type == 2 &&
						pm.Recurrence.RepeatInterval == 1 &&
						pm.Recurrence.WeeklyDays == "1,3"
				})).Return(nil)
				mockBuilder.On("SendIndexPastMeeting", mock.Anything, models.ActionCreated, mock.Anything).Return(nil)
				mockBuilder.On("SendUpdateAccessPastMeeting", mock.Anything, mock.Anything).Return(nil)
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
				ScheduledStartTime: mustParseTime(scheduledStartTime),
				ScheduledEndTime:   mustParseTime(scheduledEndTime),
				Title:              "Committee Meeting",
				Description:        "Test Description",
				Platform:           "Zoom",
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
			setupMocks: func(mockMeetingRepo *mocks.MockMeetingRepository, mockPastMeetingRepo *mocks.MockPastMeetingRepository, mockBuilder *mocks.MockMessageBuilder) {
				mockMeetingRepo.On("Exists", mock.Anything, "meeting-123").Return(true, nil)
				mockPastMeetingRepo.On("Create", mock.Anything, mock.MatchedBy(func(pm *models.PastMeeting) bool {
					return len(pm.Committees) == 2 &&
						pm.Committees[0].UID == "committee-1" &&
						len(pm.Committees[0].AllowedVotingStatuses) == 2
				})).Return(nil)
				mockBuilder.On("SendIndexPastMeeting", mock.Anything, models.ActionCreated, mock.Anything).Return(nil)
				mockBuilder.On("SendUpdateAccessPastMeeting", mock.Anything, mock.MatchedBy(func(msg models.PastMeetingAccessMessage) bool {
					return len(msg.Committees) == 2
				})).Return(nil)
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
					ScheduledStartTime: mustParseTime(scheduledStartTime),
					ScheduledEndTime:   mustParseTime(scheduledEndTime),
					Title:              "Multi-session Meeting",
					Description:        "Test Description",
					Platform:           "Zoom",
					Sessions: []models.Session{
						{
							UID:       "session-1",
							StartTime: mustParseTime(scheduledStartTime),
							EndTime:   &[]time.Time{mustParseTime(scheduledEndTime)}[0],
						},
						{
							UID:       "session-2",
							StartTime: mustParseTime(session2StartTime),
							EndTime:   &[]time.Time{mustParseTime(session2EndTime)}[0],
						},
					},
				}
			}(),
			setupMocks: func(mockMeetingRepo *mocks.MockMeetingRepository, mockPastMeetingRepo *mocks.MockPastMeetingRepository, mockBuilder *mocks.MockMessageBuilder) {
				mockMeetingRepo.On("Exists", mock.Anything, "meeting-123").Return(true, nil)
				mockPastMeetingRepo.On("Create", mock.Anything, mock.MatchedBy(func(pm *models.PastMeeting) bool {
					return len(pm.Sessions) == 2 &&
						pm.Sessions[0].UID == "session-1" &&
						pm.Sessions[1].UID == "session-2"
				})).Return(nil)
				mockBuilder.On("SendIndexPastMeeting", mock.Anything, models.ActionCreated, mock.Anything).Return(nil)
				mockBuilder.On("SendUpdateAccessPastMeeting", mock.Anything, mock.Anything).Return(nil)
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
				ScheduledStartTime: mustParseTime(scheduledStartTime),
				ScheduledEndTime:   mustParseTime(scheduledEndTime),
				Title:              "Test Past Meeting",
				Description:        "Test Description",
				Platform:           "Zoom",
			},
			setupMocks: func(mockMeetingRepo *mocks.MockMeetingRepository, mockPastMeetingRepo *mocks.MockPastMeetingRepository, mockBuilder *mocks.MockMessageBuilder) {
				mockMeetingRepo.On("Exists", mock.Anything, "meeting-123").Return(true, nil)
				mockPastMeetingRepo.On("Create", mock.Anything, mock.Anything).Return(nil)

				// Messaging fails but operation continues
				mockBuilder.On("SendIndexPastMeeting", mock.Anything, models.ActionCreated, mock.Anything).Return(errors.New("messaging error"))
				mockBuilder.On("SendUpdateAccessPastMeeting", mock.Anything, mock.Anything).Return(errors.New("messaging error"))
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, mockMeetingRepo, mockPastMeetingRepo, mockBuilder := setupPastMeetingServiceForTesting()

			// Remove repositories to test service not ready case
			if tt.name == "service not ready" {
				service.PastMeetingRepository = nil
			}

			if tt.setupMocks != nil {
				tt.setupMocks(mockMeetingRepo, mockPastMeetingRepo, mockBuilder)
			}

			result, err := service.CreatePastMeeting(ctx, tt.payload)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.expectedErr != nil {
					assert.True(t, errors.Is(err, tt.expectedErr))
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
		name        string
		setupMocks  func(*mocks.MockPastMeetingRepository)
		wantErr     bool
		expectedErr error
		expectedLen int
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
			wantErr:     true,
			expectedErr: domain.ErrServiceUnavailable,
		},
		{
			name: "repository error",
			setupMocks: func(mockRepo *mocks.MockPastMeetingRepository) {
				mockRepo.On("ListAll", mock.Anything).Return(nil, errors.New("database error"))
			},
			wantErr:     true,
			expectedErr: domain.ErrInternal,
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
			service, _, mockPastMeetingRepo, _ := setupPastMeetingServiceForTesting()

			// Test service not ready case
			if tt.name == "service not ready" {
				service.PastMeetingRepository = nil
			}

			if tt.setupMocks != nil {
				tt.setupMocks(mockPastMeetingRepo)
			}

			result, err := service.GetPastMeetings(ctx)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.expectedErr != nil {
					assert.True(t, errors.Is(err, tt.expectedErr))
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
		name         string
		uid          string
		setupMocks   func(*mocks.MockPastMeetingRepository)
		wantErr      bool
		expectedErr  error
		expectedUID  string
		expectedETag string
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
			wantErr:     true,
			expectedErr: domain.ErrServiceUnavailable,
		},
		{
			name: "empty UID",
			uid:  "",
			setupMocks: func(mockRepo *mocks.MockPastMeetingRepository) {
				mockRepo.On("GetWithRevision", mock.Anything, "").Return(nil, uint64(0), domain.ErrPastMeetingNotFound)
			},
			wantErr:     true,
			expectedErr: domain.ErrPastMeetingNotFound,
		},
		{
			name: "past meeting not found",
			uid:  "past-meeting-123",
			setupMocks: func(mockRepo *mocks.MockPastMeetingRepository) {
				mockRepo.On("GetWithRevision", mock.Anything, "past-meeting-123").Return(nil, uint64(0), domain.ErrPastMeetingNotFound)
			},
			wantErr:     true,
			expectedErr: domain.ErrPastMeetingNotFound,
		},
		{
			name: "repository error",
			uid:  "past-meeting-123",
			setupMocks: func(mockRepo *mocks.MockPastMeetingRepository) {
				mockRepo.On("GetWithRevision", mock.Anything, "past-meeting-123").Return(nil, uint64(0), errors.New("database error"))
			},
			wantErr:     true,
			expectedErr: domain.ErrInternal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, _, mockPastMeetingRepo, _ := setupPastMeetingServiceForTesting()

			// Test service not ready case
			if tt.name == "service not ready" {
				service.PastMeetingRepository = nil
			}

			if tt.setupMocks != nil {
				tt.setupMocks(mockPastMeetingRepo)
			}

			result, etag, err := service.GetPastMeeting(ctx, tt.uid)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.expectedErr != nil {
					assert.True(t, errors.Is(err, tt.expectedErr))
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
		name        string
		uid         string
		revision    uint64
		setupMocks  func(*mocks.MockPastMeetingRepository, *mocks.MockMessageBuilder, bool)
		skipEtag    bool
		wantErr     bool
		expectedErr error
	}{
		{
			name:     "successful delete",
			uid:      "past-meeting-123",
			revision: 42,
			setupMocks: func(mockRepo *mocks.MockPastMeetingRepository, mockBuilder *mocks.MockMessageBuilder, skipEtag bool) {
				mockRepo.On("Exists", mock.Anything, "past-meeting-123").Return(true, nil)
				mockRepo.On("Delete", mock.Anything, "past-meeting-123", uint64(42)).Return(nil)

				// Messages sent after successful deletion
				mockBuilder.On("SendDeleteIndexPastMeeting", mock.Anything, "past-meeting-123").Return(nil)
				mockBuilder.On("SendDeleteAllAccessPastMeeting", mock.Anything, "past-meeting-123").Return(nil)
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
			wantErr:     true,
			expectedErr: domain.ErrServiceUnavailable,
		},
		{
			name:     "empty UID",
			uid:      "",
			revision: 42,
			setupMocks: func(mockRepo *mocks.MockPastMeetingRepository, mockBuilder *mocks.MockMessageBuilder, skipEtag bool) {
				mockRepo.On("Exists", mock.Anything, "").Return(false, nil)
			},
			wantErr:     true,
			expectedErr: domain.ErrPastMeetingNotFound,
		},
		{
			name:     "past meeting not found",
			uid:      "past-meeting-123",
			revision: 42,
			setupMocks: func(mockRepo *mocks.MockPastMeetingRepository, mockBuilder *mocks.MockMessageBuilder, skipEtag bool) {
				mockRepo.On("Exists", mock.Anything, "past-meeting-123").Return(false, nil)
			},
			wantErr:     true,
			expectedErr: domain.ErrPastMeetingNotFound,
		},
		{
			name:     "revision mismatch",
			uid:      "past-meeting-123",
			revision: 42,
			setupMocks: func(mockRepo *mocks.MockPastMeetingRepository, mockBuilder *mocks.MockMessageBuilder, skipEtag bool) {
				mockRepo.On("Exists", mock.Anything, "past-meeting-123").Return(true, nil)
				mockRepo.On("Delete", mock.Anything, "past-meeting-123", uint64(42)).Return(domain.ErrRevisionMismatch)
			},
			wantErr:     true,
			expectedErr: domain.ErrRevisionMismatch,
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

				mockBuilder.On("SendDeleteIndexPastMeeting", mock.Anything, "past-meeting-123").Return(nil)
				mockBuilder.On("SendDeleteAllAccessPastMeeting", mock.Anything, "past-meeting-123").Return(nil)
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
				mockRepo.On("Delete", mock.Anything, "past-meeting-123", uint64(42)).Return(errors.New("database error"))
			},
			wantErr:     true,
			expectedErr: domain.ErrInternal,
		},
		{
			name:     "messaging failure doesn't fail operation",
			uid:      "past-meeting-123",
			revision: 42,
			setupMocks: func(mockRepo *mocks.MockPastMeetingRepository, mockBuilder *mocks.MockMessageBuilder, skipEtag bool) {
				mockRepo.On("Exists", mock.Anything, "past-meeting-123").Return(true, nil)
				mockRepo.On("Delete", mock.Anything, "past-meeting-123", uint64(42)).Return(nil)

				// Messaging fails but operation succeeds
				mockBuilder.On("SendDeleteIndexPastMeeting", mock.Anything, "past-meeting-123").Return(errors.New("messaging error"))
				mockBuilder.On("SendDeleteAllAccessPastMeeting", mock.Anything, "past-meeting-123").Return(errors.New("messaging error"))
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, _, mockPastMeetingRepo, mockBuilder := setupPastMeetingServiceForTesting()

			if tt.skipEtag {
				service.Config.SkipEtagValidation = true
			}

			// Test service not ready case
			if tt.name == "service not ready" {
				service.PastMeetingRepository = nil
			}

			if tt.setupMocks != nil {
				tt.setupMocks(mockPastMeetingRepo, mockBuilder, tt.skipEtag)
			}

			err := service.DeletePastMeeting(ctx, tt.uid, tt.revision)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.expectedErr != nil {
					assert.True(t, errors.Is(err, tt.expectedErr))
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
	scheduledStartTime := now.Add(time.Hour).Format(time.RFC3339)
	scheduledEndTime := now.Add(2 * time.Hour).Format(time.RFC3339)

	tests := []struct {
		name        string
		payload     *models.PastMeeting
		wantErr     bool
		expectedErr error
	}{
		{
			name: "valid payload",
			payload: &models.PastMeeting{
				MeetingUID:         "meeting-123",
				ProjectUID:         "project-123",
				ScheduledStartTime: mustParseTime(scheduledStartTime),
				ScheduledEndTime:   mustParseTime(scheduledEndTime),
				Title:              "Test Meeting",
				Description:        "Test Description",
				Platform:           "Zoom",
			},
			wantErr: false,
		},
		{
			name:        "nil payload",
			payload:     nil,
			wantErr:     true,
			expectedErr: domain.ErrValidationFailed,
		},
		{
			name: "empty meeting UID",
			payload: &models.PastMeeting{
				MeetingUID:         "",
				ProjectUID:         "project-123",
				ScheduledStartTime: mustParseTime(scheduledStartTime),
				ScheduledEndTime:   mustParseTime(scheduledEndTime),
				Title:              "Test Meeting",
				Description:        "Test Description",
				Platform:           "Zoom",
			},
			wantErr:     true,
			expectedErr: domain.ErrValidationFailed,
		},
		{
			name: "empty project UID",
			payload: &models.PastMeeting{
				MeetingUID:         "meeting-123",
				ProjectUID:         "",
				ScheduledStartTime: mustParseTime(scheduledStartTime),
				ScheduledEndTime:   mustParseTime(scheduledEndTime),
				Title:              "Test Meeting",
				Description:        "Test Description",
				Platform:           "Zoom",
			},
			wantErr:     true,
			expectedErr: domain.ErrValidationFailed,
		},
		{
			name: "empty title",
			payload: &models.PastMeeting{
				MeetingUID:         "meeting-123",
				ProjectUID:         "project-123",
				ScheduledStartTime: mustParseTime(scheduledStartTime),
				ScheduledEndTime:   mustParseTime(scheduledEndTime),
				Title:              "",
				Description:        "Test Description",
				Platform:           "Zoom",
			},
			wantErr:     true,
			expectedErr: domain.ErrValidationFailed,
		},
		{
			name: "empty description",
			payload: &models.PastMeeting{
				MeetingUID:         "meeting-123",
				ProjectUID:         "project-123",
				ScheduledStartTime: mustParseTime(scheduledStartTime),
				ScheduledEndTime:   mustParseTime(scheduledEndTime),
				Title:              "Test Meeting",
				Description:        "",
				Platform:           "Zoom",
			},
			wantErr:     true,
			expectedErr: domain.ErrValidationFailed,
		},
		{
			name: "empty platform",
			payload: &models.PastMeeting{
				MeetingUID:         "meeting-123",
				ProjectUID:         "project-123",
				ScheduledStartTime: mustParseTime(scheduledStartTime),
				ScheduledEndTime:   mustParseTime(scheduledEndTime),
				Title:              "Test Meeting",
				Description:        "Test Description",
				Platform:           "",
			},
			wantErr:     true,
			expectedErr: domain.ErrValidationFailed,
		},
		{
			name: "end time before start time",
			payload: &models.PastMeeting{
				MeetingUID:         "meeting-123",
				ProjectUID:         "project-123",
				ScheduledStartTime: mustParseTime(scheduledEndTime),
				ScheduledEndTime:   mustParseTime(scheduledStartTime),
				Title:              "Test Meeting",
				Description:        "Test Description",
				Platform:           "Zoom",
			},
			wantErr:     true,
			expectedErr: domain.ErrValidationFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, _, _, _ := setupPastMeetingServiceForTesting()

			err := service.validateCreatePastMeetingPayload(ctx, tt.payload)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.expectedErr != nil {
					assert.True(t, errors.Is(err, tt.expectedErr))
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
