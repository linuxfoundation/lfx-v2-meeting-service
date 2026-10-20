// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package chatfile

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
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

	t.Run("refuses non-text content types other than octet-stream", func(t *testing.T) {
		for _, contentType := range []string{"application/json", "image/png", "application/pdf", "video/mp4"} {
			t.Run(contentType, func(t *testing.T) {
				srv, f := newTLSServer(t, time.Second, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Set("Content-Type", contentType)
					_, _ = w.Write([]byte(body))
				}))
				_, _, err := f.FetchText(context.Background(), srv.URL, 1024)
				assert.ErrorIs(t, err, ErrNotText)
			})
		}
	})

	// The recording provider labels its recording files application/octet-stream
	// on production, chat files included; the label is accepted and the body is
	// returned as is.
	t.Run("accepts an octet-stream chat body", func(t *testing.T) {
		for _, contentType := range []string{"application/octet-stream", "application/octet-stream; charset=binary"} {
			t.Run(contentType, func(t *testing.T) {
				srv, f := newTLSServer(t, time.Second, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Set("Content-Type", contentType)
					_, _ = w.Write([]byte(body))
				}))
				data, truncated, err := f.FetchText(context.Background(), srv.URL+"/chat.txt", 1024)
				require.NoError(t, err)
				assert.False(t, truncated)
				assert.Equal(t, body, string(data))
			})
		}
	})

	t.Run("accepts an empty octet-stream body", func(t *testing.T) {
		srv, f := newTLSServer(t, time.Second, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/octet-stream")
		}))
		data, truncated, err := f.FetchText(context.Background(), srv.URL, 1024)
		require.NoError(t, err)
		assert.False(t, truncated)
		assert.Empty(t, data)
	})

	// A sign-in or error page can arrive under any label: the body is sniffed
	// and an HTML document is refused whatever the content type said. The error
	// is ErrHTMLBody, which matches ErrNotText and carries nothing of the body.
	t.Run("refuses an html document under an accepted label", func(t *testing.T) {
		for name, tc := range map[string]struct{ contentType, body string }{
			"octet-stream doctype":         {"application/octet-stream", "<!DOCTYPE html><html><body>Sign in</body></html>"},
			"text/plain html tag":          {"text/plain; charset=utf-8", "<html><body>Sign in</body></html>"},
			"bom then lowercase doctype":   {"application/octet-stream", "\xEF\xBB\xBF<!doctype html><html></html>"},
			"leading whitespace then html": {"text/plain", "\n\n  \t<html lang=\"en\"><head></head></html>"},
			"uppercase html tag":           {"application/octet-stream", "<HTML><BODY>Error</BODY></HTML>"},
			"leading comment then doctype": {"application/octet-stream", "<!-- provider page -->\n<!DOCTYPE html><html><body>Sign in</body></html>"},
			"two comments then html tag":   {"text/plain", "<!--a-->\n<!-- b -- c -->  <html><body>Sign in</body></html>"},
			"xml prolog then xhtml":        {"application/octet-stream", "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<!DOCTYPE html PUBLIC \"-//W3C//DTD XHTML 1.0//EN\"><html>Sign in</html>"},
			"bom, comment and whitespace":  {"application/octet-stream", "\xEF\xBB\xBF \n<!-- x -->\r\n\t<html>Sign in</html>"},
			"unterminated leading comment": {"application/octet-stream", "<!-- the page never closes this Sign in"},
		} {
			t.Run(name, func(t *testing.T) {
				srv, f := newTLSServer(t, time.Second, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Set("Content-Type", tc.contentType)
					_, _ = w.Write([]byte(tc.body))
				}))
				data, _, err := f.FetchText(context.Background(), srv.URL, 1024)
				require.ErrorIs(t, err, ErrHTMLBody)
				assert.ErrorIs(t, err, ErrNotText)
				assert.Nil(t, data)
				assert.NotContains(t, err.Error(), "Sign in")
				assert.NotContains(t, err.Error(), srv.URL)
			})
		}
	})

	// The sniff refuses only an HTML document: a chat file that starts with a
	// byte-order mark, with "<", or with fewer bytes than "<!DOCTYPE" is left to
	// the parser.
	t.Run("the html sniff leaves non-html bodies alone", func(t *testing.T) {
		for name, b := range map[string]string{
			"bom then chat header": "\xEF\xBB\xBF" + body,
			"starts with <":        "<meeting notes>\n" + body,
			"shorter than doctype": "<!DOC",
			"comment then chat":    "<!-- exported --> " + body,
		} {
			t.Run(name, func(t *testing.T) {
				srv, f := newTLSServer(t, time.Second, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Set("Content-Type", "application/octet-stream")
					_, _ = w.Write([]byte(b))
				}))
				data, _, err := f.FetchText(context.Background(), srv.URL, 1024)
				require.NoError(t, err)
				assert.Equal(t, b, string(data))
			})
		}
	})

	t.Run("refuses an html sign-in page even on 200", func(t *testing.T) {
		srv, f := newTLSServer(t, time.Second, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte("<html>sign in</html>"))
		}))
		_, _, err := f.FetchText(context.Background(), srv.URL, 1024)
		assert.ErrorIs(t, err, ErrNotText)
	})

	// The text/html label is refused by itself, before the body is read: a body
	// that would pass the sniff does not rescue it.
	t.Run("refuses the text/html label whatever the body", func(t *testing.T) {
		srv, f := newTLSServer(t, time.Second, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(body))
		}))
		_, _, err := f.FetchText(context.Background(), srv.URL, 1024)
		require.ErrorIs(t, err, ErrNotText)
		assert.NotErrorIs(t, err, ErrHTMLBody)
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

	// The two timeout cases are deterministic: the fetcher's own timeout is far
	// away, and the test expires the caller's context itself, once the server has
	// reached the point under test, with a deadline error the fetcher must map to
	// ErrTimeout. No real deadline races the TLS handshake or the race detector.
	t.Run("times out a slow server", func(t *testing.T) {
		reached := make(chan struct{})
		release := make(chan struct{})
		t.Cleanup(func() { close(release) })
		srv, f := newTLSServer(t, time.Minute, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			close(reached)
			select {
			case <-release:
			case <-r.Context().Done():
			}
		}))
		ctx := newExpirableContext()
		go func() {
			<-reached
			ctx.expire()
		}()
		_, _, err := f.FetchText(ctx, srv.URL, 1024)
		assert.ErrorIs(t, err, ErrTimeout)
	})

	t.Run("times out a slow body", func(t *testing.T) {
		flushed := make(chan struct{})
		release := make(chan struct{})
		t.Cleanup(func() { close(release) })
		srv, f := newTLSServer(t, time.Minute, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("partial"))
			w.(http.Flusher).Flush()
			close(flushed)
			select {
			case <-release:
			case <-r.Context().Done():
			}
		}))
		ctx := newExpirableContext()
		go func() {
			<-flushed
			ctx.expire()
		}()
		data, _, err := f.FetchText(ctx, srv.URL, 1024)
		assert.ErrorIs(t, err, ErrTimeout)
		assert.Nil(t, data)
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

// expirableContext is a context the test expires on demand with
// context.DeadlineExceeded, as a real deadline would, so a timeout path can be
// exercised without racing a clock. The fetcher derives its own context from
// it with context.WithTimeout, which propagates this expiry to the request.
type expirableContext struct {
	context.Context
	done chan struct{}
	once sync.Once
}

func newExpirableContext() *expirableContext {
	return &expirableContext{Context: context.Background(), done: make(chan struct{})}
}

// Done returns the channel expire closes.
func (c *expirableContext) Done() <-chan struct{} { return c.done }

// Err reports context.DeadlineExceeded once expired, nil before.
func (c *expirableContext) Err() error {
	select {
	case <-c.done:
		return context.DeadlineExceeded
	default:
		return nil
	}
}

// expire ends the context as a passed deadline would.
func (c *expirableContext) expire() { c.once.Do(func() { close(c.done) }) }
