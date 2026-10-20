// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package userservice

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
)

const testToken = "test-user-token"

// testClient returns a Client pointed at the given test server.
func testClient(server *httptest.Server) *Client {
	return newClient(server.Client(), server.URL)
}

func TestGetSelf(t *testing.T) {
	t.Run("returns SFID and emails, forwarding the bearer token", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Equal(t, "/user-service/v1/me", r.URL.Path)
			assert.Equal(t, "Bearer "+testToken, r.Header.Get("Authorization"))
			_ = json.NewEncoder(w).Encode(meResponse{
				ID: "SFID1",
				Emails: []userEmail{
					{ID: "e-1", EmailAddress: "alice@work.com", Active: true, IsVerified: true},
					{ID: "e-2", EmailAddress: "old@work.com", Active: false, IsVerified: true},
				},
			})
		}))
		defer server.Close()

		self, err := testClient(server).GetSelf(context.Background(), testToken)
		require.NoError(t, err)
		assert.Equal(t, "SFID1", self.SFID)
		require.Len(t, self.Emails, 2)
		assert.Equal(t, "alice@work.com", self.Emails[0].Address)
		assert.True(t, self.Emails[0].Verified)
	})

	t.Run("strips a Bearer prefix from the provided token", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "Bearer "+testToken, r.Header.Get("Authorization"))
			_ = json.NewEncoder(w).Encode(meResponse{ID: "SFID1"})
		}))
		defer server.Close()

		_, err := testClient(server).GetSelf(context.Background(), "Bearer "+testToken)
		require.NoError(t, err)
	})

	t.Run("blank token is a validation error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
			t.Fatal("should not call the server for a blank token")
		}))
		defer server.Close()

		_, err := testClient(server).GetSelf(context.Background(), "  ")
		assert.Equal(t, domain.ErrorTypeValidation, domain.GetErrorType(err))
	})

	t.Run("empty ID is an internal error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(w).Encode(meResponse{})
		}))
		defer server.Close()

		_, err := testClient(server).GetSelf(context.Background(), testToken)
		assert.Equal(t, domain.ErrorTypeInternal, domain.GetErrorType(err))
	})
}

func TestGetMeetingEmailPreference(t *testing.T) {
	t.Run("returns preference when present", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Equal(t, "/user-service/v1/users/SFID1/preferences/emails", r.URL.Path)
			assert.Equal(t, "type eq meeting", r.URL.Query().Get("$filter"))
			assert.Equal(t, "Bearer "+testToken, r.Header.Get("Authorization"))
			_ = json.NewEncoder(w).Encode(emailPreferenceListResponse{Data: []emailPreference{
				{ID: "pref-1", EmailID: "email-sfid", Email: "alice@work.com", Type: "Meeting"},
			}})
		}))
		defer server.Close()

		pref, err := testClient(server).GetMeetingEmailPreference(context.Background(), testToken, "SFID1")
		require.NoError(t, err)
		require.NotNil(t, pref)
		assert.Equal(t, "pref-1", pref.PreferenceID)
		assert.Equal(t, "email-sfid", pref.EmailID)
	})

	t.Run("returns nil when no preference set", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(w).Encode(emailPreferenceListResponse{})
		}))
		defer server.Close()

		pref, err := testClient(server).GetMeetingEmailPreference(context.Background(), testToken, "SFID1")
		require.NoError(t, err)
		assert.Nil(t, pref)
	})
}

func TestSetMeetingEmailPreference_CreateWhenAbsent(t *testing.T) {
	var postBody createEmailPreferenceRequest
	posted := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(emailPreferenceListResponse{})
		case http.MethodPost:
			posted = true
			assert.Equal(t, "/user-service/v1/users/SFID1/preferences/emails", r.URL.Path)
			body, _ := io.ReadAll(r.Body)
			require.NoError(t, json.Unmarshal(body, &postBody))
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(emailPreference{ID: "pref-new", EmailID: "email-sfid", Email: "alice@work.com", Type: "Meeting"})
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	}))
	defer server.Close()

	pref, err := testClient(server).SetMeetingEmailPreference(context.Background(), testToken, "SFID1", "email-sfid")
	require.NoError(t, err)
	assert.True(t, posted, "expected POST when no preference exists")
	assert.Equal(t, "email-sfid", postBody.EmailID)
	assert.Equal(t, "Meeting", postBody.Type)
	assert.True(t, postBody.IsDefault)
	assert.Equal(t, "pref-new", pref.PreferenceID)
}

