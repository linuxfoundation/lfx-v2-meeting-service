// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package email

import (
	"context"
	"testing"
	"time"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	smtpmock "github.com/mocktools/go-smtp-mock/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestNewSMTPService(t *testing.T) {
	config := SMTPConfig{
		Host: "localhost",
		Port: 1025,
		From: "test@example.com",
	}

	service, err := NewSMTPService(config)
	require.NoError(t, err)
	assert.NotNil(t, service)
	assert.Equal(t, config, service.config)
	assert.NotNil(t, service.templateManager)
	assert.NotNil(t, service.icsGenerator)
}

func TestSMTPService_SendRegistrantInvitation(t *testing.T) {
	t.Run("successful invitation with ICS attachment", func(t *testing.T) {
		// Start mock SMTP server
		server := smtpmock.New(smtpmock.ConfigurationAttr{
			LogToStdout:       false,
			LogServerActivity: false,
		})
		require.NoError(t, server.Start())
		defer func() {
			require.NoError(t, server.Stop())
		}()

		// Setup mocks
		mockTemplateManager := new(MockTemplateManager)
		mockICSGenerator := new(MockICSGenerator)

		service := &SMTPService{
			config: SMTPConfig{
				Host: "localhost",
				Port: server.PortNumber(),
				From: "test@example.com",
			},
			icsGenerator:    mockICSGenerator,
			templateManager: mockTemplateManager,
		}

		// Create test data
		startTime := time.Now().Add(24 * time.Hour)
		invitation := domain.EmailInvitation{
			RecipientEmail: "recipient@example.com",
			RecipientName:  "Test Recipient",
			MeetingTitle:   "Test Meeting",
			MeetingUID:     "test-meeting-uid",
			Description:    "Test meeting description",
			StartTime:      startTime,
			Duration:       60,
			Timezone:       "America/New_York",
			JoinLink:       "https://zoom.us/j/123456789",
			MeetingID:      "123456789",
			Passcode:       "secret123",
			ProjectName:    "Test Project",
		}

		// Setup mock expectations
		mockICSGenerator.On("GenerateMeetingInvitationICS", mock.MatchedBy(func(params ICSMeetingInvitationParams) bool {
			return params.MeetingUID == invitation.MeetingUID &&
				params.MeetingTitle == invitation.MeetingTitle &&
				params.RecipientEmail == invitation.RecipientEmail
		})).Return("ICS_FILE_CONTENT", nil)

		mockTemplateManager.On("RenderInvitation", mock.MatchedBy(func(inv domain.EmailInvitation) bool {
			// Verify the invitation has the ICS attachment added
			return inv.RecipientEmail == invitation.RecipientEmail &&
				inv.MeetingTitle == invitation.MeetingTitle &&
				len(inv.EmailFileAttachments) > 0 && // Should have ICS attachment
				inv.ICSAttachment != nil
		})).Return(&RenderedEmail{
			HTML: "<html><body>Test HTML Content</body></html>",
			Text: "Test Text Content",
		}, nil)

		// Execute
		ctx := context.Background()
		err := service.SendRegistrantInvitation(ctx, invitation)

		// Verify no error
		assert.NoError(t, err)
		mockICSGenerator.AssertExpectations(t)
		mockTemplateManager.AssertExpectations(t)

		// Verify email was sent to mock server
		messages := server.Messages()
		require.Len(t, messages, 1)
		message := messages[0]

		// Verify recipient and sender
		rcptto := message.RcpttoRequestResponse()
		require.NotEmpty(t, rcptto)
		assert.Contains(t, rcptto[0][0], "recipient@example.com")
		assert.Contains(t, message.MailfromRequest(), "test@example.com")

		// Verify message content
		msgData := message.MsgRequest()
		assert.Contains(t, msgData, "Test Meeting")
		assert.Contains(t, msgData, "meeting-invitation.ics")
	})

	t.Run("template rendering failure", func(t *testing.T) {
		// Start mock SMTP server (not used but ensures test isolation)
		server := smtpmock.New(smtpmock.ConfigurationAttr{
			LogToStdout:       false,
			LogServerActivity: false,
		})
		require.NoError(t, server.Start())
		defer func() {
			require.NoError(t, server.Stop())
		}()

		// Setup mocks
		mockTemplateManager := new(MockTemplateManager)
		mockICSGenerator := new(MockICSGenerator)

		service := &SMTPService{
			config: SMTPConfig{
				Host: "localhost",
				Port: server.PortNumber(),
				From: "test@example.com",
			},
			icsGenerator:    mockICSGenerator,
			templateManager: mockTemplateManager,
		}

		// Create minimal test data
		invitation := domain.EmailInvitation{
			RecipientEmail: "recipient@example.com",
			MeetingTitle:   "Test Meeting",
			MeetingUID:     "test-uid",
			StartTime:      time.Now().Add(24 * time.Hour),
			Duration:       60,
			Timezone:       "UTC",
		}

		// Setup mock expectations - ICS generation succeeds
		mockICSGenerator.On("GenerateMeetingInvitationICS", mock.Anything).
			Return("ICS_CONTENT", nil)

		// Template rendering fails
		mockTemplateManager.On("RenderInvitation", mock.Anything).
			Return(nil, assert.AnError)

		// Execute
		ctx := context.Background()
		err := service.SendRegistrantInvitation(ctx, invitation)

		// Verify error occurred before reaching SMTP
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to render invitation template")
		mockICSGenerator.AssertExpectations(t)
		mockTemplateManager.AssertExpectations(t)

		// No email should have been sent
		assert.Empty(t, server.Messages())
	})

	t.Run("invitation with recurrence", func(t *testing.T) {
		// Start mock SMTP server
		server := smtpmock.New(smtpmock.ConfigurationAttr{
			LogToStdout:       false,
			LogServerActivity: false,
		})
		require.NoError(t, server.Start())
		defer func() {
			require.NoError(t, server.Stop())
		}()

		// Setup mocks
		mockTemplateManager := new(MockTemplateManager)
		mockICSGenerator := new(MockICSGenerator)

		service := &SMTPService{
			config: SMTPConfig{
				Host: "localhost",
				Port: server.PortNumber(),
				From: "test@example.com",
			},
			icsGenerator:    mockICSGenerator,
			templateManager: mockTemplateManager,
		}

		// Create invitation with weekly recurrence
		recurrence := &models.Recurrence{
			Type:           2, // Weekly
			RepeatInterval: 1,
			WeeklyDays:     "2,4,6", // Mon, Wed, Fri
			EndTimes:       10,
		}

		invitation := domain.EmailInvitation{
			RecipientEmail: "recipient@example.com",
			MeetingTitle:   "Recurring Meeting",
			MeetingUID:     "recurring-uid",
			StartTime:      time.Now().Add(24 * time.Hour),
			Duration:       60,
			Timezone:       "America/Los_Angeles",
			Recurrence:     recurrence,
		}

		// Setup mocks
		mockICSGenerator.On("GenerateMeetingInvitationICS", mock.MatchedBy(func(params ICSMeetingInvitationParams) bool {
			return params.Recurrence != nil &&
				params.Recurrence.Type == 2 &&
				params.Recurrence.WeeklyDays == "2,4,6"
		})).Return("ICS_RECURRING_CONTENT", nil)

		mockTemplateManager.On("RenderInvitation", mock.Anything).
			Return(&RenderedEmail{HTML: "<html>Recurring</html>", Text: "Recurring"}, nil)

		// Execute
		ctx := context.Background()
		err := service.SendRegistrantInvitation(ctx, invitation)

		// Verify success
		assert.NoError(t, err)
		mockICSGenerator.AssertExpectations(t)
		mockTemplateManager.AssertExpectations(t)

		// Verify email was sent
		messages := server.Messages()
		require.Len(t, messages, 1)
		assert.Contains(t, messages[0].MsgRequest(), "Recurring Meeting")
	})
}

