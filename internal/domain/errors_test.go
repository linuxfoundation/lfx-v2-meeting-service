// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package domain

import (
	"errors"
	"testing"
)

func TestDomainErrors(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "ErrMeetingNotFound",
			err:      ErrMeetingNotFound,
			expected: "meeting not found",
		},
		{
			name:     "ErrInternal",
			err:      ErrInternal,
			expected: "internal error",
		},
		{
			name:     "ErrRevisionMismatch",
			err:      ErrRevisionMismatch,
			expected: "revision mismatch",
		},
		{
			name:     "ErrUnmarshal",
			err:      ErrUnmarshal,
			expected: "unmarshal error",
		},
		{
			name:     "ErrServiceUnavailable",
			err:      ErrServiceUnavailable,
			expected: "service unavailable",
		},
		{
			name:     "ErrValidationFailed",
			err:      ErrValidationFailed,
			expected: "validation failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Error() != tt.expected {
				t.Errorf("expected error message %q, got %q", tt.expected, tt.err.Error())
			}
		})
	}
}

func TestErrorsAreDistinct(t *testing.T) {
	errorVars := []error{
		ErrMeetingNotFound,
		ErrInternal,
		ErrRevisionMismatch,
		ErrUnmarshal,
		ErrServiceUnavailable,
		ErrValidationFailed,
	}

	for i, err1 := range errorVars {
		for j, err2 := range errorVars {
			if i != j && errors.Is(err1, err2) {
				t.Errorf("errors should be distinct: %v and %v are considered equal", err1, err2)
			}
		}
	}
}
