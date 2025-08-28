// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// Package platforms provides platform integration setup for the meeting service.
// It handles the initialization and configuration of various meeting platform providers
// such as Zoom, Teams, Google Meet, etc.
package platforms

import (
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/infrastructure/platform"
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

// NewPlatformRegistry creates and configures a platform registry with all available platforms
func NewPlatformRegistry(configs PlatformConfigs) domain.PlatformRegistry {
	registry := platform.NewRegistry()

	// Register Zoom if configured
	registry.RegisterProvider(models.PlatformZoom, configs.Zoom.PlatformProvider)

	return registry
}
