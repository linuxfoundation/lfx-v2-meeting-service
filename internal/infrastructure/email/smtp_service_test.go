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
	assert.NotNil(t, service.templates.Meeting.UpdatedInvitation.HTML)
	assert.NotNil(t, service.templates.Meeting.UpdatedInvitation.Text)
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

	updatedInvitation := domain.EmailUpdatedInvitation{
		RecipientEmail: "user@example.com",
		MeetingTitle:   "Test Meeting Updated",
		StartTime:      time.Now().Add(24 * time.Hour),
		Duration:       60,
		Timezone:       "UTC",
		Changes: map[string]any{
			"title":      "New Title",
			"start_time": "2024-01-15T14:30:00Z",
		},
	}

	t.Run("SendRegistrantInvitation", func(t *testing.T) {
		err := service.SendRegistrantInvitation(context.Background(), invitation)
		assert.NoError(t, err)
	})

	t.Run("SendRegistrantCancellation", func(t *testing.T) {
		err := service.SendRegistrantCancellation(context.Background(), cancellation)
		assert.NoError(t, err)
	})

	t.Run("SendRegistrantUpdatedInvitation", func(t *testing.T) {
		err := service.SendRegistrantUpdatedInvitation(context.Background(), updatedInvitation)
		assert.NoError(t, err)
	})
}
