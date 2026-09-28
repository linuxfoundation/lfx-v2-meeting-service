// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package middleware

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"unicode"
)

// maxCreateBodyBytes is the maximum request body size accepted on the guarded
// create endpoints. Bodies larger than this are rejected with 413 before any
// JSON parsing occurs, preventing memory exhaustion from unbounded io.ReadAll.
const maxCreateBodyBytes int64 = 1 << 20 // 1 MiB

// strictCreatePaths is the set of POST endpoints whose JSON bodies are
// independently parsed by both Heimdall (case-sensitive key lookup) and
// encoding/json (case-insensitive struct matching). Ambiguous bodies on these
// routes can bypass Heimdall's body-derived authorization check (CWE-436).
var strictCreatePaths = map[string]struct{}{
	"/itx/meetings":      {},
	"/itx/past_meetings": {},
}

// errBadJSON is a sentinel returned by the walker functions when the JSON
// tokenizer encounters a parse error. It is mapped back to nil by
// checkAmbiguousJSONKeys so that malformed bodies are passed through to Goa
// (which owns parse-error responses) rather than silently suppressed inside
// a loop. Using a distinct sentinel — rather than returning nil — is critical:
// returning nil from walkJSONValue on a Token() failure leaves the decoder
// stuck at the same position, causing walkJSONArray's dec.More() loop to spin
// forever at full CPU (DoS via a single malformed request body).
var errBadJSON = errors.New("bad json")

// StrictCreateBodyMiddleware guards POST /itx/meetings and POST
// /itx/past_meetings against two classes of malformed request:
//
//  1. Bodies larger than maxCreateBodyBytes (1 MiB) — rejected immediately
//     with 413 Request Entity Too Large and Connection: close. This is a
//     transport-level guard; 413 is not declared in the Goa design so
//     generated clients receive it as an untyped transport error.
//
//  2. Bodies that contain two keys in the same JSON object that are distinct
//     strings but compare equal under encoding/json's Unicode simple
//     case-folding semantics (e.g. "project_uid" and "Project_UID", or
//     "committees" and "committeeſ") — rejected with 400 Bad Request whose
//     body matches the BadRequestError shape declared in the Goa design
//     ({code, message}).
//
// Background: Heimdall authorizes these create endpoints by looking up
// .Request.Body.project_uid in its case-sensitive generic JSON parse of the
// forwarded body, while the service decodes the same bytes with
// encoding/json into a generated struct whose tags use case-insensitive
// matching. A crafted body such as
//
//	{"project_uid":"<mine>","Project_UID":"<victim>", ...}
//
// would be authorized for <mine> but forwarded to ITX as <victim>. This
// middleware detects that pattern and returns 400 before the body reaches
// Goa's decoder, ensuring the value Heimdall authorized is provably the
// value the decoder will use.
//
// Non-targeted routes, bodies within the size cap that are not JSON, and
// bodies within the size cap that have parse errors are passed through
// unchanged so that Goa can produce its own validation error.
func StrictCreateBodyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isStrictJSONTarget(r) {
			next.ServeHTTP(w, r)
			return
		}

		// Cap the body size before buffering to prevent memory exhaustion.
		r.Body = http.MaxBytesReader(w, r.Body, maxCreateBodyBytes)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			var maxErr *http.MaxBytesError
			if errors.As(err, &maxErr) {
				// Set Connection: close so the server closes the TCP connection
				// after sending the 413 response, without waiting for the client
				// to finish sending the oversized body. This is necessary because
				// the ResponseWriter may be wrapped (e.g. by RequestLoggerMiddleware)
				// and http.MaxBytesReader cannot unwrap it to call the internal
				// requestTooLarge() notification.
				w.Header().Set("Connection", "close")
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusRequestEntityTooLarge)
				_ = json.NewEncoder(w).Encode(strictErrorBody("413", "request body too large"))
				return
			}
			// Other read errors — pass through and let Goa surface the error.
			next.ServeHTTP(w, r)
			return
		}
		// Restore the body so Goa's decoder can read it normally.
		r.Body = io.NopCloser(bytes.NewReader(body))

		if len(body) > 0 {
			if ambiguousErr := checkAmbiguousJSONKeys(body); ambiguousErr != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(strictErrorBody("400", ambiguousErr.Error()))
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

// strictErrorBody returns a JSON-encodable error object with {code, message}
// keys. For 400 responses this matches the BadRequestError shape declared in
// the Goa design, so generated clients can decode the body against that schema.
// For 413 responses the status is undeclared in the design; the body is
// emitted in the same shape for consistency but will not be schema-decoded by
// generated clients.
func strictErrorBody(code, message string) map[string]string {
	return map[string]string{"code": code, "message": message}
}

// isStrictJSONTarget reports whether r is a POST to one of the guarded paths.
func isStrictJSONTarget(r *http.Request) bool {
	if r.Method != http.MethodPost {
		return false
	}
	_, guarded := strictCreatePaths[r.URL.Path]
	return guarded
}

// checkAmbiguousJSONKeys returns an error if data contains any JSON object
// with two keys k1 and k2 where k1 != k2 but encoding/json would fold them
// to the same struct field (Unicode simple case folding, matching the
// semantics of bytes.EqualFold / strings.EqualFold). Such a key pair causes
// Heimdall (case-sensitive parse) and encoding/json (case-insensitive struct
// decode) to bind the field to different values.
//
// Non-JSON input and JSON parse errors return nil so that Goa's decoder can
// produce the authoritative validation error. Internally, the walkers return
// errBadJSON on tokenizer failures (rather than nil) to break out of
// dec.More() loops that would otherwise spin forever on malformed input.
func checkAmbiguousJSONKeys(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber() // preserve numbers exactly; we only care about structure
	err := walkJSONValue(dec)
	if errors.Is(err, errBadJSON) {
		return nil // malformed JSON — not our problem; let Goa produce the error
	}
	return err
}

// foldKey returns a canonical string for key such that two keys produce the
// same foldKey if and only if strings.EqualFold reports them equal. This
// mirrors encoding/json's field-name matching, which uses bytes.EqualFold —
// Unicode simple case folding — rather than strings.ToLower.
//
// For each rune r, we walk the SimpleFold orbit (r → SimpleFold(r) → … → r)
// and keep the smallest rune in the orbit as the canonical representative.
func foldKey(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		min := r
		for f := unicode.SimpleFold(r); f != r; f = unicode.SimpleFold(f) {
			if f < min {
				min = f
			}
		}
		b.WriteRune(min)
	}
	return b.String()
}

