// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package middleware

import (
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/constants"
)

// RequestLoggerMiddleware creates a middleware that logs HTTP requests.
// Health check endpoints (/livez and /readyz) are excluded from logging to reduce noise.
func RequestLoggerMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now().UTC()
			isHealthCheck := r.URL.Path == "/livez" || r.URL.Path == "/readyz"

			ctx := r.Context()
			ctx = logging.AppendCtx(ctx, slog.String("component", "api"))
			ctx = logging.AppendCtx(ctx, slog.String("method", r.Method))
			ctx = logging.AppendCtx(ctx, slog.String("path", r.URL.Path))
			ctx = logging.AppendCtx(ctx, slog.String("query", queryForLog(r.URL.RawQuery)))
			ctx = logging.AppendCtx(ctx, slog.String("host", r.Host))
			ctx = logging.AppendCtx(ctx, slog.String("user_agent", r.UserAgent()))
			ctx = logging.AppendCtx(ctx, slog.String("remote_addr", r.RemoteAddr))

			if r.Header.Get(constants.EtagHeader) != "" {
				ctx = logging.AppendCtx(ctx, slog.String("req_header_etag", r.Header.Get(constants.EtagHeader)))
			}

			r = r.WithContext(ctx)
			ww := &responseWriter{ResponseWriter: w}

			if !isHealthCheck {
				slog.InfoContext(ctx, "HTTP request")
			}

			next.ServeHTTP(ww, r)

			duration := time.Since(start)
			if !isHealthCheck {
				slog.InfoContext(ctx, "HTTP response", "status", ww.StatusCode(), "duration", duration.String())
			}
		})
	}
}

// loggableQueryParams lists the inbound query parameters whose values carry no
// personal data, each with a check its value must pass to be logged verbatim.
// Every other value, and any listed value that fails its check, is replaced,
// so identifiers such as the join-link email/name/user_id never reach logs
// unredacted.
var loggableQueryParams = map[string]func(string) bool{
	"v":         func(v string) bool { return v == "1" },
	"use_email": isBoolString,
	"register":  isBoolString,
}

// redactedQueryParams lists the known inbound query parameters that carry
// personal data. Their names are logged but their values never are. Any
// parameter in neither list has its name redacted too, since the name is as
// request-controlled as the value.
var redactedQueryParams = map[string]bool{
	"email":   true,
	"name":    true,
	"user_id": true,
}

func isBoolString(v string) bool {
	_, err := strconv.ParseBool(v)
	return err == nil
}

// queryForLog returns rawQuery with every parameter value not accepted by
// loggableQueryParams replaced with "REDACTED", and the name of every
// parameter not listed in loggableQueryParams or redactedQueryParams replaced
// with "REDACTED", for safe use in log output.
func queryForLog(rawQuery string) string {
	if rawQuery == "" {
		return ""
	}
	q, err := url.ParseQuery(rawQuery)
	if err != nil {
		// Fail closed: a query that doesn't parse can't be checked for
		// sensitive values, and rawQuery may still carry them.
		return "REDACTED (unparseable query)"
	}
	out := make(url.Values, len(q))
	for k, vs := range q {
		loggable := loggableQueryParams[k]
		if loggable == nil && !redactedQueryParams[k] {
			k = "REDACTED"
		}
		for _, v := range vs {
			if loggable == nil || !loggable(v) {
				v = "REDACTED"
			}
			out.Add(k, v)
		}
	}
	return out.Encode()
}

// responseWriter wraps http.ResponseWriter to capture status code.
// statusCode defaults to 200 because net/http implicitly writes 200 when
// Write is called without an explicit WriteHeader call.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// StatusCode returns the captured status code, defaulting to 200 if WriteHeader
// was never called (which net/http treats as an implicit 200 OK).
func (rw *responseWriter) StatusCode() int {
	if rw.statusCode == 0 {
		return http.StatusOK
	}
	return rw.statusCode
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	return rw.ResponseWriter.Write(b)
}
