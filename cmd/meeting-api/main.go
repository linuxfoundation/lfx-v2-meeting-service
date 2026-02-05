// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// Package main is the ITX meeting proxy service that provides a lightweight proxy layer to the ITX Zoom API.
package main

import (
	"context"
	_ "expvar"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/infrastructure/idmapper"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/infrastructure/proxy"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/service"
	itxservice "github.com/linuxfoundation/lfx-v2-meeting-service/internal/service/itx"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/utils"
)

// Build-time variables set via ldflags
var (
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

func main() {
	os.Exit(run())
}

func run() int {
	env := parseEnv()
	flags := parseFlags(env.Port)

	logging.InitStructureLogConfig()

	// Set up OpenTelemetry SDK.
	// Command-line/environment OTEL_SERVICE_VERSION takes precedence over
	// the build-time Version variable.
	otelConfig := utils.OTelConfigFromEnv()
	if otelConfig.ServiceVersion == "" {
		otelConfig.ServiceVersion = Version
	}
	otelShutdown, err := utils.SetupOTelSDKWithConfig(context.Background(), otelConfig)
	if err != nil {
		slog.With(logging.ErrKey, err).Error("error setting up OpenTelemetry SDK")
		return 1
	}
	// Handle shutdown properly so nothing leaks.
	defer func() {
		if shutdownErr := otelShutdown(context.Background()); shutdownErr != nil {
			slog.With(logging.ErrKey, shutdownErr).Error("error shutting down OpenTelemetry SDK")
		}
	}()

	// Set up JWT validator needed by the [MeetingsService.JWTAuth] security handler.
	jwtAuth, err := setupJWTAuth()
	if err != nil {
		slog.With(logging.ErrKey, err).Error("error setting up JWT authentication")
		return 1
	}

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	gracefulCloseWG := sync.WaitGroup{}

	// Initialize services
	authService := service.NewAuthService(jwtAuth)

	// Initialize ID mapper for v1/v2 ID conversions
	var idMapper domain.IDMapper
	if env.IDMappingDisabled {
		slog.WarnContext(ctx, "ID mapping is DISABLED - using no-op mapper (IDs will pass through unchanged)")
		idMapper = idmapper.NewNoOpMapper()
	} else {
		// For ITX proxy, we still need ID mapping if NATS is available
		// Check if NATS_URL is set for ID mapping
		natsURL := os.Getenv("NATS_URL")
		if natsURL != "" {
			natsMapper, err := idmapper.NewNATSMapper(idmapper.Config{
				URL:     natsURL,
				Timeout: 5 * time.Second,
			})
			if err != nil {
				slog.With(logging.ErrKey, err).Warn("Failed to initialize NATS ID mapper, falling back to no-op mapper")
				idMapper = idmapper.NewNoOpMapper()
			} else {
				defer natsMapper.Close()
				idMapper = natsMapper
				slog.InfoContext(ctx, "ID mapping enabled - using NATS mapper for v1/v2 ID conversions")
			}
		} else {
			slog.WarnContext(ctx, "NATS_URL not set, using no-op ID mapper")
			idMapper = idmapper.NewNoOpMapper()
		}
	}

	// Initialize ITX proxy client and services
	itxProxyConfig := proxy.Config{
		BaseURL:      env.ITXConfig.BaseURL,
		ClientID:     env.ITXConfig.ClientID,
		ClientSecret: env.ITXConfig.ClientSecret,
		Auth0Domain:  env.ITXConfig.Auth0Domain,
		Audience:     env.ITXConfig.Audience,
		Timeout:      30 * time.Second,
	}
	itxProxyClient := proxy.NewClient(itxProxyConfig)
	itxMeetingService := itxservice.NewMeetingService(itxProxyClient, idMapper)
	itxRegistrantService := itxservice.NewRegistrantService(itxProxyClient, idMapper)
	itxPastMeetingService := itxservice.NewPastMeetingService(itxProxyClient, idMapper)
	itxPastMeetingSummaryService := itxservice.NewPastMeetingSummaryService(itxProxyClient)
	slog.InfoContext(ctx, "ITX proxy client initialized")

	svc := NewMeetingsAPI(
		authService,
		itxMeetingService,
		itxRegistrantService,
		itxPastMeetingService,
		itxPastMeetingSummaryService,
	)

	httpServer := setupHTTPServer(flags, svc, &gracefulCloseWG)

	slog.InfoContext(ctx, "ITX meeting proxy service started",
		"version", Version,
		"build_time", BuildTime,
		"git_commit", GitCommit,
		"port", flags.Port,
	)

	// This next line blocks until SIGINT or SIGTERM is received.
	<-done

	gracefulShutdown(httpServer, &gracefulCloseWG, cancel)

	return 0
}

// gracefulShutdown handles graceful shutdown of the application
func gracefulShutdown(httpServer *http.Server, gracefulCloseWG *sync.WaitGroup, cancel context.CancelFunc) {
	// Cancel the background context.
	cancel()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()

		slog.With("addr", httpServer.Addr).Info("shutting down http server")
		if err := httpServer.Shutdown(ctx); err != nil {
			slog.With(logging.ErrKey, err).Error("http shutdown error")
		}
		// Decrement the wait group.
		gracefulCloseWG.Done()
	}()

	// Wait for the HTTP graceful shutdown
	gracefulCloseWG.Wait()
}
