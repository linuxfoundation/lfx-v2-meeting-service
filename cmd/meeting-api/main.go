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
	"time"

	"github.com/linuxfoundation/lfx-v2-meeting-service/cmd/meeting-api/platforms"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/handlers"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/infrastructure/idmapper"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/infrastructure/messaging"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/infrastructure/proxy"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/service"
	itxservice "github.com/linuxfoundation/lfx-v2-meeting-service/internal/service/itx"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/constants"
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

	// Initialize platform providers
	platformConfigs := platforms.NewPlatformConfigsFromEnv()
	platformConfigs.Zoom = platforms.SetupZoom(platformConfigs.Zoom)
	platformRegistry := platforms.NewPlatformRegistry(platformConfigs)

	// Initialize email service (independent of NATS)
	emailService, err := setupEmailService(env)
	if err != nil {
		slog.With(logging.ErrKey, err).Error("error setting up email service")
		return 1
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
		return 1
	}

	// Get the key-value stores for the service.
	repos, err := getStorageRepos(ctx, natsConn)
	if err != nil {
		slog.With(logging.ErrKey, err).Error("error getting key-value stores")
		return 1
	}

	// Initialize services
	serviceConfig := service.ServiceConfig{
		SkipEtagValidation: env.SkipEtagValidation,
		ProjectLogoBaseURL: env.ProjectLogoBaseURL,
		LfxURLGenerator:    constants.NewLfxURLGenerator(env.LFXEnvironment, env.LFXAppOrigin),
	}
	messageBuilder := messaging.NewMessageBuilder(natsConn)
	authService := service.NewAuthService(jwtAuth)
	occurrenceService := service.NewOccurrenceService()
	attachmentService := service.NewMeetingAttachmentService(
		repos.Attachment,
		repos.Meeting,
		messageBuilder, // Implements MeetingAttachmentIndexSender
		messageBuilder, // Implements MeetingAttachmentAccessSender
	)
	meetingService := service.NewMeetingService(
		repos.Meeting,
		repos.Registrant,
		repos.MeetingRSVP,
		messageBuilder,
		messageBuilder,
		platformRegistry,
		occurrenceService,
		emailService,
		attachmentService,
		serviceConfig,
	)
	registrantService := service.NewMeetingRegistrantService(
		repos.Meeting,
		repos.Registrant,
		emailService,
		messageBuilder,
		messageBuilder,
		attachmentService,
		occurrenceService,
		serviceConfig,
	)
	meetingRSVPService := service.NewMeetingRSVPService(
		repos.MeetingRSVP,
		repos.Meeting,
		repos.Registrant,
		occurrenceService,
		messageBuilder, // Implements MeetingRSVPIndexSender
	)
	pastMeetingService := service.NewPastMeetingService(
		repos.Meeting,
		repos.PastMeeting,
		repos.Attachment,
		repos.PastMeetingAttachment,
		messageBuilder, // Implements PastMeetingBasicMessageSender
		serviceConfig,
	)
	pastMeetingParticipantService := service.NewPastMeetingParticipantService(
		repos.Meeting,
		repos.PastMeeting,
		repos.PastMeetingParticipant,
		messageBuilder, // Implements PastMeetingParticipantMessageSender
		serviceConfig,
	)
	pastMeetingRecordingService := service.NewPastMeetingRecordingService(
		repos.PastMeetingRecording,
		repos.PastMeeting,
		repos.PastMeetingParticipant,
		messageBuilder, // Implements PastMeetingRecordingMessageSender
		serviceConfig,
	)
	pastMeetingTranscriptService := service.NewPastMeetingTranscriptService(
		repos.PastMeetingTranscript,
		repos.PastMeeting,
		repos.PastMeetingParticipant,
		messageBuilder, // Implements PastMeetingTranscriptMessageSender
		serviceConfig,
	)
	pastMeetingSummaryService := service.NewPastMeetingSummaryService(
		repos.PastMeetingSummary,
		repos.PastMeeting,
		repos.PastMeetingParticipant,
		repos.Registrant,
		repos.Meeting,
		emailService,
		messageBuilder, // Implements PastMeetingSummaryMessageSender
		messageBuilder, // Implements ExternalServiceClient
		serviceConfig,
	)
	pastMeetingAttachmentService := service.NewPastMeetingAttachmentService(
		repos.PastMeeting,
		repos.PastMeetingAttachment,
		repos.Attachment,
		messageBuilder, // Implements PastMeetingAttachmentIndexSender
		messageBuilder, // Implements PastMeetingAttachmentAccessSender
		serviceConfig,
	)
	committeeSyncService := service.NewCommitteeSyncService(
		repos.Meeting,
		repos.Registrant,
		registrantService,
		messageBuilder, // Implements MeetingRegistrantIndexSender
		messageBuilder, // Implements ExternalServiceClient
	)
	zoomWebhookService := service.NewZoomWebhookService(
		messageBuilder, // Implements WebhookEventSender
		platformConfigs.Zoom.Validator,
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
		pastMeetingTranscriptService,
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
	projectHandler := handlers.NewProjectHandlers(
		meetingService,
	)

	// Initialize ID mapper for v1/v2 ID conversions
	var idMapper domain.IDMapper
	if env.IDMappingDisabled {
		slog.WarnContext(ctx, "ID mapping is DISABLED - using no-op mapper (IDs will pass through unchanged)")
		idMapper = idmapper.NewNoOpMapper()
	} else {
		natsMapper, err := idmapper.NewNATSMapper(idmapper.Config{
			URL:     env.NatsURL,
			Timeout: 5 * time.Second,
		})
		if err != nil {
			slog.With(logging.ErrKey, err).Error("Failed to initialize ID mapper")
			return 1
		}
		defer natsMapper.Close()
		idMapper = natsMapper
		slog.InfoContext(ctx, "ID mapping enabled - using NATS mapper for v1/v2 ID conversions")
	}

	// Initialize ITX proxy client and service (if enabled)
	var itxMeetingService *itxservice.MeetingService
	if env.ITXConfig.Enabled {
		itxProxyConfig := proxy.Config{
			BaseURL:      env.ITXConfig.BaseURL,
			ClientID:     env.ITXConfig.ClientID,
			ClientSecret: env.ITXConfig.ClientSecret,
			Auth0Domain:  env.ITXConfig.Auth0Domain,
			Audience:     env.ITXConfig.Audience,
			Timeout:      30 * time.Second,
		}
		itxProxyClient := proxy.NewClient(itxProxyConfig)
		itxMeetingService = itxservice.NewMeetingService(itxProxyClient, idMapper)
		slog.InfoContext(ctx, "ITX proxy client initialized")
	}

	svc := NewMeetingsAPI(
		authService,
		meetingService,
		registrantService,
		meetingRSVPService,
		attachmentService,
		pastMeetingService,
		pastMeetingParticipantService,
		pastMeetingSummaryService,
		pastMeetingAttachmentService,
		zoomWebhookService,
		zoomWebhookHandler,
		meetingHandler,
		committeeHandler,
		projectHandler,
		itxMeetingService,
	)

	httpServer := setupHTTPServer(flags, svc, &gracefulCloseWG)

	// Create NATS subscriptions for the service.
	err = createNatsSubcriptions(ctx, svc, natsConn)
	if err != nil {
		slog.With(logging.ErrKey, err).Error("error creating NATS subscriptions")
		return 1
	}

	// This next line blocks until SIGINT or SIGTERM is received.
	<-done

	gracefulShutdown(httpServer, natsConn, &gracefulCloseWG, cancel)

	return 0
}
