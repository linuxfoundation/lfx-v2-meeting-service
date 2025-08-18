// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package email

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"html/template"
	"log/slog"
	"net/smtp"
	"strings"
	"time"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
)

//go:embed templates/*
var templateFS embed.FS

// SMTPConfig holds the SMTP server configuration
type SMTPConfig struct {
	Host     string
	Port     int
	From     string
	Username string // Optional for authenticated SMTP
	Password string // Optional for authenticated SMTP
}

// TemplateSet holds HTML and text versions of a template
type TemplateSet struct {
	HTML *template.Template
	Text *template.Template
}

// MeetingTemplates holds all meeting-related templates
type MeetingTemplates struct {
	Invitation   TemplateSet
	Cancellation TemplateSet
}

// Templates holds all template categories
type Templates struct {
	Meeting MeetingTemplates
}

// SMTPService implements the EmailService interface using SMTP
type SMTPService struct {
	config    SMTPConfig
	templates Templates
}

// NewSMTPService creates a new SMTP email service
func NewSMTPService(config SMTPConfig) (*SMTPService, error) {
	service := &SMTPService{
		config: config,
	}

	// Load meeting invitation templates
	invitationHTML, err := template.New("meeting_invitation.html").Funcs(template.FuncMap{
		"formatTime":     formatTime,
		"formatDuration": formatDuration,
	}).ParseFS(templateFS, "templates/meeting_invitation.html")
	if err != nil {
		return nil, fmt.Errorf("failed to parse meeting invitation HTML template: %w", err)
	}

	invitationText, err := template.New("meeting_invitation.txt").Funcs(template.FuncMap{
		"formatTime":     formatTime,
		"formatDuration": formatDuration,
	}).ParseFS(templateFS, "templates/meeting_invitation.txt")
	if err != nil {
		return nil, fmt.Errorf("failed to parse meeting invitation text template: %w", err)
	}

	// Load meeting cancellation templates
	cancellationHTML, err := template.New("meeting_invitation_cancellation.html").Funcs(template.FuncMap{
		"formatTime":     formatTime,
		"formatDuration": formatDuration,
	}).ParseFS(templateFS, "templates/meeting_invitation_cancellation.html")
	if err != nil {
		return nil, fmt.Errorf("failed to parse meeting invitation cancellation HTML template: %w", err)
	}

	cancellationText, err := template.New("meeting_invitation_cancellation.txt").Funcs(template.FuncMap{
		"formatTime":     formatTime,
		"formatDuration": formatDuration,
	}).ParseFS(templateFS, "templates/meeting_invitation_cancellation.txt")
	if err != nil {
		return nil, fmt.Errorf("failed to parse meeting invitation cancellation text template: %w", err)
	}

	service.templates = Templates{
		Meeting: MeetingTemplates{
			Invitation: TemplateSet{
				HTML: invitationHTML,
				Text: invitationText,
			},
			Cancellation: TemplateSet{
				HTML: cancellationHTML,
				Text: cancellationText,
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
	htmlContent, err := s.renderHTMLTemplate(invitation)
	if err != nil {
		slog.ErrorContext(ctx, "failed to render HTML template", logging.ErrKey, err)
		return fmt.Errorf("failed to render HTML template: %w", err)
	}

	textContent, err := s.renderTextTemplate(invitation)
	if err != nil {
		slog.ErrorContext(ctx, "failed to render text template", logging.ErrKey, err)
		return fmt.Errorf("failed to render text template: %w", err)
	}

	// Build the email message
	message := s.buildMessage(invitation, htmlContent, textContent)

	// Send the email
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)

	var auth smtp.Auth
	if s.config.Username != "" && s.config.Password != "" {
		auth = smtp.PlainAuth("", s.config.Username, s.config.Password, s.config.Host)
	}

	err = smtp.SendMail(addr, auth, s.config.From, []string{invitation.RecipientEmail}, []byte(message))
	if err != nil {
		slog.ErrorContext(ctx, "failed to send email", logging.ErrKey, err)
		return fmt.Errorf("failed to send email: %w", err)
	}

	slog.InfoContext(ctx, "invitation email sent successfully")
	return nil
}

// renderHTMLTemplate renders the HTML email template
func (s *SMTPService) renderHTMLTemplate(invitation domain.EmailInvitation) (string, error) {
	var buf bytes.Buffer
	err := s.templates.Meeting.Invitation.HTML.Execute(&buf, invitation)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

// renderTextTemplate renders the plain text email template
func (s *SMTPService) renderTextTemplate(invitation domain.EmailInvitation) (string, error) {
	var buf bytes.Buffer
	err := s.templates.Meeting.Invitation.Text.Execute(&buf, invitation)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

// buildMessage builds the complete email message with headers and multipart content
func (s *SMTPService) buildMessage(invitation domain.EmailInvitation, htmlContent, textContent string) string {
	boundary := "===============1234567890123456789=="

	var message strings.Builder

	// Email headers
	message.WriteString(fmt.Sprintf("From: %s\r\n", s.config.From))
	message.WriteString(fmt.Sprintf("To: %s\r\n", invitation.RecipientEmail))
	message.WriteString(fmt.Sprintf("Subject: Invitation: %s\r\n", invitation.MeetingTitle))
	message.WriteString("MIME-Version: 1.0\r\n")
	message.WriteString(fmt.Sprintf("Content-Type: multipart/alternative; boundary=\"%s\"\r\n", boundary))
	message.WriteString("\r\n")

	// Plain text part
	message.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	message.WriteString("Content-Type: text/plain; charset=\"UTF-8\"\r\n")
	message.WriteString("\r\n")
	message.WriteString(textContent)
	message.WriteString("\r\n")

	// HTML part
	message.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	message.WriteString("Content-Type: text/html; charset=\"UTF-8\"\r\n")
	message.WriteString("\r\n")
	message.WriteString(htmlContent)
	message.WriteString("\r\n")

	// End boundary
	message.WriteString(fmt.Sprintf("--%s--\r\n", boundary))

	return message.String()
}

// SendRegistrantCancellation sends a cancellation email to a meeting registrant
func (s *SMTPService) SendRegistrantCancellation(ctx context.Context, cancellation domain.EmailCancellation) error {
	ctx = logging.AppendCtx(ctx, slog.String("recipient_email", cancellation.RecipientEmail))
	ctx = logging.AppendCtx(ctx, slog.String("meeting_title", cancellation.MeetingTitle))

	// Generate email content from templates
	htmlContent, err := s.renderCancellationHTMLTemplate(cancellation)
	if err != nil {
		slog.ErrorContext(ctx, "failed to render cancellation HTML template", logging.ErrKey, err)
		return fmt.Errorf("failed to render cancellation HTML template: %w", err)
	}

	textContent, err := s.renderCancellationTextTemplate(cancellation)
	if err != nil {
		slog.ErrorContext(ctx, "failed to render cancellation text template", logging.ErrKey, err)
		return fmt.Errorf("failed to render cancellation text template: %w", err)
	}

	// Build the email message
	message := s.buildCancellationMessage(cancellation, htmlContent, textContent)

	// Send the email
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)

	var auth smtp.Auth
	if s.config.Username != "" && s.config.Password != "" {
		auth = smtp.PlainAuth("", s.config.Username, s.config.Password, s.config.Host)
	}

	err = smtp.SendMail(addr, auth, s.config.From, []string{cancellation.RecipientEmail}, []byte(message))
	if err != nil {
		slog.ErrorContext(ctx, "failed to send cancellation email", logging.ErrKey, err)
		return fmt.Errorf("failed to send cancellation email: %w", err)
	}

	slog.InfoContext(ctx, "cancellation email sent successfully")
	return nil
}

// renderCancellationHTMLTemplate renders the HTML cancellation email template
func (s *SMTPService) renderCancellationHTMLTemplate(cancellation domain.EmailCancellation) (string, error) {
	var buf bytes.Buffer
	err := s.templates.Meeting.Cancellation.HTML.Execute(&buf, cancellation)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

// renderCancellationTextTemplate renders the plain text cancellation email template
func (s *SMTPService) renderCancellationTextTemplate(cancellation domain.EmailCancellation) (string, error) {
	var buf bytes.Buffer
	err := s.templates.Meeting.Cancellation.Text.Execute(&buf, cancellation)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

// buildCancellationMessage builds the complete cancellation email message with headers and multipart content
func (s *SMTPService) buildCancellationMessage(cancellation domain.EmailCancellation, htmlContent, textContent string) string {
	boundary := "===============1234567890123456789=="

	var message strings.Builder

	// Email headers
	message.WriteString(fmt.Sprintf("From: %s\r\n", s.config.From))
	message.WriteString(fmt.Sprintf("To: %s\r\n", cancellation.RecipientEmail))
	message.WriteString(fmt.Sprintf("Subject: Meeting Cancellation: %s\r\n", cancellation.MeetingTitle))
	message.WriteString("MIME-Version: 1.0\r\n")
	message.WriteString(fmt.Sprintf("Content-Type: multipart/alternative; boundary=\"%s\"\r\n", boundary))
	message.WriteString("\r\n")

	// Plain text part
	message.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	message.WriteString("Content-Type: text/plain; charset=\"UTF-8\"\r\n")
	message.WriteString("\r\n")
	message.WriteString(textContent)
	message.WriteString("\r\n")

	// HTML part
	message.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	message.WriteString("Content-Type: text/html; charset=\"UTF-8\"\r\n")
	message.WriteString("\r\n")
	message.WriteString(htmlContent)
	message.WriteString("\r\n")

	// End boundary
	message.WriteString(fmt.Sprintf("--%s--\r\n", boundary))

	return message.String()
}

// formatTime formats a time for display in emails
func formatTime(t time.Time, timezone string) string {
	// Load the timezone
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		// Fall back to UTC if timezone is invalid
		loc = time.UTC
	}

	// Convert time to the specified timezone
	localTime := t.In(loc)

	// Format: Monday, January 2, 2006 at 3:04 PM MST
	return localTime.Format("Monday, January 2, 2006 at 3:04 PM MST")
}

// formatDuration formats duration in minutes to a human-readable string
func formatDuration(minutes int) string {
	if minutes < 60 {
		return fmt.Sprintf("%d minutes", minutes)
	}

	hours := minutes / 60
	remainingMinutes := minutes % 60

	if remainingMinutes == 0 {
		if hours == 1 {
			return "1 hour"
		}
		return fmt.Sprintf("%d hours", hours)
	}

	if hours == 1 {
		return fmt.Sprintf("1 hour %d minutes", remainingMinutes)
	}
	return fmt.Sprintf("%d hours %d minutes", hours, remainingMinutes)
}
