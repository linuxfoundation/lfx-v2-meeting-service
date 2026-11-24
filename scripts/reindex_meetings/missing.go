// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"errors"
	"log/slog"
	"sort"
	"strings"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// markerSpec ties a v1-objects key prefix to the success marker the event
// handler writes to v1-mappings once it has published that record to the
// indexer. The marker key is markerPrefix followed by the same ID that follows
// keyPrefix in the v1-objects key.
type markerSpec struct {
	keyPrefix    string
	markerPrefix string
}

// missingMarkerConfig lists the object types -missing-only can audit. Each
// marker key is the one the corresponding handler in cmd/meeting-api/eventing
// writes after a successful publish. v1_meeting_rsvp and
// v1_past_meeting_participant are not listed: their handlers do not write a
// marker keyed by the v1-objects ID, so a missing record cannot be told apart
// from one that was indexed.
var missingMarkerConfig = map[string]markerSpec{
	"v1_meeting":                 {keyPrefix: "itx-zoom-meetings-v2.", markerPrefix: "v1_meetings."},
	"v1_meeting_registrant":      {keyPrefix: "itx-zoom-meetings-registrants-v2.", markerPrefix: "v1_meeting_registrants."},
	"v1_meeting_attachment":      {keyPrefix: "itx-zoom-meetings-attachments-v2.", markerPrefix: "v1_meeting_attachments."},
	"v1_past_meeting":            {keyPrefix: "itx-zoom-past-meetings.", markerPrefix: "v1_past_meetings."},
	"v1_past_meeting_recording":  {keyPrefix: "itx-zoom-past-meetings-recordings.", markerPrefix: "v1_past_meeting_recordings."},
	"v1_past_meeting_summary":    {keyPrefix: "itx-zoom-past-meetings-summaries.", markerPrefix: "v1_past_meeting_summaries."},
	"v1_past_meeting_attachment": {keyPrefix: "itx-zoom-past-meetings-attachments.", markerPrefix: "v1_past_meeting_attachments."},
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
		id := strings.TrimPrefix(key, spec.keyPrefix)
		if id == "" || id == key {
			continue
		}
		stats.scanned++

		markerKey := spec.markerPrefix + id
		if _, err := kvMappings.Get(ctx, markerKey); err == nil {
			stats.withMarker++
			continue
		} else if !errors.Is(err, jetstream.ErrKeyNotFound) {
			slog.ErrorContext(ctx, "failed to read success marker", "key", key, "marker", markerKey, "error", err)
			stats.failed++
			continue
		}

		stats.missing++
		if !reindex {
			slog.InfoContext(ctx, "[dry-run] no success marker, would re-put", "key", key, "marker", markerKey)
			continue
		}
		slog.InfoContext(ctx, "no success marker, re-putting", "key", key, "marker", markerKey)

		switch reputEntry(ctx, kv, key) {
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

	return stats, nil
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

		stats, err := findMissing(ctx, kv, kvMappings, objectType, missingMarkerConfig[objectType], reindex)
		if err != nil {
			slog.ErrorContext(ctx, "fatal error scanning type", "object_type", objectType, "error", err)
			return 1
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

		total.scanned += stats.scanned
		total.withMarker += stats.withMarker
		total.missing += stats.missing
		total.processed += stats.processed
		total.failed += stats.failed
		total.notFound += stats.notFound
		total.superseded += stats.superseded
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
