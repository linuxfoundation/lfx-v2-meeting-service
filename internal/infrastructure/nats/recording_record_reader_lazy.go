// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package nats

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
)

const (
	// lazyBindTimeout bounds one attempt to bind the KV bucket.
	lazyBindTimeout = 5 * time.Second
	// lazyBindRetryAfter is the negative-cache window after a failed bind: calls
	// inside it answer unavailable without I/O, which bounds retry storms.
	lazyBindRetryAfter = 10 * time.Second
)

// KeyValueBinder opens the KV bucket a reader needs. It is called lazily, on the
// first read, and again after a failure once the negative-cache window has passed.
type KeyValueBinder func(ctx context.Context) (KeyValueGetter, error)

// LazyRecordingRecordReader implements domain.RecordingRecordReader over a KV
// bucket that is bound on first use rather than at startup, so a bucket that is
// not reachable when the service starts (or that is recreated later) does not
// leave the route unavailable until a restart.
//
// Only a successful bind is memoised. The mutex guards state, never I/O: one bind
// runs at a time while concurrent callers wait for its outcome, a bind runs under
// its own timeout detached from the caller's cancellation, and after a failure the
// next attempt waits for the negative-cache window.
type LazyRecordingRecordReader struct {
	bind       KeyValueBinder
	logger     *slog.Logger
	timeout    time.Duration
	retryAfter time.Duration
	now        func() time.Time

	mu       sync.Mutex
	reader   *RecordingRecordReader // non-nil once bound
	inflight chan struct{}          // closed when the current bind finishes; nil when none runs
	failedAt time.Time              // zero when the last bind succeeded or none failed
}

// Verify interface compliance at compile time.
var _ domain.RecordingRecordReader = (*LazyRecordingRecordReader)(nil)

// NewLazyRecordingRecordReader creates a reader that binds its bucket with bind on
// first use.
func NewLazyRecordingRecordReader(bind KeyValueBinder, logger *slog.Logger) *LazyRecordingRecordReader {
	return &LazyRecordingRecordReader{
		bind:       bind,
		logger:     logger,
		timeout:    lazyBindTimeout,
		retryAfter: lazyBindRetryAfter,
		now:        time.Now,
	}
}

// GetRecordingRecord binds the bucket if needed and delegates to the bound reader.
// While the bucket cannot be bound it returns an unavailable error.
func (l *LazyRecordingRecordReader) GetRecordingRecord(ctx context.Context, meetingAndOccurrenceID string) (*models.RecordingRecord, error) {
	r, err := l.bound(ctx)
	if err != nil {
		return nil, err
	}
	return r.GetRecordingRecord(ctx, meetingAndOccurrenceID)
}

// bound returns the bound reader, binding it on first use.
func (l *LazyRecordingRecordReader) bound(ctx context.Context) (*RecordingRecordReader, error) {
	for {
		l.mu.Lock()
		if l.reader != nil {
			r := l.reader
			l.mu.Unlock()
			return r, nil
		}
		if wait := l.inflight; wait != nil {
			// Another call is binding: wait for its outcome, then re-read the state.
			l.mu.Unlock()
			select {
			case <-wait:
				continue
			case <-ctx.Done():
				return nil, domain.NewUnavailableError("recording record store unavailable")
			}
		}
		if !l.failedAt.IsZero() && l.now().Sub(l.failedAt) < l.retryAfter {
			l.mu.Unlock()
			return nil, domain.NewUnavailableError("recording record store unavailable")
		}
		done := make(chan struct{})
		l.inflight = done
		l.mu.Unlock()

		// The bind runs outside the lock, under its own timeout and detached from
		// the caller's cancellation so one aborted request cannot poison the attempt.
		bindCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), l.timeout)
		kv, err := l.bind(bindCtx)
		cancel()

		l.mu.Lock()
		if err != nil {
			l.failedAt = l.now()
			l.logger.With(logging.ErrKey, err).WarnContext(ctx, "failed to bind recording record KV bucket; will retry after the negative-cache window")
		} else if l.reader == nil {
			l.reader = NewRecordingRecordReader(kv, l.logger)
			l.failedAt = time.Time{}
		}
		l.inflight = nil
		close(done)
		l.mu.Unlock()
		if err != nil {
			return nil, domain.NewUnavailableError("recording record store unavailable")
		}
	}
}
