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

	// Generated service initialization.
	service := service.NewMeetingsService(jwtAuth, service.ServiceConfig{
		SkipEtagValidation: env.SkipEtagValidation,
	})
	svc := NewMeetingsAPI(service)

	gracefulCloseWG := sync.WaitGroup{}

	httpServer := setupHTTPServer(flags, svc, &gracefulCloseWG)

	// Initialize email service (independent of NATS)
	err = setupEmailService(env, svc)
	if err != nil {
		slog.With(logging.ErrKey, err).Error("error setting up email service")
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	natsConn, err := setupNATS(ctx, env, svc, &gracefulCloseWG, done)
	if err != nil {
		slog.With(logging.ErrKey, err).Error("error setting up NATS")
		return
	}

	// This next line blocks until SIGINT or SIGTERM is received.
	<-done

	gracefulShutdown(httpServer, natsConn, &gracefulCloseWG, cancel)
}
