// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package email

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildEmailMessage(t *testing.T) {
	config := SMTPConfig{
		Host: "localhost",
		Port: 1025,
		From: "noreply@example.com",
	}

	tests := []struct {
		name        string
		recipient   string
		subject     string
		htmlContent string
		textContent string
	}{
		{
			name:        "invitation email",
			recipient:   "user@example.com",
			subject:     "Invitation: Test Meeting",
			htmlContent: "<h1>Test HTML</h1>",
			textContent: "Test Text",
		},
		{
			name:        "cancellation email",
			recipient:   "user@example.com",
			subject:     "Meeting Cancellation: Test Meeting Cancelled",
			htmlContent: "<h1>Test Cancellation HTML</h1>",
			textContent: "Test Cancellation Text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			message := buildEmailMessage(tt.recipient, tt.subject, tt.htmlContent, tt.textContent, config)

			assert.Contains(t, message, "From: LFX One Meetings <noreply@example.com>")
			assert.Contains(t, message, fmt.Sprintf("To: %s", tt.recipient))
			assert.Contains(t, message, fmt.Sprintf("Subject: %s", tt.subject))
			assert.Contains(t, message, "MIME-Version: 1.0")
			assert.Contains(t, message, "Content-Type: multipart/alternative")
			assert.Contains(t, message, "Content-Type: text/plain")
			assert.Contains(t, message, "Content-Type: text/html")
			assert.Contains(t, message, tt.htmlContent)
			assert.Contains(t, message, tt.textContent)
		})
	}
}

func TestSendEmailMessage(t *testing.T) {
	t.Run("connection error", func(t *testing.T) {
		config := SMTPConfig{
			Host: "nonexistent.host",
			Port: 9999,
			From: "noreply@example.com",
		}

		err := sendEmailMessage("user@example.com", "Test message", config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to send email")
	})

	t.Run("with authentication configuration", func(t *testing.T) {
		// Test that the function handles authentication parameters without panicking
		config := SMTPConfig{
			Host:     "nonexistent.host",
			Port:     9999,
			From:     "noreply@example.com",
			Username: "testuser",
			Password: "testpass",
		}

		err := sendEmailMessage("user@example.com", "Test message", config)
		assert.Error(t, err) // Expected error due to nonexistent host
		assert.Contains(t, err.Error(), "failed to send email")
	})

	t.Run("without authentication configuration", func(t *testing.T) {
		// Test that the function handles no auth without panicking
		config := SMTPConfig{
			Host: "nonexistent.host",
			Port: 9999,
			From: "noreply@example.com",
		}

		err := sendEmailMessage("user@example.com", "Test message", config)
		assert.Error(t, err) // Expected error due to nonexistent host
		assert.Contains(t, err.Error(), "failed to send email")
	})
}