func TestSMTPService_SendRegistrantCancellation(t *testing.T) {
	t.Run("successful cancellation for future meeting", func(t *testing.T) {
		// Start mock SMTP server
		server := smtpmock.New(smtpmock.ConfigurationAttr{
			LogToStdout:       false,
			LogServerActivity: false,
		})
		require.NoError(t, server.Start())
		defer func() {
			require.NoError(t, server.Stop())
		}()

		// Setup mocks
		mockTemplateManager := new(MockTemplateManager)
		mockICSGenerator := new(MockICSGenerator)

		service := &SMTPService{
			config: SMTPConfig{
				Host: "localhost",
				Port: server.PortNumber(),
				From: "test@example.com",
			},
			icsGenerator:    mockICSGenerator,
			templateManager: mockTemplateManager,
		}

		// Create test data for future meeting
		startTime := time.Now().Add(24 * time.Hour)
		cancellation := domain.EmailCancellation{
			RecipientEmail: "recipient@example.com",
			RecipientName:  "Test Recipient",
			MeetingTitle:   "Cancelled Meeting",
			MeetingUID:     "cancel-uid",
			StartTime:      startTime,
			Duration:       60,
			Timezone:       "America/New_York",
			ProjectName:    "Test Project",
		}

		// Setup mock expectations
		mockICSGenerator.On("GenerateMeetingCancellationICS", mock.MatchedBy(func(params ICSMeetingCancellationParams) bool {
			return params.MeetingUID == cancellation.MeetingUID &&
				params.MeetingTitle == cancellation.MeetingTitle &&
				params.RecipientEmail == cancellation.RecipientEmail
		})).Return("ICS_CANCELLATION_CONTENT", nil)

		mockTemplateManager.On("RenderCancellation", mock.MatchedBy(func(cancel domain.EmailCancellation) bool {
			return cancel.RecipientEmail == cancellation.RecipientEmail &&
				cancel.MeetingTitle == cancellation.MeetingTitle &&
				cancel.ICSAttachment != nil // Should have ICS cancellation attachment
		})).Return(&RenderedEmail{
			HTML: "<html><body>Cancellation HTML</body></html>",
			Text: "Cancellation Text",
		}, nil)

		// Execute
		ctx := context.Background()
		err := service.SendRegistrantCancellation(ctx, cancellation)

		// Verify no error
		assert.NoError(t, err)
		mockICSGenerator.AssertExpectations(t)
		mockTemplateManager.AssertExpectations(t)

		// Verify email was sent to mock server
		messages := server.Messages()
		require.Len(t, messages, 1)
		message := messages[0]

		// Verify recipient and sender
		rcptto := message.RcpttoRequestResponse()
		require.NotEmpty(t, rcptto)
		assert.Contains(t, rcptto[0][0], "recipient@example.com")
		assert.Contains(t, message.MailfromRequest(), "test@example.com")

		// Verify message content
		msgData := message.MsgRequest()
		assert.Contains(t, msgData, "Cancelled Meeting")
		assert.Contains(t, msgData, "cancellation.ics")
	})

	t.Run("cancellation for past meeting without ICS", func(t *testing.T) {
		// Start mock SMTP server
		server := smtpmock.New(smtpmock.ConfigurationAttr{
			LogToStdout:       false,
			LogServerActivity: false,
		})
		require.NoError(t, server.Start())
		defer func() {
			require.NoError(t, server.Stop())
		}()

		// Setup mocks
		mockTemplateManager := new(MockTemplateManager)
		mockICSGenerator := new(MockICSGenerator)

		service := &SMTPService{
			config: SMTPConfig{
				Host: "localhost",
				Port: server.PortNumber(),
				From: "test@example.com",
			},
			icsGenerator:    mockICSGenerator,
			templateManager: mockTemplateManager,
		}

		// Create test data for past meeting (no ICS should be generated)
		startTime := time.Now().Add(-2 * time.Hour)
		cancellation := domain.EmailCancellation{
			RecipientEmail: "recipient@example.com",
			RecipientName:  "Test Recipient",
			MeetingTitle:   "Past Meeting",
			MeetingUID:     "past-uid",
			StartTime:      startTime,
			Duration:       60,
			Timezone:       "UTC",
			ProjectName:    "Test Project",
		}

		// ICS generator should not be called for past meetings
		mockTemplateManager.On("RenderCancellation", mock.MatchedBy(func(cancel domain.EmailCancellation) bool {
			return cancel.RecipientEmail == cancellation.RecipientEmail &&
				cancel.ICSAttachment == nil // No ICS for past meeting
		})).Return(&RenderedEmail{
			HTML: "<html>Past Meeting Cancellation</html>",
			Text: "Past Meeting Cancellation",
		}, nil)

		// Execute
		ctx := context.Background()
		err := service.SendRegistrantCancellation(ctx, cancellation)

		// Verify no error
		assert.NoError(t, err)
		mockICSGenerator.AssertNotCalled(t, "GenerateMeetingCancellationICS")
		mockTemplateManager.AssertExpectations(t)

		// Verify email was sent
		messages := server.Messages()
		require.Len(t, messages, 1)
		assert.Contains(t, messages[0].MsgRequest(), "Past Meeting")
	})

	t.Run("cancellation with recurrence", func(t *testing.T) {
		// Start mock SMTP server
		server := smtpmock.New(smtpmock.ConfigurationAttr{
			LogToStdout:       false,
			LogServerActivity: false,
		})
		require.NoError(t, server.Start())
		defer func() {
			require.NoError(t, server.Stop())
		}()

		// Setup mocks
		mockTemplateManager := new(MockTemplateManager)
		mockICSGenerator := new(MockICSGenerator)

		service := &SMTPService{
			config: SMTPConfig{
				Host: "localhost",
				Port: server.PortNumber(),
				From: "test@example.com",
			},
			icsGenerator:    mockICSGenerator,
			templateManager: mockTemplateManager,
		}

		// Create cancellation with recurrence
		startTime := time.Now().Add(24 * time.Hour)
		recurrence := &models.Recurrence{
			Type:           1, // Daily
			RepeatInterval: 1,
			EndTimes:       5,
		}

		cancellation := domain.EmailCancellation{
			RecipientEmail: "recipient@example.com",
			RecipientName:  "Test Recipient",
			MeetingTitle:   "Recurring Meeting Cancelled",
			MeetingUID:     "recurring-cancel-uid",
			StartTime:      startTime,
			Duration:       30,
			Timezone:       "UTC",
			Recurrence:     recurrence,
			ProjectName:    "Test Project",
		}

		// Setup mock expectations
		mockICSGenerator.On("GenerateMeetingCancellationICS", mock.MatchedBy(func(params ICSMeetingCancellationParams) bool {
			return params.Recurrence != nil &&
				params.Recurrence.Type == 1
		})).Return("ICS_RECURRING_CANCELLATION", nil)

		mockTemplateManager.On("RenderCancellation", mock.Anything).
			Return(&RenderedEmail{HTML: "<html>Recurring Cancellation</html>", Text: "Recurring Cancellation"}, nil)

		// Execute
		ctx := context.Background()
		err := service.SendRegistrantCancellation(ctx, cancellation)

		// Verify success
		assert.NoError(t, err)
		mockICSGenerator.AssertExpectations(t)
		mockTemplateManager.AssertExpectations(t)

		// Verify email was sent
		messages := server.Messages()
		require.Len(t, messages, 1)
		assert.Contains(t, messages[0].MsgRequest(), "Recurring Meeting Cancelled")
	})

	t.Run("template rendering failure", func(t *testing.T) {
		// Start mock SMTP server
		server := smtpmock.New(smtpmock.ConfigurationAttr{
			LogToStdout:       false,
			LogServerActivity: false,
		})
		require.NoError(t, server.Start())
		defer func() {
			require.NoError(t, server.Stop())
		}()

		// Setup mocks
		mockTemplateManager := new(MockTemplateManager)
		mockICSGenerator := new(MockICSGenerator)

		service := &SMTPService{
			config: SMTPConfig{
				Host: "localhost",
				Port: server.PortNumber(),
				From: "test@example.com",
			},
			icsGenerator:    mockICSGenerator,
			templateManager: mockTemplateManager,
		}

		// Create test data
		cancellation := domain.EmailCancellation{
			RecipientEmail: "recipient@example.com",
			MeetingTitle:   "Test Meeting",
			MeetingUID:     "test-uid",
			StartTime:      time.Now().Add(24 * time.Hour),
			Duration:       60,
			Timezone:       "UTC",
			ProjectName:    "Test Project",
		}

		// ICS generation succeeds
		mockICSGenerator.On("GenerateMeetingCancellationICS", mock.Anything).
			Return("ICS_CONTENT", nil)

		// Template rendering fails
		mockTemplateManager.On("RenderCancellation", mock.Anything).
			Return(nil, assert.AnError)

		// Execute
		ctx := context.Background()
		err := service.SendRegistrantCancellation(ctx, cancellation)

		// Verify error occurred before reaching SMTP
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to render cancellation template")
		mockICSGenerator.AssertExpectations(t)
		mockTemplateManager.AssertExpectations(t)

		// No email should have been sent
		assert.Empty(t, server.Messages())
	})

	t.Run("ICS generation failure continues without attachment", func(t *testing.T) {
		// Start mock SMTP server
		server := smtpmock.New(smtpmock.ConfigurationAttr{
			LogToStdout:       false,
			LogServerActivity: false,
		})
		require.NoError(t, server.Start())
		defer func() {
			require.NoError(t, server.Stop())
		}()

		// Setup mocks
		mockTemplateManager := new(MockTemplateManager)
		mockICSGenerator := new(MockICSGenerator)

		service := &SMTPService{
			config: SMTPConfig{
				Host: "localhost",
				Port: server.PortNumber(),
				From: "test@example.com",
			},
			icsGenerator:    mockICSGenerator,
			templateManager: mockTemplateManager,
		}

		// Create test data
		cancellation := domain.EmailCancellation{
			RecipientEmail: "recipient@example.com",
			MeetingTitle:   "Test Meeting",
			MeetingUID:     "test-uid",
			StartTime:      time.Now().Add(24 * time.Hour),
			Duration:       60,
			Timezone:       "UTC",
			ProjectName:    "Test Project",
		}

		// ICS generation fails
		mockICSGenerator.On("GenerateMeetingCancellationICS", mock.Anything).
			Return("", assert.AnError)

		// Template rendering succeeds (without ICS attachment)
		mockTemplateManager.On("RenderCancellation", mock.MatchedBy(func(cancel domain.EmailCancellation) bool {
			return cancel.ICSAttachment == nil // No ICS due to generation failure
		})).Return(&RenderedEmail{
			HTML: "<html>Cancellation</html>",
			Text: "Cancellation",
		}, nil)

		// Execute
		ctx := context.Background()
		err := service.SendRegistrantCancellation(ctx, cancellation)

		// Verify success (email sent without ICS)
		assert.NoError(t, err)
		mockICSGenerator.AssertExpectations(t)
		mockTemplateManager.AssertExpectations(t)

		// Verify email was sent
		messages := server.Messages()
		require.Len(t, messages, 1)
	})
}