// walkJSONValue consumes exactly one JSON value from dec.
// It returns errBadJSON on any tokenizer failure so that callers can break
// out of dec.More() loops — returning nil instead would leave the decoder
// stuck at the same position and cause the loop to spin forever.
func walkJSONValue(dec *json.Decoder) error {
	tok, err := dec.Token()
	if err != nil {
		return errBadJSON // signal to break loops; filtered to nil by checkAmbiguousJSONKeys
	}
	delim, ok := tok.(json.Delim)
	if !ok {
		return nil // scalar — nothing to check
	}
	switch delim {
	case '{':
		return walkJSONObject(dec)
	case '[':
		return walkJSONArray(dec)
	}
	return nil
}

// walkJSONObject checks the current object for case-fold key collisions and
// recurses into nested values. dec must be positioned immediately after the
// opening '{'.
//
// The seen map keys on foldKey(key) — the canonical Unicode simple-fold
// representative — so that variants like "committees" and "committeeſ" (long s,
// U+017F) are detected as collisions, matching encoding/json's field-matching
// semantics exactly.
func walkJSONObject(dec *json.Decoder) error {
	// seen maps foldKey(key) → first exact-case key observed.
	seen := make(map[string]string)

	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return errBadJSON
		}
		key, ok := keyTok.(string)
		if !ok {
			return errBadJSON
		}

		fold := foldKey(key)
		if orig, exists := seen[fold]; exists && orig != key {
			// Two distinct strings that fold to the same field name — reject.
			return fmt.Errorf(
				"request body contains ambiguous JSON fields %q and %q: "+
					"these differ only in case but would be decoded as the same field; "+
					"request rejected to prevent authorization bypass",
				orig, key,
			)
		}
		if _, exists := seen[fold]; !exists {
			seen[fold] = key
		}

		// Recurse into the value regardless of whether the key was a duplicate.
		if err := walkJSONValue(dec); err != nil {
			return err
		}
	}

	// Consume closing '}'.
	if _, err := dec.Token(); err != nil {
		return nil
	}
	return nil
}

// walkJSONArray recurses into each element of the current array.
// dec must be positioned immediately after the opening '['.
func walkJSONArray(dec *json.Decoder) error {
	for dec.More() {
		if err := walkJSONValue(dec); err != nil {
			return err
		}
	}
	// Consume closing ']'.
	if _, err := dec.Token(); err != nil {
		return nil
	}
	return nil
}
