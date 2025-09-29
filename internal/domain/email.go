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
	SendRegistrantUpdatedInvitation(ctx context.Context, updatedInvitation EmailUpdatedInvitation) error
	SendSummaryNotification(ctx context.Context, notification EmailSummaryNotification) error
}

// EmailInvitation contains the data needed to send a meeting invitation email
type EmailInvitation struct {
	MeetingUID     string // Meeting UID for consistent calendar event identification
	RecipientEmail string
	RecipientName  string
	MeetingTitle   string
	StartTime      time.Time
	Duration       int // Duration in minutes
	Timezone       string
	Description    string
	MeetingType    string
	Visibility     string
	JoinLink       string
	ProjectName    string             // Optional project name for context
	ProjectLogo    string             // Optional project logo URL
	Platform       string             // Meeting platform (e.g., "Zoom")
	MeetingID      string             // Zoom meeting ID for dial-in
	Passcode       string             // Zoom passcode
	Recurrence     *models.Recurrence // Recurrence pattern for ICS
	ICSAttachment  *EmailAttachment   // ICS calendar attachment
}

// EmailCancellation contains the data needed to send a meeting cancellation email
type EmailCancellation struct {
	MeetingUID     string // Meeting UID for consistent calendar event identification
	RecipientEmail string
	RecipientName  string
	MeetingTitle   string
	StartTime      time.Time
	Duration       int // Duration in minutes
	Timezone       string
	Description    string
	ProjectName    string             // Optional project name for context
	ProjectLogo    string             // Optional project logo URL
	Reason         string             // Optional reason for cancellation
	Recurrence     *models.Recurrence // Recurrence pattern for ICS
	ICSAttachment  *EmailAttachment   // ICS calendar attachment for cancellation
}

// EmailUpdatedInvitation contains the data needed to send a meeting update notification email
type EmailUpdatedInvitation struct {
	MeetingUID     string // Meeting UID for consistent calendar event identification
	RecipientEmail string
	RecipientName  string
	MeetingTitle   string
	StartTime      time.Time
	Duration       int // Duration in minutes
	Timezone       string
	Description    string
	JoinLink       string
	Visibility     string
	MeetingType    string
	ProjectName    string             // Optional project name for context
	ProjectLogo    string             // Optional project logo URL
	Platform       string             // Meeting platform (e.g., "Zoom")
	MeetingID      string             // Zoom meeting ID for dial-in
	Passcode       string             // Zoom passcode
	Recurrence     *models.Recurrence // Recurrence pattern for ICS
	Changes        map[string]any     // Map of what changed (field names to new values)
	ICSAttachment  *EmailAttachment   // Updated ICS calendar attachment

	// Previous meeting data for showing what changed
	OldStartTime   time.Time          // Previous start time
	OldDuration    int                // Previous duration in minutes
	OldTimezone    string             // Previous timezone
	OldRecurrence  *models.Recurrence // Previous recurrence pattern
	OldDescription string             // Previous description
}

// EmailSummaryNotification contains the data needed to send a meeting summary notification email
type EmailSummaryNotification struct {
	RecipientEmail string    // Email address of the recipient
	RecipientName  string    // Name of the recipient
	MeetingTitle   string    // Title of the meeting
	MeetingDate    time.Time // Date when the meeting occurred
	ProjectName    string    // Optional project name for context
	ProjectLogo    string    // Optional project logo URL
	SummaryContent string    // The summary content
	SummaryDocURL  string    // Optional URL to the full summary document
	SummaryTitle   string    // Title of the summary (if different from meeting title)
}

// EmailAttachment represents a file attachment for an email
type EmailAttachment struct {
	Filename    string // Name of the attachment file
	ContentType string // MIME type of the attachment
	Content     string // Base64 encoded content
}