func TestSMTPService_SendOccurrenceCancellation(t *testing.T) {
	t.Run("successful occurrence cancellation for future occurrence", func(t *testing.T) {
		// Start mock SMTP server
		server := smtpmock.New(smtpmock.ConfigurationAttr{
			LogToStdout:       false,
			LogServerActivity: false,
		})
		require.NoError(t, server.Start())
		defer func() {
			require.NoError(t, server.Stop())
		}()

		// Setup mocks
		mockTemplateManager := new(MockTemplateManager)
		mockICSGenerator := new(MockICSGenerator)

		service := &SMTPService{
			config: SMTPConfig{
				Host: "localhost",
				Port: server.PortNumber(),
				From: "test@example.com",
			},
			icsGenerator:    mockICSGenerator,
			templateManager: mockTemplateManager,
		}

		// Create test data for future occurrence
		occurrenceStartTime := time.Now().Add(48 * time.Hour)
		cancellation := domain.EmailOccurrenceCancellation{
			RecipientEmail:      "recipient@example.com",
			RecipientName:       "Test Recipient",
			MeetingTitle:        "Weekly Standup",
			MeetingUID:          "weekly-standup-uid",
			OccurrenceID:        "occurrence-123",
			OccurrenceStartTime: occurrenceStartTime,
			Duration:            30,
			Timezone:            "America/New_York",
			ProjectName:         "Test Project",
		}

		// Setup mock expectations
		mockICSGenerator.On("GenerateOccurrenceCancellationICS", mock.MatchedBy(func(params ICSOccurrenceCancellationParams) bool {
			return params.MeetingUID == cancellation.MeetingUID &&
				params.MeetingTitle == cancellation.MeetingTitle &&
				params.OccurrenceStartTime.Equal(cancellation.OccurrenceStartTime)
		})).Return("ICS_OCCURRENCE_CANCELLATION", nil)

		mockTemplateManager.On("RenderOccurrenceCancellation", mock.MatchedBy(func(cancel domain.EmailOccurrenceCancellation) bool {
			return cancel.RecipientEmail == cancellation.RecipientEmail &&
				cancel.MeetingTitle == cancellation.MeetingTitle &&
				cancel.ICSAttachment != nil // Should have ICS cancellation attachment
		})).Return(&RenderedEmail{
			HTML: "<html><body>Occurrence Cancellation HTML</body></html>",
			Text: "Occurrence Cancellation Text",
		}, nil)

		// Execute
		ctx := context.Background()
		err := service.SendOccurrenceCancellation(ctx, cancellation)

		// Verify no error
		assert.NoError(t, err)
		mockICSGenerator.AssertExpectations(t)
		mockTemplateManager.AssertExpectations(t)

		// Verify email was sent to mock server
		messages := server.Messages()
		require.Len(t, messages, 1)
		message := messages[0]

		// Verify recipient and sender
		rcptto := message.RcpttoRequestResponse()
		require.NotEmpty(t, rcptto)
		assert.Contains(t, rcptto[0][0], "recipient@example.com")
		assert.Contains(t, message.MailfromRequest(), "test@example.com")

		// Verify message content
		msgData := message.MsgRequest()
		assert.Contains(t, msgData, "Weekly Standup")
		assert.Contains(t, msgData, "occurrence-cancellation.ics")
	})

	t.Run("occurrence cancellation for past occurrence without ICS", func(t *testing.T) {
		// Start mock SMTP server
		server := smtpmock.New(smtpmock.ConfigurationAttr{
			LogToStdout:       false,
			LogServerActivity: false,
		})
		require.NoError(t, server.Start())
		defer func() {
			require.NoError(t, server.Stop())
		}()

		// Setup mocks
		mockTemplateManager := new(MockTemplateManager)
		mockICSGenerator := new(MockICSGenerator)

		service := &SMTPService{
			config: SMTPConfig{
				Host: "localhost",
				Port: server.PortNumber(),
				From: "test@example.com",
			},
			icsGenerator:    mockICSGenerator,
			templateManager: mockTemplateManager,
		}

		// Create test data for past occurrence (no ICS should be generated)
		occurrenceStartTime := time.Now().Add(-3 * time.Hour)
		cancellation := domain.EmailOccurrenceCancellation{
			RecipientEmail:      "recipient@example.com",
			RecipientName:       "Test Recipient",
			MeetingTitle:        "Past Occurrence",
			MeetingUID:          "past-occurrence-uid",
			OccurrenceID:        "occurrence-past",
			OccurrenceStartTime: occurrenceStartTime,
			Duration:            60,
			Timezone:            "UTC",
			ProjectName:         "Test Project",
		}

		// ICS generator should not be called for past occurrences
		mockTemplateManager.On("RenderOccurrenceCancellation", mock.MatchedBy(func(cancel domain.EmailOccurrenceCancellation) bool {
			return cancel.RecipientEmail == cancellation.RecipientEmail &&
				cancel.ICSAttachment == nil // No ICS for past occurrence
		})).Return(&RenderedEmail{
			HTML: "<html>Past Occurrence Cancellation</html>",
			Text: "Past Occurrence Cancellation",
		}, nil)

		// Execute
		ctx := context.Background()
		err := service.SendOccurrenceCancellation(ctx, cancellation)

		// Verify no error
		assert.NoError(t, err)
		mockICSGenerator.AssertNotCalled(t, "GenerateOccurrenceCancellationICS")
		mockTemplateManager.AssertExpectations(t)

		// Verify email was sent
		messages := server.Messages()
		require.Len(t, messages, 1)
		assert.Contains(t, messages[0].MsgRequest(), "Past Occurrence")
	})

	t.Run("template rendering failure", func(t *testing.T) {
		// Start mock SMTP server
		server := smtpmock.New(smtpmock.ConfigurationAttr{
			LogToStdout:       false,
			LogServerActivity: false,
		})
		require.NoError(t, server.Start())
		defer func() {
			require.NoError(t, server.Stop())
		}()

		// Setup mocks
		mockTemplateManager := new(MockTemplateManager)
		mockICSGenerator := new(MockICSGenerator)

		service := &SMTPService{
			config: SMTPConfig{
				Host: "localhost",
				Port: server.PortNumber(),
				From: "test@example.com",
			},
			icsGenerator:    mockICSGenerator,
			templateManager: mockTemplateManager,
		}

		// Create test data
		cancellation := domain.EmailOccurrenceCancellation{
			RecipientEmail:      "recipient@example.com",
			MeetingTitle:        "Test Meeting",
			MeetingUID:          "test-uid",
			OccurrenceID:        "occurrence-1",
			OccurrenceStartTime: time.Now().Add(24 * time.Hour),
			Duration:            60,
			Timezone:            "UTC",
			ProjectName:         "Test Project",
		}

		// ICS generation succeeds
		mockICSGenerator.On("GenerateOccurrenceCancellationICS", mock.Anything).
			Return("ICS_CONTENT", nil)

		// Template rendering fails
		mockTemplateManager.On("RenderOccurrenceCancellation", mock.Anything).
			Return(nil, assert.AnError)

		// Execute
		ctx := context.Background()
		err := service.SendOccurrenceCancellation(ctx, cancellation)

		// Verify error occurred before reaching SMTP
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to render occurrence cancellation template")
		mockICSGenerator.AssertExpectations(t)
		mockTemplateManager.AssertExpectations(t)

		// No email should have been sent
		assert.Empty(t, server.Messages())
	})

	t.Run("ICS generation failure continues without attachment", func(t *testing.T) {
		// Start mock SMTP server
		server := smtpmock.New(smtpmock.ConfigurationAttr{
			LogToStdout:       false,
			LogServerActivity: false,
		})
		require.NoError(t, server.Start())
		defer func() {
			require.NoError(t, server.Stop())
		}()

		// Setup mocks
		mockTemplateManager := new(MockTemplateManager)
		mockICSGenerator := new(MockICSGenerator)

		service := &SMTPService{
			config: SMTPConfig{
				Host: "localhost",
				Port: server.PortNumber(),
				From: "test@example.com",
			},
			icsGenerator:    mockICSGenerator,
			templateManager: mockTemplateManager,
		}

		// Create test data
		cancellation := domain.EmailOccurrenceCancellation{
			RecipientEmail:      "recipient@example.com",
			MeetingTitle:        "Test Meeting",
			MeetingUID:          "test-uid",
			OccurrenceID:        "occurrence-1",
			OccurrenceStartTime: time.Now().Add(24 * time.Hour),
			Duration:            60,
			Timezone:            "UTC",
			ProjectName:         "Test Project",
		}

		// ICS generation fails
		mockICSGenerator.On("GenerateOccurrenceCancellationICS", mock.Anything).
			Return("", assert.AnError)

		// Template rendering succeeds (without ICS attachment)
		mockTemplateManager.On("RenderOccurrenceCancellation", mock.MatchedBy(func(cancel domain.EmailOccurrenceCancellation) bool {
			return cancel.ICSAttachment == nil // No ICS due to generation failure
		})).Return(&RenderedEmail{
			HTML: "<html>Occurrence Cancellation</html>",
			Text: "Occurrence Cancellation",
		}, nil)

		// Execute
		ctx := context.Background()
		err := service.SendOccurrenceCancellation(ctx, cancellation)

		// Verify success (email sent without ICS)
		assert.NoError(t, err)
		mockICSGenerator.AssertExpectations(t)
		mockTemplateManager.AssertExpectations(t)

		// Verify email was sent
		messages := server.Messages()
		require.Len(t, messages, 1)
	})
}

