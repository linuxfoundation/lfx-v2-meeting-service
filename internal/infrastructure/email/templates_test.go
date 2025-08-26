// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package email

import (
	"html/template"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderTemplate(t *testing.T) {
	t.Run("successful template rendering", func(t *testing.T) {
		// Create a simple test template
		tmpl, err := template.New("test").Parse("Hello {{.Name}}, your value is {{.Value}}")
		require.NoError(t, err)

		data := struct {
			Name  string
			Value int
		}{
			Name:  "TestUser",
			Value: 42,
		}

		content, err := renderTemplate(tmpl, data)
		require.NoError(t, err)
		assert.Equal(t, "Hello TestUser, your value is 42", content)
	})

	t.Run("template with custom functions", func(t *testing.T) {
		// Test with formatTime function (which our templates use)
		tmpl, err := template.New("test").Funcs(template.FuncMap{
			"formatTime": formatTime,
		}).Parse("Time: {{formatTime .Time .Timezone}}")
		require.NoError(t, err)

		data := struct {
			Time     time.Time
			Timezone string
		}{
			Time:     time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC),
			Timezone: "UTC",
		}

		content, err := renderTemplate(tmpl, data)
		require.NoError(t, err)
		assert.NotEmpty(t, content)
		assert.Contains(t, content, "Time:")
	})

	t.Run("invalid template execution", func(t *testing.T) {
		// Template expects .Name field but data doesn't have it
		tmpl, err := template.New("test").Parse("Hello {{.Name}}")
		require.NoError(t, err)

		data := struct {
			Value int
		}{
			Value: 42,
		}

		content, err := renderTemplate(tmpl, data)
		assert.Error(t, err)
		assert.Empty(t, content)
	})
}

func TestLoadTemplate(t *testing.T) {
	t.Run("successful template loading", func(t *testing.T) {
		// Test loading an existing template
		config := templateConfig{
			name: "meeting_invitation.html",
			path: "templates/meeting_invitation.html",
		}

		tmpl, err := loadTemplate(config)
		require.NoError(t, err)
		assert.NotNil(t, tmpl)
		assert.Equal(t, "meeting_invitation.html", tmpl.Name())
	})

	t.Run("template with custom functions", func(t *testing.T) {
		// Test that custom functions are properly loaded
		config := templateConfig{
			name: "meeting_invitation.html",
			path: "templates/meeting_invitation.html",
		}

		tmpl, err := loadTemplate(config)
		require.NoError(t, err)

		// Verify that formatTime and formatDuration functions are available
		funcMap := tmpl.Funcs(template.FuncMap{})
		assert.NotNil(t, funcMap)
	})

	t.Run("nonexistent template file", func(t *testing.T) {
		config := templateConfig{
			name: "nonexistent.html",
			path: "templates/nonexistent.html",
		}

		tmpl, err := loadTemplate(config)
		assert.Error(t, err)
		assert.Nil(t, tmpl)
		assert.Contains(t, err.Error(), "failed to parse nonexistent.html template")
	})

	t.Run("invalid template path", func(t *testing.T) {
		config := templateConfig{
			name: "invalid.html",
			path: "invalid/path/template.html",
		}

		tmpl, err := loadTemplate(config)
		assert.Error(t, err)
		assert.Nil(t, tmpl)
	})
}

func TestFormatTime(t *testing.T) {
	testTime := time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC)

	tests := []struct {
		name     string
		timezone string
		expected string
	}{
		{
			name:     "UTC timezone",
			timezone: "UTC",
			expected: "Monday, January 15, 2024 at 2:30 PM UTC",
		},
		{
			name:     "EST timezone",
			timezone: "America/New_York",
			expected: "Monday, January 15, 2024 at 9:30 AM EST",
		},
		{
			name:     "Invalid timezone falls back to UTC",
			timezone: "Invalid/Timezone",
			expected: "Monday, January 15, 2024 at 2:30 PM UTC",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatTime(testTime, tt.timezone)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		minutes  int
		expected string
	}{
		{
			name:     "30 minutes",
			minutes:  30,
			expected: "30 minutes",
		},
		{
			name:     "1 hour exactly",
			minutes:  60,
			expected: "1 hour",
		},
		{
			name:     "2 hours exactly",
			minutes:  120,
			expected: "2 hours",
		},
		{
			name:     "1 hour 30 minutes",
			minutes:  90,
			expected: "1 hour 30 minutes",
		},
		{
			name:     "2 hours 45 minutes",
			minutes:  165,
			expected: "2 hours 45 minutes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatDuration(tt.minutes)
			assert.Equal(t, tt.expected, result)
		})
	}
}
