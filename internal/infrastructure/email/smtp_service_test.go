// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package email

import (
	"context"
	"testing"
	"time"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/stretchr/testify/assert"
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
	assert.NotNil(t, service.templates.Meeting.Invitation.HTML)
	assert.NotNil(t, service.templates.Meeting.Invitation.Text)
	assert.NotNil(t, service.templates.Meeting.Cancellation.HTML)
	assert.NotNil(t, service.templates.Meeting.Cancellation.Text)
}

func TestSMTPService_RenderTemplates(t *testing.T) {
	config := SMTPConfig{
		Host: "localhost",
		Port: 1025,
		From: "test@example.com",
	}

	service, err := NewSMTPService(config)
	require.NoError(t, err)

	invitation := domain.EmailInvitation{
		RecipientEmail: "user@example.com",
		RecipientName:  "John Doe",
		MeetingTitle:   "Test Meeting",
		StartTime:      time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC),
		Duration:       60,
		Timezone:       "UTC",
		Description:    "This is a test meeting",
		JoinLink:       "https://zoom.us/j/123456789",
		ProjectName:    "Test Project",
	}

	t.Run("HTML template rendering", func(t *testing.T) {
		htmlContent, err := service.renderHTMLTemplate(invitation)
		require.NoError(t, err)
		assert.Contains(t, htmlContent, "Test Meeting")
		assert.Contains(t, htmlContent, "John Doe")
		assert.Contains(t, htmlContent, "https://zoom.us/j/123456789")
		assert.Contains(t, htmlContent, "This is a test meeting")
		assert.Contains(t, htmlContent, "Test Project")
	})

	t.Run("Text template rendering", func(t *testing.T) {
		textContent, err := service.renderTextTemplate(invitation)
		require.NoError(t, err)
		assert.Contains(t, textContent, "Test Meeting")
		assert.Contains(t, textContent, "John Doe")
		assert.Contains(t, textContent, "https://zoom.us/j/123456789")
		assert.Contains(t, textContent, "This is a test meeting")
		assert.Contains(t, textContent, "Test Project")
	})
}

func TestSMTPService_BuildMessage(t *testing.T) {
	config := SMTPConfig{
		Host: "localhost",
		Port: 1025,
		From: "noreply@example.com",
	}

	service, err := NewSMTPService(config)
	require.NoError(t, err)

	invitation := domain.EmailInvitation{
		RecipientEmail: "user@example.com",
		RecipientName:  "John Doe",
		MeetingTitle:   "Test Meeting",
		StartTime:      time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC),
		Duration:       60,
		Timezone:       "UTC",
		Description:    "This is a test meeting",
		JoinLink:       "https://zoom.us/j/123456789",
	}

	htmlContent := "<h1>Test HTML</h1>"
	textContent := "Test Text"

	message := service.buildMessage(invitation, htmlContent, textContent)

	assert.Contains(t, message, "From: noreply@example.com")
	assert.Contains(t, message, "To: user@example.com")
	assert.Contains(t, message, "Subject: Invitation: Test Meeting")
	assert.Contains(t, message, "MIME-Version: 1.0")
	assert.Contains(t, message, "Content-Type: multipart/alternative")
	assert.Contains(t, message, "Content-Type: text/plain")
	assert.Contains(t, message, "Content-Type: text/html")
	assert.Contains(t, message, htmlContent)
	assert.Contains(t, message, textContent)
}

