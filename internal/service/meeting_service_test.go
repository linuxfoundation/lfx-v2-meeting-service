// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"testing"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/infrastructure/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestNewMeetingsService(t *testing.T) {
	tests := []struct {
		name string
		auth auth.IJWTAuth
	}{
		{
			name: "create service with valid dependencies",
			auth: &auth.MockJWTAuth{},
		},
		{
			name: "create service with nil auth",
			auth: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewMeetingsService(tt.auth, ServiceConfig{})

			assert.NotNil(t, service)
			assert.Equal(t, tt.auth, service.Auth)
			assert.Nil(t, service.MessageBuilder) // Should be set separately
		})
	}
}

func TestMeetingsService_ServiceReady(t *testing.T) {
	tests := []struct {
		name          string
		setupService  func() *MeetingsService
		expectedReady bool
	}{
		{
			name: "service ready with all dependencies",
			setupService: func() *MeetingsService {
				return &MeetingsService{
					MeetingRepository:                &domain.MockMeetingRepository{},
					RegistrantRepository:             &domain.MockRegistrantRepository{},
					PastMeetingRepository:            &domain.MockPastMeetingRepository{},
					PastMeetingParticipantRepository: &domain.MockPastMeetingParticipantRepository{},
					MessageBuilder:                   &domain.MockMessageBuilder{},
					PlatformRegistry:                 &domain.MockPlatformRegistry{},
					Auth:                             &auth.MockJWTAuth{},
				}
			},
			expectedReady: true,
		},
		{
			name: "service not ready - missing repository",
			setupService: func() *MeetingsService {
				return &MeetingsService{
					MeetingRepository:                nil,
					RegistrantRepository:             &domain.MockRegistrantRepository{},
					PastMeetingRepository:            &domain.MockPastMeetingRepository{},
					PastMeetingParticipantRepository: &domain.MockPastMeetingParticipantRepository{},
					MessageBuilder:                   &domain.MockMessageBuilder{},
					PlatformRegistry:                 &domain.MockPlatformRegistry{},
					Auth:                             &auth.MockJWTAuth{},
				}
			},
			expectedReady: false,
		},
		{
			name: "service not ready - missing message builder",
			setupService: func() *MeetingsService {
				return &MeetingsService{
					MeetingRepository:                &domain.MockMeetingRepository{},
					RegistrantRepository:             &domain.MockRegistrantRepository{},
					PastMeetingRepository:            &domain.MockPastMeetingRepository{},
					PastMeetingParticipantRepository: &domain.MockPastMeetingParticipantRepository{},
					MessageBuilder:                   nil,
					PlatformRegistry:                 &domain.MockPlatformRegistry{},
					Auth:                             &auth.MockJWTAuth{},
				}
			},
			expectedReady: false,
		},
		{
			name: "service not ready - missing registrant repository",
			setupService: func() *MeetingsService {
				return &MeetingsService{
					MeetingRepository:                &domain.MockMeetingRepository{},
					RegistrantRepository:             nil,
					PastMeetingRepository:            &domain.MockPastMeetingRepository{},
					PastMeetingParticipantRepository: &domain.MockPastMeetingParticipantRepository{},
					MessageBuilder:                   &domain.MockMessageBuilder{},
					PlatformRegistry:                 &domain.MockPlatformRegistry{},
					Auth:                             &auth.MockJWTAuth{},
				}
			},
			expectedReady: false,
		},
		{
			name: "service not ready - missing platform registry",
			setupService: func() *MeetingsService {
				return &MeetingsService{
					MeetingRepository:                &domain.MockMeetingRepository{},
					RegistrantRepository:             &domain.MockRegistrantRepository{},
					PastMeetingRepository:            &domain.MockPastMeetingRepository{},
					PastMeetingParticipantRepository: &domain.MockPastMeetingParticipantRepository{},
					MessageBuilder:                   &domain.MockMessageBuilder{},
					PlatformRegistry:                 nil,
					Auth:                             &auth.MockJWTAuth{},
				}
			},
			expectedReady: false,
		},
		{
			name: "service not ready - missing both critical dependencies",
			setupService: func() *MeetingsService {
				return &MeetingsService{
					MeetingRepository:                nil,
					RegistrantRepository:             nil,
					PastMeetingRepository:            nil,
					PastMeetingParticipantRepository: nil,
					MessageBuilder:                   nil,
					PlatformRegistry:                 nil,
					Auth:                             &auth.MockJWTAuth{},
				}
			},
			expectedReady: false,
		},
		{
			name: "service ready without auth (auth is not checked in ServiceReady)",
			setupService: func() *MeetingsService {
				return &MeetingsService{
					MeetingRepository:                &domain.MockMeetingRepository{},
					RegistrantRepository:             &domain.MockRegistrantRepository{},
					PastMeetingRepository:            &domain.MockPastMeetingRepository{},
					PastMeetingParticipantRepository: &domain.MockPastMeetingParticipantRepository{},
					MessageBuilder:                   &domain.MockMessageBuilder{},
					PlatformRegistry:                 &domain.MockPlatformRegistry{},
					Auth:                             nil,
				}
			},
			expectedReady: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := tt.setupService()
			ready := service.ServiceReady()
			assert.Equal(t, tt.expectedReady, ready)
		})
	}
}

