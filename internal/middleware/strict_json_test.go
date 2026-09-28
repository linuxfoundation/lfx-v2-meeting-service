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
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// okBody is the body written by okHandler. Pass-through tests assert the
// response body equals okBody (proving next was called). Reject tests assert
// the response body does NOT equal okBody (proving next was not called and the
// middleware short-circuited with its own error response).
var okBody = `{"status":"ok"}`

// okHandler writes okBody with 200 OK so tests can distinguish "middleware
// called next" from "middleware returned early with its own response".
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
	assert.Equal(t, okBody, rr.Body.String(), "next must be called for non-POST requests")
}

func TestStrictCreateBodyMiddleware_NonTargetPathPassesThrough(t *testing.T) {
	// POST to a sub-path should not be guarded (it has path parameters).
	req := postJSON("/itx/meetings/mtg-123/registrants", `{"project_uid":"a","Project_UID":"b"}`)
	rr := httptest.NewRecorder()

	StrictCreateBodyMiddleware(okHandler()).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, okBody, rr.Body.String(), "next must be called for non-guarded paths")
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
	assert.Contains(t, rr.Body.String(), "authorization bypass")
	assert.Contains(t, rr.Body.String(), `"code"`)
	assert.Contains(t, rr.Body.String(), `"message"`)
	assert.NotContains(t, rr.Body.String(), okBody, "next must NOT be called on reject")
}

func TestStrictCreateBodyMiddleware_MeetingAmbiguousProjectUIDReversedOrderRejected(t *testing.T) {
	// Variant-first order — Heimdall sees nil, but we still reject.
	body := `{"Project_UID":"victim-project","project_uid":"my-project","title":"Evil"}`
	req := postJSON("/itx/meetings", body)
	rr := httptest.NewRecorder()

	StrictCreateBodyMiddleware(okHandler()).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.NotContains(t, rr.Body.String(), okBody, "next must NOT be called on reject")
}

func TestStrictCreateBodyMiddleware_MeetingAmbiguousCommitteeUIDRejected(t *testing.T) {
	// Committee uid bypass: uid + UID in the same committee object.
	body := `{"project_uid":"my-project","committees":[{"uid":"my-comm","UID":"victim-comm"}]}`
	req := postJSON("/itx/meetings", body)
	rr := httptest.NewRecorder()

	StrictCreateBodyMiddleware(okHandler()).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "authorization bypass")
	assert.NotContains(t, rr.Body.String(), okBody, "next must NOT be called on reject")
}

func TestStrictCreateBodyMiddleware_MeetingAmbiguousCommitteesArrayKeyRejected(t *testing.T) {
	// Top-level "Committees" vs "committees" — a second case-variant array.
	body := `{"project_uid":"mine","committees":[{"uid":"c1"}],"Committees":[{"uid":"victim"}]}`
	req := postJSON("/itx/meetings", body)
	rr := httptest.NewRecorder()

	StrictCreateBodyMiddleware(okHandler()).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.NotContains(t, rr.Body.String(), okBody, "next must NOT be called on reject")
}

func TestStrictCreateBodyMiddleware_MeetingDuplicateExactKeyPassesThrough(t *testing.T) {
	// Two identical exact-case keys: both Heimdall and encoding/json see the
	// same (last) value — no differential, no bypass.
	body := `{"project_uid":"a","project_uid":"b","title":"T"}`
	req := postJSON("/itx/meetings", body)
	rr := httptest.NewRecorder()

	StrictCreateBodyMiddleware(okHandler()).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, okBody, rr.Body.String(), "next must be called for exact-duplicate keys")
}

func TestStrictCreateBodyMiddleware_MeetingEmptyBodyPassesThrough(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/itx/meetings", bytes.NewReader([]byte{}))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	StrictCreateBodyMiddleware(okHandler()).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, okBody, rr.Body.String(), "next must be called for empty body")
}

func TestStrictCreateBodyMiddleware_MeetingInvalidJSONPassesThrough(t *testing.T) {
	// Malformed JSON (non-array context) — not our concern; let Goa return its own 400.
	req := postJSON("/itx/meetings", `{"project_uid": NOTJSON}`)
	rr := httptest.NewRecorder()

	StrictCreateBodyMiddleware(okHandler()).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, okBody, rr.Body.String(), "next must be called for invalid JSON (Goa owns the error)")
}

