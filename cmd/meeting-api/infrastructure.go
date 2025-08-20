// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	nats "github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/infrastructure/auth"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/infrastructure/email"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/infrastructure/messaging"
	store "github.com/linuxfoundation/lfx-v2-meeting-service/internal/infrastructure/store"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
)

const (
	// gracefulShutdownSeconds should be higher than NATS client
	// request timeout, and lower than the pod or liveness probe's
	// terminationGracePeriodSeconds.
	gracefulShutdownSeconds = 25
)

// setupJWTAuth configures JWT authentication for the service
func setupJWTAuth() (*auth.JWTAuth, error) {
	jwtAuthConfig := auth.JWTAuthConfig{
		JWKSURL:            os.Getenv("JWKS_URL"),
		Audience:           os.Getenv("JWT_AUDIENCE"),
		MockLocalPrincipal: os.Getenv("JWT_AUTH_DISABLED_MOCK_LOCAL_PRINCIPAL"),
	}
	return auth.NewJWTAuth(jwtAuthConfig)
}

// setupEmailService initializes the email service based on configuration
func setupEmailService(env environment, svc *MeetingsAPI) error {
	if env.EmailConfig.Enabled {
		smtpConfig := email.SMTPConfig{
			Host:     env.EmailConfig.SMTPHost,
			Port:     env.EmailConfig.SMTPPort,
			From:     env.EmailConfig.SMTPFrom,
			Username: env.EmailConfig.SMTPUsername,
			Password: env.EmailConfig.SMTPPassword,
		}
		emailService, err := email.NewSMTPService(smtpConfig)
		if err != nil {
			slog.With(logging.ErrKey, err).Error("error creating email service")
			return err
		}
		svc.service.EmailService = emailService
		slog.With("smtp_host", env.EmailConfig.SMTPHost, "smtp_port", env.EmailConfig.SMTPPort).Info("email service enabled")
	} else {
		svc.service.EmailService = email.NewNoOpService()
		slog.Info("email service disabled")
	}
	return nil
}

// setupNATS configures NATS connection and related infrastructure
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
				slog.With(logging.ErrKey, err, "subject", s.Subject, "queue", s.Queue).Error("async NATS error")
			} else {
				slog.With(logging.ErrKey, err).Error("async NATS error outside subscription")
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
			// Send a synthetic interrupt without blocking and give any graceful-shutdown tasks 5 seconds to clean up.
			select {
			case done <- os.Interrupt:
			default:
				slog.Warn("shutdown signal channel is not ready; skipping synthetic interrupt")
			}
			time.Sleep(5 * time.Second)
			// Exit with an error instead of decrementing the wait group.
			os.Exit(1)
		}),
	)
	if err != nil {
		slog.With("nats_url", env.NatsURL, logging.ErrKey, err).Error("error creating NATS client")
		return nil, err
	}

	// Get the key-value stores for the service.
	meetingRepo, registrantRepo, err := getKeyValueStores(ctx, natsConn)
	if err != nil {
		return natsConn, err
	}
	svc.service.MeetingRepository = meetingRepo
	svc.service.RegistrantRepository = registrantRepo

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
func getKeyValueStores(ctx context.Context, natsConn *nats.Conn) (*store.NatsMeetingRepository, *store.NatsRegistrantRepository, error) {
	js, err := jetstream.New(natsConn)
	if err != nil {
		slog.ErrorContext(ctx, "error creating NATS JetStream client", "nats_url", natsConn.ConnectedUrl(), logging.ErrKey, err)
		return nil, nil, err
	}

	meetingsKV, err := js.KeyValue(ctx, store.KVStoreNameMeetings)
	if err != nil {
		slog.ErrorContext(ctx, "error getting NATS JetStream key-value store", "nats_url", natsConn.ConnectedUrl(), logging.ErrKey, err, "store", store.KVStoreNameMeetings)
		return nil, nil, err
	}

	meetingSettingsKV, err := js.KeyValue(ctx, store.KVStoreNameMeetingSettings)
	if err != nil {
		slog.ErrorContext(ctx, "error getting NATS JetStream key-value store", "nats_url", natsConn.ConnectedUrl(), logging.ErrKey, err, "store", store.KVStoreNameMeetingSettings)
		return nil, nil, err
	}

	meetingRegistrantsKV, err := js.KeyValue(ctx, store.KVStoreNameMeetingRegistrants)
	if err != nil {
		slog.ErrorContext(ctx, "error getting NATS JetStream key-value store", "nats_url", natsConn.ConnectedUrl(), logging.ErrKey, err, "store", store.KVStoreNameMeetingRegistrants)
		return nil, nil, err
	}

	meetingRepo := store.NewNatsMeetingRepository(meetingsKV, meetingSettingsKV)
	registrantRepo := store.NewNatsRegistrantRepository(meetingRegistrantsKV)

	return meetingRepo, registrantRepo, nil
}

// createNatsSubcriptions creates the NATS subscriptions for the meeting service.
func createNatsSubcriptions(ctx context.Context, svc *MeetingsAPI, natsConn *nats.Conn) error {
	subjects := []string{
		// Get meeting title subscription
		models.MeetingGetTitleSubject,
		// Meeting deletion cleanup subscription
		models.MeetingDeletedSubject,
	}

	slog.InfoContext(ctx, "subscribing to NATS subjects", "nats_url", natsConn.ConnectedUrl(), "servers", natsConn.Servers(), "subjects", subjects)
	queueName := models.MeetingsAPIQueue

	for _, subject := range subjects {
		_, err := natsConn.QueueSubscribe(subject, queueName, func(msg *nats.Msg) {
			natsMsg := &messaging.NatsMsg{Msg: msg}
			svc.service.HandleMessage(ctx, natsMsg)
		})
		if err != nil {
			slog.ErrorContext(ctx, "error creating NATS queue subscription", logging.ErrKey, err, "subject", subject)
			return err
		}
		slog.With("subject", subject, "queue", queueName).Debug("successfully subscribed to NATS subject")
	}

	return nil
}

// gracefulShutdown handles graceful shutdown of the application
func gracefulShutdown(httpServer *http.Server, natsConn *nats.Conn, gracefulCloseWG *sync.WaitGroup, cancel context.CancelFunc) {
	// Cancel the background context.
	cancel()

	go func() {
		// Run the HTTP shutdown in a goroutine so the NATS draining can also start.
		ctx, cancel := context.WithTimeout(context.Background(), gracefulShutdownSeconds*time.Second)
		defer cancel()

		slog.With("addr", httpServer.Addr).Info("shutting down http server")
		if err := httpServer.Shutdown(ctx); err != nil {
			slog.With(logging.ErrKey, err).Error("http shutdown error")
		}
		// Decrement the wait group.
		gracefulCloseWG.Done()
	}()

	// Drain the NATS connection, which will drain all subscriptions, then close the
	// connection when complete.
	if !natsConn.IsClosed() && !natsConn.IsDraining() {
		slog.Info("draining NATS connections")
		if err := natsConn.Drain(); err != nil {
			slog.With(logging.ErrKey, err).Error("error draining NATS connection")
			// Skip waiting or checking error channel.
			return
		}
	}

	// Wait for the HTTP graceful shutdown and for the NATS connection to be
	// closed (see nats.Connect options for the timeout and the handler that
	// decrements the wait group).
	gracefulCloseWG.Wait()
}
