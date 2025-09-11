// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// Package main is the meeting service API that provides a RESTful API for managing meetings
// and handles NATS messages for the meeting service.
package main

import (
	"context"
	_ "expvar"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/linuxfoundation/lfx-v2-meeting-service/cmd/meeting-api/platforms"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/handlers"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/infrastructure/messaging"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/service"
)

func main() {
	env := parseEnv()
	flags := parseFlags(env.Port)

	logging.InitStructureLogConfig()

	// Set up JWT validator needed by the [MeetingsService.JWTAuth] security handler.
	jwtAuth, err := setupJWTAuth()
	if err != nil {
		slog.With(logging.ErrKey, err).Error("error setting up JWT authentication")
		os.Exit(1)
	}

	// Initialize platform providers
	platformConfigs := platforms.NewPlatformConfigsFromEnv()
	platformConfigs.Zoom = platforms.SetupZoom(platformConfigs.Zoom)
	platformRegistry := platforms.NewPlatformRegistry(platformConfigs)

	// Initialize email service (independent of NATS)
	emailService, err := setupEmailService(env)
	if err != nil {
		slog.With(logging.ErrKey, err).Error("error setting up email service")
		return
	}

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	gracefulCloseWG := sync.WaitGroup{}

	// Setup NATS connection
	natsConn, err := setupNATS(ctx, env, &gracefulCloseWG, done)
	if err != nil {
		slog.With(logging.ErrKey, err).Error("error setting up NATS")
		return
	}

	// Get the key-value stores for the service.
	repos, err := getKeyValueStores(ctx, natsConn)
	if err != nil {
		slog.With(logging.ErrKey, err).Error("error getting key-value stores")
		return
	}

	// Initialize services
	serviceConfig := service.ServiceConfig{
		SkipEtagValidation: env.SkipEtagValidation,
	}
	messageBuilder := messaging.NewMessageBuilder(natsConn)
	authService := service.NewAuthService(jwtAuth)
	occurrenceService := service.NewOccurrenceService()
	meetingService := service.NewMeetingService(
		repos.Meeting,
		messageBuilder,
		platformRegistry,
		occurrenceService,
		serviceConfig,
	)
	registrantService := service.NewMeetingRegistrantService(
		repos.Meeting,
		repos.Registrant,
		emailService,
		messageBuilder,
		serviceConfig,
	)
	pastMeetingService := service.NewPastMeetingService(
		repos.Meeting,
		repos.PastMeeting,
		messageBuilder,
		serviceConfig,
	)
	pastMeetingParticipantService := service.NewPastMeetingParticipantService(
		repos.Meeting,
		repos.PastMeeting,
		repos.PastMeetingParticipant,
		messageBuilder,
		serviceConfig,
	)
	pastMeetingRecordingService := service.NewPastMeetingRecordingService(
		repos.PastMeetingRecording,
		messageBuilder,
		serviceConfig,
	)
	pastMeetingSummaryService := service.NewPastMeetingSummaryService(
		repos.PastMeetingSummary,
		repos.PastMeeting,
		messageBuilder,
		serviceConfig,
	)
	committeeSyncService := service.NewCommitteeSyncService(
		repos.Registrant,
		registrantService,
		messageBuilder,
	)

	// Initialize handlers
	meetingHandler := handlers.NewMeetingHandler(
		meetingService,
		registrantService,
		pastMeetingService,
		pastMeetingParticipantService,
		committeeSyncService,
	)
	zoomWebhookHandler := handlers.NewZoomWebhookHandler(
		meetingService,
		registrantService,
		pastMeetingService,
		pastMeetingParticipantService,
		pastMeetingRecordingService,
		pastMeetingSummaryService,
		occurrenceService,
		platformConfigs.Zoom.Validator,
	)
	committeeHandler := handlers.NewCommitteeHandlers(
		meetingService,
		registrantService,
		committeeSyncService,
		messageBuilder,
	)

	svc := NewMeetingsAPI(
		authService,
		meetingService,
		registrantService,
		pastMeetingService,
		pastMeetingParticipantService,
		pastMeetingSummaryService,
		zoomWebhookHandler,
		meetingHandler,
		committeeHandler,
	)

	httpServer := setupHTTPServer(flags, svc, &gracefulCloseWG)

	// Create NATS subscriptions for the service.
	err = createNatsSubcriptions(ctx, svc, natsConn)
	if err != nil {
		slog.With(logging.ErrKey, err).Error("error creating NATS subscriptions")
		return
	}

	// This next line blocks until SIGINT or SIGTERM is received.
	<-done

	gracefulShutdown(httpServer, natsConn, &gracefulCloseWG, cancel)
}
