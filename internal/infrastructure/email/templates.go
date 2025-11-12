// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package email

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"strings"
	"time"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
)

//go:embed templates/*
var templateFS embed.FS

// RenderedEmail holds both HTML and text versions of a rendered email
type RenderedEmail struct {
	HTML string
	Text string
}

// MeetingTemplateManager defines the interface for rendering meeting email templates
type MeetingTemplateManager interface {
	RenderInvitation(data domain.EmailInvitation) (*RenderedEmail, error)
	RenderCancellation(data domain.EmailCancellation) (*RenderedEmail, error)
	RenderOccurrenceCancellation(data domain.EmailOccurrenceCancellation) (*RenderedEmail, error)
	RenderUpdatedInvitation(data domain.EmailUpdatedInvitation) (*RenderedEmail, error)
	RenderSummaryNotification(data domain.EmailSummaryNotification) (*RenderedEmail, error)
}

// TemplateManager is the default implementation of MeetingTemplateManager
type TemplateManager struct {
	templates Templates
}

// NewTemplateManager creates a new template manager with all templates loaded
func NewTemplateManager() (*TemplateManager, error) {
	tm := &TemplateManager{}

	// Define all templates to load
	templateConfigs := map[string]templateConfig{
		"invitationHTML":             {"meeting_invitation.html", "templates/meeting_invitation.html"},
		"invitationText":             {"meeting_invitation.txt", "templates/meeting_invitation.txt"},
		"cancellationHTML":           {"meeting_invitation_cancellation.html", "templates/meeting_invitation_cancellation.html"},
		"cancellationText":           {"meeting_invitation_cancellation.txt", "templates/meeting_invitation_cancellation.txt"},
		"occurrenceCancellationHTML": {"meeting_occurrence_cancellation.html", "templates/meeting_occurrence_cancellation.html"},
		"occurrenceCancellationText": {"meeting_occurrence_cancellation.txt", "templates/meeting_occurrence_cancellation.txt"},
		"updatedInvitationHTML":      {"meeting_updated_invitation.html", "templates/meeting_updated_invitation.html"},
		"updatedInvitationText":      {"meeting_updated_invitation.txt", "templates/meeting_updated_invitation.txt"},
		"summaryNotificationHTML":    {"meeting_summary_notification.html", "templates/meeting_summary_notification.html"},
		"summaryNotificationText":    {"meeting_summary_notification.txt", "templates/meeting_summary_notification.txt"},
	}

	// Load all templates
	loadedTemplates := make(map[string]*template.Template)
	for key, cfg := range templateConfigs {
		tmpl, err := loadTemplate(cfg)
		if err != nil {
			return nil, err
		}
		loadedTemplates[key] = tmpl
	}

	// Organize templates into the structure
	tm.templates = Templates{
		Meeting: MeetingTemplates{
			Invitation: TemplateSet{
				HTML: loadedTemplates["invitationHTML"],
				Text: loadedTemplates["invitationText"],
			},
			Cancellation: TemplateSet{
				HTML: loadedTemplates["cancellationHTML"],
				Text: loadedTemplates["cancellationText"],
			},
			OccurrenceCancellation: TemplateSet{
				HTML: loadedTemplates["occurrenceCancellationHTML"],
				Text: loadedTemplates["occurrenceCancellationText"],
			},
			UpdatedInvitation: TemplateSet{
				HTML: loadedTemplates["updatedInvitationHTML"],
				Text: loadedTemplates["updatedInvitationText"],
			},
			SummaryNotification: TemplateSet{
				HTML: loadedTemplates["summaryNotificationHTML"],
				Text: loadedTemplates["summaryNotificationText"],
			},
		},
	}

	return tm, nil
}

// Ensure TemplateManager implements MeetingTemplateManager
var _ MeetingTemplateManager = (*TemplateManager)(nil)

// RenderInvitation renders an invitation email with both HTML and text versions
func (tm *TemplateManager) RenderInvitation(data domain.EmailInvitation) (*RenderedEmail, error) {
	html, err := renderTemplate(tm.templates.Meeting.Invitation.HTML, data)
	if err != nil {
		return nil, fmt.Errorf("failed to render invitation HTML: %w", err)
	}

	text, err := renderTemplate(tm.templates.Meeting.Invitation.Text, data)
	if err != nil {
		return nil, fmt.Errorf("failed to render invitation text: %w", err)
	}

	return &RenderedEmail{HTML: html, Text: text}, nil
}