// ---- POST /itx/past_meetings ----

func TestStrictCreateBodyMiddleware_PastMeetingCleanBodyPassesThrough(t *testing.T) {
	body := `{"project_uid":"proj-1","meeting_id":"mtg-1"}`
	req := postJSON("/itx/past_meetings", body)
	rr := httptest.NewRecorder()

	StrictCreateBodyMiddleware(okHandler()).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, okBody, rr.Body.String(), "next must be called for clean past-meeting body")
}

func TestStrictCreateBodyMiddleware_PastMeetingAmbiguousProjectUIDRejected(t *testing.T) {
	body := `{"project_uid":"mine","Project_UID":"victim","meeting_id":"mtg-1"}`
	req := postJSON("/itx/past_meetings", body)
	rr := httptest.NewRecorder()

	StrictCreateBodyMiddleware(okHandler()).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "authorization bypass")
	assert.NotContains(t, rr.Body.String(), okBody, "next must NOT be called on past-meeting reject")
}

func TestStrictCreateBodyMiddleware_PastMeetingAmbiguousCommitteeUIDRejected(t *testing.T) {
	body := `{"project_uid":"mine","committees":[{"uid":"c1","Uid":"victim"}]}`
	req := postJSON("/itx/past_meetings", body)
	rr := httptest.NewRecorder()

	StrictCreateBodyMiddleware(okHandler()).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.NotContains(t, rr.Body.String(), okBody, "next must NOT be called on past-meeting committee reject")
}

// ---- single non-canonical key bypass tests ----
// These guard against the case where only ONE non-canonical key is present
// (no lowercase counterpart). The collision guard cannot detect it, but
// encoding/json still decodes it — while Heimdall authorizes via a different
// branch (e.g. committee), opening a single-key CWE-436 bypass.

func TestStrictCreateBodyMiddleware_SingleNonCanonicalProjectUIDRejected(t *testing.T) {
	// "Project_UID" alone (no "project_uid") — Heimdall would authorize via the
	// committee branch; encoding/json decodes Project_UID → victim project.
	body := `{"Project_UID":"victim","committees":[{"uid":"owned"}],"title":"T","start_time":"2026-10-01T10:00:00Z","duration":60,"timezone":"UTC"}`
	req := postJSON("/itx/meetings", body)
	rr := httptest.NewRecorder()

	StrictCreateBodyMiddleware(okHandler()).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "non-canonical")
	assert.NotContains(t, rr.Body.String(), okBody, "next must NOT be called on single non-canonical key")
}

func TestStrictCreateBodyMiddleware_SingleNonCanonicalCommitteeUIDRejected(t *testing.T) {
	// "UID" alone inside a committee object (no lowercase "uid") — Heimdall
	// authorizes via project:mine; encoding/json decodes UID → victim committee.
	body := `{"project_uid":"mine","committees":[{"UID":"victim"}],"title":"T","start_time":"2026-10-01T10:00:00Z","duration":60,"timezone":"UTC"}`
	req := postJSON("/itx/meetings", body)
	rr := httptest.NewRecorder()

	StrictCreateBodyMiddleware(okHandler()).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "non-canonical")
	assert.NotContains(t, rr.Body.String(), okBody, "next must NOT be called on single non-canonical committee uid")
}

func TestStrictCreateBodyMiddleware_SingleNonCanonicalCommitteesKeyRejected(t *testing.T) {
	// "Committees" (capital C) alone — Heimdall sees no committees, encoding/json
	// decodes Committees → victim committee array.
	body := `{"project_uid":"mine","Committees":[{"uid":"victim"}],"title":"T","start_time":"2026-10-01T10:00:00Z","duration":60,"timezone":"UTC"}`
	req := postJSON("/itx/meetings", body)
	rr := httptest.NewRecorder()

	StrictCreateBodyMiddleware(okHandler()).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "non-canonical")
	assert.NotContains(t, rr.Body.String(), okBody, "next must NOT be called on single non-canonical Committees key")
}

