// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// migrate_event_state is the explicit, offline legacy-to-Postgres migration.
// It never changes NATS state or enables/stops deployed writers.
package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"time"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/infrastructure/eventstate"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

func run() error {
	apply := flag.Bool("apply", false, "apply schema, import and verify state; all legacy mappings writers must already be stopped")
	flag.Parse()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
	defer cancel()
	store, err := eventstate.Open(ctx, os.Getenv("MEETING_STATE_DATABASE_URL"))
	if err != nil {
		return err
	}
	defer store.Close()
	nc, err := nats.Connect(os.Getenv("NATS_URL"), nats.Timeout(10*time.Second))
	if err != nil {
		return err
	}
	defer nc.Close()
	js, err := jetstream.New(nc)
	if err != nil {
		return err
	}
	count, err := store.Import(ctx, js, *apply)
	if err == nil {
		slog.Info("meeting state migration verified", "entries", count, "applied", *apply)
	}
	return err
}

func main() {
	if err := run(); err != nil {
		// Avoid driver errors that can include connection details or source PII.
		slog.Error("meeting state migration failed; verify cutover state before retrying", "failure_category", eventstate.MigrationFailureCategory(err))
		os.Exit(1)
	}
}
