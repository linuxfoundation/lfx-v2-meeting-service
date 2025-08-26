// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package middleware

import (
	"bytes"
	"context"
	"io"
	"net/http"
)

// WebhookBodyContextKey is the context key for storing raw webhook body
type WebhookBodyContextKey struct{}

// WebhookBodyCaptureMiddleware captures the raw request body for webhook endpoints
// and stores it in the request context for signature validation
func WebhookBodyCaptureMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Only capture body for Zoom webhook endpoint
			if r.URL.Path == "/webhooks/zoom" {
				// Read the body
				body, err := io.ReadAll(r.Body)
				if err != nil {
					http.Error(w, "Failed to read request body", http.StatusBadRequest)
					return
				}

				// Close the original body
				_ = r.Body.Close()

				// Create a new reader with the same data for the next handler
				r.Body = io.NopCloser(bytes.NewReader(body))

				// Store the raw body in context
				ctx := context.WithValue(r.Context(), WebhookBodyContextKey{}, body)
				r = r.WithContext(ctx)
			}

			next.ServeHTTP(w, r)
		})
	}
}

// GetRawBodyFromContext extracts the raw body from the context
func GetRawBodyFromContext(ctx context.Context) ([]byte, bool) {
	body, ok := ctx.Value(WebhookBodyContextKey{}).([]byte)
	return body, ok
}
