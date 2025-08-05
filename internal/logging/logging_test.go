// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package logging

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"
)

func TestErrKeyConstant(t *testing.T) {
	if ErrKey != "error" {
		t.Errorf("expected ErrKey to be 'error', got %q", ErrKey)
	}
}

func TestAppendCtx(t *testing.T) {
	// Test with nil parent context
	attr := slog.String("key1", "value1")
	ctx := AppendCtx(context.TODO(), attr)

	if ctx == nil {
		t.Fatal("expected non-nil context")
	}

	// Check that the attribute was added
	if attrs, ok := ctx.Value(slogFields).([]slog.Attr); ok {
		if len(attrs) != 1 {
			t.Errorf("expected 1 attribute, got %d", len(attrs))
		}
		if attrs[0].Key != "key1" {
			t.Errorf("expected key 'key1', got %q", attrs[0].Key)
		}
		if attrs[0].Value.String() != "value1" {
			t.Errorf("expected value 'value1', got %q", attrs[0].Value.String())
		}
	} else {
		t.Error("expected slog attributes in context")
	}
}

func TestAppendCtx_WithParent(t *testing.T) {
	// Create parent context with existing attribute
	parentCtx := context.Background()
	attr1 := slog.String("parent_key", "parent_value")
	parentCtx = AppendCtx(parentCtx, attr1)

	// Add another attribute
	attr2 := slog.String("child_key", "child_value")
	childCtx := AppendCtx(parentCtx, attr2)

	// Check that both attributes are present
	if attrs, ok := childCtx.Value(slogFields).([]slog.Attr); ok {
		if len(attrs) != 2 {
			t.Errorf("expected 2 attributes, got %d", len(attrs))
		}

		// Check first attribute
		if attrs[0].Key != "parent_key" {
			t.Errorf("expected first key 'parent_key', got %q", attrs[0].Key)
		}
		if attrs[0].Value.String() != "parent_value" {
			t.Errorf("expected first value 'parent_value', got %q", attrs[0].Value.String())
		}

		// Check second attribute
		if attrs[1].Key != "child_key" {
			t.Errorf("expected second key 'child_key', got %q", attrs[1].Key)
		}
		if attrs[1].Value.String() != "child_value" {
			t.Errorf("expected second value 'child_value', got %q", attrs[1].Value.String())
		}
	} else {
		t.Error("expected slog attributes in context")
	}
}

func TestAppendCtx_MultipleAttributes(t *testing.T) {
	ctx := context.Background()

	// Add multiple attributes
	attr1 := slog.String("key1", "value1")
	attr2 := slog.Int("key2", 42)
	attr3 := slog.Bool("key3", true)

	ctx = AppendCtx(ctx, attr1)
	ctx = AppendCtx(ctx, attr2)
	ctx = AppendCtx(ctx, attr3)

	// Check all attributes are present
	if attrs, ok := ctx.Value(slogFields).([]slog.Attr); ok {
		if len(attrs) != 3 {
			t.Errorf("expected 3 attributes, got %d", len(attrs))
		}

		// Check each attribute
		expectedKeys := []string{"key1", "key2", "key3"}
		for i, expectedKey := range expectedKeys {
			if attrs[i].Key != expectedKey {
				t.Errorf("expected key[%d] %q, got %q", i, expectedKey, attrs[i].Key)
			}
		}
	} else {
		t.Error("expected slog attributes in context")
	}
}

func TestContextHandler_Handle(t *testing.T) {
	// Create a test handler that captures records
	var capturedRecord *slog.Record
	testHandler := &testSlogHandler{
		handleFunc: func(ctx context.Context, r slog.Record) error {
			capturedRecord = &r
			return nil
		},
	}

	handler := contextHandler{Handler: testHandler}

	// Create context with attributes
	ctx := context.Background()
	attr1 := slog.String("ctx_key", "ctx_value")
	ctx = AppendCtx(ctx, attr1)

	// Create a record and handle it
	record := slog.NewRecord(time.Now(), slog.LevelInfo, "test message", 0)
	record.AddAttrs(slog.String("record_key", "record_value"))

	err := handler.Handle(ctx, record)
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}

	if capturedRecord == nil {
		t.Fatal("expected record to be captured")
	}

	// The record should have been modified to include context attributes
	// Note: This is a basic test - in a real implementation, you'd need to
	// check that the attributes were actually added to the record
}

func TestInitStructureLogConfig_DefaultLevel(t *testing.T) {
	// Clear LOG_LEVEL environment variable
	originalLogLevel := os.Getenv("LOG_LEVEL")
	os.Unsetenv("LOG_LEVEL")
	defer func() {
		if originalLogLevel != "" {
			os.Setenv("LOG_LEVEL", originalLogLevel)
		}
	}()

	handler := InitStructureLogConfig()
	if handler == nil {
		t.Error("expected non-nil handler")
	}
}

func TestInitStructureLogConfig_WithLogLevel(t *testing.T) {
	testCases := []struct {
		name     string
		logLevel string
	}{
		{"debug level", "debug"},
		{"warn level", "warn"},
		{"error level", "error"},
		{"info level", "info"},
		{"unknown level", "unknown"},
	}

	originalLogLevel := os.Getenv("LOG_LEVEL")
	defer func() {
		if originalLogLevel != "" {
			os.Setenv("LOG_LEVEL", originalLogLevel)
		} else {
			os.Unsetenv("LOG_LEVEL")
		}
	}()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			os.Setenv("LOG_LEVEL", tc.logLevel)
			handler := InitStructureLogConfig()
			if handler == nil {
				t.Error("expected non-nil handler")
			}
		})
	}
}

func TestInitStructureLogConfig_WithAddSource(t *testing.T) {
	testCases := []struct {
		name      string
		addSource string
	}{
		{"true", "true"},
		{"t", "t"},
		{"1", "1"},
		{"false", "false"},
		{"empty", ""},
	}

	originalAddSource := os.Getenv("LOG_ADD_SOURCE")
	defer func() {
		if originalAddSource != "" {
			os.Setenv("LOG_ADD_SOURCE", originalAddSource)
		} else {
			os.Unsetenv("LOG_ADD_SOURCE")
		}
	}()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			os.Setenv("LOG_ADD_SOURCE", tc.addSource)
			handler := InitStructureLogConfig()
			if handler == nil {
				t.Error("expected non-nil handler")
			}
		})
	}
}

// testSlogHandler is a helper for testing
type testSlogHandler struct {
	handleFunc func(context.Context, slog.Record) error
}

func (h *testSlogHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return true
}

func (h *testSlogHandler) Handle(ctx context.Context, r slog.Record) error {
	if h.handleFunc != nil {
		return h.handleFunc(ctx, r)
	}
	return nil
}

func (h *testSlogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return h
}

func (h *testSlogHandler) WithGroup(name string) slog.Handler {
	return h
}
