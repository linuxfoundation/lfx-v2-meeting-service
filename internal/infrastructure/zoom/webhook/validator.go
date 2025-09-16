// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
)

// ZoomWebhookValidator handles validation of Zoom webhook signatures
type ZoomWebhookValidator struct {
	SecretToken string
}

// NewZoomWebhookValidator creates a new Zoom webhook validator
func NewZoomWebhookValidator(secretToken string) *ZoomWebhookValidator {
	return &ZoomWebhookValidator{
		SecretToken: secretToken,
	}
}

// ValidateSignature validates the Zoom webhook signature
func (v *ZoomWebhookValidator) ValidateSignature(body []byte, signature, timestamp string) error {
	if v.SecretToken == "" {
		return fmt.Errorf("webhook secret token not configured")
	}

	if signature == "" {
		return fmt.Errorf("missing webhook signature")
	}

	if timestamp == "" {
		return fmt.Errorf("missing webhook timestamp")
	}

	// Create the message to sign: v0=timestamp:body
	message := fmt.Sprintf("v0:%s:%s", timestamp, body)

	// Calculate HMAC-SHA256
	h := hmac.New(sha256.New, []byte(v.SecretToken))
	h.Write([]byte(message))
	expectedSignature := "v0=" + hex.EncodeToString(h.Sum(nil))

	// Compare signatures using constant-time comparison
	if signature != expectedSignature {
		slog.Error("zoom webhook signature does not match expected signature")
		return fmt.Errorf("zoom webhook signature does not match expected signature")
	}

	return nil
}

// IsValidEvent checks if the event type is supported
func (v *ZoomWebhookValidator) IsValidEvent(eventType string) bool {
	validEvents := map[string]bool{
		"meeting.started":                true,
		"meeting.ended":                  true,
		"meeting.deleted":                true,
		"meeting.participant_joined":     true,
		"meeting.participant_left":       true,
		"recording.completed":            true,
		"recording.transcript_completed": true,
		"meeting.summary_completed":      true,
		"endpoint.url_validation":        true,
	}

	return validEvents[eventType]
}
