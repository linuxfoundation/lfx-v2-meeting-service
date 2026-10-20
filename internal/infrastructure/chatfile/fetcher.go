// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// Package chatfile fetches text recording files (such as a saved chat) from the
// provider links held in a recording record.
//
// Those links are bearer links: anyone holding one can download the file. The
// fetcher therefore treats the URL as a secret: it is never logged, never put on
// a span (the client is deliberately not instrumented with otelhttp, whose spans
// carry the full URL) and never part of an error. Every error this package
// returns is one of the sentinels below, optionally annotated with an HTTP status,
// so callers can log them freely.
package chatfile

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
)

// Sentinel errors. None carries the URL, the host or the file name.
var (
	// ErrInvalidLink means the recording record holds no usable https link.
	ErrInvalidLink = errors.New("recording file link is not a usable https link")
	// ErrTimeout means the fetch did not complete within the timeout.
	ErrTimeout = errors.New("recording file fetch timed out")
	// ErrTransport means the connection or the transfer failed.
	ErrTransport = errors.New("recording file fetch failed")
	// ErrStatus means the provider answered with a non-2xx status.
	ErrStatus = errors.New("recording file fetch returned a non-success status")
	// ErrNotText means the provider answered with a content type the fetcher does
	// not accept (see isAcceptableContentType), or with a body that is an HTML
	// document (ErrHTMLBody wraps it).
	ErrNotText = errors.New("recording file is not served as text")
	// ErrHTMLBody means the body is an HTML document, whatever its content type: a
	// sign-in or error page, not the file. It matches ErrNotText with errors.Is.
	ErrHTMLBody = fmt.Errorf("%w: body is an html document", ErrNotText)
)

// maxRedirects bounds the redirect chain the provider may send the fetch through
// (a download link commonly redirects once to a storage URL).
const maxRedirects = 5

// Fetcher implements domain.RecordingFileFetcher over net/http.
type Fetcher struct {
	client  *http.Client
	timeout time.Duration
}

// Verify interface compliance at compile time.
var _ domain.RecordingFileFetcher = (*Fetcher)(nil)

// NewFetcher creates a Fetcher whose every fetch, body included, is bounded by timeout.
// transport may be nil, in which case a plain http.Transport is used; it must not be
// an instrumented transport that records URLs.
func NewFetcher(timeout time.Duration, transport http.RoundTripper) *Fetcher {
	if transport == nil {
		transport = &http.Transport{
			Proxy:               http.ProxyFromEnvironment,
			MaxIdleConns:        10,
			IdleConnTimeout:     30 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
		}
	}
	return &Fetcher{
		client: &http.Client{
			Transport: transport,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= maxRedirects {
					return ErrTransport
				}
				if req.URL.Scheme != "https" {
					return ErrInvalidLink
				}
				// net/http sets Referer to the previous URL (path and query included)
				// before calling this hook; the previous URL is the bearer link, so it
				// must not reach the redirect destination or its logs.
				req.Header.Del("Referer")
				return nil
			},
		},
		timeout: timeout,
	}
}

