// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package handlers

import (
	"context"
	"testing"
	"time"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/mocks"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/infrastructure/platform"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// setupHandlerForTesting creates a MeetingHandler with all mock dependencies for testing
func setupHandlerForTesting() (*MeetingHandler, *mocks.MockMeetingRepository, *mocks.MockRegistrantRepository, *mocks.MockMessageBuilder, *mocks.MockEmailService) {
	mockMeetingRepo := new(mocks.MockMeetingRepository)
	mockRegistrantRepo := new(mocks.MockRegistrantRepository)
	mockMessageBuilder := new(mocks.MockMessageBuilder)
	mockEmailService := new(mocks.MockEmailService)
	mockPlatformRegistry := platform.NewRegistry()

	config := service.ServiceConfig{
		SkipEtagValidation: false,
	}

	occurrenceService := service.NewOccurrenceService()
	meetingService := &service.MeetingService{
		MeetingRepository: mockMeetingRepo,
		MessageBuilder:    mockMessageBuilder,
		PlatformRegistry:  mockPlatformRegistry,
		OccurrenceService: occurrenceService,
		Config:            config,
	}

	registrantService := &service.MeetingRegistrantService{
		MeetingRepository:    mockMeetingRepo,
		RegistrantRepository: mockRegistrantRepo,
		MessageBuilder:       mockMessageBuilder,
		EmailService:         mockEmailService,
		OccurrenceService:    occurrenceService,
		Config:               config,
	}

	// Create a committee sync service for testing
	committeeSyncService := service.NewCommitteeSyncService(
		mockMeetingRepo,
		mockRegistrantRepo,
		registrantService, // registrant service is needed for ServiceReady check
		mockMessageBuilder,
	)

	// For now, using nil for services that aren't tested in this file
	handler := NewMeetingHandler(
		meetingService,
		registrantService,
		nil, // pastMeetingService
		nil, // pastMeetingParticipantService
		committeeSyncService,
	)

	return handler, mockMeetingRepo, mockRegistrantRepo, mockMessageBuilder, mockEmailService
}

func TestMeetingHandler_HandleMessage(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		subject     string
		messageData []byte
		setupMocks  func(*mocks.MockMeetingRepository, *mocks.MockRegistrantRepository, *mocks.MockMessageBuilder, *mocks.MockEmailService)
		expectReply bool
	}{
		{
			name:        "handle meeting get title message",
			subject:     models.MeetingGetTitleSubject,
			messageData: []byte("01234567-89ab-cdef-0123-456789abcdef"),
			setupMocks: func(mockMeetingRepo *mocks.MockMeetingRepository, mockRegistrantRepo *mocks.MockRegistrantRepository, mockBuilder *mocks.MockMessageBuilder, mockEmailService *mocks.MockEmailService) {
				now := time.Now()
				mockMeetingRepo.On("GetBaseWithRevision", mock.Anything, "01234567-89ab-cdef-0123-456789abcdef").Return(
					&models.MeetingBase{
						UID:       "01234567-89ab-cdef-0123-456789abcdef",
						Title:     "Test Meeting",
						CreatedAt: &now,
						UpdatedAt: &now,
					},
					uint64(1),
					nil,
				)
			},
			expectReply: true,
		},
		{
			name:        "handle meeting deleted message",
			subject:     models.MeetingDeletedSubject,
			messageData: []byte(`{"meeting_uid":"meeting-to-delete","meeting":{"uid":"meeting-to-delete","project_uid":"project-123","title":"Meeting to Delete","start_time":"2023-12-01T10:00:00Z","duration":60,"timezone":"UTC","description":"Meeting to be deleted"}}`),
			setupMocks: func(mockMeetingRepo *mocks.MockMeetingRepository, mockRegistrantRepo *mocks.MockRegistrantRepository, mockBuilder *mocks.MockMessageBuilder, mockEmailService *mocks.MockEmailService) {
				now := time.Now()
				// Setup registrants for deletion
				registrants := []*models.Registrant{
					{
						UID:        "registrant-1",
						MeetingUID: "meeting-to-delete",
						Username:   "user1",
						Email:      "user1@example.com",
						Host:       false,
					},
				}
				// Mock for GetBase call in ListMeetingRegistrants service method
				mockMeetingRepo.On("GetBase", mock.Anything, "meeting-to-delete").Return(&models.MeetingBase{UID: "meeting-to-delete"}, nil)
				mockRegistrantRepo.On("ListByMeeting", mock.Anything, "meeting-to-delete").Return(registrants, nil)
				mockRegistrantRepo.On("Delete", mock.Anything, "registrant-1", uint64(0)).Return(nil)
				mockBuilder.On("SendDeleteIndexMeetingRegistrant", mock.Anything, "registrant-1").Return(nil)
				mockBuilder.On("SendRemoveMeetingRegistrantAccess", mock.Anything, mock.MatchedBy(func(msg models.MeetingRegistrantAccessMessage) bool {
					return msg.UID == "registrant-1"
				})).Return(nil)
				// Mock GetBase for cancellation email (called in goroutine)
				mockMeetingRepo.On("GetBaseWithRevision", mock.Anything, "meeting-to-delete").Return(&models.MeetingBase{
					UID:         "meeting-to-delete",
					ProjectUID:  "project-123",
					Title:       "Test Meeting",
					StartTime:   now,
					Duration:    60,
					Timezone:    "UTC",
					Description: "Test meeting description",
				}, uint64(0), nil).Maybe() // Maybe() because it's called in a goroutine
				// Mock GetProjectName for cancellation email (called in goroutine)
				mockBuilder.On("GetProjectName", mock.Anything, "project-123").Return("Test Project", nil).Maybe()
				// Mock email service for cancellation
				mockEmailService.On("SendRegistrantCancellation", mock.Anything, mock.AnythingOfType("domain.EmailCancellation")).Return(nil).Maybe()
			},
			expectReply: false,
		},
		{
			name:        "unknown subject",
			subject:     "unknown.subject",
			messageData: []byte(`{}`),
			setupMocks: func(mockMeetingRepo *mocks.MockMeetingRepository, mockRegistrantRepo *mocks.MockRegistrantRepository, mockBuilder *mocks.MockMessageBuilder, mockEmailService *mocks.MockEmailService) {
				// No mock calls expected
			},
			expectReply: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, mockMeetingRepo, mockRegistrantRepo, mockBuilder, mockEmailService := setupHandlerForTesting()

			tt.setupMocks(mockMeetingRepo, mockRegistrantRepo, mockBuilder, mockEmailService)

			// Create mock message
			mockMsg := mocks.NewMockMessageWithReply(tt.messageData, tt.subject, tt.expectReply)
			if tt.expectReply {
				mockMsg.On("Respond", mock.Anything).Return(nil)
			}

			// Call HandleMessage
			handler.HandleMessage(ctx, mockMsg)

			// Give goroutines a chance to complete
			time.Sleep(100 * time.Millisecond)

			// Verify expectations
			if tt.expectReply {
				mockMsg.AssertExpectations(t)
			}
			mockMeetingRepo.AssertExpectations(t)
			mockRegistrantRepo.AssertExpectations(t)
			mockBuilder.AssertExpectations(t)
			// Don't assert email service for async operations
		})
	}
}

