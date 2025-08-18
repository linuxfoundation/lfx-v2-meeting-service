// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// Package logging contains the logging functionality for the meeting service.
package logging

import (
	"context"
	"log"
	"log/slog"
	"os"
)

type ctxKey string

// Public constants
const (
	ErrKey = "error"
)

// Private constants
const (
	slogFields      ctxKey = "slog_fields"
	logLevelDefault        = slog.LevelDebug

	// Log levels
	debug = "debug"
	warn  = "warn"
	err   = "error"
	info  = "info"

	// Log field for critical errors.
	// TODO: we will want logs with this field set to alert the team to take action.
	priorityCritical = "critical"
)

type contextHandler struct {
	slog.Handler
}

// Handle adds contextual attributes to the Record before calling the underlying handler
func (h contextHandler) Handle(ctx context.Context, r slog.Record) error {
	if attrs, ok := ctx.Value(slogFields).([]slog.Attr); ok {
		for _, v := range attrs {
			r.AddAttrs(v)
		}
	}

	return h.Handler.Handle(ctx, r)
}

// AppendCtx adds an slog attribute to the provided context so that it will be
// included in any Record created with such context
func AppendCtx(parent context.Context, attr slog.Attr) context.Context {
	if parent == nil {
		parent = context.Background()
	}

	if v, ok := parent.Value(slogFields).([]slog.Attr); ok {
		v = append(v, attr)
		return context.WithValue(parent, slogFields, v)
	}

	v := []slog.Attr{}
	v = append(v, attr)
	return context.WithValue(parent, slogFields, v)
}

// InitStructureLogConfig sets the structured log behavior
func InitStructureLogConfig() slog.Handler {
	logOptions := &slog.HandlerOptions{}
	var h slog.Handler

	// Configure log level
	logLevel := os.Getenv("LOG_LEVEL")
	switch logLevel {
	case debug:
		logOptions.Level = slog.LevelDebug
	case warn:
		logOptions.Level = slog.LevelWarn
	case err:
		logOptions.Level = slog.LevelError
	case info:
		logOptions.Level = slog.LevelInfo
	default:
		logOptions.Level = logLevelDefault
	}

	// Configure source information
	addSource := os.Getenv("LOG_ADD_SOURCE")
	logOptions.AddSource = addSource == "true" || addSource == "t" || addSource == "1"

	h = slog.NewJSONHandler(os.Stdout, logOptions)
	log.SetFlags(log.Llongfile)
	logger := contextHandler{h}
	slog.SetDefault(slog.New(logger))

	slog.Info("log config",
		"logLevel", logOptions.Level,
		"addSource", logOptions.AddSource,
	)

	return h
}

// Priority creates a slog.Attr for error priority classification
func Priority(level string) slog.Attr {
	return slog.String("priority", level)
}

// PriorityCritical creates a slog.Attr for critical errors
// this is used to identify critical errors in the logs
// the ones that should be escalated to the team
func PriorityCritical() slog.Attr {
	return Priority(priorityCritical)
}
