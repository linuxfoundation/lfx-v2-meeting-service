// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	infraeventing "github.com/linuxfoundation/lfx-v2-meeting-service/internal/infrastructure/eventing"
)

// fakeJetStreamMsg records how a message was settled so tests can assert on it.
type fakeJetStreamMsg struct {
	subject     string
	numDelivery uint64
	metadataErr error

	acked       bool
	naked       bool
	nakDelay    time.Duration
	terminated  bool
	nakErr      error
	ackErr      error
	terminateEr error
}

func (m *fakeJetStreamMsg) Metadata() (*jetstream.MsgMetadata, error) {
	if m.metadataErr != nil {
		return nil, m.metadataErr
	}
	return &jetstream.MsgMetadata{NumDelivered: m.numDelivery}, nil
}
func (m *fakeJetStreamMsg) Data() []byte         { return nil }
func (m *fakeJetStreamMsg) Headers() nats.Header { return nil }
func (m *fakeJetStreamMsg) Subject() string      { return m.subject }
func (m *fakeJetStreamMsg) Reply() string        { return "" }
func (m *fakeJetStreamMsg) Ack() error {
	m.acked = true
	return m.ackErr
}
func (m *fakeJetStreamMsg) DoubleAck(context.Context) error { return nil }
func (m *fakeJetStreamMsg) Nak() error {
	m.naked = true
	return m.nakErr
}
func (m *fakeJetStreamMsg) NakWithDelay(delay time.Duration) error {
	m.naked = true
	m.nakDelay = delay
	return m.nakErr
}
func (m *fakeJetStreamMsg) InProgress() error { return nil }
func (m *fakeJetStreamMsg) Term() error {
	m.terminated = true
	return m.terminateEr
}
func (m *fakeJetStreamMsg) TermWithReason(string) error {
	m.terminated = true
	return m.terminateEr
}

func TestGetRetryDelay(t *testing.T) {
	tests := []struct {
		name         string
		numDelivered uint64
		want         time.Duration
	}{
		{name: "unknown delivery count uses the default", numDelivered: 0, want: unknownAttemptRetryDelay},
		{name: "first delivery", numDelivered: 1, want: 2 * time.Second},
		{name: "second delivery", numDelivered: 2, want: 10 * time.Second},
		{name: "third delivery", numDelivered: 3, want: 30 * time.Second},
		{name: "fourth delivery", numDelivered: 4, want: time.Minute},
		{name: "fifth delivery", numDelivered: 5, want: 5 * time.Minute},
		{name: "sixth delivery", numDelivered: 6, want: 10 * time.Minute},
		{name: "seventh delivery", numDelivered: 7, want: 15 * time.Minute},
		{name: "beyond the schedule reuses the last delay", numDelivered: 50, want: 15 * time.Minute},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, getRetryDelay(tt.numDelivered))
		})
	}
}

func TestRetryDelaysSpanAnOutage(t *testing.T) {
	// With the default MaxDeliver of 8 a message is NAKed after deliveries 1 to 7.
	// The schedule must keep it retrying long enough to outlast a dependency outage.
	var total time.Duration
	for attempt := uint64(1); attempt < 8; attempt++ {
		total += getRetryDelay(attempt)
	}
	assert.GreaterOrEqual(t, total, 30*time.Minute)
}

