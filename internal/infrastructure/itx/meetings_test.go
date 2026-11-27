// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package itx

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/models/itx"
)

func TestClient_DeleteZoomMeeting_Body(t *testing.T) {
	t.Parallel()

	type captured struct {
		path        string
		contentType string
		body        []byte
	}
	newServer := func(t *testing.T) (*Client, func() captured) {
		t.Helper()
		var got captured
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got.path, got.contentType = r.URL.Path, r.Header.Get("Content-Type")
			got.body, _ = io.ReadAll(r.Body)
			w.WriteHeader(http.StatusNoContent)
		}))
		t.Cleanup(server.Close)
		return NewClientWithHTTPClient(Config{BaseURL: server.URL}, server.Client()), func() captured { return got }
	}

	t.Run("a nil request sends no body", func(t *testing.T) {
		t.Parallel()
		client, got := newServer(t)

		require.NoError(t, client.DeleteZoomMeeting(context.Background(), "mtg-1", nil))

		assert.Equal(t, "/v2/zoom/meetings/mtg-1", got().path)
		assert.Empty(t, got().body, `a nil request must not be sent as "null"`)
		assert.Empty(t, got().contentType)
	})

	t.Run("a note is sent with updated_by", func(t *testing.T) {
		t.Parallel()
		client, got := newServer(t)

		err := client.DeleteZoomMeeting(context.Background(), "mtg-1", &itx.DeleteMeetingRequest{
			Note:      "Series ended",
			UpdatedBy: &itx.User{Username: "alice"},
		})
		require.NoError(t, err)
		assert.Equal(t, "application/json", got().contentType)
		assert.JSONEq(t, `{"note":"Series ended","updated_by":{"id":"","username":"alice","name":"","email":""}}`, string(got().body))
	})
}
