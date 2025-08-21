// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/infrastructure/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestMeetingsService_HandleMessage(t *testing.T) {

	ctx := context.Background()

	tests := []struct {
		name         string
		subject      string
		messageData  []byte
		setupService func() *MeetingsService
		setupMocks   func(*domain.MockMeetingRepository, *domain.MockRegistrantRepository, *domain.MockMessageBuilder)
	}{
		{
			name:        "handle meeting get title message",
			subject:     models.MeetingGetTitleSubject,
			messageData: []byte("01234567-89ab-cdef-0123-456789abcdef"),
			setupMocks: func(mockMeetingRepo *domain.MockMeetingRepository, mockRegistrantRepo *domain.MockRegistrantRepository, mockBuilder *domain.MockMessageBuilder) {
				now := time.Now()
				mockMeetingRepo.On("GetBase", mock.Anything, "01234567-89ab-cdef-0123-456789abcdef").Return(
					&models.MeetingBase{
						UID:       "01234567-89ab-cdef-0123-456789abcdef",
						Title:     "Test Meeting",
						CreatedAt: &now,
						UpdatedAt: &now,
					},
					nil,
				)
			},
		},
		{
			name:        "handle meeting deleted message",
			subject:     models.MeetingDeletedSubject,
			messageData: []byte(`{"meeting_uid":"meeting-to-delete"}`),
			setupMocks: func(mockMeetingRepo *domain.MockMeetingRepository, mockRegistrantRepo *domain.MockRegistrantRepository, mockBuilder *domain.MockMessageBuilder) {
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
				mockRegistrantRepo.On("ListByMeeting", mock.Anything, "meeting-to-delete").Return(registrants, nil)
				mockRegistrantRepo.On("Delete", mock.Anything, "registrant-1", uint64(0)).Return(nil)
				mockBuilder.On("SendDeleteIndexMeetingRegistrant", mock.Anything, "registrant-1").Return(nil)
				mockBuilder.On("SendRemoveMeetingRegistrantAccess", mock.Anything, mock.MatchedBy(func(msg models.MeetingRegistrantAccessMessage) bool {
					return msg.UID == "registrant-1"
				})).Return(nil)
				// Mock GetBase for cancellation email (called in goroutine)
				mockMeetingRepo.On("GetBase", mock.Anything, "meeting-to-delete").Return(&models.MeetingBase{
					UID:         "meeting-to-delete",
					Title:       "Test Meeting",
					StartTime:   now,
					Duration:    60,
					Timezone:    "UTC",
					Description: "Test meeting description",
				}, nil)
			},
		},
		{
			name:        "unknown subject",
			subject:     "unknown.subject",
			messageData: []byte(`{}`),
			setupMocks: func(mockMeetingRepo *domain.MockMeetingRepository, mockRegistrantRepo *domain.MockRegistrantRepository, mockBuilder *domain.MockMessageBuilder) {
				// No mock calls expected
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var service *MeetingsService
			var mockMeetingRepo *domain.MockMeetingRepository
			var mockRegistrantRepo *domain.MockRegistrantRepository
			var mockBuilder *domain.MockMessageBuilder
			var mockAuth *auth.MockJWTAuth

			if tt.setupService != nil {
				service = tt.setupService()
				// Create mock repositories for setup function, even if not used
				mockMeetingRepo = &domain.MockMeetingRepository{}
				mockRegistrantRepo = &domain.MockRegistrantRepository{}
				mockBuilder = &domain.MockMessageBuilder{}
				mockAuth = &auth.MockJWTAuth{}
			} else {
				service, mockMeetingRepo, mockBuilder, mockAuth = setupServiceForTesting()
				mockRegistrantRepo = &domain.MockRegistrantRepository{}
				service.RegistrantRepository = mockRegistrantRepo
			}

			tt.setupMocks(mockMeetingRepo, mockRegistrantRepo, mockBuilder)

			// Add email service mocks if needed for meeting deletion
			if tt.subject == models.MeetingDeletedSubject {
				if mockEmailService, ok := service.EmailService.(*domain.MockEmailService); ok {
					// Add flexible email mock that accepts any cancellation email
					mockEmailService.On("SendRegistrantCancellation", mock.Anything, mock.AnythingOfType("domain.EmailCancellation")).Return(nil).Maybe()
				}
			}

			// Create mock message - meeting deletion messages don't expect replies
			var mockMsg *mockMessage
			if tt.subject == models.MeetingDeletedSubject {
				mockMsg = newMockMessageNoReply(tt.subject, tt.messageData)
			} else {
				mockMsg = newMockMessage(tt.subject, tt.messageData)
				mockMsg.On("Respond", mock.Anything).Return(nil)
			}

			// Call HandleMessage
			service.HandleMessage(ctx, mockMsg)

			// Verify expectations
			if tt.subject != models.MeetingDeletedSubject {
				mockMsg.AssertExpectations(t)
			}

			// Only assert expectations if service was properly set up
			if tt.setupService == nil {
				mockMeetingRepo.AssertExpectations(t)
				mockRegistrantRepo.AssertExpectations(t)
				mockBuilder.AssertExpectations(t)
				mockAuth.AssertExpectations(t)
			}
		})
	}

	// Error handling test cases
	t.Run("error cases", func(t *testing.T) {
		errorTests := []struct {
			name         string
			setupService func() *MeetingsService
			subject      string
			messageData  []byte
			description  string
		}{
			{
				name: "service not ready",
				setupService: func() *MeetingsService {
					return &MeetingsService{
						MeetingRepository:    nil,
						RegistrantRepository: nil,
						MessageBuilder:       nil,
						Auth:                 &auth.MockJWTAuth{},
					}
				},
				subject:     models.MeetingGetTitleSubject,
				messageData: []byte("01234567-89ab-cdef-0123-456789abcdef"),
				description: "should handle service not ready gracefully",
			},
			{
				name: "repository error",
				setupService: func() *MeetingsService {
					mockRepo := &domain.MockMeetingRepository{}
					mockRepo.On("GetBase", mock.Anything, "01234567-89ab-cdef-0123-456789abcdef").Return(
						nil, domain.ErrInternal,
					)

					return &MeetingsService{
						MeetingRepository:    mockRepo,
						RegistrantRepository: &domain.MockRegistrantRepository{},
						MessageBuilder:       &domain.MockMessageBuilder{},
						Auth:                 &auth.MockJWTAuth{},
						PlatformRegistry:     &domain.MockPlatformRegistry{},
					}
				},
				subject:     models.MeetingGetTitleSubject,
				messageData: []byte("01234567-89ab-cdef-0123-456789abcdef"),
				description: "should handle repository errors gracefully",
			},
		}

		for _, tt := range errorTests {
			t.Run(tt.name, func(t *testing.T) {
				service := tt.setupService()

				mockMsg := newMockMessage(tt.subject, tt.messageData)
				mockMsg.On("Respond", mock.Anything).Return(nil)

				// Should not panic
				assert.NotPanics(t, func() {
					service.HandleMessage(ctx, mockMsg)
				})

				if mockRepo, ok := service.MeetingRepository.(*domain.MockMeetingRepository); ok {
					mockRepo.AssertExpectations(t)
				}
			})
		}
	})

	// Integration test cases
	t.Run("integration tests", func(t *testing.T) {
		t.Run("end to end message handling", func(t *testing.T) {
			service, mockRepo, mockBuilder, mockAuth := setupServiceForTesting()

			// Setup expectations for a complete flow
			now := time.Now()
			mockRepo.On("GetBase", mock.Anything, "01234567-89ab-cdef-0123-456789abcdef").Return(
				&models.MeetingBase{
					UID:       "integration-test-uid",
					Title:     "Integration Test Meeting",
					CreatedAt: &now,
					UpdatedAt: &now,
				},
				nil,
			)

			// Create message and set up response expectation
			messageData := []byte("01234567-89ab-cdef-0123-456789abcdef")
			mockMsg := newMockMessage(models.MeetingGetTitleSubject, messageData)

			// Expect a response with the meeting title
			mockMsg.On("Respond", mock.MatchedBy(func(data []byte) bool {
				return string(data) == "Integration Test Meeting"
			})).Return(nil)

			// Execute
			service.HandleMessage(ctx, mockMsg)

			// Verify all expectations
			mockRepo.AssertExpectations(t)
			mockBuilder.AssertExpectations(t)
			mockAuth.AssertExpectations(t)
			mockMsg.AssertExpectations(t)
		})
	})
}

