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
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/infrastructure/zoom/webhook"
)

// ZoomConfig holds Zoom-specific configuration
type ZoomConfig struct {
	AccountID          string
	ClientID           string
	ClientSecret       string
	WebhookSecretToken string
}

// NewZoomConfigFromEnv creates a ZoomConfig from environment variables
func NewZoomConfigFromEnv() ZoomConfig {
	return ZoomConfig{
		AccountID:          os.Getenv("ZOOM_ACCOUNT_ID"),
		ClientID:           os.Getenv("ZOOM_CLIENT_ID"),
		ClientSecret:       os.Getenv("ZOOM_CLIENT_SECRET"),
		WebhookSecretToken: os.Getenv("ZOOM_WEBHOOK_SECRET_TOKEN"),
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

// SetupZoom configures Zoom platform integration and returns the webhook validator if configured
func SetupZoom(platformRegistry domain.PlatformRegistry, config ZoomConfig) domain.WebhookValidator {
	// Setup Zoom platform provider
	if config.IsConfigured() {
		zoomClient := api.NewClient(config.ToAPIConfig())
		zoomProvider := zoom.NewZoomProvider(zoomClient)
		platformRegistry.RegisterProvider(models.PlatformZoom, zoomProvider)

		slog.Info("Zoom platform integration configured",
			"account_id", config.AccountID,
			"client_id", config.ClientID)
	} else {
		slog.Warn("Zoom platform integration not configured - missing required environment variables",
			"has_account_id", config.AccountID != "",
			"has_client_id", config.ClientID != "",
			"has_client_secret", config.ClientSecret != "")
	}

	// Setup Zoom webhook validator if webhook secret is configured
	if config.WebhookSecretToken != "" {
		validator := webhook.NewZoomWebhookValidator(config.WebhookSecretToken)
		slog.Info("Zoom webhook validation configured")
		return validator
	}

	slog.Warn("Zoom webhook validation not configured - missing ZOOM_WEBHOOK_SECRET_TOKEN")
	return nil
}
