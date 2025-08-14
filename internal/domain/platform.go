// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package domain

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
)

// PlatformProvider defines the interface for external meeting platform integrations
type PlatformProvider interface {
	// CreateMeeting creates a meeting on the external platform
	// Returns the platform-specific meeting ID and join URL
	CreateMeeting(ctx context.Context, meeting *models.MeetingBase) (platformMeetingID string, joinURL string, err error)

	// UpdateMeeting updates an existing meeting on the external platform
	UpdateMeeting(ctx context.Context, platformMeetingID string, meeting *models.MeetingBase) error

	// DeleteMeeting deletes a meeting from the external platform
	DeleteMeeting(ctx context.Context, platformMeetingID string) error
}

// PlatformRegistry manages platform providers
type PlatformRegistry interface {
	// GetProvider returns the platform provider for the specified platform name
	GetProvider(platform string) (PlatformProvider, error)

	// RegisterProvider registers a platform provider
	RegisterProvider(platform string, provider PlatformProvider)
}
