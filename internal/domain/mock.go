// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package domain

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
)

// MockMeetingRepository implements MeetingRepository for testing
type MockMeetingRepository struct {
	mock.Mock
}

func (m *MockMeetingRepository) GetBase(ctx context.Context, meetingUID string) (*models.MeetingBase, error) {
	args := m.Called(ctx, meetingUID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.MeetingBase), args.Error(1)
}

func (m *MockMeetingRepository) GetBaseWithRevision(ctx context.Context, meetingUID string) (*models.MeetingBase, uint64, error) {
	args := m.Called(ctx, meetingUID)
	if args.Get(0) == nil {
		return nil, args.Get(1).(uint64), args.Error(2)
	}
	return args.Get(0).(*models.MeetingBase), args.Get(1).(uint64), args.Error(2)
}

func (m *MockMeetingRepository) UpdateBase(ctx context.Context, meeting *models.MeetingBase, revision uint64) error {
	args := m.Called(ctx, meeting, revision)
	return args.Error(0)
}

func (m *MockMeetingRepository) GetSettings(ctx context.Context, meetingUID string) (*models.MeetingSettings, error) {
	args := m.Called(ctx, meetingUID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.MeetingSettings), args.Error(1)
}

func (m *MockMeetingRepository) GetSettingsWithRevision(ctx context.Context, meetingUID string) (*models.MeetingSettings, uint64, error) {
	args := m.Called(ctx, meetingUID)
	if args.Get(0) == nil {
		return nil, args.Get(1).(uint64), args.Error(2)
	}
	return args.Get(0).(*models.MeetingSettings), args.Get(1).(uint64), args.Error(2)
}

func (m *MockMeetingRepository) UpdateSettings(ctx context.Context, meetingSettings *models.MeetingSettings, revision uint64) error {
	args := m.Called(ctx, meetingSettings, revision)
	return args.Error(0)
}

func (m *MockMeetingRepository) Exists(ctx context.Context, meetingUID string) (bool, error) {
	args := m.Called(ctx, meetingUID)
	return args.Bool(0), args.Error(1)
}

func (m *MockMeetingRepository) ListAll(ctx context.Context) ([]*models.MeetingBase, []*models.MeetingSettings, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, nil, args.Error(2)
	}
	return args.Get(0).([]*models.MeetingBase), args.Get(1).([]*models.MeetingSettings), args.Error(2)
}

func (m *MockMeetingRepository) Create(ctx context.Context, meeting *models.MeetingBase, settings *models.MeetingSettings) error {
	args := m.Called(ctx, meeting, settings)
	return args.Error(0)
}

func (m *MockMeetingRepository) Delete(ctx context.Context, meetingUID string, revision uint64) error {
	args := m.Called(ctx, meetingUID, revision)
	return args.Error(0)
}

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

// MockMessage implements Message for testing
type MockMessage struct {
	mock.Mock
	data    []byte
	subject string
}

func (m *MockMessage) Subject() string {
	return m.subject
}

func (m *MockMessage) Data() []byte {
	return m.data
}

func (m *MockMessage) Respond(data []byte) error {
	args := m.Called(data)
	return args.Error(0)
}

// NewMockMessage creates a mock message for testing
func NewMockMessage(data []byte, subject string) *MockMessage {
	return &MockMessage{
		data:    data,
		subject: subject,
	}
}

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

// NewMockRegistrantRepository creates a new mock registrant repository for testing
func NewMockRegistrantRepository(t interface{ Cleanup(func()) }) *MockRegistrantRepository {
	return &MockRegistrantRepository{}
}

// MockPlatformProvider implements PlatformProvider for testing
type MockPlatformProvider struct {
	mock.Mock
}

func (m *MockPlatformProvider) CreateMeeting(ctx context.Context, meeting *models.MeetingBase) (*CreateMeetingResult, error) {
	args := m.Called(ctx, meeting)
	result := args.Get(0)
	if result == nil {
		return nil, args.Error(1)
	}
	return result.(*CreateMeetingResult), args.Error(1)
}

func (m *MockPlatformProvider) UpdateMeeting(ctx context.Context, platformMeetingID string, meeting *models.MeetingBase) error {
	args := m.Called(ctx, platformMeetingID, meeting)
	return args.Error(0)
}

