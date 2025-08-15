// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_CreateMeeting(t *testing.T) {
	tests := []struct {
		name           string
		userID         string
		request        *CreateMeetingRequest
		mockResponse   string
		mockStatus     int
		expectedError  bool
		expectedID     int64
		expectedJoinURL string
	}{
		{
			name:   "successful creation",
			userID: "user123",
			request: &CreateMeetingRequest{
				Topic:    "Test Meeting",
				Type:     MeetingTypeScheduled,
				Duration: 60,
				Timezone: "UTC",
			},
			mockResponse: `{
				"id": 123456789,
				"uuid": "test-uuid-123",
				"host_id": "user123",
				"topic": "Test Meeting",
				"type": 2,
				"status": "waiting",
				"duration": 60,
				"timezone": "UTC",
				"join_url": "https://zoom.us/j/123456789",
				"password": "test123"
			}`,
			mockStatus:      http.StatusCreated,
			expectedError:   false,
			expectedID:      123456789,
			expectedJoinURL: "https://zoom.us/j/123456789",
		},
		{
			name:   "meeting with settings",
			userID: "user456",
			request: &CreateMeetingRequest{
				Topic:    "Meeting with Settings",
				Type:     MeetingTypeRecurringNoFixedTime,
				Duration: 90,
				Settings: &MeetingSettings{
					AutoRecording:                 "cloud",
					AutoStartAICompanionQuestions: true,
					AutoStartMeetingSummary:       true,
				},
			},
			mockResponse: `{
				"id": 987654321,
				"topic": "Meeting with Settings",
				"type": 3,
				"duration": 90,
				"join_url": "https://zoom.us/j/987654321",
				"password": "pass456"
			}`,
			mockStatus:      http.StatusCreated,
			expectedError:   false,
			expectedID:      987654321,
			expectedJoinURL: "https://zoom.us/j/987654321",
		},
		{
			name:   "API error - unauthorized",
			userID: "invalid-user",
			request: &CreateMeetingRequest{
				Topic: "Test Meeting",
				Type:  MeetingTypeScheduled,
			},
			mockResponse:  `{"code": 401, "message": "Unauthorized"}`,
			mockStatus:    http.StatusUnauthorized,
			expectedError: true,
		},
		{
			name:   "API error - user not found",
			userID: "nonexistent-user",
			request: &CreateMeetingRequest{
				Topic: "Test Meeting",
				Type:  MeetingTypeScheduled,
			},
			mockResponse:  `{"code": 1001, "message": "User does not exist"}`,
			mockStatus:    http.StatusNotFound,
			expectedError: true,
		},
		{
			name:   "invalid JSON response",
			userID: "user123",
			request: &CreateMeetingRequest{
				Topic: "Test Meeting",
				Type:  MeetingTypeScheduled,
			},
			mockResponse:  `invalid json`,
			mockStatus:    http.StatusCreated,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock server that handles both OAuth and API requests
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Handle OAuth token request first
				if r.URL.Path == "/oauth/token" {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`{"access_token": "test-token", "token_type": "Bearer", "expires_in": 3600}`))
					return
				}

				// Handle API request
				expectedPath := "/users/" + tt.userID + "/meetings"
				if r.URL.Path != expectedPath {
					t.Errorf("expected path %s, got %s", expectedPath, r.URL.Path)
				}
				if r.Method != http.MethodPost {
					t.Errorf("expected method POST, got %s", r.Method)
				}

				// Verify content type
				if r.Header.Get("Content-Type") != "application/json" {
					t.Errorf("expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
				}

				// Verify Authorization header
				authHeader := r.Header.Get("Authorization")
				if authHeader != "Bearer test-token" {
					t.Errorf("expected Authorization 'Bearer test-token', got %s", authHeader)
				}

				// Verify request body
				var reqBody CreateMeetingRequest
				if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil && tt.mockStatus < 400 {
					t.Errorf("failed to decode request body: %v", err)
				} else if tt.mockStatus < 400 {
					if reqBody.Topic != tt.request.Topic {
						t.Errorf("expected topic %s, got %s", tt.request.Topic, reqBody.Topic)
					}
					if reqBody.Type != tt.request.Type {
						t.Errorf("expected type %d, got %d", tt.request.Type, reqBody.Type)
					}
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
				AuthURL:      server.URL + "/oauth/token", // Not used in this test but needed for client
			})

			ctx := context.Background()
			resp, err := client.CreateMeeting(ctx, tt.userID, tt.request)

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

			if resp == nil {
				t.Fatal("expected response but got nil")
			}

			if resp.ID != tt.expectedID {
				t.Errorf("expected ID %d, got %d", tt.expectedID, resp.ID)
			}

			if resp.JoinURL != tt.expectedJoinURL {
				t.Errorf("expected JoinURL %s, got %s", tt.expectedJoinURL, resp.JoinURL)
			}
		})
	}
}

