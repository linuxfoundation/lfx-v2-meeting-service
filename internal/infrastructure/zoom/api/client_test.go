// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	tests := []struct {
		name            string
		config          Config
		expectedBaseURL string
		expectedAuthURL string
		expectedTimeout time.Duration
	}{
		{
			name: "with all config provided",
			config: Config{
				AccountID:    "test-account",
				ClientID:     "test-client-id",
				ClientSecret: "test-secret",
				BaseURL:      "https://custom.api.zoom.us/v2",
				AuthURL:      "https://custom.zoom.us/oauth/token",
				Timeout:      45 * time.Second,
			},
			expectedBaseURL: "https://custom.api.zoom.us/v2",
			expectedAuthURL: "https://custom.zoom.us/oauth/token",
			expectedTimeout: 45 * time.Second,
		},
		{
			name: "with minimal config - uses defaults",
			config: Config{
				AccountID:    "test-account",
				ClientID:     "test-client-id",
				ClientSecret: "test-secret",
			},
			expectedBaseURL: BaseURL,
			expectedAuthURL: AuthURL,
			expectedTimeout: DefaultClientTimeout,
		},
		{
			name: "with partial config - fills defaults",
			config: Config{
				AccountID:    "test-account",
				ClientID:     "test-client-id",
				ClientSecret: "test-secret",
				BaseURL:      "https://custom.api.zoom.us/v2",
			},
			expectedBaseURL: "https://custom.api.zoom.us/v2",
			expectedAuthURL: AuthURL,
			expectedTimeout: DefaultClientTimeout,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(tt.config)

			if client == nil {
				t.Fatal("NewClient returned nil")
			}

			if client.config.BaseURL != tt.expectedBaseURL {
				t.Errorf("expected BaseURL %s, got %s", tt.expectedBaseURL, client.config.BaseURL)
			}

			if client.config.AuthURL != tt.expectedAuthURL {
				t.Errorf("expected AuthURL %s, got %s", tt.expectedAuthURL, client.config.AuthURL)
			}

			if client.config.Timeout != tt.expectedTimeout {
				t.Errorf("expected Timeout %v, got %v", tt.expectedTimeout, client.config.Timeout)
			}

			if client.httpClient == nil {
				t.Error("httpClient should not be nil")
			}

			if client.httpClient.Timeout != tt.expectedTimeout {
				t.Errorf("expected HTTP client timeout %v, got %v", tt.expectedTimeout, client.httpClient.Timeout)
			}

			if client.oauthConfig == nil {
				t.Error("oauthConfig should not be nil")
			}

			// Verify OAuth config
			if client.oauthConfig.ClientID != tt.config.ClientID {
				t.Errorf("expected ClientID %s, got %s", tt.config.ClientID, client.oauthConfig.ClientID)
			}

			if client.oauthConfig.ClientSecret != tt.config.ClientSecret {
				t.Errorf("expected ClientSecret %s, got %s", tt.config.ClientSecret, client.oauthConfig.ClientSecret)
			}

			if client.oauthConfig.TokenURL != tt.expectedAuthURL {
				t.Errorf("expected TokenURL %s, got %s", tt.expectedAuthURL, client.oauthConfig.TokenURL)
			}

			// Verify endpoint params
			if client.oauthConfig.EndpointParams == nil {
				t.Error("EndpointParams should not be nil")
			}

			grantType := client.oauthConfig.EndpointParams.Get("grant_type")
			if grantType != "account_credentials" {
				t.Errorf("expected grant_type 'account_credentials', got %s", grantType)
			}

			accountID := client.oauthConfig.EndpointParams.Get("account_id")
			if accountID != tt.config.AccountID {
				t.Errorf("expected account_id %s, got %s", tt.config.AccountID, accountID)
			}
		})
	}
}

func TestParseErrorResponse(t *testing.T) {
	tests := []struct {
		name           string
		body           []byte
		expectedError  string
		expectedSubstr string
	}{
		{
			name:          "valid JSON error response",
			body:          []byte(`{"code": 404, "message": "Meeting not found"}`),
			expectedError: "zoom API error (code 404): Meeting not found",
		},
		{
			name:           "invalid JSON - fallback to raw body",
			body:           []byte(`invalid json response`),
			expectedSubstr: "zoom API error: invalid json response",
		},
		{
			name:           "empty message in JSON",
			body:           []byte(`{"code": 500, "message": ""}`),
			expectedSubstr: "zoom API error: {\"code\": 500, \"message\": \"\"}",
		},
		{
			name:           "empty body",
			body:           []byte(`{}`),
			expectedSubstr: "zoom API error: {}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := parseErrorResponse(tt.body)

			if err == nil {
				t.Fatal("expected error but got nil")
			}

			errMsg := err.Error()
			if tt.expectedError != "" {
				if errMsg != tt.expectedError {
					t.Errorf("expected error %q, got %q", tt.expectedError, errMsg)
				}
			} else if tt.expectedSubstr != "" {
				if !strings.Contains(errMsg, tt.expectedSubstr) {
					t.Errorf("expected error to contain %q, got %q", tt.expectedSubstr, errMsg)
				}
			}
		})
	}
}

