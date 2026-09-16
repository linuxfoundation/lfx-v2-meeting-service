// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package itx

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/models/itx"
)

// withCapturedLogs installs a text-handler slog.Logger as the default for
// the duration of the test and returns a buffer containing everything
// logged through slog's package-level *Context functions, restoring the
// prior default logger on cleanup.
func withCapturedLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

func TestRedactQueryParams(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		rawURL string
		keys   []string
		want   string
	}{
		{
			name:   "redacts listed keys, leaves others",
			rawURL: "https://itx.example.com/v2/zoom/meetings/m1/join_link?email=alice%40example.com&name=Alice&user_id=u1&use_email=true",
			keys:   []string{"email", "name", "user_id"},
			want:   "https://itx.example.com/v2/zoom/meetings/m1/join_link?email=REDACTED&name=REDACTED&use_email=true&user_id=REDACTED",
		},
		{
			name:   "no matching keys present leaves URL unchanged",
			rawURL: "https://itx.example.com/v2/zoom/meetings/m1/join_link?use_email=true",
			keys:   []string{"email", "name", "user_id"},
			want:   "https://itx.example.com/v2/zoom/meetings/m1/join_link?use_email=true",
		},
		{
			name:   "no query string at all",
			rawURL: "https://itx.example.com/v2/zoom/meetings/m1/join_link",
			keys:   []string{"email"},
			want:   "https://itx.example.com/v2/zoom/meetings/m1/join_link",
		},
		{
			name:   "malformed URL fails closed to a safe placeholder",
			rawURL: "://not a url",
			keys:   []string{"email"},
			want:   "REDACTED (unparseable URL)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := redactQueryParams(tt.rawURL, tt.keys)
			if got != tt.want {
				t.Errorf("redactQueryParams(%q, %v) = %q, want %q", tt.rawURL, tt.keys, got, tt.want)
			}
		})
	}
}

func TestTruncateForLog(t *testing.T) {
	t.Parallel()

	if got := truncateForLog("short", 10); got != "short" {
		t.Errorf("short string should be unchanged, got %q", got)
	}

	long := strings.Repeat("a", 20)
	got := truncateForLog(long, 5)
	if !strings.HasPrefix(got, "aaaaa") || !strings.HasSuffix(got, "(truncated)") {
		t.Errorf("truncateForLog(long, 5) = %q, want 5-char prefix + truncated marker", got)
	}
}

// TestGetMeetingJoinLink_ErrorResponse_LogsWithoutLeakingPII exercises the
// exact scenario in lfx-self-serve#1897: a non-2xx from ITX's join_link
// endpoint must produce a log line with the ITX status code, and the
// email/name/user_id supplied on the request must never appear in that log.
func TestGetMeetingJoinLink_ErrorResponse_LogsWithoutLeakingPII(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("email"); got != "alice@example.com" {
			t.Errorf("server did not receive the real email; got %q", got)
		}
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"forbidden"}`))
	}))
	t.Cleanup(server.Close)

	buf := withCapturedLogs(t)

	client := NewClientWithHTTPClient(Config{BaseURL: server.URL}, server.Client())
	_, err := client.GetMeetingJoinLink(context.Background(), &itx.GetJoinLinkRequest{
		MeetingID: "meeting-1",
		Email:     "alice@example.com",
		Name:      "Alice Example",
		UserID:    "user-1",
	})
	if err == nil {
		t.Fatal("expected an error from the 403 response")
	}

	out := buf.String()
	if !strings.Contains(out, "statusCode=403") {
		t.Errorf("expected the error-response log to include statusCode=403, got:\n%s", out)
	}
	if !strings.Contains(out, "ITX GetMeetingJoinLink error response") {
		t.Errorf("expected the error-response log to name the operation, got:\n%s", out)
	}
	assertNoLeakedJoinLinkPII(t, out)
}

// assertNoLeakedJoinLinkPII fails t if the join-link email/name/user_id from
// TestGetMeetingJoinLink_*_LogsWithoutLeakingPII's request appear in out —
// checked both decoded ("alice@example.com") and in the url.Values.Encode
// form ("alice%40example.com", "Alice+Example") those values actually take
// once they're part of a query string, so an encoded leak can't slip past a
// check that only looks for the decoded form.
func assertNoLeakedJoinLinkPII(t *testing.T, out string) {
	t.Helper()
	for _, pii := range []string{
		"alice@example.com", url.QueryEscape("alice@example.com"),
		"Alice Example", url.QueryEscape("Alice Example"),
		"user-1",
	} {
		if strings.Contains(out, pii) {
			t.Errorf("log output leaked PII %q:\n%s", pii, out)
		}
	}
}

// erroringTransport always fails the request, returning an *url.Error whose
// Error() string embeds the full request URL — the shape http.Client.Do
// actually returns on a transport failure (DNS, timeout, connection refused).
type erroringTransport struct{}

func (erroringTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return nil, fmt.Errorf("dial tcp: connection refused")
}

// TestGetMeetingJoinLink_TransportFailure_LogsWithoutLeakingPII verifies that
// a transport-level failure (which surfaces the raw request URL via
// *url.Error) doesn't leak the join-link email/name/user_id into the Error
// log — the same redaction applied to logURL must also apply to the error text.
func TestGetMeetingJoinLink_TransportFailure_LogsWithoutLeakingPII(t *testing.T) {
	buf := withCapturedLogs(t)

	client := NewClientWithHTTPClient(Config{BaseURL: "https://itx.example.com"}, &http.Client{Transport: erroringTransport{}})
	_, err := client.GetMeetingJoinLink(context.Background(), &itx.GetJoinLinkRequest{
		MeetingID: "meeting-1",
		Email:     "alice@example.com",
		Name:      "Alice Example",
		UserID:    "user-1",
	})
	if err == nil {
		t.Fatal("expected an error from the transport failure")
	}

	out := buf.String()
	if !strings.Contains(out, "ITX GetMeetingJoinLink request failed") {
		t.Errorf("expected the transport-failure log line, got:\n%s", out)
	}
	assertNoLeakedJoinLinkPII(t, out)
}

// TestGetMeetingJoinLink_Success_DebugLogsRedactUpstreamURL verifies the
// success path's Debug request log also uses the redacted URL, so enabling
// LOG_LEVEL=debug for troubleshooting doesn't itself create a PII leak.
func TestGetMeetingJoinLink_Success_DebugLogsRedactUpstreamURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"link":"https://zoom.example.com/j/123"}`))
	}))
	t.Cleanup(server.Close)

	buf := withCapturedLogs(t)

	client := NewClientWithHTTPClient(Config{BaseURL: server.URL}, server.Client())
	resp, err := client.GetMeetingJoinLink(context.Background(), &itx.GetJoinLinkRequest{
		MeetingID: "meeting-1",
		Email:     "bob@example.com",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Link != "https://zoom.example.com/j/123" {
		t.Fatalf("unexpected response: %+v", resp)
	}

	out := buf.String()
	for _, email := range []string{"bob@example.com", url.QueryEscape("bob@example.com")} {
		if strings.Contains(out, email) {
			t.Errorf("Debug request log leaked email %q:\n%s", email, out)
		}
	}
	if !strings.Contains(out, "email=REDACTED") {
		t.Errorf("expected the logged URL to show email=REDACTED, got:\n%s", out)
	}
	if strings.Contains(out, "zoom.example.com/j/123") {
		t.Errorf("Debug response log leaked the join URL (skipResponseLog should suppress it):\n%s", out)
	}
}
