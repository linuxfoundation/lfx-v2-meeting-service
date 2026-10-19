// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package itx

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/models/itx"
)

func TestClient_DeleteRegistrant_Body(t *testing.T) {
	t.Parallel()

	type captured struct {
		method string
		path   string
		body   []byte
	}
	newServer := func(t *testing.T) (*Client, func() captured) {
		t.Helper()
		var got captured
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got.method, got.path = r.Method, r.URL.Path
			got.body, _ = io.ReadAll(r.Body)
			w.WriteHeader(http.StatusNoContent)
		}))
		t.Cleanup(server.Close)
		return NewClientWithHTTPClient(Config{BaseURL: server.URL}, server.Client()), func() captured { return got }
	}

	t.Run("admin removal sends no body", func(t *testing.T) {
		t.Parallel()
		client, got := newServer(t)

		require.NoError(t, client.DeleteRegistrant(context.Background(), "mtg-1", "reg-1", nil))

		assert.Equal(t, http.MethodDelete, got().method)
		assert.Equal(t, "/v2/zoom/meetings/mtg-1/registrants/reg-1", got().path)
		assert.Empty(t, got().body, `a nil request must not be sent as "null"`)
	})

	t.Run("self removal sends self_removal and updated_by", func(t *testing.T) {
		t.Parallel()
		client, got := newServer(t)

		err := client.DeleteRegistrant(context.Background(), "mtg-1", "reg-1", &itx.DeleteRegistrantRequest{
			SelfRemoval: true,
			UpdatedBy:   &itx.User{Username: "alice", Email: "alice@example.com"},
		})
		require.NoError(t, err)

		var body struct {
			SelfRemoval bool `json:"self_removal"`
			UpdatedBy   struct {
				Username string `json:"username"`
				Email    string `json:"email"`
			} `json:"updated_by"`
		}
		require.NoError(t, json.Unmarshal(got().body, &body), "body: %s", got().body)
		assert.True(t, body.SelfRemoval)
		assert.Equal(t, "alice", body.UpdatedBy.Username)
		assert.Equal(t, "alice@example.com", body.UpdatedBy.Email)
	})
}
