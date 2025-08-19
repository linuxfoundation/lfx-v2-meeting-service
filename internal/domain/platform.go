// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package domain

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
)

// CreateMeetingResult contains the results from creating a meeting on an external platform
type CreateMeetingResult struct {
	PlatformMeetingID string
	JoinURL           string
	Passcode          string
}

// PlatformProvider defines the interface for external meeting platform integrations
type PlatformProvider interface {
	// CreateMeeting creates a meeting on the external platform
	// Returns comprehensive meeting creation results
	CreateMeeting(ctx context.Context, meeting *models.MeetingBase) (*CreateMeetingResult, error)

	// UpdateMeeting updates an existing meeting on the external platform
	UpdateMeeting(ctx context.Context, platformMeetingID string, meeting *models.MeetingBase) error

	// DeleteMeeting deletes a meeting from the external platform
	DeleteMeeting(ctx context.Context, platformMeetingID string) error

	// StorePlatformData stores platform-specific data in the meeting model after creation
	StorePlatformData(meeting *models.MeetingBase, result *CreateMeetingResult)

	// GetPlatformMeetingID retrieves the platform meeting ID from the meeting model
	GetPlatformMeetingID(meeting *models.MeetingBase) string
}

// PlatformRegistry manages platform providers
type PlatformRegistry interface {
	// GetProvider returns the platform provider for the specified platform name
	GetProvider(platform string) (PlatformProvider, error)

	// RegisterProvider registers a platform provider
	RegisterProvider(platform string, provider PlatformProvider)
}

// WebhookHandler defines the interface for handling webhook events from external platforms
type WebhookHandler interface {
	// HandleEvent processes a webhook event from the platform
	HandleEvent(ctx context.Context, eventType string, payload interface{}) error

	// ValidateSignature validates the webhook signature to ensure authenticity
	ValidateSignature(body []byte, signature, timestamp string) error

	// SupportedEvents returns the list of event types this handler supports
	SupportedEvents() []string
}

// WebhookRegistry manages webhook handlers for different platforms
type WebhookRegistry interface {
	// GetHandler returns the webhook handler for the specified platform
	GetHandler(platform string) (WebhookHandler, error)

	// RegisterHandler registers a webhook handler for a platform
	RegisterHandler(platform string, handler WebhookHandler)
}
