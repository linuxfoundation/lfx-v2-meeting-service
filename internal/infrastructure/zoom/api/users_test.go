// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClient_GetUsers(t *testing.T) {
	tests := []struct {
		name          string
		mockResponse  string
		mockStatus    int
		expectedError bool
		expectedCount int
		expectedFirst *ZoomUser
	}{
		{
			name: "successful get users",
			mockResponse: `{
				"page_count": 1,
				"page_number": 1,
				"page_size": 100,
				"total_records": 3,
				"users": [
					{
						"id": "user1",
						"email": "user1@example.com",
						"first_name": "John",
						"last_name": "Doe",
						"type": 2,
						"status": "active"
					},
					{
						"id": "user2",
						"email": "user2@example.com",
						"first_name": "Jane",
						"last_name": "Smith",
						"type": 1,
						"status": "active"
					},
					{
						"id": "user3",
						"email": "user3@example.com",
						"first_name": "Bob",
						"last_name": "Johnson",
						"type": 2,
						"status": "inactive"
					}
				]
			}`,
			mockStatus:    http.StatusOK,
			expectedError: false,
			expectedCount: 3,
			expectedFirst: &ZoomUser{
				ID:        "user1",
				Email:     "user1@example.com",
				FirstName: "John",
				LastName:  "Doe",
				Type:      UserTypeLicensed,
				Status:    UserStatusActive,
			},
		},
		{
			name: "empty users list",
			mockResponse: `{
				"page_count": 1,
				"page_number": 1,
				"page_size": 100,
				"total_records": 0,
				"users": []
			}`,
			mockStatus:    http.StatusOK,
			expectedError: false,
			expectedCount: 0,
		},
		{
			name: "single user",
			mockResponse: `{
				"page_count": 1,
				"page_number": 1,
				"page_size": 100,
				"total_records": 1,
				"users": [
					{
						"id": "admin-user",
						"email": "admin@example.com",
						"first_name": "Admin",
						"last_name": "User",
						"type": 2,
						"status": "active"
					}
				]
			}`,
			mockStatus:    http.StatusOK,
			expectedError: false,
			expectedCount: 1,
			expectedFirst: &ZoomUser{
				ID:        "admin-user",
				Email:     "admin@example.com",
				FirstName: "Admin",
				LastName:  "User",
				Type:      UserTypeLicensed,
				Status:    UserStatusActive,
			},
		},
		{
			name:          "unauthorized access",
			mockResponse:  `{"code": 401, "message": "Unauthorized"}`,
			mockStatus:    http.StatusUnauthorized,
			expectedError: true,
		},
		{
			name:          "forbidden access",
			mockResponse:  `{"code": 403, "message": "Forbidden"}`,
			mockStatus:    http.StatusForbidden,
			expectedError: true,
		},
		{
			name:          "rate limit exceeded",
			mockResponse:  `{"code": 429, "message": "Too Many Requests"}`,
			mockStatus:    http.StatusTooManyRequests,
			expectedError: true,
		},
		{
			name:          "server error",
			mockResponse:  `{"code": 500, "message": "Internal Server Error"}`,
			mockStatus:    http.StatusInternalServerError,
			expectedError: true,
		},
		{
			name:          "invalid JSON response",
			mockResponse:  `invalid json response`,
			mockStatus:    http.StatusOK,
			expectedError: true,
		},
		{
			name: "malformed users data",
			mockResponse: `{
				"page_count": 1,
				"page_number": 1,
				"page_size": 100,
				"total_records": 1,
				"users": [
					{
						"id": "user1",
						"email": "invalid-email",
						"type": "invalid-type",
						"status": 123
					}
				]
			}`,
			mockStatus:    http.StatusOK,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Handle OAuth token request
				if r.URL.Path == "/oauth/token" {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`{"access_token": "test-token", "token_type": "Bearer", "expires_in": 3600}`))
					return
				}

				// Verify request method and path
				expectedPath := "/users"
				if r.URL.Path != expectedPath {
					t.Errorf("expected path %s, got %s", expectedPath, r.URL.Path)
				}
				if r.Method != http.MethodGet {
					t.Errorf("expected method GET, got %s", r.Method)
				}

				// Verify query parameters
				expectedParams := "status=active&page_size=100"
				if r.URL.RawQuery != expectedParams {
					t.Errorf("expected query params %s, got %s", expectedParams, r.URL.RawQuery)
				}

				// Verify Authorization header
				authHeader := r.Header.Get("Authorization")
				if authHeader != "Bearer test-token" {
					t.Errorf("expected Authorization 'Bearer test-token', got %s", authHeader)
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.mockStatus)
				_, _ = w.Write([]byte(tt.mockResponse))
			}))
			defer server.Close()

			// Create client with mock server URL
			client := NewClient(Config{
				AccountID:    "test-account",
				ClientID:     "test-client",
				ClientSecret: "test-secret",
				BaseURL:      server.URL,
				AuthURL:      server.URL + "/oauth/token",
			})

			ctx := context.Background()
			users, err := client.GetUsers(ctx)

			if tt.expectedError {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if len(users) != tt.expectedCount {
				t.Errorf("expected %d users, got %d", tt.expectedCount, len(users))
				return
			}

			if tt.expectedFirst != nil && tt.expectedCount > 0 {
				firstUser := users[0]
				if firstUser.ID != tt.expectedFirst.ID {
					t.Errorf("expected first user ID %s, got %s", tt.expectedFirst.ID, firstUser.ID)
				}
				if firstUser.Email != tt.expectedFirst.Email {
					t.Errorf("expected first user email %s, got %s", tt.expectedFirst.Email, firstUser.Email)
				}
				if firstUser.FirstName != tt.expectedFirst.FirstName {
					t.Errorf("expected first user first name %s, got %s", tt.expectedFirst.FirstName, firstUser.FirstName)
				}
				if firstUser.LastName != tt.expectedFirst.LastName {
					t.Errorf("expected first user last name %s, got %s", tt.expectedFirst.LastName, firstUser.LastName)
				}
				if firstUser.Type != tt.expectedFirst.Type {
					t.Errorf("expected first user type %d, got %d", tt.expectedFirst.Type, firstUser.Type)
				}
				if firstUser.Status != tt.expectedFirst.Status {
					t.Errorf("expected first user status %s, got %s", tt.expectedFirst.Status, firstUser.Status)
				}
			}
		})
	}
}

func TestClient_GetUsers_ContextCancellation(t *testing.T) {
	// Test context cancellation during request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handle OAuth token request
		if r.URL.Path == "/oauth/token" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"access_token": "test-token", "token_type": "Bearer", "expires_in": 3600}`))
			return
		}

		// Simulate slow response
		select {
		case <-r.Context().Done():
			return
		case <-time.After(100 * time.Millisecond):
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"users": []}`))
		}
	}))
	defer server.Close()

	client := NewClient(Config{
		AccountID:    "test-account",
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		BaseURL:      server.URL,
		AuthURL:      server.URL + "/oauth/token",
	})

	// Create context that cancels immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.GetUsers(ctx)
	if err == nil {
		t.Error("expected error due to context cancellation")
	}

	if !strings.Contains(err.Error(), "context canceled") {
		t.Errorf("expected context cancellation error, got: %v", err)
	}
}
