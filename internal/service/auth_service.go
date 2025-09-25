// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"log/slog"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/infrastructure/auth"
)

type AuthService struct {
	auth auth.IJWTAuth
}

func NewAuthService(auth auth.IJWTAuth) *AuthService {
	return &AuthService{
		auth: auth,
	}
}

// ServiceReady checks if the service is ready for use.
func (s *AuthService) ServiceReady() bool {
	return s.auth != nil
}

// ParsePrincipal parses the Heimdall-authorized principal from the bearer token.
func (s *AuthService) ParsePrincipal(ctx context.Context, bearerToken string, logger *slog.Logger) (string, error) {
	if !s.ServiceReady() {
		return "", domain.NewUnavailableError("auth service not ready")
	}

	return s.auth.ParsePrincipal(ctx, bearerToken, logger)
}
