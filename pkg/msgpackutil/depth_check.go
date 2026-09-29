// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// Package msgpackutil provides helpers for safe msgpack decoding.
package msgpackutil

import (
	"errors"
	"fmt"
)

// ErrStructural is the sentinel wrapped into every error returned by
// CheckNestingDepth.  Callers can detect a permanent structural failure with
// errors.Is(err, msgpackutil.ErrStructural).
var ErrStructural = errors.New("msgpack structural check failed")

// MaxNestingDepth is the maximum container-nesting depth accepted by
// CheckNestingDepth.  Payloads nested up to and including this depth pass;
// the first rejected depth is MaxNestingDepth+1.
//
// Background: vmihailenco/msgpack v5 decodes into interface{} by recursing
// once per nesting level (DecodeInterface → decodeSlice/decodeMap →
// DecodeInterface).  There is no built-in depth cap.  A crafted ~1 MB
// fixarray(1)-chain payload encodes ~1 M levels in 1 byte each; at a few
// hundred bytes of stack frame per level the goroutine stack exceeds the
// pod's 512 Mi memory limit before Go's 1 GB ceiling is reached.
// runtime.recover() cannot intercept a fatal stack-overflow, so the whole
// process is killed.
//
// Legitimate v1-objects records are flat; 64 is very generous.
const MaxNestingDepth = 64

