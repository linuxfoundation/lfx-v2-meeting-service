// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package middleware

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestLoggerMiddleware(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		path           string
		expectedStatus int
		userAgent      string
		remoteAddr     string
	}{
		{
			name:           "logs GET request correctly",
			method:         "GET",
			path:           "/meetings",
			expectedStatus: http.StatusOK,
			userAgent:      "test-agent",
			remoteAddr:     "127.0.0.1:12345",
		},
		{
			name:           "logs POST request correctly",
			method:         "POST",
			path:           "/meetings",
			expectedStatus: http.StatusCreated,
			userAgent:      "curl/7.68.0",
			remoteAddr:     "192.168.1.1:54321",
		},
		{
			name:           "logs error status correctly",
			method:         "GET",
			path:           "/nonexistent",
			expectedStatus: http.StatusNotFound,
			userAgent:      "Mozilla/5.0",
			remoteAddr:     "10.0.0.1:8080",
		},
	}

	assertion := assert.New(t)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create a test handler that returns the expected status
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Simulate some processing time
				time.Sleep(10 * time.Millisecond)
				w.WriteHeader(tc.expectedStatus)
				_, err := w.Write([]byte("test response"))
				assertion.NoError(err)
			})

			// Wrap with RequestLoggerMiddleware
			middleware := RequestLoggerMiddleware()
			wrappedHandler := middleware(handler)

			// Create request
			req := httptest.NewRequest(tc.method, tc.path, nil)
			req.Header.Set("User-Agent", tc.userAgent)
			req.RemoteAddr = tc.remoteAddr

			rec := httptest.NewRecorder()

			// Record start time
			start := time.Now()
			wrappedHandler.ServeHTTP(rec, req)
			duration := time.Since(start)

			// Verify response
			assertion.Equal(tc.expectedStatus, rec.Code)
			assertion.Equal("test response", rec.Body.String())

			// Verify duration is reasonable (should be at least 10ms due to sleep)
			assertion.GreaterOrEqual(duration, 10*time.Millisecond)
		})
	}
}

// captureServiceLogs installs the production log handler chain (including the
// context handler that stamps AppendCtx attributes on every record) at info
// level, runs fn, and returns everything written to stdout.
func captureServiceLogs(t *testing.T, fn func()) string {
	t.Helper()
	t.Setenv("LOG_LEVEL", "info")

	prevLogger := slog.Default()
	prevStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	t.Cleanup(func() {
		os.Stdout = prevStdout
		slog.SetDefault(prevLogger)
	})

	logging.InitStructureLogConfig()
	fn()

	require.NoError(t, w.Close())
	os.Stdout = prevStdout
	out, err := io.ReadAll(r)
	require.NoError(t, err)
	return string(out)
}

func TestRequestLoggerMiddleware_RedactsQueryPII(t *testing.T) {
	const (
		email  = "alice@example.com"
		name   = "Alice Example"
		userID = "00000000-0000-0000-0000-000000000001"
	)

	out := captureServiceLogs(t, func() {
		handler := RequestLoggerMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Any other record written with the request context also
			// carries the middleware's attributes.
			slog.InfoContext(r.Context(), "handler log")
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodGet,
			"/itx/meetings/m1/join_link?v=1&use_email=true&register=true"+
				"&user_id="+userID+"&name=Alice%20Example&email=alice%40example.com", nil)
		handler.ServeHTTP(httptest.NewRecorder(), req)
	})

	require.Contains(t, out, "HTTP request")
	require.Contains(t, out, "HTTP response")
	require.Contains(t, out, "handler log")
	for _, secret := range []string{email, "alice%40example.com", name, "Alice", userID} {
		assert.NotContains(t, out, secret)
	}
	assert.Contains(t, out, "email=REDACTED")
	assert.Contains(t, out, "name=REDACTED")
	assert.Contains(t, out, "user_id=REDACTED")
	assert.Contains(t, out, "use_email=true")
	assert.Equal(t, 3, strings.Count(out, "user_id=REDACTED"), "query attribute should be stamped on every request-scoped record")
}

func TestQueryForLog(t *testing.T) {
	tests := []struct {
		name     string
		rawQuery string
		want     string
	}{
		{
			name:     "empty query",
			rawQuery: "",
			want:     "",
		},
		{
			name:     "join link identifiers are redacted, allowlisted params kept",
			rawQuery: "email=alice%40example.com&name=Alice+Example&user_id=u1&use_email=true&register=true&v=1",
			want:     "email=REDACTED&name=REDACTED&register=true&use_email=true&user_id=REDACTED&v=1",
		},
		{
			name:     "unknown params are redacted",
			rawQuery: "foo=alice%40example.com",
			want:     "foo=REDACTED",
		},
		{
			name:     "repeated values are all redacted",
			rawQuery: "email=a%40example.com&email=b%40example.com",
			want:     "email=REDACTED&email=REDACTED",
		},
		{
			name:     "unparseable query fails closed",
			rawQuery: "email=alice%zz",
			want:     "REDACTED (unparseable query)",
		},
		{
			name:     "semicolon separator fails closed",
			rawQuery: "v=1;email=alice@example.com",
			want:     "REDACTED (unparseable query)",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, queryForLog(tc.rawQuery))
		})
	}
}

func TestResponseWriterWrapper(t *testing.T) {
	assertion := assert.New(t)

	// Create a mock response writer
	rec := httptest.NewRecorder()

	// Wrap it with our responseWriter
	rw := &responseWriter{ResponseWriter: rec, statusCode: http.StatusOK}

	// Test WriteHeader
	rw.WriteHeader(http.StatusNotFound)
	assertion.Equal(http.StatusNotFound, rw.statusCode)
	assertion.Equal(http.StatusNotFound, rec.Code)

	// Test Write
	content := []byte("test content")
	n, err := rw.Write(content)
	assertion.NoError(err)
	assertion.Equal(len(content), n)
	assertion.Equal(content, rec.Body.Bytes())
}

func TestResponseWriterStatusCodeDefault(t *testing.T) {
	t.Run("fresh wrapper reports 200 before any write", func(t *testing.T) {
		rw := &responseWriter{ResponseWriter: httptest.NewRecorder()}
		if rw.StatusCode() != http.StatusOK {
			t.Errorf("expected 200, got %d", rw.StatusCode())
		}
	})

	t.Run("body-only Write without WriteHeader still reports 200", func(t *testing.T) {
		rec := httptest.NewRecorder()
		rw := &responseWriter{ResponseWriter: rec}
		_, _ = rw.Write([]byte("ok"))
		if rw.StatusCode() != http.StatusOK {
			t.Errorf("expected 200 after Write, got %d", rw.StatusCode())
		}
	})

	t.Run("StatusCode accessor returns explicit status set via WriteHeader", func(t *testing.T) {
		rw := &responseWriter{ResponseWriter: httptest.NewRecorder()}
		rw.WriteHeader(http.StatusNotFound)
		if rw.StatusCode() != http.StatusNotFound {
			t.Errorf("expected %d from StatusCode(), got %d", http.StatusNotFound, rw.StatusCode())
		}
	})
}
