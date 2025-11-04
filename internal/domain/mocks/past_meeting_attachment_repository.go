// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
)

// MockPastMeetingAttachmentRepository implements PastMeetingAttachmentRepository for testing
type MockPastMeetingAttachmentRepository struct {
	mock.Mock
}

func (m *MockPastMeetingAttachmentRepository) PutMetadata(ctx context.Context, attachment *models.PastMeetingAttachment) error {
	args := m.Called(ctx, attachment)
	return args.Error(0)
}

func (m *MockPastMeetingAttachmentRepository) GetMetadata(ctx context.Context, attachmentUID string) (*models.PastMeetingAttachment, error) {
	args := m.Called(ctx, attachmentUID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.PastMeetingAttachment), args.Error(1)
}

func (m *MockPastMeetingAttachmentRepository) ListByPastMeeting(ctx context.Context, pastMeetingUID string) ([]*models.PastMeetingAttachment, error) {
	args := m.Called(ctx, pastMeetingUID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.PastMeetingAttachment), args.Error(1)
}

func (m *MockPastMeetingAttachmentRepository) Delete(ctx context.Context, attachmentUID string) error {
	args := m.Called(ctx, attachmentUID)
	return args.Error(0)
}
