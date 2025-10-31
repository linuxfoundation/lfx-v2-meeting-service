// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
)

// MockMeetingRSVPRepository implements MeetingRSVPRepository for testing
type MockMeetingRSVPRepository struct {
	mock.Mock
}

func (m *MockMeetingRSVPRepository) Create(ctx context.Context, rsvp *models.RSVPResponse) error {
	args := m.Called(ctx, rsvp)
	return args.Error(0)
}

func (m *MockMeetingRSVPRepository) Exists(ctx context.Context, rsvpID string) (bool, error) {
	args := m.Called(ctx, rsvpID)
	return args.Bool(0), args.Error(1)
}

func (m *MockMeetingRSVPRepository) Delete(ctx context.Context, rsvpID string, revision uint64) error {
	args := m.Called(ctx, rsvpID, revision)
	return args.Error(0)
}

func (m *MockMeetingRSVPRepository) Get(ctx context.Context, rsvpID string) (*models.RSVPResponse, error) {
	args := m.Called(ctx, rsvpID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.RSVPResponse), args.Error(1)
}

func (m *MockMeetingRSVPRepository) GetWithRevision(ctx context.Context, rsvpID string) (*models.RSVPResponse, uint64, error) {
	args := m.Called(ctx, rsvpID)
	if args.Get(0) == nil {
		return nil, args.Get(1).(uint64), args.Error(2)
	}
	return args.Get(0).(*models.RSVPResponse), args.Get(1).(uint64), args.Error(2)
}

func (m *MockMeetingRSVPRepository) Update(ctx context.Context, rsvp *models.RSVPResponse, revision uint64) error {
	args := m.Called(ctx, rsvp, revision)
	return args.Error(0)
}

func (m *MockMeetingRSVPRepository) ListByMeeting(ctx context.Context, meetingUID string) ([]*models.RSVPResponse, error) {
	args := m.Called(ctx, meetingUID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.RSVPResponse), args.Error(1)
}

// NewMockMeetingRSVPRepository creates a new mock RSVP repository for testing
func NewMockMeetingRSVPRepository(t interface{ Cleanup(func()) }) *MockMeetingRSVPRepository {
	return &MockMeetingRSVPRepository{}
}
