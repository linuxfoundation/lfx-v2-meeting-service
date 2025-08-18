// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// Package platforms provides platform integration setup for the meeting service.
// It handles the initialization and configuration of various meeting platform providers
// such as Zoom, Teams, Google Meet, etc.
package platforms

import (
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/infrastructure/platform"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/service"
)

// PlatformConfigs holds configuration for all supported platforms
type PlatformConfigs struct {
	Zoom ZoomConfig
}

// NewPlatformConfigsFromEnv creates platform configurations from environment variables
func NewPlatformConfigsFromEnv() PlatformConfigs {
	return PlatformConfigs{
		Zoom: NewZoomConfigFromEnv(),
	}
}

// Initialize sets up all configured platforms and registers them with the service
func Initialize(configs PlatformConfigs, svc *service.MeetingsService) {
	// Create platform registry
	registry := platform.NewRegistry()

	// Setup individual platforms
	SetupZoom(registry, configs.Zoom)

	// Set the platform registry in the service
	svc.PlatformRegistry = registry
}