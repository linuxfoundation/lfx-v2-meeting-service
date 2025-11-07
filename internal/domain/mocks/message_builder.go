// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
)

// MockMessageBuilder implements MessageBuilder for testing
type MockMessageBuilder struct {
	mock.Mock
}

func (m *MockMessageBuilder) SendIndexMeeting(ctx context.Context, action models.MessageAction, data models.MeetingBase) error {
	args := m.Called(ctx, action, data)
	return args.Error(0)
}

func (m *MockMessageBuilder) SendDeleteIndexMeeting(ctx context.Context, data string) error {
	args := m.Called(ctx, data)
	return args.Error(0)
}

func (m *MockMessageBuilder) SendIndexMeetingSettings(ctx context.Context, action models.MessageAction, data models.MeetingSettings) error {
	args := m.Called(ctx, action, data)
	return args.Error(0)
}

func (m *MockMessageBuilder) SendDeleteIndexMeetingSettings(ctx context.Context, data string) error {
	args := m.Called(ctx, data)
	return args.Error(0)
}

func (m *MockMessageBuilder) SendIndexMeetingRegistrant(ctx context.Context, action models.MessageAction, data models.Registrant) error {
	args := m.Called(ctx, action, data)
	return args.Error(0)
}

func (m *MockMessageBuilder) SendDeleteIndexMeetingRegistrant(ctx context.Context, data string) error {
	args := m.Called(ctx, data)
	return args.Error(0)
}

func (m *MockMessageBuilder) SendIndexMeetingRSVP(ctx context.Context, action models.MessageAction, data models.RSVPResponse) error {
	args := m.Called(ctx, action, data)
	return args.Error(0)
}

func (m *MockMessageBuilder) SendDeleteIndexMeetingRSVP(ctx context.Context, data string) error {
	args := m.Called(ctx, data)
	return args.Error(0)
}

func (m *MockMessageBuilder) SendUpdateAccessMeeting(ctx context.Context, data models.MeetingAccessMessage) error {
	args := m.Called(ctx, data)
	return args.Error(0)
}

func (m *MockMessageBuilder) SendDeleteAllAccessMeeting(ctx context.Context, data string) error {
	args := m.Called(ctx, data)
	return args.Error(0)
}

func (m *MockMessageBuilder) SendPutMeetingRegistrantAccess(ctx context.Context, data models.MeetingRegistrantAccessMessage) error {
	args := m.Called(ctx, data)
	return args.Error(0)
}

func (m *MockMessageBuilder) SendRemoveMeetingRegistrantAccess(ctx context.Context, data models.MeetingRegistrantAccessMessage) error {
	args := m.Called(ctx, data)
	return args.Error(0)
}

func (m *MockMessageBuilder) PublishZoomWebhookEvent(ctx context.Context, subject string, message models.ZoomWebhookEventMessage) error {
	args := m.Called(ctx, subject, message)
	return args.Error(0)
}

func (m *MockMessageBuilder) SendMeetingDeleted(ctx context.Context, data models.MeetingDeletedMessage) error {
	args := m.Called(ctx, data)
	return args.Error(0)
}

func (m *MockMessageBuilder) SendMeetingCreated(ctx context.Context, data models.MeetingCreatedMessage) error {
	args := m.Called(ctx, data)
	return args.Error(0)
}

func (m *MockMessageBuilder) SendMeetingUpdated(ctx context.Context, data models.MeetingUpdatedMessage) error {
	args := m.Called(ctx, data)
	return args.Error(0)
}

func (m *MockMessageBuilder) SendIndexPastMeeting(ctx context.Context, action models.MessageAction, data models.PastMeeting) error {
	args := m.Called(ctx, action, data)
	return args.Error(0)
}

func (m *MockMessageBuilder) SendDeleteIndexPastMeeting(ctx context.Context, data string) error {
	args := m.Called(ctx, data)
	return args.Error(0)
}

func (m *MockMessageBuilder) SendIndexPastMeetingParticipant(ctx context.Context, action models.MessageAction, data models.PastMeetingParticipant) error {
	args := m.Called(ctx, action, data)
	return args.Error(0)
}

func (m *MockMessageBuilder) SendDeleteIndexPastMeetingParticipant(ctx context.Context, data string) error {
	args := m.Called(ctx, data)
	return args.Error(0)
}

func (m *MockMessageBuilder) SendUpdateAccessPastMeeting(ctx context.Context, data models.PastMeetingAccessMessage) error {
	args := m.Called(ctx, data)
	return args.Error(0)
}

func (m *MockMessageBuilder) SendDeleteAllAccessPastMeeting(ctx context.Context, data string) error {
	args := m.Called(ctx, data)
	return args.Error(0)
}

func (m *MockMessageBuilder) SendPutPastMeetingParticipantAccess(ctx context.Context, data models.PastMeetingParticipantAccessMessage) error {
	args := m.Called(ctx, data)
	return args.Error(0)
}

