// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import "github.com/linuxfoundation/lfx-v2-meeting-service/pkg/constants"

type Service interface {
	ServiceReady() bool
}

// ServiceConfig is the configuration for the Services.
type ServiceConfig struct {
	// SkipEtagValidation is a flag to skip the Etag validation - only meant for local development.
	SkipEtagValidation bool
	// ProjectLogoBaseURL is the base URL for project logo PNG images.
	ProjectLogoBaseURL string
	// LfxURLGenerator generates LFX app URLs with the configured environment and custom origin.
	LfxURLGenerator *constants.LfxURLGenerator
}
