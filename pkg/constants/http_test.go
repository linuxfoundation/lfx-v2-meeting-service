// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package constants

import (
	"testing"
)

func TestHTTPHeaderConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant string
		expected string
	}{
		{
			name:     "AuthorizationHeader",
			constant: AuthorizationHeader,
			expected: "authorization",
		},
		{
			name:     "RequestIDHeader",
			constant: RequestIDHeader,
			expected: "X-REQUEST-ID",
		},
		{
			name:     "EtagHeader",
			constant: EtagHeader,
			expected: "ETag",
		},
		{
			name:     "XOnBehalfOfHeader",
			constant: XOnBehalfOfHeader,
			expected: "x-on-behalf-of",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.constant != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, tt.constant)
			}
		})
	}
}

func TestContextIDConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant string
		expected string
	}{
		{
			name:     "RequestIDContextID",
			constant: string(RequestIDContextID),
			expected: "X-REQUEST-ID",
		},
		{
			name:     "AuthorizationContextID",
			constant: string(AuthorizationContextID),
			expected: "authorization",
		},
		{
			name:     "PrincipalContextID",
			constant: string(PrincipalContextID),
			expected: "x-on-behalf-of",
		},
		{
			name:     "ETagContextID",
			constant: string(ETagContextID),
			expected: "etag",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.constant != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, tt.constant)
			}
		})
	}
}

func TestContextIDTypes(t *testing.T) {
	// Test that context ID types are properly typed
	var requestID contextRequestID = "test"
	if string(requestID) != "test" {
		t.Error("contextRequestID type conversion failed")
	}

	var authorization contextAuthorization = "test"
	if string(authorization) != "test" {
		t.Error("contextAuthorization type conversion failed")
	}

	var principal contextPrincipal = "test"
	if string(principal) != "test" {
		t.Error("contextPrincipal type conversion failed")
	}

	var etag contextEtag = "test"
	if string(etag) != "test" {
		t.Error("contextEtag type conversion failed")
	}
}

func TestContextIDConstantsAreUnique(t *testing.T) {
	contextIDs := map[string]string{
		"RequestIDContextID":     string(RequestIDContextID),
		"AuthorizationContextID": string(AuthorizationContextID),
		"PrincipalContextID":     string(PrincipalContextID),
		"ETagContextID":          string(ETagContextID),
	}

	// Check for duplicates
	seen := make(map[string]string)
	for name, value := range contextIDs {
		if existingName, exists := seen[value]; exists {
			t.Errorf("duplicate context ID value %q found in both %s and %s", value, existingName, name)
		}
		seen[value] = name
	}
}

func TestHeaderConstants(t *testing.T) {
	// Test that header constants match their intended usage
	if AuthorizationHeader != "authorization" {
		t.Errorf("AuthorizationHeader should be 'authorization' for standard HTTP auth header")
	}

	if RequestIDHeader != "X-REQUEST-ID" {
		t.Errorf("RequestIDHeader should be 'X-REQUEST-ID' for request tracing")
	}

	if EtagHeader != "ETag" {
		t.Errorf("EtagHeader should be 'ETag' for HTTP caching")
	}

	if XOnBehalfOfHeader != "x-on-behalf-of" {
		t.Errorf("XOnBehalfOfHeader should be 'x-on-behalf-of' for delegation")
	}
}

func TestContextMappingConsistency(t *testing.T) {
	// Test that context IDs match their corresponding header names where appropriate
	if string(RequestIDContextID) != RequestIDHeader {
		t.Errorf("RequestIDContextID (%q) should match RequestIDHeader (%q)", RequestIDContextID, RequestIDHeader)
	}

	if string(AuthorizationContextID) != AuthorizationHeader {
		t.Errorf("AuthorizationContextID (%q) should match AuthorizationHeader (%q)", AuthorizationContextID, AuthorizationHeader)
	}

	if string(PrincipalContextID) != XOnBehalfOfHeader {
		t.Errorf("PrincipalContextID (%q) should match XOnBehalfOfHeader (%q)", PrincipalContextID, XOnBehalfOfHeader)
	}
}

func TestGenerateLFXMeetingURL(t *testing.T) {
	tests := []struct {
		name        string
		envValue    string
		meetingUID  string
		password    string
		expectedURL string
	}{
		{
			name:        "valid meeting URL production",
			envValue:    "prod",
			meetingUID:  "123e4567-e89b-12d3-a456-426614174000",
			password:    "456e7890-e89b-12d3-a456-426614174001",
			expectedURL: "https://" + LFXDomainProd + "/meetings/123e4567-e89b-12d3-a456-426614174000?password=456e7890-e89b-12d3-a456-426614174001",
		},
		{
			name:        "valid meeting URL development",
			envValue:    "dev",
			meetingUID:  "123e4567-e89b-12d3-a456-426614174000",
			password:    "456e7890-e89b-12d3-a456-426614174001",
			expectedURL: "https://" + LFXDomainDev + "/meetings/123e4567-e89b-12d3-a456-426614174000?password=456e7890-e89b-12d3-a456-426614174001",
		},
		{
			name:        "valid meeting URL staging",
			envValue:    "staging",
			meetingUID:  "123e4567-e89b-12d3-a456-426614174000",
			password:    "456e7890-e89b-12d3-a456-426614174001",
			expectedURL: "https://" + LFXDomainStaging + "/meetings/123e4567-e89b-12d3-a456-426614174000?password=456e7890-e89b-12d3-a456-426614174001",
		},
		{
			name:        "empty values",
			envValue:    "",
			meetingUID:  "",
			password:    "",
			expectedURL: "https://" + LFXDomainProd + "/meetings/?password=",
		},
		{
			name:        "special characters in password",
			envValue:    "",
			meetingUID:  "meeting-123",
			password:    "pass@#$%",
			expectedURL: "https://" + LFXDomainProd + "/meetings/meeting-123?password=pass%40%23%24%25",
		},
		{
			name:        "password with spaces",
			envValue:    "",
			meetingUID:  "meeting-456",
			password:    "password with spaces",
			expectedURL: "https://" + LFXDomainProd + "/meetings/meeting-456?password=password+with+spaces",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateLFXMeetingURL(tt.meetingUID, tt.password, tt.envValue)
			if result != tt.expectedURL {
				t.Errorf("GenerateLFXMeetingURL(%q, %q, %q) = %q, expected %q", tt.meetingUID, tt.password, tt.envValue, result, tt.expectedURL)
			}
		})
	}
}

func TestGetLFXAppDomain(t *testing.T) {
	tests := []struct {
		name        string
		envValue    string
		expectedURL string
	}{
		{
			name:        "development environment",
			envValue:    "dev",
			expectedURL: LFXDomainDev,
		},
		{
			name:        "staging environment",
			envValue:    "staging",
			expectedURL: LFXDomainStaging,
		},
		{
			name:        "production environment",
			envValue:    "prod",
			expectedURL: LFXDomainProd,
		},
		{
			name:        "empty environment defaults to production",
			envValue:    "",
			expectedURL: LFXDomainProd,
		},
		{
			name:        "unknown environment defaults to production",
			envValue:    "unknown",
			expectedURL: LFXDomainProd,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetLFXAppDomain(tt.envValue)
			if result != tt.expectedURL {
				t.Errorf("GetLFXAppDomain(%q) = %q, expected %q", tt.envValue, result, tt.expectedURL)
			}
		})
	}
}
