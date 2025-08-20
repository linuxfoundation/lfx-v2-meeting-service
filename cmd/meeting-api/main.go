// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// Package main is the meeting service API that provides a RESTful API for managing meetings
// and handles NATS messages for the meeting service.
package main

import (
	"context"
	_ "expvar"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	nats "github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	goahttp "goa.design/goa/v3/http"

	"github.com/linuxfoundation/lfx-v2-meeting-service/cmd/meeting-api/platforms"
	genhttp "github.com/linuxfoundation/lfx-v2-meeting-service/gen/http/meeting_service/server"
	genquerysvc "github.com/linuxfoundation/lfx-v2-meeting-service/gen/meeting_service"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/infrastructure/auth"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/infrastructure/messaging"
	store "github.com/linuxfoundation/lfx-v2-meeting-service/internal/infrastructure/store"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/middleware"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/service"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/constants"
)

const (
	// errKey is the key for the error field in the slog.
	errKey = "error"
	// gracefulShutdownSeconds should be higher than NATS client
	// request timeout, and lower than the pod or liveness probe's
	// terminationGracePeriodSeconds.
	gracefulShutdownSeconds = 25
)

func main() {
	env := parseEnv()
	flags := parseFlags(env.Port)

	logging.InitStructureLogConfig()

	// Set up JWT validator needed by the [MeetingsService.JWTAuth] security handler.
	jwtAuthConfig := auth.JWTAuthConfig{
		JWKSURL:            os.Getenv("JWKS_URL"),
		Audience:           os.Getenv("JWT_AUDIENCE"),
		MockLocalPrincipal: os.Getenv("JWT_AUTH_DISABLED_MOCK_LOCAL_PRINCIPAL"),
	}
	jwtAuth, err := auth.NewJWTAuth(jwtAuthConfig)
	if err != nil {
		slog.With(errKey, err).Error("error setting up JWT authentication")
		os.Exit(1)
	}

	// Generated service initialization.
	service := service.NewMeetingsService(jwtAuth, service.ServiceConfig{
		SkipEtagValidation: env.SkipEtagValidation,
	})

	// Initialize platform providers
	platformConfigs := platforms.NewPlatformConfigsFromEnv()
	platforms.Initialize(platformConfigs, service)

	svc := NewMeetingsAPI(service)

	gracefulCloseWG := sync.WaitGroup{}

	httpServer := setupHTTPServer(flags, svc, &gracefulCloseWG)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	natsConn, err := setupNATS(ctx, env, svc, &gracefulCloseWG, done)
	if err != nil {
		slog.With(errKey, err).Error("error setting up NATS")
		return
	}

	// This next line blocks until SIGINT or SIGTERM is received.
	<-done

	gracefulShutdown(httpServer, natsConn, &gracefulCloseWG, cancel)

}

// flags are the command line flags for the meeting service.
type flags struct {
	Debug bool
	Port  string
	Bind  string
}

func parseFlags(defaultPort string) flags {
	var debug = flag.Bool("d", false, "enable debug logging")
	var port = flag.String("p", defaultPort, "listen port")
	var bind = flag.String("bind", "*", "interface to bind on")

	flag.Usage = func() {
		flag.PrintDefaults()
		os.Exit(2)
	}
	flag.Parse()

	// Based on the debug flag, set the log level environment variable used by [log.InitStructureLogConfig]
	if *debug {
		err := os.Setenv("LOG_LEVEL", "debug")
		if err != nil {
			slog.With(errKey, err).Error("error setting log level")
			os.Exit(1)
		}
	}

	return flags{
		Debug: *debug,
		Port:  *port,
		Bind:  *bind,
	}
}

// environment are the environment variables for the meeting service.
type environment struct {
	NatsURL            string
	Port               string
	SkipEtagValidation bool
}

func parseEnv() environment {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = "nats://localhost:4222"
	}
	skipEtagValidation := false
	skipEtagValidationStr := os.Getenv("SKIP_ETAG_VALIDATION")
	if skipEtagValidationStr == "true" {
		skipEtagValidation = true
	}

	return environment{
		NatsURL:            natsURL,
		Port:               port,
		SkipEtagValidation: skipEtagValidation,
	}
}

