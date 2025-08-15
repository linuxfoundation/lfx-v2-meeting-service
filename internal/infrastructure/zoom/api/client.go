// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

// ClientAPI defines the interface for Zoom API operations
// This allows for easy mocking and testing of the Zoom client
type ClientAPI interface {
	CreateMeeting(ctx context.Context, userID string, request *CreateMeetingRequest) (*CreateMeetingResponse, error)
	UpdateMeeting(ctx context.Context, meetingID string, request *UpdateMeetingRequest) error
	DeleteMeeting(ctx context.Context, meetingID string) error
	GetUsers(ctx context.Context) ([]ZoomUser, error)
}

const (
	// BaseURL is the base URL for Zoom API
	BaseURL = "https://api.zoom.us/v2"
	// AuthURL is the OAuth token endpoint
	AuthURL = "https://zoom.us/oauth/token"
	// DefaultClientTimeout is the default HTTP client timeout for Zoom API requests
	DefaultClientTimeout = 30 * time.Second
)

// Client represents a Zoom API client
type Client struct {
	httpClient  *http.Client
	config      Config
	oauthConfig *clientcredentials.Config
}

// Config holds the configuration for the Zoom client
type Config struct {
	AccountID    string
	ClientID     string
	ClientSecret string
	// Optional: override base URL for testing
	BaseURL string
	// Optional: override auth URL for testing
	AuthURL string
	// Optional: override timeout for HTTP requests
	Timeout time.Duration
}

// Ensure that Client implements ClientAPI
var _ ClientAPI = (*Client)(nil)

// NewClient creates a new Zoom API client
func NewClient(config Config) *Client {
	// Set defaults if not provided
	if config.BaseURL == "" {
		config.BaseURL = BaseURL
	}
	if config.AuthURL == "" {
		config.AuthURL = AuthURL
	}
	if config.Timeout == 0 {
		config.Timeout = DefaultClientTimeout
	}

	// Set up OAuth2 client credentials config for Zoom
	// Zoom Server-to-Server OAuth requires specific grant_type and account_id
	oauthConfig := &clientcredentials.Config{
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
		TokenURL:     config.AuthURL,
		EndpointParams: url.Values{
			"grant_type": []string{"account_credentials"},
			"account_id": []string{config.AccountID},
		},
		AuthStyle: oauth2.AuthStyleInParams, // Try form parameters instead of header
	}

	return &Client{
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		config:      config,
		oauthConfig: oauthConfig,
	}
}

// getAuthenticatedClient returns an HTTP client that automatically handles OAuth2 authentication
func (c *Client) getAuthenticatedClient(ctx context.Context) *http.Client {
	ts := c.oauthConfig.TokenSource(ctx)
	return &http.Client{
		Timeout: c.config.Timeout,
		Transport: &oauth2.Transport{
			Base:   http.DefaultTransport,
			Source: ts,
		},
	}
}

// doRequest performs an authenticated HTTP request to the Zoom API
func (c *Client) doRequest(ctx context.Context, method, path string, body any) (*http.Response, error) {
	startTime := time.Now()

	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	url := c.config.BaseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	slog.DebugContext(ctx, "making Zoom API request",
		"method", method,
		"path", path,
		"body", body,
	)

	// Use OAuth2 authenticated client which automatically handles token management
	authenticatedClient := c.getAuthenticatedClient(ctx)
	resp, err := authenticatedClient.Do(req)

	// Calculate and log request duration
	duration := time.Since(startTime)

	if err != nil {
		slog.ErrorContext(ctx, "Zoom API request failed",
			"method", method,
			"path", path,
			"duration", duration.String(),
			logging.ErrKey, err)
		return nil, fmt.Errorf("request failed: %w", err)
	}

	// Log successful requests with duration
	slog.InfoContext(ctx, "Zoom API request completed",
		"method", method,
		"path", path,
		"status", resp.StatusCode,
		"duration", duration.String())

	// Log error responses with additional details
	if resp.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		resp.Body = io.NopCloser(bytes.NewReader(body))
		slog.ErrorContext(ctx, "Zoom API error response",
			"method", method,
			"path", path,
			"status", resp.StatusCode,
			"duration", duration.String(),
			"body", string(body),
			logging.ErrKey, fmt.Errorf("status: %d", resp.StatusCode))
	}

	return resp, nil
}

// parseErrorResponse attempts to parse a Zoom API error response
func parseErrorResponse(body []byte) error {
	var errResp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &errResp); err == nil && errResp.Message != "" {
		return fmt.Errorf("zoom API error (code %d): %s", errResp.Code, errResp.Message)
	}
	return fmt.Errorf("zoom API error: %s", string(body))
}
