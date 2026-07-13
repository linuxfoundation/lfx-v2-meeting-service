// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// backfill_participant_mappings migrates legacy mapping records (the old "1" sentinel
// and pipe-delimited values) for registrants, invitees, and attendees to the new JSON
// format {"uid":"...","username":"...","meeting_id":"..."}.
//
// Without the JSON format the handler cannot detect username changes, which means
// stale FGA access is never revoked when a user's username is cleared or replaced.
//
// The script scans three key families in the v1-mappings KV bucket:
//
//	v1_meeting_registrants.*
//	v1_past_meeting_invitees.*
//	v1_past_meeting_attendees.*
//
// For each entry whose value does NOT start with '{' (i.e. it is not already JSON),
// the script looks up the corresponding object in the v1-objects KV bucket, extracts
// uid / username / meetingID, and writes the JSON mapping value back.
//
// Usage:
//
//	NATS_URL=nats://localhost:4222 go run ./scripts/backfill_participant_mappings/
//	NATS_URL=nats://localhost:4222 go run ./scripts/backfill_participant_mappings/ -apply
//	NATS_URL=nats://localhost:4222 go run ./scripts/backfill_participant_mappings/ -apply -workers 50
//
// Flags:
//
//	-apply    Write updated mapping values (default: dry-run, logs only)
//	-workers  Number of concurrent workers for object-lookup + mapping-write (default: 20)
//
// Build:
//
//	go build -o scripts/backfill_participant_mappings/bin/backfill_participant_mappings \
//	  ./scripts/backfill_participant_mappings/
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/vmihailenco/msgpack/v5"
)

const (
	kvObjectsBucket  = "v1-objects"
	kvMappingsBucket = "v1-mappings"
)

// mappingConfig describes how to backfill one family of mapping keys.
type mappingConfig struct {
	// mappingPrefix is the key prefix in v1-mappings (without trailing dot).
	mappingPrefix string
	// objectPrefix is the key prefix in v1-objects (including trailing dot).
	objectPrefix string
	// usernameField is the JSON field that holds the username in the v1 object.
	usernameField string
	// meetingIDField is the JSON field that holds the meeting / meeting-occurrence ID.
	meetingIDField string
}

var configs = []mappingConfig{
	{
		mappingPrefix:  "v1_meeting_registrants",
		objectPrefix:   "itx-zoom-meetings-registrants-v2.",
		usernameField:  "username",
		meetingIDField: "meeting_id",
	},
	{
		mappingPrefix:  "v1_past_meeting_invitees",
		objectPrefix:   "itx-zoom-past-meetings-invitees.",
		usernameField:  "lf_sso",
		meetingIDField: "meeting_and_occurrence_id",
	},
	{
		mappingPrefix:  "v1_past_meeting_attendees",
		objectPrefix:   "itx-zoom-past-meetings-attendees.",
		usernameField:  "lf_sso",
		meetingIDField: "meeting_and_occurrence_id",
	},
}

// legacyEntry holds the data collected during the watch scan phase.
type legacyEntry struct {
	mappingKey   string
	currentValue string
	uid          string
}

func main() {
	apply := flag.Bool("apply", false, "write updated mapping values (default: dry-run, logs only)")
	workers := flag.Int("workers", 20, "number of concurrent workers for object-lookup + mapping-write")
	flag.Parse()

	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = nats.DefaultURL
	}

	ctx := context.Background()

	slog.InfoContext(ctx, "backfill_participant_mappings starting",
		"nats_url", natsURL,
		"apply", *apply,
		"workers", *workers,
	)

	nc, err := nats.Connect(natsURL,
		nats.Timeout(60*time.Second),
		nats.MaxReconnects(5),
		nats.ReconnectWait(2*time.Second),
	)
	if err != nil {
		slog.ErrorContext(ctx, "failed to connect to NATS", "error", err)
		os.Exit(1)
	}

	exitCode := run(ctx, nc, *apply, *workers)
	nc.Close()
	os.Exit(exitCode)
}

