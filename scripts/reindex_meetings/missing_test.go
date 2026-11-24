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
	t.Run("every reindexable type is audited", func(t *testing.T) {
		for objectType := range objectTypeConfig {
			assert.Contains(t, missingMarkerConfig, objectType)
		}
		assert.Len(t, missingMarkerConfig, len(objectTypeConfig))
	})

	t.Run("key prefixes match the OpenSearch mode where it has fixed prefixes", func(t *testing.T) {
		for objectType, specs := range missingMarkerConfig {
			want := objectTypeConfig[objectType]
			if want == nil {
				continue // participants are resolved through a cross-reference there
			}
			var got []string
			for _, spec := range specs {
				got = append(got, spec.keyPrefix)
			}
			assert.Equal(t, want, got, objectType)
		}
	})

	t.Run("participants scan invitees and attendees", func(t *testing.T) {
		specs := missingMarkerConfig["v1_past_meeting_participant"]
		require.Len(t, specs, 2)
		assert.Equal(t, markerSpec{keyPrefix: "itx-zoom-past-meetings-invitees.", markerPrefix: "v1_past_meeting_invitees."}, specs[0])
		assert.Equal(t, markerSpec{keyPrefix: "itx-zoom-past-meetings-attendees.", markerPrefix: "v1_past_meeting_attendees."}, specs[1])
	})

	t.Run("prefixes end in a dot so an ID can be appended", func(t *testing.T) {
		for objectType, specs := range missingMarkerConfig {
			for _, spec := range specs {
				assert.True(t, strings.HasSuffix(spec.keyPrefix, "."), "%s key prefix", objectType)
				assert.True(t, strings.HasSuffix(spec.markerPrefix, "."), "%s marker prefix", objectType)
			}
		}
	})

	t.Run("no key prefix contains another, so a scan never lists another type's keys", func(t *testing.T) {
		var all []string
		for _, specs := range missingMarkerConfig {
			for _, spec := range specs {
				all = append(all, spec.keyPrefix)
			}
		}
		for i, a := range all {
			for j, b := range all {
				if i != j {
					assert.False(t, strings.HasPrefix(a, b), "%s overlaps %s", a, b)
				}
			}
		}
	})

	t.Run("markers match the keys the handlers write", func(t *testing.T) {
		assert.Equal(t, "v1_meetings.", missingMarkerConfig["v1_meeting"][0].markerPrefix)
		assert.Equal(t, "v1_past_meetings.", missingMarkerConfig["v1_past_meeting"][0].markerPrefix)
		assert.Equal(t, "v1_invite_responses.", missingMarkerConfig["v1_meeting_rsvp"][0].markerPrefix)
	})
}