func TestMeetingsService_HandleMeetingGetTitle(t *testing.T) {

	ctx := context.Background()

	tests := []struct {
		name        string
		messageData []byte
		setupMocks  func(*domain.MockMeetingRepository)
		expectedErr bool
		validate    func(*testing.T, []byte)
	}{
		{
			name:        "successful get meeting title",
			messageData: []byte("01234567-89ab-cdef-0123-456789abcdef"),
			setupMocks: func(mockRepo *domain.MockMeetingRepository) {
				now := time.Now()
				mockRepo.On("GetBase", mock.Anything, "01234567-89ab-cdef-0123-456789abcdef").Return(
					&models.MeetingBase{
						UID:       "01234567-89ab-cdef-0123-456789abcdef",
						Title:     "Test Meeting Title",
						CreatedAt: &now,
						UpdatedAt: &now,
					},
					nil,
				)
			},
			expectedErr: false,
			validate: func(t *testing.T, response []byte) {
				assert.Equal(t, "Test Meeting Title", string(response))
			},
		},
		{
			name:        "meeting not found",
			messageData: []byte("01234567-89ab-cdef-0123-456789abcd00"),
			setupMocks: func(mockRepo *domain.MockMeetingRepository) {
				mockRepo.On("GetBase", mock.Anything, "01234567-89ab-cdef-0123-456789abcd00").Return(
					nil, domain.ErrMeetingNotFound,
				)
			},
			expectedErr: true,
		},
		{
			name:        "invalid JSON",
			messageData: []byte(`invalid-json`),
			setupMocks: func(mockRepo *domain.MockMeetingRepository) {
				// No repo calls expected
			},
			expectedErr: true,
		},
		{
			name:        "missing UID",
			messageData: []byte(`{}`),
			setupMocks: func(mockRepo *domain.MockMeetingRepository) {
				// No repo calls expected
			},
			expectedErr: true,
		},
		{
			name:        "empty UID",
			messageData: []byte(`{"uid": ""}`),
			setupMocks: func(mockRepo *domain.MockMeetingRepository) {
				// No repo calls expected
			},
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, mockRepo, _, _ := setupServiceForTesting()
			tt.setupMocks(mockRepo)

			mockMsg := newMockMessage(models.MeetingGetTitleSubject, tt.messageData)

			response, err := service.HandleMeetingGetTitle(ctx, mockMsg)

			if tt.expectedErr {
				assert.Error(t, err)
				assert.Nil(t, response)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, response)
				if tt.validate != nil {
					tt.validate(t, response)
				}
			}

			mockRepo.AssertExpectations(t)
		})
	}
}

