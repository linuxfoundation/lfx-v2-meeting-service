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

func TestNewNoOpService(t *testing.T) {
	service := NewNoOpService()
	require.NotNil(t, service)
}

func TestNoOpService_SendRegistrantInvitation(t *testing.T) {
	service := NewNoOpService()
	ctx := context.Background()

	invitation := domain.EmailInvitation{
		RecipientEmail: "user@example.com",
		MeetingTitle:   "Test Meeting",
		MeetingUID:     "meeting-123",
		Description:    "Test Description",
		StartTime:      time.Now().Add(24 * time.Hour),
		Duration:       60,
		Timezone:       "UTC",
		JoinLink:       "https://example.com/join/123",
		ProjectName:    "Test Project",
	}

	err := service.SendRegistrantInvitation(ctx, invitation)
	assert.NoError(t, err, "NoOpService should not return error")
}

func TestNoOpService_SendRegistrantCancellation(t *testing.T) {
	service := NewNoOpService()
	ctx := context.Background()

	cancellation := domain.EmailCancellation{
		RecipientEmail: "user@example.com",
		MeetingTitle:   "Test Meeting Cancelled",
		MeetingUID:     "meeting-456",
		StartTime:      time.Now().Add(24 * time.Hour),
		Duration:       60,
		Timezone:       "UTC",
		ProjectName:    "Test Project",
	}

	err := service.SendRegistrantCancellation(ctx, cancellation)
	assert.NoError(t, err, "NoOpService should not return error")
}

func TestNoOpService_SendOccurrenceCancellation(t *testing.T) {
	service := NewNoOpService()
	ctx := context.Background()

	cancellation := domain.EmailOccurrenceCancellation{
		RecipientEmail:      "user@example.com",
		MeetingTitle:        "Test Meeting",
		MeetingUID:          "meeting-789",
		OccurrenceID:        "occurrence-123",
		OccurrenceStartTime: time.Now().Add(48 * time.Hour),
		Duration:            60,
		Timezone:            "America/New_York",
		ProjectName:         "Test Project",
	}

	err := service.SendOccurrenceCancellation(ctx, cancellation)
	assert.NoError(t, err, "NoOpService should not return error")
}

func TestNoOpService_SendRegistrantUpdatedInvitation(t *testing.T) {
	service := NewNoOpService()
	ctx := context.Background()

	updatedInvitation := domain.EmailUpdatedInvitation{
		RecipientEmail: "user@example.com",
		MeetingTitle:   "Test Meeting Updated",
		MeetingUID:     "meeting-abc",
		StartTime:      time.Now().Add(24 * time.Hour),
		Duration:       60,
		Timezone:       "UTC",
		Changes: map[string]any{
			"title":      "New Title",
			"start_time": "2024-01-15T14:30:00Z",
		},
		ProjectName: "Test Project",
	}

	err := service.SendRegistrantUpdatedInvitation(ctx, updatedInvitation)
	assert.NoError(t, err, "NoOpService should not return error")
}

func TestNoOpService_SendSummaryNotification(t *testing.T) {
	service := NewNoOpService()
	ctx := context.Background()

	notification := domain.EmailSummaryNotification{
		RecipientEmail:     "host@example.com",
		RecipientName:      "Host User",
		MeetingTitle:       "Test Meeting",
		MeetingDate:        time.Now().Add(-1 * time.Hour),
		ProjectName:        "Test Project",
		ProjectLogo:        "https://example.com/logo.png",
		SummaryContent:     "This is a test summary",
		SummaryTitle:       "Test Summary",
		MeetingDetailsLink: "https://example.com/meetings/456",
	}

	err := service.SendSummaryNotification(ctx, notification)
	assert.NoError(t, err, "NoOpService should not return error")
}

func TestNoOpService_AllMethodsWithEmptyFields(t *testing.T) {
	service := NewNoOpService()
	ctx := context.Background()

	t.Run("SendRegistrantInvitation with minimal data", func(t *testing.T) {
		invitation := domain.EmailInvitation{
			RecipientEmail: "user@example.com",
			MeetingTitle:   "Meeting",
		}
		err := service.SendRegistrantInvitation(ctx, invitation)
		assert.NoError(t, err)
	})

	t.Run("SendRegistrantCancellation with minimal data", func(t *testing.T) {
		cancellation := domain.EmailCancellation{
			RecipientEmail: "user@example.com",
			MeetingTitle:   "Meeting",
		}
		err := service.SendRegistrantCancellation(ctx, cancellation)
		assert.NoError(t, err)
	})

	t.Run("SendOccurrenceCancellation with minimal data", func(t *testing.T) {
		cancellation := domain.EmailOccurrenceCancellation{
			RecipientEmail: "user@example.com",
			MeetingTitle:   "Meeting",
			OccurrenceID:   "occurrence-1",
		}
		err := service.SendOccurrenceCancellation(ctx, cancellation)
		assert.NoError(t, err)
	})

	t.Run("SendRegistrantUpdatedInvitation with minimal data", func(t *testing.T) {
		updatedInvitation := domain.EmailUpdatedInvitation{
			RecipientEmail: "user@example.com",
			MeetingTitle:   "Meeting",
		}
		err := service.SendRegistrantUpdatedInvitation(ctx, updatedInvitation)
		assert.NoError(t, err)
	})

	t.Run("SendSummaryNotification with minimal data", func(t *testing.T) {
		notification := domain.EmailSummaryNotification{
			RecipientEmail: "host@example.com",
			MeetingTitle:   "Meeting",
		}
		err := service.SendSummaryNotification(ctx, notification)
		assert.NoError(t, err)
	})
}

func TestNoOpService_WithCancelledContext(t *testing.T) {
	service := NewNoOpService()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	t.Run("SendRegistrantInvitation with cancelled context", func(t *testing.T) {
		invitation := domain.EmailInvitation{
			RecipientEmail: "user@example.com",
			MeetingTitle:   "Meeting",
		}
		err := service.SendRegistrantInvitation(ctx, invitation)
		assert.NoError(t, err, "NoOpService ignores context cancellation")
	})

	t.Run("SendRegistrantCancellation with cancelled context", func(t *testing.T) {
		cancellation := domain.EmailCancellation{
			RecipientEmail: "user@example.com",
			MeetingTitle:   "Meeting",
		}
		err := service.SendRegistrantCancellation(ctx, cancellation)
		assert.NoError(t, err, "NoOpService ignores context cancellation")
	})

	t.Run("SendOccurrenceCancellation with cancelled context", func(t *testing.T) {
		cancellation := domain.EmailOccurrenceCancellation{
			RecipientEmail: "user@example.com",
			MeetingTitle:   "Meeting",
			OccurrenceID:   "occurrence-1",
		}
		err := service.SendOccurrenceCancellation(ctx, cancellation)
		assert.NoError(t, err, "NoOpService ignores context cancellation")
	})

	t.Run("SendRegistrantUpdatedInvitation with cancelled context", func(t *testing.T) {
		updatedInvitation := domain.EmailUpdatedInvitation{
			RecipientEmail: "user@example.com",
			MeetingTitle:   "Meeting",
		}
		err := service.SendRegistrantUpdatedInvitation(ctx, updatedInvitation)
		assert.NoError(t, err, "NoOpService ignores context cancellation")
	})

	t.Run("SendSummaryNotification with cancelled context", func(t *testing.T) {
		notification := domain.EmailSummaryNotification{
			RecipientEmail: "host@example.com",
			MeetingTitle:   "Meeting",
		}
		err := service.SendSummaryNotification(ctx, notification)
		assert.NoError(t, err, "NoOpService ignores context cancellation")
	})
}