func TestSMTPService_SendRegistrantUpdatedInvitation(t *testing.T) {
	t.Run("successful updated invitation for future meeting", func(t *testing.T) {
		// Start mock SMTP server
		server := smtpmock.New(smtpmock.ConfigurationAttr{
			LogToStdout:       false,
			LogServerActivity: false,
		})
		require.NoError(t, server.Start())
		defer func() {
			require.NoError(t, server.Stop())
		}()

		mockICSGenerator := new(MockICSGenerator)
		mockTemplateManager := new(MockTemplateManager)

		service := &SMTPService{
			config: SMTPConfig{
				Host: "localhost",
				Port: server.PortNumber(),
				From: "test@example.com",
			},
			icsGenerator:    mockICSGenerator,
			templateManager: mockTemplateManager,
		}

		startTime := time.Now().Add(24 * time.Hour)
		updatedInvitation := domain.EmailUpdatedInvitation{
			RecipientEmail: "recipient@example.com",
			RecipientName:  "Test Recipient",
			MeetingTitle:   "Updated Meeting",
			MeetingUID:     "updated-meeting-uid",
			Description:    "Meeting details have been updated",
			StartTime:      startTime,
			Duration:       45,
			Timezone:       "America/New_York",
			JoinLink:       "https://zoom.us/j/updated123",
			MeetingID:      "updated123",
			Passcode:       "updated456",
			ProjectName:    "Test Project",
			IcsSequence:    2, // Indicates this is an update
		}

		// Setup mock expectations for ICS generation
		mockICSGenerator.On("GenerateMeetingUpdateICS", mock.MatchedBy(func(params ICSMeetingUpdateParams) bool {
			return params.MeetingUID == "updated-meeting-uid" &&
				params.MeetingTitle == "Updated Meeting" &&
				params.Sequence == 2
		})).Return("ICS_UPDATE_CONTENT", nil)

		// Setup mock expectations for template rendering
		mockTemplateManager.On("RenderUpdatedInvitation", mock.MatchedBy(func(inv domain.EmailUpdatedInvitation) bool {
			return inv.RecipientEmail == "recipient@example.com" &&
				inv.MeetingTitle == "Updated Meeting"
		})).Return(&RenderedEmail{
			HTML: "<html>Updated Meeting Invitation</html>",
			Text: "Updated Meeting Invitation",
		}, nil)

		ctx := context.Background()
		err := service.SendRegistrantUpdatedInvitation(ctx, updatedInvitation)

		assert.NoError(t, err)
		mockICSGenerator.AssertExpectations(t)
		mockTemplateManager.AssertExpectations(t)

		// Verify email was sent
		messages := server.Messages()
		require.Len(t, messages, 1)
		message := messages[0]

		// Verify recipient and sender
		rcptto := message.RcpttoRequestResponse()
		require.NotEmpty(t, rcptto)
		assert.Contains(t, rcptto[0][0], "recipient@example.com")
		assert.Contains(t, message.MailfromRequest(), "test@example.com")

		// Verify message content - ICS is base64 encoded in attachment
		msgData := message.MsgRequest()
		assert.Contains(t, msgData, "Updated Meeting")
		// Verify the ICS attachment filename is present
		assert.Contains(t, msgData, "Updated_Meeting-updated.ics")
	})

	t.Run("updated invitation for past meeting without ICS", func(t *testing.T) {
		// Start mock SMTP server
		server := smtpmock.New(smtpmock.ConfigurationAttr{
			LogToStdout:       false,
			LogServerActivity: false,
		})
		require.NoError(t, server.Start())
		defer func() {
			require.NoError(t, server.Stop())
		}()

		mockICSGenerator := new(MockICSGenerator)
		mockTemplateManager := new(MockTemplateManager)

		service := &SMTPService{
			config: SMTPConfig{
				Host: "localhost",
				Port: server.PortNumber(),
				From: "test@example.com",
			},
			icsGenerator:    mockICSGenerator,
			templateManager: mockTemplateManager,
		}

		startTime := time.Now().Add(-2 * time.Hour) // Past meeting
		updatedInvitation := domain.EmailUpdatedInvitation{
			RecipientEmail: "recipient@example.com",
			RecipientName:  "Test Recipient",
			MeetingTitle:   "Past Updated Meeting",
			MeetingUID:     "past-updated-meeting-uid",
			StartTime:      startTime,
			Duration:       30,
			Timezone:       "America/New_York",
			ProjectName:    "Test Project",
		}

		// Setup mock expectations - ICS should NOT be generated for past meetings
		mockTemplateManager.On("RenderUpdatedInvitation", mock.Anything).Return(&RenderedEmail{
			HTML: "<html>Past Updated Meeting</html>",
			Text: "Past Updated Meeting",
		}, nil)

		ctx := context.Background()
		err := service.SendRegistrantUpdatedInvitation(ctx, updatedInvitation)

		assert.NoError(t, err)
		mockICSGenerator.AssertNotCalled(t, "GenerateMeetingUpdateICS")
		mockTemplateManager.AssertExpectations(t)

		// Verify email was sent without ICS attachment
		messages := server.Messages()
		require.Len(t, messages, 1)
	})

	t.Run("updated invitation with recurrence", func(t *testing.T) {
		// Start mock SMTP server
		server := smtpmock.New(smtpmock.ConfigurationAttr{
			LogToStdout:       false,
			LogServerActivity: false,
		})
		require.NoError(t, server.Start())
		defer func() {
			require.NoError(t, server.Stop())
		}()

		mockICSGenerator := new(MockICSGenerator)
		mockTemplateManager := new(MockTemplateManager)

		service := &SMTPService{
			config: SMTPConfig{
				Host: "localhost",
				Port: server.PortNumber(),
				From: "test@example.com",
			},
			icsGenerator:    mockICSGenerator,
			templateManager: mockTemplateManager,
		}

		startTime := time.Now().Add(24 * time.Hour)
		recurrence := &models.Recurrence{
			Type:           2, // Weekly
			RepeatInterval: 1,
			EndTimes:       10,
		}

		updatedInvitation := domain.EmailUpdatedInvitation{
			RecipientEmail: "recipient@example.com",
			RecipientName:  "Test Recipient",
			MeetingTitle:   "Updated Recurring Meeting",
			MeetingUID:     "recurring-meeting-uid",
			StartTime:      startTime,
			Duration:       60,
			Timezone:       "America/New_York",
			ProjectName:    "Test Project",
			Recurrence:     recurrence,
			IcsSequence:    3,
		}

		// Setup mock expectations
		mockICSGenerator.On("GenerateMeetingUpdateICS", mock.MatchedBy(func(params ICSMeetingUpdateParams) bool {
			return params.Recurrence != nil &&
				params.Recurrence.Type == 2 &&
				params.Sequence == 3
		})).Return("ICS_RECURRING_UPDATE_CONTENT", nil)

		mockTemplateManager.On("RenderUpdatedInvitation", mock.Anything).Return(&RenderedEmail{
			HTML: "<html>Updated Recurring Meeting</html>",
			Text: "Updated Recurring Meeting",
		}, nil)

		ctx := context.Background()
		err := service.SendRegistrantUpdatedInvitation(ctx, updatedInvitation)

		assert.NoError(t, err)
		mockICSGenerator.AssertExpectations(t)
		mockTemplateManager.AssertExpectations(t)

		// Verify email was sent
		messages := server.Messages()
		require.Len(t, messages, 1)
	})

	t.Run("template rendering failure", func(t *testing.T) {
		// Start mock SMTP server
		server := smtpmock.New(smtpmock.ConfigurationAttr{
			LogToStdout:       false,
			LogServerActivity: false,
		})
		require.NoError(t, server.Start())
		defer func() {
			require.NoError(t, server.Stop())
		}()

		mockICSGenerator := new(MockICSGenerator)
		mockTemplateManager := new(MockTemplateManager)

		service := &SMTPService{
			config: SMTPConfig{
				Host: "localhost",
				Port: server.PortNumber(),
				From: "test@example.com",
			},
			icsGenerator:    mockICSGenerator,
			templateManager: mockTemplateManager,
		}

		startTime := time.Now().Add(24 * time.Hour)
		updatedInvitation := domain.EmailUpdatedInvitation{
			RecipientEmail: "recipient@example.com",
			MeetingTitle:   "Test Meeting",
			StartTime:      startTime,
			Duration:       30,
			Timezone:       "America/New_York",
			ProjectName:    "Test Project",
		}

		// Setup mocks - template rendering fails
		mockICSGenerator.On("GenerateMeetingUpdateICS", mock.Anything).Return("ICS_CONTENT", nil)
		mockTemplateManager.On("RenderUpdatedInvitation", mock.Anything).Return(nil, assert.AnError)

		ctx := context.Background()
		err := service.SendRegistrantUpdatedInvitation(ctx, updatedInvitation)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to render updated invitation template")

		// Verify no email was sent
		assert.Empty(t, server.Messages())
	})

	t.Run("ICS generation failure continues without attachment", func(t *testing.T) {
		// Start mock SMTP server
		server := smtpmock.New(smtpmock.ConfigurationAttr{
			LogToStdout:       false,
			LogServerActivity: false,
		})
		require.NoError(t, server.Start())
		defer func() {
			require.NoError(t, server.Stop())
		}()

		mockICSGenerator := new(MockICSGenerator)
		mockTemplateManager := new(MockTemplateManager)

		service := &SMTPService{
			config: SMTPConfig{
				Host: "localhost",
				Port: server.PortNumber(),
				From: "test@example.com",
			},
			icsGenerator:    mockICSGenerator,
			templateManager: mockTemplateManager,
		}

		startTime := time.Now().Add(24 * time.Hour)
		updatedInvitation := domain.EmailUpdatedInvitation{
			RecipientEmail: "recipient@example.com",
			MeetingTitle:   "Test Meeting",
			StartTime:      startTime,
			Duration:       30,
			Timezone:       "America/New_York",
			ProjectName:    "Test Project",
		}

		// Setup mocks - ICS generation fails but email should still be sent
		mockICSGenerator.On("GenerateMeetingUpdateICS", mock.Anything).Return("", assert.AnError)
		mockTemplateManager.On("RenderUpdatedInvitation", mock.Anything).Return(&RenderedEmail{
			HTML: "<html>Updated Invitation</html>",
			Text: "Updated Invitation",
		}, nil)

		ctx := context.Background()
		err := service.SendRegistrantUpdatedInvitation(ctx, updatedInvitation)

		// Should succeed without ICS attachment
		assert.NoError(t, err)

		// Verify email was sent
		messages := server.Messages()
		require.Len(t, messages, 1)
	})
}

