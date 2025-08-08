// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package domain

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
)

// MockMeetingRepository implements MeetingRepository for testing
type MockMeetingRepository struct {
	mock.Mock
}

func (m *MockMeetingRepository) Get(ctx context.Context, meetingUID string) (*models.Meeting, error) {
	args := m.Called(ctx, meetingUID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Meeting), args.Error(1)
}

func (m *MockMeetingRepository) GetWithRevision(ctx context.Context, meetingUID string) (*models.Meeting, uint64, error) {
	args := m.Called(ctx, meetingUID)
	if args.Get(0) == nil {
		return nil, args.Get(1).(uint64), args.Error(2)
	}
	return args.Get(0).(*models.Meeting), args.Get(1).(uint64), args.Error(2)
}

func (m *MockMeetingRepository) Update(ctx context.Context, meeting *models.Meeting, revision uint64) error {
	args := m.Called(ctx, meeting, revision)
	return args.Error(0)
}

func (m *MockMeetingRepository) Exists(ctx context.Context, meetingUID string) (bool, error) {
	args := m.Called(ctx, meetingUID)
	return args.Bool(0), args.Error(1)
}

func (m *MockMeetingRepository) ListAll(ctx context.Context) ([]*models.Meeting, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Meeting), args.Error(1)
}

func (m *MockMeetingRepository) Create(ctx context.Context, meeting *models.Meeting) error {
	args := m.Called(ctx, meeting)
	return args.Error(0)
}

func (m *MockMeetingRepository) Delete(ctx context.Context, meetingUID string, revision uint64) error {
	args := m.Called(ctx, meetingUID, revision)
	return args.Error(0)
}

// MockMessageBuilder implements MessageBuilder for testing
type MockMessageBuilder struct {
	mock.Mock
}

func (m *MockMessageBuilder) SendIndexMeeting(ctx context.Context, action models.MessageAction, data models.Meeting) error {
	args := m.Called(ctx, action, data)
	return args.Error(0)
}

func (m *MockMessageBuilder) SendDeleteIndexMeeting(ctx context.Context, data string) error {
	args := m.Called(ctx, data)
	return args.Error(0)
}

func (m *MockMessageBuilder) SendUpdateAccessMeeting(ctx context.Context, data models.MeetingAccessMessage) error {
	args := m.Called(ctx, data)
	return args.Error(0)
}

func (m *MockMessageBuilder) SendDeleteAllAccessMeeting(ctx context.Context, data string) error {
	args := m.Called(ctx, data)
	return args.Error(0)
}

// MockMessage implements Message for testing
type MockMessage struct {
	mock.Mock
	data    []byte
	subject string
}

func (m *MockMessage) Subject() string {
	return m.subject
}

func (m *MockMessage) Data() []byte {
	return m.data
}

func (m *MockMessage) Respond(data []byte) error {
	args := m.Called(data)
	return args.Error(0)
}

// NewMockMessage creates a mock message for testing
func NewMockMessage(data []byte, subject string) *MockMessage {
	return &MockMessage{
		data:    data,
		subject: subject,
	}
}

// MockRegistrantRepository implements RegistrantRepository for testing
type MockRegistrantRepository struct {
	mock.Mock
}

func (m *MockRegistrantRepository) Create(ctx context.Context, registrant *models.Registrant) error {
	args := m.Called(ctx, registrant)
	return args.Error(0)
}

func (m *MockRegistrantRepository) Exists(ctx context.Context, registrantUID string) (bool, error) {
	args := m.Called(ctx, registrantUID)
	return args.Bool(0), args.Error(1)
}

func (m *MockRegistrantRepository) Delete(ctx context.Context, registrantUID string, revision uint64) error {
	args := m.Called(ctx, registrantUID, revision)
	return args.Error(0)
}

func (m *MockRegistrantRepository) Get(ctx context.Context, registrantUID string) (*models.Registrant, error) {
	args := m.Called(ctx, registrantUID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Registrant), args.Error(1)
}

func (m *MockRegistrantRepository) GetWithRevision(ctx context.Context, registrantUID string) (*models.Registrant, uint64, error) {
	args := m.Called(ctx, registrantUID)
	if args.Get(0) == nil {
		return nil, args.Get(1).(uint64), args.Error(2)
	}
	return args.Get(0).(*models.Registrant), args.Get(1).(uint64), args.Error(2)
}

func (m *MockRegistrantRepository) Update(ctx context.Context, registrant *models.Registrant, revision uint64) error {
	args := m.Called(ctx, registrant, revision)
	return args.Error(0)
}

func (m *MockRegistrantRepository) ListByMeeting(ctx context.Context, meetingUID string) ([]*models.Registrant, error) {
	args := m.Called(ctx, meetingUID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Registrant), args.Error(1)
}

func (m *MockRegistrantRepository) ListByEmail(ctx context.Context, email string) ([]*models.Registrant, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Registrant), args.Error(1)
}

// NewMockRegistrantRepository creates a new mock registrant repository for testing
func NewMockRegistrantRepository(t interface{ Cleanup(func()) }) *MockRegistrantRepository {
	return &MockRegistrantRepository{}
}