func TestFindMissing(t *testing.T) {
	ctx := context.Background()
	spec := missingMarkerConfig["v1_past_meeting"][0]

	// a is indexed, b was deliberately tombstoned, c has no marker. The
	// recording key shares a name prefix with the past meeting prefix but is a
	// different type and must be ignored.
	objects := func() *fakeBucket {
		b := newFakeBucket(
			"itx-zoom-past-meetings.a",
			"itx-zoom-past-meetings.b",
			"itx-zoom-past-meetings.c",
			"itx-zoom-past-meetings-recordings.c",
		)
		b.values["itx-zoom-past-meetings.b"] = []byte(`{"id":"b","_sdc_deleted_at":"2026-01-01T00:00:00Z"}`)
		return b
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

	t.Run("a tombstoned marker on a soft-deleted record counts as handled", func(t *testing.T) {
		kv := newFakeBucket("itx-zoom-past-meetings.b")
		kv.values["itx-zoom-past-meetings.b"] = []byte(`{"_sdc_deleted_at":"2026-01-01T00:00:00Z"}`)
		kvMappings := newFakeBucket("v1_past_meetings.b")
		kvMappings.values["v1_past_meetings.b"] = []byte("!del")

		stats, err := findMissing(ctx, kv, kvMappings, "v1_past_meeting", spec, true)

		require.NoError(t, err)
		assert.Equal(t, 1, stats.withMarker)
		assert.Equal(t, 0, stats.missing)
		assert.Empty(t, kv.updated)
	})

	t.Run("a tombstoned marker on a live record is missing, its restore was dropped", func(t *testing.T) {
		for name, value := range map[string]string{
			"no deleted field":   `{"id":"b"}`,
			"null deleted field": `{"id":"b","_sdc_deleted_at":null}`,
			"empty deleted":      `{"id":"b","_sdc_deleted_at":""}`,
		} {
			kv := newFakeBucket("itx-zoom-past-meetings.b")
			kv.values["itx-zoom-past-meetings.b"] = []byte(value)
			kvMappings := newFakeBucket("v1_past_meetings.b")
			kvMappings.values["v1_past_meetings.b"] = []byte("!del")

			stats, err := findMissing(ctx, kv, kvMappings, "v1_past_meeting", spec, true)

			require.NoError(t, err, name)
			assert.Equal(t, 1, stats.missing, name)
			assert.Equal(t, 1, stats.processed, name)
			assert.Equal(t, []string{"itx-zoom-past-meetings.b"}, kv.updated, name)
		}
	})

	t.Run("a record behind a tombstone that cannot be decoded is a failure and is not re-put", func(t *testing.T) {
		kv := newFakeBucket("itx-zoom-past-meetings.b")
		kv.values["itx-zoom-past-meetings.b"] = []byte("\xff not a record")
		kvMappings := newFakeBucket("v1_past_meetings.b")
		kvMappings.values["v1_past_meetings.b"] = []byte("!del")

		stats, err := findMissing(ctx, kv, kvMappings, "v1_past_meeting", spec, true)

		require.NoError(t, err)
		assert.Equal(t, 1, stats.failed)
		assert.Equal(t, 0, stats.missing)
		assert.Empty(t, kv.updated)
	})

	t.Run("the scan stops when the context is cancelled", func(t *testing.T) {
		cancelled, cancel := context.WithCancel(ctx)
		cancel()

		_, err := findMissing(cancelled, objects(), mappings(), "v1_past_meeting", spec, true)

		require.ErrorIs(t, err, context.Canceled)
	})

	t.Run("participants are scanned per key prefix against their own markers", func(t *testing.T) {
		kv := newFakeBucket(
			"itx-zoom-past-meetings-invitees.i1",
			"itx-zoom-past-meetings-invitees.i2",
			"itx-zoom-past-meetings-attendees.a1",
		)
		kvMappings := newFakeBucket("v1_past_meeting_invitees.i1", "v1_past_meeting_attendees.a1")

		var total missingStats
		for _, s := range missingMarkerConfig["v1_past_meeting_participant"] {
			stats, err := findMissing(ctx, kv, kvMappings, "v1_past_meeting_participant", s, true)
			require.NoError(t, err)
			total.add(stats)
		}

		assert.Equal(t, 3, total.scanned)
		assert.Equal(t, 2, total.withMarker)
		assert.Equal(t, 1, total.missing)
		assert.Equal(t, []string{"itx-zoom-past-meetings-invitees.i2"}, kv.updated)
	})

	t.Run("rsvps use the invite response marker", func(t *testing.T) {
		kv := newFakeBucket("itx-zoom-meetings-invite-responses-v2.r1", "itx-zoom-meetings-invite-responses-v2.r2")
		kvMappings := newFakeBucket("v1_invite_responses.r1")

		stats, err := findMissing(ctx, kv, kvMappings, "v1_meeting_rsvp", missingMarkerConfig["v1_meeting_rsvp"][0], false)

		require.NoError(t, err)
		assert.Equal(t, 1, stats.withMarker)
		assert.Equal(t, 1, stats.missing)
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
