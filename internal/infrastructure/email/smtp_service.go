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
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/redaction"
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
		"invitationHTML":             {"meeting_invitation.html", "templates/meeting_invitation.html"},
		"invitationText":             {"meeting_invitation.txt", "templates/meeting_invitation.txt"},
		"cancellationHTML":           {"meeting_invitation_cancellation.html", "templates/meeting_invitation_cancellation.html"},
		"cancellationText":           {"meeting_invitation_cancellation.txt", "templates/meeting_invitation_cancellation.txt"},
		"occurrenceCancellationHTML": {"meeting_occurrence_cancellation.html", "templates/meeting_occurrence_cancellation.html"},
		"occurrenceCancellationText": {"meeting_occurrence_cancellation.txt", "templates/meeting_occurrence_cancellation.txt"},
		"updatedInvitationHTML":      {"meeting_updated_invitation.html", "templates/meeting_updated_invitation.html"},
		"updatedInvitationText":      {"meeting_updated_invitation.txt", "templates/meeting_updated_invitation.txt"},
		"summaryNotificationHTML":    {"meeting_summary_notification.html", "templates/meeting_summary_notification.html"},
		"summaryNotificationText":    {"meeting_summary_notification.txt", "templates/meeting_summary_notification.txt"},
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
			OccurrenceCancellation: TemplateSet{
				HTML: loadedTemplates["occurrenceCancellationHTML"],
				Text: loadedTemplates["occurrenceCancellationText"],
			},
			UpdatedInvitation: TemplateSet{
				HTML: loadedTemplates["updatedInvitationHTML"],
				Text: loadedTemplates["updatedInvitationText"],
			},
			SummaryNotification: TemplateSet{
				HTML: loadedTemplates["summaryNotificationHTML"],
				Text: loadedTemplates["summaryNotificationText"],
			},
		},
	}

	return service, nil
}