func TestMeetingsService_Dependencies(t *testing.T) {
	t.Run("service maintains dependency references", func(t *testing.T) {
		mockRepo := &domain.MockMeetingRepository{}
		mockRegistrantRepo := &domain.MockRegistrantRepository{}
		mockPastMeetingRepo := &domain.MockPastMeetingRepository{}
		mockPastMeetingParticipantRepo := &domain.MockPastMeetingParticipantRepository{}
		mockAuth := &auth.MockJWTAuth{}
		mockBuilder := &domain.MockMessageBuilder{}

		service := NewMeetingsService(mockAuth, ServiceConfig{})
		service.MeetingRepository = mockRepo
		service.RegistrantRepository = mockRegistrantRepo
		service.PastMeetingRepository = mockPastMeetingRepo
		service.PastMeetingParticipantRepository = mockPastMeetingParticipantRepo
		service.MessageBuilder = mockBuilder

		// Verify dependencies are correctly set
		assert.Same(t, mockRepo, service.MeetingRepository)
		assert.Same(t, mockRegistrantRepo, service.RegistrantRepository)
		assert.Same(t, mockPastMeetingRepo, service.PastMeetingRepository)
		assert.Same(t, mockPastMeetingParticipantRepo, service.PastMeetingParticipantRepository)
		assert.Same(t, mockAuth, service.Auth)
		assert.Same(t, mockBuilder, service.MessageBuilder)
	})
}

func TestMeetingsService_Interfaces(t *testing.T) {
	t.Run("service implements MessageHandler interface", func(t *testing.T) {
		service := &MeetingsService{}
		assert.Implements(t, (*domain.MessageHandler)(nil), service)
	})
}

// Setup helper for common test scenarios
func setupServiceForTesting() (*MeetingsService, *domain.MockMeetingRepository, *domain.MockMessageBuilder, *auth.MockJWTAuth) {
	mockRepo := &domain.MockMeetingRepository{}
	mockBuilder := &domain.MockMessageBuilder{}
	mockAuth := &auth.MockJWTAuth{}

	service := NewMeetingsService(mockAuth, ServiceConfig{})
	service.MeetingRepository = mockRepo
	service.RegistrantRepository = &domain.MockRegistrantRepository{}
	service.PastMeetingRepository = &domain.MockPastMeetingRepository{}
	service.PastMeetingParticipantRepository = &domain.MockPastMeetingParticipantRepository{}
	service.MessageBuilder = mockBuilder
	service.PlatformRegistry = &domain.MockPlatformRegistry{}

	return service, mockRepo, mockBuilder, mockAuth
}

// Mock message for testing
type mockMessage struct {
	subject  string
	data     []byte
	hasReply bool
	mock.Mock
}

func (m *mockMessage) Subject() string {
	return m.subject
}

func (m *mockMessage) Data() []byte {
	return m.data
}

func (m *mockMessage) Respond(data []byte) error {
	args := m.Called(data)
	return args.Error(0)
}

func (m *mockMessage) HasReply() bool {
	return m.hasReply
}

func newMockMessage(subject string, data []byte) *mockMessage {
	return &mockMessage{
		subject: subject,
		data:    data,
		// Default to true for backward compatibility with existing tests
		hasReply: true,
	}
}

func newMockMessageNoReply(subject string, data []byte) *mockMessage {
	return &mockMessage{
		subject:  subject,
		data:     data,
		hasReply: false,
	}
}
