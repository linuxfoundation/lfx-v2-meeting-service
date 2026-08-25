// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package itx

import (
	"context"
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

// TestRecordAndMapHTTPError_5xx_RecordsErrorOnSpan verifies the PII-safety
// mechanism: 5xx responses record a sanitised "HTTP <code>" error on the span
// rather than the raw upstream response body.
func TestRecordAndMapHTTPError_5xx_RecordsErrorOnSpan(t *testing.T) {
	tracer, sr := newTestTracer(t)
	ctx, span := tracer.Start(context.Background(), "op")

	body := []byte(`{"message":"internal error containing PII"}`)
	err := recordAndMapHTTPError(ctx, 500, body)
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
	// PII-safety: exception.message must be the safe "HTTP <code>" form,
	// never the raw upstream body text which may contain user-identifiable data.
	if msg := exceptionMessage(sr.Ended()); msg != "HTTP 500" {
		t.Errorf("exception.message = %q, want %q (upstream text must not leak into spans)", msg, "HTTP 500")
	}
}

// TestRecordAndMapHTTPError_503_RecordsErrorOnSpan verifies the 5xx branch
// with 503 Unavailable (the other common ITX-unavailability status).
func TestRecordAndMapHTTPError_503_RecordsErrorOnSpan(t *testing.T) {
	tracer, sr := newTestTracer(t)
	ctx, span := tracer.Start(context.Background(), "op")

	err := recordAndMapHTTPError(ctx, 503, []byte(`{"message":"downstream unavailable"}`))
	span.End()

	if err == nil {
		t.Fatal("expected error from 503 response")
	}
	if !hasExceptionEvent(sr.Ended()) {
		t.Error("RecordError should have been called on the span for a 503 response")
	}
	if msg := exceptionMessage(sr.Ended()); msg != "HTTP 503" {
		t.Errorf("exception.message = %q, want %q", msg, "HTTP 503")
	}
}

// TestRecordAndMapHTTPError_4xx_DoesNotRecordErrorOnSpan verifies that 4xx
// client errors do not record an error on the span (they are not server faults).
func TestRecordAndMapHTTPError_4xx_DoesNotRecordErrorOnSpan(t *testing.T) {
	tracer, sr := newTestTracer(t)
	ctx, span := tracer.Start(context.Background(), "op")

	err := recordAndMapHTTPError(ctx, 404, []byte(`{"message":"not found"}`))
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
