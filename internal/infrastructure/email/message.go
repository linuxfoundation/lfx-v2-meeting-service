// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package email

import (
	"fmt"
	"net/smtp"
	"strings"
)

// buildEmailMessage builds the complete email message with headers and multipart content
func buildEmailMessage(recipient, subject, htmlContent, textContent string, config SMTPConfig) string {
	boundary := "===============1234567890123456789=="

	var message strings.Builder

	// Email headers
	message.WriteString(fmt.Sprintf("From: %s\r\n", config.From))
	message.WriteString(fmt.Sprintf("To: %s\r\n", recipient))
	message.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
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

// sendEmailMessage sends a pre-built email message via SMTP
func sendEmailMessage(recipient, message string, config SMTPConfig) error {
	addr := fmt.Sprintf("%s:%d", config.Host, config.Port)

	var auth smtp.Auth
	if config.Username != "" && config.Password != "" {
		auth = smtp.PlainAuth("", config.Username, config.Password, config.Host)
	}

	err := smtp.SendMail(addr, auth, config.From, []string{recipient}, []byte(message))
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}
