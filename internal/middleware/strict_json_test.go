// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package middleware

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// okHandler is a simple sentinel that records whether it was called.
var okBody = `{"status":"ok"}`

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(okBody))
	})
}

func postJSON(path, body string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

// ---- route-guard tests ----

func TestStrictCreateBodyMiddleware_NonTargetMethodPassesThrough(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/itx/meetings", nil)
	rr := httptest.NewRecorder()

	StrictCreateBodyMiddleware(okHandler()).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestStrictCreateBodyMiddleware_NonTargetPathPassesThrough(t *testing.T) {
	// POST to a sub-path should not be guarded (it has path parameters).
	req := postJSON("/itx/meetings/mtg-123/registrants", `{"project_uid":"a","Project_UID":"b"}`)
	rr := httptest.NewRecorder()

	StrictCreateBodyMiddleware(okHandler()).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

// ---- POST /itx/meetings ----

func TestStrictCreateBodyMiddleware_MeetingCleanBodyPassesThrough(t *testing.T) {
	body := `{"project_uid":"proj-1","title":"Test Meeting","committees":[{"uid":"comm-1"}]}`
	req := postJSON("/itx/meetings", body)
	rr := httptest.NewRecorder()

	StrictCreateBodyMiddleware(okHandler()).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	// Ensure the body is still fully readable by the downstream handler.
	downstream := httptest.NewRecorder()
	var captured []byte
	StrictCreateBodyMiddleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		captured, _ = io.ReadAll(r.Body)
	})).ServeHTTP(downstream, postJSON("/itx/meetings", body))
	assert.JSONEq(t, body, string(captured))
}

func TestStrictCreateBodyMiddleware_MeetingAmbiguousProjectUIDRejected(t *testing.T) {
	// Classic bypass: lowercase key for Heimdall, case-variant for encoding/json.
	body := `{"project_uid":"my-project","Project_UID":"victim-project","title":"Evil"}`
	req := postJSON("/itx/meetings", body)
	rr := httptest.NewRecorder()

	StrictCreateBodyMiddleware(okHandler()).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "ambiguous")
	assert.Contains(t, rr.Body.String(), `"code"`)
	assert.Contains(t, rr.Body.String(), `"message"`)
}

func TestStrictCreateBodyMiddleware_MeetingAmbiguousProjectUIDReversedOrderRejected(t *testing.T) {
	// Variant-first order — Heimdall sees nil, but we still reject.
	body := `{"Project_UID":"victim-project","project_uid":"my-project","title":"Evil"}`
	req := postJSON("/itx/meetings", body)
	rr := httptest.NewRecorder()

	StrictCreateBodyMiddleware(okHandler()).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestStrictCreateBodyMiddleware_MeetingAmbiguousCommitteeUIDRejected(t *testing.T) {
	// Committee uid bypass: uid + UID in the same committee object.
	body := `{"project_uid":"my-project","committees":[{"uid":"my-comm","UID":"victim-comm"}]}`
	req := postJSON("/itx/meetings", body)
	rr := httptest.NewRecorder()

	StrictCreateBodyMiddleware(okHandler()).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "ambiguous")
}