func TestShouldRetry(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		err        error
		expected   bool
	}{
		{
			name:       "500 server error should retry",
			statusCode: 500,
			err:        nil,
			expected:   true,
		},
		{
			name:       "502 bad gateway should retry",
			statusCode: 502,
			err:        nil,
			expected:   true,
		},
		{
			name:       "503 service unavailable should retry",
			statusCode: 503,
			err:        nil,
			expected:   true,
		},
		{
			name:       "429 rate limit should retry",
			statusCode: 429,
			err:        nil,
			expected:   true,
		},
		{
			name:       "400 bad request should not retry",
			statusCode: 400,
			err:        nil,
			expected:   false,
		},
		{
			name:       "401 unauthorized should not retry",
			statusCode: 401,
			err:        nil,
			expected:   false,
		},
		{
			name:       "404 not found should not retry",
			statusCode: 404,
			err:        nil,
			expected:   false,
		},
		{
			name:       "200 success should not retry",
			statusCode: 200,
			err:        nil,
			expected:   false,
		},
		{
			name:       "network error should retry",
			statusCode: 0,
			err:        errors.New("connection refused"),
			expected:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shouldRetry(tt.statusCode, tt.err)
			if result != tt.expected {
				t.Errorf("shouldRetry(%d, %v) = %v, expected %v",
					tt.statusCode, tt.err, result, tt.expected)
			}
		})
	}
}

func TestCalculateBackoff(t *testing.T) {
	client := NewClient(Config{
		AccountID:         "test",
		ClientID:          "test",
		ClientSecret:      "test",
		InitialBackoff:    100 * time.Millisecond,
		MaxBackoff:        5 * time.Second,
		BackoffMultiplier: 2.0,
	})

	tests := []struct {
		name            string
		attempt         int
		expectedMinimum time.Duration
		expectedMaximum time.Duration
	}{
		{
			name:            "attempt 0 should return initial backoff",
			attempt:         0,
			expectedMinimum: 75 * time.Millisecond,  // 25% jitter tolerance
			expectedMaximum: 125 * time.Millisecond, // 25% jitter tolerance
		},
		{
			name:            "attempt 1 should double",
			attempt:         1,
			expectedMinimum: 100 * time.Millisecond, // At least initial backoff
			expectedMaximum: 250 * time.Millisecond, // 200ms + 25% jitter
		},
		{
			name:            "attempt 2 should be 4x initial",
			attempt:         2,
			expectedMinimum: 100 * time.Millisecond, // At least initial backoff
			expectedMaximum: 500 * time.Millisecond, // 400ms + 25% jitter
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backoff := client.calculateBackoff(tt.attempt)

			if backoff < tt.expectedMinimum {
				t.Errorf("calculateBackoff(%d) = %v, expected >= %v",
					tt.attempt, backoff, tt.expectedMinimum)
			}

			if backoff > tt.expectedMaximum {
				t.Errorf("calculateBackoff(%d) = %v, expected <= %v",
					tt.attempt, backoff, tt.expectedMaximum)
			}
		})
	}

	// Test max backoff is respected
	t.Run("max backoff is respected", func(t *testing.T) {
		backoff := client.calculateBackoff(10)          // Very high attempt
		if backoff > client.config.MaxBackoff*125/100 { // Allow 25% jitter
			t.Errorf("calculateBackoff(10) = %v, expected <= %v (with jitter)",
				backoff, client.config.MaxBackoff*125/100)
		}
	})
}

