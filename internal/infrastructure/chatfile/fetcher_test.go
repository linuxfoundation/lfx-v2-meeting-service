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
			"head tag":                     {"application/octet-stream", "<head><title>Sign in</title></head>"},
			"body tag":                     {"application/octet-stream", "<body>Sign in</body>"},
			"script tag":                   {"text/plain", "<script>location='/login'</script>Sign in"},
			"meta tag":                     {"application/octet-stream", "<meta http-equiv=\"refresh\" content=\"0\">Sign in"},
			"div tag":                      {"application/octet-stream", "<div class=\"login\">Sign in</div>"},
			"uppercase head tag":           {"application/octet-stream", "<HEAD><TITLE>Sign in</TITLE></HEAD>"},
			"cdata section":                {"application/octet-stream", "<![CDATA[Sign in]]>"},
			"truncated doctype":            {"application/octet-stream", "<!DOC"},
			"other tag then chat":          {"application/octet-stream", "<meeting notes>\n" + body},
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

	// The sniff refuses only a body that opens like markup: a chat file that
	// starts with a byte-order mark, a comment, or "<" followed by something
	// other than a letter or "!" is left to the parser.
	t.Run("the html sniff leaves non-html bodies alone", func(t *testing.T) {
		for name, b := range map[string]string{
			"bom then chat header":  "\xEF\xBB\xBF" + body,
			"comment then chat":     "<!-- exported --> " + body,
			"< then space":          "< 3 people joined\n" + body,
			"< then slash":          "</>\n" + body,
			"< then digit":          "<3\n" + body,
			"lone < at end of body": "<",
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

	// The two timeout cases are deterministic and socket-free: a fake transport
	// stands in for the provider (no connection, so no race between the server
	// finishing the response and the client observing the cancellation), the
	// fetcher's own timeout is far away, and the test expires the caller's
	// context itself, once the transport has reached the point under test, with
	// a deadline error the fetcher must map to ErrTimeout.
	t.Run("times out a slow server", func(t *testing.T) {
		transport := &slowServerTransport{started: make(chan struct{})}
		f := NewFetcher(time.Minute, transport)
		ctx := newExpirableContext()
		go func() {
			<-transport.started
			ctx.expire()
		}()
		data, _, err := f.FetchText(ctx, "https://provider.invalid/chat.txt", 1024)
		assert.ErrorIs(t, err, ErrTimeout)
		assert.Nil(t, data)
	})

	t.Run("times out a slow body", func(t *testing.T) {
		transport := &slowBodyTransport{partial: "partial", delivered: make(chan struct{})}
		f := NewFetcher(time.Minute, transport)
		ctx := newExpirableContext()
		go func() {
			<-transport.delivered
			ctx.expire()
		}()
		data, _, err := f.FetchText(ctx, "https://provider.invalid/chat.txt", 1024)
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

// slowServerTransport is an http.RoundTripper standing in for a provider that
// never answers: the round trip blocks until the request's context is done
// and returns its error. started is closed once the round trip is under way.
type slowServerTransport struct {
	started chan struct{}
}

func (t *slowServerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	close(t.started)
	<-req.Context().Done()
	return nil, req.Context().Err()
}

// slowBodyTransport is an http.RoundTripper standing in for a provider that
// answers 200 text/plain, delivers partial bytes of the body, then never sends
// the rest: further reads block until the request's context is done and
// return its error. delivered is closed once the partial bytes were read.
type slowBodyTransport struct {
	partial   string
	delivered chan struct{}
}

func (t *slowBodyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{
		Status:     "200 OK",
		StatusCode: http.StatusOK,
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     http.Header{"Content-Type": {"text/plain"}},
		Body:       &slowBody{ctx: req.Context(), rest: []byte(t.partial), delivered: t.delivered},
		Request:    req,
	}, nil
}

// slowBody is the body slowBodyTransport returns.
type slowBody struct {
	ctx       context.Context
	rest      []byte
	delivered chan struct{}
	once      sync.Once
}

func (b *slowBody) Read(p []byte) (int, error) {
	if len(b.rest) > 0 {
		n := copy(p, b.rest)
		b.rest = b.rest[n:]
		if len(b.rest) == 0 {
			b.once.Do(func() { close(b.delivered) })
		}
		return n, nil
	}
	<-b.ctx.Done()
	return 0, b.ctx.Err()
}

func (b *slowBody) Close() error { return nil }
