// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/vmihailenco/msgpack/v5"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/infrastructure/v1objects"
)

// tombstoneMarker is the value the event handler writes to a v1-mappings key
// when it processes a delete. A restore replaces it with the live marker.
const tombstoneMarker = "!del"

// markerSpec ties a v1-objects key prefix to the success marker the event
// handler writes to v1-mappings once it has published that record to the
// indexer. The marker key is markerPrefix followed by the same ID that follows
// keyPrefix in the v1-objects key.
type markerSpec struct {
	keyPrefix    string
	markerPrefix string
}

// missingMarkerConfig lists, per object type, the v1-objects key prefixes
// -missing-only scans. Each marker key is the one the corresponding handler in
// cmd/meeting-api/eventing writes after a successful publish, built from the
// same ID that follows the v1-objects key prefix. Participants span two key
// prefixes, one for invitees and one for attendees.
var missingMarkerConfig = map[string][]markerSpec{
	"v1_meeting":                  {{keyPrefix: "itx-zoom-meetings-v2.", markerPrefix: "v1_meetings."}},
	"v1_meeting_registrant":       {{keyPrefix: "itx-zoom-meetings-registrants-v2.", markerPrefix: "v1_meeting_registrants."}},
	"v1_meeting_rsvp":             {{keyPrefix: "itx-zoom-meetings-invite-responses-v2.", markerPrefix: "v1_invite_responses."}},
	"v1_meeting_attachment":       {{keyPrefix: "itx-zoom-meetings-attachments-v2.", markerPrefix: "v1_meeting_attachments."}},
	"v1_past_meeting":             {{keyPrefix: "itx-zoom-past-meetings.", markerPrefix: "v1_past_meetings."}},
	"v1_past_meeting_participant": {{keyPrefix: "itx-zoom-past-meetings-invitees.", markerPrefix: "v1_past_meeting_invitees."}, {keyPrefix: "itx-zoom-past-meetings-attendees.", markerPrefix: "v1_past_meeting_attendees."}},
	"v1_past_meeting_recording":   {{keyPrefix: "itx-zoom-past-meetings-recordings.", markerPrefix: "v1_past_meeting_recordings."}},
	"v1_past_meeting_summary":     {{keyPrefix: "itx-zoom-past-meetings-summaries.", markerPrefix: "v1_past_meeting_summaries."}},
	"v1_past_meeting_attachment":  {{keyPrefix: "itx-zoom-past-meetings-attachments.", markerPrefix: "v1_past_meeting_attachments."}},
}

// supportedMissingTypes returns the sorted object types -missing-only accepts.
func supportedMissingTypes() []string {
	types := make([]string, 0, len(missingMarkerConfig))
	for t := range missingMarkerConfig {
		types = append(types, t)
	}
	sort.Strings(types)
	return types
}

// missingStats counts what findMissing saw for one object type.
type missingStats struct {
	scanned    int // v1-objects keys examined
	withMarker int // keys whose success marker exists (indexed, or deliberately tombstoned)
	missing    int // keys with no success marker
	processed  int // missing keys re-put (always 0 in dry-run)
	failed     int // lookups or re-puts that failed
	notFound   int // keys that vanished between listing and re-put
	superseded int // keys a live writer changed since they were read
}

// add accumulates other into s.
func (s *missingStats) add(other missingStats) {
	s.scanned += other.scanned
	s.withMarker += other.withMarker
	s.missing += other.missing
	s.processed += other.processed
	s.failed += other.failed
	s.notFound += other.notFound
	s.superseded += other.superseded
}

// decodeRecord decodes a v1-objects value the way the event handler does: JSON
// first, then msgpack. The value is read from the platform's own bucket, so,
// unlike the service, this operator script does not bound the msgpack nesting
// depth.
func decodeRecord(value []byte) (map[string]any, error) {
	var data map[string]any
	if err := json.Unmarshal(value, &data); err != nil {
		if err := msgpack.Unmarshal(value, &data); err != nil {
			return nil, fmt.Errorf("decode record: %w", err)
		}
	}
	return data, nil
}

// checkDecodable reports whether a value decodes, for use as a re-put check.
func checkDecodable(value []byte) error {
	_, err := decodeRecord(value)
	return err
}

// readRecord reads and decodes the v1-objects record at key. A key that no
// longer exists is reported as jetstream.ErrKeyNotFound.
func readRecord(ctx context.Context, kv jetstream.KeyValue, key string) (map[string]any, error) {
	entry, err := kv.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	return decodeRecord(entry.Value())
}

// recordIsSoftDeleted reports whether the v1-objects record at key is a soft
// delete (_sdc_deleted_at set), the same rule the event pipeline applies. A
// record that has vanished counts as deleted.
func recordIsSoftDeleted(ctx context.Context, kv jetstream.KeyValue, key string) (bool, error) {
	data, err := readRecord(ctx, kv, key)
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			return true, nil
		}
		return false, err
	}
	_, deleted := v1objects.SoftDeletedAt(data)
	return deleted, nil
}

