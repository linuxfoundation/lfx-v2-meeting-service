// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package email

import (
	"context"
	"encoding/base64"
	"fmt"
	"html/template"
	"log/slog"
	"strings"
	"time"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
)

// SMTPService implements the EmailService interface using SMTP
type SMTPService struct {
	config       SMTPConfig
	templates    Templates
	icsGenerator *ICSGenerator
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
		config:       config,
		icsGenerator: NewICSGenerator(),
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

	// Generate ICS file content
	icsContent, err := s.icsGenerator.GenerateMeetingInvitationICS(ICSMeetingInvitationParams{
		MeetingUID:     invitation.MeetingUID,
		MeetingTitle:   invitation.MeetingTitle,
		Description:    invitation.Description,
		StartTime:      invitation.StartTime,
		Duration:       invitation.Duration,
		Timezone:       invitation.Timezone,
		JoinLink:       invitation.JoinLink,
		MeetingID:      invitation.MeetingID,
		Passcode:       invitation.Passcode,
		RecipientEmail: invitation.RecipientEmail,
		Recurrence:     invitation.Recurrence,
	})
	if err != nil {
		slog.ErrorContext(ctx, "failed to generate ICS file", logging.ErrKey, err)
		// Continue without attachment if ICS generation fails
		icsContent = ""
	}

	// Create ICS attachment if generated successfully
	var attachment *domain.EmailAttachment
	if icsContent != "" {
		// Encode ICS content to base64
		encodedContent := base64.StdEncoding.EncodeToString([]byte(icsContent))
		attachment = &domain.EmailAttachment{
			Filename:    "meeting-invitation.ics",
			ContentType: "text/calendar",
			Content:     encodedContent,
		}
		// Store in invitation for template access
		invitation.ICSAttachment = attachment
	}

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

	// Build and send the email with attachment
	subject := fmt.Sprintf("Invitation: %s", invitation.MeetingTitle)
	message := buildEmailMessageWithAttachment(invitation.RecipientEmail, subject, htmlContent, textContent, attachment, s.config)
	err = sendEmailMessage(invitation.RecipientEmail, message, s.config)
	if err != nil {
		slog.ErrorContext(ctx, "failed to send invitation email", logging.ErrKey, err)
		return err
	}

	slog.InfoContext(ctx, "invitation email sent successfully")
	if attachment != nil {
		slog.InfoContext(ctx, "ICS attachment included in invitation")
	}
	return nil
}

// SendRegistrantCancellation sends a cancellation email to a meeting registrant
func (s *SMTPService) SendRegistrantCancellation(ctx context.Context, cancellation domain.EmailCancellation) error {
	ctx = logging.AppendCtx(ctx, slog.String("recipient_email", cancellation.RecipientEmail))
	ctx = logging.AppendCtx(ctx, slog.String("meeting_title", cancellation.MeetingTitle))

	// Generate ICS cancellation file if we have the necessary data
	var attachment *domain.EmailAttachment
	if cancellation.StartTime.After(time.Now()) {
		icsContent, err := s.icsGenerator.GenerateMeetingCancellationICS(ICSMeetingCancellationParams{
			MeetingUID:     cancellation.MeetingUID,
			MeetingTitle:   cancellation.MeetingTitle,
			StartTime:      cancellation.StartTime,
			Duration:       cancellation.Duration,
			Timezone:       cancellation.Timezone,
			RecipientEmail: cancellation.RecipientEmail,
			Recurrence:     cancellation.Recurrence,
		})
		if err != nil {
			slog.ErrorContext(ctx, "failed to generate ICS cancellation", logging.ErrKey, err)
			// Continue without ICS - don't fail the whole email
		} else {
			// Create attachment
			attachment = &domain.EmailAttachment{
				Filename:    fmt.Sprintf("%s-cancellation.ics", strings.ReplaceAll(cancellation.MeetingTitle, " ", "_")),
				ContentType: "text/calendar; charset=UTF-8; method=CANCEL",
				Content:     base64.StdEncoding.EncodeToString([]byte(icsContent)),
			}
			cancellation.ICSAttachment = attachment
		}
	}

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

	// Build and send the email with attachment
	subject := fmt.Sprintf("Meeting Cancellation: %s", cancellation.MeetingTitle)
	message := buildEmailMessageWithParams(EmailMessageParams{
		Recipient:   cancellation.RecipientEmail,
		Subject:     subject,
		HTMLContent: htmlContent,
		TextContent: textContent,
		Attachment:  attachment,
		Config:      s.config,
	})
	err = sendEmailMessage(cancellation.RecipientEmail, message, s.config)
	if err != nil {
		slog.ErrorContext(ctx, "failed to send cancellation email", logging.ErrKey, err)
		return err
	}

	slog.InfoContext(ctx, "cancellation email sent successfully")
	if attachment != nil {
		slog.InfoContext(ctx, "ICS cancellation attachment included")
	}
	return nil
}
