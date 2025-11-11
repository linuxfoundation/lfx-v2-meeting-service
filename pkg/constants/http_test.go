// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package constants

import (
	"testing"
)

func TestContextIDConstantsAreUnique(t *testing.T) {
	// Verify that context ID constants don't have duplicate values
	// which could cause context key collisions at runtime
	contextIDs := map[string]string{
		"RequestIDContextID":     string(RequestIDContextID),
		"AuthorizationContextID": string(AuthorizationContextID),
		"PrincipalContextID":     string(PrincipalContextID),
		"ETagContextID":          string(ETagContextID),
	}

	seen := make(map[string]string)
	for name, value := range contextIDs {
		if existingName, exists := seen[value]; exists {
			t.Errorf("duplicate context ID value %q found in both %s and %s", value, existingName, name)
		}
		seen[value] = name
	}
}

func TestContextMappingConsistency(t *testing.T) {
	// Verify that context IDs match their corresponding header names
	// This is a rule to maintain consistency between HTTP headers and context keys
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

func TestLfxURLGenerator_GenerateMeetingURL(t *testing.T) {
	tests := []struct {
		name            string
		environment     string
		customAppOrigin string
		meetingUID      string
		password        string
		expectedURL     string
	}{
		{
			name:            "valid meeting URL production",
			environment:     "prod",
			customAppOrigin: "",
			meetingUID:      "123e4567-e89b-12d3-a456-426614174000",
			password:        "456e7890-e89b-12d3-a456-426614174001",
			expectedURL:     "https://" + LFXDomainProd + "/meetings/123e4567-e89b-12d3-a456-426614174000?password=456e7890-e89b-12d3-a456-426614174001",
		},
		{
			name:            "valid meeting URL development",
			environment:     "dev",
			customAppOrigin: "",
			meetingUID:      "123e4567-e89b-12d3-a456-426614174000",
			password:        "456e7890-e89b-12d3-a456-426614174001",
			expectedURL:     "https://" + LFXDomainDev + "/meetings/123e4567-e89b-12d3-a456-426614174000?password=456e7890-e89b-12d3-a456-426614174001",
		},
		{
			name:            "valid meeting URL staging",
			environment:     "staging",
			customAppOrigin: "",
			meetingUID:      "123e4567-e89b-12d3-a456-426614174000",
			password:        "456e7890-e89b-12d3-a456-426614174001",
			expectedURL:     "https://" + LFXDomainStaging + "/meetings/123e4567-e89b-12d3-a456-426614174000?password=456e7890-e89b-12d3-a456-426614174001",
		},
		{
			name:            "custom app origin overrides environment",
			environment:     "prod",
			customAppOrigin: "https://custom.example.com",
			meetingUID:      "123e4567-e89b-12d3-a456-426614174000",
			password:        "456e7890-e89b-12d3-a456-426614174001",
			expectedURL:     "https://custom.example.com/meetings/123e4567-e89b-12d3-a456-426614174000?password=456e7890-e89b-12d3-a456-426614174001",
		},
		{
			name:            "empty values",
			environment:     "",
			customAppOrigin: "",
			meetingUID:      "",
			password:        "",
			expectedURL:     "https://" + LFXDomainProd + "/meetings/?password=",
		},
		{
			name:            "special characters in password",
			environment:     "",
			customAppOrigin: "",
			meetingUID:      "meeting-123",
			password:        "pass@#$%",
			expectedURL:     "https://" + LFXDomainProd + "/meetings/meeting-123?password=pass%40%23%24%25",
		},
		{
			name:            "password with spaces",
			environment:     "",
			customAppOrigin: "",
			meetingUID:      "meeting-456",
			password:        "password with spaces",
			expectedURL:     "https://" + LFXDomainProd + "/meetings/meeting-456?password=password+with+spaces",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generator := NewLfxURLGenerator(tt.environment, tt.customAppOrigin)
			result := generator.GenerateMeetingURL(tt.meetingUID, tt.password)
			if result != tt.expectedURL {
				t.Errorf("GenerateMeetingURL(%q, %q) = %q, expected %q", tt.meetingUID, tt.password, result, tt.expectedURL)
			}
		})
	}
}

