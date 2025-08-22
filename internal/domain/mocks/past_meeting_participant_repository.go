// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
)

// MockPastMeetingParticipantRepository implements PastMeetingParticipantRepository for testing
type MockPastMeetingParticipantRepository struct {
	mock.Mock
}

func (m *MockPastMeetingParticipantRepository) Create(ctx context.Context, participant *models.PastMeetingParticipant) error {
	args := m.Called(ctx, participant)
	return args.Error(0)
}

func (m *MockPastMeetingParticipantRepository) Exists(ctx context.Context, participantUID string) (bool, error) {
	args := m.Called(ctx, participantUID)
	return args.Bool(0), args.Error(1)
}

func (m *MockPastMeetingParticipantRepository) Delete(ctx context.Context, participantUID string, revision uint64) error {
	args := m.Called(ctx, participantUID, revision)
	return args.Error(0)
}

func (m *MockPastMeetingParticipantRepository) Get(ctx context.Context, participantUID string) (*models.PastMeetingParticipant, error) {
	args := m.Called(ctx, participantUID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.PastMeetingParticipant), args.Error(1)
}

func (m *MockPastMeetingParticipantRepository) GetWithRevision(ctx context.Context, participantUID string) (*models.PastMeetingParticipant, uint64, error) {
	args := m.Called(ctx, participantUID)
	if args.Get(0) == nil {
		return nil, args.Get(1).(uint64), args.Error(2)
	}
	return args.Get(0).(*models.PastMeetingParticipant), args.Get(1).(uint64), args.Error(2)
}

func (m *MockPastMeetingParticipantRepository) Update(ctx context.Context, participant *models.PastMeetingParticipant, revision uint64) error {
	args := m.Called(ctx, participant, revision)
	return args.Error(0)
}

func (m *MockPastMeetingParticipantRepository) ListByPastMeeting(ctx context.Context, pastMeetingUID string) ([]*models.PastMeetingParticipant, error) {
	args := m.Called(ctx, pastMeetingUID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.PastMeetingParticipant), args.Error(1)
}

func (m *MockPastMeetingParticipantRepository) ListByEmail(ctx context.Context, email string) ([]*models.PastMeetingParticipant, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.PastMeetingParticipant), args.Error(1)
}

func (m *MockPastMeetingParticipantRepository) GetByPastMeetingAndEmail(ctx context.Context, pastMeetingUID, email string) (*models.PastMeetingParticipant, error) {
	args := m.Called(ctx, pastMeetingUID, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.PastMeetingParticipant), args.Error(1)
}