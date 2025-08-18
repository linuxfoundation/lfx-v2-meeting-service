// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package platforms

import (
	"log/slog"
	"os"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/infrastructure/zoom"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/infrastructure/zoom/api"
)

// ZoomConfig holds Zoom-specific configuration
type ZoomConfig struct {
	AccountID    string
	ClientID     string
	ClientSecret string
}

// NewZoomConfigFromEnv creates a ZoomConfig from environment variables
func NewZoomConfigFromEnv() ZoomConfig {
	return ZoomConfig{
		AccountID:    os.Getenv("ZOOM_ACCOUNT_ID"),
		ClientID:     os.Getenv("ZOOM_CLIENT_ID"),
		ClientSecret: os.Getenv("ZOOM_CLIENT_SECRET"),
	}
}

// IsConfigured returns true if all required Zoom credentials are provided
func (z ZoomConfig) IsConfigured() bool {
	return z.AccountID != "" && z.ClientID != "" && z.ClientSecret != ""
}

// ToAPIConfig converts the ZoomConfig to an api.Config
func (z ZoomConfig) ToAPIConfig() api.Config {
	return api.Config{
		AccountID:    z.AccountID,
		ClientID:     z.ClientID,
		ClientSecret: z.ClientSecret,
	}
}

// SetupZoom configures Zoom integration and registers it with the platform registry
func SetupZoom(registry domain.PlatformRegistry, config ZoomConfig) {
	if !config.IsConfigured() {
		slog.Warn("Zoom integration not configured - missing required environment variables",
			"has_account_id", config.AccountID != "",
			"has_client_id", config.ClientID != "",
			"has_client_secret", config.ClientSecret != "")
		return
	}

	zoomClient := api.NewClient(config.ToAPIConfig())
	zoomProvider := zoom.NewZoomProvider(zoomClient)
	registry.RegisterProvider(models.PlatformZoom, zoomProvider)

	slog.Info("Zoom integration configured",
		"account_id", config.AccountID,
		"client_id", config.ClientID)
}