func TestDoRequest_RetryBehavior(t *testing.T) {
	t.Run("retries 5xx errors", func(t *testing.T) {
		attemptCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Mock OAuth2 token endpoint
			if r.URL.Path == "/token" {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"access_token": "test_token", "token_type": "Bearer"}`))
				return
			}

			// Mock API endpoint with retry behavior
			attemptCount++
			if attemptCount <= 2 {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"code": 500, "message": "Internal Server Error"}`))
			} else {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"status": "success"}`))
			}
		}))
		defer server.Close()

		client := NewClient(Config{
			AccountID:         "test",
			ClientID:          "test",
			ClientSecret:      "test",
			BaseURL:           server.URL,
			AuthURL:           server.URL + "/token",
			MaxRetries:        3,
			InitialBackoff:    10 * time.Millisecond,
			MaxBackoff:        100 * time.Millisecond,
			BackoffMultiplier: 2.0,
		})

		resp, err := client.doRequest(context.Background(), "GET", "/test", nil)

		if err != nil {
			t.Fatalf("expected success after retries, got error: %v", err)
		}

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		if attemptCount != 3 {
			t.Errorf("expected 3 attempts, got %d", attemptCount)
		}

		_ = resp.Body.Close()
	})

	t.Run("retries 429 rate limiting", func(t *testing.T) {
		attemptCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Mock OAuth2 token endpoint
			if r.URL.Path == "/token" {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"access_token": "test_token", "token_type": "Bearer"}`))
				return
			}

			// Mock API endpoint with rate limiting
			attemptCount++
			if attemptCount == 1 {
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"code": 429, "message": "Too Many Requests"}`))
			} else {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"status": "success"}`))
			}
		}))
		defer server.Close()

		client := NewClient(Config{
			AccountID:         "test",
			ClientID:          "test",
			ClientSecret:      "test",
			BaseURL:           server.URL,
			AuthURL:           server.URL + "/token",
			MaxRetries:        2,
			InitialBackoff:    10 * time.Millisecond,
			MaxBackoff:        100 * time.Millisecond,
			BackoffMultiplier: 2.0,
		})

		resp, err := client.doRequest(context.Background(), "GET", "/test", nil)

		if err != nil {
			t.Fatalf("expected success after retry, got error: %v", err)
		}

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		if attemptCount != 2 {
			t.Errorf("expected 2 attempts, got %d", attemptCount)
		}

		_ = resp.Body.Close()
	})

	t.Run("does not retry 4xx errors", func(t *testing.T) {
		attemptCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Mock OAuth2 token endpoint
			if r.URL.Path == "/token" {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"access_token": "test_token", "token_type": "Bearer"}`))
				return
			}

			// Mock API endpoint with 4xx error
			attemptCount++
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"code": 401, "message": "Unauthorized"}`))
		}))
		defer server.Close()

		client := NewClient(Config{
			AccountID:         "test",
			ClientID:          "test",
			ClientSecret:      "test",
			BaseURL:           server.URL,
			AuthURL:           server.URL + "/token",
			MaxRetries:        3,
			InitialBackoff:    10 * time.Millisecond,
			MaxBackoff:        100 * time.Millisecond,
			BackoffMultiplier: 2.0,
		})

		resp, err := client.doRequest(context.Background(), "GET", "/test", nil)

		if err != nil {
			t.Fatalf("expected response with 401 status, got error: %v", err)
		}

		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", resp.StatusCode)
		}

		if attemptCount != 1 {
			t.Errorf("expected 1 attempt (no retries), got %d", attemptCount)
		}

		_ = resp.Body.Close()
	})

	t.Run("gives up after max retries", func(t *testing.T) {
		attemptCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Mock OAuth2 token endpoint
			if r.URL.Path == "/token" {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"access_token": "test_token", "token_type": "Bearer"}`))
				return
			}

			// Mock API endpoint that always fails
			attemptCount++
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"code": 500, "message": "Persistent Error"}`))
		}))
		defer server.Close()

		client := NewClient(Config{
			AccountID:         "test",
			ClientID:          "test",
			ClientSecret:      "test",
			BaseURL:           server.URL,
			AuthURL:           server.URL + "/token",
			MaxRetries:        2,
			InitialBackoff:    10 * time.Millisecond,
			MaxBackoff:        100 * time.Millisecond,
			BackoffMultiplier: 2.0,
		})

		resp, err := client.doRequest(context.Background(), "GET", "/test", nil)

		if err != nil {
			t.Fatalf("expected response with 500 status, got error: %v", err)
		}

		if resp.StatusCode != http.StatusInternalServerError {
			t.Errorf("expected status 500, got %d", resp.StatusCode)
		}

		// Should try initial + 2 retries = 3 total attempts
		if attemptCount != 3 {
			t.Errorf("expected 3 attempts (1 + 2 retries), got %d", attemptCount)
		}

		_ = resp.Body.Close()
	})

	t.Run("respects context cancellation", func(t *testing.T) {
		attemptCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Mock OAuth2 token endpoint
			if r.URL.Path == "/token" {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"access_token": "test_token", "token_type": "Bearer"}`))
				return
			}

			// Mock API endpoint that always fails
			attemptCount++
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"code": 500, "message": "Server Error"}`))
		}))
		defer server.Close()

		client := NewClient(Config{
			AccountID:         "test",
			ClientID:          "test",
			ClientSecret:      "test",
			BaseURL:           server.URL,
			AuthURL:           server.URL + "/token",
			MaxRetries:        5,
			InitialBackoff:    50 * time.Millisecond,
			MaxBackoff:        1 * time.Second,
			BackoffMultiplier: 2.0,
		})

		ctx, cancel := context.WithTimeout(context.Background(), 75*time.Millisecond)
		defer cancel()

		start := time.Now()
		_, err := client.doRequest(ctx, "GET", "/test", nil)
		elapsed := time.Since(start)

		if err == nil {
			t.Fatal("expected context cancellation error")
		}

		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("expected context.DeadlineExceeded, got: %v", err)
		}

		// Should have been cancelled before completing all retries
		if attemptCount > 3 {
			t.Errorf("expected fewer attempts due to context cancellation, got %d", attemptCount)
		}

		// Should have been cancelled reasonably quickly (within context timeout + small buffer)
		if elapsed > 150*time.Millisecond {
			t.Errorf("expected quick cancellation, took %v", elapsed)
		}
	})
}
