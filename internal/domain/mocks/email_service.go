// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
)

// MockEmailService implements EmailService for testing
type MockEmailService struct {
	mock.Mock
}

func (m *MockEmailService) SendRegistrantInvitation(ctx context.Context, invitation domain.EmailInvitation) error {
	args := m.Called(ctx, invitation)
	return args.Error(0)
}

func (m *MockEmailService) SendRegistrantCancellation(ctx context.Context, cancellation domain.EmailCancellation) error {
	args := m.Called(ctx, cancellation)
	return args.Error(0)
}

func (m *MockEmailService) SendRegistrantUpdatedInvitation(ctx context.Context, updatedInvitation domain.EmailUpdatedInvitation) error {
	args := m.Called(ctx, updatedInvitation)
	return args.Error(0)
}
