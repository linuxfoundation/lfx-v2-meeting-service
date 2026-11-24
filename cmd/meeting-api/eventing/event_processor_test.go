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
	streamSeq   uint64
	headers     nats.Header
	data        []byte
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
	return &jetstream.MsgMetadata{
		NumDelivered: m.numDelivery,
		Sequence:     jetstream.SequencePair{Stream: m.streamSeq},
	}, nil
}
func (m *fakeJetStreamMsg) Data() []byte         { return m.data }
func (m *fakeJetStreamMsg) Headers() nats.Header { return m.headers }
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
			logger:      slog.New(slog.NewTextHandler(logs, nil)),
			config:      infraeventing.Config{MaxDeliver: 8},
			v1ObjectsKV: &fakeLatestKV{}, // revision 0: every message is current
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

// fakeRevisionEntry is a KV entry that only carries a revision.
type fakeRevisionEntry struct {
	jetstream.KeyValueEntry
	revision uint64
}

func (e fakeRevisionEntry) Revision() uint64 { return e.revision }

// fakeLatestKV answers Get with a fixed revision, or a fixed error. It embeds
// jetstream.KeyValue so it can be assigned to EventProcessor; only Get is called.
type fakeLatestKV struct {
	jetstream.KeyValue
	revision uint64
	err      error
	gets     []string
}

func (k *fakeLatestKV) Get(_ context.Context, key string) (jetstream.KeyValueEntry, error) {
	k.gets = append(k.gets, key)
	if k.err != nil {
		return nil, k.err
	}
	return fakeRevisionEntry{revision: k.revision}, nil
}

