// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package webhook

import (
	"fmt"
	"sync"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
)

// Registry implements domain.WebhookRegistry
type Registry struct {
	handlers map[string]domain.WebhookHandler
	mu       sync.RWMutex
}

// NewRegistry creates a new webhook registry
func NewRegistry() *Registry {
	return &Registry{
		handlers: make(map[string]domain.WebhookHandler),
	}
}

// GetHandler returns the webhook handler for the specified platform
func (r *Registry) GetHandler(platform string) (domain.WebhookHandler, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	handler, exists := r.handlers[platform]
	if !exists {
		return nil, fmt.Errorf("webhook handler for platform %s not found", platform)
	}

	return handler, nil
}

// RegisterHandler registers a webhook handler for a platform
func (r *Registry) RegisterHandler(platform string, handler domain.WebhookHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.handlers[platform] = handler
}
