// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package nats

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
)

// bindScript scripts successive bind outcomes and counts calls.
type bindScript struct {
	kv      KeyValueGetter
	errs    []error // outcome per call; past the end every call succeeds
	calls   atomic.Int32
	release chan struct{} // when non-nil, each call blocks until it is closed
}

func (b *bindScript) bind(ctx context.Context) (KeyValueGetter, error) {
	n := int(b.calls.Add(1)) - 1
	if b.release != nil {
		select {
		case <-b.release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if n < len(b.errs) && b.errs[n] != nil {
		return nil, b.errs[n]
	}
	return b.kv, nil
}

func newLazyForTest(script *bindScript, now func() time.Time) *LazyRecordingRecordReader {
	l := NewLazyRecordingRecordReader(script.bind, slog.New(slog.DiscardHandler))
	l.timeout = time.Second
	l.retryAfter = 10 * time.Second
	l.now = now
	return l
}

func TestLazyRecordingRecordReader(t *testing.T) {
	const id = "96365872728-1630560600000"
	newKV := func() *fakeKV {
		return &fakeKV{values: map[string][]byte{"itx-zoom-past-meetings-recordings." + id: []byte(recordingJSON)}}
	}

	t.Run("binds on first use and memoises success", func(t *testing.T) {
		script := &bindScript{kv: newKV()}
		l := newLazyForTest(script, time.Now)
		assert.Equal(t, int32(0), script.calls.Load(), "nothing is bound at construction")

		for i := 0; i < 3; i++ {
			rec, err := l.GetRecordingRecord(context.Background(), id)
			require.NoError(t, err)
			assert.Equal(t, id, rec.MeetingAndOccurrenceID)
		}
		assert.Equal(t, int32(1), script.calls.Load())
	})

	t.Run("a failed bind is not memoised: the next call after the window binds and serves", func(t *testing.T) {
		clock := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
		script := &bindScript{kv: newKV(), errs: []error{errors.New("nats: bucket not found")}}
		l := newLazyForTest(script, func() time.Time { return clock })

		_, err := l.GetRecordingRecord(context.Background(), id)
		require.Error(t, err)
		assert.Equal(t, domain.ErrorTypeUnavailable, domain.GetErrorType(err))
		assert.Equal(t, int32(1), script.calls.Load())

		clock = clock.Add(11 * time.Second)
		rec, err := l.GetRecordingRecord(context.Background(), id)
		require.NoError(t, err)
		assert.Equal(t, id, rec.MeetingAndOccurrenceID)
		assert.Equal(t, int32(2), script.calls.Load())
	})

	t.Run("inside the negative-cache window no bind is attempted", func(t *testing.T) {
		clock := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
		script := &bindScript{kv: newKV(), errs: []error{errors.New("nats: bucket not found")}}
		l := newLazyForTest(script, func() time.Time { return clock })

		_, err := l.GetRecordingRecord(context.Background(), id)
		require.Error(t, err)
		for i := 0; i < 5; i++ {
			clock = clock.Add(time.Second)
			_, err := l.GetRecordingRecord(context.Background(), id)
			assert.Equal(t, domain.ErrorTypeUnavailable, domain.GetErrorType(err))
		}
		assert.Equal(t, int32(1), script.calls.Load(), "no I/O inside the window")
		assert.Empty(t, script.kv.(*fakeKV).keys, "no KV read happened without a bound bucket")
	})

	t.Run("concurrent first calls bind once", func(t *testing.T) {
		script := &bindScript{kv: newKV(), release: make(chan struct{})}
		l := newLazyForTest(script, time.Now)

		const callers = 16
		var wg sync.WaitGroup
		results := make(chan error, callers)
		for i := 0; i < callers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := l.GetRecordingRecord(context.Background(), id)
				results <- err
			}()
		}
		// Let every caller reach the bind or the wait before the bind completes.
		require.Eventually(t, func() bool { return script.calls.Load() == 1 }, time.Second, time.Millisecond)
		time.Sleep(20 * time.Millisecond)
		close(script.release)
		wg.Wait()
		close(results)

		for err := range results {
			assert.NoError(t, err)
		}
		assert.Equal(t, int32(1), script.calls.Load())
	})

	t.Run("a waiter whose context ends while a bind is in flight gets unavailable", func(t *testing.T) {
		script := &bindScript{kv: newKV(), release: make(chan struct{})}
		l := newLazyForTest(script, time.Now)
		t.Cleanup(func() { close(script.release) })

		go func() { _, _ = l.GetRecordingRecord(context.Background(), id) }()
		require.Eventually(t, func() bool { return script.calls.Load() == 1 }, time.Second, time.Millisecond)

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()
		_, err := l.GetRecordingRecord(ctx, id)
		assert.Equal(t, domain.ErrorTypeUnavailable, domain.GetErrorType(err))
		assert.Equal(t, int32(1), script.calls.Load())
	})

	t.Run("the bind is detached from the caller's cancellation", func(t *testing.T) {
		script := &bindScript{kv: newKV()}
		l := newLazyForTest(script, time.Now)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		// The bind itself sees a live context; the read afterwards uses the caller's.
		_, _ = l.GetRecordingRecord(ctx, id)
		l.mu.Lock()
		bound := l.reader != nil
		l.mu.Unlock()
		assert.True(t, bound)
	})
}