func TestSetMeetingEmailPreference_PatchWhenPresent(t *testing.T) {
	patched := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(emailPreferenceListResponse{Data: []emailPreference{
				{ID: "pref-1", EmailID: "old", Email: "old@work.com", Type: "Meeting"},
			}})
		case http.MethodPatch:
			patched = true
			assert.Equal(t, "/user-service/v1/users/SFID1/preferences/emails/pref-1", r.URL.Path)
			// The PATCH body must NOT include Type — sending it on the update path makes the
			// upstream user-service return an empty-body 502 (the write still lands).
			var raw map[string]any
			body, _ := io.ReadAll(r.Body)
			require.NoError(t, json.Unmarshal(body, &raw))
			_, hasType := raw["Type"]
			assert.False(t, hasType, "PATCH body must not include Type")
			assert.Equal(t, "new", raw["EmailID"])
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(emailPreference{ID: "pref-1", EmailID: "new", Email: "new@work.com", Type: "Meeting"})
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	}))
	defer server.Close()

	pref, err := testClient(server).SetMeetingEmailPreference(context.Background(), testToken, "SFID1", "new")
	require.NoError(t, err)
	assert.True(t, patched, "expected PATCH when a preference already exists")
	assert.Equal(t, "new", pref.EmailID)
}

func TestSetMeetingEmailPreference_WriteErrorButPersisted(t *testing.T) {
	// The PATCH returns a bodyless 502 (as observed against dev), but the change is
	// persisted — the follow-up GET reflects the new EmailID. Expect success.
	patched := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPatch {
			patched = true
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		emailID := "old"
		if patched {
			emailID = "new"
		}
		_ = json.NewEncoder(w).Encode(emailPreferenceListResponse{Data: []emailPreference{
			{ID: "pref-1", EmailID: emailID, Email: "x@work.com", Type: "Meeting"},
		}})
	}))
	defer server.Close()

	pref, err := testClient(server).SetMeetingEmailPreference(context.Background(), testToken, "SFID1", "new")
	require.NoError(t, err)
	require.NotNil(t, pref)
	assert.Equal(t, "new", pref.EmailID)
}

func TestSetMeetingEmailPreference_WriteErrorNotPersisted(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPatch {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		_ = json.NewEncoder(w).Encode(emailPreferenceListResponse{Data: []emailPreference{
			{ID: "pref-1", EmailID: "old", Email: "x@work.com", Type: "Meeting"},
		}})
	}))
	defer server.Close()

	_, err := testClient(server).SetMeetingEmailPreference(context.Background(), testToken, "SFID1", "new")
	require.Error(t, err)
	// A 502 that did not persist maps to a retryable Unavailable error.
	assert.Equal(t, domain.ErrorTypeUnavailable, domain.GetErrorType(err))
}

func TestClearMeetingEmailPreference_DeleteErrorButGone(t *testing.T) {
	deleted := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			deleted = true
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		if deleted {
			_ = json.NewEncoder(w).Encode(emailPreferenceListResponse{})
			return
		}
		_ = json.NewEncoder(w).Encode(emailPreferenceListResponse{Data: []emailPreference{
			{ID: "pref-1", EmailID: "e", Email: "e@work.com", Type: "Meeting"},
		}})
	}))
	defer server.Close()

	require.NoError(t, testClient(server).ClearMeetingEmailPreference(context.Background(), testToken, "SFID1"))
}

func TestSetMeetingEmailPreference_BlankEmailID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Fatal("should not call the server for a blank email_id")
	}))
	defer server.Close()

	_, err := testClient(server).SetMeetingEmailPreference(context.Background(), testToken, "SFID1", "  ")
	assert.Equal(t, domain.ErrorTypeValidation, domain.GetErrorType(err))
}

