// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
)

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

func (m *MockRegistrantRepository) ListByEmailAndCommittee(ctx context.Context, email string, committeeUID string) ([]*models.Registrant, error) {
	args := m.Called(ctx, email, committeeUID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Registrant), args.Error(1)
}

func (m *MockRegistrantRepository) ExistsByMeetingAndEmail(ctx context.Context, meetingUID, email string) (bool, error) {
	args := m.Called(ctx, meetingUID, email)
	return args.Bool(0), args.Error(1)
}

func (m *MockRegistrantRepository) GetByMeetingAndEmail(ctx context.Context, meetingUID, email string) (*models.Registrant, uint64, error) {
	args := m.Called(ctx, meetingUID, email)
	if args.Get(0) == nil {
		return nil, args.Get(1).(uint64), args.Error(2)
	}
	return args.Get(0).(*models.Registrant), args.Get(1).(uint64), args.Error(2)
}

// NewMockRegistrantRepository creates a new mock registrant repository for testing
func NewMockRegistrantRepository(t interface{ Cleanup(func()) }) *MockRegistrantRepository {
	return &MockRegistrantRepository{}
}
