// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package userservice

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
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
}
