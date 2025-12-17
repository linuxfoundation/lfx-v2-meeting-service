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
	// TODO: Rename this method and audit the parameter to make sure it's as concise as possible
	SendRegistrantCancellation(ctx context.Context, cancellation EmailCancellation) error
	SendOccurrenceCancellation(ctx context.Context, cancellation EmailOccurrenceCancellation) error
	SendRegistrantUpdatedInvitation(ctx context.Context, updatedInvitation EmailUpdatedInvitation) error
	SendSummaryNotification(ctx context.Context, notification EmailSummaryNotification) error
}

// EmailInvitation contains the data needed to send a meeting invitation email
type EmailInvitation struct {
	MeetingUID               string // Meeting UID for consistent calendar event identification
	RecipientEmail           string
	RecipientName            string
	MeetingTitle             string
	StartTime                time.Time
	Duration                 int // Duration in minutes
	Timezone                 string
	Description              string
	MeetingType              string
	Visibility               string
	PlatformJoinLink         string                      // URL to join the meeting via the LFX One platform
	DirectZoomJoinLink       string                      // URL to join the meeting directly via Zoom
	MeetingDetailsLink       string                      // URL to meeting details in LFX One
	ProjectName              string                      // Optional project name for context
	ProjectLogo              string                      // Optional project logo URL
	Platform                 string                      // Meeting platform (e.g., "Zoom")
	MeetingID                string                      // Zoom meeting ID for dial-in
	Passcode                 string                      // Zoom passcode
	Recurrence               *models.Recurrence          // Recurrence pattern for ICS
	IcsSequence              int                         // ICS sequence number for calendar updates
	ICSAttachment            *EmailAttachment            // ICS calendar attachment
	CancelledOccurrenceTimes []time.Time                 // Cancelled occurrence start times to exclude from ICS
	MeetingAttachments       []*models.MeetingAttachment // Meeting attachments to display in email
	EmailFileAttachments     []*EmailAttachment          // File attachments to include in email attachments
}

// EmailCancellation contains the data needed to send a meeting cancellation email
type EmailCancellation struct {
	MeetingUID         string // Meeting UID for consistent calendar event identification
	RecipientEmail     string
	RecipientName      string
	MeetingTitle       string
	StartTime          time.Time
	Duration           int // Duration in minutes
	Timezone           string
	Description        string
	Visibility         string
	MeetingType        string
	Platform           string             // Meeting platform (e.g., "Zoom")
	MeetingDetailsLink string             // URL to meeting details in LFX One
	ProjectName        string             // Optional project name for context
	ProjectLogo        string             // Optional project logo URL
	Reason             string             // Optional reason for cancellation
	Recurrence         *models.Recurrence // Recurrence pattern for ICS
	IcsSequence        int                // ICS sequence number for calendar updates
	ICSAttachment      *EmailAttachment   // ICS calendar attachment for cancellation
}

// EmailOccurrenceCancellation contains the data needed to send a single occurrence cancellation email
type EmailOccurrenceCancellation struct {
	MeetingUID          string // Meeting UID for consistent calendar event identification
	RecipientEmail      string
	RecipientName       string
	MeetingTitle        string
	OccurrenceID        string    // ID of the cancelled occurrence
	OccurrenceStartTime time.Time // Start time of the cancelled occurrence
	Duration            int       // Duration in minutes
	Timezone            string
	Description         string
	Visibility          string
	MeetingType         string
	Platform            string             // Meeting platform (e.g., "Zoom")
	MeetingDetailsLink  string             // URL to meeting details in LFX One
	ProjectName         string             // Optional project name for context
	ProjectLogo         string             // Optional project logo URL
	Reason              string             // Optional reason for cancellation
	Recurrence          *models.Recurrence // Recurrence pattern of the series for context
	IcsSequence         int                // ICS sequence number for calendar updates
	ICSAttachment       *EmailAttachment   // ICS calendar attachment for occurrence cancellation
}

// EmailUpdatedInvitation contains the data needed to send a meeting update notification email
type EmailUpdatedInvitation struct {
	MeetingUID           string // Meeting UID for consistent calendar event identification
	RecipientEmail       string
	RecipientName        string
	MeetingTitle         string
	StartTime            time.Time
	Duration             int // Duration in minutes
	Timezone             string
	Description          string
	PlatformJoinLink     string // URL to join the meeting via the LFX One platform
	DirectZoomJoinLink   string // URL to join the meeting directly via Zoom
	MeetingDetailsLink   string // URL to meeting details in LFX One
	Visibility           string
	MeetingType          string
	ProjectName          string                      // Optional project name for context
	ProjectLogo          string                      // Optional project logo URL
	Platform             string                      // Meeting platform (e.g., "Zoom")
	MeetingID            string                      // Zoom meeting ID for dial-in
	Passcode             string                      // Zoom passcode
	Recurrence           *models.Recurrence          // Recurrence pattern for ICS
	Changes              map[string]any              // Map of what changed (field names to new values)
	IcsSequence          int                         // ICS sequence number for calendar updates
	ICSAttachment        *EmailAttachment            // Updated ICS calendar attachment
	MeetingAttachments   []*models.MeetingAttachment // Meeting attachments to display in email
	EmailFileAttachments []*EmailAttachment          // File attachments to include in email attachments

	// Previous meeting data for showing what changed
	OldStartTime   time.Time          // Previous start time
	OldDuration    int                // Previous duration in minutes
	OldTimezone    string             // Previous timezone
	OldRecurrence  *models.Recurrence // Previous recurrence pattern
	OldDescription string             // Previous description
}

// EmailSummaryNotification contains the data needed to send a meeting summary notification email
type EmailSummaryNotification struct {
	RecipientEmail     string    // Email address of the recipient
	RecipientName      string    // Name of the recipient
	MeetingTitle       string    // Title of the meeting
	MeetingDate        time.Time // Date when the meeting occurred
	ProjectName        string    // Optional project name for context
	ProjectLogo        string    // Optional project logo URL
	SummaryContent     string    // The summary content
	SummaryTitle       string    // Title of the summary (if different from meeting title)
	MeetingDetailsLink string    // URL to meeting details in LFX One
}

// EmailAttachment represents a file attachment for an email
type EmailAttachment struct {
	Filename    string // Name of the attachment file
	ContentType string // MIME type of the attachment
	Content     string // Base64 encoded content
}
