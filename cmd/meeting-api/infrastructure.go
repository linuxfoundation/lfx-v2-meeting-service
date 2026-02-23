// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package main

import (
	"os"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/infrastructure/auth"
)

// setupJWTAuth configures JWT authentication for the service
func setupJWTAuth() (*auth.JWTAuth, error) {
	jwtAuthConfig := auth.JWTAuthConfig{
		JWKSURL:            os.Getenv("JWKS_URL"),
		Audience:           os.Getenv("JWT_AUDIENCE"),
		MockLocalPrincipal: os.Getenv("JWT_AUTH_DISABLED_MOCK_LOCAL_PRINCIPAL"),
	}
	return auth.NewJWTAuth(jwtAuthConfig)
}
