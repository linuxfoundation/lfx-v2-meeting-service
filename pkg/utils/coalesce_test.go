// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package utils

import (
	"testing"
)

func TestCoalesceString(t *testing.T) {
	tests := []struct {
		name     string
		values   []string
		expected string
	}{
		{
			name:     "returns first non-empty string",
			values:   []string{"", "", "hello", "world"},
			expected: "hello",
		},
		{
			name:     "returns empty string when all empty",
			values:   []string{"", "", ""},
			expected: "",
		},
		{
			name:     "returns empty string when no arguments",
			values:   []string{},
			expected: "",
		},
		{
			name:     "returns first value when non-empty",
			values:   []string{"first", "second", "third"},
			expected: "first",
		},
		{
			name:     "skips empty strings until non-empty found",
			values:   []string{"", "", "", "found", "not-this"},
			expected: "found",
		},
		{
			name:     "single non-empty value",
			values:   []string{"only"},
			expected: "only",
		},
		{
			name:     "single empty value",
			values:   []string{""},
			expected: "",
		},
		{
			name:     "handles spaces as non-empty",
			values:   []string{"", "  ", "hello"},
			expected: "  ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CoalesceString(tt.values...)
			if result != tt.expected {
				t.Errorf("CoalesceString(%v) = %q, expected %q", tt.values, result, tt.expected)
			}
		})
	}
}