// CheckNestingDepth iteratively scans raw msgpack bytes and returns a non-nil
// error (wrapping ErrStructural) if any container (array or map) would push
// the nesting depth past MaxNestingDepth.  It allocates only a small
// depth-tracking slice and never recurses, so it is safe to call on untrusted
// input before passing the same bytes to msgpack.Unmarshal.
//
// Also rejects structurally truncated type-header fields (e.g. a str16
// missing its length bytes) and containers whose declared element count
// exceeds what the payload actually contains.  Other decode errors are left to
// msgpack.Unmarshal.
func CheckNestingDepth(data []byte) error {
	// remaining[level] = number of msgpack values still to consume at that
	// level.  We start with 1 root value.
	remaining := make([]int, 1, MaxNestingDepth+1)
	remaining[0] = 1

	i := 0
	for i < len(data) && len(remaining) > 0 {
		b := data[i]
		i++

		var skip int     // raw bytes to skip for this type's inline payload
		var children int // child-value count for containers (0 = leaf / empty container)

		switch {
		// single-byte atoms (no payload)
		case b == 0xc0, b == 0xc2, b == 0xc3: // nil, false, true
		case b <= 0x7f: // positive fixint
		case b >= 0xe0: // negative fixint

		// integers
		case b == 0xcc:
			skip = 1 // uint8
		case b == 0xcd:
			skip = 2 // uint16
		case b == 0xce:
			skip = 4 // uint32
		case b == 0xcf:
			skip = 8 // uint64
		case b == 0xd0:
			skip = 1 // int8
		case b == 0xd1:
			skip = 2 // int16
		case b == 0xd2:
			skip = 4 // int32
		case b == 0xd3:
			skip = 8 // int64

		// floats
		case b == 0xca:
			skip = 4 // float32
		case b == 0xcb:
			skip = 8 // float64

		// str
		case b >= 0xa0 && b <= 0xbf: // fixstr
			skip = int(b & 0x1f)
		case b == 0xd9: // str8
			if i >= len(data) {
				return fmt.Errorf("%w: str8 length byte missing", ErrStructural)
			}
			skip = int(data[i])
			i++
		case b == 0xda: // str16
			if i+2 > len(data) {
				return fmt.Errorf("%w: str16 length bytes missing", ErrStructural)
			}
			skip = int(data[i])<<8 | int(data[i+1])
			i += 2
		case b == 0xdb: // str32
			if i+4 > len(data) {
				return fmt.Errorf("%w: str32 length bytes missing", ErrStructural)
			}
			skip = int(data[i])<<24 | int(data[i+1])<<16 | int(data[i+2])<<8 | int(data[i+3])
			i += 4

		// bin
		case b == 0xc4: // bin8
			if i >= len(data) {
				return fmt.Errorf("%w: bin8 length byte missing", ErrStructural)
			}
			skip = int(data[i])
			i++
		case b == 0xc5: // bin16
			if i+2 > len(data) {
				return fmt.Errorf("%w: bin16 length bytes missing", ErrStructural)
			}
			skip = int(data[i])<<8 | int(data[i+1])
			i += 2
		case b == 0xc6: // bin32
			if i+4 > len(data) {
				return fmt.Errorf("%w: bin32 length bytes missing", ErrStructural)
			}
			skip = int(data[i])<<24 | int(data[i+1])<<16 | int(data[i+2])<<8 | int(data[i+3])
			i += 4

		// ext (type byte + data bytes; type byte already counted in skip)
		case b == 0xd4:
			skip = 2 // fixext1: 1 type + 1 data
		case b == 0xd5:
			skip = 3 // fixext2
		case b == 0xd6:
			skip = 5 // fixext4
		case b == 0xd7:
			skip = 9 // fixext8
		case b == 0xd8:
			skip = 17 // fixext16
		case b == 0xc7: // ext8: length(1) + type(1) + data(length)
			if i >= len(data) {
				return fmt.Errorf("%w: ext8 length byte missing", ErrStructural)
			}
			skip = int(data[i]) + 1 // +1 for type byte
			i++
		case b == 0xc8: // ext16
			if i+2 > len(data) {
				return fmt.Errorf("%w: ext16 length bytes missing", ErrStructural)
			}
			skip = (int(data[i])<<8 | int(data[i+1])) + 1
			i += 2
		case b == 0xc9: // ext32
			if i+4 > len(data) {
				return fmt.Errorf("%w: ext32 length bytes missing", ErrStructural)
			}
			skip = (int(data[i])<<24 | int(data[i+1])<<16 | int(data[i+2])<<8 | int(data[i+3])) + 1
			i += 4

		// arrays
		case b >= 0x90 && b <= 0x9f: // fixarray
			children = int(b & 0x0f)
		case b == 0xdc: // array16
			if i+2 > len(data) {
				return fmt.Errorf("%w: array16 length bytes missing", ErrStructural)
			}
			children = int(data[i])<<8 | int(data[i+1])
			i += 2
		case b == 0xdd: // array32
			if i+4 > len(data) {
				return fmt.Errorf("%w: array32 length bytes missing", ErrStructural)
			}
			children = int(data[i])<<24 | int(data[i+1])<<16 | int(data[i+2])<<8 | int(data[i+3])
			i += 4

		// maps: each entry is a key-value pair → 2 values per entry
		case b >= 0x80 && b <= 0x8f: // fixmap
			children = int(b&0x0f) * 2
		case b == 0xde: // map16
			if i+2 > len(data) {
				return fmt.Errorf("%w: map16 length bytes missing", ErrStructural)
			}
			children = (int(data[i])<<8 | int(data[i+1])) * 2
			i += 2
		case b == 0xdf: // map32
			if i+4 > len(data) {
				return fmt.Errorf("%w: map32 length bytes missing", ErrStructural)
			}
			children = (int(data[i])<<24 | int(data[i+1])<<16 | int(data[i+2])<<8 | int(data[i+3])) * 2
			i += 4

		default:
			return fmt.Errorf("%w: unknown format byte 0x%02x at offset %d", ErrStructural, b, i-1)
		}

		// Advance past inline scalar payload bytes.
		if skip > 0 {
			if i+skip > len(data) {
				return fmt.Errorf("%w: payload truncated at offset %d: need %d more bytes", ErrStructural, i, skip)
			}
			i += skip
		}

		if children > 0 {
			// Non-empty container: push a new depth level.
			if len(remaining) > MaxNestingDepth {
				return fmt.Errorf("%w: nesting depth exceeds limit of %d", ErrStructural, MaxNestingDepth)
			}
			remaining = append(remaining, children)
		} else {
			// Leaf or empty container: one value consumed; close any finished levels.
			for len(remaining) > 0 {
				top := len(remaining) - 1
				remaining[top]--
				if remaining[top] > 0 {
					break
				}
				remaining = remaining[:top] // pop finished level; loop to decrement parent
			}
		}
	}

	// If any container still has declared children remaining the payload was
	// truncated before those children were present.  Reject to avoid passing a
	// truncated buffer to msgpack.Unmarshal, which would try to allocate a
	// collection sized by the declared (attacker-controlled) count.
	if len(remaining) != 0 {
		return fmt.Errorf("%w: %d container level(s) left open (truncated payload)", ErrStructural, len(remaining))
	}

	return nil
}