func TestSettleMessage(t *testing.T) {
	ctx := context.Background()

	t.Run("acks when no retry is requested", func(t *testing.T) {
		var logs bytes.Buffer
		msg := &fakeJetStreamMsg{subject: "$KV.v1-objects.itx-zoom-past-meetings.abc", numDelivery: 1}

		settleMessage(ctx, slog.New(slog.NewTextHandler(&logs, nil)), msg, false, 8)

		assert.True(t, msg.acked)
		assert.False(t, msg.naked)
		assert.False(t, msg.terminated)
		assert.Empty(t, logs.String())
	})

	t.Run("naks with the scheduled delay when a retry remains", func(t *testing.T) {
		var logs bytes.Buffer
		msg := &fakeJetStreamMsg{subject: "$KV.v1-objects.itx-zoom-past-meetings.abc", numDelivery: 3}

		settleMessage(ctx, slog.New(slog.NewTextHandler(&logs, nil)), msg, true, 8)

		assert.True(t, msg.naked)
		assert.Equal(t, 30*time.Second, msg.nakDelay)
		assert.False(t, msg.acked)
		assert.False(t, msg.terminated)
		assert.NotContains(t, logs.String(), "dropped after max deliveries")
	})

	t.Run("still naks on the delivery before the last", func(t *testing.T) {
		msg := &fakeJetStreamMsg{numDelivery: 7}

		settleMessage(ctx, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)), msg, true, 8)

		assert.True(t, msg.naked)
		assert.Equal(t, 15*time.Minute, msg.nakDelay)
		assert.False(t, msg.terminated)
	})

	t.Run("logs an error and terminates on the final delivery", func(t *testing.T) {
		var logs bytes.Buffer
		msg := &fakeJetStreamMsg{subject: "$KV.v1-objects.itx-zoom-past-meetings.abc", numDelivery: 8}

		settleMessage(ctx, slog.New(slog.NewTextHandler(&logs, nil)), msg, true, 8)

		assert.True(t, msg.terminated)
		assert.False(t, msg.naked)
		assert.False(t, msg.acked)
		out := logs.String()
		assert.Contains(t, out, "level=ERROR")
		assert.Contains(t, out, "event dropped after max deliveries")
		assert.Contains(t, out, "$KV.v1-objects.itx-zoom-past-meetings.abc")
		assert.Contains(t, out, "num_delivered=8")
		assert.Contains(t, out, "max_deliver=8")
		assert.NotContains(t, out, "NAKed for retry")
	})

	t.Run("a delivery count past the limit is also dropped", func(t *testing.T) {
		msg := &fakeJetStreamMsg{numDelivery: 9}

		settleMessage(ctx, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)), msg, true, 8)

		assert.True(t, msg.terminated)
		assert.False(t, msg.naked)
	})

	t.Run("never drops when deliveries are unlimited", func(t *testing.T) {
		for _, maxDeliver := range []int{0, -1} {
			msg := &fakeJetStreamMsg{numDelivery: 1000}

			settleMessage(ctx, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)), msg, true, maxDeliver)

			assert.True(t, msg.naked, "maxDeliver=%d", maxDeliver)
			assert.False(t, msg.terminated, "maxDeliver=%d", maxDeliver)
		}
	})

	t.Run("falls back to the default delay when metadata is unavailable", func(t *testing.T) {
		var logs bytes.Buffer
		msg := &fakeJetStreamMsg{metadataErr: errors.New("no metadata")}

		settleMessage(ctx, slog.New(slog.NewTextHandler(&logs, nil)), msg, true, 8)

		assert.True(t, msg.naked)
		assert.Equal(t, unknownAttemptRetryDelay, msg.nakDelay)
		assert.False(t, msg.terminated)
		assert.Contains(t, logs.String(), "failed to get message metadata")
	})

	t.Run("logs but does not panic when settling fails", func(t *testing.T) {
		var logs bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&logs, nil))

		ackFail := &fakeJetStreamMsg{ackErr: errors.New("ack failed")}
		require.NotPanics(t, func() { settleMessage(ctx, logger, ackFail, false, 8) })

		nakFail := &fakeJetStreamMsg{numDelivery: 1, nakErr: errors.New("nak failed")}
		require.NotPanics(t, func() { settleMessage(ctx, logger, nakFail, true, 8) })

		termFail := &fakeJetStreamMsg{numDelivery: 8, terminateEr: errors.New("term failed")}
		require.NotPanics(t, func() { settleMessage(ctx, logger, termFail, true, 8) })

		out := logs.String()
		assert.Contains(t, out, "failed to ACK message")
		assert.Contains(t, out, "failed to NAK message")
		assert.Contains(t, out, "failed to terminate message")
	})
}

func TestMsgHandlerPanic(t *testing.T) {
	ctx := context.Background()

	// A nil handlers value makes kvHandler dereference nil, which is the panic
	// the handler's recover block exists to contain.
	newProcessor := func(logs *bytes.Buffer) *EventProcessor {
		return &EventProcessor{
			logger: slog.New(slog.NewTextHandler(logs, nil)),
			config: infraeventing.Config{MaxDeliver: 8},
		}
	}

	t.Run("a panic is retried with the scheduled delay", func(t *testing.T) {
		var logs bytes.Buffer
		msg := &fakeJetStreamMsg{subject: "$KV.v1-objects.itx-zoom-past-meetings.abc", numDelivery: 2}

		require.NotPanics(t, func() { newProcessor(&logs).msgHandler(ctx)(msg) })

		assert.True(t, msg.naked)
		assert.Equal(t, 10*time.Second, msg.nakDelay)
		assert.False(t, msg.terminated)
		assert.Contains(t, logs.String(), "panic in event handler")
	})

	t.Run("a panic on the final delivery is logged as a drop", func(t *testing.T) {
		var logs bytes.Buffer
		msg := &fakeJetStreamMsg{subject: "$KV.v1-objects.itx-zoom-past-meetings.abc", numDelivery: 8}

		require.NotPanics(t, func() { newProcessor(&logs).msgHandler(ctx)(msg) })

		assert.True(t, msg.terminated)
		assert.False(t, msg.naked)
		assert.Contains(t, logs.String(), "event dropped after max deliveries")
	})
}
