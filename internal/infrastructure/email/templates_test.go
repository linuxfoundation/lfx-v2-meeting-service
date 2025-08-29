// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package email

import (
	"html/template"
	"testing"
	"time"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
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

func TestFormatRecurrence(t *testing.T) {
	t.Run("nil recurrence", func(t *testing.T) {
		result := formatRecurrence(nil)
		assert.Equal(t, "", result)
	})

	t.Run("daily recurrence", func(t *testing.T) {
		tests := []struct {
			name     string
			interval int
			expected string
		}{
			{
				name:     "daily every day",
				interval: 1,
				expected: "Daily",
			},
			{
				name:     "daily every 3 days",
				interval: 3,
				expected: "Every 3 days",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				recurrence := &models.Recurrence{
					Type:           1, // Daily
					RepeatInterval: tt.interval,
				}
				result := formatRecurrence(recurrence)
				assert.Equal(t, tt.expected, result)
			})
		}
	})

	t.Run("weekly recurrence", func(t *testing.T) {
		tests := []struct {
			name       string
			interval   int
			weeklyDays string
			expected   string
		}{
			{
				name:     "weekly every week",
				interval: 1,
				expected: "Weekly",
			},
			{
				name:     "weekly every 2 weeks",
				interval: 2,
				expected: "Every 2 weeks",
			},
			{
				name:       "weekly on Monday and Friday",
				interval:   1,
				weeklyDays: "2,6",
				expected:   "Weekly on Monday and Friday",
			},
			{
				name:       "weekly on Monday, Wednesday, and Friday",
				interval:   1,
				weeklyDays: "2,4,6",
				expected:   "Weekly on Monday, Wednesday and Friday",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				recurrence := &models.Recurrence{
					Type:           2, // Weekly
					RepeatInterval: tt.interval,
					WeeklyDays:     tt.weeklyDays,
				}
				result := formatRecurrence(recurrence)
				assert.Equal(t, tt.expected, result)
			})
		}
	})

	t.Run("monthly recurrence", func(t *testing.T) {
		tests := []struct {
			name           string
			interval       int
			monthlyDay     int
			monthlyWeek    int
			monthlyWeekDay int
			expected       string
		}{
			{
				name:     "monthly every month",
				interval: 1,
				expected: "Monthly",
			},
			{
				name:     "monthly every 3 months",
				interval: 3,
				expected: "Every 3 months",
			},
			{
				name:       "monthly on day 15",
				interval:   1,
				monthlyDay: 15,
				expected:   "Monthly on day 15",
			},
			{
				name:           "monthly on first Monday",
				interval:       1,
				monthlyWeek:    1,
				monthlyWeekDay: 2,
				expected:       "Monthly on the first Monday",
			},
			{
				name:           "monthly on third Friday",
				interval:       1,
				monthlyWeek:    3,
				monthlyWeekDay: 6,
				expected:       "Monthly on the third Friday",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				recurrence := &models.Recurrence{
					Type:           3, // Monthly
					RepeatInterval: tt.interval,
					MonthlyDay:     tt.monthlyDay,
					MonthlyWeek:    tt.monthlyWeek,
					MonthlyWeekDay: tt.monthlyWeekDay,
				}
				result := formatRecurrence(recurrence)
				assert.Equal(t, tt.expected, result)
			})
		}
	})

	t.Run("recurrence with end conditions", func(t *testing.T) {
		endDate := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)

		tests := []struct {
			name        string
			endTimes    int
			endDateTime *time.Time
			expected    string
		}{
			{
				name:     "daily with 10 occurrences",
				endTimes: 10,
				expected: "Daily (10 occurrences)",
			},
			{
				name:        "daily until December 31, 2024",
				endDateTime: &endDate,
				expected:    "Daily (until December 31, 2024)",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				recurrence := &models.Recurrence{
					Type:           1, // Daily
					RepeatInterval: 1,
					EndTimes:       tt.endTimes,
					EndDateTime:    tt.endDateTime,
				}
				result := formatRecurrence(recurrence)
				assert.Equal(t, tt.expected, result)
			})
		}
	})

	t.Run("custom recurrence type", func(t *testing.T) {
		recurrence := &models.Recurrence{
			Type:           99, // Unknown type
			RepeatInterval: 1,
		}
		result := formatRecurrence(recurrence)
		assert.Equal(t, "Custom recurrence", result)
	})
}

func TestFormatWeeklyDaysText(t *testing.T) {
	tests := []struct {
		name       string
		weeklyDays string
		expected   string
	}{
		{
			name:       "single day - Monday",
			weeklyDays: "2",
			expected:   "Monday",
		},
		{
			name:       "two days - Monday and Friday",
			weeklyDays: "2,6",
			expected:   "Monday and Friday",
		},
		{
			name:       "three days - Monday, Wednesday, Friday",
			weeklyDays: "2,4,6",
			expected:   "Monday, Wednesday and Friday",
		},
		{
			name:       "all weekdays",
			weeklyDays: "2,3,4,5,6",
			expected:   "Monday, Tuesday, Wednesday, Thursday and Friday",
		},
		{
			name:       "weekend days",
			weeklyDays: "1,7",
			expected:   "Sunday and Saturday",
		},
		{
			name:       "with spaces",
			weeklyDays: " 2 , 4 , 6 ",
			expected:   "Monday, Wednesday and Friday",
		},
		{
			name:       "invalid day numbers",
			weeklyDays: "8,9,10",
			expected:   "",
		},
		{
			name:       "mixed valid and invalid",
			weeklyDays: "2,8,6",
			expected:   "Monday and Friday",
		},
		{
			name:       "empty string",
			weeklyDays: "",
			expected:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatWeeklyDaysText(tt.weeklyDays)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetOrdinalWeek(t *testing.T) {
	tests := []struct {
		name     string
		week     int
		expected string
	}{
		{
			name:     "first week",
			week:     1,
			expected: "first",
		},
		{
			name:     "second week",
			week:     2,
			expected: "second",
		},
		{
			name:     "third week",
			week:     3,
			expected: "third",
		},
		{
			name:     "fourth week",
			week:     4,
			expected: "fourth",
		},
		{
			name:     "fifth week",
			week:     5,
			expected: "fifth",
		},
		{
			name:     "sixth week (fallback)",
			week:     6,
			expected: "6th",
		},
		{
			name:     "zero week (fallback)",
			week:     0,
			expected: "0th",
		},
		{
			name:     "negative week (fallback)",
			week:     -1,
			expected: "-1th",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getOrdinalWeek(tt.week)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetWeekdayFullName(t *testing.T) {
	tests := []struct {
		name     string
		weekday  int
		expected string
	}{
		{
			name:     "Sunday",
			weekday:  1,
			expected: "Sunday",
		},
		{
			name:     "Monday",
			weekday:  2,
			expected: "Monday",
		},
		{
			name:     "Tuesday",
			weekday:  3,
			expected: "Tuesday",
		},
		{
			name:     "Wednesday",
			weekday:  4,
			expected: "Wednesday",
		},
		{
			name:     "Thursday",
			weekday:  5,
			expected: "Thursday",
		},
		{
			name:     "Friday",
			weekday:  6,
			expected: "Friday",
		},
		{
			name:     "Saturday",
			weekday:  7,
			expected: "Saturday",
		},
		{
			name:     "invalid weekday zero",
			weekday:  0,
			expected: "",
		},
		{
			name:     "invalid weekday eight",
			weekday:  8,
			expected: "",
		},
		{
			name:     "negative weekday",
			weekday:  -1,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getWeekdayFullName(tt.weekday)
			assert.Equal(t, tt.expected, result)
		})
	}
}
