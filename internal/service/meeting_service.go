// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/infrastructure/auth"
)

// MeetingsService implements the meetingsvc.Service interface and domain.MessageHandler
type MeetingsService struct {
	MeetingRepository domain.MeetingRepository
	MessageBuilder    domain.MessageBuilder
	Auth              auth.IJWTAuth
	Config            ServiceConfig
}

// NewMeetingsService creates a new MeetingsService.
func NewMeetingsService(auth auth.IJWTAuth, config ServiceConfig) *MeetingsService {
	return &MeetingsService{
		Auth:   auth,
		Config: config,
	}
}

// ServiceReady checks if the service is ready for use.
func (s *MeetingsService) ServiceReady() bool {
	return s.MeetingRepository != nil && s.MessageBuilder != nil
}

// ServiceConfig is the configuration for the MeetingsService.
type ServiceConfig struct {
	// SkipEtagValidation is a flag to skip the Etag validation - only meant for local development.
	SkipEtagValidation bool
}
