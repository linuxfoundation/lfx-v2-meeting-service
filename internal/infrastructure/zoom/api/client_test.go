// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package api

import (
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