func TestClient_UpdateMeeting(t *testing.T) {
	tests := []struct {
		name          string
		meetingID     string
		request       *UpdateMeetingRequest
		mockResponse  string
		mockStatus    int
		expectedError bool
	}{
		{
			name:      "successful update",
			meetingID: "123456789",
			request: &UpdateMeetingRequest{
				Topic:    "Updated Meeting",
				Duration: 120,
				Timezone: "America/New_York",
			},
			mockResponse:  ``, // Update typically returns empty body
			mockStatus:    http.StatusNoContent,
			expectedError: false,
		},
		{
			name:      "update with settings",
			meetingID: "987654321",
			request: &UpdateMeetingRequest{
				Topic: "Meeting with New Settings",
				Settings: &MeetingSettings{
					AutoRecording: "local",
				},
			},
			mockResponse:  ``,
			mockStatus:    http.StatusOK,
			expectedError: false,
		},
		{
			name:          "meeting not found",
			meetingID:     "nonexistent",
			request:       &UpdateMeetingRequest{Topic: "Test"},
			mockResponse:  `{"code": 3001, "message": "Meeting does not exist"}`,
			mockStatus:    http.StatusNotFound,
			expectedError: true,
		},
		{
			name:          "invalid meeting ID",
			meetingID:     "invalid-id",
			request:       &UpdateMeetingRequest{Topic: "Test"},
			mockResponse:  `{"code": 300, "message": "Invalid meeting ID"}`,
			mockStatus:    http.StatusBadRequest,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Handle OAuth token request
				if r.URL.Path == "/oauth/token" {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`{"access_token": "test-token", "token_type": "Bearer", "expires_in": 3600}`))
					return
				}

				expectedPath := "/meetings/" + tt.meetingID
				if r.URL.Path != expectedPath {
					t.Errorf("expected path %s, got %s", expectedPath, r.URL.Path)
				}
				if r.Method != http.MethodPatch {
					t.Errorf("expected method PATCH, got %s", r.Method)
				}

				if r.Header.Get("Content-Type") != "application/json" {
					t.Errorf("expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
				}

				// Verify Authorization header
				authHeader := r.Header.Get("Authorization")
				if authHeader != "Bearer test-token" {
					t.Errorf("expected Authorization 'Bearer test-token', got %s", authHeader)
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.mockStatus)
				if tt.mockResponse != "" {
					_, _ = w.Write([]byte(tt.mockResponse))
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

			ctx := context.Background()
			err := client.UpdateMeeting(ctx, tt.meetingID, tt.request)

			if tt.expectedError {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestClient_DeleteMeeting(t *testing.T) {
	tests := []struct {
		name          string
		meetingID     string
		mockResponse  string
		mockStatus    int
		expectedError bool
	}{
		{
			name:          "successful deletion",
			meetingID:     "123456789",
			mockResponse:  ``,
			mockStatus:    http.StatusNoContent,
			expectedError: false,
		},
		{
			name:          "successful deletion with OK status",
			meetingID:     "987654321",
			mockResponse:  ``,
			mockStatus:    http.StatusOK,
			expectedError: false,
		},
		{
			name:          "meeting not found",
			meetingID:     "nonexistent",
			mockResponse:  `{"code": 3001, "message": "Meeting does not exist"}`,
			mockStatus:    http.StatusNotFound,
			expectedError: true,
		},
		{
			name:          "invalid meeting ID",
			meetingID:     "invalid",
			mockResponse:  `{"code": 300, "message": "Invalid meeting ID"}`,
			mockStatus:    http.StatusBadRequest,
			expectedError: true,
		},
		{
			name:          "unauthorized",
			meetingID:     "123456789",
			mockResponse:  `{"code": 401, "message": "Unauthorized"}`,
			mockStatus:    http.StatusUnauthorized,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Handle OAuth token request
				if r.URL.Path == "/oauth/token" {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`{"access_token": "test-token", "token_type": "Bearer", "expires_in": 3600}`))
					return
				}

				expectedPath := "/meetings/" + tt.meetingID
				if r.URL.Path != expectedPath {
					t.Errorf("expected path %s, got %s", expectedPath, r.URL.Path)
				}
				if r.Method != http.MethodDelete {
					t.Errorf("expected method DELETE, got %s", r.Method)
				}

				// Verify Authorization header
				authHeader := r.Header.Get("Authorization")
				if authHeader != "Bearer test-token" {
					t.Errorf("expected Authorization 'Bearer test-token', got %s", authHeader)
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.mockStatus)
				if tt.mockResponse != "" {
					_, _ = w.Write([]byte(tt.mockResponse))
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

			ctx := context.Background()
			err := client.DeleteMeeting(ctx, tt.meetingID)

			if tt.expectedError {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}