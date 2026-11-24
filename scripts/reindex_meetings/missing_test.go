// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeKeyLister serves a fixed list of keys.
type fakeKeyLister struct {
	keys chan string
}

func (l *fakeKeyLister) Keys() <-chan string { return l.keys }
func (l *fakeKeyLister) Stop() error         { return nil }

// fakeBucket is an in-memory KV bucket that supports the calls findMissing and
// reputEntry make: Get, Update (compare-and-set on the revision) and
// ListKeysFiltered with a "<prefix>>" filter.
type fakeBucket struct {
	jetstream.KeyValue
	values   map[string][]byte
	revs     map[string]uint64
	getErrs  map[string]error
	updated  []string
	listErr  error
	racedKey string // when set, Update on this key reports a revision conflict
}

func newFakeBucket(keys ...string) *fakeBucket {
	b := &fakeBucket{
		values:  map[string][]byte{},
		revs:    map[string]uint64{},
		getErrs: map[string]error{},
	}
	for _, k := range keys {
		b.values[k] = []byte("value-of-" + k)
		b.revs[k] = 1
	}
	return b
}

func (b *fakeBucket) Get(_ context.Context, key string) (jetstream.KeyValueEntry, error) {
	if err := b.getErrs[key]; err != nil {
		return nil, err
	}
	v, ok := b.values[key]
	if !ok {
		return nil, jetstream.ErrKeyNotFound
	}
	return fakeEntry{value: v, revision: b.revs[key]}, nil
}

func (b *fakeBucket) Update(_ context.Context, key string, value []byte, revision uint64) (uint64, error) {
	if key == b.racedKey || b.revs[key] != revision {
		return 0, fmt.Errorf("%w: %w", errors.New("wrong last sequence"), jetstream.ErrKeyRevisionMismatch)
	}
	b.updated = append(b.updated, key)
	b.values[key] = value
	b.revs[key]++
	return b.revs[key], nil
}

func (b *fakeBucket) ListKeysFiltered(_ context.Context, filters ...string) (jetstream.KeyLister, error) {
	if b.listErr != nil {
		return nil, b.listErr
	}
	var keys []string
	for k := range b.values {
		for _, f := range filters {
			if strings.HasPrefix(k, strings.TrimSuffix(f, ">")) {
				keys = append(keys, k)
				break
			}
		}
	}
	sort.Strings(keys)
	ch := make(chan string, len(keys))
	for _, k := range keys {
		ch <- k
	}
	close(ch)
	return &fakeKeyLister{keys: ch}, nil
}

func TestMissingMarkerConfig(t *testing.T) {
	t.Run("every audited type uses the same v1-objects prefix as the OpenSearch mode", func(t *testing.T) {
		for objectType, spec := range missingMarkerConfig {
			prefixes, ok := objectTypeConfig[objectType]
			require.True(t, ok, "%s is audited but not a reindexable type", objectType)
			assert.Equal(t, []string{spec.keyPrefix}, prefixes, objectType)
		}
	})

	t.Run("prefixes end in a dot so an ID can be appended", func(t *testing.T) {
		for objectType, spec := range missingMarkerConfig {
			assert.True(t, strings.HasSuffix(spec.keyPrefix, "."), "%s key prefix", objectType)
			assert.True(t, strings.HasSuffix(spec.markerPrefix, "."), "%s marker prefix", objectType)
		}
	})

	t.Run("no key prefix contains another, so a type never lists another type's keys", func(t *testing.T) {
		for a, specA := range missingMarkerConfig {
			for b, specB := range missingMarkerConfig {
				if a == b {
					continue
				}
				assert.False(t, strings.HasPrefix(specA.keyPrefix, specB.keyPrefix), "%s overlaps %s", a, b)
			}
		}
	})

	t.Run("types without a marker keyed by the v1-objects ID are not audited", func(t *testing.T) {
		assert.NotContains(t, missingMarkerConfig, "v1_meeting_rsvp")
		assert.NotContains(t, missingMarkerConfig, "v1_past_meeting_participant")
	})

	t.Run("past meeting maps to the marker the handler writes", func(t *testing.T) {
		assert.Equal(t, "v1_past_meetings.", missingMarkerConfig["v1_past_meeting"].markerPrefix)
		assert.Equal(t, "v1_meetings.", missingMarkerConfig["v1_meeting"].markerPrefix)
	})
}