func TestStrictCreateBodyMiddleware_MeetingAmbiguousCommitteesArrayKeyRejected(t *testing.T) {
	// Top-level "Committees" vs "committees" — a second case-variant array.
	body := `{"project_uid":"mine","committees":[{"uid":"c1"}],"Committees":[{"uid":"victim"}]}`
	req := postJSON("/itx/meetings", body)
	rr := httptest.NewRecorder()

	StrictCreateBodyMiddleware(okHandler()).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestStrictCreateBodyMiddleware_MeetingDuplicateExactKeyPassesThrough(t *testing.T) {
	// Two identical exact-case keys: both Heimdall and encoding/json see the
	// same (last) value — no differential, no bypass.
	body := `{"project_uid":"a","project_uid":"b","title":"T"}`
	req := postJSON("/itx/meetings", body)
	rr := httptest.NewRecorder()

	StrictCreateBodyMiddleware(okHandler()).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestStrictCreateBodyMiddleware_MeetingEmptyBodyPassesThrough(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/itx/meetings", bytes.NewReader([]byte{}))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	StrictCreateBodyMiddleware(okHandler()).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestStrictCreateBodyMiddleware_MeetingInvalidJSONPassesThrough(t *testing.T) {
	// Malformed JSON is not our concern — let Goa return its own 400.
	req := postJSON("/itx/meetings", `{"project_uid": NOTJSON}`)
	rr := httptest.NewRecorder()

	StrictCreateBodyMiddleware(okHandler()).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

// ---- POST /itx/past_meetings ----

func TestStrictCreateBodyMiddleware_PastMeetingCleanBodyPassesThrough(t *testing.T) {
	body := `{"project_uid":"proj-1","meeting_id":"mtg-1"}`
	req := postJSON("/itx/past_meetings", body)
	rr := httptest.NewRecorder()

	StrictCreateBodyMiddleware(okHandler()).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestStrictCreateBodyMiddleware_PastMeetingAmbiguousProjectUIDRejected(t *testing.T) {
	body := `{"project_uid":"mine","Project_UID":"victim","meeting_id":"mtg-1"}`
	req := postJSON("/itx/past_meetings", body)
	rr := httptest.NewRecorder()

	StrictCreateBodyMiddleware(okHandler()).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "ambiguous")
}

func TestStrictCreateBodyMiddleware_PastMeetingAmbiguousCommitteeUIDRejected(t *testing.T) {
	body := `{"project_uid":"mine","committees":[{"uid":"c1","Uid":"victim"}]}`
	req := postJSON("/itx/past_meetings", body)
	rr := httptest.NewRecorder()

	StrictCreateBodyMiddleware(okHandler()).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// ---- checkAmbiguousJSONKeys unit tests ----

// ---- body size limit tests ----

func TestStrictCreateBodyMiddleware_BodyExceedsLimitRejected(t *testing.T) {
	// Build a body slightly larger than the 1 MiB cap by padding with whitespace.
	padding := strings.Repeat(" ", int(maxCreateBodyBytes)+1)
	body := `{"project_uid":"p"}` + padding
	req := postJSON("/itx/meetings", body)
	rr := httptest.NewRecorder()

	StrictCreateBodyMiddleware(okHandler()).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, rr.Code)
	assert.Contains(t, rr.Body.String(), "too large")
	assert.Contains(t, rr.Body.String(), "413")
	assert.Equal(t, "close", rr.Header().Get("Connection"), "413 response must set Connection: close")
}

func TestStrictCreateBodyMiddleware_BodyClearlyUnderLimitPassesThrough(t *testing.T) {
	// Build a body of exactly maxCreateBodyBytes-1 bytes to verify the boundary
	// is correctly placed: a body one byte under the cap must pass through.
	// (Using the constant explicitly so the test breaks if the limit changes.)
	prefix := `{"project_uid":"proj-1"}`
	paddingLen := int(maxCreateBodyBytes) - 1 - len(prefix)
	require.Positive(t, paddingLen, "prefix must be shorter than maxCreateBodyBytes")
	body := prefix + strings.Repeat(" ", paddingLen)
	require.Equal(t, int(maxCreateBodyBytes)-1, len(body))

	req := postJSON("/itx/meetings", body)
	rr := httptest.NewRecorder()

	StrictCreateBodyMiddleware(okHandler()).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

// ---- Unicode fold tests ----

func TestStrictCreateBodyMiddleware_UnicodeLongSFoldRejected(t *testing.T) {
	// "committeeſ" (U+017F long s) folds to "committees" under encoding/json's
	// bytes.EqualFold semantics but not under strings.ToLower. This is the
	// canonical Unicode bypass that the foldKey fix addresses.
	body := "{\"project_uid\":\"mine\",\"committees\":[{\"uid\":\"c1\"}],\"committee\u017f\":[{\"uid\":\"victim\"}]}"
	req := postJSON("/itx/meetings", body)
	rr := httptest.NewRecorder()

	StrictCreateBodyMiddleware(okHandler()).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "ambiguous")
}

func TestStrictCreateBodyMiddleware_UnicodeKelvinSignFoldRejected(t *testing.T) {
	// U+212A (Kelvin sign) folds to 'k' under Unicode simple case folding.
	// A key like "\u212aey" collides with "key".
	body := "{\"project_uid\":\"mine\",\"\u212aey\":\"v1\",\"key\":\"v2\"}"
	req := postJSON("/itx/meetings", body)
	rr := httptest.NewRecorder()

	StrictCreateBodyMiddleware(okHandler()).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestCheckAmbiguousJSONKeys_Clean(t *testing.T) {
	cases := []struct {
		name string
		json string
	}{
		{"flat object", `{"a":1,"b":2}`},
		{"nested object", `{"a":{"b":1}}`},
		{"array of objects", `[{"uid":"x"},{"uid":"y"}]`},
		{"empty object", `{}`},
		{"empty array", `[]`},
		{"scalar string", `"hello"`},
		{"scalar number", `42`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := checkAmbiguousJSONKeys([]byte(tc.json))
			require.NoError(t, err)
		})
	}
}

func TestCheckAmbiguousJSONKeys_Ambiguous(t *testing.T) {
	cases := []struct {
		name string
		json string
	}{
		{"top-level case variant", `{"project_uid":"a","Project_UID":"b"}`},
		{"nested case variant", `{"x":{"uid":"a","UID":"b"}}`},
		{"array element case variant", `[{"uid":"a","Uid":"b"}]`},
		{"mid-word case variant", `{"fooBar":1,"foobar":2}`},
		// Unicode simple case folding: ſ (U+017F) folds to s.
		{"long s vs s", "{\"committee\u017f\":\"a\",\"committees\":\"b\"}"},
		// Unicode simple case folding: K (U+212A Kelvin) folds to k.
		{"kelvin vs k", "{\"\u212aey\":\"a\",\"key\":\"b\"}"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := checkAmbiguousJSONKeys([]byte(tc.json))
			require.Error(t, err)
			assert.Contains(t, err.Error(), "ambiguous")
		})
	}
}

func TestCheckAmbiguousJSONKeys_InvalidJSONReturnsNil(t *testing.T) {
	// Invalid JSON must not return an error from this function — Goa owns that.
	err := checkAmbiguousJSONKeys([]byte(`{"key": }`))
	require.NoError(t, err)
}
