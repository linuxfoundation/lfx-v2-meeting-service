// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"
)

// MockIDMapper implements domain.IDMapper for testing.
type MockIDMapper struct {
	mock.Mock
}

func (m *MockIDMapper) MapProjectV2ToV1(ctx context.Context, v2UID string) (string, error) {
	args := m.Called(ctx, v2UID)
	return args.String(0), args.Error(1)
}

func (m *MockIDMapper) MapProjectV1ToV2(ctx context.Context, v1SFID string) (string, error) {
	args := m.Called(ctx, v1SFID)
	return args.String(0), args.Error(1)
}

func (m *MockIDMapper) MapCommitteeV2ToV1(ctx context.Context, v2UID string) (string, error) {
	args := m.Called(ctx, v2UID)
	return args.String(0), args.Error(1)
}

func (m *MockIDMapper) MapCommitteeV1ToV2(ctx context.Context, v1SFID string) (string, error) {
	args := m.Called(ctx, v1SFID)
	return args.String(0), args.Error(1)
}

func (m *MockIDMapper) MapInviteeIDToParticipantV2(ctx context.Context, inviteeID string) (string, error) {
	args := m.Called(ctx, inviteeID)
	return args.String(0), args.Error(1)
}

func (m *MockIDMapper) MapAttendeeIDToParticipantV2(ctx context.Context, attendeeID string) (string, error) {
	args := m.Called(ctx, attendeeID)
	return args.String(0), args.Error(1)
}

func (m *MockIDMapper) MapParticipantV2ToInviteeID(ctx context.Context, v2ParticipantID string) (string, error) {
	args := m.Called(ctx, v2ParticipantID)
	return args.String(0), args.Error(1)
}

func (m *MockIDMapper) MapParticipantV2ToAttendeeID(ctx context.Context, v2ParticipantID string) (string, error) {
	args := m.Called(ctx, v2ParticipantID)
	return args.String(0), args.Error(1)
}
