// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

type Service interface {
	ServiceReady() bool
}

// ServiceConfig is the configuration for the Services.
type ServiceConfig struct {
	// SkipEtagValidation is a flag to skip the Etag validation - only meant for local development.
	SkipEtagValidation bool
	// LFXEnvironment is the environment name for LFX app domain generation.
	LFXEnvironment string
}
