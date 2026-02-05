// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package main

import (
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	goahttp "goa.design/goa/v3/http"

	genhttp "github.com/linuxfoundation/lfx-v2-meeting-service/gen/http/meeting_service/server"
	meetingsvc "github.com/linuxfoundation/lfx-v2-meeting-service/gen/meeting_service"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/middleware"
)

// setupHTTPServer configures and starts the HTTP server
func setupHTTPServer(flags flags, svc *MeetingsAPI, gracefulCloseWG *sync.WaitGroup) *http.Server {
	endpoints := meetingsvc.NewEndpoints(svc)

	mux := goahttp.NewMuxer()

	// Use default encoders and decoders (ITX proxy only needs JSON)
	requestDecoder := goahttp.RequestDecoder
	responseEncoder := goahttp.ResponseEncoder

	koDataPath := os.Getenv("KO_DATA_PATH")
	if koDataPath == "" {
		koDataPath = "../../gen/http"
	}

	koDataDir := http.Dir(koDataPath)

	genHttpServer := genhttp.New(
		endpoints,
		mux,
		requestDecoder,
		responseEncoder,
		nil, // Error handler
		nil, // Formatter
		koDataDir,
		koDataDir,
		koDataDir,
		koDataDir,
	)

	// Mount the handler on the mux
	genhttp.Mount(mux, genHttpServer)

	var handler http.Handler = mux

	// Add HTTP middleware
	// Note: Order matters - RequestIDMiddleware should come first in the chain,
	// so it should be the last middleware added to the handler since it is executed in reverse order.
	handler = middleware.RequestLoggerMiddleware()(handler)
	handler = middleware.RequestIDMiddleware()(handler)
	handler = middleware.AuthorizationMiddleware()(handler)
	// Wrap the handler with OpenTelemetry instrumentation
	handler = otelhttp.NewHandler(handler, "meeting-api")

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
			slog.With(logging.ErrKey, err).Error("http listener error")
			os.Exit(1)
		}
		// Because ErrServerClosed is *immediately* returned when Shutdown is
		// called, not when when Shutdown completes, this must not yet decrement
		// the wait group.
	}()

	return httpServer
}
