// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package email

import (
	"context"
	"testing"
	"time"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/stretchr/testify/assert"
)

// TestNoOpService_ImplementsInterface verifies that NoOpService correctly implements
// the EmailService interface and that all methods execute without panicking.
func TestNoOpService_ImplementsInterface(t *testing.T) {
	// Compile-time check that NoOpService implements domain.EmailService
	var _ domain.EmailService = (*NoOpService)(nil)

	service := &NoOpService{}
	ctx := context.Background()

	// Runtime check that all methods execute without panicking and return nil
	assert.NotPanics(t, func() {
		err := service.SendRegistrantInvitation(ctx, domain.EmailInvitation{
			RecipientEmail: "test@example.com",
			MeetingTitle:   "Test Meeting",
			StartTime:      time.Now(),
		})
		assert.NoError(t, err)

		err = service.SendRegistrantCancellation(ctx, domain.EmailCancellation{
			RecipientEmail: "test@example.com",
			MeetingTitle:   "Test Meeting",
			StartTime:      time.Now(),
		})
		assert.NoError(t, err)

		err = service.SendOccurrenceCancellation(ctx, domain.EmailOccurrenceCancellation{
			RecipientEmail:      "test@example.com",
			MeetingTitle:        "Test Meeting",
			OccurrenceStartTime: time.Now(),
		})
		assert.NoError(t, err)

		err = service.SendRegistrantUpdatedInvitation(ctx, domain.EmailUpdatedInvitation{
			RecipientEmail: "test@example.com",
			MeetingTitle:   "Test Meeting",
			StartTime:      time.Now(),
		})
		assert.NoError(t, err)

		err = service.SendSummaryNotification(ctx, domain.EmailSummaryNotification{
			RecipientEmail: "test@example.com",
			MeetingTitle:   "Test Meeting",
			MeetingDate:    time.Now(),
		})
		assert.NoError(t, err)
	})
}
