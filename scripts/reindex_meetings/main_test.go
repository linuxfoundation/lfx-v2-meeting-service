// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"errors"
	"testing"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
)

type fakeEntry struct {
	jetstream.KeyValueEntry
	value    []byte
	revision uint64
}

func (e fakeEntry) Value() []byte    { return e.value }
func (e fakeEntry) Revision() uint64 { return e.revision }

// fakeKV holds one key. afterGet, when set, runs between Get and Update to
// simulate a live writer racing the script.
type fakeKV struct {
	jetstream.KeyValue
	value     []byte
	revision  uint64
	exists    bool
	afterGet  func(*fakeKV)
	updateErr error
	updates   int
}

func (f *fakeKV) Get(_ context.Context, _ string) (jetstream.KeyValueEntry, error) {
	if !f.exists {
		return nil, jetstream.ErrKeyNotFound
	}
	entry := fakeEntry{value: f.value, revision: f.revision}
	if f.afterGet != nil {
		f.afterGet(f)
	}
	return entry, nil
}

func (f *fakeKV) Update(_ context.Context, _ string, value []byte, revision uint64) (uint64, error) {
	if f.updateErr != nil {
		return 0, f.updateErr
	}
	if !f.exists || revision != f.revision {
		return 0, jetstream.ErrKeyExists
	}
	f.updates++
	f.value = value
	f.revision++
	return f.revision, nil
}

func TestReputEntry(t *testing.T) {
	ctx := context.Background()
	const key = "itx-zoom-meetings-registrants-v2.00000000-0000-0000-0000-000000000001"

	t.Run("unchanged key is re-put at the read revision", func(t *testing.T) {
		kv := &fakeKV{value: []byte(`{"username":"alice"}`), revision: 7, exists: true}

		assert.Equal(t, reputProcessed, reputEntry(ctx, kv, key))
		assert.Equal(t, 1, kv.updates)
		assert.Equal(t, uint64(8), kv.revision)
	})

	t.Run("concurrent write is not overwritten with the stale value", func(t *testing.T) {
		kv := &fakeKV{value: []byte(`{"username":"alice"}`), revision: 7, exists: true}
		kv.afterGet = func(f *fakeKV) {
			f.value = []byte(`{"username":"alice","_sdc_deleted_at":"2026-01-01T00:00:00Z"}`)
			f.revision = 8
		}

		assert.Equal(t, reputSuperseded, reputEntry(ctx, kv, key))
		assert.Equal(t, 0, kv.updates)
		assert.JSONEq(t, `{"username":"alice","_sdc_deleted_at":"2026-01-01T00:00:00Z"}`, string(kv.value))
	})

	t.Run("concurrent hard delete is not overwritten", func(t *testing.T) {
		kv := &fakeKV{value: []byte(`{"username":"alice"}`), revision: 7, exists: true}
		kv.afterGet = func(f *fakeKV) {
			f.exists = false
			f.revision = 8
		}

		assert.Equal(t, reputSuperseded, reputEntry(ctx, kv, key))
		assert.Equal(t, 0, kv.updates)
		assert.False(t, kv.exists)
	})

	t.Run("missing key is reported not found", func(t *testing.T) {
		kv := &fakeKV{}

		assert.Equal(t, reputNotFound, reputEntry(ctx, kv, key))
	})

	t.Run("other update error is a failure", func(t *testing.T) {
		kv := &fakeKV{value: []byte(`{}`), revision: 1, exists: true, updateErr: errors.New("timeout")}

		assert.Equal(t, reputFailed, reputEntry(ctx, kv, key))
	})
}
