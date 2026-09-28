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

// buildMapWrappedNestedFixarrays wraps `depth` nested fixarray(1) containers
// as the value of a single-entry fixmap keyed by "k":
//
//	fixmap(1){ fixstr("k"): fixarray(1){ fixarray(1){ … nil } } }
//
// This exercises the vulnerable decode path: msgpack.Unmarshal accepts the
// root map format without a type error (result is map[string]any), then
// recurses into the nested arrays.  A root-only array payload would be
// rejected immediately by DecodeMap before any recursion.
func buildMapWrappedNestedFixarrays(depth int) []byte {
	header := []byte{0x81, 0xa1, 0x6b} // fixmap(1), fixstr("k")
	return append(header, buildNestedFixarrays(depth)...)
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

	t.Run("rejects payload at first rejected depth (msgpackMaxNestingDepth containers)", func(t *testing.T) {
		// The scanner rejects when len(remaining) >= msgpackMaxNestingDepth at
		// push time.  len(remaining) starts at 1 (root sentinel) and grows by 1
		// per nested container, so the msgpackMaxNestingDepth-th container is the
		// first one that triggers the rejection.
		payload := buildNestedFixarrays(msgpackMaxNestingDepth)
		err := checkMsgpackNestingDepth(payload)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "nesting depth")
	})

	t.Run("rejects payload one beyond rejection threshold", func(t *testing.T) {
		// Also rejected — one past the first rejected depth.
		payload := buildNestedFixarrays(msgpackMaxNestingDepth + 1)
		err := checkMsgpackNestingDepth(payload)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "nesting depth")
	})

	t.Run("accepts max accepted depth (msgpackMaxNestingDepth-1 containers)", func(t *testing.T) {
		// The deepest accepted payload: msgpackMaxNestingDepth-1 nested
		// fixarray(1) containers.  len(remaining) grows to msgpackMaxNestingDepth
		// after the last push (63 < 64), so the check is not triggered.
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

	t.Run("rejects truncated container with huge declared entry count", func(t *testing.T) {
		// A 5-byte map32 header claiming ~2 billion entries followed by no
		// actual child data.  Without the truncation check the scanner returns
		// nil and msgpack.Unmarshal tries to make(map[string]interface{}, 2B)
		// exhausting process memory.  With the check, len(remaining) != 0 after
		// the loop and the payload is rejected.
		//
		// Bytes: 0xdf (map32) + 4-byte big-endian count 0x7fffffff (~2B entries)
		payload := []byte{0xdf, 0x7f, 0xff, 0xff, 0xff}
		err := checkMsgpackNestingDepth(payload)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "truncated")
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
		// Security regression test.  The payload is a fixmap(1) keyed by "k"
		// whose value is a deeply nested fixarray(1) chain:
		//
		//   fixmap(1){ "k": fixarray(1){ fixarray(1){ … nil } } }
		//
		// A root-only array is rejected by msgpack.Unmarshal immediately (wrong
		// type for map[string]any) before any recursion — it would not exercise
		// the depth guard.  The map wrapper causes Unmarshal to begin decoding a
		// map (no type error), then recurse into the nested arrays.  Without the
		// depth guard this exhausts the goroutine stack and kills the process.
		payload := buildMapWrappedNestedFixarrays(msgpackMaxNestingDepth + 100)

		_, err := decodeData(payload)
		require.Error(t, err, "decodeData must return an error for a deeply-nested msgpack payload; the process must not crash")
	})

	t.Run("returns error for invalid data", func(t *testing.T) {
		_, err := decodeData([]byte("not json or msgpack \xff\xfe"))
		assert.Error(t, err)
	})
}