func (m *MockMessageBuilder) SendRemovePastMeetingParticipantAccess(ctx context.Context, data models.PastMeetingParticipantAccessMessage) error {
	args := m.Called(ctx, data)
	return args.Error(0)
}

func (m *MockMessageBuilder) SendIndexPastMeetingRecording(ctx context.Context, action models.MessageAction, data models.PastMeetingRecording) error {
	args := m.Called(ctx, action, data)
	return args.Error(0)
}

func (m *MockMessageBuilder) SendDeleteIndexPastMeetingRecording(ctx context.Context, data string) error {
	args := m.Called(ctx, data)
	return args.Error(0)
}

func (m *MockMessageBuilder) SendIndexPastMeetingSummary(ctx context.Context, action models.MessageAction, data models.PastMeetingSummary) error {
	args := m.Called(ctx, action, data)
	return args.Error(0)
}

func (m *MockMessageBuilder) SendDeleteIndexPastMeetingSummary(ctx context.Context, data string) error {
	args := m.Called(ctx, data)
	return args.Error(0)
}

func (m *MockMessageBuilder) SendIndexMeetingAttachment(ctx context.Context, action models.MessageAction, data models.MeetingAttachment) error {
	args := m.Called(ctx, action, data)
	return args.Error(0)
}

func (m *MockMessageBuilder) SendDeleteIndexMeetingAttachment(ctx context.Context, data string) error {
	args := m.Called(ctx, data)
	return args.Error(0)
}

func (m *MockMessageBuilder) SendUpdateAccessMeetingAttachment(ctx context.Context, data models.MeetingAttachmentAccessMessage) error {
	args := m.Called(ctx, data)
	return args.Error(0)
}

func (m *MockMessageBuilder) SendDeleteAccessMeetingAttachment(ctx context.Context, data string) error {
	args := m.Called(ctx, data)
	return args.Error(0)
}

func (m *MockMessageBuilder) SendIndexPastMeetingAttachment(ctx context.Context, action models.MessageAction, data models.PastMeetingAttachment) error {
	args := m.Called(ctx, action, data)
	return args.Error(0)
}

func (m *MockMessageBuilder) SendDeleteIndexPastMeetingAttachment(ctx context.Context, data string) error {
	args := m.Called(ctx, data)
	return args.Error(0)
}

func (m *MockMessageBuilder) SendUpdateAccessPastMeetingAttachment(ctx context.Context, data models.PastMeetingAttachmentAccessMessage) error {
	args := m.Called(ctx, data)
	return args.Error(0)
}

func (m *MockMessageBuilder) SendDeleteAccessPastMeetingAttachment(ctx context.Context, data string) error {
	args := m.Called(ctx, data)
	return args.Error(0)
}

func (m *MockMessageBuilder) SendIndexPastMeetingTranscript(ctx context.Context, action models.MessageAction, data models.PastMeetingTranscript) error {
	args := m.Called(ctx, action, data)
	return args.Error(0)
}

func (m *MockMessageBuilder) SendDeleteIndexPastMeetingTranscript(ctx context.Context, data string) error {
	args := m.Called(ctx, data)
	return args.Error(0)
}

func (m *MockMessageBuilder) SendUpdateAccessPastMeetingRecording(ctx context.Context, data models.PastMeetingRecordingAccessMessage) error {
	args := m.Called(ctx, data)
	return args.Error(0)
}

func (m *MockMessageBuilder) SendUpdateAccessPastMeetingTranscript(ctx context.Context, data models.PastMeetingTranscriptAccessMessage) error {
	args := m.Called(ctx, data)
	return args.Error(0)
}

func (m *MockMessageBuilder) SendUpdateAccessPastMeetingSummary(ctx context.Context, data models.PastMeetingSummaryAccessMessage) error {
	args := m.Called(ctx, data)
	return args.Error(0)
}

func (m *MockMessageBuilder) GetCommitteeName(ctx context.Context, committeeUID string) (string, error) {
	args := m.Called(ctx, committeeUID)
	return args.String(0), args.Error(1)
}

func (m *MockMessageBuilder) GetCommitteeMembers(ctx context.Context, committeeUID string) ([]models.CommitteeMember, error) {
	args := m.Called(ctx, committeeUID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.CommitteeMember), args.Error(1)
}

func (m *MockMessageBuilder) GetProjectName(ctx context.Context, projectUID string) (string, error) {
	args := m.Called(ctx, projectUID)
	return args.String(0), args.Error(1)
}

func (m *MockMessageBuilder) GetProjectLogo(ctx context.Context, projectUID string) (string, error) {
	args := m.Called(ctx, projectUID)
	return args.String(0), args.Error(1)
}

func (m *MockMessageBuilder) GetProjectSlug(ctx context.Context, projectUID string) (string, error) {
	args := m.Called(ctx, projectUID)
	return args.String(0), args.Error(1)
}

func (m *MockMessageBuilder) EmailToUsernameLookup(ctx context.Context, email string) (string, error) {
	args := m.Called(ctx, email)
	return args.String(0), args.Error(1)
}
