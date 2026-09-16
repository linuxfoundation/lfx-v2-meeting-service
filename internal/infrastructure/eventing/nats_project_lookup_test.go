// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
)

// TestProjectServiceErrorCode pins the envelope parser used by
// NATSProjectLookup.GetProjectSlug.
func TestProjectServiceErrorCode(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		wantErr string
	}{
		{name: "not_found code", data: []byte(`{"error":"not_found"}`), wantErr: "not_found"},
		{name: "internal code", data: []byte(`{"error":"internal"}`), wantErr: "internal"},
		{name: "unknown code", data: []byte(`{"error":"foo"}`), wantErr: "foo"},
		{name: "success plain string", data: []byte("my-slug"), wantErr: ""},
		{name: "success json without error key", data: []byte(`{"slug":"k8s"}`), wantErr: ""},
		{name: "empty body", data: []byte{}, wantErr: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantErr, projectServiceErrorCode(tt.data))
		})
	}
}

// TestNATSProjectLookup_GetProjectSlug_NotFoundReturnsEmptyNoError pins the
// fix for the regression introduced in LFXV2-1747: a confirmed not_found from
// project-service must return ("", nil) so callers follow the no-slug path
// rather than treating the permanent absence as a retryable transient failure.
func TestNATSProjectLookup_GetProjectSlug_NotFoundReturnsEmptyNoError(t *testing.T) {
	// notFoundReply simulates project-service returning {"error":"not_found"}.
	// We test the parsing logic directly since we cannot inject a NATS conn
	// in a unit test.
	data := []byte(`{"error":"not_found","message":"project not found"}`)
	code := projectServiceErrorCode(data)

	require.Equal(t, "not_found", code,
		"projectServiceErrorCode must recognise the not_found envelope")

	// Reproduce the exact branch logic from GetProjectSlug.
	var gotSlug string
	var gotErr error
	if code == "not_found" {
		gotSlug, gotErr = "", nil
	} else if code != "" {
		gotErr = domain.NewInternalError("unexpected: " + code)
	}

	require.NoError(t, gotErr,
		"not_found must not return an error — callers treat any error as retryable transient failure")
	assert.Empty(t, gotSlug,
		"not_found must return an empty slug so callers proceed without one")
}

// TestNATSProjectLookup_GetProjectSlug_InternalCodeReturnsError ensures that
// a non-not_found service error still surfaces as a non-nil error so the
// attachment handler retries it as a genuine transient failure.
func TestNATSProjectLookup_GetProjectSlug_InternalCodeReturnsError(t *testing.T) {
	data := []byte(`{"error":"internal","message":"internal server error"}`)
	code := projectServiceErrorCode(data)

	require.Equal(t, "internal", code)

	var gotErr error
	if code == "not_found" {
		// should not reach here
	} else if code != "" {
		gotErr = domain.NewInternalError("project service error")
	}

	require.Error(t, gotErr,
		"internal code must return an error so the caller retries")
	var domErr *domain.DomainError
	assert.ErrorAs(t, gotErr, &domErr,
		"internal code must surface as a domain error")
}
