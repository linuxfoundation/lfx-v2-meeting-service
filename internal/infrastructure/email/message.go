// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package email

import (
	"crypto/rand"
	"fmt"
	"net/mail"
	"net/smtp"
	"strings"
	"time"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
)

// generateBoundary creates a unique boundary string for multipart messages
func generateBoundary() string {
	bytes := make([]byte, 16)
	_, _ = rand.Read(bytes)
	return fmt.Sprintf("===============%x==", bytes)
}

// generateMessageID creates a unique Message-ID header
func generateMessageID(config SMTPConfig) string {
	bytes := make([]byte, 8)
	_, _ = rand.Read(bytes)

	// Extract domain from From address, fallback to hostname
	fromAddr, err := mail.ParseAddress(config.From)
	domain := "localhost"
	if err == nil && strings.Contains(fromAddr.Address, "@") {
		domain = strings.Split(fromAddr.Address, "@")[1]
	}

	return fmt.Sprintf("<%x.%d@%s>", bytes, time.Now().UnixNano(), domain)
}

// EmailMessageParams contains all the information needed to build an email message
type EmailMessageParams struct {
	Recipient   string
	Subject     string
	HTMLContent string
	TextContent string
	Attachment  *domain.EmailAttachment
	Config      SMTPConfig
}

// buildEmailMessage builds the complete email message with headers and multipart content
func buildEmailMessage(recipient, subject, htmlContent, textContent string, config SMTPConfig) string {
	return buildEmailMessageWithParams(EmailMessageParams{
		Recipient:   recipient,
		Subject:     subject,
		HTMLContent: htmlContent,
		TextContent: textContent,
		Attachment:  nil,
		Config:      config,
	})
}

// buildEmailMessageWithAttachment builds the complete email message with optional attachment
func buildEmailMessageWithAttachment(recipient, subject, htmlContent, textContent string, attachment *domain.EmailAttachment, config SMTPConfig) string {
	return buildEmailMessageWithParams(EmailMessageParams{
		Recipient:   recipient,
		Subject:     subject,
		HTMLContent: htmlContent,
		TextContent: textContent,
		Attachment:  attachment,
		Config:      config,
	})
}

// buildEmailMessageWithParams builds the complete email message using structured parameters
func buildEmailMessageWithParams(params EmailMessageParams) string {
	messageID := generateMessageID(params.Config)
	var message strings.Builder

	// RFC 5322 required and recommended headers
	message.WriteString(fmt.Sprintf("From: %s\r\n", params.Config.From))
	message.WriteString(fmt.Sprintf("To: %s\r\n", params.Recipient))
	message.WriteString(fmt.Sprintf("Subject: %s\r\n", params.Subject))
	message.WriteString(fmt.Sprintf("Date: %s\r\n", time.Now().Format(time.RFC1123Z)))
	message.WriteString(fmt.Sprintf("Message-ID: %s\r\n", messageID))
	message.WriteString("MIME-Version: 1.0\r\n")

	if params.Attachment != nil {
		// With attachment: use multipart/mixed
		mixedBoundary := generateBoundary()
		message.WriteString(fmt.Sprintf("Content-Type: multipart/mixed; boundary=\"%s\"\r\n", mixedBoundary))
		message.WriteString("\r\n")

		// Start mixed multipart
		message.WriteString(fmt.Sprintf("--%s\r\n", mixedBoundary))

		// Alternative part (text and HTML)
		alternativeBoundary := generateBoundary()
		message.WriteString(fmt.Sprintf("Content-Type: multipart/alternative; boundary=\"%s\"\r\n", alternativeBoundary))
		message.WriteString("\r\n")

		// Plain text part
		message.WriteString(fmt.Sprintf("--%s\r\n", alternativeBoundary))
		message.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
		message.WriteString("Content-Transfer-Encoding: 8bit\r\n")
		message.WriteString("\r\n")
		message.WriteString(params.TextContent)
		message.WriteString("\r\n")

		// HTML part
		message.WriteString(fmt.Sprintf("--%s\r\n", alternativeBoundary))
		message.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
		message.WriteString("Content-Transfer-Encoding: 8bit\r\n")
		message.WriteString("\r\n")
		message.WriteString(params.HTMLContent)
		message.WriteString("\r\n")

		// End alternative boundary
		message.WriteString(fmt.Sprintf("--%s--\r\n", alternativeBoundary))

		// Attachment part
		message.WriteString(fmt.Sprintf("--%s\r\n", mixedBoundary))
		message.WriteString(fmt.Sprintf("Content-Type: %s; name=\"%s\"\r\n", params.Attachment.ContentType, params.Attachment.Filename))
		message.WriteString(fmt.Sprintf("Content-Disposition: attachment; filename=\"%s\"\r\n", params.Attachment.Filename))
		message.WriteString("Content-Transfer-Encoding: base64\r\n")

		// Add method=REQUEST for calendar files
		if params.Attachment.ContentType == "text/calendar" {
			message.WriteString("Content-Type: text/calendar; charset=UTF-8; method=REQUEST\r\n")
		}

		message.WriteString("\r\n")
		message.WriteString(params.Attachment.Content)
		message.WriteString("\r\n")

		// End mixed boundary
		message.WriteString(fmt.Sprintf("--%s--\r\n", mixedBoundary))
	} else {
		// Without attachment: use multipart/alternative (original logic)
		boundary := generateBoundary()
		message.WriteString(fmt.Sprintf("Content-Type: multipart/alternative; boundary=\"%s\"\r\n", boundary))
		message.WriteString("\r\n")

		// Plain text part
		message.WriteString(fmt.Sprintf("--%s\r\n", boundary))
		message.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
		message.WriteString("Content-Transfer-Encoding: 8bit\r\n")
		message.WriteString("\r\n")
		message.WriteString(params.TextContent)
		message.WriteString("\r\n")

		// HTML part
		message.WriteString(fmt.Sprintf("--%s\r\n", boundary))
		message.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
		message.WriteString("Content-Transfer-Encoding: 8bit\r\n")
		message.WriteString("\r\n")
		message.WriteString(params.HTMLContent)
		message.WriteString("\r\n")

		// End boundary
		message.WriteString(fmt.Sprintf("--%s--\r\n", boundary))
	}

	return message.String()
}

// sendEmailMessage sends a pre-built email message via SMTP
func sendEmailMessage(recipient, message string, config SMTPConfig) error {
	addr := fmt.Sprintf("%s:%d", config.Host, config.Port)

	var auth smtp.Auth
	if config.Username != "" && config.Password != "" {
		auth = smtp.PlainAuth("", config.Username, config.Password, config.Host)
	}

	fromAddr, err := mail.ParseAddress(config.From)
	if err != nil {
		return fmt.Errorf("invalid From address: %w", err)
	}
	toAddr, err := mail.ParseAddress(recipient)
	if err != nil {
		return fmt.Errorf("invalid recipient address: %w", err)
	}

	err = smtp.SendMail(addr, auth, fromAddr.Address, []string{toAddr.Address}, []byte(message))
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}