func TestClearMeetingEmailPreference(t *testing.T) {
	t.Run("deletes existing preference", func(t *testing.T) {
		deleted := false
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet:
				_ = json.NewEncoder(w).Encode(emailPreferenceListResponse{Data: []emailPreference{
					{ID: "pref-1", EmailID: "e", Email: "e@work.com", Type: "Meeting"},
				}})
			case http.MethodDelete:
				deleted = true
				assert.Equal(t, "/user-service/v1/users/SFID1/preferences/emails/pref-1", r.URL.Path)
				w.WriteHeader(http.StatusNoContent)
			default:
				t.Fatalf("unexpected method %s", r.Method)
			}
		}))
		defer server.Close()

		require.NoError(t, testClient(server).ClearMeetingEmailPreference(context.Background(), testToken, "SFID1"))
		assert.True(t, deleted, "expected DELETE for existing preference")
	})

	t.Run("no-op when no preference exists", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodDelete {
				t.Fatal("should not DELETE when no preference exists")
			}
			_ = json.NewEncoder(w).Encode(emailPreferenceListResponse{})
		}))
		defer server.Close()

		require.NoError(t, testClient(server).ClearMeetingEmailPreference(context.Background(), testToken, "SFID1"))
	})
}

func TestDoJSON_ResponseBodyCap(t *testing.T) {
	// streamBytes writes n bytes of b to w in chunks, stopping early once the client
	// hangs up. n is finite only so a regression fails the test instead of hanging it.
	streamBytes := func(w io.Writer, b byte, n int) {
		chunk := bytes.Repeat([]byte{b}, 32<<10)
		for written := 0; written < n; written += len(chunk) {
			if _, err := w.Write(chunk); err != nil {
				return
			}
		}
	}
	// io.ReadAll grows its buffer in steps, allocating a few times the cap in total
	// while reading up to it. The bound still sits far below the 256 MiB streamed, so
	// it fails if the whole body is ever buffered.
	allocBound := uint64(8 * maxResponseBodyBytes)

	t.Run("endless body fails with bounded memory", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"ID":"`))
			streamBytes(w, 'a', 256<<20)
		}))
		defer server.Close()

		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		_, err := testClient(server).GetSelf(context.Background(), testToken)
		runtime.ReadMemStats(&after)

		require.Error(t, err)
		assert.Equal(t, domain.ErrorTypeInternal, domain.GetErrorType(err))
		assert.Contains(t, err.Error(), "get_self response exceeds")
		assert.Less(t, after.TotalAlloc-before.TotalAlloc, allocBound,
			"reading the response must not buffer the oversized body")
	})

	t.Run("gzip-amplified body is capped after decompression", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Content-Encoding", "gzip")
			gz, _ := gzip.NewWriterLevel(w, gzip.BestCompression)
			_, _ = gz.Write([]byte(`{"ID":"`))
			streamBytes(gz, 'a', 256<<20)
			_ = gz.Close()
		}))
		defer server.Close()

		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		_, err := testClient(server).GetSelf(context.Background(), testToken)
		runtime.ReadMemStats(&after)

		require.Error(t, err)
		assert.Equal(t, domain.ErrorTypeInternal, domain.GetErrorType(err))
		assert.Contains(t, err.Error(), "exceeds")
		assert.Less(t, after.TotalAlloc-before.TotalAlloc, allocBound,
			"reading the response must not buffer the decompressed body")
	})

	t.Run("oversized error body keeps its status classification", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
			streamBytes(w, 'x', 2*maxResponseBodyBytes)
		}))
		defer server.Close()

		_, err := testClient(server).GetSelf(context.Background(), testToken)
		require.Error(t, err)
		assert.Equal(t, domain.ErrorTypeUnavailable, domain.GetErrorType(err))
	})

	t.Run("large /me with a big Permissions list decodes", func(t *testing.T) {
		// /me always carries the user's full Permissions list, which outgrew the old
		// 256 KiB cap (lfx-self-serve-ops#881). Build a real-shaped body well past it.
		perms := make([]map[string]string, 0, 8000)
		for len(perms) < cap(perms) {
			perms = append(perms, map[string]string{
				"ObjectID": "00000000-0000-0000-0000-000000000001", "ObjectType": "project", "Role": "viewer",
			})
		}
		body, err := json.Marshal(map[string]any{
			"ID":          "SFID1",
			"Emails":      []userEmail{{ID: "e-1", EmailAddress: "alice@example.com", Active: true, IsVerified: true}},
			"Permissions": perms,
		})
		require.NoError(t, err)
		require.Greater(t, len(body), 512<<10, "body must exceed the old cap to cover the regression")

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write(body)
		}))
		defer server.Close()

		self, err := testClient(server).GetSelf(context.Background(), testToken)
		require.NoError(t, err)
		assert.Equal(t, "SFID1", self.SFID)
		require.Len(t, self.Emails, 1)
		assert.Equal(t, "alice@example.com", self.Emails[0].Address)
	})

	t.Run("body at the cap is accepted", func(t *testing.T) {
		prefix, suffix := `{"ID":"SFID1","pad":"`, `"}`
		pad := strings.Repeat("a", maxResponseBodyBytes-len(prefix)-len(suffix))
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(prefix + pad + suffix))
		}))
		defer server.Close()

		self, err := testClient(server).GetSelf(context.Background(), testToken)
		require.NoError(t, err)
		assert.Equal(t, "SFID1", self.SFID)
	})
}