func TestMeetingHandler_HandleGetTitleMessage(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		messageData   []byte
		setupMocks    func(*mocks.MockMeetingRepository)
		expectError   bool
		expectedReply []byte
	}{
		// Success cases
		{
			name:        "successfully retrieve meeting title",
			messageData: []byte("01234567-89ab-cdef-0123-456789abcdef"),
			setupMocks: func(mockRepo *mocks.MockMeetingRepository) {
				now := time.Now()
				mockRepo.On("GetBaseWithRevision", mock.Anything, "01234567-89ab-cdef-0123-456789abcdef").Return(
					&models.MeetingBase{
						UID:       "01234567-89ab-cdef-0123-456789abcdef",
						Title:     "Important Team Meeting",
						CreatedAt: &now,
						UpdatedAt: &now,
					},
					uint64(1),
					nil,
				)
			},
			expectError:   false,
			expectedReply: []byte("Important Team Meeting"),
		},
		{
			name:        "successfully retrieve meeting with special characters in title",
			messageData: []byte("11111111-2222-3333-4444-555555555555"),
			setupMocks: func(mockRepo *mocks.MockMeetingRepository) {
				now := time.Now()
				mockRepo.On("GetBaseWithRevision", mock.Anything, "11111111-2222-3333-4444-555555555555").Return(
					&models.MeetingBase{
						UID:       "11111111-2222-3333-4444-555555555555",
						Title:     "Meeting: Q1 Review & Planning (Team A)",
						CreatedAt: &now,
						UpdatedAt: &now,
					},
					uint64(1),
					nil,
				)
			},
			expectError:   false,
			expectedReply: []byte("Meeting: Q1 Review & Planning (Team A)"),
		},
		// Error cases
		{
			name:        "repository error",
			messageData: []byte("01234567-89ab-cdef-0123-456789abcdef"),
			setupMocks: func(mockRepo *mocks.MockMeetingRepository) {
				mockRepo.On("GetBaseWithRevision", mock.Anything, "01234567-89ab-cdef-0123-456789abcdef").Return(
					nil, uint64(0), domain.NewInternalError("internal error"),
				)
			},
			expectError:   true,
			expectedReply: nil,
		},
		{
			name:        "meeting not found",
			messageData: []byte("00000000-0000-0000-0000-000000000000"),
			setupMocks: func(mockRepo *mocks.MockMeetingRepository) {
				mockRepo.On("GetBaseWithRevision", mock.Anything, "00000000-0000-0000-0000-000000000000").Return(
					nil, uint64(0), domain.NewNotFoundError("meeting not found"),
				)
			},
			expectError:   true,
			expectedReply: nil,
		},
		{
			name:        "invalid meeting UID",
			messageData: []byte(""),
			setupMocks: func(mockRepo *mocks.MockMeetingRepository) {
				// No repository call expected - validation fails before reaching repository
			},
			expectError:   true,
			expectedReply: nil,
		},
		{
			name:        "invalid UUID format",
			messageData: []byte("not-a-valid-uuid"),
			setupMocks: func(mockRepo *mocks.MockMeetingRepository) {
				// No repository call expected - validation fails before reaching repository
			},
			expectError:   true,
			expectedReply: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, mockMeetingRepo, _, _, _ := setupHandlerForTesting()

			tt.setupMocks(mockMeetingRepo)

			// Create mock message with reply
			mockMsg := mocks.NewMockMessageWithReply(tt.messageData, models.MeetingGetTitleSubject, true)

			if tt.expectError {
				// Handler responds with nil on error
				mockMsg.On("Respond", mock.Anything).Return(nil)
			} else {
				// Handler responds with the expected reply on success
				mockMsg.On("Respond", tt.expectedReply).Return(nil)
			}

			// Call HandleMessage
			handler.HandleMessage(ctx, mockMsg)

			// Verify expectations
			mockMsg.AssertExpectations(t)
			mockMeetingRepo.AssertExpectations(t)
		})
	}
}

