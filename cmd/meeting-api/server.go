// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
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

	requestDecoder := goahttp.RequestDecoder
	responseEncoder := createResponseEncoder()

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

	// Register route-tagging middleware inside chi's routing chain so that
	// http.route is set on the OTel span after chi has matched the route pattern.
	// The span name is also updated here to avoid high-cardinality names from
	// using raw URL paths (which contain actual path parameter values).
	// Must be registered before Mount calls per chi convention.
	// Reads RoutePattern after next.ServeHTTP because chi populates the pattern
	// during routing (inside ServeHTTP), not before.
	mux.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				rctx := chi.RouteContext(r.Context())
				if rctx != nil {
					routePattern := rctx.RoutePattern()
					if routePattern != "" {
						if labeler, ok := otelhttp.LabelerFromContext(r.Context()); ok {
							labeler.Add(semconv.HTTPRoute(routePattern))
						}
						trace.SpanFromContext(r.Context()).SetName(r.Method + " " + routePattern)
					}
				}
			}()
			next.ServeHTTP(w, r)
		})
	})

	genhttp.Mount(mux, genHttpServer)

	var handler http.Handler = mux

	// Middleware is executed in reverse order; RequestIDMiddleware runs first.
	handler = middleware.RequestLoggerMiddleware()(handler)
	handler = middleware.RequestIDMiddleware()(handler)
	handler = middleware.AuthorizationMiddleware()(handler)
	handler = otelhttp.NewHandler(handler, "meeting-api",
		otelhttp.WithFilter(func(r *http.Request) bool {
			p := r.URL.Path
			return p != genhttp.LivezMeetingServicePath() && p != genhttp.ReadyzMeetingServicePath()
		}),
	)

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

// createResponseEncoder creates a custom response encoder that handles raw bytes for ICS endpoints
func createResponseEncoder() func(context.Context, http.ResponseWriter) goahttp.Encoder {
	return func(ctx context.Context, w http.ResponseWriter) goahttp.Encoder {
		contentType, _ := ctx.Value(goahttp.ContentTypeKey).(string)

		// For text/calendar content type, write raw bytes directly
		if contentType == "text/calendar" {
			return &rawBytesEncoder{w: w}
		}

		// For other content types, use default JSON encoder
		return goahttp.ResponseEncoder(ctx, w)
	}
}

// rawBytesEncoder writes raw bytes directly to the response writer without encoding
type rawBytesEncoder struct {
	w http.ResponseWriter
}

// Encode writes raw bytes directly to the response
func (e *rawBytesEncoder) Encode(v any) error {
	if bytes, ok := v.([]byte); ok {
		_, err := e.w.Write(bytes)
		return err
	}
	// Fallback for non-bytes (shouldn't happen for ICS endpoint)
	return json.NewEncoder(e.w).Encode(v)
}
