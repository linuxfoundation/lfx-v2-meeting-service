// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
)

// MockMeetingAttachmentRepository implements MeetingAttachmentRepository for testing
type MockMeetingAttachmentRepository struct {
	mock.Mock
}

func (m *MockMeetingAttachmentRepository) PutObject(ctx context.Context, attachmentUID string, fileData []byte) error {
	args := m.Called(ctx, attachmentUID, fileData)
	return args.Error(0)
}

func (m *MockMeetingAttachmentRepository) PutMetadata(ctx context.Context, attachment *models.MeetingAttachment) error {
	args := m.Called(ctx, attachment)
	return args.Error(0)
}

func (m *MockMeetingAttachmentRepository) GetObject(ctx context.Context, attachmentUID string) ([]byte, error) {
	args := m.Called(ctx, attachmentUID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockMeetingAttachmentRepository) GetMetadata(ctx context.Context, attachmentUID string) (*models.MeetingAttachment, error) {
	args := m.Called(ctx, attachmentUID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.MeetingAttachment), args.Error(1)
}

func (m *MockMeetingAttachmentRepository) ListByMeeting(ctx context.Context, meetingUID string) ([]*models.MeetingAttachment, error) {
	args := m.Called(ctx, meetingUID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.MeetingAttachment), args.Error(1)
}

func (m *MockMeetingAttachmentRepository) Delete(ctx context.Context, attachmentUID string) error {
	args := m.Called(ctx, attachmentUID)
	return args.Error(0)
}