func TestMeetingHandler_HandleMeetingDeletedMessage(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		messageData []byte
		setupMocks  func(*mocks.MockMeetingRepository, *mocks.MockRegistrantRepository, *mocks.MockMessageBuilder, *mocks.MockEmailService)
		expectError bool
	}{
		// Success cases
		{
			name:        "successfully delete single registrant",
			messageData: []byte(`{"meeting_uid":"meeting-123","meeting":{"uid":"meeting-123","project_uid":"project-123","title":"Test Meeting","start_time":"2023-12-01T10:00:00Z","duration":60,"timezone":"UTC","description":"Test meeting description"}}`),
			setupMocks: func(mockMeetingRepo *mocks.MockMeetingRepository, mockRegistrantRepo *mocks.MockRegistrantRepository, mockBuilder *mocks.MockMessageBuilder, mockEmailService *mocks.MockEmailService) {
				now := time.Now()
				registrants := []*models.Registrant{
					{
						UID:        "registrant-1",
						MeetingUID: "meeting-123",
						Username:   "user1",
						Email:      "user1@example.com",
						Host:       false,
					},
				}
				// Mock for GetBase call in ListMeetingRegistrants service method
				mockMeetingRepo.On("GetBase", mock.Anything, "meeting-123").Return(&models.MeetingBase{UID: "meeting-123"}, nil)
				mockRegistrantRepo.On("ListByMeeting", mock.Anything, "meeting-123").Return(registrants, nil)
				mockRegistrantRepo.On("Delete", mock.Anything, "registrant-1", uint64(0)).Return(nil)
				mockBuilder.On("SendDeleteIndexMeetingRegistrant", mock.Anything, "registrant-1").Return(nil)
				mockBuilder.On("SendRemoveMeetingRegistrantAccess", mock.Anything, mock.MatchedBy(func(msg models.MeetingRegistrantAccessMessage) bool {
					return msg.UID == "registrant-1"
				})).Return(nil)
				// Mock for cancellation email (called in goroutine)
				mockMeetingRepo.On("GetBaseWithRevision", mock.Anything, "meeting-123").Return(&models.MeetingBase{
					UID:         "meeting-123",
					ProjectUID:  "project-123",
					Title:       "Test Meeting",
					StartTime:   now,
					Duration:    60,
					Timezone:    "UTC",
					Description: "Test meeting description",
				}, uint64(0), nil).Maybe()
				mockBuilder.On("GetProjectName", mock.Anything, "project-123").Return("Test Project", nil).Maybe()
				mockEmailService.On("SendRegistrantCancellation", mock.Anything, mock.AnythingOfType("domain.EmailCancellation")).Return(nil).Maybe()
			},
			expectError: false,
		},
		{
			name:        "successfully delete multiple registrants",
			messageData: []byte(`{"meeting_uid":"meeting-456","meeting":{"uid":"meeting-456","project_uid":"project-456","title":"Test Meeting 2","start_time":"2023-12-01T11:00:00Z","duration":60,"timezone":"UTC","description":"Test meeting 2 description"}}`),
			setupMocks: func(mockMeetingRepo *mocks.MockMeetingRepository, mockRegistrantRepo *mocks.MockRegistrantRepository, mockBuilder *mocks.MockMessageBuilder, mockEmailService *mocks.MockEmailService) {
				now := time.Now()
				registrants := []*models.Registrant{
					{
						UID:        "registrant-1",
						MeetingUID: "meeting-456",
						Username:   "user1",
						Email:      "user1@example.com",
						Host:       false,
					},
					{
						UID:        "registrant-2",
						MeetingUID: "meeting-456",
						Username:   "user2",
						Email:      "user2@example.com",
						Host:       false,
					},
				}
				// Mock for GetBase call in ListMeetingRegistrants service method
				mockMeetingRepo.On("GetBase", mock.Anything, "meeting-456").Return(&models.MeetingBase{UID: "meeting-456"}, nil)
				mockRegistrantRepo.On("ListByMeeting", mock.Anything, "meeting-456").Return(registrants, nil)
				mockRegistrantRepo.On("Delete", mock.Anything, "registrant-1", uint64(0)).Return(nil)
				mockRegistrantRepo.On("Delete", mock.Anything, "registrant-2", uint64(0)).Return(nil)
				mockBuilder.On("SendDeleteIndexMeetingRegistrant", mock.Anything, "registrant-1").Return(nil)
				mockBuilder.On("SendDeleteIndexMeetingRegistrant", mock.Anything, "registrant-2").Return(nil)
				mockBuilder.On("SendRemoveMeetingRegistrantAccess", mock.Anything, mock.MatchedBy(func(msg models.MeetingRegistrantAccessMessage) bool {
					return msg.UID == "registrant-1" || msg.UID == "registrant-2"
				})).Return(nil).Times(2)
				// Mock for cancellation emails (called in goroutines)
				mockMeetingRepo.On("GetBaseWithRevision", mock.Anything, "meeting-456").Return(&models.MeetingBase{
					UID:         "meeting-456",
					ProjectUID:  "project-456",
					Title:       "Team Sync",
					StartTime:   now,
					Duration:    30,
					Timezone:    "America/New_York",
					Description: "Weekly team sync",
				}, uint64(0), nil).Maybe()
				mockBuilder.On("GetProjectName", mock.Anything, "project-456").Return("Team Project", nil).Maybe()
				mockEmailService.On("SendRegistrantCancellation", mock.Anything, mock.AnythingOfType("domain.EmailCancellation")).Return(nil).Maybe()
			},
			expectError: false,
		},
		{
			name:        "successfully handle meeting with no registrants",
			messageData: []byte(`{"meeting_uid":"meeting-789","meeting":{"uid":"meeting-789","project_uid":"project-789","title":"Test Meeting 3","start_time":"2023-12-01T12:00:00Z","duration":60,"timezone":"UTC","description":"Test meeting 3 description"}}`),
			setupMocks: func(mockMeetingRepo *mocks.MockMeetingRepository, mockRegistrantRepo *mocks.MockRegistrantRepository, mockBuilder *mocks.MockMessageBuilder, mockEmailService *mocks.MockEmailService) {
				// Mock for GetBase call in ListMeetingRegistrants service method
				mockMeetingRepo.On("GetBase", mock.Anything, "meeting-789").Return(&models.MeetingBase{UID: "meeting-789"}, nil)
				mockRegistrantRepo.On("ListByMeeting", mock.Anything, "meeting-789").Return([]*models.Registrant{}, nil)
				// No further mocks needed - no registrants to delete
			},
			expectError: false,
		},
		// Error cases
		{
			name:        "invalid JSON",
			messageData: []byte(`{invalid json}`),
			setupMocks: func(*mocks.MockMeetingRepository, *mocks.MockRegistrantRepository, *mocks.MockMessageBuilder, *mocks.MockEmailService) {
			},
			expectError: true,
		},
		{
			name:        "empty meeting UID",
			messageData: []byte(`{"meeting_uid":"","meeting":{"uid":"","project_uid":"project-123","title":"Test Meeting","start_time":"2023-12-01T10:00:00Z","duration":60,"timezone":"UTC","description":"Test meeting description"}}`),
			setupMocks: func(*mocks.MockMeetingRepository, *mocks.MockRegistrantRepository, *mocks.MockMessageBuilder, *mocks.MockEmailService) {
			},
			expectError: true,
		},
		{
			name:        "missing meeting object",
			messageData: []byte(`{"meeting_uid":"meeting-123"}`),
			setupMocks: func(*mocks.MockMeetingRepository, *mocks.MockRegistrantRepository, *mocks.MockMessageBuilder, *mocks.MockEmailService) {
			},
			expectError: true,
		},
		{
			name:        "repository error when listing registrants",
			messageData: []byte(`{"meeting_uid":"meeting-error","meeting":{"uid":"meeting-error","project_uid":"project-error","title":"Error Meeting","start_time":"2023-12-01T10:00:00Z","duration":60,"timezone":"UTC","description":"Error meeting description"}}`),
			setupMocks: func(mockMeetingRepo *mocks.MockMeetingRepository, mockRegistrantRepo *mocks.MockRegistrantRepository, mockBuilder *mocks.MockMessageBuilder, mockEmailService *mocks.MockEmailService) {
				// Mock for GetBase call in ListMeetingRegistrants service method
				mockMeetingRepo.On("GetBase", mock.Anything, "meeting-error").Return(&models.MeetingBase{UID: "meeting-error"}, nil)
				mockRegistrantRepo.On("ListByMeeting", mock.Anything, "meeting-error").Return(
					nil, domain.NewInternalError("internal error"),
				)
			},
			expectError: true,
		},
		{
			name:        "partial deletion failure returns error",
			messageData: []byte(`{"meeting_uid":"meeting-partial","meeting":{"uid":"meeting-partial","project_uid":"project-partial","title":"Partial Meeting","start_time":"2023-12-01T10:00:00Z","duration":60,"timezone":"UTC","description":"Partial meeting description"}}`),
			setupMocks: func(mockMeetingRepo *mocks.MockMeetingRepository, mockRegistrantRepo *mocks.MockRegistrantRepository, mockBuilder *mocks.MockMessageBuilder, mockEmailService *mocks.MockEmailService) {
				registrants := []*models.Registrant{
					{
						UID:        "registrant-1",
						MeetingUID: "meeting-partial",
						Username:   "user1",
						Email:      "user1@example.com",
					},
					{
						UID:        "registrant-2",
						MeetingUID: "meeting-partial",
						Username:   "user2",
						Email:      "user2@example.com",
					},
				}
				// Mock for GetBase call in ListMeetingRegistrants service method
				mockMeetingRepo.On("GetBase", mock.Anything, "meeting-partial").Return(&models.MeetingBase{UID: "meeting-partial"}, nil)
				mockRegistrantRepo.On("ListByMeeting", mock.Anything, "meeting-partial").Return(registrants, nil)
				// Both deletions may be attempted concurrently, but at least one will fail
				mockRegistrantRepo.On("Delete", mock.Anything, "registrant-1", uint64(0)).Return(nil).Maybe()
				mockRegistrantRepo.On("Delete", mock.Anything, "registrant-2", uint64(0)).Return(domain.NewInternalError("internal error"))
				// Messaging calls may or may not happen due to concurrent execution and fail-fast behavior
				mockBuilder.On("SendDeleteIndexMeetingRegistrant", mock.Anything, "registrant-1").Return(nil).Maybe()
				mockBuilder.On("SendRemoveMeetingRegistrantAccess", mock.Anything, mock.MatchedBy(func(msg models.MeetingRegistrantAccessMessage) bool {
					return msg.UID == "registrant-1"
				})).Return(nil).Maybe()
				// Email sending might be attempted for successful deletion
				mockMeetingRepo.On("GetBaseWithRevision", mock.Anything, "meeting-partial").Return(&models.MeetingBase{
					UID:        "meeting-partial",
					ProjectUID: "project-partial",
					Title:      "Test",
				}, uint64(0), nil).Maybe()
				mockBuilder.On("GetProjectName", mock.Anything, "project-partial").Return("Test Project", nil).Maybe()
				mockEmailService.On("SendRegistrantCancellation", mock.Anything, mock.AnythingOfType("domain.EmailCancellation")).Return(nil).Maybe()
			},
			expectError: true, // Handler fails when any deletion fails due to WorkerPool fail-fast behavior
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, mockMeetingRepo, mockRegistrantRepo, mockBuilder, mockEmailService := setupHandlerForTesting()

			tt.setupMocks(mockMeetingRepo, mockRegistrantRepo, mockBuilder, mockEmailService)

			// Create mock message (no reply expected for deletion messages)
			mockMsg := mocks.NewMockMessage(tt.messageData, models.MeetingDeletedSubject)

			// Call HandleMessage - should handle errors gracefully
			if tt.expectError {
				// Even with errors, handler shouldn't panic
				assert.NotPanics(t, func() {
					handler.HandleMessage(ctx, mockMsg)
				})
			} else {
				handler.HandleMessage(ctx, mockMsg)
			}

			// Give goroutines a chance to complete
			time.Sleep(100 * time.Millisecond)

			// Verify expectations
			mockMeetingRepo.AssertExpectations(t)
			mockRegistrantRepo.AssertExpectations(t)
			mockBuilder.AssertExpectations(t)
			// Don't assert email service for async operations
		})
	}
}