func TestFindMissing(t *testing.T) {
	ctx := context.Background()
	spec := missingMarkerConfig["v1_past_meeting"]

	// a is indexed, b was deliberately tombstoned, c has no marker. The
	// recording key shares a name prefix with the past meeting prefix but is a
	// different type and must be ignored.
	objects := func() *fakeBucket {
		return newFakeBucket(
			"itx-zoom-past-meetings.a",
			"itx-zoom-past-meetings.b",
			"itx-zoom-past-meetings.c",
			"itx-zoom-past-meetings-recordings.c",
		)
	}
	mappings := func() *fakeBucket {
		m := newFakeBucket("v1_past_meetings.a", "v1_past_meetings.b")
		m.values["v1_past_meetings.b"] = []byte("!del")
		return m
	}

	t.Run("dry-run reports the record without a marker and changes nothing", func(t *testing.T) {
		kv, kvMappings := objects(), mappings()

		stats, err := findMissing(ctx, kv, kvMappings, "v1_past_meeting", spec, false)

		require.NoError(t, err)
		assert.Equal(t, 3, stats.scanned)
		assert.Equal(t, 2, stats.withMarker)
		assert.Equal(t, 1, stats.missing)
		assert.Equal(t, 0, stats.processed)
		assert.Empty(t, kv.updated)
	})

	t.Run("reindex re-puts only the record without a marker", func(t *testing.T) {
		kv, kvMappings := objects(), mappings()

		stats, err := findMissing(ctx, kv, kvMappings, "v1_past_meeting", spec, true)

		require.NoError(t, err)
		assert.Equal(t, 1, stats.missing)
		assert.Equal(t, 1, stats.processed)
		assert.Equal(t, []string{"itx-zoom-past-meetings.c"}, kv.updated)
		assert.Empty(t, kvMappings.updated, "the mappings bucket is only read")
	})

	t.Run("a tombstoned marker counts as handled", func(t *testing.T) {
		kv := newFakeBucket("itx-zoom-past-meetings.b")
		kvMappings := newFakeBucket("v1_past_meetings.b")
		kvMappings.values["v1_past_meetings.b"] = []byte("!del")

		stats, err := findMissing(ctx, kv, kvMappings, "v1_past_meeting", spec, true)

		require.NoError(t, err)
		assert.Equal(t, 1, stats.withMarker)
		assert.Equal(t, 0, stats.missing)
		assert.Empty(t, kv.updated)
	})

	t.Run("nothing to do when every record has a marker", func(t *testing.T) {
		kv := newFakeBucket("itx-zoom-past-meetings.a")
		kvMappings := newFakeBucket("v1_past_meetings.a")

		stats, err := findMissing(ctx, kv, kvMappings, "v1_past_meeting", spec, true)

		require.NoError(t, err)
		assert.Equal(t, missingStats{scanned: 1, withMarker: 1}, stats)
	})

	t.Run("a marker lookup error is a failure and the record is not re-put", func(t *testing.T) {
		kv, kvMappings := objects(), mappings()
		kvMappings.getErrs["v1_past_meetings.c"] = errors.New("nats timeout")

		stats, err := findMissing(ctx, kv, kvMappings, "v1_past_meeting", spec, true)

		require.NoError(t, err)
		assert.Equal(t, 1, stats.failed)
		assert.Equal(t, 0, stats.missing, "an unreadable marker is not evidence the record is missing")
		assert.Empty(t, kv.updated)
	})

	t.Run("a record a live writer changed is superseded, not overwritten", func(t *testing.T) {
		kv, kvMappings := objects(), mappings()
		kv.racedKey = "itx-zoom-past-meetings.c"

		stats, err := findMissing(ctx, kv, kvMappings, "v1_past_meeting", spec, true)

		require.NoError(t, err)
		assert.Equal(t, 1, stats.superseded)
		assert.Equal(t, 0, stats.processed)
		assert.Empty(t, kv.updated)
	})

	t.Run("a record that disappears before the re-put is counted not found", func(t *testing.T) {
		kv, kvMappings := objects(), mappings()
		kv.getErrs["itx-zoom-past-meetings.c"] = jetstream.ErrKeyNotFound

		stats, err := findMissing(ctx, kv, kvMappings, "v1_past_meeting", spec, true)

		require.NoError(t, err)
		assert.Equal(t, 1, stats.notFound)
		assert.Equal(t, 0, stats.failed)
	})

	t.Run("a listing failure is returned", func(t *testing.T) {
		kv := objects()
		kv.listErr = errors.New("list failed")

		_, err := findMissing(ctx, kv, mappings(), "v1_past_meeting", spec, true)

		require.ErrorContains(t, err, "list failed")
	})
}
