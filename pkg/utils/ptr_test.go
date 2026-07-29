// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package utils

import (
	"strconv"
	"testing"
)

func TestStringValue(t *testing.T) {
	// Test with nil pointer
	result := StringValue(nil)
	if result != "" {
		t.Errorf("expected empty string for nil pointer, got %q", result)
	}

	// Test with valid pointer
	tests := []string{
		"",
		"hello",
		"hello world",
		"special chars: !@#$%^&*()",
		"unicode: 你好世界",
	}

	for _, test := range tests {
		t.Run(test, func(t *testing.T) {
			ptr := &test
			result := StringValue(ptr)
			if result != test {
				t.Errorf("expected %q, got %q", test, result)
			}
		})
	}
}

func TestBoolPtr(t *testing.T) {
	tests := []struct {
		name  string
		value bool
	}{
		{"true", true},
		{"false", false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ptr := BoolPtr(test.value)
			if ptr == nil {
				t.Error("expected non-nil pointer")
				return
			}
			if *ptr != test.value {
				t.Errorf("expected %t, got %t", test.value, *ptr)
			}
		})
	}
}

func TestBoolValue(t *testing.T) {
	// Test with nil pointer
	result := BoolValue(nil)
	if result != false {
		t.Errorf("expected false for nil pointer, got %t", result)
	}

	// Test with valid pointers
	tests := []struct {
		name  string
		value bool
	}{
		{"true", true},
		{"false", false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ptr := &test.value
			result := BoolValue(ptr)
			if result != test.value {
				t.Errorf("expected %t, got %t", test.value, result)
			}
		})
	}
}

func TestBoolPtrValueRoundTrip(t *testing.T) {
	tests := []struct {
		name  string
		value bool
	}{
		{"true", true},
		{"false", false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ptr := BoolPtr(test.value)
			result := BoolValue(ptr)
			if result != test.value {
				t.Errorf("round trip failed: expected %t, got %t", test.value, result)
			}
		})
	}
}

func TestIntValue(t *testing.T) {
	// Test with nil pointer
	result := IntValue(nil)
	if result != 0 {
		t.Errorf("expected 0 for nil pointer, got %d", result)
	}

	// Test with valid pointers
	tests := []int{
		0,
		1,
		-1,
		42,
		-42,
		1000000,
		-1000000,
	}

	for _, test := range tests {
		t.Run(strconv.Itoa(test), func(t *testing.T) {
			ptr := &test
			result := IntValue(ptr)
			if result != test {
				t.Errorf("expected %d, got %d", test, result)
			}
		})
	}
}

func TestPointerIndependence(t *testing.T) {
	ptr1 := BoolPtr(true)
	ptr2 := BoolPtr(true)

	// Pointers should be different
	if ptr1 == ptr2 {
		t.Error("expected different pointer addresses")
	}

	// But values should be the same
	if *ptr1 != *ptr2 {
		t.Errorf("expected same values: %t vs %t", *ptr1, *ptr2)
	}

	// Modifying one shouldn't affect the other
	*ptr1 = false
	if *ptr2 == false {
		t.Error("modifying one pointer affected the other")
	}
}

func TestNilSafety(t *testing.T) {
	stringResult := StringValue(nil)
	if stringResult != "" {
		t.Errorf("StringValue(nil) should return empty string, got %q", stringResult)
	}

	boolResult := BoolValue(nil)
	if boolResult != false {
		t.Errorf("BoolValue(nil) should return false, got %t", boolResult)
	}

	intResult := IntValue(nil)
	if intResult != 0 {
		t.Errorf("IntValue(nil) should return 0, got %d", intResult)
	}
}
