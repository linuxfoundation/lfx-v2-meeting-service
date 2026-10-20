// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureLogs routes slog's default logger into a buffer for the test.
func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

// TestStartPreferredEmailResponder_DisabledDoesNotDial pins the switch: with
// NATS_RESPONDERS_ENABLED=false the responder is not started and nothing is
// dialled, even with a NATS URL set. The URL points at a closed port, so a dial
// would fail at once and leave the connection-failure warning in the log.
func TestStartPreferredEmailResponder_DisabledDoesNotDial(t *testing.T) {
	logs := captureLogs(t)
	env := environment{NATSRespondersEnabled: false, UserServiceConfig: userServiceConfig{BaseURL: "https://api-gw.dev.example.com"}}

	responder, nc := startPreferredEmailResponder(context.Background(), env, "nats://127.0.0.1:1")

	assert.Nil(t, responder)
	assert.Nil(t, nc)
	out := logs.String()
	assert.Contains(t, out, "NATS_RESPONDERS_ENABLED=false; preferred_email RPC responder will not start")
	assert.NotContains(t, out, "failed to connect to NATS")
	assert.NotContains(t, out, "preferred_email RPC responder initialized")
}

// TestStartPreferredEmailResponder_EnabledDials pins the complement, so the
// test above cannot pass by the responder never dialling: enabled, the same
// closed-port URL is dialled and the connection failure is logged.
func TestStartPreferredEmailResponder_EnabledDials(t *testing.T) {
	logs := captureLogs(t)
	env := environment{NATSRespondersEnabled: true, UserServiceConfig: userServiceConfig{BaseURL: "https://api-gw.dev.example.com"}}

	responder, nc := startPreferredEmailResponder(context.Background(), env, "nats://127.0.0.1:1")

	require.Nil(t, responder)
	require.Nil(t, nc)
	assert.Contains(t, logs.String(), "failed to connect to NATS for preferred_email responder")
}