// RenderCancellation renders a cancellation email with both HTML and text versions
func (tm *TemplateManager) RenderCancellation(data domain.EmailCancellation) (*RenderedEmail, error) {
	html, err := renderTemplate(tm.templates.Meeting.Cancellation.HTML, data)
	if err != nil {
		return nil, fmt.Errorf("failed to render cancellation HTML: %w", err)
	}

	text, err := renderTemplate(tm.templates.Meeting.Cancellation.Text, data)
	if err != nil {
		return nil, fmt.Errorf("failed to render cancellation text: %w", err)
	}

	return &RenderedEmail{HTML: html, Text: text}, nil
}

// RenderOccurrenceCancellation renders an occurrence cancellation email with both HTML and text versions
func (tm *TemplateManager) RenderOccurrenceCancellation(data domain.EmailOccurrenceCancellation) (*RenderedEmail, error) {
	html, err := renderTemplate(tm.templates.Meeting.OccurrenceCancellation.HTML, data)
	if err != nil {
		return nil, fmt.Errorf("failed to render occurrence cancellation HTML: %w", err)
	}

	text, err := renderTemplate(tm.templates.Meeting.OccurrenceCancellation.Text, data)
	if err != nil {
		return nil, fmt.Errorf("failed to render occurrence cancellation text: %w", err)
	}

	return &RenderedEmail{HTML: html, Text: text}, nil
}

// RenderUpdatedInvitation renders an updated invitation email with both HTML and text versions
func (tm *TemplateManager) RenderUpdatedInvitation(data domain.EmailUpdatedInvitation) (*RenderedEmail, error) {
	html, err := renderTemplate(tm.templates.Meeting.UpdatedInvitation.HTML, data)
	if err != nil {
		return nil, fmt.Errorf("failed to render updated invitation HTML: %w", err)
	}

	text, err := renderTemplate(tm.templates.Meeting.UpdatedInvitation.Text, data)
	if err != nil {
		return nil, fmt.Errorf("failed to render updated invitation text: %w", err)
	}

	return &RenderedEmail{HTML: html, Text: text}, nil
}

// RenderSummaryNotification renders a summary notification email with both HTML and text versions
func (tm *TemplateManager) RenderSummaryNotification(data domain.EmailSummaryNotification) (*RenderedEmail, error) {
	html, err := renderTemplate(tm.templates.Meeting.SummaryNotification.HTML, data)
	if err != nil {
		return nil, fmt.Errorf("failed to render summary notification HTML: %w", err)
	}

	text, err := renderTemplate(tm.templates.Meeting.SummaryNotification.Text, data)
	if err != nil {
		return nil, fmt.Errorf("failed to render summary notification text: %w", err)
	}

	return &RenderedEmail{HTML: html, Text: text}, nil
}

// TemplateSet holds HTML and text versions of a template
type TemplateSet struct {
	HTML *template.Template
	Text *template.Template
}

