// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package store

import (
	"context"
	"errors"
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

// mockKeyValueEntry implements jetstream.KeyValueEntry for testing
type mockKeyValueEntry struct {
	key      string
	value    []byte
	revision uint64
}

func (m *mockKeyValueEntry) Key() string                     { return m.key }
func (m *mockKeyValueEntry) Value() []byte                   { return m.value }
func (m *mockKeyValueEntry) Revision() uint64                { return m.revision }
func (m *mockKeyValueEntry) Created() time.Time              { return time.Now() }
func (m *mockKeyValueEntry) Delta() uint64                   { return 0 }
func (m *mockKeyValueEntry) Operation() jetstream.KeyValueOp { return jetstream.KeyValuePut }
func (m *mockKeyValueEntry) Bucket() string                  { return "test-bucket" }

// mockKeyLister implements jetstream.KeyLister for testing
type mockKeyLister struct {
	keys  []string
	index int
}

func (m *mockKeyLister) Next() (jetstream.KeyValueEntry, error) {
	if m.index >= len(m.keys) {
		return nil, errors.New("no more keys")
	}
	key := m.keys[m.index]
	m.index++
	return &mockKeyValueEntry{key: key, value: []byte(`{"uid":"` + key + `","title":"Test Meeting"}`)}, nil
}

func (m *mockKeyLister) Keys() <-chan string {
	ch := make(chan string)
	go func() {
		defer close(ch)
		for _, key := range m.keys {
			ch <- key
		}
	}()
	return ch
}

func (m *mockKeyLister) Stop() error { return nil }

// mockNatsKeyValue implements INatsKeyValue for testing
type mockNatsKeyValue struct {
	data        map[string][]byte
	revisions   map[string]uint64
	putError    error
	getError    error
	deleteError error
	updateError error
}

func newMockNatsKeyValue() *mockNatsKeyValue {
	return &mockNatsKeyValue{
		data:      make(map[string][]byte),
		revisions: make(map[string]uint64),
	}
}

func (m *mockNatsKeyValue) ListKeys(ctx context.Context, opts ...jetstream.WatchOpt) (jetstream.KeyLister, error) {
	keys := make([]string, 0, len(m.data))
	for key := range m.data {
		keys = append(keys, key)
	}
	return &mockKeyLister{keys: keys}, nil
}

func (m *mockNatsKeyValue) Get(ctx context.Context, key string) (jetstream.KeyValueEntry, error) {
	if m.getError != nil {
		return nil, m.getError
	}
	value, exists := m.data[key]
	if !exists {
		return nil, jetstream.ErrKeyNotFound
	}
	revision := m.revisions[key]
	return &mockKeyValueEntry{key: key, value: value, revision: revision}, nil
}

func (m *mockNatsKeyValue) Put(ctx context.Context, key string, data []byte) (uint64, error) {
	if m.putError != nil {
		return 0, m.putError
	}
	// Store with the key as-is (already encoded)
	m.data[key] = data
	revision := uint64(1)
	if existingRevision, exists := m.revisions[key]; exists {
		revision = existingRevision + 1
	}
	m.revisions[key] = revision
	return revision, nil
}

func (m *mockNatsKeyValue) Update(ctx context.Context, key string, data []byte, expectedRevision uint64) (uint64, error) {
	if m.updateError != nil {
		return 0, m.updateError
	}
	currentRevision, exists := m.revisions[key]
	if !exists {
		return 0, jetstream.ErrKeyNotFound
	}
	if currentRevision != expectedRevision {
		return 0, errors.New("wrong last sequence")
	}
	m.data[key] = data
	newRevision := currentRevision + 1
	m.revisions[key] = newRevision
	return newRevision, nil
}

func (m *mockNatsKeyValue) Delete(ctx context.Context, key string, opts ...jetstream.KVDeleteOpt) error {
	if m.deleteError != nil {
		return m.deleteError
	}
	// Check if revision mismatch should be simulated
	// We need to check if the key exists and handle revision checking for delete operations
	if _, exists := m.data[key]; !exists {
		return jetstream.ErrKeyNotFound
	}
	delete(m.data, key)
	delete(m.revisions, key)
	return nil
}