func TestMapHTTPError(t *testing.T) {
	tests := []struct {
		status int
		want   domain.ErrorType
	}{
		{http.StatusBadRequest, domain.ErrorTypeValidation},
		{http.StatusUnauthorized, domain.ErrorTypeValidation},
		{http.StatusForbidden, domain.ErrorTypeValidation},
		{http.StatusNotFound, domain.ErrorTypeNotFound},
		{http.StatusConflict, domain.ErrorTypeConflict},
		{http.StatusTooManyRequests, domain.ErrorTypeUnavailable},
		{http.StatusBadGateway, domain.ErrorTypeUnavailable},
		{http.StatusServiceUnavailable, domain.ErrorTypeUnavailable},
		{http.StatusGatewayTimeout, domain.ErrorTypeUnavailable},
		{http.StatusTeapot, domain.ErrorTypeInternal},
	}
	for _, tt := range tests {
		err := mapHTTPError(tt.status, []byte(`{"Message":"boom"}`))
		assert.Equal(t, tt.want, domain.GetErrorType(err))
	}

	// The caller keeps the upstream detail, but what a log attribute emits must not carry
	// any upstream body text (it can echo the user's email addresses).
	logOutput := func(err error) string {
		var buf bytes.Buffer
		slog.New(slog.NewJSONHandler(&buf, nil)).Warn("failed", logging.ErrKey, err)
		return buf.String()
	}
	t.Run("structured message reaches the caller but not the log", func(t *testing.T) {
		err := mapHTTPError(http.StatusConflict, []byte(`{"Message":"email alice@example.com already exists"}`))
		assert.Contains(t, err.Error(), "alice@example.com")

		out := logOutput(err)
		assert.NotContains(t, out, "alice@example.com")
		assert.Contains(t, out, "user-service returned HTTP 409 (conflict)")
	})
	t.Run("raw body snippet reaches the caller but not the log", func(t *testing.T) {
		err := mapHTTPError(http.StatusBadGateway, []byte("upstream failed for alice@example.com"))
		assert.Contains(t, err.Error(), "alice@example.com")

		out := logOutput(err)
		assert.NotContains(t, out, "alice@example.com")
		assert.Contains(t, out, "user-service returned HTTP 502 (unavailable)")
	})
	t.Run("rejected token message is not logged either", func(t *testing.T) {
		err := mapHTTPError(http.StatusForbidden, []byte(`{"error":"token for alice@example.com rejected"}`))
		assert.Contains(t, err.Error(), "alice@example.com")

		out := logOutput(err)
		assert.NotContains(t, out, "alice@example.com")
		assert.Contains(t, out, "user-service returned HTTP 403 (validation)")
	})
}
