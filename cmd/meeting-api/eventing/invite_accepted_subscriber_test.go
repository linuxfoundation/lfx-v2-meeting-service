// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	inviteapi "github.com/linuxfoundation/lfx-v2-invite-service/pkg/api"
	natsgo "github.com/nats-io/nats.go"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestInviteAcceptedSubscriber_StopWithNoInFlightMessages(t *testing.T) {
	sub := NewInviteAcceptedSubscriber(nil, nil, slog.Default())

	done := make(chan struct{})
	go func() {
		sub.Stop()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop() blocked — possible WaitGroup misuse")
	}
}

// fakeAcceptanceClient implements domain.InviteAcceptanceClient for testing.
type fakeAcceptanceClient struct {
	err error
}

func (f *fakeAcceptanceClient) AcceptInvite(_ context.Context, _, _ string) error {
	return f.err
}

func TestInviteAcceptedSubscriber_Handle_ErrorSpanSanitized(t *testing.T) {
	// PII that must never reach the trace backend.
	piiEmail := "alice@example.com"
	piiBody := `{"message":"alice@example.com is not registered"}`

	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	prevTP := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	defer otel.SetTracerProvider(prevTP)
	defer func() { _ = tp.Shutdown(context.Background()) }()

	client := &fakeAcceptanceClient{err: errors.New(piiBody)}
	sub := NewInviteAcceptedSubscriber(nil, client, slog.Default())
	sub.ctx = context.Background() // set ctx directly; Start is not called (no NATS conn needed)

	evt := inviteapi.InviteServiceAcceptedEvent{
		Invite: inviteapi.Invite{
			Recipient:  inviteapi.Recipient{Email: piiEmail},
			AcceptedBy: "alice-lfid",
		},
	}
	data, _ := json.Marshal(evt)
	sub.handle(&natsgo.Msg{Data: data})

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 exported span, got %d", len(spans))
	}
	s := spans[0]

	// Check every event attribute and the status description for PII.
	for _, ev := range s.Events {
		for _, attr := range ev.Attributes {
			val := attr.Value.AsString()
			if strings.Contains(val, piiEmail) || strings.Contains(val, piiBody) {
				t.Errorf("PII found in span event %q attr %q=%q", ev.Name, attr.Key, val)
			}
		}
		// exception.message must not carry the upstream body.
		if ev.Name == "exception" {
			for _, attr := range ev.Attributes {
				if string(attr.Key) == "exception.message" {
					if strings.Contains(attr.Value.AsString(), piiEmail) || strings.Contains(attr.Value.AsString(), piiBody) {
						t.Errorf("PII in exception.message: %q", attr.Value.AsString())
					}
				}
			}
		}
	}

	// Span status description must not carry upstream error text.
	statusDesc := s.Status.Description
	if strings.Contains(statusDesc, piiEmail) || strings.Contains(statusDesc, piiBody) {
		t.Errorf("PII in span status description: %q", statusDesc)
	}
}
