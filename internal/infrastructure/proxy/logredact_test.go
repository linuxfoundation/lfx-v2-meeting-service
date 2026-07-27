// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package proxy

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/models/itx"
)

// TestRequestJSONForLog_RedactsAuditPII pins down that debug-level ITX request
// logs (which serialize the full request body) do not leak the requester's
// name and email via CreatedBy / UpdatedBy audit fields. The wire body sent to
// ITX is unaffected — the redaction only applies to what proxy.Client emits
// via slog.DebugContext.
func TestRequestJSONForLog_RedactsAuditPII(t *testing.T) {
	// Present in the outbound request struct so we can confirm the redaction
	// only touches the CreatedBy / UpdatedBy fields and doesn't accidentally
	// strip other user-provided attachment metadata (description, filename).
	const nonAuditDescription = "Meeting notes for Q3 planning"

	created := &itx.CreatedUpdatedBy{
		Username: "alice",
		Email:    "alice@example.com",
		Name:     "Alice Example",
	}
	updated := &itx.CreatedUpdatedBy{
		Username: "bob",
		Email:    "bob@example.com",
		Name:     "Bob Example",
	}

	assertRedacted := func(t *testing.T, name string, req any, extraKept ...string) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			logged := requestJSONForLog(req)

			assert.NotContains(t, logged, "alice@example.com",
				"redacted log body must not contain requester email")
			assert.NotContains(t, logged, "bob@example.com",
				"redacted log body must not contain requester email")
			assert.NotContains(t, logged, "Alice Example",
				"redacted log body must not contain requester display name")
			assert.NotContains(t, logged, "Bob Example",
				"redacted log body must not contain requester display name")

			// Confirm the redacted log body is still useful for debugging by
			// keeping non-PII request fields intact.
			for _, kept := range extraKept {
				assert.Contains(t, logged, kept,
					"redacted log body should retain non-audit request fields for debugging")
			}
			assert.Contains(t, logged, nonAuditDescription,
				"redacted log body should retain user-supplied attachment metadata")
		})
	}

	assertRedacted(t, "CreateMeetingAttachmentRequest", &itx.CreateMeetingAttachmentRequest{
		Type:        "file",
		Category:    "Notes",
		Name:        "notes.pdf",
		Description: nonAuditDescription,
		CreatedBy:   created,
	}, "notes.pdf")

	assertRedacted(t, "UpdateMeetingAttachmentRequest", &itx.UpdateMeetingAttachmentRequest{
		Type:        "file",
		Category:    "Notes",
		Name:        "notes.pdf",
		Description: nonAuditDescription,
		UpdatedBy:   updated,
	}, "notes.pdf")

	assertRedacted(t, "CreateAttachmentPresignRequest", &itx.CreateAttachmentPresignRequest{
		Name:        "notes.pdf",
		Description: nonAuditDescription,
		FileSize:    1024,
		FileType:    "application/pdf",
		CreatedBy:   created,
	}, "application/pdf")

	assertRedacted(t, "CreatePastMeetingAttachmentRequest", &itx.CreatePastMeetingAttachmentRequest{
		Type:        "file",
		Category:    "Notes",
		Name:        "notes.pdf",
		Description: nonAuditDescription,
		CreatedBy:   created,
	}, "notes.pdf")

	assertRedacted(t, "UpdatePastMeetingAttachmentRequest", &itx.UpdatePastMeetingAttachmentRequest{
		Type:        "file",
		Category:    "Notes",
		Name:        "notes.pdf",
		Description: nonAuditDescription,
		UpdatedBy:   updated,
	}, "notes.pdf")
}

// TestRequestJSONForLog_LeavesUnknownTypesAlone confirms that request types
// without audit fields (e.g. meeting create/update, which don't hit this
// debug-log code path today) still round-trip cleanly.
func TestRequestJSONForLog_LeavesUnknownTypesAlone(t *testing.T) {
	req := &itx.CreateZoomMeetingRequest{
		Topic:     "Weekly sync",
		StartTime: "2026-01-01T00:00:00Z",
	}
	logged := requestJSONForLog(req)

	assert.Contains(t, logged, "Weekly sync")
	assert.Contains(t, logged, "2026-01-01T00:00:00Z")
}

