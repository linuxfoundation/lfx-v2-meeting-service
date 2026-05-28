// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/models/itx"
)

// MockITXRegistrantClient implements domain.ITXRegistrantClient for testing.
type MockITXRegistrantClient struct {
	mock.Mock
}

func (m *MockITXRegistrantClient) CreateRegistrant(ctx context.Context, meetingID string, req *itx.ZoomMeetingRegistrant) (*itx.ZoomMeetingRegistrant, error) {
	args := m.Called(ctx, meetingID, req)
	if v := args.Get(0); v != nil {
		return v.(*itx.ZoomMeetingRegistrant), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockITXRegistrantClient) GetRegistrant(ctx context.Context, meetingID, registrantID string) (*itx.ZoomMeetingRegistrant, error) {
	args := m.Called(ctx, meetingID, registrantID)
	if v := args.Get(0); v != nil {
		return v.(*itx.ZoomMeetingRegistrant), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockITXRegistrantClient) UpdateRegistrant(ctx context.Context, meetingID, registrantID string, req *itx.ZoomMeetingRegistrant) error {
	args := m.Called(ctx, meetingID, registrantID, req)
	return args.Error(0)
}

func (m *MockITXRegistrantClient) DeleteRegistrant(ctx context.Context, meetingID, registrantID string) error {
	args := m.Called(ctx, meetingID, registrantID)
	return args.Error(0)
}

func (m *MockITXRegistrantClient) GetRegistrantICS(ctx context.Context, meetingID, registrantID string) (*itx.RegistrantICS, error) {
	args := m.Called(ctx, meetingID, registrantID)
	if v := args.Get(0); v != nil {
		return v.(*itx.RegistrantICS), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockITXRegistrantClient) ResendRegistrantInvitation(ctx context.Context, meetingID, registrantID string) error {
	args := m.Called(ctx, meetingID, registrantID)
	return args.Error(0)
}

func (m *MockITXRegistrantClient) UpdateRegistrantInvite(ctx context.Context, registrantID string, fields domain.ITXRegistrantInviteFields) error {
	args := m.Called(ctx, registrantID, fields)
	return args.Error(0)
}

func (m *MockITXRegistrantClient) AcceptInvite(ctx context.Context, email, username string) (*domain.ITXAcceptInviteResult, error) {
	args := m.Called(ctx, email, username)
	if v := args.Get(0); v != nil {
		return v.(*domain.ITXAcceptInviteResult), args.Error(1)
	}
	return nil, args.Error(1)
}