func run(ctx context.Context, nc *nats.Conn, apply bool, workers int) int {
	js, err := jetstream.New(nc)
	if err != nil {
		slog.ErrorContext(ctx, "failed to create JetStream context", "error", err)
		return 1
	}

	objectsKV, err := js.KeyValue(ctx, kvObjectsBucket)
	if err != nil {
		slog.ErrorContext(ctx, "failed to bind to KV bucket", "bucket", kvObjectsBucket, "error", err)
		return 1
	}

	mappingsKV, err := js.KeyValue(ctx, kvMappingsBucket)
	if err != nil {
		slog.ErrorContext(ctx, "failed to bind to KV bucket", "bucket", kvMappingsBucket, "error", err)
		return 1
	}

	var totalUpdated, totalSkipped, totalFailed, totalNotFound int

	for _, cfg := range configs {
		updated, skipped, failed, notFound, err := backfillType(ctx, objectsKV, mappingsKV, cfg, apply, workers)
		if err != nil {
			slog.ErrorContext(ctx, "fatal error during backfill", "mapping_prefix", cfg.mappingPrefix, "error", err)
			return 1
		}

		slog.InfoContext(ctx, "finished mapping type",
			"mapping_prefix", cfg.mappingPrefix,
			"updated", updated,
			"skipped", skipped,
			"failed", failed,
			"not_found", notFound,
		)

		totalUpdated += updated
		totalSkipped += skipped
		totalFailed += failed
		totalNotFound += notFound
	}

	slog.InfoContext(ctx, "backfill_participant_mappings complete",
		"updated", totalUpdated,
		"skipped", totalSkipped,
		"failed", totalFailed,
		"not_found", totalNotFound,
	)

	if totalFailed > 0 {
		return 1
	}
	return 0
}

// backfillType runs in two phases:
//  1. Scan: drain the KV watch to collect all legacy (non-JSON) entries — fast,
//     NATS streams them all to the client in one shot.
//  2. Process: fan the legacy entries out to `workers` goroutines, each doing
//     Get(v1 object) + Put(mapping) concurrently.
func backfillType(
	ctx context.Context,
	objectsKV, mappingsKV jetstream.KeyValue,
	cfg mappingConfig,
	apply bool,
	workers int,
) (updated, skipped, failed, notFound int, err error) {
	// --- Phase 1: scan ---
	// Give the Watch consumer creation and initial-value delivery a long deadline.
	// The nats.go jetstream package applies an internal 5-second timeout when the
	// context has no deadline; overriding it with an explicit one prevents that.
	watchKey := cfg.mappingPrefix + ".>"
	watchCtx, watchCancel := context.WithTimeout(ctx, 10*time.Minute)
	defer watchCancel()
	watcher, watchErr := mappingsKV.Watch(watchCtx, watchKey, jetstream.IgnoreDeletes())
	if watchErr != nil {
		return 0, 0, 0, 0, fmt.Errorf("watch %q: %w", watchKey, watchErr)
	}

	var legacy []legacyEntry
	for entry := range watcher.Updates() {
		if entry == nil {
			break // nil = end of initial values
		}
		currentValue := string(entry.Value())
		// Skip already-JSON entries and tombstones. Tombstones are written as a
		// regular Put with value "!del" (not a NATS-level delete), so IgnoreDeletes()
		// alone does not filter them out.
		if strings.HasPrefix(currentValue, "{") || currentValue == "!del" {
			skipped++
			continue
		}
		uid := strings.TrimPrefix(entry.Key(), cfg.mappingPrefix+".")
		if uid == "" || uid == entry.Key() {
			slog.WarnContext(ctx, "unexpected mapping key format, skipping", "key", entry.Key())
			skipped++
			continue
		}
		legacy = append(legacy, legacyEntry{
			mappingKey:   entry.Key(),
			currentValue: currentValue,
			uid:          uid,
		})
	}
	watcher.Stop()

	slog.InfoContext(ctx, "scan complete",
		"mapping_prefix", cfg.mappingPrefix,
		"legacy_count", len(legacy),
		"already_json", skipped,
	)

	if len(legacy) == 0 {
		return 0, skipped, 0, 0, nil
	}

	// --- Phase 2: concurrent processing ---
	total := len(legacy)
	var (
		atomicDone         atomic.Int64
		atomicUpdated      atomic.Int64
		atomicWithUsername atomic.Int64
		atomicFailed       atomic.Int64
		atomicNotFound     atomic.Int64
	)

	work := make(chan legacyEntry, total)
	for _, e := range legacy {
		work <- e
	}
	close(work)

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for e := range work {
				result, hasUsername := processEntry(ctx, objectsKV, mappingsKV, cfg, e, apply)
				switch result {
				case resultUpdated:
					atomicUpdated.Add(1)
					if hasUsername {
						atomicWithUsername.Add(1)
					}
				case resultFailed:
					atomicFailed.Add(1)
				case resultNotFound:
					atomicNotFound.Add(1)
				}
				done := atomicDone.Add(1)
				slog.InfoContext(ctx, "progress",
					"mapping_prefix", cfg.mappingPrefix,
					"progress", fmt.Sprintf("%d/%d", done, total),
					"with_username", atomicWithUsername.Load(),
				)
			}
		}()
	}
	wg.Wait()

	return int(atomicUpdated.Load()), skipped, int(atomicFailed.Load()), int(atomicNotFound.Load()), nil
}