// findMissing lists the v1-objects keys of one type and checks each for its
// success marker in v1-mappings. A key without a marker never made it through
// the event handler: the event was dropped, or the handler skipped it on
// purpose (for example a project that is not in v2 yet). With reindex set, each
// such key is re-put so the consumer processes it again; otherwise it is only
// logged. Unlike the OpenSearch-driven reindex, this finds records that are
// absent from the index.
func findMissing(
	ctx context.Context,
	kv, kvMappings jetstream.KeyValue,
	objectType string,
	spec markerSpec,
	reindex bool,
) (missingStats, error) {
	var stats missingStats

	lister, err := kv.ListKeysFiltered(ctx, spec.keyPrefix+">")
	if err != nil {
		return stats, err
	}
	defer func() {
		if err := lister.Stop(); err != nil {
			slog.WarnContext(ctx, "failed to stop key lister", "object_type", objectType, "error", err)
		}
	}()

	for key := range lister.Keys() {
		if err := ctx.Err(); err != nil {
			return stats, err
		}
		id := strings.TrimPrefix(key, spec.keyPrefix)
		if id == "" || id == key {
			continue
		}
		stats.scanned++

		markerKey := spec.markerPrefix + id
		reason := "no success marker"
		entry, err := kvMappings.Get(ctx, markerKey)
		switch {
		case err == nil && string(entry.Value()) != tombstoneMarker:
			stats.withMarker++
			continue
		case err == nil:
			// A delete tombstone is only right while the record is deleted. A
			// restore replaces it with the live marker, so a live record that still
			// has it had its restore event dropped.
			deleted, derr := recordIsSoftDeleted(ctx, kv, key)
			if derr != nil {
				slog.ErrorContext(ctx, "failed to read record behind a delete tombstone", "key", key, "marker", markerKey, "error", derr)
				stats.failed++
				continue
			}
			if deleted {
				stats.withMarker++
				continue
			}
			reason = "live record with a delete tombstone"
		case errors.Is(err, jetstream.ErrKeyNotFound):
			// No marker. Before calling the record missing, make sure it is a record the
			// consumer can process: one that does not decode would be rejected again and
			// stay unindexed, however many times it is re-put.
			if _, rerr := readRecord(ctx, kv, key); rerr != nil {
				if errors.Is(rerr, jetstream.ErrKeyNotFound) {
					stats.notFound++
				} else {
					slog.ErrorContext(ctx, "record with no success marker cannot be read or decoded, not re-putting it", "key", key, "marker", markerKey, "error", rerr)
					stats.failed++
				}
				continue
			}
		default:
			slog.ErrorContext(ctx, "failed to read success marker", "key", key, "marker", markerKey, "error", err)
			stats.failed++
			continue
		}

		stats.missing++
		if !reindex {
			slog.InfoContext(ctx, "[dry-run] would re-put", "key", key, "marker", markerKey, "reason", reason)
			continue
		}
		slog.InfoContext(ctx, "re-putting", "key", key, "marker", markerKey, "reason", reason)

		switch reputEntryChecked(ctx, kv, key, checkDecodable) {
		case reputProcessed:
			stats.processed++
		case reputNotFound:
			stats.notFound++
		case reputSuperseded:
			stats.superseded++
		case reputFailed:
			stats.failed++
		}
	}

	// The key channel also closes when ctx is cancelled mid-listing, which would
	// otherwise report a partial scan as complete.
	return stats, ctx.Err()
}

// runMissing is the -missing-only counterpart of run. It needs only NATS: it
// compares the v1-objects and v1-mappings buckets and never queries OpenSearch.
func runMissing(ctx context.Context, nc *nats.Conn, requestedTypes []string, reindex bool) int {
	js, err := jetstream.New(nc)
	if err != nil {
		slog.ErrorContext(ctx, "failed to create JetStream context", "error", err)
		return 1
	}

	kv, err := js.KeyValue(ctx, kvBucketName)
	if err != nil {
		slog.ErrorContext(ctx, "failed to bind to KV bucket", "bucket", kvBucketName, "error", err)
		return 1
	}

	kvMappings, err := js.KeyValue(ctx, kvMappingsBucketName)
	if err != nil {
		slog.ErrorContext(ctx, "failed to bind to KV bucket", "bucket", kvMappingsBucketName, "error", err)
		return 1
	}

	var total missingStats
	for _, objectType := range requestedTypes {
		objectType = strings.TrimSpace(objectType)
		slog.InfoContext(ctx, "scanning object type for missing success markers", "object_type", objectType)

		var stats missingStats
		for _, spec := range missingMarkerConfig[objectType] {
			specStats, err := findMissing(ctx, kv, kvMappings, objectType, spec, reindex)
			if err != nil {
				slog.ErrorContext(ctx, "fatal error scanning type", "object_type", objectType, "key_prefix", spec.keyPrefix, "error", err)
				return 1
			}
			stats.add(specStats)
		}

		slog.InfoContext(ctx, "finished object type",
			"object_type", objectType,
			"scanned", stats.scanned,
			"with_marker", stats.withMarker,
			"missing", stats.missing,
			"processed", stats.processed,
			"failed", stats.failed,
			"not_found", stats.notFound,
			"superseded", stats.superseded,
		)
		if stats.scanned > 0 && stats.withMarker == 0 {
			slog.WarnContext(ctx, "no key of this type has a success marker; the marker mapping for this type may be wrong, check a few keys by hand before trusting the result",
				"object_type", objectType)
		}

		total.add(stats)
	}

	slog.InfoContext(ctx, "reindex_meetings missing-only complete",
		"reindex", reindex,
		"scanned", total.scanned,
		"with_marker", total.withMarker,
		"missing", total.missing,
		"processed", total.processed,
		"failed", total.failed,
		"not_found", total.notFound,
		"superseded", total.superseded,
	)

	if total.failed > 0 {
		return 1
	}
	return 0
}
