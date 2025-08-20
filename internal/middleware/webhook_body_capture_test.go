// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package middleware

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWebhookBodyCaptureMiddleware(t *testing.T) {
	tests := []struct {
		name          string
		path          string
		body          string
		expectCapture bool
	}{
		{
			name:          "captures zoom webhook request body",
			path:          "/webhooks/zoom",
			body:          `{"event": "meeting.started", "payload": {"id": "123"}}`,
			expectCapture: true,
		},
		{
			name:          "does not capture other webhook paths",
			path:          "/webhooks/teams",
			body:          `{"event": "meeting.ended"}`,
			expectCapture: false,
		},
		{
			name:          "does not capture non-webhook request body",
			path:          "/api/meetings",
			body:          `{"title": "Test Meeting"}`,
			expectCapture: false,
		},
		{
			name:          "handles empty zoom webhook body",
			path:          "/webhooks/zoom",
			body:          "",
			expectCapture: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedBody []byte
			var bodyFromContext []byte
			var contextHasBody bool

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Try to get the raw body from context
				bodyFromContext, contextHasBody = GetRawBodyFromContext(r.Context())

				// Also read the body normally to ensure it's still available
				body, err := io.ReadAll(r.Body)
				require.NoError(t, err)
				capturedBody = body

				w.WriteHeader(http.StatusOK)
			})

			// Wrap with middleware
			middleware := WebhookBodyCaptureMiddleware()
			wrappedHandler := middleware(handler)

			// Create request
			req := httptest.NewRequest("POST", tt.path, strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			// Execute request
			wrappedHandler.ServeHTTP(w, req)

			// Verify response
			assert.Equal(t, http.StatusOK, w.Code)

			// Verify body is still readable by handler
			assert.Equal(t, tt.body, string(capturedBody))

			// Verify context capture behavior
			if tt.expectCapture {
				assert.True(t, contextHasBody, "Expected body to be available in context for zoom webhook path")
				assert.Equal(t, tt.body, string(bodyFromContext), "Body in context should match expected")
			} else {
				assert.False(t, contextHasBody, "Expected body NOT to be available in context for non-zoom webhook path")
			}
		})
	}
}

func TestGetRawBodyFromContext(t *testing.T) {
	tests := []struct {
		name          string
		setupContext  func() context.Context
		expectedBody  []byte
		expectedFound bool
	}{
		{
			name: "returns body when present in context",
			setupContext: func() context.Context {
				body := []byte(`{"test": "data"}`)
				return context.WithValue(context.Background(), WebhookBodyContextKey{}, body)
			},
			expectedBody:  []byte(`{"test": "data"}`),
			expectedFound: true,
		},
		{
			name: "returns false when body not in context",
			setupContext: func() context.Context {
				return context.Background()
			},
			expectedBody:  nil,
			expectedFound: false,
		},
		{
			name: "returns false when wrong type in context",
			setupContext: func() context.Context {
				return context.WithValue(context.Background(), WebhookBodyContextKey{}, "wrong type")
			},
			expectedBody:  nil,
			expectedFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.setupContext()
			body, found := GetRawBodyFromContext(ctx)

			assert.Equal(t, tt.expectedFound, found)
			assert.Equal(t, tt.expectedBody, body)
		})
	}
}