// MeetingTemplates holds all meeting-related templates
type MeetingTemplates struct {
	Invitation             TemplateSet
	Cancellation           TemplateSet
	OccurrenceCancellation TemplateSet
	UpdatedInvitation      TemplateSet
	SummaryNotification    TemplateSet
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

// LoadTemplate loads a single template with the shared function map
func (tm *TemplateManager) LoadTemplate(config templateConfig) (*template.Template, error) {
	return loadTemplate(config)
}

// RenderTemplate renders any template with the provided data
func (tm *TemplateManager) RenderTemplate(tmpl *template.Template, data any) (string, error) {
	return renderTemplate(tmpl, data)
}

// loadTemplate loads a single template with the shared function map
func loadTemplate(config templateConfig) (*template.Template, error) {
	tmpl, err := template.New(config.name).Funcs(template.FuncMap{
		"formatTime":         formatTime,
		"formatDuration":     formatDuration,
		"formatRecurrence":   formatRecurrence,
		"capitalize":         capitalize,
		"newLineToBreakLine": newLineToBreakLine,
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

	// Format with ordinal day suffix
	day := localTime.Day()
	var suffix string
	switch {
	case day >= 11 && day <= 13:
		suffix = "th"
	case day%10 == 1:
		suffix = "st"
	case day%10 == 2:
		suffix = "nd"
	case day%10 == 3:
		suffix = "rd"
	default:
		suffix = "th"
	}

	// Format: Wednesday, September 15th, 10:30 Africa/Johannesburg
	return fmt.Sprintf("%s, %s %d%s, %s %s",
		localTime.Format("Monday"),
		localTime.Format("January"),
		day,
		suffix,
		localTime.Format("15:04"),
		timezone)
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

// formatRecurrence formats the recurrence pattern for display
func formatRecurrence(recurrence *models.Recurrence, t time.Time, timezone string) string {
	if recurrence == nil {
		return ""
	}

	var pattern strings.Builder

	switch recurrence.Type {
	case 1: // Daily
		if recurrence.RepeatInterval == 1 {
			pattern.WriteString("Daily")
		} else {
			pattern.WriteString(fmt.Sprintf("Every %d days", recurrence.RepeatInterval))
		}
	case 2: // Weekly
		if recurrence.RepeatInterval == 1 {
			pattern.WriteString("Weekly")
		} else {
			pattern.WriteString(fmt.Sprintf("Every %d weeks", recurrence.RepeatInterval))
		}
		if recurrence.WeeklyDays != "" {
			days := formatWeeklyDaysText(recurrence.WeeklyDays)
			if days != "" {
				pattern.WriteString(" on ")
				pattern.WriteString(days)
			}
		}
	case 3: // Monthly
		if recurrence.RepeatInterval == 1 {
			pattern.WriteString("Monthly")
		} else {
			pattern.WriteString(fmt.Sprintf("Every %d months", recurrence.RepeatInterval))
		}
		if recurrence.MonthlyDay > 0 {
			pattern.WriteString(fmt.Sprintf(" on day %d", recurrence.MonthlyDay))
		} else if recurrence.MonthlyWeek > 0 && recurrence.MonthlyWeekDay > 0 {
			weekName := getOrdinalWeek(recurrence.MonthlyWeek)
			dayName := getWeekdayFullName(recurrence.MonthlyWeekDay)
			pattern.WriteString(fmt.Sprintf(" on the %s %s", weekName, dayName))
		}
	default:
		return "Custom recurrence"
	}

	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return pattern.String()
	}
	localTime := t.In(loc)
	pattern.WriteString(fmt.Sprintf(" at %s %s", localTime.Format("15:04"), timezone))

	// Add end condition
	if recurrence.EndTimes > 0 {
		pattern.WriteString(fmt.Sprintf(" (%d occurrences)", recurrence.EndTimes))
	} else if recurrence.EndDateTime != nil {
		endDate := recurrence.EndDateTime.Format("January 2, 2006")
		pattern.WriteString(fmt.Sprintf(" (until %s)", endDate))
	}

	return pattern.String()
}

// formatWeeklyDaysText converts numeric weekdays to readable text
func formatWeeklyDaysText(weeklyDays string) string {
	dayNames := map[string]string{
		"1": "Sunday",
		"2": "Monday",
		"3": "Tuesday",
		"4": "Wednesday",
		"5": "Thursday",
		"6": "Friday",
		"7": "Saturday",
	}

	days := strings.Split(weeklyDays, ",")
	var dayTexts []string
	for _, day := range days {
		day = strings.TrimSpace(day)
		if name, ok := dayNames[day]; ok {
			dayTexts = append(dayTexts, name)
		}
	}

	if len(dayTexts) == 0 {
		return ""
	}
	if len(dayTexts) == 1 {
		return dayTexts[0]
	}
	if len(dayTexts) == 2 {
		return dayTexts[0] + " and " + dayTexts[1]
	}

	// Join with commas and "and" for the last item
	return strings.Join(dayTexts[:len(dayTexts)-1], ", ") + " and " + dayTexts[len(dayTexts)-1]
}

// getOrdinalWeek converts week number to ordinal text
func getOrdinalWeek(week int) string {
	switch week {
	case 1:
		return "first"
	case 2:
		return "second"
	case 3:
		return "third"
	case 4:
		return "fourth"
	case 5:
		return "fifth"
	default:
		return fmt.Sprintf("%dth", week)
	}
}

// getWeekdayFullName converts numeric weekday to full name
func getWeekdayFullName(weekday int) string {
	weekdays := []string{"", "Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}
	if weekday >= 1 && weekday < len(weekdays) {
		return weekdays[weekday]
	}
	return ""
}

// capitalize capitalizes the first letter of a string
func capitalize(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + strings.ToLower(s[1:])
}

// newLineToBreakLine converts newlines to HTML break tags for proper email formatting
func newLineToBreakLine(s string) template.HTML {
	// Replace newlines with <br> tags
	escaped := template.HTMLEscapeString(s)
	replaced := strings.ReplaceAll(escaped, "\n", "<br>")
	// Return as template.HTML to prevent double escaping
	return template.HTML(replaced)
}