func setupHTTPServer(flags flags, svc *MeetingsAPI, gracefulCloseWG *sync.WaitGroup) *http.Server {
	// Wrap it in the generated endpoints
	endpoints := genquerysvc.NewEndpoints(svc)

	// Build an HTTP handler
	mux := goahttp.NewMuxer()
	requestDecoder := goahttp.RequestDecoder
	responseEncoder := goahttp.ResponseEncoder

	// Create a custom encoder that sets ETag header for get-one-meeting
	customEncoder := func(ctx context.Context, w http.ResponseWriter) goahttp.Encoder {
		encoder := responseEncoder(ctx, w)

		// Check if we have an ETag in the context
		if etag, ok := ctx.Value(constants.ETagContextID).(string); ok {
			w.Header().Set("ETag", etag)
		}

		return encoder
	}

	genHttpServer := genhttp.New(
		endpoints,
		mux,
		requestDecoder,
		customEncoder,
		nil,
		nil,
		nil)

	// Mount the handler on the mux
	genhttp.Mount(mux, genHttpServer)

	var handler http.Handler = mux

	// Add HTTP middleware
	// Note: Order matters - RequestIDMiddleware should come first in the chain,
	// so it should be the last middleware added to the handler since it is executed in reverse order.
	handler = middleware.WebhookBodyCaptureMiddleware()(handler)
	handler = middleware.RequestLoggerMiddleware()(handler)
	handler = middleware.RequestIDMiddleware()(handler)
	handler = middleware.AuthorizationMiddleware()(handler)

	// Set up http listener in a goroutine using provided command line parameters.
	var addr string
	if flags.Bind == "*" {
		addr = ":" + flags.Port
	} else {
		addr = flags.Bind + ":" + flags.Port
	}
	httpServer := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 3 * time.Second,
	}
	gracefulCloseWG.Add(1)
	go func() {
		slog.With("addr", addr).Debug("starting http server, listening on port " + flags.Port)
		err := httpServer.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			slog.With(errKey, err).Error("http listener error")
			os.Exit(1)
		}
		// Because ErrServerClosed is *immediately* returned when Shutdown is
		// called, not when when Shutdown completes, this must not yet decrement
		// the wait group.
	}()

	return httpServer
}

func setupNATS(ctx context.Context, env environment, svc *MeetingsAPI, gracefulCloseWG *sync.WaitGroup, done chan os.Signal) (*nats.Conn, error) {
	// Create NATS connection.
	gracefulCloseWG.Add(1)
	var err error
	slog.With("nats_url", env.NatsURL).Info("attempting to connect to NATS")
	natsConn, err := nats.Connect(
		env.NatsURL,
		nats.DrainTimeout(gracefulShutdownSeconds*time.Second),
		nats.ConnectHandler(func(_ *nats.Conn) {
			slog.With("nats_url", env.NatsURL).Info("NATS connection established")
		}),
		nats.ErrorHandler(func(_ *nats.Conn, s *nats.Subscription, err error) {
			if s != nil {
				slog.With(errKey, err, "subject", s.Subject, "queue", s.Queue).Error("async NATS error")
			} else {
				slog.With(errKey, err).Error("async NATS error outside subscription")
			}
		}),
		nats.ClosedHandler(func(_ *nats.Conn) {
			if ctx.Err() != nil {
				// If our parent background context has already been canceled, this is
				// a graceful shutdown. Decrement the wait group but do not exit, to
				// allow other graceful shutdown steps to complete.
				slog.With("nats_url", env.NatsURL).Info("NATS connection closed gracefully")
				gracefulCloseWG.Done()
				return
			}
			// Otherwise, this handler means that max reconnect attempts have been
			// exhausted.
			slog.With("nats_url", env.NatsURL).Error("NATS max-reconnects exhausted; connection closed")
			// Send a synthetic interrupt and give any graceful-shutdown tasks 5
			// seconds to clean up.
			done <- os.Interrupt
			time.Sleep(5 * time.Second)
			// Exit with an error instead of decrementing the wait group.
			os.Exit(1)
		}),
	)
	if err != nil {
		slog.With("nats_url", env.NatsURL, errKey, err).Error("error creating NATS client")
		return nil, err
	}

	// Get the key-value stores for the service.
	repos, err := getKeyValueStores(ctx, natsConn)
	if err != nil {
		return natsConn, err
	}
	svc.service.MeetingRepository = repos.Meeting
	svc.service.RegistrantRepository = repos.Registrant
	svc.service.PastMeetingRepository = repos.PastMeeting
	svc.service.PastMeetingParticipantRepository = repos.PastMeetingParticipant

	svc.service.MessageBuilder = &messaging.MessageBuilder{
		NatsConn: natsConn,
	}

	// Create NATS subscriptions for the service.
	err = createNatsSubcriptions(ctx, svc, natsConn)
	if err != nil {
		return natsConn, err
	}

	return natsConn, nil
}

// getKeyValueStores creates a JetStream client and gets separate repositories for meetings and registrants.
// Repositories holds all the repository instances for the service
type Repositories struct {
	Meeting                *store.NatsMeetingRepository
	Registrant             *store.NatsRegistrantRepository
	PastMeeting            *store.NatsPastMeetingRepository
	PastMeetingParticipant *store.NatsPastMeetingParticipantRepository
}