func TestGetLFXAppDomain(t *testing.T) {
	tests := []struct {
		name        string
		environment string
		expectedURL string
	}{
		{
			name:        "development environment",
			environment: "dev",
			expectedURL: LFXDomainDev,
		},
		{
			name:        "staging environment",
			environment: "staging",
			expectedURL: LFXDomainStaging,
		},
		{
			name:        "production environment",
			environment: "prod",
			expectedURL: LFXDomainProd,
		},
		{
			name:        "empty environment defaults to production",
			environment: "",
			expectedURL: LFXDomainProd,
		},
		{
			name:        "unknown environment defaults to production",
			environment: "unknown",
			expectedURL: LFXDomainProd,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetLFXAppDomain(tt.environment)
			if result != tt.expectedURL {
				t.Errorf("GetLFXAppDomain(%q) = %q, expected %q", tt.environment, result, tt.expectedURL)
			}
		})
	}
}

func TestLfxURLGenerator_GenerateMeetingDetailsURL(t *testing.T) {
	tests := []struct {
		name            string
		environment     string
		customAppOrigin string
		projectSlug     string
		meetingUID      string
		expectedURL     string
	}{
		{
			name:            "valid meeting details URL production",
			environment:     "prod",
			customAppOrigin: "",
			projectSlug:     "thelinuxfoundation",
			meetingUID:      "123e4567-e89b-12d3-a456-426614174000",
			expectedURL:     "https://" + LFXDomainProd + "/project/thelinuxfoundation/meetings#meeting-123e4567-e89b-12d3-a456-426614174000",
		},
		{
			name:            "valid meeting details URL development",
			environment:     "dev",
			customAppOrigin: "",
			projectSlug:     "kubernetes",
			meetingUID:      "223e4567-e89b-12d3-a456-426614174000",
			expectedURL:     "https://" + LFXDomainDev + "/project/kubernetes/meetings#meeting-223e4567-e89b-12d3-a456-426614174000",
		},
		{
			name:            "valid meeting details URL staging",
			environment:     "staging",
			customAppOrigin: "",
			projectSlug:     "cncf",
			meetingUID:      "323e4567-e89b-12d3-a456-426614174000",
			expectedURL:     "https://" + LFXDomainStaging + "/project/cncf/meetings#meeting-323e4567-e89b-12d3-a456-426614174000",
		},
		{
			name:            "custom app origin overrides environment",
			environment:     "prod",
			customAppOrigin: "https://custom.example.com",
			projectSlug:     "test-project",
			meetingUID:      "423e4567-e89b-12d3-a456-426614174000",
			expectedURL:     "https://custom.example.com/project/test-project/meetings#meeting-423e4567-e89b-12d3-a456-426614174000",
		},
		{
			name:            "empty environment defaults to production",
			environment:     "",
			customAppOrigin: "",
			projectSlug:     "test-project",
			meetingUID:      "423e4567-e89b-12d3-a456-426614174000",
			expectedURL:     "https://" + LFXDomainProd + "/project/test-project/meetings#meeting-423e4567-e89b-12d3-a456-426614174000",
		},
		{
			name:            "unknown environment defaults to production",
			environment:     "unknown",
			customAppOrigin: "",
			projectSlug:     "another-project",
			meetingUID:      "523e4567-e89b-12d3-a456-426614174000",
			expectedURL:     "https://" + LFXDomainProd + "/project/another-project/meetings#meeting-523e4567-e89b-12d3-a456-426614174000",
		},
		{
			name:            "empty project slug",
			environment:     "prod",
			customAppOrigin: "",
			projectSlug:     "",
			meetingUID:      "623e4567-e89b-12d3-a456-426614174000",
			expectedURL:     "https://" + LFXDomainProd + "/project//meetings#meeting-623e4567-e89b-12d3-a456-426614174000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generator := NewLfxURLGenerator(tt.environment, tt.customAppOrigin)
			result := generator.GenerateMeetingDetailsURL(tt.projectSlug, tt.meetingUID)
			if result != tt.expectedURL {
				t.Errorf("GenerateMeetingDetailsURL(%q, %q) = %q, expected %q", tt.projectSlug, tt.meetingUID, result, tt.expectedURL)
			}
		})
	}
}
