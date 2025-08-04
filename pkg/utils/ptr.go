// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package utils

// StringPtr converts a string to a pointer to a string.
func StringPtr(s string) *string {
	return &s
}

// StringValue safely dereferences a string pointer, returning empty string if nil.
func StringValue(s *string) string {
	if s != nil {
		return *s
	}
	return ""
}

// BoolPtr converts a bool to a pointer to a bool.
func BoolPtr(b bool) *bool {
	return &b
}

// BoolValue safely dereferences a bool pointer, returning false if nil.
func BoolValue(b *bool) bool {
	if b != nil {
		return *b
	}
	return false
}

// IntPtr converts an int to a pointer to an int.
func IntPtr(i int) *int {
	return &i
}

// IntValue safely dereferences an int pointer, returning 0 if nil.
func IntValue(i *int) int {
	if i != nil {
		return *i
	}
	return 0
}
