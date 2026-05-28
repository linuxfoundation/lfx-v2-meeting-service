// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package mocks

import (
	"context"

	inviteapi "github.com/linuxfoundation/lfx-v2-invite-service/pkg/api"
	"github.com/stretchr/testify/mock"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
)

// MockInviteSender implements domain.InviteSender for testing.
type MockInviteSender struct {
	mock.Mock
}

func (m *MockInviteSender) SendInvite(ctx context.Context, req inviteapi.SendInviteRequest) (domain.InviteResult, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(domain.InviteResult), args.Error(1)
}