func getKeyValueStores(ctx context.Context, natsConn *nats.Conn) (*Repositories, error) {
	js, err := jetstream.New(natsConn)
	if err != nil {
		slog.ErrorContext(ctx, "error creating NATS JetStream client", "nats_url", natsConn.ConnectedUrl(), errKey, err)
		return nil, err
	}

	meetingsKV, err := js.KeyValue(ctx, store.KVStoreNameMeetings)
	if err != nil {
		slog.ErrorContext(ctx, "error getting NATS JetStream key-value store", "nats_url", natsConn.ConnectedUrl(), errKey, err, "store", store.KVStoreNameMeetings)
		return nil, err
	}

	meetingSettingsKV, err := js.KeyValue(ctx, store.KVStoreNameMeetingSettings)
	if err != nil {
		slog.ErrorContext(ctx, "error getting NATS JetStream key-value store", "nats_url", natsConn.ConnectedUrl(), errKey, err, "store", store.KVStoreNameMeetingSettings)
		return nil, err
	}

	meetingRegistrantsKV, err := js.KeyValue(ctx, store.KVStoreNameMeetingRegistrants)
	if err != nil {
		slog.ErrorContext(ctx, "error getting NATS JetStream key-value store", "nats_url", natsConn.ConnectedUrl(), errKey, err, "store", store.KVStoreNameMeetingRegistrants)
		return nil, err
	}

	pastMeetingsKV, err := js.KeyValue(ctx, store.KVStoreNamePastMeetings)
	if err != nil {
		slog.ErrorContext(ctx, "error getting NATS JetStream key-value store", "nats_url", natsConn.ConnectedUrl(), errKey, err, "store", store.KVStoreNamePastMeetings)
		return nil, err
	}

	pastMeetingParticipantsKV, err := js.KeyValue(ctx, store.KVStoreNamePastMeetingParticipants)
	if err != nil {
		slog.ErrorContext(ctx, "error getting NATS JetStream key-value store", "nats_url", natsConn.ConnectedUrl(), errKey, err, "store", store.KVStoreNamePastMeetingParticipants)
		return nil, err
	}

	repos := &Repositories{
		Meeting:                store.NewNatsMeetingRepository(meetingsKV, meetingSettingsKV),
		Registrant:             store.NewNatsRegistrantRepository(meetingRegistrantsKV),
		PastMeeting:            store.NewNatsPastMeetingRepository(pastMeetingsKV),
		PastMeetingParticipant: store.NewNatsPastMeetingParticipantRepository(pastMeetingParticipantsKV),
	}

	return repos, nil
}

// createNatsSubcriptions creates the NATS subscriptions for the meeting service.
func createNatsSubcriptions(ctx context.Context, svc *MeetingsAPI, natsConn *nats.Conn) error {
	subjects := []string{
		models.MeetingGetTitleSubject,
		models.ZoomWebhookMeetingStartedSubject,
		models.ZoomWebhookMeetingEndedSubject,
		models.ZoomWebhookMeetingDeletedSubject,
		models.ZoomWebhookMeetingParticipantJoinedSubject,
		models.ZoomWebhookMeetingParticipantLeftSubject,
		models.ZoomWebhookRecordingCompletedSubject,
		models.ZoomWebhookRecordingTranscriptCompletedSubject,
		models.ZoomWebhookMeetingSummaryCompletedSubject,
	}

	slog.InfoContext(ctx, "subscribing to NATS subjects", "nats_url", natsConn.ConnectedUrl(), "servers", natsConn.Servers(), "subjects", subjects)
	queueName := models.MeetingsAPIQueue

	// Subscribe to all subjects using the same handler pattern
	for _, subject := range subjects {
		_, err := natsConn.QueueSubscribe(subject, queueName, func(msg *nats.Msg) {
			natsMsg := &messaging.NatsMsg{Msg: msg}
			svc.service.HandleMessage(ctx, natsMsg)
		})
		if err != nil {
			slog.ErrorContext(ctx, "error creating NATS queue subscription", errKey, err, "subject", subject)
			return err
		}
		slog.DebugContext(ctx, "subscribed to NATS subject", "subject", subject)
	}

	return nil
}

func gracefulShutdown(httpServer *http.Server, natsConn *nats.Conn, gracefulCloseWG *sync.WaitGroup, cancel context.CancelFunc) {
	// Cancel the background context.
	cancel()

	go func() {
		// Run the HTTP shutdown in a goroutine so the NATS draining can also start.
		ctx, cancel := context.WithTimeout(context.Background(), gracefulShutdownSeconds*time.Second)
		defer cancel()

		slog.With("addr", httpServer.Addr).Info("shutting down http server")
		if err := httpServer.Shutdown(ctx); err != nil {
			slog.With(errKey, err).Error("http shutdown error")
		}
		// Decrement the wait group.
		gracefulCloseWG.Done()
	}()

	// Drain the NATS connection, which will drain all subscriptions, then close the
	// connection when complete.
	if !natsConn.IsClosed() && !natsConn.IsDraining() {
		slog.Info("draining NATS connections")
		if err := natsConn.Drain(); err != nil {
			slog.With(errKey, err).Error("error draining NATS connection")
			// Skip waiting or checking error channel.
			return
		}
	}

	// Wait for the HTTP graceful shutdown and for the NATS connection to be
	// closed (see nats.Connect options for the timeout and the handler that
	// decrements the wait group).
	gracefulCloseWG.Wait()
}
