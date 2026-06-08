// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeLFXEnvironment(t *testing.T) {
	tests := []struct {
		raw  string
		want string
	}{
		{"production", "prod"},
		{"prod", "prod"},
		{"", "prod"},
		{"stage", "staging"},
		{"stg", "staging"},
		{"staging", "staging"},
		{"development", "dev"},
		{"dev", "dev"},
		{"unknown", "prod"},
	}
	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			assert.Equal(t, tt.want, normalizeLFXEnvironment(tt.raw))
		})
	}
}

func TestParseInviteConfig_DefaultURLUsesNormalizedEnvironment(t *testing.T) {
	t.Setenv("INVITES_ENABLED", "true")
	t.Setenv("LFX_SELF_SERVE_BASE_URL", "")

	got := parseInviteConfig("prod")
	assert.Equal(t, "https://app.lfx.dev", got.SelfServeBaseURL)
	assert.True(t, got.Enabled)
}

func TestParseInviteConfig_InvalidURLDisablesOutboundOnly(t *testing.T) {
	t.Setenv("INVITES_ENABLED", "true")
	t.Setenv("LFX_SELF_SERVE_BASE_URL", "not-a-valid-url")

	got := parseInviteConfig("prod")
	assert.True(t, got.Enabled, "invite_accepted subscriber should remain enabled")
	assert.Empty(t, got.SelfServeBaseURL, "outbound invites disabled via empty return URL")
}
