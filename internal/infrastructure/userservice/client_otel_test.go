// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package userservice

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

func newTestTracer(t *testing.T) (trace.Tracer, *tracetest.SpanRecorder) {
	t.Helper()
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })
	return tp.Tracer("test"), sr
}

func hasExceptionEvent(spans []sdktrace.ReadOnlySpan) bool {
	for _, s := range spans {
		for _, e := range s.Events() {
			if e.Name == "exception" {
				return true
			}
		}
	}
	return false
}

func exceptionMessage(spans []sdktrace.ReadOnlySpan) string {
	for _, s := range spans {
		for _, e := range s.Events() {
			if e.Name == "exception" {
				for _, a := range e.Attributes {
					if string(a.Key) == "exception.message" {
						return a.Value.AsString()
					}
				}
			}
		}
	}
	return ""
}

func hasErrorStatus(spans []sdktrace.ReadOnlySpan) bool {
	for _, s := range spans {
		if s.Status().Code == codes.Error {
			return true
		}
	}
	return false
}

func TestDoJSON_5xx_RecordsErrorOnSpan(t *testing.T) {
	tracer, sr := newTestTracer(t)
	ctx, span := tracer.Start(context.Background(), "op")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	c := newClient(server.Client(), server.URL)
	err := c.doJSON(ctx, testToken, http.MethodGet, server.URL+"/user-service/v1/me", nil, nil)
	span.End()

	if err == nil {
		t.Fatal("expected error from 5xx response")
	}
	if !hasExceptionEvent(sr.Ended()) {
		t.Error("RecordError should have been called on the span for a 5xx response")
	}
	if !hasErrorStatus(sr.Ended()) {
		t.Error("SetStatus(codes.Error) should have been called on the span for a 5xx response")
	}
	// Verify sanitization: exception.message must be the safe "HTTP <code>" form,
	// not the raw domain error which may contain upstream PII.
	if msg := exceptionMessage(sr.Ended()); msg != "HTTP 500" {
		t.Errorf("exception.message = %q, want %q", msg, "HTTP 500")
	}
}

func TestDoJSON_4xx_DoesNotRecordErrorOnSpan(t *testing.T) {
	tracer, sr := newTestTracer(t)
	ctx, span := tracer.Start(context.Background(), "op")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	c := newClient(server.Client(), server.URL)
	err := c.doJSON(ctx, testToken, http.MethodGet, server.URL+"/user-service/v1/me", nil, nil)
	span.End()

	if err == nil {
		t.Fatal("expected error from 4xx response")
	}
	if hasExceptionEvent(sr.Ended()) {
		t.Error("RecordError must not be called on the span for a 4xx response")
	}
	if hasErrorStatus(sr.Ended()) {
		t.Error("SetStatus(codes.Error) must not be called on the span for a 4xx response")
	}
}
