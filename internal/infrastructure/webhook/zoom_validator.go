// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ZoomWebhookValidator handles validation of Zoom webhook signatures
type ZoomWebhookValidator struct {
	secretToken string
}

// NewZoomWebhookValidator creates a new Zoom webhook validator
func NewZoomWebhookValidator(secretToken string) *ZoomWebhookValidator {
	return &ZoomWebhookValidator{
		secretToken: secretToken,
	}
}

// ValidateSignature validates the Zoom webhook signature
func (v *ZoomWebhookValidator) ValidateSignature(body []byte, signature, timestamp string) error {
	if v.secretToken == "" {
		return fmt.Errorf("webhook secret token not configured")
	}

	if signature == "" {
		return fmt.Errorf("missing webhook signature")
	}

	if timestamp == "" {
		return fmt.Errorf("missing webhook timestamp")
	}

	// Parse timestamp for replay protection
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid timestamp format: %w", err)
	}

	// Check if request is too old (5 minutes tolerance)
	now := time.Now().Unix()
	if now-ts > 300 {
		return fmt.Errorf("request timestamp too old")
	}

	// Create the message to sign: v0=timestamp:body
	message := fmt.Sprintf("v0:%s:%s", timestamp, string(body))

	// Calculate HMAC-SHA256
	h := hmac.New(sha256.New, []byte(v.secretToken))
	h.Write([]byte(message))
	expectedSignature := "v0=" + hex.EncodeToString(h.Sum(nil))

	// Extract signature from header (remove "v0=" prefix if present)
	providedSignature := strings.TrimPrefix(signature, "v0=")
	expectedSignatureNoPrefix := strings.TrimPrefix(expectedSignature, "v0=")

	// Compare signatures using constant-time comparison
	if !hmac.Equal([]byte(providedSignature), []byte(expectedSignatureNoPrefix)) {
		return fmt.Errorf("invalid webhook signature")
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
	}

	return validEvents[eventType]
}
