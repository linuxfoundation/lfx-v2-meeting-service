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
	// ErrNotText means the provider answered with something other than text/*.
	ErrNotText = errors.New("recording file is not served as text")
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
				return nil
			},
		},
		timeout: timeout,
	}
}

// FetchText fetches downloadURL and returns at most maxBytes of its body.
// truncated reports whether the body was longer than maxBytes.
//
// The link must be https; redirects to anything else are refused. The response
// must be 2xx with a text/* content type other than text/html (a sign-in page
// served on a redirect is refused by that check). The body is streamed and counted,
// so a missing or understated Content-Length cannot defeat the cap.
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
	req.Header.Set("Accept", "text/plain, text/*;q=0.9")

	resp, err := f.client.Do(req)
	if err != nil {
		// err is a *url.Error carrying the URL: classify it and drop it.
		return nil, false, classifyTransportError(ctx, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, false, fmt.Errorf("%w: HTTP %d", ErrStatus, resp.StatusCode)
	}
	if !isTextContentType(resp.Header.Get("Content-Type")) {
		return nil, false, ErrNotText
	}

	// Read one byte past the cap to learn whether the body continues.
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, false, classifyTransportError(ctx, err)
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

// isTextContentType accepts text/* media types other than text/html: a chat file
// is served as plain text, while an HTML body is a sign-in or error page even when
// it comes with a 200 after a redirect. An absent or unparsable Content-Type is
// not text.
func isTextContentType(header string) bool {
	mediaType, _, err := mime.ParseMediaType(header)
	if err != nil {
		return false
	}
	return strings.HasPrefix(mediaType, "text/") && mediaType != "text/html"
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
