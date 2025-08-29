// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package domain

import (
	"context"
	"time"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
)

// EmailService defines the interface for sending emails
type EmailService interface {
	SendRegistrantInvitation(ctx context.Context, invitation EmailInvitation) error
	SendRegistrantCancellation(ctx context.Context, cancellation EmailCancellation) error
}

// EmailInvitation contains the data needed to send a meeting invitation email
type EmailInvitation struct {
	RecipientEmail string
	RecipientName  string
	MeetingTitle   string
	StartTime      time.Time
	Duration       int // Duration in minutes
	Timezone       string
	Description    string
	JoinLink       string
	ProjectName    string             // Optional project name for context
	MeetingID      string             // Zoom meeting ID for dial-in
	Passcode       string             // Zoom passcode
	Recurrence     *models.Recurrence // Recurrence pattern for ICS
	ICSAttachment  *EmailAttachment   // ICS calendar attachment
}

// EmailCancellation contains the data needed to send a meeting cancellation email
type EmailCancellation struct {
	RecipientEmail string
	RecipientName  string
	MeetingTitle   string
	StartTime      time.Time
	Duration       int // Duration in minutes
	Timezone       string
	Description    string
	ProjectName    string // Optional project name for context
	Reason         string // Optional reason for cancellation
}

// EmailAttachment represents a file attachment for an email
type EmailAttachment struct {
	Filename    string // Name of the attachment file
	ContentType string // MIME type of the attachment
	Content     string // Base64 encoded content
}