func TestStrictCreateBodyMiddleware_PastMeetingSingleNonCanonicalProjectUIDRejected(t *testing.T) {
	body := `{"Project_UID":"victim","committees":[{"uid":"owned"}]}`
	req := postJSON("/itx/past_meetings", body)
	rr := httptest.NewRecorder()

	StrictCreateBodyMiddleware(okHandler()).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "non-canonical")
	assert.NotContains(t, rr.Body.String(), okBody)
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
	assert.NotContains(t, rr.Body.String(), okBody, "next must NOT be called on 413")
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
	assert.Equal(t, okBody, rr.Body.String(), "next must be called for body under the size cap")
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
	assert.Contains(t, rr.Body.String(), "authorization bypass")
	assert.NotContains(t, rr.Body.String(), okBody, "next must NOT be called on Unicode long-s reject")
}

func TestStrictCreateBodyMiddleware_UnicodeKelvinSignFoldRejected(t *testing.T) {
	// U+212A (Kelvin sign) folds to 'k' under Unicode simple case folding.
	// A key like "\u212aey" collides with "key".
	body := "{\"project_uid\":\"mine\",\"\u212aey\":\"v1\",\"key\":\"v2\"}"
	req := postJSON("/itx/meetings", body)
	rr := httptest.NewRecorder()

	StrictCreateBodyMiddleware(okHandler()).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.NotContains(t, rr.Body.String(), okBody, "next must NOT be called on Unicode Kelvin-sign reject")
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
		// Single non-canonical spellings of sensitive keys.
		{"single Project_UID", `{"Project_UID":"victim","committees":[{"uid":"owned"}]}`},
		{"single UID in committee", `{"project_uid":"mine","committees":[{"UID":"victim"}]}`},
		{"single Committees", `{"project_uid":"mine","Committees":[{"uid":"c1"}]}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := checkAmbiguousJSONKeys([]byte(tc.json))
			require.Error(t, err)
			assert.Contains(t, err.Error(), "authorization bypass")
		})
	}
}

func TestCheckAmbiguousJSONKeys_InvalidJSONReturnsNil(t *testing.T) {
	// Invalid JSON must not return an error from this function — Goa owns that.
	err := checkAmbiguousJSONKeys([]byte(`{"key": }`))
	require.NoError(t, err)
}

// ---- Infinite-loop regression tests ----
// These guard against the DoS where walkJSONValue returned nil on Token()
// failure, leaving dec.More() stuck at true and spinning the loop forever.

func TestCheckAmbiguousJSONKeys_MalformedArrayNoHang(t *testing.T) {
	// Missing comma between array elements — Token() fails mid-array.
	// Previously caused an infinite loop; must return within 1 second.
	done := make(chan error, 1)
	go func() {
		done <- checkAmbiguousJSONKeys([]byte(`{"project_uid":"p","committees":[{"uid":"a"} {"uid":"b"}]}`))
	}()
	select {
	case err := <-done:
		require.NoError(t, err, "malformed array body must pass through (nil), not hang")
	case <-time.After(time.Second):
		t.Fatal("checkAmbiguousJSONKeys hung on malformed array body (missing comma)")
	}
}

func TestCheckAmbiguousJSONKeys_TrailingCommaNoHang(t *testing.T) {
	done := make(chan error, 1)
	go func() {
		done <- checkAmbiguousJSONKeys([]byte(`{"project_uid":"p","committees":[{"uid":"a"},]}`))
	}()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("checkAmbiguousJSONKeys hung on trailing-comma body")
	}
}

func TestCheckAmbiguousJSONKeys_TruncatedArrayNoHang(t *testing.T) {
	done := make(chan error, 1)
	go func() {
		done <- checkAmbiguousJSONKeys([]byte(`{"committees":[`))
	}()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("checkAmbiguousJSONKeys hung on truncated array body")
	}
}

func TestStrictCreateBodyMiddleware_MalformedArrayBodyPassesThrough(t *testing.T) {
	// Malformed JSON inside a committees array — middleware must pass through
	// within 1 second so Goa can return its own parse error (not hang).
	body := `{"project_uid":"p","committees":[{"uid":"a"} {"uid":"b"}]}`
	req := postJSON("/itx/meetings", body)
	rr := httptest.NewRecorder()

	done := make(chan struct{}, 1)
	go func() {
		StrictCreateBodyMiddleware(okHandler()).ServeHTTP(rr, req)
		done <- struct{}{}
	}()
	select {
	case <-done:
		// Malformed JSON is not our concern — middleware must have called next.
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, okBody, rr.Body.String(), "next must be called for malformed array body")
	case <-time.After(time.Second):
		t.Fatal("middleware hung on malformed array body (missing comma between objects)")
	}
}