// SendRegistrantInvitation sends an invitation email to a meeting registrant
func (s *SMTPService) SendRegistrantInvitation(ctx context.Context, invitation domain.EmailInvitation) error {
	ctx = logging.AppendCtx(ctx, slog.String("recipient_email", redaction.RedactEmail(invitation.RecipientEmail)))
	ctx = logging.AppendCtx(ctx, slog.String("meeting_title", invitation.MeetingTitle))

	// Generate ICS file content
	icsContent, err := s.icsGenerator.GenerateMeetingInvitationICS(ICSMeetingInvitationParams{
		MeetingUID:               invitation.MeetingUID,
		MeetingTitle:             invitation.MeetingTitle,
		Description:              invitation.Description,
		StartTime:                invitation.StartTime,
		Duration:                 invitation.Duration,
		Timezone:                 invitation.Timezone,
		JoinLink:                 invitation.JoinLink,
		MeetingID:                invitation.MeetingID,
		Passcode:                 invitation.Passcode,
		RecipientEmail:           invitation.RecipientEmail,
		ProjectName:              invitation.ProjectName,
		Recurrence:               invitation.Recurrence,
		Sequence:                 invitation.IcsSequence,
		CancelledOccurrenceTimes: invitation.CancelledOccurrenceTimes,
		Attachments:              invitation.Attachments,
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
			ContentType: "text/calendar; charset=UTF-8; method=REQUEST",
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

	// Build and send the email with attachments (ICS + file attachments)
	subject := fmt.Sprintf("Invitation: %s", invitation.MeetingTitle)
	metadata := &EmailMetadata{
		ProjectName: invitation.ProjectName,
	}
	message := buildEmailMessageWithParams(EmailMessageParams{
		Recipient:   invitation.RecipientEmail,
		Subject:     subject,
		HTMLContent: htmlContent,
		TextContent: textContent,
		Attachment:  attachment,                 // ICS calendar attachment
		Attachments: invitation.FileAttachments, // Meeting file attachments
		Config:      s.config,
		Metadata:    metadata,
	})
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
	ctx = logging.AppendCtx(ctx, slog.String("recipient_email", redaction.RedactEmail(cancellation.RecipientEmail)))
	ctx = logging.AppendCtx(ctx, slog.String("meeting_title", cancellation.MeetingTitle))

	// Generate ICS cancellation file if the meeting series is not completed yet.
	// Otherwise if it is completed, then we don't need to remove the series from the user's calendar.
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
			Sequence:       cancellation.IcsSequence,
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
		Metadata: &EmailMetadata{
			ProjectName: cancellation.ProjectName,
		},
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

// SendOccurrenceCancellation sends an occurrence cancellation email to a meeting registrant
func (s *SMTPService) SendOccurrenceCancellation(ctx context.Context, cancellation domain.EmailOccurrenceCancellation) error {
	ctx = logging.AppendCtx(ctx, slog.String("recipient_email", redaction.RedactEmail(cancellation.RecipientEmail)))
	ctx = logging.AppendCtx(ctx, slog.String("meeting_title", cancellation.MeetingTitle))
	ctx = logging.AppendCtx(ctx, slog.String("occurrence_id", cancellation.OccurrenceID))

	// Generate ICS cancellation file for the specific occurrence if it's in the future
	var attachment *domain.EmailAttachment
	if cancellation.OccurrenceStartTime.After(time.Now()) {
		icsContent, err := s.icsGenerator.GenerateOccurrenceCancellationICS(ICSOccurrenceCancellationParams{
			MeetingUID:          cancellation.MeetingUID,
			MeetingTitle:        cancellation.MeetingTitle,
			OccurrenceStartTime: cancellation.OccurrenceStartTime,
			Duration:            cancellation.Duration,
			Timezone:            cancellation.Timezone,
			RecipientEmail:      cancellation.RecipientEmail,
			Sequence:            cancellation.IcsSequence,
		})
		if err != nil {
			slog.ErrorContext(ctx, "failed to generate ICS cancellation for occurrence", logging.ErrKey, err)
			// Continue without ICS - don't fail the whole email
		} else {
			// Create attachment
			attachment = &domain.EmailAttachment{
				Filename:    fmt.Sprintf("%s-occurrence-cancellation.ics", strings.ReplaceAll(cancellation.MeetingTitle, " ", "_")),
				ContentType: "text/calendar; charset=UTF-8; method=CANCEL",
				Content:     base64.StdEncoding.EncodeToString([]byte(icsContent)),
			}
			cancellation.ICSAttachment = attachment
		}
	}

	// Generate email content from templates
	htmlContent, err := renderTemplate(s.templates.Meeting.OccurrenceCancellation.HTML, cancellation)
	if err != nil {
		slog.ErrorContext(ctx, "failed to render occurrence cancellation HTML template", logging.ErrKey, err)
		return fmt.Errorf("failed to render occurrence cancellation HTML template: %w", err)
	}

	textContent, err := renderTemplate(s.templates.Meeting.OccurrenceCancellation.Text, cancellation)
	if err != nil {
		slog.ErrorContext(ctx, "failed to render occurrence cancellation text template", logging.ErrKey, err)
		return fmt.Errorf("failed to render occurrence cancellation text template: %w", err)
	}

	// Build and send the email with attachment
	subject := fmt.Sprintf("Updated Invitation: %s", cancellation.MeetingTitle)
	message := buildEmailMessageWithParams(EmailMessageParams{
		Recipient:   cancellation.RecipientEmail,
		Subject:     subject,
		HTMLContent: htmlContent,
		TextContent: textContent,
		Attachment:  attachment,
		Config:      s.config,
		Metadata: &EmailMetadata{
			ProjectName: cancellation.ProjectName,
		},
	})
	err = sendEmailMessage(cancellation.RecipientEmail, message, s.config)
	if err != nil {
		slog.ErrorContext(ctx, "failed to send occurrence cancellation email", logging.ErrKey, err)
		return err
	}

	slog.InfoContext(ctx, "occurrence cancellation email sent successfully")
	if attachment != nil {
		slog.InfoContext(ctx, "ICS occurrence cancellation attachment included")
	}
	return nil
}

// SendRegistrantUpdatedInvitation sends an update notification email to a meeting registrant
func (s *SMTPService) SendRegistrantUpdatedInvitation(ctx context.Context, updatedInvitation domain.EmailUpdatedInvitation) error {
	ctx = logging.AppendCtx(ctx, slog.String("recipient_email", redaction.RedactEmail(updatedInvitation.RecipientEmail)))
	ctx = logging.AppendCtx(ctx, slog.String("meeting_title", updatedInvitation.MeetingTitle))

	// Generate ICS update file if we have the necessary data and it's a future meeting
	var attachment *domain.EmailAttachment
	if updatedInvitation.StartTime.After(time.Now()) {
		icsContent, err := s.icsGenerator.GenerateMeetingUpdateICS(ICSMeetingUpdateParams{
			MeetingUID:     updatedInvitation.MeetingUID,
			MeetingTitle:   updatedInvitation.MeetingTitle,
			Description:    updatedInvitation.Description,
			StartTime:      updatedInvitation.StartTime,
			Duration:       updatedInvitation.Duration,
			Timezone:       updatedInvitation.Timezone,
			JoinLink:       updatedInvitation.JoinLink,
			MeetingID:      updatedInvitation.MeetingID,
			Passcode:       updatedInvitation.Passcode,
			RecipientEmail: updatedInvitation.RecipientEmail,
			ProjectName:    updatedInvitation.ProjectName,
			Recurrence:     updatedInvitation.Recurrence,
			Sequence:       updatedInvitation.IcsSequence,
			Attachments:    updatedInvitation.Attachments,
		})
		if err != nil {
			slog.ErrorContext(ctx, "failed to generate ICS update", logging.ErrKey, err)
			// Continue without ICS - don't fail the whole email
		} else {
			// Create attachment
			attachment = &domain.EmailAttachment{
				Filename:    fmt.Sprintf("%s-updated.ics", strings.ReplaceAll(updatedInvitation.MeetingTitle, " ", "_")),
				ContentType: "text/calendar; charset=UTF-8; method=REQUEST",
				Content:     base64.StdEncoding.EncodeToString([]byte(icsContent)),
			}
			updatedInvitation.ICSAttachment = attachment
		}
	}

	// Generate email content from templates
	htmlContent, err := renderTemplate(s.templates.Meeting.UpdatedInvitation.HTML, updatedInvitation)
	if err != nil {
		slog.ErrorContext(ctx, "failed to render updated invitation HTML template", logging.ErrKey, err)
		return fmt.Errorf("failed to render updated invitation HTML template: %w", err)
	}

	textContent, err := renderTemplate(s.templates.Meeting.UpdatedInvitation.Text, updatedInvitation)
	if err != nil {
		slog.ErrorContext(ctx, "failed to render updated invitation text template", logging.ErrKey, err)
		return fmt.Errorf("failed to render updated invitation text template: %w", err)
	}

	// Build and send the email with attachments (ICS + file attachments)
	subject := fmt.Sprintf("Meeting Updated: %s", updatedInvitation.MeetingTitle)
	message := buildEmailMessageWithParams(EmailMessageParams{
		Recipient:   updatedInvitation.RecipientEmail,
		Subject:     subject,
		HTMLContent: htmlContent,
		TextContent: textContent,
		Attachment:  attachment,                        // ICS calendar attachment
		Attachments: updatedInvitation.FileAttachments, // Meeting file attachments
		Config:      s.config,
		Metadata: &EmailMetadata{
			ProjectName: updatedInvitation.ProjectName,
		},
	})

	err = sendEmailMessage(updatedInvitation.RecipientEmail, message, s.config)
	if err != nil {
		slog.ErrorContext(ctx, "failed to send updated invitation email", logging.ErrKey, err)
		return err
	}

	slog.InfoContext(ctx, "updated invitation email sent successfully")
	if attachment != nil {
		slog.InfoContext(ctx, "ICS update attachment included")
	}
	return nil
}

// SendSummaryNotification sends a meeting summary notification email to a meeting host
func (s *SMTPService) SendSummaryNotification(ctx context.Context, notification domain.EmailSummaryNotification) error {
	ctx = logging.AppendCtx(ctx, slog.String("recipient_email", redaction.RedactEmail(notification.RecipientEmail)))
	ctx = logging.AppendCtx(ctx, slog.String("meeting_title", notification.MeetingTitle))

	// Generate email content from templates
	htmlContent, err := renderTemplate(s.templates.Meeting.SummaryNotification.HTML, notification)
	if err != nil {
		slog.ErrorContext(ctx, "failed to render summary notification HTML template", logging.ErrKey, err)
		return fmt.Errorf("failed to render summary notification HTML template: %w", err)
	}

	textContent, err := renderTemplate(s.templates.Meeting.SummaryNotification.Text, notification)
	if err != nil {
		slog.ErrorContext(ctx, "failed to render summary notification text template", logging.ErrKey, err)
		return fmt.Errorf("failed to render summary notification text template: %w", err)
	}

	// Build and send the email
	subject := fmt.Sprintf("Meeting Summary Available: %s", notification.MeetingTitle)
	message := buildEmailMessageWithParams(EmailMessageParams{
		Recipient:   notification.RecipientEmail,
		Subject:     subject,
		HTMLContent: htmlContent,
		TextContent: textContent,
		Attachment:  nil, // No attachments for summary notifications
		Config:      s.config,
		Metadata: &EmailMetadata{
			ProjectName: notification.ProjectName,
		},
	})

	err = sendEmailMessage(notification.RecipientEmail, message, s.config)
	if err != nil {
		slog.ErrorContext(ctx, "failed to send summary notification email", logging.ErrKey, err)
		return err
	}

	slog.InfoContext(ctx, "summary notification email sent successfully")
	return nil
}