type entryResult int

const (
	resultUpdated  entryResult = iota
	resultFailed   entryResult = iota
	resultNotFound entryResult = iota
)

// processEntry looks up the v1 object for e, builds the JSON mapping value, and
// writes it (when apply=true). Returns the result and whether the username was non-empty.
func processEntry(
	ctx context.Context,
	objectsKV, mappingsKV jetstream.KeyValue,
	cfg mappingConfig,
	e legacyEntry,
	apply bool,
) (entryResult, bool) {
	objectKey := cfg.objectPrefix + e.uid
	objectEntry, getErr := objectsKV.Get(ctx, objectKey)
	if getErr != nil {
		if errors.Is(getErr, jetstream.ErrKeyNotFound) {
			slog.WarnContext(ctx, "v1 object not found for mapping key",
				"mapping_key", e.mappingKey,
				"object_key", objectKey,
			)
			return resultNotFound, false
		}
		slog.ErrorContext(ctx, "failed to get v1 object",
			"mapping_key", e.mappingKey,
			"object_key", objectKey,
			"error", getErr,
		)
		return resultFailed, false
	}

	objectData, decErr := decodeData(objectEntry.Value())
	if decErr != nil {
		slog.ErrorContext(ctx, "failed to decode v1 object",
			"mapping_key", e.mappingKey,
			"object_key", objectKey,
			"error", decErr,
		)
		return resultFailed, false
	}

	username := getString(objectData, cfg.usernameField)
	meetingID := getString(objectData, cfg.meetingIDField)
	newValue := buildMappingValue(e.uid, username, meetingID)

	if !apply {
		slog.InfoContext(ctx, "[dry-run] would update mapping",
			"mapping_key", e.mappingKey,
			"old_value", e.currentValue,
			"new_value", newValue,
		)
		return resultUpdated, username != ""
	}

	if _, putErr := mappingsKV.Put(ctx, e.mappingKey, []byte(newValue)); putErr != nil {
		slog.ErrorContext(ctx, "failed to write updated mapping",
			"mapping_key", e.mappingKey,
			"error", putErr,
		)
		return resultFailed, false
	}

	slog.InfoContext(ctx, "updated mapping",
		"mapping_key", e.mappingKey,
		"old_value", e.currentValue,
		"new_value", newValue,
	)
	return resultUpdated, username != ""
}

// registrantMappingData is the JSON structure written to v1-mappings.
// Must stay in sync with the same type in cmd/meeting-api/eventing/registrant_event_handler.go.
type registrantMappingData struct {
	UID       string `json:"uid"`
	Username  string `json:"username"`
	MeetingID string `json:"meeting_id"`
}

func buildMappingValue(uid, username, meetingID string) string {
	b, _ := json.Marshal(registrantMappingData{UID: uid, Username: username, MeetingID: meetingID})
	return string(b)
}

// decodeData attempts to parse v1 object bytes as JSON, then as MessagePack.
// Matches the logic in cmd/meeting-api/eventing/kv_handler.go.
func decodeData(data []byte) (map[string]any, error) {
	var result map[string]any
	if err := json.Unmarshal(data, &result); err == nil {
		return result, nil
	}
	if err := msgpack.Unmarshal(data, &result); err == nil {
		return result, nil
	}
	return nil, json.Unmarshal(data, &result)
}

func getString(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
