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
	"math"
	"math/rand"
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
	// Default retry configuration
	DefaultMaxRetries        = 3
	DefaultInitialBackoff    = 1 * time.Second
	DefaultMaxBackoff        = 30 * time.Second
	DefaultBackoffMultiplier = 2.0
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
	// Optional: retry configuration
	MaxRetries         int
	InitialBackoff     time.Duration
	MaxBackoff         time.Duration
	BackoffMultiplier  float64
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
	if config.MaxRetries == 0 {
		config.MaxRetries = DefaultMaxRetries
	}
	if config.InitialBackoff == 0 {
		config.InitialBackoff = DefaultInitialBackoff
	}
	if config.MaxBackoff == 0 {
		config.MaxBackoff = DefaultMaxBackoff
	}
	if config.BackoffMultiplier == 0 {
		config.BackoffMultiplier = DefaultBackoffMultiplier
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

// shouldRetry determines if an error or HTTP status code should be retried
func shouldRetry(statusCode int, err error) bool {
	// Don't retry if context was cancelled
	if err != nil {
		if ctx, ok := err.(interface{ Err() error }); ok {
			if ctx.Err() == context.Canceled || ctx.Err() == context.DeadlineExceeded {
				return false
			}
		}
	}

	// Retry on network/connection errors
	if err != nil {
		return true
	}

	// Retry on server errors (5xx)
	if statusCode >= 500 && statusCode < 600 {
		return true
	}

	// Retry on rate limiting (429)
	if statusCode == http.StatusTooManyRequests {
		return true
	}

	// Don't retry on client errors (4xx)
	return false
}

// calculateBackoff calculates the backoff duration for a retry attempt with jitter
func (c *Client) calculateBackoff(attempt int) time.Duration {
	if attempt <= 0 {
		return c.config.InitialBackoff
	}

	// Calculate exponential backoff
	backoff := float64(c.config.InitialBackoff) * math.Pow(c.config.BackoffMultiplier, float64(attempt))
	
	// Cap at max backoff
	if time.Duration(backoff) > c.config.MaxBackoff {
		backoff = float64(c.config.MaxBackoff)
	}

	// Add jitter (±25% of backoff duration) to prevent thundering herd
	jitter := backoff * 0.25 * (rand.Float64()*2 - 1) // Random number between -0.25 and +0.25
	backoffWithJitter := time.Duration(backoff + jitter)

	// Ensure we don't go below initial backoff
	if backoffWithJitter < c.config.InitialBackoff {
		backoffWithJitter = c.config.InitialBackoff
	}

	return backoffWithJitter
}

// doRequest performs an authenticated HTTP request to the Zoom API with retry logic
func (c *Client) doRequest(ctx context.Context, method, path string, body any) (*http.Response, error) {
	var jsonBody []byte
	var err error
	
	// Marshal request body once to avoid re-marshalling on retries
	if body != nil {
		jsonBody, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
	}

	url := c.config.BaseURL + path
	var lastErr error
	var lastResp *http.Response

	for attempt := 0; attempt <= c.config.MaxRetries; attempt++ {
		// Create a new request for each attempt (body reader gets consumed)
		var bodyReader io.Reader
		if jsonBody != nil {
			bodyReader = bytes.NewReader(jsonBody)
		}

		req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		// Log initial request (not retries to avoid spam)
		if attempt == 0 {
			slog.DebugContext(ctx, "making Zoom API request",
				"method", method,
				"path", path,
				"body", body,
				"max_retries", c.config.MaxRetries,
			)
		} else {
			slog.DebugContext(ctx, "retrying Zoom API request",
				"method", method,
				"path", path,
				"attempt", attempt,
				"max_retries", c.config.MaxRetries,
			)
		}

		startTime := time.Now()

		// Use OAuth2 authenticated client which automatically handles token management
		authenticatedClient := c.getAuthenticatedClient(ctx)
		resp, err := authenticatedClient.Do(req)
		
		duration := time.Since(startTime)

		// If request succeeded, log and return
		if err == nil && resp.StatusCode < http.StatusInternalServerError && resp.StatusCode != http.StatusTooManyRequests {
			// Log successful requests with duration
			slog.InfoContext(ctx, "Zoom API request completed",
				"method", method,
				"path", path,
				"status", resp.StatusCode,
				"duration", duration.String(),
				"attempt", attempt+1,
			)

			// Log error responses with additional details (but don't retry 4xx)
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

		// Store the error/response for potential retry
		lastErr = err
		if resp != nil {
			if lastResp != nil {
				_ = lastResp.Body.Close()
			}
			lastResp = resp
		}

		// Check if we should retry
		statusCode := 0
		if resp != nil {
			statusCode = resp.StatusCode
		}

		if !shouldRetry(statusCode, err) {
			// Log non-retryable error
			if err != nil {
				slog.ErrorContext(ctx, "Zoom API request failed (not retryable)",
					"method", method,
					"path", path,
					"duration", duration.String(),
					"attempt", attempt+1,
					logging.ErrKey, err)
			} else {
				slog.ErrorContext(ctx, "Zoom API request failed (not retryable)",
					"method", method,
					"path", path,
					"status", statusCode,
					"duration", duration.String(),
					"attempt", attempt+1)
			}
			break
		}

		// Don't sleep after the last attempt
		if attempt < c.config.MaxRetries {
			backoff := c.calculateBackoff(attempt)
			slog.WarnContext(ctx, "Zoom API request failed, retrying",
				"method", method,
				"path", path,
				"status", statusCode,
				"duration", duration.String(),
				"attempt", attempt+1,
				"max_retries", c.config.MaxRetries,
				"backoff", backoff.String(),
				logging.ErrKey, err)

			// Wait with backoff, but check for context cancellation
			select {
			case <-ctx.Done():
				if lastResp != nil {
					_ = lastResp.Body.Close()
				}
				return nil, ctx.Err()
			case <-time.After(backoff):
				// Continue with retry
			}
		} else {
			// Log final failure
			if err != nil {
				slog.ErrorContext(ctx, "Zoom API request failed after all retries",
					"method", method,
					"path", path,
					"duration", duration.String(),
					"attempts", attempt+1,
					"max_retries", c.config.MaxRetries,
					logging.ErrKey, err)
			} else {
				slog.ErrorContext(ctx, "Zoom API request failed after all retries",
					"method", method,
					"path", path,
					"status", statusCode,
					"duration", duration.String(),
					"attempts", attempt+1,
					"max_retries", c.config.MaxRetries)
			}
		}
	}

	// Return the last error/response we got
	if lastErr != nil {
		if lastResp != nil {
			_ = lastResp.Body.Close()
		}
		return nil, fmt.Errorf("request failed after %d attempts: %w", c.config.MaxRetries+1, lastErr)
	}

	// If we got a response, prepare it for error handling (read body for error logging)
	if lastResp != nil && lastResp.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(lastResp.Body)
		_ = lastResp.Body.Close()
		lastResp.Body = io.NopCloser(bytes.NewReader(body))
		slog.ErrorContext(ctx, "Zoom API error response after all retries",
			"method", method,
			"path", path,
			"status", lastResp.StatusCode,
			"body", string(body),
			"attempts", c.config.MaxRetries+1,
			logging.ErrKey, fmt.Errorf("status: %d", lastResp.StatusCode))
	}

	return lastResp, nil
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
