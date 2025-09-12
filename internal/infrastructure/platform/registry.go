// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package platform

import (
	"fmt"
	"sync"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
)

// Registry implements the PlatformRegistry interface
type Registry struct {
	providers map[string]domain.PlatformProvider
	mu        sync.RWMutex
}

// NewRegistry creates a new platform registry
func NewRegistry() domain.PlatformRegistry {
	return &Registry{
		providers: make(map[string]domain.PlatformProvider),
	}
}

// GetProvider returns the platform provider for the specified platform name
func (r *Registry) GetProvider(platform string) (domain.PlatformProvider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	provider, exists := r.providers[platform]
	if !exists {
		return nil, fmt.Errorf("%w: %s", domain.NewNotFoundError("platform provider not found", nil), platform)
	}

	return provider, nil
}

// RegisterProvider registers a platform provider
func (r *Registry) RegisterProvider(platform string, provider domain.PlatformProvider) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.providers[platform] = provider
}
