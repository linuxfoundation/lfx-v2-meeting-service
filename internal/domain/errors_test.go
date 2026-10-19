// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package domain

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrorType_String(t *testing.T) {
	tests := []struct {
		name string
		t    ErrorType
		want string
	}{
		{"validation", ErrorTypeValidation, "validation"},
		{"forbidden", ErrorTypeForbidden, "forbidden"},
		{"not found", ErrorTypeNotFound, "not_found"},
		{"conflict", ErrorTypeConflict, "conflict"},
		{"internal", ErrorTypeInternal, "internal"},
		{"unavailable", ErrorTypeUnavailable, "unavailable"},
		{"unmapped value falls back to internal", ErrorType(99), "internal"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.t.String())
		})
	}
}

func TestErrEmailNotSynced(t *testing.T) {
	t.Run("detectable via errors.Is when wrapped in NewUnavailableError", func(t *testing.T) {
		err := NewUnavailableError("email not yet available", ErrEmailNotSynced)
		assert.ErrorIs(t, err, ErrEmailNotSynced)
		assert.Equal(t, ErrorTypeUnavailable, GetErrorType(err))
	})

	t.Run("not detected on an unavailable error without the sentinel", func(t *testing.T) {
		err := NewUnavailableError("upstream unavailable")
		assert.NotErrorIs(t, err, ErrEmailNotSynced)
	})
}

func TestDomainError_LogValue(t *testing.T) {
	logErr := func(err error) string {
		var buf bytes.Buffer
		slog.New(slog.NewJSONHandler(&buf, nil)).Warn("failed", "error", err)
		return buf.String()
	}

	t.Run("logs LogMessage instead of the detailed message when set", func(t *testing.T) {
		err := NewConflictError("email alice@example.com already exists")
		err.LogMessage = "user-service returned HTTP 409 (conflict)"

		out := logErr(err)
		assert.Contains(t, out, "user-service returned HTTP 409 (conflict)")
		assert.NotContains(t, out, "alice@example.com")
		// The caller-facing text is unchanged.
		assert.Equal(t, "email alice@example.com already exists", err.Error())
	})

	t.Run("logs the full error text when LogMessage is unset", func(t *testing.T) {
		out := logErr(NewValidationError("bad input"))
		assert.Contains(t, out, `"error":"bad input"`)
	})
}
