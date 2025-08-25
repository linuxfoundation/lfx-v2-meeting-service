// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
)

// MockPlatformProvider implements PlatformProvider for testing
type MockPlatformProvider struct {
	mock.Mock
}

func (m *MockPlatformProvider) CreateMeeting(ctx context.Context, meeting *models.MeetingBase) (*domain.CreateMeetingResult, error) {
	args := m.Called(ctx, meeting)
	result := args.Get(0)
	if result == nil {
		return nil, args.Error(1)
	}
	return result.(*domain.CreateMeetingResult), args.Error(1)
}

func (m *MockPlatformProvider) UpdateMeeting(ctx context.Context, platformMeetingID string, meeting *models.MeetingBase) error {
	args := m.Called(ctx, platformMeetingID, meeting)
	return args.Error(0)
}

func (m *MockPlatformProvider) DeleteMeeting(ctx context.Context, platformMeetingID string) error {
	args := m.Called(ctx, platformMeetingID)
	return args.Error(0)
}

func (m *MockPlatformProvider) StorePlatformData(meeting *models.MeetingBase, result *domain.CreateMeetingResult) {
	m.Called(meeting, result)
}

func (m *MockPlatformProvider) GetPlatformMeetingID(meeting *models.MeetingBase) string {
	args := m.Called(meeting)
	return args.String(0)
}

// MockPlatformRegistry implements PlatformRegistry for testing
type MockPlatformRegistry struct {
	mock.Mock
}

func (m *MockPlatformRegistry) GetProvider(platform string) (domain.PlatformProvider, error) {
	args := m.Called(platform)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(domain.PlatformProvider), args.Error(1)
}

func (m *MockPlatformRegistry) RegisterProvider(platform string, provider domain.PlatformProvider) {
	m.Called(platform, provider)
}
