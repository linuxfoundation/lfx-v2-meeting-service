// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package auth

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHeimdallClaims_Validate tests the Validate method of HeimdallClaims
func TestHeimdallClaims_Validate(t *testing.T) {
	tests := []struct {
		name      string
		principal string
		wantErr   bool
	}{
		{
			name:      "valid principal",
			principal: "user123",
			wantErr:   false,
		},
		{
			name:      "empty principal returns error",
			principal: "",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims := &HeimdallClaims{Principal: tt.principal}
			err := claims.Validate(context.Background())

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "principal must be provided")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestNewJWTAuth tests the NewJWTAuth constructor
func TestNewJWTAuth(t *testing.T) {
	tests := []struct {
		name      string
		config    JWTAuthConfig
		wantErr   bool
		expectNil bool
	}{
		{
			name:      "default configuration",
			config:    JWTAuthConfig{},
			wantErr:   false,
			expectNil: false,
		},
		{
			name: "custom configuration",
			config: JWTAuthConfig{
				JWKSURL:  "http://custom:4457/.well-known/jwks",
				Audience: "custom-audience",
			},
			wantErr:   false,
			expectNil: false,
		},
		{
			name: "invalid JWKS URL",
			config: JWTAuthConfig{
				JWKSURL: "://invalid-url",
			},
			wantErr:   true,
			expectNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth, err := NewJWTAuth(tt.config)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if tt.expectNil {
				assert.Nil(t, auth)
			} else {
				assert.NotNil(t, auth)
				assert.NotNil(t, auth.validator)
			}
		})
	}
}

// TestParsePrincipal tests the ParsePrincipal method
func TestParsePrincipal(t *testing.T) {
	t.Run("mock mode returns configured principal", func(t *testing.T) {
		auth := &JWTAuth{
			config: JWTAuthConfig{
				MockLocalPrincipal: "test-user",
			},
		}

		principal, err := auth.ParsePrincipal(context.Background(), "any-token", slog.Default())

		assert.NoError(t, err)
		assert.Equal(t, "test-user", principal)
	})

	t.Run("nil validator returns error", func(t *testing.T) {
		auth := &JWTAuth{
			validator: nil,
			config:    JWTAuthConfig{}, // No mock principal
		}

		principal, err := auth.ParsePrincipal(context.Background(), "some-token", slog.Default())

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "JWT validator is not set up")
		assert.Empty(t, principal)
	})

	t.Run("invalid tokens return validation errors", func(t *testing.T) {
		// Create real JWT auth instance
		auth, err := NewJWTAuth(JWTAuthConfig{
			JWKSURL:  "http://localhost:9999/.well-known/jwks",
			Audience: "test-audience",
		})
		require.NoError(t, err)

		tests := []struct {
			name  string
			token string
		}{
			{"empty token", ""},
			{"malformed token", "invalid.token"},
			{"wrong algorithm", "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.invalidsignature"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				principal, err := auth.ParsePrincipal(context.Background(), tt.token, slog.Default())

				assert.Error(t, err)
				assert.Empty(t, principal)
			})
		}
	})
}
