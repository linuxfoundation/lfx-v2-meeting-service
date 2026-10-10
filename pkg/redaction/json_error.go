// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package redaction

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
)

// maxJSONFieldPathLen bounds the struct/field path echoed from a decode error.
// The paths this service's own schemas produce are far shorter; the cap exists so
// a hand-built error cannot set a log line's length.
const maxJSONFieldPathLen = 128

// RedactJSONError describes a json.Unmarshal failure without echoing any byte of
// the document that failed to parse.
//
// Decoding an untrusted document and logging the resulting error forwards
// attacker-chosen text into this service's logs, because several standard library
// errors embed the offending input verbatim:
//
//   - encoding/json builds *json.UnmarshalTypeError with Value "number <literal>",
//     so a number literal of any length decoded into a string field lands whole in
//     err.Error().
//   - A field whose type defines its own UnmarshalJSON has that error returned
//     untouched: (*json.decodeState).addErrorContext only rewrites
//     *json.UnmarshalTypeError. time.Time is the live case here — invite records
//     carry timestamps, and (*time.ParseError).Error() renders the full input with
//     no truncation.
//
// That second case is why this is an allow-list rather than a sanitizer: anything
// that is not a recognised, field-by-field-vetted decode error is reported as a
// fixed string. Only values derived from *our* schema — the Go target type, the
// struct field path, and a byte offset — are ever repeated.
func RedactJSONError(err error) string {
	if err == nil {
		return ""
	}

	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) {
		return fmt.Sprintf("cannot unmarshal JSON %s into %s field %s at byte offset %d",
			boundedJSONValueKind(typeErr.Value),
			boundedJSONGoType(typeErr.Type),
			boundedJSONFieldPath(typeErr.Struct, typeErr.Field),
			typeErr.Offset,
		)
	}

	var syntaxErr *json.SyntaxError
	if errors.As(err, &syntaxErr) {
		// The standard library message is bounded (one strconv.Quote'd byte plus a
		// fixed phrase), but the offset locates the fault just as well without
		// repeating anything from the document.
		return fmt.Sprintf("malformed JSON at byte offset %d", syntaxErr.Offset)
	}

	return "(unrecognised JSON decode error)"
}

// boundedJSONValueKind renders UnmarshalTypeError.Value. encoding/json emits a
// closed set of kind words, except for the "number <literal>" forms, which carry
// the document's own bytes — those are reduced to the kind.
func boundedJSONValueKind(value string) string {
	switch value {
	case "array", "object", "bool", "string", "number", "null":
		return value
	}
	if strings.HasPrefix(value, "number ") {
		return "number"
	}
	return "(unrecognised kind)"
}

// boundedJSONGoType renders the Go type the decode targeted. reflect.Type comes
// from the compiled schema, never from the document; it is nil only when a
// hand-built error omitted it.
func boundedJSONGoType(t reflect.Type) string {
	if t == nil {
		return "(unknown type)"
	}
	return t.String()
}

// boundedJSONFieldPath renders the struct and field path. encoding/json fills both
// from our own type's field names, never from the document's keys — but a type
// with its own UnmarshalJSON may return a hand-built *json.UnmarshalTypeError, so
// the path is allow-listed rather than trusted.
func boundedJSONFieldPath(structName, field string) string {
	path := field
	switch {
	case structName != "" && field != "":
		path = structName + "." + field
	case structName != "":
		path = structName
	}
	if path == "" {
		return "(top level)"
	}
	if len(path) > maxJSONFieldPathLen || !isJSONIdentPath(path) {
		return "(unrecognised field)"
	}
	return path
}

// isJSONIdentPath reports whether s looks like a Go/JSON identifier path, i.e. the
// shape encoding/json builds from a schema's own field names.
func isJSONIdentPath(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z',
			c >= '0' && c <= '9', c == '_', c == '.', c == '-':
		default:
			return false
		}
	}
	return true
}
