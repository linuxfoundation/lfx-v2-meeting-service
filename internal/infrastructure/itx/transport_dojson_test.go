// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package itx

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
)

func TestDoJSONTyped_rejectsEmptyResponseBody(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	client := NewClientWithHTTPClient(Config{BaseURL: server.URL}, server.Client())
	_, err := doJSONTyped[struct {
		ID string `json:"id"`
	}](client, context.Background(), apiRequest{
		method:   http.MethodGet,
		path:     "/v2/zoom/meetings/%s",
		pathArgs: []any{"meeting-id"},
		accept:   acceptJSON,
	})
	if err == nil {
		t.Fatal("expected error for empty JSON response body")
	}
	if domain.GetErrorType(err) != domain.ErrorTypeInternal {
		t.Fatalf("GetErrorType() = %v, want %v (err=%v)", domain.GetErrorType(err), domain.ErrorTypeInternal, err)
	}
}