func TestCheckRedelivery(t *testing.T) {
	ctx := context.Background()
	const subject = "$KV.v1-objects.itx-zoom-meetings-registrants-v2.abc"
	softDeletePayload := []byte(`{"id":"abc","_sdc_deleted_at":"2026-01-01T00:00:00Z"}`)
	livePayload := []byte(`{"id":"abc"}`)
	deleteHeaders := nats.Header{"KV-Operation": []string{"DEL"}}
	purgeHeaders := nats.Header{"KV-Operation": []string{"PURGE"}}

	tests := []struct {
		name    string
		msg     *fakeJetStreamMsg
		kv      *fakeLatestKV
		want    redeliveryVerdict
		wantGet bool
	}{
		{
			name: "the first delivery is never checked",
			msg:  &fakeJetStreamMsg{subject: subject, numDelivery: 1, streamSeq: 5},
			kv:   &fakeLatestKV{revision: 9},
			want: redeliveryCurrent,
		},
		{
			name:    "a redelivery of the latest revision is current",
			msg:     &fakeJetStreamMsg{subject: subject, numDelivery: 3, streamSeq: 9},
			kv:      &fakeLatestKV{revision: 9},
			want:    redeliveryCurrent,
			wantGet: true,
		},
		{
			name:    "a redelivery of an older revision is superseded",
			msg:     &fakeJetStreamMsg{subject: subject, numDelivery: 3, streamSeq: 5},
			kv:      &fakeLatestKV{revision: 9},
			want:    redeliverySuperseded,
			wantGet: true,
		},
		{
			name:    "a put whose key was deleted since is superseded",
			msg:     &fakeJetStreamMsg{subject: subject, numDelivery: 2, streamSeq: 5},
			kv:      &fakeLatestKV{err: jetstream.ErrKeyNotFound},
			want:    redeliverySuperseded,
			wantGet: true,
		},
		{
			name:    "a delete whose key is still deleted is current and must still be applied",
			msg:     &fakeJetStreamMsg{subject: subject, numDelivery: 2, streamSeq: 5, headers: deleteHeaders},
			kv:      &fakeLatestKV{err: jetstream.ErrKeyNotFound},
			want:    redeliveryCurrent,
			wantGet: true,
		},
		{
			name:    "a purge whose key is still deleted is current",
			msg:     &fakeJetStreamMsg{subject: subject, numDelivery: 2, streamSeq: 5, headers: purgeHeaders},
			kv:      &fakeLatestKV{err: jetstream.ErrKeyNotFound},
			want:    redeliveryCurrent,
			wantGet: true,
		},
		{
			name:    "a delete whose key was recreated since is superseded",
			msg:     &fakeJetStreamMsg{subject: subject, numDelivery: 2, streamSeq: 5, headers: deleteHeaders},
			kv:      &fakeLatestKV{revision: 9},
			want:    redeliverySuperseded,
			wantGet: true,
		},
		{
			name:    "a soft delete whose key was hard-deleted since is current, its cleanup needs the payload",
			msg:     &fakeJetStreamMsg{subject: subject, numDelivery: 2, streamSeq: 5, data: softDeletePayload},
			kv:      &fakeLatestKV{err: jetstream.ErrKeyNotFound},
			want:    redeliveryCurrent,
			wantGet: true,
		},
		{
			name:    "a soft delete whose key was restored since is superseded",
			msg:     &fakeJetStreamMsg{subject: subject, numDelivery: 2, streamSeq: 5, data: softDeletePayload},
			kv:      &fakeLatestKV{revision: 9},
			want:    redeliverySuperseded,
			wantGet: true,
		},
		{
			name:    "a live put whose key was hard-deleted since is superseded",
			msg:     &fakeJetStreamMsg{subject: subject, numDelivery: 2, streamSeq: 5, data: livePayload},
			kv:      &fakeLatestKV{err: jetstream.ErrKeyNotFound},
			want:    redeliverySuperseded,
			wantGet: true,
		},
		{
			name:    "an empty deleted marker is a live put, not a soft delete",
			msg:     &fakeJetStreamMsg{subject: subject, numDelivery: 2, streamSeq: 5, data: []byte(`{"id":"abc","_sdc_deleted_at":""}`)},
			kv:      &fakeLatestKV{err: jetstream.ErrKeyNotFound},
			want:    redeliverySuperseded,
			wantGet: true,
		},
		{
			name:    "a payload that cannot be decoded is superseded when its key is gone",
			msg:     &fakeJetStreamMsg{subject: subject, numDelivery: 2, streamSeq: 5, data: []byte("\xff not a record")},
			kv:      &fakeLatestKV{err: jetstream.ErrKeyNotFound},
			want:    redeliverySuperseded,
			wantGet: true,
		},
		{
			name:    "a lookup error means the message cannot be shown to be current",
			msg:     &fakeJetStreamMsg{subject: subject, numDelivery: 2, streamSeq: 5},
			kv:      &fakeLatestKV{err: errors.New("nats timeout")},
			want:    redeliveryUnknown,
			wantGet: true,
		},
		{
			name: "unavailable metadata is treated as current",
			msg:  &fakeJetStreamMsg{subject: subject, metadataErr: errors.New("no metadata")},
			kv:   &fakeLatestKV{revision: 9},
			want: redeliveryCurrent,
		},
		{
			name: "a malformed subject is left for the handler to reject",
			msg:  &fakeJetStreamMsg{subject: "$KV.v1-objects", numDelivery: 2, streamSeq: 5},
			kv:   &fakeLatestKV{revision: 9},
			want: redeliveryCurrent,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, checkRedelivery(ctx, tt.kv, tt.msg))
			if tt.wantGet {
				assert.Equal(t, []string{"itx-zoom-meetings-registrants-v2.abc"}, tt.kv.gets, "the key is the part after $KV.<bucket>.")
			} else {
				assert.Empty(t, tt.kv.gets)
			}
		})
	}

	t.Run("a missing bucket is treated as current", func(t *testing.T) {
		msg := &fakeJetStreamMsg{subject: subject, numDelivery: 3, streamSeq: 5}
		assert.Equal(t, redeliveryCurrent, checkRedelivery(ctx, nil, msg))
	})
}

