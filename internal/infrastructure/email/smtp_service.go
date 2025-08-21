// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package email

import (
	"context"
	"fmt"
	"html/template"
	"log/slog"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
)

// SMTPService implements the EmailService interface using SMTP
type SMTPService struct {
	config    SMTPConfig
	templates Templates
}

// SMTPConfig holds the SMTP server configuration
type SMTPConfig struct {
	Host     string
	Port     int
	From     string
	Username string // Optional for authenticated SMTP
	Password string // Optional for authenticated SMTP
}

// NewSMTPService creates a new SMTP email service
func NewSMTPService(config SMTPConfig) (*SMTPService, error) {
	service := &SMTPService{
		config: config,
	}

	// Define all templates to load
	templateConfigs := map[string]templateConfig{
		"invitationHTML":   {"meeting_invitation.html", "templates/meeting_invitation.html"},
		"invitationText":   {"meeting_invitation.txt", "templates/meeting_invitation.txt"},
		"cancellationHTML": {"meeting_invitation_cancellation.html", "templates/meeting_invitation_cancellation.html"},
		"cancellationText": {"meeting_invitation_cancellation.txt", "templates/meeting_invitation_cancellation.txt"},
	}

	// Load all templates
	loadedTemplates := make(map[string]*template.Template)
	for key, cfg := range templateConfigs {
		tmpl, err := loadTemplate(cfg)
		if err != nil {
			return nil, err
		}
		loadedTemplates[key] = tmpl
	}

	// Organize templates into the service structure
	service.templates = Templates{
		Meeting: MeetingTemplates{
			Invitation: TemplateSet{
				HTML: loadedTemplates["invitationHTML"],
				Text: loadedTemplates["invitationText"],
			},
			Cancellation: TemplateSet{
				HTML: loadedTemplates["cancellationHTML"],
				Text: loadedTemplates["cancellationText"],
			},
		},
	}

	return service, nil
}

// SendRegistrantInvitation sends an invitation email to a meeting registrant
func (s *SMTPService) SendRegistrantInvitation(ctx context.Context, invitation domain.EmailInvitation) error {
	ctx = logging.AppendCtx(ctx, slog.String("recipient_email", invitation.RecipientEmail))
	ctx = logging.AppendCtx(ctx, slog.String("meeting_title", invitation.MeetingTitle))

	// Generate email content from templates
	htmlContent, err := renderTemplate(s.templates.Meeting.Invitation.HTML, invitation)
	if err != nil {
		slog.ErrorContext(ctx, "failed to render HTML template", logging.ErrKey, err)
		return fmt.Errorf("failed to render HTML template: %w", err)
	}

	textContent, err := renderTemplate(s.templates.Meeting.Invitation.Text, invitation)
	if err != nil {
		slog.ErrorContext(ctx, "failed to render text template", logging.ErrKey, err)
		return fmt.Errorf("failed to render text template: %w", err)
	}

	// Build and send the email
	subject := fmt.Sprintf("Invitation: %s", invitation.MeetingTitle)
	message := buildEmailMessage(invitation.RecipientEmail, subject, htmlContent, textContent, s.config)
	err = sendEmailMessage(invitation.RecipientEmail, message, s.config)
	if err != nil {
		slog.ErrorContext(ctx, "failed to send invitation email", logging.ErrKey, err)
		return err
	}

	slog.InfoContext(ctx, "invitation email sent successfully")
	return nil
}

// SendRegistrantCancellation sends a cancellation email to a meeting registrant
func (s *SMTPService) SendRegistrantCancellation(ctx context.Context, cancellation domain.EmailCancellation) error {
	ctx = logging.AppendCtx(ctx, slog.String("recipient_email", cancellation.RecipientEmail))
	ctx = logging.AppendCtx(ctx, slog.String("meeting_title", cancellation.MeetingTitle))

	// Generate email content from templates
	htmlContent, err := renderTemplate(s.templates.Meeting.Cancellation.HTML, cancellation)
	if err != nil {
		slog.ErrorContext(ctx, "failed to render cancellation HTML template", logging.ErrKey, err)
		return fmt.Errorf("failed to render cancellation HTML template: %w", err)
	}

	textContent, err := renderTemplate(s.templates.Meeting.Cancellation.Text, cancellation)
	if err != nil {
		slog.ErrorContext(ctx, "failed to render cancellation text template", logging.ErrKey, err)
		return fmt.Errorf("failed to render cancellation text template: %w", err)
	}

	// Build and send the email
	subject := fmt.Sprintf("Meeting Cancellation: %s", cancellation.MeetingTitle)
	message := buildEmailMessage(cancellation.RecipientEmail, subject, htmlContent, textContent, s.config)
	err = sendEmailMessage(cancellation.RecipientEmail, message, s.config)
	if err != nil {
		slog.ErrorContext(ctx, "failed to send cancellation email", logging.ErrKey, err)
		return err
	}

	slog.InfoContext(ctx, "cancellation email sent successfully")
	return nil
}