func (m *MockPlatformProvider) DeleteMeeting(ctx context.Context, platformMeetingID string) error {
	args := m.Called(ctx, platformMeetingID)
	return args.Error(0)
}

func (m *MockPlatformProvider) StorePlatformData(meeting *models.MeetingBase, result *CreateMeetingResult) {
	m.Called(meeting, result)
}

func (m *MockPlatformProvider) GetPlatformMeetingID(meeting *models.MeetingBase) string {
	args := m.Called(meeting)
	return args.String(0)
}

// MockPlatformRegistry implements PlatformRegistry for testing
type MockPlatformRegistry struct {
	mock.Mock
}

func (m *MockPlatformRegistry) GetProvider(platform string) (PlatformProvider, error) {
	args := m.Called(platform)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(PlatformProvider), args.Error(1)
}

func (m *MockPlatformRegistry) RegisterProvider(platform string, provider PlatformProvider) {
	m.Called(platform, provider)
}

// MockWebhookHandler implements WebhookHandler for testing
type MockWebhookHandler struct {
	mock.Mock
}

func (m *MockWebhookHandler) HandleEvent(ctx context.Context, eventType string, payload interface{}) error {
	args := m.Called(ctx, eventType, payload)
	return args.Error(0)
}

func (m *MockWebhookHandler) ValidateSignature(body []byte, signature, timestamp string) error {
	args := m.Called(body, signature, timestamp)
	return args.Error(0)
}

func (m *MockWebhookHandler) SupportedEvents() []string {
	args := m.Called()
	return args.Get(0).([]string)
}

// MockWebhookRegistry implements WebhookRegistry for testing
type MockWebhookRegistry struct {
	mock.Mock
}

func (m *MockWebhookRegistry) GetHandler(platform string) (WebhookHandler, error) {
	args := m.Called(platform)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(WebhookHandler), args.Error(1)
}

func (m *MockWebhookRegistry) RegisterHandler(platform string, handler WebhookHandler) {
	m.Called(platform, handler)
}

// MockPastMeetingRepository implements PastMeetingRepository for testing
type MockPastMeetingRepository struct {
	mock.Mock
}

func (m *MockPastMeetingRepository) Create(ctx context.Context, pastMeeting *models.PastMeeting) error {
	args := m.Called(ctx, pastMeeting)
	return args.Error(0)
}

func (m *MockPastMeetingRepository) Exists(ctx context.Context, pastMeetingUID string) (bool, error) {
	args := m.Called(ctx, pastMeetingUID)
	return args.Bool(0), args.Error(1)
}

func (m *MockPastMeetingRepository) Delete(ctx context.Context, pastMeetingUID string, revision uint64) error {
	args := m.Called(ctx, pastMeetingUID, revision)
	return args.Error(0)
}

func (m *MockPastMeetingRepository) Get(ctx context.Context, pastMeetingUID string) (*models.PastMeeting, error) {
	args := m.Called(ctx, pastMeetingUID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.PastMeeting), args.Error(1)
}

func (m *MockPastMeetingRepository) GetWithRevision(ctx context.Context, pastMeetingUID string) (*models.PastMeeting, uint64, error) {
	args := m.Called(ctx, pastMeetingUID)
	if args.Get(0) == nil {
		return nil, args.Get(1).(uint64), args.Error(2)
	}
	return args.Get(0).(*models.PastMeeting), args.Get(1).(uint64), args.Error(2)
}

func (m *MockPastMeetingRepository) Update(ctx context.Context, pastMeeting *models.PastMeeting, revision uint64) error {
	args := m.Called(ctx, pastMeeting, revision)
	return args.Error(0)
}

func (m *MockPastMeetingRepository) ListByMeeting(ctx context.Context, meetingUID string) ([]*models.PastMeeting, error) {
	args := m.Called(ctx, meetingUID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.PastMeeting), args.Error(1)
}

func (m *MockPastMeetingRepository) GetByMeetingAndOccurrence(ctx context.Context, meetingUID, occurrenceID string) (*models.PastMeeting, error) {
	args := m.Called(ctx, meetingUID, occurrenceID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.PastMeeting), args.Error(1)
}

func (m *MockPastMeetingRepository) GetByPlatformMeetingID(ctx context.Context, platform, platformMeetingID string) (*models.PastMeeting, error) {
	args := m.Called(ctx, platform, platformMeetingID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.PastMeeting), args.Error(1)
}

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
