// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package redaction

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedactJSONError(t *testing.T) {
	t.Run("describes nothing for a nil error", func(t *testing.T) {
		assert.Empty(t, RedactJSONError(nil))
	})

	t.Run("a huge number literal never reaches the description", func(t *testing.T) {
		// A numeric target is the reachable shape: encoding/json embeds the literal
		// only when the number must be converted (int, float, interface{}, a map
		// key), not when it is simply the wrong kind for a string field. The invite
		// schema exposes this through Invite.ExpirationDays.
		huge := strings.Repeat("9", 100_000)
		var target struct {
			ExpirationDays int `json:"expiration_days"`
		}
		err := json.Unmarshal([]byte(`{"expiration_days": `+huge+`}`), &target)
		require.Error(t, err)
		// Premise: without the guard this error really does carry the literal. If
		// the standard library ever stops doing that, this fails loudly rather
		// than the assertions below passing for the wrong reason.
		require.Contains(t, err.Error(), strings.Repeat("9", 1_000),
			"premise: encoding/json embeds the number literal in the error")

		out := RedactJSONError(err)

		assert.NotContains(t, out, "99999")
		assert.Less(t, len(out), 200, "the payload must not set the log line length")
		assert.Contains(t, out, "expiration_days", "our own field path stays readable")
		assert.Contains(t, out, "int", "the target type stays readable")
		assert.Contains(t, out, "number", "the offending kind stays readable")
	})

	t.Run("a wrong-kind number against a string field is already bounded", func(t *testing.T) {
		// Documents the boundary: this shape never carried the literal, so the
		// guard is not what makes it safe. Recorded so a future reader does not
		// mistake it for a case the guard is covering.
		huge := strings.Repeat("9", 100_000)
		var target struct {
			AcceptedBy string `json:"accepted_by"`
		}
		err := json.Unmarshal([]byte(`{"accepted_by": `+huge+`}`), &target)
		require.Error(t, err)
		assert.NotContains(t, err.Error(), "99999",
			"encoding/json reports the kind only when no conversion was attempted")

		assert.Less(t, len(RedactJSONError(err)), 200)
	})

	t.Run("a huge timestamp is not described at all", func(t *testing.T) {
		// time.Time's UnmarshalJSON returns *time.ParseError, which encoding/json
		// passes through untouched and which renders the whole input value. This
		// is the case that makes the helper an allow-list rather than a type
		// switch with an err.Error() fallthrough.
		huge := strings.Repeat("A", 100_000)
		var target struct {
			At time.Time `json:"at"`
		}
		err := json.Unmarshal([]byte(`{"at":"`+huge+`"}`), &target)
		require.Error(t, err)
		require.Contains(t, err.Error(), strings.Repeat("A", 1_000),
			"premise: time.ParseError renders the full input value")

		out := RedactJSONError(err)

		assert.NotContains(t, out, "AAAA")
		assert.Equal(t, "(unrecognised JSON decode error)", out,
			"an unvetted error type must be reported, never echoed")
	})

	t.Run("a syntax error reports the offset without the document", func(t *testing.T) {
		var target struct {
			Name string `json:"name"`
		}
		err := json.Unmarshal([]byte(`{"name": @}`), &target)
		require.Error(t, err)

		out := RedactJSONError(err)

		assert.Contains(t, out, "offset")
		assert.NotContains(t, out, "@")
		assert.NotContains(t, out, "invalid character")
	})

	t.Run("the known value kinds stay readable", func(t *testing.T) {
		// A guard that collapsed every kind would cost real diagnostics.
		for _, kind := range []string{"array", "object", "bool", "string", "number", "null"} {
			t.Run(kind, func(t *testing.T) {
				err := &json.UnmarshalTypeError{
					Value: kind,
					Type:  reflect.TypeOf(""),
					Field: "accepted_by",
				}
				assert.Contains(t, RedactJSONError(err), kind)
			})
		}
	})

	t.Run("a hand-built hostile field path is not echoed", func(t *testing.T) {
		err := &json.UnmarshalTypeError{
			Value:  "number",
			Type:   reflect.TypeOf(""),
			Struct: "Invite",
			Field:  "victim@example.com",
		}

		out := RedactJSONError(err)

		assert.NotContains(t, out, "example.com")
		assert.Contains(t, out, "(unrecognised field)")
	})

	t.Run("an over-long field path is not echoed", func(t *testing.T) {
		err := &json.UnmarshalTypeError{
			Value: "number",
			Type:  reflect.TypeOf(""),
			Field: strings.Repeat("a", 500),
		}

		out := RedactJSONError(err)

		assert.Less(t, len(out), 200)
		assert.Contains(t, out, "(unrecognised field)")
	})

	t.Run("an unrecognised error type is not echoed", func(t *testing.T) {
		err := errors.New("s3cret victim@example.com")

		out := RedactJSONError(err)

		assert.NotContains(t, out, "s3cret")
		assert.NotContains(t, out, "example.com")
		assert.Equal(t, "(unrecognised JSON decode error)", out)
	})

	t.Run("a wrapped decode error is still recognised", func(t *testing.T) {
		// errors.As, not a bare type switch: the error may arrive wrapped.
		var target struct {
			Name string `json:"name"`
		}
		inner := json.Unmarshal([]byte(`{"name": 12}`), &target)
		require.Error(t, inner)

		out := RedactJSONError(fmt.Errorf("outer: %w", inner))

		assert.Contains(t, out, "cannot unmarshal JSON number")
		assert.NotEqual(t, "(unrecognised JSON decode error)", out)
	})
}
