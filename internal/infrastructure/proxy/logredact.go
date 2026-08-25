// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package proxy

import (
	"encoding/json"

	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/models/itx"
)

// auditFieldsForResponseRedaction lists the JSON keys on ITX attachment
// response bodies whose values carry user-identity PII (requester name +
// email) and must be redacted before appearing in debug logs.
//
// Kept as a package-level var so it can be extended if ITX adds new
// audit-user fields in the future.
var auditFieldsForResponseRedaction = []string{
	"created_by",
	"updated_by",
	"modified_by",
	"file_uploaded_by",
}

// RequestJSONForLog is the exported form of requestJSONForLog, for use by
// packages that share the same PII-redaction requirement.
func RequestJSONForLog(req any) string { return requestJSONForLog(req) }

// ResponseJSONForLog is the exported form of responseJSONForLog, for use by
// packages that share the same PII-redaction requirement.
func ResponseJSONForLog(respBody []byte) string { return responseJSONForLog(respBody) }

// requestJSONForLog returns a JSON-encoded string of req with any
// user-identity audit fields (CreatedBy / UpdatedBy) stripped, so that
// debug-level logging of ITX request bodies does not leak requester PII
// (name / email) into logs.
//
// The wire body sent to ITX is unaffected; this only produces the payload
// used inside slog.DebugContext(..., "request", ...) calls.
//
// Only attachment request types currently carry these audit fields on this
// service's outbound calls (POST/PUT/presign for meeting attachments and
// past-meeting attachments). Other request types pass through unchanged.
// The itx.User and itx.CreatedUpdatedBy types also implement slog.LogValuer
// (see pkg/models/itx), which handles the case where they're logged directly
// as a slog attribute value; this helper handles the harder case of an audit
// field nested inside a JSON-encoded request body, which slog cannot resolve
// on its own.
func requestJSONForLog(req any) string {
	b, err := json.Marshal(redactAuditForLog(req))
	if err != nil {
		// Fall back to a marker rather than logging the raw body, which
		// would defeat the purpose of the redaction.
		return `{"error":"failed to marshal request for log"}`
	}
	return string(b)
}

// redactAuditForLog returns a shallow copy of req with any CreatedBy /
// UpdatedBy audit fields cleared. The original req is not mutated. Types
// without audit fields (or that are nil) are returned unchanged.
func redactAuditForLog(req any) any {
	switch r := req.(type) {
	case *itx.CreateMeetingAttachmentRequest:
		if r == nil {
			return req
		}
		cp := *r
		cp.CreatedBy = nil
		return &cp
	case *itx.UpdateMeetingAttachmentRequest:
		if r == nil {
			return req
		}
		cp := *r
		cp.UpdatedBy = nil
		return &cp
	case *itx.CreateAttachmentPresignRequest:
		if r == nil {
			return req
		}
		cp := *r
		cp.CreatedBy = nil
		return &cp
	case *itx.CreatePastMeetingAttachmentRequest:
		if r == nil {
			return req
		}
		cp := *r
		cp.CreatedBy = nil
		return &cp
	case *itx.UpdatePastMeetingAttachmentRequest:
		if r == nil {
			return req
		}
		cp := *r
		cp.UpdatedBy = nil
		return &cp
	default:
		return req
	}
}

// responseJSONForLog returns a JSON-encoded string of respBody with any
// user-identity audit fields (created_by / updated_by / modified_by /
// file_uploaded_by) redacted to just their username, so that debug-level
// logging of ITX attachment response bodies does not leak PII. The response
// consumed by the service is unaffected — this only produces the payload
// used inside slog.DebugContext(..., "response", ...) calls.
//
// This complements requestJSONForLog and covers three PII vectors on the
// response side:
//  1. Attachment create/update/presign echo back the requester's created_by /
//     updated_by, which now carries name + email.
//  2. GET attachment responses expose the persisted values from whoever
//     originally created / last updated / uploaded the attachment — which
//     may be a different user than the caller.
//  3. Some responses carry file_uploaded_by, populated by ITX when a file
//     upload completes.
//
// The redaction shape mirrors the itx.CreatedUpdatedBy LogValue: only the
// username survives, so a debug operator can still see who touched what.
// Non-audit response fields (id, name, description, timestamps, etc.) are
// left intact so the log remains useful.
func responseJSONForLog(respBody []byte) string {
	if len(respBody) == 0 {
		return ""
	}
	var parsed map[string]any
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		// Non-JSON, JSON-array, or malformed body: return a safe marker
		// rather than logging bytes we can't confirm are PII-free.
		return `{"error":"failed to parse response for log"}`
	}
	for _, key := range auditFieldsForResponseRedaction {
		raw, ok := parsed[key]
		if !ok || raw == nil {
			continue
		}
		if m, ok := raw.(map[string]any); ok {
			parsed[key] = map[string]any{"username": m["username"]}
		} else {
			// Unexpected shape — safest to strip the whole value.
			parsed[key] = nil
		}
	}
	b, err := json.Marshal(parsed)
	if err != nil {
		return `{"error":"failed to marshal response for log"}`
	}
	return string(b)
}
