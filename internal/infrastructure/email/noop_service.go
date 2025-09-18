// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package email

import (
	"context"
	"log/slog"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/redaction"
)

// NoOpService is a no-operation email service that logs but doesn't send emails
type NoOpService struct{}

// NewNoOpService creates a new no-op email service
func NewNoOpService() *NoOpService {
	return &NoOpService{}
}

// SendRegistrantInvitation logs the invitation but doesn't send an email
func (s *NoOpService) SendRegistrantInvitation(ctx context.Context, invitation domain.EmailInvitation) error {
	ctx = logging.AppendCtx(ctx, slog.String("recipient_email", redaction.RedactEmail(invitation.RecipientEmail)))
	ctx = logging.AppendCtx(ctx, slog.String("meeting_title", invitation.MeetingTitle))

	slog.DebugContext(ctx, "email service disabled, skipping invitation email")
	return nil
}

// SendRegistrantCancellation logs the cancellation but doesn't send an email
func (s *NoOpService) SendRegistrantCancellation(ctx context.Context, cancellation domain.EmailCancellation) error {
	ctx = logging.AppendCtx(ctx, slog.String("recipient_email", redaction.RedactEmail(cancellation.RecipientEmail)))
	ctx = logging.AppendCtx(ctx, slog.String("meeting_title", cancellation.MeetingTitle))

	slog.DebugContext(ctx, "email service disabled, skipping cancellation email")
	return nil
}

// SendRegistrantUpdatedInvitation logs the update but doesn't send an email
func (s *NoOpService) SendRegistrantUpdatedInvitation(ctx context.Context, updatedInvitation domain.EmailUpdatedInvitation) error {
	ctx = logging.AppendCtx(ctx, slog.String("recipient_email", redaction.RedactEmail(updatedInvitation.RecipientEmail)))
	ctx = logging.AppendCtx(ctx, slog.String("meeting_title", updatedInvitation.MeetingTitle))

	slog.DebugContext(ctx, "email service disabled, skipping update notification email")
	return nil
}

// SendSummaryNotification logs the summary notification but doesn't send an email
func (s *NoOpService) SendSummaryNotification(ctx context.Context, notification domain.EmailSummaryNotification) error {
	ctx = logging.AppendCtx(ctx, slog.String("recipient_email", redaction.RedactEmail(notification.RecipientEmail)))
	ctx = logging.AppendCtx(ctx, slog.String("meeting_title", notification.MeetingTitle))

	slog.DebugContext(ctx, "email service disabled, skipping summary notification email")
	return nil
}
