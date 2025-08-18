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

	goahttp "goa.design/goa/v3/http"

	genhttp "github.com/linuxfoundation/lfx-v2-meeting-service/gen/http/meeting_service/server"
	genquerysvc "github.com/linuxfoundation/lfx-v2-meeting-service/gen/meeting_service"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/middleware"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/constants"
)

// setupHTTPServer configures and starts the HTTP server
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
			slog.With(logging.ErrKey, err).Error("http listener error")
			os.Exit(1)
		}
		// Because ErrServerClosed is *immediately* returned when Shutdown is
		// called, not when when Shutdown completes, this must not yet decrement
		// the wait group.
	}()

	return httpServer
}