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

// buildEmailMessage builds the complete email message with headers and multipart content
func buildEmailMessage(recipient, subject, htmlContent, textContent string, config SMTPConfig) string {
	boundary := generateBoundary()
	messageID := generateMessageID(config)

	var message strings.Builder

	// RFC 5322 required and recommended headers
	message.WriteString(fmt.Sprintf("From: %s\r\n", config.From))
	message.WriteString(fmt.Sprintf("To: %s\r\n", recipient))
	message.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	message.WriteString(fmt.Sprintf("Date: %s\r\n", time.Now().Format(time.RFC1123Z)))
	message.WriteString(fmt.Sprintf("Message-ID: %s\r\n", messageID))
	message.WriteString("MIME-Version: 1.0\r\n")
	message.WriteString(fmt.Sprintf("Content-Type: multipart/alternative; boundary=\"%s\"\r\n", boundary))
	message.WriteString("\r\n")

	// Plain text part
	message.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	message.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	message.WriteString("Content-Transfer-Encoding: 8bit\r\n")
	message.WriteString("\r\n")
	message.WriteString(textContent)
	message.WriteString("\r\n")

	// HTML part
	message.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	message.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	message.WriteString("Content-Transfer-Encoding: 8bit\r\n")
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