// TestRedactAuditForLog_DoesNotMutateOriginal ensures the wire body is not
// affected by the log-time redaction. Without this guarantee, ITX would
// receive requests with created_by / updated_by cleared.
func TestRedactAuditForLog_DoesNotMutateOriginal(t *testing.T) {
	original := &itx.CreateMeetingAttachmentRequest{
		Type: "file",
		Name: "notes.pdf",
		CreatedBy: &itx.CreatedUpdatedBy{
			Username: "alice",
			Email:    "alice@example.com",
			Name:     "Alice Example",
		},
	}

	_ = requestJSONForLog(original)

	require.NotNil(t, original.CreatedBy, "log-time redaction must not clear the original request's CreatedBy — that would strip the audit stamp from the wire body")
	assert.Equal(t, "alice", original.CreatedBy.Username)
	assert.Equal(t, "alice@example.com", original.CreatedBy.Email)
	assert.Equal(t, "Alice Example", original.CreatedBy.Name)
}

// TestRedactAuditForLog_HandlesNil guards against panics when the request is
// nil or a nil typed pointer.
func TestRedactAuditForLog_HandlesNil(t *testing.T) {
	// Untyped nil.
	assert.Equal(t, `null`, requestJSONForLog(nil))

	// Typed nil pointer to a redactable type.
	var typedNil *itx.CreateMeetingAttachmentRequest
	logged := requestJSONForLog(typedNil)
	// Whatever the marshaled representation is, we just need to not panic
	// and to not produce a body claiming to contain audit PII.
	assert.NotContains(t, logged, "alice@example.com")
}

// Sanity check that the wire-body JSON — which is what the audit stamp is
// carried on — still includes name and email. This guards against future
// changes that might extend the log-time redaction into the marshaling path
// itself (which would break the actual purpose of the audit stamp).
func TestWireBody_StillContainsAuditPII(t *testing.T) {
	req := &itx.CreateMeetingAttachmentRequest{
		Name: "notes.pdf",
		CreatedBy: &itx.CreatedUpdatedBy{
			Username: "alice",
			Email:    "alice@example.com",
			Name:     "Alice Example",
		},
	}

	// This is what proxy.Client sends to ITX (via json.Marshal(req)).
	body, err := json.Marshal(req)
	require.NoError(t, err)

	assert.True(t, strings.Contains(string(body), "alice@example.com"),
		"wire body must carry the requester's email so ITX records the correct audit trail")
	assert.True(t, strings.Contains(string(body), "Alice Example"),
		"wire body must carry the requester's display name so ITX records the correct audit trail")
}

// TestResponseJSONForLog_RedactsAuditPII pins down that debug-level ITX
// response logs (which serialize the full response body) don't leak PII via
// the audit fields ITX echoes back. Three vectors are covered:
//  1. Requester echo on create/update (created_by / updated_by with our own
//     name+email).
//  2. Persisted values on GET responses (audit stamps from a *different* user
//     who originally created the attachment).
//  3. file_uploaded_by populated by ITX after upload completion.
func TestResponseJSONForLog_RedactsAuditPII(t *testing.T) {
	assertRedacted := func(t *testing.T, name string, respJSON string, mustKeep ...string) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			out := responseJSONForLog([]byte(respJSON))

			assert.NotContains(t, out, "alice@example.com", "requester email must not appear in response log")
			assert.NotContains(t, out, "bob@example.com", "prior-updater email must not appear in response log")
			assert.NotContains(t, out, "eve@example.com", "prior-uploader email must not appear in response log")
			assert.NotContains(t, out, "Alice Example")
			assert.NotContains(t, out, "Bob Example")
			assert.NotContains(t, out, "Eve Example")

			// Debug value should still be present: username survives,
			// non-audit response fields untouched.
			assert.Contains(t, out, `"username":"alice"`, "username should survive redaction for debugging")

			for _, kept := range mustKeep {
				assert.Contains(t, out, kept, "non-audit response fields should be preserved for debugging")
			}
		})
	}

	// Create/update echo shape.
	assertRedacted(t, "created_by echo",
		`{"id":"att-1","name":"notes.pdf","created_by":{"username":"alice","email":"alice@example.com","name":"Alice Example"}}`,
		`"id":"att-1"`, `"name":"notes.pdf"`)

	assertRedacted(t, "updated_by echo",
		`{"id":"att-1","name":"notes.pdf","updated_by":{"username":"alice","email":"alice@example.com","name":"Alice Example"}}`,
		`"id":"att-1"`)

	// GET response with a *different* user's audit stamps.
	assertRedacted(t, "GET with prior creator + updater",
		`{"id":"att-1","name":"notes.pdf","created_by":{"username":"alice","email":"alice@example.com","name":"Alice Example"},"updated_by":{"username":"bob","email":"bob@example.com","name":"Bob Example"}}`,
		`"id":"att-1"`)

	// file_uploaded_by populated by ITX after upload completes.
	assertRedacted(t, "file_uploaded_by",
		`{"id":"att-1","file_uploaded_by":{"username":"eve","email":"eve@example.com","name":"Eve Example"},"created_by":{"username":"alice","email":"alice@example.com","name":"Alice Example"}}`,
		`"id":"att-1"`)

	// modified_by (past-meeting-summary shape — same helper covers it defensively).
	assertRedacted(t, "modified_by",
		`{"id":"sum-1","modified_by":{"username":"alice","email":"alice@example.com","name":"Alice Example"}}`,
		`"id":"sum-1"`)
}