func TestMsgHandlerStaleRedelivery(t *testing.T) {
	ctx := context.Background()
	const subject = "$KV.v1-objects.itx-zoom-meetings-registrants-v2.abc"

	// handlers is nil on purpose: kvHandler dereferences it, so a message that
	// reaches the handler panics and is NAKed. A skipped message must not.
	newProcessor := func(logs *bytes.Buffer, kv *fakeLatestKV) *EventProcessor {
		return &EventProcessor{
			logger:      slog.New(slog.NewTextHandler(logs, nil)),
			config:      infraeventing.Config{MaxDeliver: 8},
			v1ObjectsKV: kv,
		}
	}

	t.Run("a superseded redelivery is acknowledged without being processed", func(t *testing.T) {
		var logs bytes.Buffer
		msg := &fakeJetStreamMsg{subject: subject, numDelivery: 4, streamSeq: 5}

		newProcessor(&logs, &fakeLatestKV{revision: 9}).msgHandler(ctx)(msg)

		assert.True(t, msg.acked)
		assert.False(t, msg.naked)
		assert.False(t, msg.terminated)
		out := logs.String()
		assert.Contains(t, out, "skipping redelivery of a superseded revision")
		assert.NotContains(t, out, "panic in event handler", "the handler must not run")
	})

	t.Run("a deleted key supersedes a redelivered put", func(t *testing.T) {
		var logs bytes.Buffer
		msg := &fakeJetStreamMsg{subject: subject, numDelivery: 2, streamSeq: 5}

		newProcessor(&logs, &fakeLatestKV{err: jetstream.ErrKeyNotFound}).msgHandler(ctx)(msg)

		assert.True(t, msg.acked)
		assert.NotContains(t, logs.String(), "panic in event handler")
	})

	t.Run("a lookup error is retried with the scheduled delay", func(t *testing.T) {
		var logs bytes.Buffer
		msg := &fakeJetStreamMsg{subject: subject, numDelivery: 2, streamSeq: 5}

		newProcessor(&logs, &fakeLatestKV{err: errors.New("nats timeout")}).msgHandler(ctx)(msg)

		assert.True(t, msg.naked)
		assert.Equal(t, 10*time.Second, msg.nakDelay)
		assert.False(t, msg.acked)
		assert.NotContains(t, logs.String(), "panic in event handler")
	})

	t.Run("a lookup error on the final delivery is logged as a drop", func(t *testing.T) {
		var logs bytes.Buffer
		msg := &fakeJetStreamMsg{subject: subject, numDelivery: 8, streamSeq: 5}

		newProcessor(&logs, &fakeLatestKV{err: errors.New("nats timeout")}).msgHandler(ctx)(msg)

		assert.True(t, msg.terminated)
		assert.Contains(t, logs.String(), "event dropped after max deliveries")
	})

	t.Run("a failed soft delete whose key was hard-deleted since still reaches the handler", func(t *testing.T) {
		var logs bytes.Buffer
		msg := &fakeJetStreamMsg{
			subject:     subject,
			numDelivery: 2,
			streamSeq:   5,
			data:        []byte(`{"id":"abc","_sdc_deleted_at":"2026-01-01T00:00:00Z"}`),
		}

		newProcessor(&logs, &fakeLatestKV{err: jetstream.ErrKeyNotFound}).msgHandler(ctx)(msg)

		assert.Contains(t, logs.String(), "panic in event handler", "reaching the nil handlers proves it was processed")
		assert.NotContains(t, logs.String(), "skipping redelivery")
		assert.False(t, msg.acked)
	})

	t.Run("a current redelivery still reaches the handler", func(t *testing.T) {
		var logs bytes.Buffer
		msg := &fakeJetStreamMsg{subject: subject, numDelivery: 2, streamSeq: 9}

		newProcessor(&logs, &fakeLatestKV{revision: 9}).msgHandler(ctx)(msg)

		assert.Contains(t, logs.String(), "panic in event handler", "reaching the nil handlers proves it was processed")
		assert.NotContains(t, logs.String(), "skipping redelivery")
	})
}
