// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package itx

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_ListMeetingAttachments(t *testing.T) {
	t.Parallel()

	newServer := func(t *testing.T, status int, body string) (*Client, func() *http.Request) {
		t.Helper()
		var got *http.Request
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got = r.Clone(r.Context())
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			_, _ = w.Write([]byte(body))
		}))
		t.Cleanup(server.Close)
		return NewClientWithHTTPClient(Config{BaseURL: server.URL}, server.Client()), func() *http.Request { return got }
	}

	t.Run("without an occurrence it sends no occurrence_id and decodes the list", func(t *testing.T) {
		t.Parallel()
		client, got := newServer(t, http.StatusOK, `[{"id":"att-1","meeting_id":"mtg-1"},{"id":"att-2","meeting_id":"mtg-1","occurrence_id":"1788000000"}]`)

		attachments, err := client.ListMeetingAttachments(context.Background(), "mtg-1", "")
		require.NoError(t, err)

		assert.Equal(t, http.MethodGet, got().Method)
		assert.Equal(t, "/v2/zoom/meetings/mtg-1/attachments", got().URL.Path)
		assert.False(t, got().URL.Query().Has("occurrence_id"))
		require.Len(t, attachments, 2)
		assert.Empty(t, attachments[0].OccurrenceID)
		assert.Equal(t, "1788000000", attachments[1].OccurrenceID)
	})

	t.Run("with an occurrence it passes occurrence_id to ITX", func(t *testing.T) {
		t.Parallel()
		client, got := newServer(t, http.StatusOK, `[]`)

		attachments, err := client.ListMeetingAttachments(context.Background(), "mtg-1", "1788000000")
		require.NoError(t, err)

		assert.Equal(t, "1788000000", got().URL.Query().Get("occurrence_id"))
		assert.NotNil(t, attachments, "an empty list must be an empty slice, not nil")
		assert.Empty(t, attachments)
	})

	t.Run("a null body is returned as an empty slice", func(t *testing.T) {
		t.Parallel()
		client, _ := newServer(t, http.StatusOK, `null`)

		attachments, err := client.ListMeetingAttachments(context.Background(), "mtg-1", "")
		require.NoError(t, err)
		assert.NotNil(t, attachments)
		assert.Empty(t, attachments)
	})

	t.Run("a missing meeting is an error", func(t *testing.T) {
		t.Parallel()
		client, _ := newServer(t, http.StatusNotFound, `{"code":"404","message":"meeting not found"}`)

		_, err := client.ListMeetingAttachments(context.Background(), "nope", "")
		require.Error(t, err)
	})
}
