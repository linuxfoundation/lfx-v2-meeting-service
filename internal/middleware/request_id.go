// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// Package middleware contains the middleware to be used by services
package middleware

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/constants"

	"github.com/google/uuid"
)

// RequestIDMiddleware creates a middleware that adds a request ID to the context
func RequestIDMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := r.Header.Get(constants.RequestIDHeader)
			if requestID == "" {
				requestID = generateRequestID()
			}
			w.Header().Set(constants.RequestIDHeader, requestID)
			ctx := context.WithValue(r.Context(), constants.RequestIDContextID, requestID)
			// Append to context so the request ID is included in all logs for this request
			ctx = logging.AppendCtx(ctx, slog.String(constants.RequestIDHeader, requestID))
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// generateRequestID generates a new unique request ID
func generateRequestID() string {
	return uuid.New().String()
}
