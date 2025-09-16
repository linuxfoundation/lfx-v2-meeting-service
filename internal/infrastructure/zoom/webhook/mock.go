// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package webhook

import (
	"log/slog"
)

// MockWebhookValidator is a mock implementation that always passes validation for testing
type MockWebhookValidator struct{}

// NewMockWebhookValidator creates a new mock webhook validator
func NewMockWebhookValidator() *MockWebhookValidator {
	return &MockWebhookValidator{}
}

// ValidateSignature always returns nil for mock mode
func (m *MockWebhookValidator) ValidateSignature(body []byte, signature, timestamp string) error {
	slog.Debug("Mock webhook validator - bypassing signature validation")
	return nil
}
