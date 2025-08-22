// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
)

// MockPastMeetingRepository implements PastMeetingRepository for testing
type MockPastMeetingRepository struct {
	mock.Mock
}

func (m *MockPastMeetingRepository) Create(ctx context.Context, pastMeeting *models.PastMeeting) error {
	args := m.Called(ctx, pastMeeting)
	return args.Error(0)
}

func (m *MockPastMeetingRepository) Exists(ctx context.Context, pastMeetingUID string) (bool, error) {
	args := m.Called(ctx, pastMeetingUID)
	return args.Bool(0), args.Error(1)
}

func (m *MockPastMeetingRepository) Delete(ctx context.Context, pastMeetingUID string, revision uint64) error {
	args := m.Called(ctx, pastMeetingUID, revision)
	return args.Error(0)
}

func (m *MockPastMeetingRepository) Get(ctx context.Context, pastMeetingUID string) (*models.PastMeeting, error) {
	args := m.Called(ctx, pastMeetingUID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.PastMeeting), args.Error(1)
}

func (m *MockPastMeetingRepository) GetWithRevision(ctx context.Context, pastMeetingUID string) (*models.PastMeeting, uint64, error) {
	args := m.Called(ctx, pastMeetingUID)
	if args.Get(0) == nil {
		return nil, args.Get(1).(uint64), args.Error(2)
	}
	return args.Get(0).(*models.PastMeeting), args.Get(1).(uint64), args.Error(2)
}

func (m *MockPastMeetingRepository) Update(ctx context.Context, pastMeeting *models.PastMeeting, revision uint64) error {
	args := m.Called(ctx, pastMeeting, revision)
	return args.Error(0)
}

func (m *MockPastMeetingRepository) ListByMeeting(ctx context.Context, meetingUID string) ([]*models.PastMeeting, error) {
	args := m.Called(ctx, meetingUID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.PastMeeting), args.Error(1)
}

func (m *MockPastMeetingRepository) GetByMeetingAndOccurrence(ctx context.Context, meetingUID, occurrenceID string) (*models.PastMeeting, error) {
	args := m.Called(ctx, meetingUID, occurrenceID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.PastMeeting), args.Error(1)
}

func (m *MockPastMeetingRepository) GetByPlatformMeetingID(ctx context.Context, platform, platformMeetingID string) (*models.PastMeeting, error) {
	args := m.Called(ctx, platform, platformMeetingID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.PastMeeting), args.Error(1)
}