// TestResponseJSONForLog_UsernameOnly confirms that when the audit field is
// already just a username (no email/name present) the redaction still emits
// the same shape rather than turning it into null.
func TestResponseJSONForLog_UsernameOnly(t *testing.T) {
	out := responseJSONForLog([]byte(`{"created_by":{"username":"alice"}}`))
	assert.Contains(t, out, `"username":"alice"`)
}

// TestResponseJSONForLog_EmptyBody covers the DELETE / 204 path where ITX
// returns no body — the helper must not emit stray JSON.
func TestResponseJSONForLog_EmptyBody(t *testing.T) {
	assert.Equal(t, "", responseJSONForLog(nil))
	assert.Equal(t, "", responseJSONForLog([]byte("")))
}

// TestResponseJSONForLog_UnparseableBody protects against silently forwarding
// a body we couldn't parse. A malformed / array-shaped / non-JSON body must
// produce a safe marker instead of the raw bytes (which could carry PII).
func TestResponseJSONForLog_UnparseableBody(t *testing.T) {
	cases := []string{
		`not json`,
		`[{"created_by":{"email":"leak@example.com"}}]`, // JSON array, not an object
		`{malformed`,
	}
	for _, in := range cases {
		out := responseJSONForLog([]byte(in))
		assert.NotContains(t, out, "leak@example.com")
		assert.Contains(t, out, "failed to parse response for log")
	}
}

// TestResponseJSONForLog_UnexpectedAuditShape guards the fallback path where
// an audit field is present but not the expected object shape (e.g. a plain
// string). The value must be nulled rather than passed through.
func TestResponseJSONForLog_UnexpectedAuditShape(t *testing.T) {
	out := responseJSONForLog([]byte(`{"created_by":"alice@example.com"}`))
	assert.NotContains(t, out, "alice@example.com")
	assert.Contains(t, out, `"created_by":null`)
}

// TestResponseJSONForLog_LeavesNonAuditFieldsAlone confirms the redaction is
// scoped: unrelated response fields (id, name, description, timestamps, URLs)
// pass through unchanged so the log stays useful for debugging.
func TestResponseJSONForLog_LeavesNonAuditFieldsAlone(t *testing.T) {
	in := `{"id":"att-1","name":"notes.pdf","description":"Q3 notes","file_url":"https://example.com/notes.pdf","file_size":1024,"created_at":"2026-01-01T00:00:00Z"}`
	out := responseJSONForLog([]byte(in))

	assert.Contains(t, out, `"id":"att-1"`)
	assert.Contains(t, out, `"name":"notes.pdf"`)
	assert.Contains(t, out, `"description":"Q3 notes"`)
	assert.Contains(t, out, `"file_url":"https://example.com/notes.pdf"`)
	assert.Contains(t, out, `"file_size":1024`)
	assert.Contains(t, out, `"created_at":"2026-01-01T00:00:00Z"`)
}
