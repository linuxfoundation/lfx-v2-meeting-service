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
//
// Flags:
//
//	-apply  Write updated mapping values (default: dry-run, logs only)
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

func main() {
	apply := flag.Bool("apply", false, "write updated mapping values (default: dry-run, logs only)")
	flag.Parse()

	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = nats.DefaultURL
	}

	ctx := context.Background()

	slog.InfoContext(ctx, "backfill_participant_mappings starting",
		"nats_url", natsURL,
		"apply", *apply,
	)

	nc, err := nats.Connect(natsURL,
		nats.Timeout(10*time.Second),
		nats.MaxReconnects(5),
		nats.ReconnectWait(2*time.Second),
	)
	if err != nil {
		slog.ErrorContext(ctx, "failed to connect to NATS", "error", err)
		os.Exit(1)
	}
	defer nc.Close()

	exitCode := run(ctx, nc, *apply)
	os.Exit(exitCode)
}

func run(ctx context.Context, nc *nats.Conn, apply bool) int {
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
		updated, skipped, failed, notFound, err := backfillType(ctx, objectsKV, mappingsKV, cfg, apply)
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

// backfillType scans all live entries under cfg.mappingPrefix and upgrades any
// non-JSON value to the {"uid","username","meeting_id"} JSON format.
func backfillType(
	ctx context.Context,
	objectsKV, mappingsKV jetstream.KeyValue,
	cfg mappingConfig,
	apply bool,
) (updated, skipped, failed, notFound int, err error) {
	watchKey := cfg.mappingPrefix + ".>"
	watcher, watchErr := mappingsKV.Watch(ctx, watchKey, jetstream.IgnoreDeletes())
	if watchErr != nil {
		return 0, 0, 0, 0, fmt.Errorf("watch %q: %w", watchKey, watchErr)
	}
	defer watcher.Stop()

	for entry := range watcher.Updates() {
		if entry == nil {
			// Nil signals end of initial values; stop here (we don't process live changes).
			break
		}

		mappingKey := entry.Key()
		currentValue := string(entry.Value())

		// Already JSON — nothing to do.
		if strings.HasPrefix(currentValue, "{") {
			skipped++
			continue
		}

		// Derive the UID from the mapping key suffix.
		uid := strings.TrimPrefix(mappingKey, cfg.mappingPrefix+".")
		if uid == "" || uid == mappingKey {
			slog.WarnContext(ctx, "unexpected mapping key format, skipping", "key", mappingKey)
			skipped++
			continue
		}

		// Look up the v1 object to recover username and meeting ID.
		objectKey := cfg.objectPrefix + uid
		objectEntry, getErr := objectsKV.Get(ctx, objectKey)
		if getErr != nil {
			if errors.Is(getErr, jetstream.ErrKeyNotFound) {
				slog.WarnContext(ctx, "v1 object not found for mapping key",
					"mapping_key", mappingKey,
					"object_key", objectKey,
				)
				notFound++
				continue
			}
			slog.ErrorContext(ctx, "failed to get v1 object",
				"mapping_key", mappingKey,
				"object_key", objectKey,
				"error", getErr,
			)
			failed++
			continue
		}

		objectData, decErr := decodeData(objectEntry.Value())
		if decErr != nil {
			slog.ErrorContext(ctx, "failed to decode v1 object",
				"mapping_key", mappingKey,
				"object_key", objectKey,
				"error", decErr,
			)
			failed++
			continue
		}

		username := getString(objectData, cfg.usernameField)
		meetingID := getString(objectData, cfg.meetingIDField)

		newValue := buildMappingValue(uid, username, meetingID)

		if !apply {
			slog.InfoContext(ctx, "[dry-run] would update mapping",
				"mapping_key", mappingKey,
				"old_value", currentValue,
				"new_value", newValue,
			)
			updated++
			continue
		}

		if _, putErr := mappingsKV.Put(ctx, mappingKey, []byte(newValue)); putErr != nil {
			slog.ErrorContext(ctx, "failed to write updated mapping",
				"mapping_key", mappingKey,
				"error", putErr,
			)
			failed++
			continue
		}

		slog.InfoContext(ctx, "updated mapping",
			"mapping_key", mappingKey,
			"old_value", currentValue,
			"new_value", newValue,
		)
		updated++
	}

	return updated, skipped, failed, notFound, nil
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