func TestMeetingHandler_HandleMeetingUpdatedMessage(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		messageData []byte
		setupMocks  func(mockMeetingRepo *mocks.MockMeetingRepository, mockRegistrantRepo *mocks.MockRegistrantRepository, mockBuilder *mocks.MockMessageBuilder, mockEmailService *mocks.MockEmailService)
		expectError bool
	}{
		{
			name:        "successfully send update notifications to registrants",
			messageData: []byte(`{"meeting_uid":"meeting-updated","changes":{"Title":"New Meeting Title","Start Time":"2024-01-01 10:00:00 UTC"}}`),
			setupMocks: func(mockMeetingRepo *mocks.MockMeetingRepository, mockRegistrantRepo *mocks.MockRegistrantRepository, mockBuilder *mocks.MockMessageBuilder, mockEmailService *mocks.MockEmailService) {
				registrants := []*models.Registrant{
					{
						UID:        "registrant-1",
						MeetingUID: "meeting-updated",
						Email:      "user1@example.com",
						FirstName:  "John",
						LastName:   "Doe",
					},
					{
						UID:        "registrant-2",
						MeetingUID: "meeting-updated",
						Email:      "user2@example.com",
						FirstName:  "Jane",
						LastName:   "Smith",
					},
				}
				// Mock for ListMeetingRegistrants service method
				mockMeetingRepo.On("GetBase", mock.Anything, "meeting-updated").Return(&models.MeetingBase{UID: "meeting-updated"}, nil)
				mockRegistrantRepo.On("ListByMeeting", mock.Anything, "meeting-updated").Return(registrants, nil)

				// Mock meeting retrieval for each registrant
				meeting := &models.MeetingBase{
					UID:       "meeting-updated",
					Title:     "New Meeting Title",
					StartTime: time.Now().Add(24 * time.Hour),
					Duration:  60,
					Timezone:  "UTC",
					JoinURL:   "https://zoom.us/j/123456789",
					ZoomConfig: &models.ZoomConfig{
						MeetingID: "123456789",
						Passcode:  "secret",
					},
				}
				mockMeetingRepo.On("GetBaseWithRevision", mock.Anything, "meeting-updated").Return(meeting, uint64(0), nil)

				// Mock GetProjectName for email notifications
				mockBuilder.On("GetProjectName", mock.Anything, mock.AnythingOfType("string")).Return("Test Project", nil)

				// Expect email notifications to be sent
				mockEmailService.On("SendRegistrantUpdatedInvitation", mock.Anything, mock.MatchedBy(func(invitation domain.EmailUpdatedInvitation) bool {
					return invitation.MeetingUID == "meeting-updated" &&
						len(invitation.Changes) == 2 &&
						(invitation.RecipientEmail == "user1@example.com" || invitation.RecipientEmail == "user2@example.com")
				})).Return(nil).Times(2)
			},
			expectError: false,
		},
		{
			name:        "successfully handle meeting with no registrants",
			messageData: []byte(`{"meeting_uid":"meeting-no-registrants","changes":{"Title":"Updated Title"}}`),
			setupMocks: func(mockMeetingRepo *mocks.MockMeetingRepository, mockRegistrantRepo *mocks.MockRegistrantRepository, mockBuilder *mocks.MockMessageBuilder, mockEmailService *mocks.MockEmailService) {
				// Mock for ListMeetingRegistrants service method
				mockMeetingRepo.On("GetBase", mock.Anything, "meeting-no-registrants").Return(&models.MeetingBase{UID: "meeting-no-registrants"}, nil)
				mockRegistrantRepo.On("ListByMeeting", mock.Anything, "meeting-no-registrants").Return([]*models.Registrant{}, nil)
			},
			expectError: false,
		},
		{
			name:        "successfully handle meeting with no meaningful changes",
			messageData: []byte(`{"meeting_uid":"meeting-no-changes","changes":{}}`),
			setupMocks: func(mockMeetingRepo *mocks.MockMeetingRepository, mockRegistrantRepo *mocks.MockRegistrantRepository, mockBuilder *mocks.MockMessageBuilder, mockEmailService *mocks.MockEmailService) {
				// No mock expectations since no processing should occur
			},
			expectError: false,
		},
		{
			name:        "handle invalid JSON gracefully",
			messageData: []byte(`invalid json`),
			setupMocks: func(mockMeetingRepo *mocks.MockMeetingRepository, mockRegistrantRepo *mocks.MockRegistrantRepository, mockBuilder *mocks.MockMessageBuilder, mockEmailService *mocks.MockEmailService) {
				// No mock expectations
			},
			expectError: true,
		},
		{
			name:        "handle empty meeting UID",
			messageData: []byte(`{"meeting_uid":"","changes":{"Title":"Updated"}}`),
			setupMocks: func(mockMeetingRepo *mocks.MockMeetingRepository, mockRegistrantRepo *mocks.MockRegistrantRepository, mockBuilder *mocks.MockMessageBuilder, mockEmailService *mocks.MockEmailService) {
				// No mock expectations
			},
			expectError: true,
		},
		{
			name:        "handle repository error when listing registrants",
			messageData: []byte(`{"meeting_uid":"meeting-repo-error","changes":{"Title":"Updated"}}`),
			setupMocks: func(mockMeetingRepo *mocks.MockMeetingRepository, mockRegistrantRepo *mocks.MockRegistrantRepository, mockBuilder *mocks.MockMessageBuilder, mockEmailService *mocks.MockEmailService) {
				// Mock for ListMeetingRegistrants service method
				mockMeetingRepo.On("GetBase", mock.Anything, "meeting-repo-error").Return(&models.MeetingBase{UID: "meeting-repo-error"}, nil)
				mockRegistrantRepo.On("ListByMeeting", mock.Anything, "meeting-repo-error").Return([]*models.Registrant{}, domain.NewInternalError("internal error"))
			},
			expectError: true,
		},
		{
			name:        "handle partial email notification failures",
			messageData: []byte(`{"meeting_uid":"meeting-partial-email-fail","changes":{"Duration":"120 minutes"}}`),
			setupMocks: func(mockMeetingRepo *mocks.MockMeetingRepository, mockRegistrantRepo *mocks.MockRegistrantRepository, mockBuilder *mocks.MockMessageBuilder, mockEmailService *mocks.MockEmailService) {
				registrants := []*models.Registrant{
					{
						UID:        "registrant-success",
						MeetingUID: "meeting-partial-email-fail",
						Email:      "success@example.com",
						FirstName:  "Success",
						LastName:   "User",
					},
					{
						UID:        "registrant-fail",
						MeetingUID: "meeting-partial-email-fail",
						Email:      "fail@example.com",
						FirstName:  "Fail",
						LastName:   "User",
					},
				}
				// Mock for ListMeetingRegistrants service method
				mockMeetingRepo.On("GetBase", mock.Anything, "meeting-partial-email-fail").Return(&models.MeetingBase{UID: "meeting-partial-email-fail"}, nil)
				mockRegistrantRepo.On("ListByMeeting", mock.Anything, "meeting-partial-email-fail").Return(registrants, nil)

				meeting := &models.MeetingBase{
					UID:       "meeting-partial-email-fail",
					Title:     "Test Meeting",
					StartTime: time.Now().Add(24 * time.Hour),
					Duration:  120,
					Timezone:  "UTC",
				}
				mockMeetingRepo.On("GetBaseWithRevision", mock.Anything, "meeting-partial-email-fail").Return(meeting, uint64(0), nil)

				// Mock GetProjectName for email notifications
				mockBuilder.On("GetProjectName", mock.Anything, mock.AnythingOfType("string")).Return("Test Project", nil)

				// First email succeeds, second fails
				mockEmailService.On("SendRegistrantUpdatedInvitation", mock.Anything, mock.MatchedBy(func(invitation domain.EmailUpdatedInvitation) bool {
					return invitation.RecipientEmail == "success@example.com"
				})).Return(nil).Maybe()

				mockEmailService.On("SendRegistrantUpdatedInvitation", mock.Anything, mock.MatchedBy(func(invitation domain.EmailUpdatedInvitation) bool {
					return invitation.RecipientEmail == "fail@example.com"
				})).Return(domain.NewInternalError("internal error")).Maybe()
			},
			expectError: true, // WorkerPool fails fast on errors
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, mockMeetingRepo, mockRegistrantRepo, mockBuilder, mockEmailService := setupHandlerForTesting()

			tt.setupMocks(mockMeetingRepo, mockRegistrantRepo, mockBuilder, mockEmailService)

			// Create mock message
			mockMsg := mocks.NewMockMessage(tt.messageData, models.MeetingUpdatedSubject)

			// Call the handler's HandleMessage which should route to HandleMeetingUpdated
			if tt.expectError {
				assert.NotPanics(t, func() {
					handler.HandleMessage(ctx, mockMsg)
				})
			} else {
				handler.HandleMessage(ctx, mockMsg)
			}

			// Give async operations time to complete
			time.Sleep(200 * time.Millisecond)

			// Verify expectations
			mockMeetingRepo.AssertExpectations(t)
			mockRegistrantRepo.AssertExpectations(t)
			mockBuilder.AssertExpectations(t)
			mockEmailService.AssertExpectations(t)
		})
	}
}