// FetchText fetches downloadURL and returns at most maxBytes of its body.
// truncated reports whether the body was longer than maxBytes.
//
// The link must be https; redirects to anything else are refused, and the Referer
// header net/http would derive from the link is removed on every redirect. The response
// must be 2xx with a content type isAcceptableContentType admits: text/* other than
// text/html, or application/octet-stream, which is what the recording provider
// actually labels its recording files with (observed on production; a chat file
// is text whatever its label). The label alone is not trusted: after the body is
// read, isHTMLDocument refuses an HTML document under any label, since a sign-in
// or error page served on a redirect can arrive as text/plain or octet-stream as
// well as text/html. The body is streamed and counted, so a missing or understated
// Content-Length cannot defeat the cap; the sniff reads only the bytes within it.
func (f *Fetcher) FetchText(ctx context.Context, downloadURL string, maxBytes int64) ([]byte, bool, error) {
	if !isHTTPSLink(downloadURL) {
		return nil, false, ErrInvalidLink
	}

	// WithTimeout takes the sooner of the caller's deadline and ours, so a shorter
	// request deadline still applies.
	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return nil, false, ErrInvalidLink
	}
	req.Header.Set("Accept", "text/plain, text/*;q=0.9, application/octet-stream;q=0.8")

	resp, err := f.client.Do(req)
	if err != nil {
		// err is a *url.Error carrying the URL: classify it and drop it.
		return nil, false, classifyTransportError(ctx, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, false, fmt.Errorf("%w: HTTP %d", ErrStatus, resp.StatusCode)
	}
	if !isAcceptableContentType(resp.Header.Get("Content-Type")) {
		return nil, false, ErrNotText
	}

	// Read one byte past the cap to learn whether the body continues.
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, false, classifyTransportError(ctx, err)
	}
	// Fail closed on a body that opens like HTML whatever the label said. The
	// error carries nothing of the body.
	if isHTMLDocument(data) {
		return nil, false, ErrHTMLBody
	}
	if int64(len(data)) > maxBytes {
		return data[:maxBytes], true, nil
	}
	return data, false, nil
}

// isHTTPSLink reports whether s parses as an absolute https URL with a host.
func isHTTPSLink(s string) bool {
	u, err := url.Parse(s)
	return err == nil && u.Scheme == "https" && u.Host != ""
}

// isAcceptableContentType accepts the two labels a recording provider puts on a
// chat file: a text/* media type other than text/html, and
// application/octet-stream, the generic label the provider serves its recording
// files with on production (parameters such as charset are ignored). text/html is
// refused before the body is read: it is a sign-in or error page even when it
// comes with a 200 after a redirect; isHTMLDocument catches the same page under
// another label. An absent or unparsable Content-Type is not accepted.
func isAcceptableContentType(header string) bool {
	mediaType, _, err := mime.ParseMediaType(header)
	if err != nil {
		return false
	}
	if mediaType == "application/octet-stream" {
		return true
	}
	return strings.HasPrefix(mediaType, "text/") && mediaType != "text/html"
}

// isHTMLDocument reports whether body opens like an HTML document: after an
// optional UTF-8 byte-order mark, leading whitespace, and any leading HTML
// comments ("<!-- -->") or XML prologs and processing instructions ("<? ?>",
// the start of an XHTML page), which pages commonly open with, it starts with
// "<" followed by an ASCII letter or "!": a document declaration ("<!DOCTYPE",
// even truncated), a CDATA section, or any tag ("<html", "<head", "<body",
// "<script", "<div", in any case). A leading comment or prolog left
// unterminated within the bytes read is taken as HTML too. A real chat line
// opens with a timestamp, so nothing a chat file starts with is lost; the
// route fails closed on everything else that opens like markup. An empty body
// is not HTML (an empty chat file is a session with no messages), and "<"
// followed by anything else (a digit, a space, "/", the end of the body) is
// left to the parser. The body is already bounded by the caller; the scan is
// linear in it.
func isHTMLDocument(body []byte) bool {
	body = bytes.TrimPrefix(body, []byte("\xEF\xBB\xBF"))
	for {
		body = bytes.TrimLeft(body, " \t\r\n\f")
		var closer []byte
		switch {
		case bytes.HasPrefix(body, []byte("<!--")):
			closer = []byte("-->")
		case bytes.HasPrefix(body, []byte("<?")):
			closer = []byte("?>")
		default:
			if len(body) < 2 || body[0] != '<' {
				return false
			}
			c := body[1]
			return c == '!' || ('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z')
		}
		end := bytes.Index(body[2:], closer)
		if end < 0 {
			return true
		}
		body = body[2+end+len(closer):]
	}
}

// classifyTransportError maps a transport-level error to a sentinel. The
// original error is dropped on purpose: *url.Error and *net.OpError format the
// URL or the host into their messages.
func classifyTransportError(ctx context.Context, err error) error {
	if errors.Is(err, ErrInvalidLink) {
		return ErrInvalidLink
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return ErrTimeout
	}
	return ErrTransport
}
