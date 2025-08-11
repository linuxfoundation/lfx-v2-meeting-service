// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
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
		name        string
		subject     string
		messageData []byte
		setupMocks  func(*domain.MockMeetingRepository, *domain.MockMessageBuilder)
		expectCalls bool
	}{
		{
			name:        "handle meeting get title message",
			subject:     models.MeetingGetTitleSubject,
			messageData: []byte("01234567-89ab-cdef-0123-456789abcdef"),
			setupMocks: func(mockRepo *domain.MockMeetingRepository, mockBuilder *domain.MockMessageBuilder) {
				now := time.Now()
				mockRepo.On("GetBase", mock.Anything, "01234567-89ab-cdef-0123-456789abcdef").Return(
					&models.MeetingBase{
						UID:       "01234567-89ab-cdef-0123-456789abcdef",
						Title:     "Test Meeting",
						CreatedAt: &now,
						UpdatedAt: &now,
					},
					nil,
				)
			},
			expectCalls: true,
		},
		{
			name:        "unknown subject",
			subject:     "unknown.subject",
			messageData: []byte(`{}`),
			setupMocks: func(mockRepo *domain.MockMeetingRepository, mockBuilder *domain.MockMessageBuilder) {
				// No mock calls expected
			},
			expectCalls: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, mockRepo, mockBuilder, mockAuth := setupServiceForTesting()
			tt.setupMocks(mockRepo, mockBuilder)

			// Create mock message
			mockMsg := newMockMessage(tt.subject, tt.messageData)

			if tt.expectCalls {
				mockMsg.On("Respond", mock.Anything).Return(nil)
			}

			// Call HandleMessage
			service.HandleMessage(ctx, mockMsg)

			// Verify expectations
			if tt.expectCalls {
				mockMsg.AssertExpectations(t)
			}
			mockRepo.AssertExpectations(t)
			mockBuilder.AssertExpectations(t)
			mockAuth.AssertExpectations(t)
		})
	}
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

func TestMeetingsService_MessageHandling_ErrorCases(t *testing.T) {

	ctx := context.Background()

	tests := []struct {
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
				}
			},
			subject:     models.MeetingGetTitleSubject,
			messageData: []byte("01234567-89ab-cdef-0123-456789abcdef"),
			description: "should handle repository errors gracefully",
		},
	}

	for _, tt := range tests {
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
}

func TestMeetingsService_MessageHandling_Integration(t *testing.T) {

	ctx := context.Background()

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
}
