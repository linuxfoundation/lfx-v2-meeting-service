// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vmihailenco/msgpack/v5"
)

// buildNestedFixarrays returns a raw msgpack byte slice consisting of `depth`
// nested fixarray(1) containers (0x91) followed by a nil byte (0xc0).
// Each level costs exactly 1 byte, so a depth-N payload is N+1 bytes total.
func buildNestedFixarrays(depth int) []byte {
	payload := make([]byte, depth+1)
	for i := 0; i < depth; i++ {
		payload[i] = 0x91 // fixarray(1)
	}
	payload[depth] = 0xc0 // nil
	return payload
}

// TestCheckMsgpackNestingDepth exercises the iterative depth scanner.
func TestCheckMsgpackNestingDepth(t *testing.T) {
	t.Run("accepts flat map", func(t *testing.T) {
		data, err := msgpack.Marshal(map[string]any{
			"id":    "00000000-0000-0000-0000-000000000001",
			"topic": "Test Meeting",
		})
		require.NoError(t, err)
		assert.NoError(t, checkMsgpackNestingDepth(data))
	})

	t.Run("accepts nested map within limit", func(t *testing.T) {
		// 10 levels of map nesting — well within the 64-level limit.
		var buildNested func(depth int) any
		buildNested = func(depth int) any {
			if depth == 0 {
				return "leaf"
			}
			return map[string]any{"child": buildNested(depth - 1)}
		}
		data, err := msgpack.Marshal(buildNested(10))
		require.NoError(t, err)
		assert.NoError(t, checkMsgpackNestingDepth(data))
	})

	t.Run("rejects payload at exact limit+1 fixarray depth", func(t *testing.T) {
		// One level beyond the limit must be rejected.
		payload := buildNestedFixarrays(msgpackMaxNestingDepth + 1)
		err := checkMsgpackNestingDepth(payload)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "nesting depth")
	})

	t.Run("accepts payload at exact limit depth", func(t *testing.T) {
		// Exactly at the limit must be accepted.
		// msgpackMaxNestingDepth-1 nested fixarrays push the depth to
		// msgpackMaxNestingDepth (root level + N containers = N+1 checks,
		// but the root sentinel counts as 1, so N containers = depth N).
		payload := buildNestedFixarrays(msgpackMaxNestingDepth - 1)
		assert.NoError(t, checkMsgpackNestingDepth(payload))
	})

	t.Run("rejects large deeply-nested fixarray payload", func(t *testing.T) {
		// Simulate a ~1 KB hostile payload (depth >> limit).
		payload := buildNestedFixarrays(msgpackMaxNestingDepth + 1000)
		err := checkMsgpackNestingDepth(payload)
		require.Error(t, err)
	})

	t.Run("accepts empty array", func(t *testing.T) {
		data, err := msgpack.Marshal([]any{})
		require.NoError(t, err)
		assert.NoError(t, checkMsgpackNestingDepth(data))
	})

	t.Run("accepts empty map", func(t *testing.T) {
		data, err := msgpack.Marshal(map[string]any{})
		require.NoError(t, err)
		assert.NoError(t, checkMsgpackNestingDepth(data))
	})

	t.Run("accepts nil", func(t *testing.T) {
		data, err := msgpack.Marshal(nil)
		require.NoError(t, err)
		assert.NoError(t, checkMsgpackNestingDepth(data))
	})

	t.Run("accepts various scalar types", func(t *testing.T) {
		values := []any{
			int64(42),
			float64(3.14),
			true,
			false,
			"hello",
			[]byte("binary"),
		}
		for _, v := range values {
			data, err := msgpack.Marshal(v)
			require.NoError(t, err)
			assert.NoError(t, checkMsgpackNestingDepth(data))
		}
	})
}

// TestDecodeData covers the public decodeData helper used by kvHandler and
// the KV lookup helpers (kv_helpers.go).
func TestDecodeData(t *testing.T) {
	t.Run("decodes valid JSON map", func(t *testing.T) {
		input := map[string]any{"id": "abc", "topic": "hello"}
		data, err := json.Marshal(input)
		require.NoError(t, err)

		result, err := decodeData(data)
		require.NoError(t, err)
		assert.Equal(t, "abc", result["id"])
		assert.Equal(t, "hello", result["topic"])
	})

	t.Run("decodes valid msgpack map", func(t *testing.T) {
		input := map[string]any{
			"id":    "00000000-0000-0000-0000-000000000001",
			"topic": "Test Meeting",
		}
		data, err := msgpack.Marshal(input)
		require.NoError(t, err)

		result, err := decodeData(data)
		require.NoError(t, err)
		assert.Equal(t, "00000000-0000-0000-0000-000000000001", result["id"])
		assert.Equal(t, "Test Meeting", result["topic"])
	})

	t.Run("rejects deeply nested msgpack without crashing", func(t *testing.T) {
		// This is the core security regression test.  A payload of
		// msgpackMaxNestingDepth+100 nested fixarray(1) containers followed by
		// nil exceeds the depth limit and must be rejected with an error.
		// Crucially, the process must NOT crash (no stack overflow).
		payload := buildNestedFixarrays(msgpackMaxNestingDepth + 100)

		_, err := decodeData(payload)
		require.Error(t, err, "decodeData must return an error for a deeply-nested msgpack payload; the process must not crash")
	})

	t.Run("returns error for invalid data", func(t *testing.T) {
		_, err := decodeData([]byte("not json or msgpack \xff\xfe"))
		assert.Error(t, err)
	})
}
