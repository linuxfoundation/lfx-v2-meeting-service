// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package idmapper

import (
	"context"
	"errors"
	"time"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/infrastructure/eventstate"
	"github.com/nats-io/nats.go/jetstream"
)

// lookupParticipantAlias never sends meeting-owned lookups to the helper. A
// missing alias retains the legacy validation/miss behavior; unavailable PG is
// a 503, not permission to guess an ID or consult a stale mapping snapshot.
func (m *NATSMapper) lookupParticipantAlias(ctx context.Context, key string) (string, error) {
	mapperCtx, cancel := context.WithTimeout(ctx, m.aliasTimeout())
	defer cancel()
	store, err := m.aliasStore(mapperCtx)
	if err != nil {
		if mapperCtx.Err() != nil {
			return "", domain.NewUnavailableError("participant state unavailable", mapperCtx.Err())
		}
		return "", domain.NewUnavailableError("participant state unavailable")
	}
	e, err := store.Get(mapperCtx, key)
	if errors.Is(err, jetstream.ErrKeyNotFound) || (err == nil && (len(e.Value()) == 0 || string(e.Value()) == "!del")) {
		return "", domain.NewValidationError("participant alias not found")
	}
	if err != nil {
		return "", domain.NewUnavailableError("participant state unavailable")
	}
	return string(e.Value()), nil
}

type aliasState interface {
	Get(context.Context, string) (jetstream.KeyValueEntry, error)
	Ready(context.Context) error
	Close()
}

type aliasInitialization struct {
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
	state  aliasState
	err    error
}

func (m *NATSMapper) aliasTimeout() time.Duration {
	if m.timeout > 0 {
		return m.timeout
	}
	return defaultTimeout
}

func (m *NATSMapper) aliasStore(ctx context.Context) (aliasState, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.stateMu.Lock()
	if m.stateClosed {
		m.stateMu.Unlock()
		return nil, domain.NewUnavailableError("participant mapper closed")
	}
	if m.state != nil {
		state := m.state
		m.stateMu.Unlock()
		return state, nil
	}
	init := m.stateInit
	start := init == nil
	if start {
		// Initialization is shared, with its own configured budget and shutdown
		// cancellation. One caller's earlier deadline does not cancel other
		// waiters; each caller still returns within its own request deadline.
		initCtx, cancel := context.WithTimeout(context.Background(), m.aliasTimeout())
		init = &aliasInitialization{ctx: initCtx, cancel: cancel, done: make(chan struct{})}
		m.stateInit = init
	}
	m.stateMu.Unlock()
	if start {
		go m.initializeAliasState(init)
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-init.done:
		m.stateMu.Lock()
		defer m.stateMu.Unlock()
		if m.stateClosed {
			return nil, domain.NewUnavailableError("participant mapper closed")
		}
		return init.state, init.err
	}
}

func (m *NATSMapper) initializeAliasState(init *aliasInitialization) {
	defer init.cancel()
	opener := m.openState
	if opener == nil {
		opener = func(ctx context.Context, dsn string) (aliasState, error) {
			state, err := eventstate.Open(ctx, dsn)
			if err != nil {
				return nil, err
			}
			return state, nil
		}
	}
	state, err := opener(init.ctx, m.stateDatabaseURL)
	if err == nil && state != nil {
		err = state.Ready(init.ctx)
	}
	if err == nil {
		err = init.ctx.Err()
	}
	if err == nil && state == nil {
		err = domain.NewUnavailableError("participant state unavailable")
	}
	if err != nil && state != nil {
		state.Close()
		state = nil
	}
	m.stateMu.Lock()
	closed := m.stateClosed
	if closed {
		init.err = domain.NewUnavailableError("participant mapper closed")
	} else {
		init.state, init.err = state, err
		if err == nil {
			m.state = state
		}
	}
	m.stateMu.Unlock()
	if closed && state != nil {
		state.Close()
	}
	m.stateMu.Lock()
	close(init.done)
	if m.stateInit == init {
		m.stateInit = nil
	}
	m.stateMu.Unlock()
}
