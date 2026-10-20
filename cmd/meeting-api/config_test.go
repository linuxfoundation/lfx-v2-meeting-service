// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package main

import (
	"os"
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

func TestParseUserServiceConfig_DefaultBaseURLPerEnvironment(t *testing.T) {
	tests := []struct {
		env  string
		want string
	}{
		{"prod", "https://api-gw.platform.linuxfoundation.org"},
		{"staging", "https://api-gw.staging.platform.linuxfoundation.org"},
		{"dev", "https://api-gw.dev.platform.linuxfoundation.org"},
	}
	for _, tt := range tests {
		t.Run(tt.env, func(t *testing.T) {
			t.Setenv("USER_SERVICE_BASE_URL", "")
			got := parseUserServiceConfig(tt.env)
			assert.Equal(t, tt.want, got.BaseURL)
		})
	}
}

func TestParseUserServiceConfig_EnvOverridesDefault(t *testing.T) {
	t.Setenv("USER_SERVICE_BASE_URL", "https://api-gw.custom.example.org")
	got := parseUserServiceConfig("prod")
	assert.Equal(t, "https://api-gw.custom.example.org", got.BaseURL)
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

// TestParseNATSRespondersEnabled_SameRuleAsEventProcessingEnabled pins the
// switch's semantics to EVENT_PROCESSING_ENABLED's: only the exact value
// "false" disables; unset, "FALSE" and "0" keep the default of true.
func TestParseNATSRespondersEnabled_SameRuleAsEventProcessingEnabled(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value *string
		want  bool
	}{
		{name: "unset defaults to true", value: nil, want: true},
		{name: `"false" disables`, value: strPtr("false"), want: false},
		{name: `"FALSE" is not the switch`, value: strPtr("FALSE"), want: true},
		{name: `"0" is not the switch`, value: strPtr("0"), want: true},
		{name: `"true" enables`, value: strPtr("true"), want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.value == nil {
				t.Setenv("NATS_RESPONDERS_ENABLED", "")
				_ = os.Unsetenv("NATS_RESPONDERS_ENABLED")
			} else {
				t.Setenv("NATS_RESPONDERS_ENABLED", *tc.value)
			}
			assert.Equal(t, tc.want, parseNATSRespondersEnabled())
			// The same input gives the same answer for the switch this one mirrors.
			if tc.value == nil {
				t.Setenv("EVENT_PROCESSING_ENABLED", "")
				_ = os.Unsetenv("EVENT_PROCESSING_ENABLED")
			} else {
				t.Setenv("EVENT_PROCESSING_ENABLED", *tc.value)
			}
			assert.Equal(t, tc.want, parseEventConfig().Enabled)
		})
	}
}

func strPtr(s string) *string { return &s }
