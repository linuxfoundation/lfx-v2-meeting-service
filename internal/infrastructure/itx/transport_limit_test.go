// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package itx

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
)

// zeroStream simulates an upstream streaming a body without holding it in memory.
type zeroStream struct{}

func (zeroStream) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}

type countingBody struct {
	io.ReadCloser
	read   int64
	closed bool
}

func (b *countingBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	b.read += int64(n)
	return n, err
}

func (b *countingBody) Close() error {
	b.closed = true
	return b.ReadCloser.Close()
}

type countingTransport struct {
	base http.RoundTripper
	body *countingBody
}

func (tr *countingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := tr.base.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	tr.body = &countingBody{ReadCloser: resp.Body}
	resp.Body = tr.body
	return resp, nil
}

type responseOperation uint8

const (
	readRaw responseOperation = iota
	decodeJSON
	ignoreBody
)

func TestExecuteResponseSizeLimit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                 string
		accept               string
		status               int
		bodySize             int64
		limit                int64
		operation            responseOperation
		declareContentLength bool
		wantErr              bool
	}{
		{name: "JSON at limit", accept: acceptJSON, status: http.StatusOK, bodySize: maxJSONResponseBytes, limit: maxJSONResponseBytes},
		{name: "JSON at declared limit", accept: acceptJSON, status: http.StatusOK, bodySize: maxJSONResponseBytes, limit: maxJSONResponseBytes, declareContentLength: true},
		{name: "JSON over limit", accept: acceptJSON, status: http.StatusOK, bodySize: 2 * maxJSONResponseBytes, limit: maxJSONResponseBytes, wantErr: true},
		{name: "JSON declared over limit", accept: acceptJSON, status: http.StatusOK, bodySize: 2 * maxJSONResponseBytes, limit: maxJSONResponseBytes, declareContentLength: true, wantErr: true},
		{name: "JSON decoding over limit", accept: acceptJSON, status: http.StatusOK, bodySize: 2 * maxJSONResponseBytes, limit: maxJSONResponseBytes, operation: decodeJSON, wantErr: true},
		{name: "no-content operation over limit", accept: acceptJSON, status: http.StatusOK, bodySize: 2 * maxJSONResponseBytes, limit: maxJSONResponseBytes, operation: ignoreBody, wantErr: true},
		{name: "error over limit", accept: acceptJSON, status: http.StatusBadRequest, bodySize: 2 * maxJSONResponseBytes, limit: maxJSONResponseBytes, wantErr: true},
		{name: "calendar at limit", accept: acceptCalendar, status: http.StatusOK, bodySize: maxCalendarResponseBytes, limit: maxCalendarResponseBytes},
		{name: "calendar over limit", accept: acceptCalendar, status: http.StatusOK, bodySize: 2 * maxCalendarResponseBytes, limit: maxCalendarResponseBytes, wantErr: true},
		{name: "calendar declared over limit", accept: acceptCalendar, status: http.StatusOK, bodySize: 2 * maxCalendarResponseBytes, limit: maxCalendarResponseBytes, declareContentLength: true, wantErr: true},
		{name: "calendar error uses JSON limit", accept: acceptCalendar, status: http.StatusBadRequest, bodySize: 2 * maxJSONResponseBytes, limit: maxJSONResponseBytes, wantErr: true},
		{name: "calendar error declared over JSON limit", accept: acceptCalendar, status: http.StatusBadRequest, bodySize: 2 * maxJSONResponseBytes, limit: maxJSONResponseBytes, declareContentLength: true, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if tt.declareContentLength {
					w.Header().Set("Content-Length", strconv.FormatInt(tt.bodySize, 10))
				}
				w.WriteHeader(tt.status)
				_, _ = io.CopyN(w, zeroStream{}, tt.bodySize)
			}))
			t.Cleanup(server.Close)

			httpClient := server.Client()
			transport := &countingTransport{base: httpClient.Transport}
			httpClient.Transport = transport
			client := NewClientWithHTTPClient(Config{BaseURL: server.URL}, httpClient)
			req := apiRequest{method: http.MethodGet, path: "/response", accept: tt.accept, skipResponseLog: true}
			var body []byte
			var err error
			switch tt.operation {
			case ignoreBody:
				err = client.doNoContent(context.Background(), req)
			case decodeJSON:
				err = client.doJSON(context.Background(), req, &struct{}{})
			case readRaw:
				body, err = client.doRaw(context.Background(), req)
			}
			if tt.wantErr {
				if err == nil || domain.GetErrorType(err) != domain.ErrorTypeUnavailable {
					t.Fatalf("GetErrorType(%v) = %v, want unavailable", err, domain.GetErrorType(err))
				}
				if body != nil {
					t.Fatal("oversized body returned to caller")
				}
			} else if err != nil || int64(len(body)) != tt.bodySize {
				t.Fatalf("body length = %d, err = %v; want %d bytes", len(body), err, tt.bodySize)
			}
			if transport.body == nil || !transport.body.closed {
				t.Fatal("upstream response body was not closed")
			}
			if transport.body.read > tt.limit+1 {
				t.Fatalf("read %d bytes from upstream, limit is %d", transport.body.read, tt.limit+1)
			}
			if tt.declareContentLength && tt.wantErr && transport.body.read != 0 {
				t.Fatalf("read %d bytes despite oversized Content-Length", transport.body.read)
			}
		})
	}
}
