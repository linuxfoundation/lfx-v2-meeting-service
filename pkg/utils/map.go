// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package utils

import "fmt"

// GetInt coerces an interface{} value to int.
func GetInt(val interface{}) int {
	switch v := val.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case string:
		var i int
		fmt.Sscanf(v, "%d", &i)
		return i
	default:
		return 0
	}
}

// GetBool coerces an interface{} value to bool.
func GetBool(val interface{}) bool {
	switch v := val.(type) {
	case bool:
		return v
	case string:
		return v == "true" || v == "1"
	case int:
		return v != 0
	case float64:
		return v != 0
	default:
		return false
	}
}

// GetString coerces an interface{} value to string.
func GetString(val interface{}) string {
	if val == nil {
		return ""
	}
	if s, ok := val.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", val)
}

// GetStringFromMap returns the string value for key from m, or "" if absent.
func GetStringFromMap(m map[string]interface{}, key string) string {
	if val, ok := m[key]; ok {
		return GetString(val)
	}
	return ""
}

// GetStringSliceFromMap returns a []string for key from m, or nil if absent.
func GetStringSliceFromMap(m map[string]interface{}, key string) []string {
	if val, ok := m[key]; ok {
		if slice, ok := val.([]interface{}); ok {
			result := make([]string, 0, len(slice))
			for _, item := range slice {
				result = append(result, GetString(item))
			}
			return result
		}
		if slice, ok := val.([]string); ok {
			return slice
		}
	}
	return nil
}
