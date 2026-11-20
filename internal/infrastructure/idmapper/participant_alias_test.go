// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package idmapper

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/require"
)

type aliasEntry struct {
	jetstream.KeyValueEntry
	value []byte
}

func (e aliasEntry) Value() []byte { return e.value }

type fakeAliasState struct {
	get    func(context.Context, string) (jetstream.KeyValueEntry, error)
	closed atomic.Int32
}

func (s *fakeAliasState) Get(ctx context.Context, key string) (jetstream.KeyValueEntry, error) {
	if s.get != nil {
		return s.get(ctx, key)
	}
	return aliasEntry{value: []byte("participant")}, nil
}
func (s *fakeAliasState) Ready(ctx context.Context) error { return ctx.Err() }
func (s *fakeAliasState) Close()                          { s.closed.Add(1) }

func TestLookupParticipantAlias(t *testing.T) {
	t.Run("concurrent callers share one initialization", func(t *testing.T) {
		var opens atomic.Int32
		state := &fakeAliasState{}
		started := make(chan struct{})
		release := make(chan struct{})
		mapper := &NATSMapper{timeout: time.Second, openState: func(ctx context.Context, _ string) (aliasState, error) {
			opens.Add(1)
			close(started)
			select {
			case <-release:
				return state, nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}}
		t.Cleanup(mapper.Close)
		var wg sync.WaitGroup
		results := make(chan error, 16)
		for range 16 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := mapper.lookupParticipantAlias(context.Background(), "past_meeting_participant.invitee_id.test")
				results <- err
			}()
		}
		<-started
		close(release)
		wg.Wait()
		close(results)
		for err := range results {
			require.NoError(t, err)
		}
		require.Equal(t, int32(1), opens.Load())
		mapper.Close()
		require.Equal(t, int32(1), state.closed.Load())
	})
	t.Run("canceled waiter does not wait for connection setup", func(t *testing.T) {
		started := make(chan struct{})
		release := make(chan struct{})
		state := &fakeAliasState{}
		mapper := &NATSMapper{timeout: 5 * time.Second, openState: func(ctx context.Context, _ string) (aliasState, error) {
			close(started)
			select {
			case <-release:
				return state, nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}}
		t.Cleanup(mapper.Close)
		first := make(chan error, 1)
		go func() {
			_, err := mapper.lookupParticipantAlias(context.Background(), "past_meeting_participant.invitee_id.test")
			first <- err
		}()
		<-started
		ctx, cancel := context.WithCancel(context.Background())
		waiter := make(chan error, 1)
		go func() {
			_, err := mapper.lookupParticipantAlias(ctx, "past_meeting_participant.invitee_id.test")
			waiter <- err
		}()
		cancel()
		select {
		case err := <-waiter:
			require.Error(t, err)
			require.Equal(t, domain.ErrorTypeUnavailable, domain.GetErrorType(err))
		case <-time.After(time.Second):
			t.Fatal("canceled waiter blocked on initialization")
		}
		close(release)
		require.NoError(t, <-first)
	})
	t.Run("configured timeout and earlier caller deadline bound reads", func(t *testing.T) {
		for _, callerBudget := range []time.Duration{time.Second, 10 * time.Millisecond} {
			state := &fakeAliasState{get: func(ctx context.Context, _ string) (jetstream.KeyValueEntry, error) {
				deadline, ok := ctx.Deadline()
				require.True(t, ok)
				require.LessOrEqual(t, time.Until(deadline), min(callerBudget, 30*time.Millisecond))
				<-ctx.Done()
				return nil, ctx.Err()
			}}
			mapper := &NATSMapper{timeout: 30 * time.Millisecond, openState: func(context.Context, string) (aliasState, error) { return state, nil }}
			ctx, cancel := context.WithTimeout(context.Background(), callerBudget)
			_, err := mapper.lookupParticipantAlias(ctx, "past_meeting_participant.invitee_id.test")
			cancel()
			require.Error(t, err)
			require.Equal(t, domain.ErrorTypeUnavailable, domain.GetErrorType(err))
			mapper.Close()
		}
	})
	t.Run("shutdown cancels initialization and releases any late pool", func(t *testing.T) {
		started := make(chan struct{})
		state := &fakeAliasState{}
		var opens atomic.Int32
		mapper := &NATSMapper{timeout: time.Second, openState: func(ctx context.Context, _ string) (aliasState, error) {
			opens.Add(1)
			close(started)
			<-ctx.Done()
			return state, nil
		}}
		result := make(chan error, 1)
		go func() {
			_, err := mapper.lookupParticipantAlias(context.Background(), "past_meeting_participant.invitee_id.test")
			result <- err
		}()
		<-started
		mapper.Close()
		require.Error(t, <-result)
		require.Equal(t, int32(1), state.closed.Load())
		_, err := mapper.lookupParticipantAlias(context.Background(), "past_meeting_participant.invitee_id.test")
		require.Error(t, err)
		require.Equal(t, int32(1), opens.Load())
	})
}
