// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package email

import (
	"context"
	"log/slog"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
)

// NoOpService is a no-operation email service that logs but doesn't send emails
type NoOpService struct{}

// NewNoOpService creates a new no-op email service
func NewNoOpService() *NoOpService {
	return &NoOpService{}
}

// SendRegistrantInvitation logs the invitation but doesn't send an email
func (s *NoOpService) SendRegistrantInvitation(ctx context.Context, invitation domain.EmailInvitation) error {
	ctx = logging.AppendCtx(ctx, slog.String("recipient_email", invitation.RecipientEmail))
	ctx = logging.AppendCtx(ctx, slog.String("meeting_title", invitation.MeetingTitle))

	slog.DebugContext(ctx, "email service disabled, skipping invitation email")
	return nil
}

// SendRegistrantCancellation logs the cancellation but doesn't send an email
func (s *NoOpService) SendRegistrantCancellation(ctx context.Context, cancellation domain.EmailCancellation) error {
	ctx = logging.AppendCtx(ctx, slog.String("recipient_email", cancellation.RecipientEmail))
	ctx = logging.AppendCtx(ctx, slog.String("meeting_title", cancellation.MeetingTitle))

	slog.DebugContext(ctx, "email service disabled, skipping cancellation email")
	return nil
}

// SendRegistrantUpdatedInvitation logs the update but doesn't send an email
func (s *NoOpService) SendRegistrantUpdatedInvitation(ctx context.Context, updatedInvitation domain.EmailUpdatedInvitation) error {
	ctx = logging.AppendCtx(ctx, slog.String("recipient_email", updatedInvitation.RecipientEmail))
	ctx = logging.AppendCtx(ctx, slog.String("meeting_title", updatedInvitation.MeetingTitle))

	slog.DebugContext(ctx, "email service disabled, skipping update notification email")
	return nil
}