func TestMeetingsService_HandleMeetingDeleted(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name           string
		messageData    []byte
		setupMocks     func(*domain.MockMeetingRepository, *domain.MockRegistrantRepository, *domain.MockMessageBuilder)
		setupService   func() *MeetingsService
		expectedErr    bool
		expectedResult string
		validate       func(*testing.T, []byte)
	}{
		{
			name:        "successful cleanup with multiple registrants",
			messageData: []byte(`{"meeting_uid":"meeting-123"}`),
			setupMocks: func(mockMeetingRepo *domain.MockMeetingRepository, mockRegistrantRepo *domain.MockRegistrantRepository, mockBuilder *domain.MockMessageBuilder) {
				now := time.Now()
				// Setup registrants to be cleaned up
				registrants := []*models.Registrant{
					{
						UID:        "registrant-1",
						MeetingUID: "meeting-123",
						Username:   "user1",
						Email:      "user1@example.com",
						Host:       false,
					},
					{
						UID:        "registrant-2",
						MeetingUID: "meeting-123",
						Username:   "user2",
						Email:      "user2@example.com",
						Host:       true,
					},
					{
						UID:        "registrant-3",
						MeetingUID: "meeting-123",
						Username:   "", // Guest user without username
						Email:      "guest@example.com",
						Host:       false,
					},
				}

				mockRegistrantRepo.On("ListByMeeting", mock.Anything, "meeting-123").Return(registrants, nil)

				// Setup expectations for each registrant deletion
				for _, reg := range registrants {
					mockRegistrantRepo.On("Delete", mock.Anything, reg.UID, uint64(0)).Return(nil)
					mockBuilder.On("SendDeleteIndexMeetingRegistrant", mock.Anything, reg.UID).Return(nil)

					// Only expect access messages for registrants with usernames
					if reg.Username != "" {
						mockBuilder.On("SendRemoveMeetingRegistrantAccess", mock.Anything, mock.MatchedBy(func(msg models.MeetingRegistrantAccessMessage) bool {
							return msg.UID == reg.UID && msg.Username == reg.Username && msg.MeetingUID == reg.MeetingUID && msg.Host == reg.Host
						})).Return(nil)
					}
				}

				// Mock GetBase for cancellation emails (called in goroutines)
				mockMeetingRepo.On("GetBase", mock.Anything, "meeting-123").Return(&models.MeetingBase{
					UID:         "meeting-123",
					Title:       "Test Meeting",
					StartTime:   now,
					Duration:    60,
					Timezone:    "UTC",
					Description: "Test meeting description",
				}, nil)
			},
			expectedErr: false,
			validate: func(t *testing.T, response []byte) {
				assert.Equal(t, "success", string(response))
			},
		},
		{
			name:        "successful cleanup with no registrants",
			messageData: []byte(`{"meeting_uid":"meeting-empty"}`),
			setupMocks: func(mockMeetingRepo *domain.MockMeetingRepository, mockRegistrantRepo *domain.MockRegistrantRepository, mockBuilder *domain.MockMessageBuilder) {
				mockRegistrantRepo.On("ListByMeeting", mock.Anything, "meeting-empty").Return([]*models.Registrant{}, nil)
			},
			expectedErr: false,
			validate: func(t *testing.T, response []byte) {
				assert.Equal(t, "success", string(response))
			},
		},
		{
			name:        "partial failure in registrant cleanup",
			messageData: []byte(`{"meeting_uid":"meeting-456"}`),
			setupMocks: func(mockMeetingRepo *domain.MockMeetingRepository, mockRegistrantRepo *domain.MockRegistrantRepository, mockBuilder *domain.MockMessageBuilder) {
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

				mockRegistrantRepo.On("ListByMeeting", mock.Anything, "meeting-456").Return(registrants, nil)

				// First registrant succeeds
				mockRegistrantRepo.On("Delete", mock.Anything, "registrant-1", uint64(0)).Return(nil)
				mockBuilder.On("SendDeleteIndexMeetingRegistrant", mock.Anything, "registrant-1").Return(nil)
				mockBuilder.On("SendRemoveMeetingRegistrantAccess", mock.Anything, mock.MatchedBy(func(msg models.MeetingRegistrantAccessMessage) bool {
					return msg.UID == "registrant-1"
				})).Return(nil)

				// Second registrant fails
				mockRegistrantRepo.On("Delete", mock.Anything, "registrant-2", uint64(0)).Return(domain.ErrInternal)

				// Mock GetBase for cancellation emails (called in goroutines) - first successful registrant will call this
				mockMeetingRepo.On("GetBase", mock.Anything, "meeting-456").Return(&models.MeetingBase{
					UID:         "meeting-456",
					Title:       "Test Meeting",
					StartTime:   now,
					Duration:    60,
					Timezone:    "UTC",
					Description: "Test meeting description",
				}, nil)
			},
			expectedErr: true,
		},
		{
			name:        "error getting registrants",
			messageData: []byte(`{"meeting_uid":"meeting-error"}`),
			setupMocks: func(mockMeetingRepo *domain.MockMeetingRepository, mockRegistrantRepo *domain.MockRegistrantRepository, mockBuilder *domain.MockMessageBuilder) {
				mockRegistrantRepo.On("ListByMeeting", mock.Anything, "meeting-error").Return(nil, domain.ErrInternal)
			},
			expectedErr: true,
		},
		{
			name:        "invalid JSON message",
			messageData: []byte(`invalid-json`),
			setupMocks: func(mockMeetingRepo *domain.MockMeetingRepository, mockRegistrantRepo *domain.MockRegistrantRepository, mockBuilder *domain.MockMessageBuilder) {
				// No mock calls expected
			},
			expectedErr: true,
		},
		{
			name:        "empty meeting UID",
			messageData: []byte(`{"meeting_uid":""}`),
			setupMocks: func(mockMeetingRepo *domain.MockMeetingRepository, mockRegistrantRepo *domain.MockRegistrantRepository, mockBuilder *domain.MockMessageBuilder) {
				// No mock calls expected
			},
			expectedErr: true,
		},
		{
			name:        "missing meeting UID",
			messageData: []byte(`{}`),
			setupMocks: func(mockMeetingRepo *domain.MockMeetingRepository, mockRegistrantRepo *domain.MockRegistrantRepository, mockBuilder *domain.MockMessageBuilder) {
				// No mock calls expected
			},
			expectedErr: true,
		},
		{
			name:        "registrant already deleted during cleanup",
			messageData: []byte(`{"meeting_uid":"meeting-already-deleted"}`),
			setupMocks: func(mockMeetingRepo *domain.MockMeetingRepository, mockRegistrantRepo *domain.MockRegistrantRepository, mockBuilder *domain.MockMessageBuilder) {
				registrants := []*models.Registrant{
					{
						UID:        "registrant-already-deleted",
						MeetingUID: "meeting-already-deleted",
						Username:   "user1",
						Email:      "user1@example.com",
						Host:       false,
					},
				}

				mockRegistrantRepo.On("ListByMeeting", mock.Anything, "meeting-already-deleted").Return(registrants, nil)

				// Registrant was already deleted (should not fail since we skip revision check)
				mockRegistrantRepo.On("Delete", mock.Anything, "registrant-already-deleted", uint64(0)).Return(domain.ErrRegistrantNotFound)
			},
			expectedErr: false,
			validate: func(t *testing.T, response []byte) {
				assert.Equal(t, "success", string(response))
			},
		},
		{
			name:        "service not ready",
			messageData: []byte(`{"meeting_uid":"test"}`),
			setupService: func() *MeetingsService {
				return &MeetingsService{
					MeetingRepository:    nil,
					RegistrantRepository: nil,
					MessageBuilder:       nil,
					Auth:                 &auth.MockJWTAuth{},
				}
			},
			setupMocks: func(mockMeetingRepo *domain.MockMeetingRepository, mockRegistrantRepo *domain.MockRegistrantRepository, mockBuilder *domain.MockMessageBuilder) {
				// No mock calls expected for service not ready
			},
			expectedErr: true,
			validate: func(t *testing.T, response []byte) {
				// No response expected when service not ready
			},
		},
		{
			name:        "concurrent processing of many registrants",
			messageData: []byte(`{"meeting_uid":"meeting-concurrent"}`),
			setupMocks: func(mockMeetingRepo *domain.MockMeetingRepository, mockRegistrantRepo *domain.MockRegistrantRepository, mockBuilder *domain.MockMessageBuilder) {
				now := time.Now()
				// Create many registrants to test concurrent processing
				const numRegistrants = 50
				registrants := make([]*models.Registrant, numRegistrants)
				for i := range numRegistrants {
					registrants[i] = &models.Registrant{
						UID:        fmt.Sprintf("registrant-%d", i),
						MeetingUID: "meeting-concurrent",
						Username:   fmt.Sprintf("user%d", i),
						Email:      fmt.Sprintf("user%d@example.com", i),
						Host:       i%2 == 0, // Alternate host status
					}
				}

				mockRegistrantRepo.On("ListByMeeting", mock.Anything, "meeting-concurrent").Return(registrants, nil)

				// Setup expectations for each registrant deletion
				for _, reg := range registrants {
					mockRegistrantRepo.On("Delete", mock.Anything, reg.UID, uint64(0)).Return(nil)
					mockBuilder.On("SendDeleteIndexMeetingRegistrant", mock.Anything, reg.UID).Return(nil)
					mockBuilder.On("SendRemoveMeetingRegistrantAccess", mock.Anything, mock.MatchedBy(func(msg models.MeetingRegistrantAccessMessage) bool {
						return msg.UID == reg.UID && msg.Username == reg.Username && msg.MeetingUID == reg.MeetingUID && msg.Host == reg.Host
					})).Return(nil)
				}

				// Mock GetBase for cancellation emails (called in goroutines for all registrants)
				mockMeetingRepo.On("GetBase", mock.Anything, "meeting-concurrent").Return(&models.MeetingBase{
					UID:         "meeting-concurrent",
					Title:       "Test Meeting",
					StartTime:   now,
					Duration:    60,
					Timezone:    "UTC",
					Description: "Test meeting description",
				}, nil)
			},
			expectedErr: false,
			validate: func(t *testing.T, response []byte) {
				assert.Equal(t, "success", string(response))
			},
		},
		{
			name:        "end-to-end message handling integration",
			messageData: []byte(`{"meeting_uid":"integration-test-meeting"}`),
			setupMocks: func(mockMeetingRepo *domain.MockMeetingRepository, mockRegistrantRepo *domain.MockRegistrantRepository, mockBuilder *domain.MockMessageBuilder) {
				now := time.Now()
				// Test the full message handling flow
				registrants := []*models.Registrant{
					{
						UID:        "integration-test-registrant",
						MeetingUID: "integration-test-meeting",
						Username:   "integration-user",
						Email:      "integration@example.com",
						Host:       true,
					},
				}

				mockRegistrantRepo.On("ListByMeeting", mock.Anything, "integration-test-meeting").Return(registrants, nil)
				mockRegistrantRepo.On("Delete", mock.Anything, "integration-test-registrant", uint64(0)).Return(nil)
				mockBuilder.On("SendDeleteIndexMeetingRegistrant", mock.Anything, "integration-test-registrant").Return(nil)
				mockBuilder.On("SendRemoveMeetingRegistrantAccess", mock.Anything, mock.MatchedBy(func(msg models.MeetingRegistrantAccessMessage) bool {
					return msg.UID == "integration-test-registrant" &&
						msg.Username == "integration-user" &&
						msg.MeetingUID == "integration-test-meeting" &&
						msg.Host == true
				})).Return(nil)
				// Mock GetBase for cancellation email (called in goroutine)
				mockMeetingRepo.On("GetBase", mock.Anything, "integration-test-meeting").Return(&models.MeetingBase{
					UID:         "integration-test-meeting",
					Title:       "Test Meeting",
					StartTime:   now,
					Duration:    60,
					Timezone:    "UTC",
					Description: "Test meeting description",
				}, nil)
			},
			expectedErr: false,
			validate: func(t *testing.T, response []byte) {
				assert.Equal(t, "success", string(response))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var service *MeetingsService
			var mockMeetingRepo *domain.MockMeetingRepository
			var mockRegistrantRepo *domain.MockRegistrantRepository
			var mockBuilder *domain.MockMessageBuilder

			if tt.setupService != nil {
				service = tt.setupService()
				// Create mock repositories for setup function, even if not used
				mockMeetingRepo = &domain.MockMeetingRepository{}
				mockRegistrantRepo = &domain.MockRegistrantRepository{}
				mockBuilder = &domain.MockMessageBuilder{}
			} else {
				service, mockMeetingRepo, mockBuilder, _ = setupServiceForTesting()
				mockRegistrantRepo = &domain.MockRegistrantRepository{}
				service.RegistrantRepository = mockRegistrantRepo
			}

			tt.setupMocks(mockMeetingRepo, mockRegistrantRepo, mockBuilder)

			// Add email service mocks for meeting deletion tests
			if mockEmailService, ok := service.EmailService.(*domain.MockEmailService); ok {
				// Add flexible email mock that accepts any cancellation email
				mockEmailService.On("SendRegistrantCancellation", mock.Anything, mock.AnythingOfType("domain.EmailCancellation")).Return(nil).Maybe()
			}

			mockMsg := newMockMessage(models.MeetingDeletedSubject, tt.messageData)

			response, err := service.HandleMeetingDeleted(ctx, mockMsg)

			if tt.expectedErr {
				assert.Error(t, err)
				assert.Nil(t, response)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, response)
				if tt.validate != nil {
					tt.validate(t, response)
				}
			}

			// Only assert expectations if service was properly set up
			if tt.setupService == nil {
				mockMeetingRepo.AssertExpectations(t)
				mockRegistrantRepo.AssertExpectations(t)
				mockBuilder.AssertExpectations(t)
			}
		})
	}

	// Special integration test that goes through HandleMessage
	t.Run("full message handler integration", func(t *testing.T) {
		service, mockMeetingRepo, mockBuilder, _ := setupServiceForTesting()
		mockRegistrantRepo := &domain.MockRegistrantRepository{}
		service.RegistrantRepository = mockRegistrantRepo

		now := time.Now()
		// Test the full message handling flow
		registrants := []*models.Registrant{
			{
				UID:        "integration-test-registrant",
				MeetingUID: "integration-test-meeting",
				Username:   "integration-user",
				Email:      "integration@example.com",
				Host:       true,
			},
		}

		mockRegistrantRepo.On("ListByMeeting", mock.Anything, "integration-test-meeting").Return(registrants, nil)
		mockRegistrantRepo.On("Delete", mock.Anything, "integration-test-registrant", uint64(0)).Return(nil)
		mockBuilder.On("SendDeleteIndexMeetingRegistrant", mock.Anything, "integration-test-registrant").Return(nil)
		mockBuilder.On("SendRemoveMeetingRegistrantAccess", mock.Anything, mock.MatchedBy(func(msg models.MeetingRegistrantAccessMessage) bool {
			return msg.UID == "integration-test-registrant" &&
				msg.Username == "integration-user" &&
				msg.MeetingUID == "integration-test-meeting" &&
				msg.Host == true
		})).Return(nil)
		// Mock GetBase for cancellation email (called in goroutine)
		mockMeetingRepo.On("GetBase", mock.Anything, "integration-test-meeting").Return(&models.MeetingBase{
			UID:         "integration-test-meeting",
			Title:       "Test Meeting",
			StartTime:   now,
			Duration:    60,
			Timezone:    "UTC",
			Description: "Test meeting description",
		}, nil)

		// Add email service mock for cancellation email
		if mockEmailService, ok := service.EmailService.(*domain.MockEmailService); ok {
			mockEmailService.On("SendRegistrantCancellation", mock.Anything, mock.AnythingOfType("domain.EmailCancellation")).Return(nil).Maybe()
		}

		messageData := []byte(`{"meeting_uid":"integration-test-meeting"}`)
		mockMsg := newMockMessage(models.MeetingDeletedSubject, messageData)
		mockMsg.On("Respond", mock.MatchedBy(func(data []byte) bool {
			return string(data) == "success"
		})).Return(nil)

		// Execute through HandleMessage (the main entry point)
		service.HandleMessage(ctx, mockMsg)

		// Verify all expectations
		mockMeetingRepo.AssertExpectations(t)
		mockRegistrantRepo.AssertExpectations(t)
		mockBuilder.AssertExpectations(t)
		mockMsg.AssertExpectations(t)
	})
}
