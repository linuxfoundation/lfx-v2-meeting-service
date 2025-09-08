// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
)

// MockMeetingRepository implements MeetingRepository for testing
type MockMeetingRepository struct {
	mock.Mock
}

func (m *MockMeetingRepository) GetBase(ctx context.Context, meetingUID string) (*models.MeetingBase, error) {
	args := m.Called(ctx, meetingUID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.MeetingBase), args.Error(1)
}

func (m *MockMeetingRepository) GetBaseWithRevision(ctx context.Context, meetingUID string) (*models.MeetingBase, uint64, error) {
	args := m.Called(ctx, meetingUID)
	if args.Get(0) == nil {
		return nil, args.Get(1).(uint64), args.Error(2)
	}
	return args.Get(0).(*models.MeetingBase), args.Get(1).(uint64), args.Error(2)
}

func (m *MockMeetingRepository) UpdateBase(ctx context.Context, meeting *models.MeetingBase, revision uint64) error {
	args := m.Called(ctx, meeting, revision)
	return args.Error(0)
}

func (m *MockMeetingRepository) GetSettings(ctx context.Context, meetingUID string) (*models.MeetingSettings, error) {
	args := m.Called(ctx, meetingUID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.MeetingSettings), args.Error(1)
}

func (m *MockMeetingRepository) GetSettingsWithRevision(ctx context.Context, meetingUID string) (*models.MeetingSettings, uint64, error) {
	args := m.Called(ctx, meetingUID)
	if args.Get(0) == nil {
		return nil, args.Get(1).(uint64), args.Error(2)
	}
	return args.Get(0).(*models.MeetingSettings), args.Get(1).(uint64), args.Error(2)
}

func (m *MockMeetingRepository) UpdateSettings(ctx context.Context, meetingSettings *models.MeetingSettings, revision uint64) error {
	args := m.Called(ctx, meetingSettings, revision)
	return args.Error(0)
}

func (m *MockMeetingRepository) Exists(ctx context.Context, meetingUID string) (bool, error) {
	args := m.Called(ctx, meetingUID)
	return args.Bool(0), args.Error(1)
}

func (m *MockMeetingRepository) ListAll(ctx context.Context) ([]*models.MeetingBase, []*models.MeetingSettings, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, nil, args.Error(2)
	}
	return args.Get(0).([]*models.MeetingBase), args.Get(1).([]*models.MeetingSettings), args.Error(2)
}

func (m *MockMeetingRepository) ListByCommittee(ctx context.Context, committeeUID string) ([]*models.MeetingBase, []*models.MeetingSettings, error) {
	args := m.Called(ctx, committeeUID)
	if args.Get(0) == nil {
		return nil, nil, args.Error(2)
	}
	return args.Get(0).([]*models.MeetingBase), args.Get(1).([]*models.MeetingSettings), args.Error(2)
}

func (m *MockMeetingRepository) Create(ctx context.Context, meeting *models.MeetingBase, settings *models.MeetingSettings) error {
	args := m.Called(ctx, meeting, settings)
	return args.Error(0)
}

func (m *MockMeetingRepository) Delete(ctx context.Context, meetingUID string, revision uint64) error {
	args := m.Called(ctx, meetingUID, revision)
	return args.Error(0)
}

func (m *MockMeetingRepository) GetByZoomMeetingID(ctx context.Context, zoomMeetingID string) (*models.MeetingBase, error) {
	args := m.Called(ctx, zoomMeetingID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.MeetingBase), args.Error(1)
}
