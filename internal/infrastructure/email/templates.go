// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package email

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"time"
)

//go:embed templates/*
var templateFS embed.FS

// TemplateSet holds HTML and text versions of a template
type TemplateSet struct {
	HTML *template.Template
	Text *template.Template
}

// MeetingTemplates holds all meeting-related templates
type MeetingTemplates struct {
	Invitation   TemplateSet
	Cancellation TemplateSet
}

// Templates holds all template categories
type Templates struct {
	Meeting MeetingTemplates
}

// templateConfig defines a template to be loaded
type templateConfig struct {
	name string
	path string
}

// loadTemplate loads a single template with the shared function map
func loadTemplate(config templateConfig) (*template.Template, error) {
	tmpl, err := template.New(config.name).Funcs(template.FuncMap{
		"formatTime":     formatTime,
		"formatDuration": formatDuration,
	}).ParseFS(templateFS, config.path)
	if err != nil {
		return nil, fmt.Errorf("failed to parse %s template: %w", config.name, err)
	}
	return tmpl, nil
}

// renderTemplate renders any template with the provided data
func renderTemplate(tmpl *template.Template, data any) (string, error) {
	var buf bytes.Buffer
	err := tmpl.Execute(&buf, data)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

// formatTime formats a time for display in emails
func formatTime(t time.Time, timezone string) string {
	// Load the timezone
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		// Fall back to UTC if timezone is invalid
		loc = time.UTC
	}

	// Convert time to the specified timezone
	localTime := t.In(loc)

	// Format: Monday, January 2, 2006 at 3:04 PM MST
	return localTime.Format("Monday, January 2, 2006 at 3:04 PM MST")
}

// formatDuration formats duration in minutes to a human-readable string
func formatDuration(minutes int) string {
	if minutes < 60 {
		if minutes == 1 {
			return "1 minute"
		}
		return fmt.Sprintf("%d minutes", minutes)
	}

	hours := minutes / 60
	remainingMinutes := minutes % 60

	if remainingMinutes == 0 {
		if hours == 1 {
			return "1 hour"
		}
		return fmt.Sprintf("%d hours", hours)
	}

	hourLabel := "hours"
	if hours == 1 {
		hourLabel = "hour"
	}
	minuteLabel := "minutes"
	if remainingMinutes == 1 {
		minuteLabel = "minute"
	}
	return fmt.Sprintf("%d %s %d %s", hours, hourLabel, remainingMinutes, minuteLabel)
}
