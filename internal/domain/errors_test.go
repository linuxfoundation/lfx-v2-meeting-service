// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package domain

import (
	"errors"
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
		assert.True(t, errors.Is(err, ErrEmailNotSynced))
		assert.Equal(t, ErrorTypeUnavailable, GetErrorType(err))
	})

	t.Run("not detected on an unavailable error without the sentinel", func(t *testing.T) {
		err := NewUnavailableError("upstream unavailable")
		assert.False(t, errors.Is(err, ErrEmailNotSynced))
	})
}
