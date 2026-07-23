// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package auth

import (
	"context"
	"log/slog"
	"testing"

	"github.com/auth0/go-jwt-middleware/v2/validator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	jose "gopkg.in/go-jose/go-jose.v2"
	"gopkg.in/go-jose/go-jose.v2/jwt"
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

// signHeimdallToken builds and HS256-signs a JWT carrying the given HeimdallClaims, for use
// against a validator constructed with a matching HMAC keyFunc.
func signHeimdallToken(t *testing.T, secret []byte, claims HeimdallClaims) string {
	t.Helper()

	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.HS256, Key: secret}, nil)
	require.NoError(t, err)

	token, err := jwt.Signed(signer).
		Claims(jwt.Claims{Issuer: defaultIssuer, Audience: jwt.Audience{defaultAudience}}).
		Claims(claims).
		CompactSerialize()
	require.NoError(t, err)

	return token
}

// TestParsePrincipalAndEmail tests the ParsePrincipalAndEmail method, including the
// successfully-validated-token path with an email claim (regression coverage for the
// created_by identity resolution used by meeting creation).
func TestParsePrincipalAndEmail(t *testing.T) {
	secret := []byte("test-secret")
	v, err := validator.New(
		func(_ context.Context) (interface{}, error) { return secret, nil },
		validator.HS256,
		defaultIssuer,
		[]string{defaultAudience},
		validator.WithCustomClaims(customClaims),
	)
	require.NoError(t, err)
	auth := &JWTAuth{validator: v}

	t.Run("validated token with email claim returns principal and email", func(t *testing.T) {
		token := signHeimdallToken(t, secret, HeimdallClaims{Principal: "user123", Email: "user123@example.com"})

		principal, email, err := auth.ParsePrincipalAndEmail(context.Background(), token, slog.Default())

		assert.NoError(t, err)
		assert.Equal(t, "user123", principal)
		assert.Equal(t, "user123@example.com", email)
	})

	t.Run("validated token without email claim returns empty email", func(t *testing.T) {
		token := signHeimdallToken(t, secret, HeimdallClaims{Principal: "user123"})

		principal, email, err := auth.ParsePrincipalAndEmail(context.Background(), token, slog.Default())

		assert.NoError(t, err)
		assert.Equal(t, "user123", principal)
		assert.Empty(t, email)
	})
}
