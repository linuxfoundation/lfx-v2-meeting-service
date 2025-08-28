// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import "github.com/linuxfoundation/lfx-v2-meeting-service/internal/infrastructure/auth"

type AuthService struct {
	Auth auth.IJWTAuth
}

func NewAuthService(auth auth.IJWTAuth) *AuthService {
	return &AuthService{
		Auth: auth,
	}
}

// ServiceReady checks if the service is ready for use.
func (s *AuthService) ServiceReady() bool {
	return s.Auth != nil
}