func TestSMTPService_SendSummaryNotification(t *testing.T) {
	t.Run("successful summary notification", func(t *testing.T) {
		// Start mock SMTP server
		server := smtpmock.New(smtpmock.ConfigurationAttr{
			LogToStdout:       false,
			LogServerActivity: false,
		})
		require.NoError(t, server.Start())
		defer func() {
			require.NoError(t, server.Stop())
		}()

		mockTemplateManager := new(MockTemplateManager)

		service := &SMTPService{
			config: SMTPConfig{
				Host: "localhost",
				Port: server.PortNumber(),
				From: "test@example.com",
			},
			templateManager: mockTemplateManager,
		}

		notification := domain.EmailSummaryNotification{
			RecipientEmail:     "host@example.com",
			RecipientName:      "Host User",
			MeetingTitle:       "Team Sync",
			MeetingDate:        time.Now().Add(-1 * time.Hour),
			ProjectName:        "Test Project",
			ProjectLogo:        "https://example.com/logo.png",
			SummaryContent:     "Meeting summary content here",
			SummaryTitle:       "Team Sync Summary",
			MeetingDetailsLink: "https://example.com/meetings/123",
		}

		// Setup mock expectations
		mockTemplateManager.On("RenderSummaryNotification", mock.MatchedBy(func(notif domain.EmailSummaryNotification) bool {
			return notif.RecipientEmail == notification.RecipientEmail &&
				notif.MeetingTitle == notification.MeetingTitle &&
				notif.SummaryTitle == notification.SummaryTitle
		})).Return(&RenderedEmail{
			HTML: "<html><body>Summary Notification HTML</body></html>",
			Text: "Summary Notification Text",
		}, nil)

		ctx := context.Background()
		err := service.SendSummaryNotification(ctx, notification)

		assert.NoError(t, err)
		mockTemplateManager.AssertExpectations(t)

		// Verify email was sent
		messages := server.Messages()
		require.Len(t, messages, 1)
		message := messages[0]

		// Verify recipient and sender
		rcptto := message.RcpttoRequestResponse()
		require.NotEmpty(t, rcptto)
		assert.Contains(t, rcptto[0][0], "host@example.com")
		assert.Contains(t, message.MailfromRequest(), "test@example.com")

		// Verify message content
		msgData := message.MsgRequest()
		assert.Contains(t, msgData, "Team Sync")
		assert.Contains(t, msgData, "Meeting Summary Available")
	})

	t.Run("template rendering failure", func(t *testing.T) {
		// Start mock SMTP server
		server := smtpmock.New(smtpmock.ConfigurationAttr{
			LogToStdout:       false,
			LogServerActivity: false,
		})
		require.NoError(t, server.Start())
		defer func() {
			require.NoError(t, server.Stop())
		}()

		mockTemplateManager := new(MockTemplateManager)

		service := &SMTPService{
			config: SMTPConfig{
				Host: "localhost",
				Port: server.PortNumber(),
				From: "test@example.com",
			},
			templateManager: mockTemplateManager,
		}

		notification := domain.EmailSummaryNotification{
			RecipientEmail: "host@example.com",
			MeetingTitle:   "Test Meeting",
			ProjectName:    "Test Project",
		}

		// Setup mocks - template rendering fails
		mockTemplateManager.On("RenderSummaryNotification", mock.Anything).Return(nil, assert.AnError)

		ctx := context.Background()
		err := service.SendSummaryNotification(ctx, notification)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to render summary notification template")

		// Verify no email was sent
		assert.Empty(t, server.Messages())
	})

	t.Run("summary notification with minimal fields", func(t *testing.T) {
		// Start mock SMTP server
		server := smtpmock.New(smtpmock.ConfigurationAttr{
			LogToStdout:       false,
			LogServerActivity: false,
		})
		require.NoError(t, server.Start())
		defer func() {
			require.NoError(t, server.Stop())
		}()

		mockTemplateManager := new(MockTemplateManager)

		service := &SMTPService{
			config: SMTPConfig{
				Host: "localhost",
				Port: server.PortNumber(),
				From: "test@example.com",
			},
			templateManager: mockTemplateManager,
		}

		// Minimal notification with only required fields
		notification := domain.EmailSummaryNotification{
			RecipientEmail: "host@example.com",
			MeetingTitle:   "Minimal Meeting",
			MeetingDate:    time.Now(),
			ProjectName:    "Test Project",
		}

		// Setup mock expectations
		mockTemplateManager.On("RenderSummaryNotification", mock.Anything).Return(&RenderedEmail{
			HTML: "<html>Minimal Summary</html>",
			Text: "Minimal Summary",
		}, nil)

		ctx := context.Background()
		err := service.SendSummaryNotification(ctx, notification)

		assert.NoError(t, err)
		mockTemplateManager.AssertExpectations(t)

		// Verify email was sent
		messages := server.Messages()
		require.Len(t, messages, 1)
		assert.Contains(t, messages[0].MsgRequest(), "Minimal Meeting")
	})
}
