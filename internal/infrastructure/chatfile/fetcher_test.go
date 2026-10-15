// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package chatfile

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTLSServer starts a TLS test server (the fetcher accepts https links only) and
// returns a Fetcher trusting its certificate.
func newTLSServer(t *testing.T, timeout time.Duration, handler http.Handler) (*httptest.Server, *Fetcher) {
	t.Helper()
	srv := httptest.NewTLSServer(handler)
	t.Cleanup(srv.Close)
	return srv, NewFetcher(timeout, srv.Client().Transport)
}

func TestFetcher_FetchText(t *testing.T) {
	const body = "00:00:01\tFrom Alice Example to Everyone:\thello\n"

	t.Run("returns a text body", func(t *testing.T) {
		srv, f := newTLSServer(t, time.Second, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			_, _ = w.Write([]byte(body))
		}))
		data, truncated, err := f.FetchText(context.Background(), srv.URL+"/chat.txt", 1024)
		require.NoError(t, err)
		assert.False(t, truncated)
		assert.Equal(t, body, string(data))
	})

	t.Run("accepts any text subtype", func(t *testing.T) {
		srv, f := newTLSServer(t, time.Second, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/vtt")
			_, _ = w.Write([]byte(body))
		}))
		_, _, err := f.FetchText(context.Background(), srv.URL, 1024)
		require.NoError(t, err)
	})

	t.Run("refuses a non-text content type", func(t *testing.T) {
		srv, f := newTLSServer(t, time.Second, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write([]byte(body))
		}))
		_, _, err := f.FetchText(context.Background(), srv.URL, 1024)
		assert.ErrorIs(t, err, ErrNotText)
	})

	t.Run("refuses an html sign-in page even on 200", func(t *testing.T) {
		srv, f := newTLSServer(t, time.Second, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte("<html>sign in</html>"))
		}))
		_, _, err := f.FetchText(context.Background(), srv.URL, 1024)
		assert.ErrorIs(t, err, ErrNotText)
	})

	t.Run("refuses a missing content type", func(t *testing.T) {
		srv, f := newTLSServer(t, time.Second, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header()["Content-Type"] = nil // suppress Go's sniffing default
			_, _ = w.Write([]byte(body))
		}))
		_, _, err := f.FetchText(context.Background(), srv.URL, 1024)
		assert.ErrorIs(t, err, ErrNotText)
	})

	t.Run("maps a non-2xx status to ErrStatus without the URL", func(t *testing.T) {
		srv, f := newTLSServer(t, time.Second, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "gone", http.StatusNotFound)
		}))
		_, _, err := f.FetchText(context.Background(), srv.URL+"/secret-token", 1024)
		require.ErrorIs(t, err, ErrStatus)
		assert.Contains(t, err.Error(), "HTTP 404")
		assert.NotContains(t, err.Error(), "secret-token")
		assert.NotContains(t, err.Error(), srv.URL)
	})

	t.Run("times out a slow server", func(t *testing.T) {
		release := make(chan struct{})
		t.Cleanup(func() { close(release) })
		srv, f := newTLSServer(t, 100*time.Millisecond, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			select {
			case <-release:
			case <-r.Context().Done():
			}
		}))
		start := time.Now()
		_, _, err := f.FetchText(context.Background(), srv.URL, 1024)
		assert.ErrorIs(t, err, ErrTimeout)
		assert.Less(t, time.Since(start), 5*time.Second)
	})

	t.Run("times out a slow body", func(t *testing.T) {
		release := make(chan struct{})
		t.Cleanup(func() { close(release) })
		srv, f := newTLSServer(t, 100*time.Millisecond, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("partial"))
			w.(http.Flusher).Flush()
			select {
			case <-release:
			case <-r.Context().Done():
			}
		}))
		_, _, err := f.FetchText(context.Background(), srv.URL, 1024)
		assert.ErrorIs(t, err, ErrTimeout)
	})

	t.Run("caps the body by streamed bytes, not Content-Length", func(t *testing.T) {
		long := strings.Repeat("x", 100)
		srv, f := newTLSServer(t, time.Second, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			w.Header().Set("Content-Length", "10") // understated on purpose
			_, _ = w.Write([]byte(long))
		}))
		data, truncated, err := f.FetchText(context.Background(), srv.URL, 64)
		if err != nil {
			// Go's client may reject the mismatched Content-Length as a transport
			// error; either way the caller never receives more than the cap.
			assert.ErrorIs(t, err, ErrTransport)
			return
		}
		assert.True(t, truncated)
		assert.Len(t, data, 64)
	})

	t.Run("marks a chunked body over the cap as truncated", func(t *testing.T) {
		long := strings.Repeat("y", 100)
		srv, f := newTLSServer(t, time.Second, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			for i := 0; i < len(long); i += 10 {
				_, _ = w.Write([]byte(long[i : i+10]))
				w.(http.Flusher).Flush()
			}
		}))
		data, truncated, err := f.FetchText(context.Background(), srv.URL, 64)
		require.NoError(t, err)
		assert.True(t, truncated)
		assert.Equal(t, long[:64], string(data))
	})

	t.Run("a body exactly at the cap is not truncated", func(t *testing.T) {
		exact := strings.Repeat("z", 64)
		srv, f := newTLSServer(t, time.Second, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte(exact))
		}))
		data, truncated, err := f.FetchText(context.Background(), srv.URL, 64)
		require.NoError(t, err)
		assert.False(t, truncated)
		assert.Equal(t, exact, string(data))
	})

	t.Run("follows an https redirect to text", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("/start", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/final", http.StatusFound)
		})
		mux.HandleFunc("/final", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte(body))
		})
		srv, f := newTLSServer(t, time.Second, mux)
		data, _, err := f.FetchText(context.Background(), srv.URL+"/start", 1024)
		require.NoError(t, err)
		assert.Equal(t, body, string(data))
	})

	t.Run("sends no Referer to a cross-origin redirect destination", func(t *testing.T) {
		var gotReferer, gotHeader string
		var destinationHits int
		destination := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			destinationHits++
			gotReferer = r.Referer()
			gotHeader = r.Header.Get("Referer")
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte(body))
		}))
		t.Cleanup(destination.Close)

		origin := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, destination.URL+"/final", http.StatusFound)
		}))
		t.Cleanup(origin.Close)

		// Trust both servers' certificates: they are distinct origins.
		transport := origin.Client().Transport.(*http.Transport).Clone()
		transport.TLSClientConfig.InsecureSkipVerify = true //nolint:gosec // test-only: two self-signed httptest origins
		f := NewFetcher(time.Second, transport)

		data, _, err := f.FetchText(context.Background(), origin.URL+"/download?access_token=synthetic-secret", 1024)
		require.NoError(t, err)
		assert.Equal(t, body, string(data))
		assert.Equal(t, 1, destinationHits)
		assert.Empty(t, gotReferer)
		assert.Empty(t, gotHeader)
	})

	t.Run("sends no Referer on a same-origin redirect either", func(t *testing.T) {
		var gotReferer string
		mux := http.NewServeMux()
		mux.HandleFunc("/start", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/final", http.StatusFound)
		})
		mux.HandleFunc("/final", func(w http.ResponseWriter, r *http.Request) {
			gotReferer = r.Header.Get("Referer")
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte(body))
		})
		srv, f := newTLSServer(t, time.Second, mux)
		_, _, err := f.FetchText(context.Background(), srv.URL+"/start?access_token=synthetic-secret", 1024)
		require.NoError(t, err)
		assert.Empty(t, gotReferer)
	})

	t.Run("refuses a redirect off https", func(t *testing.T) {
		srv, f := newTLSServer(t, time.Second, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "http://example.test/chat.txt", http.StatusFound)
		}))
		_, _, err := f.FetchText(context.Background(), srv.URL, 1024)
		assert.ErrorIs(t, err, ErrInvalidLink)
	})

	t.Run("refuses links that are not https", func(t *testing.T) {
		f := NewFetcher(time.Second, nil)
		for _, link := range []string{"", "http://example.test/chat.txt", "ftp://example.test/x", "chat.txt", "https://"} {
			_, _, err := f.FetchText(context.Background(), link, 1024)
			assert.ErrorIs(t, err, ErrInvalidLink, link)
		}
	})

	t.Run("maps a connection failure to ErrTransport without the URL", func(t *testing.T) {
		srv, f := newTLSServer(t, time.Second, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		link := srv.URL + "/token-in-path"
		srv.Close()
		_, _, err := f.FetchText(context.Background(), link, 1024)
		require.Error(t, err)
		assert.True(t, err == ErrTransport || err == ErrTimeout, err)
		assert.NotContains(t, err.Error(), "token-in-path")
	})
}