func TestFormatTime(t *testing.T) {
	testTime := time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC)

	tests := []struct {
		name     string
		timezone string
		expected string
	}{
		{
			name:     "UTC timezone",
			timezone: "UTC",
			expected: "Monday, January 15, 2024 at 2:30 PM UTC",
		},
		{
			name:     "EST timezone",
			timezone: "America/New_York",
			expected: "Monday, January 15, 2024 at 9:30 AM EST",
		},
		{
			name:     "Invalid timezone falls back to UTC",
			timezone: "Invalid/Timezone",
			expected: "Monday, January 15, 2024 at 2:30 PM UTC",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatTime(testTime, tt.timezone)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		minutes  int
		expected string
	}{
		{
			name:     "30 minutes",
			minutes:  30,
			expected: "30 minutes",
		},
		{
			name:     "1 hour exactly",
			minutes:  60,
			expected: "1 hour",
		},
		{
			name:     "2 hours exactly",
			minutes:  120,
			expected: "2 hours",
		},
		{
			name:     "1 hour 30 minutes",
			minutes:  90,
			expected: "1 hour 30 minutes",
		},
		{
			name:     "2 hours 45 minutes",
			minutes:  165,
			expected: "2 hours 45 minutes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatDuration(tt.minutes)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSMTPService_RenderCancellationTemplates(t *testing.T) {
	config := SMTPConfig{
		Host: "localhost",
		Port: 1025,
		From: "test@example.com",
	}

	service, err := NewSMTPService(config)
	require.NoError(t, err)

	cancellation := domain.EmailCancellation{
		RecipientEmail: "user@example.com",
		RecipientName:  "John Doe",
		MeetingTitle:   "Test Meeting Cancelled",
		StartTime:      time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC),
		Duration:       60,
		Timezone:       "UTC",
		Description:    "This meeting has been cancelled",
		ProjectName:    "Test Project",
		Reason:         "Meeting no longer needed",
	}

	t.Run("HTML cancellation template rendering", func(t *testing.T) {
		htmlContent, err := service.renderCancellationHTMLTemplate(cancellation)
		require.NoError(t, err)
		assert.Contains(t, htmlContent, "Test Meeting Cancelled")
		assert.Contains(t, htmlContent, "John Doe")
		assert.Contains(t, htmlContent, "Meeting Cancellation Notice")
		assert.Contains(t, htmlContent, "This meeting has been cancelled")
		assert.Contains(t, htmlContent, "Test Project")
		assert.Contains(t, htmlContent, "Meeting no longer needed")
		assert.Contains(t, htmlContent, "cancelled")
	})

	t.Run("Text cancellation template rendering", func(t *testing.T) {
		textContent, err := service.renderCancellationTextTemplate(cancellation)
		require.NoError(t, err)
		assert.Contains(t, textContent, "Test Meeting Cancelled")
		assert.Contains(t, textContent, "John Doe")
		assert.Contains(t, textContent, "Meeting Cancellation Notice")
		assert.Contains(t, textContent, "This meeting has been cancelled")
		assert.Contains(t, textContent, "Test Project")
		assert.Contains(t, textContent, "Meeting no longer needed")
		assert.Contains(t, textContent, "CANCELLED")
	})
}

func TestSMTPService_BuildCancellationMessage(t *testing.T) {
	config := SMTPConfig{
		Host: "localhost",
		Port: 1025,
		From: "noreply@example.com",
	}

	service, err := NewSMTPService(config)
	require.NoError(t, err)

	cancellation := domain.EmailCancellation{
		RecipientEmail: "user@example.com",
		RecipientName:  "John Doe",
		MeetingTitle:   "Test Meeting Cancelled",
		StartTime:      time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC),
		Duration:       60,
		Timezone:       "UTC",
		Description:    "This meeting has been cancelled",
		Reason:         "Meeting no longer needed",
	}

	htmlContent := "<h1>Test Cancellation HTML</h1>"
	textContent := "Test Cancellation Text"

	message := service.buildCancellationMessage(cancellation, htmlContent, textContent)

	assert.Contains(t, message, "From: noreply@example.com")
	assert.Contains(t, message, "To: user@example.com")
	assert.Contains(t, message, "Subject: Meeting Cancellation: Test Meeting Cancelled")
	assert.Contains(t, message, "MIME-Version: 1.0")
	assert.Contains(t, message, "Content-Type: multipart/alternative")
	assert.Contains(t, message, "Content-Type: text/plain")
	assert.Contains(t, message, "Content-Type: text/html")
	assert.Contains(t, message, htmlContent)
	assert.Contains(t, message, textContent)
}

func TestNoOpService(t *testing.T) {
	service := NewNoOpService()
	assert.NotNil(t, service)

	invitation := domain.EmailInvitation{
		RecipientEmail: "user@example.com",
		MeetingTitle:   "Test Meeting",
	}

	cancellation := domain.EmailCancellation{
		RecipientEmail: "user@example.com",
		MeetingTitle:   "Test Meeting Cancelled",
	}

	t.Run("SendRegistrantInvitation", func(t *testing.T) {
		err := service.SendRegistrantInvitation(context.Background(), invitation)
		assert.NoError(t, err)
	})

	t.Run("SendRegistrantCancellation", func(t *testing.T) {
		err := service.SendRegistrantCancellation(context.Background(), cancellation)
		assert.NoError(t, err)
	})
}
