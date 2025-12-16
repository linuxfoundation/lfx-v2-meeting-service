// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package email

import (
	"strings"
	"testing"
	"time"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestICSGenerator_GenerateMeetingICS(t *testing.T) {
	generator := NewICSGenerator()

	// Test time
	startTime := time.Date(2024, 1, 15, 14, 0, 0, 0, time.UTC)

	t.Run("basic meeting without recurrence", func(t *testing.T) {
		ics, err := generator.GenerateMeetingInvitationICS(ICSMeetingInvitationParams{
			MeetingUID:       "meeting-123",
			MeetingTitle:     "Team Standup",
			Description:      "Weekly team sync meeting",
			StartTime:        startTime,
			DurationMinutes:  60,
			Timezone:         "America/New_York",
			PlatformJoinLink: "https://zoom.us/j/123456789",
			MeetingID:        "123456789",
			Passcode:         "abc123",
			RecipientEmail:   "user@example.com",
			ProjectName:      "Test Project",
			Recurrence:       nil,
			Sequence:         0,
		})

		require.NoError(t, err)
		assert.Contains(t, ics, "BEGIN:VCALENDAR")
		assert.Contains(t, ics, "END:VCALENDAR")
		assert.Contains(t, ics, "BEGIN:VEVENT")
		assert.Contains(t, ics, "END:VEVENT")
		assert.Contains(t, ics, "UID:meeting-123")
		assert.Contains(t, ics, "SUMMARY:Team Standup")
		assert.Contains(t, ics, "ORGANIZER;CN=ITX:mailto:itx@linuxfoundation.org")
		assert.Contains(t, ics, "LOCATION:https://zoom.us/j/123456789")
		assert.Contains(t, ics, "Meeting ID: 123456789")
		assert.Contains(t, ics, "Passcode: abc123")
		assert.Contains(t, ics, "ATTENDEE")
		assert.Contains(t, ics, "user@example.com")
		assert.Contains(t, ics, "BEGIN:VALARM")
		assert.Contains(t, ics, "TRIGGER:-PT10M")
	})

	t.Run("meeting with daily recurrence", func(t *testing.T) {
		recurrence := &models.Recurrence{
			Type:           1, // Daily
			RepeatInterval: 1,
			EndTimes:       10,
		}

		ics, err := generator.GenerateMeetingInvitationICS(ICSMeetingInvitationParams{
			MeetingUID:       "daily-meeting-456",
			MeetingTitle:     "Daily Standup",
			Description:      "Daily team sync",
			StartTime:        startTime,
			DurationMinutes:  30,
			Timezone:         "UTC",
			PlatformJoinLink: "https://zoom.us/j/987654321",
			MeetingID:        "987654321",
			Passcode:         "xyz789",
			RecipientEmail:   "team@example.com",
			ProjectName:      "",
			Recurrence:       recurrence,
			Sequence:         0,
		})

		require.NoError(t, err)
		assert.Contains(t, ics, "RRULE:FREQ=DAILY;COUNT=10")
		assert.Contains(t, ics, "SUMMARY:Daily Standup")
	})

	t.Run("meeting with weekly recurrence", func(t *testing.T) {
		recurrence := &models.Recurrence{
			Type:           2, // Weekly
			RepeatInterval: 2,
			WeeklyDays:     "2,4,6", // Monday, Wednesday, Friday
			EndTimes:       20,
		}

		ics, err := generator.GenerateMeetingInvitationICS(ICSMeetingInvitationParams{
			MeetingUID:       "biweekly-meeting-789",
			MeetingTitle:     "Bi-weekly Meeting",
			Description:      "Bi-weekly team meeting",
			StartTime:        startTime,
			DurationMinutes:  45,
			Timezone:         "Europe/London",
			PlatformJoinLink: "https://zoom.us/j/555555555",
			MeetingID:        "555555555",
			Passcode:         "",
			RecipientEmail:   "group@example.com",
			ProjectName:      "",
			Recurrence:       recurrence,
			Sequence:         0,
		})

		require.NoError(t, err)
		assert.Contains(t, ics, "RRULE:FREQ=WEEKLY;INTERVAL=2;BYDAY=MO,WE,FR;COUNT=20")
	})

	t.Run("meeting with monthly recurrence by day", func(t *testing.T) {
		recurrence := &models.Recurrence{
			Type:           3, // Monthly
			RepeatInterval: 1,
			MonthlyDay:     15,
			EndTimes:       12,
		}

		ics, err := generator.GenerateMeetingInvitationICS(ICSMeetingInvitationParams{
			MeetingUID:       "monthly-review-101",
			MeetingTitle:     "Monthly Review",
			Description:      "Monthly project review",
			StartTime:        startTime,
			DurationMinutes:  90,
			Timezone:         "Asia/Tokyo",
			PlatformJoinLink: "",
			MeetingID:        "",
			Passcode:         "",
			RecipientEmail:   "manager@example.com",
			ProjectName:      "",
			Recurrence:       recurrence,
			Sequence:         0,
		})

		require.NoError(t, err)
		assert.Contains(t, ics, "RRULE:FREQ=MONTHLY;BYMONTHDAY=15;COUNT=12")
	})

	t.Run("meeting with monthly recurrence by week", func(t *testing.T) {
		endDate := startTime.Add(365 * 24 * time.Hour)
		recurrence := &models.Recurrence{
			Type:           3, // Monthly
			RepeatInterval: 1,
			MonthlyWeek:    2, // Second week
			MonthlyWeekDay: 3, // Tuesday
			EndDateTime:    &endDate,
		}

		ics, err := generator.GenerateMeetingInvitationICS(ICSMeetingInvitationParams{
			MeetingUID:       "board-meeting-202",
			MeetingTitle:     "Monthly Board Meeting",
			Description:      "Board meeting",
			StartTime:        startTime,
			DurationMinutes:  120,
			Timezone:         "America/Los_Angeles",
			PlatformJoinLink: "https://zoom.us/j/111111111",
			MeetingID:        "111111111",
			Passcode:         "secure",
			RecipientEmail:   "board@example.com",
			ProjectName:      "",
			Recurrence:       recurrence,
			Sequence:         0,
		})

		require.NoError(t, err)
		assert.Contains(t, ics, "RRULE:FREQ=MONTHLY;BYDAY=2TU;UNTIL=")
	})

	t.Run("meeting without join link", func(t *testing.T) {
		ics, err := generator.GenerateMeetingInvitationICS(ICSMeetingInvitationParams{
			MeetingUID:       "inperson-meeting-303",
			MeetingTitle:     "In-Person Meeting",
			Description:      "Meet at the office",
			StartTime:        startTime,
			DurationMinutes:  60,
			Timezone:         "America/Chicago",
			PlatformJoinLink: "",
			MeetingID:        "",
			Passcode:         "",
			RecipientEmail:   "office@example.com",
			ProjectName:      "",
			Recurrence:       nil,
			Sequence:         0,
		})

		require.NoError(t, err)
		// Check that LOCATION field is not present (but X-LIC-LOCATION in VTIMEZONE is ok)
		assert.NotContains(t, ics, "\nLOCATION:")
		assert.NotContains(t, ics, "\nURL:")
		assert.NotContains(t, ics, "Meeting ID:")
		assert.NotContains(t, ics, "Passcode:")
	})
}

func TestConvertWeeklyDays(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "single day",
			input:    "2",
			expected: "MO",
		},
		{
			name:     "multiple days",
			input:    "1,3,5",
			expected: "SU,TU,TH",
		},
		{
			name:     "all weekdays",
			input:    "2,3,4,5,6",
			expected: "MO,TU,WE,TH,FR",
		},
		{
			name:     "weekend",
			input:    "1,7",
			expected: "SU,SA",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "invalid day",
			input:    "8,9",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertWeeklyDays(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetWeekdayName(t *testing.T) {
	tests := []struct {
		weekday  int
		expected string
	}{
		{1, "SU"},
		{2, "MO"},
		{3, "TU"},
		{4, "WE"},
		{5, "TH"},
		{6, "FR"},
		{7, "SA"},
		{0, ""},
		{8, ""},
	}

	for _, tt := range tests {
		t.Run(string(rune(tt.weekday)), func(t *testing.T) {
			result := getWeekdayName(tt.weekday)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEscapeICSText(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple text",
			input:    "Hello World",
			expected: "Hello World",
		},
		{
			name:     "text with comma",
			input:    "Hello, World",
			expected: `Hello\, World`,
		},
		{
			name:     "text with semicolon",
			input:    "Title: Meeting; Description: Test",
			expected: `Title: Meeting\; Description: Test`,
		},
		{
			name:     "text with newline",
			input:    "Line 1\nLine 2",
			expected: `Line 1\nLine 2`,
		},
		{
			name:     "text with backslash",
			input:    `Path\to\file`,
			expected: `Path\\to\\file`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := escapeICSText(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFoldICSLine(t *testing.T) {
	t.Run("short line", func(t *testing.T) {
		input := "Short line"
		result := foldICSLine(input, 75)
		assert.Equal(t, input, result)
	})

	t.Run("long line", func(t *testing.T) {
		input := strings.Repeat("a", 100)
		result := foldICSLine(input, 75)

		// Check that the line was folded
		assert.Contains(t, result, "\r\n ")

		// Check that no individual line exceeds the limit
		lines := strings.Split(result, "\r\n")
		for i, line := range lines {
			if i > 0 {
				// Continued lines should start with a space
				assert.True(t, strings.HasPrefix(line, " "))
				// Remove the leading space for length check
				line = strings.TrimPrefix(line, " ")
			}
			assert.LessOrEqual(t, len(line), 75)
		}
	})
}

func TestBuildDescription(t *testing.T) {
	t.Run("with all details", func(t *testing.T) {
		desc := buildDescription(DescriptionParams{
			MeetingDescription: "Original description",
			MeetingID:          "123456789",
			MeetingPasscode:    "abc123",
			PlatformJoinLink:   "https://zoom.us/j/123456789",
			ProjectName:        "Test Project",
			MeetingAttachments: nil,
		})

		assert.Contains(t, desc, "Test Project Meeting")
		assert.Contains(t, desc, "Original description")
		assert.Contains(t, desc, "Meeting ID: 123456789")
		assert.Contains(t, desc, "Passcode: abc123")
		assert.Contains(t, desc, "Join Meeting: https://zoom.us/j/123456789")
		assert.Contains(t, desc, "find your local number")
	})

	t.Run("without passcode", func(t *testing.T) {
		desc := buildDescription(DescriptionParams{
			MeetingDescription: "",
			MeetingID:          "987654321",
			MeetingPasscode:    "",
			PlatformJoinLink:   "https://zoom.us/j/987654321",
			ProjectName:        "",
			MeetingAttachments: nil,
		})

		assert.Contains(t, desc, "Meeting ID: 987654321")
		assert.NotContains(t, desc, "Passcode:")
		assert.Contains(t, desc, "enter Meeting ID: 987654321#")
	})

	t.Run("without meeting details", func(t *testing.T) {
		desc := buildDescription(DescriptionParams{
			MeetingDescription: "Simple meeting",
			MeetingID:          "",
			MeetingPasscode:    "",
			PlatformJoinLink:   "",
			ProjectName:        "",
			MeetingAttachments: nil,
		})

		assert.Contains(t, desc, "Simple meeting")
		assert.NotContains(t, desc, "Meeting ID:")
		assert.NotContains(t, desc, "Passcode:")
		assert.NotContains(t, desc, "Join Meeting:")
		assert.NotContains(t, desc, "find your local number")
	})

	t.Run("with attachments", func(t *testing.T) {
		attachments := []*models.MeetingAttachment{
			{
				Type:        "link",
				Name:        "Agenda",
				Link:        "https://example.com/agenda",
				Description: "Meeting agenda",
			},
			{
				Type:        "file",
				Name:        "Presentation",
				FileName:    "slides.pdf",
				Description: "Slide deck",
			},
			{
				Type: "link",
				Link: "https://example.com/notes",
				Name: "",
			},
			{
				Type:     "file",
				FileName: "document.docx",
				Name:     "",
			},
		}

		desc := buildDescription(DescriptionParams{
			MeetingDescription: "Meeting with attachments",
			MeetingID:          "123456789",
			MeetingPasscode:    "abc123",
			PlatformJoinLink:   "https://zoom.us/j/123456789",
			ProjectName:        "Test Project",
			MeetingAttachments: attachments,
		})

		// Check that attachments section is present and appears before description
		assert.Contains(t, desc, "Attachments:")
		// Link attachments always show the URL with description (name is ignored for clickability)
		assert.Contains(t, desc, "• https://example.com/agenda - Meeting agenda")
		assert.NotContains(t, desc, "Agenda:")
		// File with name should show just the name (filename is not shown)
		assert.Contains(t, desc, "• Presentation - Slide deck")
		// Link without name and description should show just URL
		assert.Contains(t, desc, "• https://example.com/notes")
		// File without name should show just filename
		assert.Contains(t, desc, "• document.docx")
		assert.Contains(t, desc, "Meeting with attachments")
		// Verify instructional message is present for files
		assert.Contains(t, desc, "To download files, click on the 'Join meeting' link:")

		// Verify attachments appear before description
		attachmentsIndex := strings.Index(desc, "Attachments:")
		descriptionIndex := strings.Index(desc, "Meeting with attachments")
		assert.Less(t, attachmentsIndex, descriptionIndex, "Attachments should appear before description")
	})
}

// Test ICS cancellation generation
func TestGenerateMeetingCancellationICS(t *testing.T) {
	generator := NewICSGenerator()
	startTime := time.Date(2024, 10, 25, 10, 0, 0, 0, time.UTC)

	params := ICSMeetingCancellationParams{
		MeetingUID:      "test-meeting-cancel-123",
		MeetingTitle:    "Test Meeting",
		StartTime:       startTime,
		DurationMinutes: 60,
		Timezone:        "America/New_York",
		RecipientEmail:  "test@example.com",
		Recurrence:      nil,
		Sequence:        2,
	}

	icsContent, err := generator.GenerateMeetingCancellationICS(params)
	assert.NoError(t, err)

	// Check for required ICS fields
	assert.Contains(t, icsContent, "BEGIN:VCALENDAR")
	assert.Contains(t, icsContent, "END:VCALENDAR")
	assert.Contains(t, icsContent, "UID:test-meeting-cancel-123")
	assert.Contains(t, icsContent, "METHOD:CANCEL")
	assert.Contains(t, icsContent, "STATUS:CANCELLED")
	assert.Contains(t, icsContent, "SUMMARY:Test Meeting (CANCELLED)")
	assert.Contains(t, icsContent, "ORGANIZER;CN=ITX:mailto:itx@linuxfoundation.org")
	assert.Contains(t, icsContent, "ATTENDEE;PARTSTAT=NEEDS-ACTION;RSVP=TRUE:mailto:test@example.com")
	assert.Contains(t, icsContent, "SEQUENCE:2")
	assert.Contains(t, icsContent, "DTSTART;TZID=America/New_York:20241025T060000")
	assert.Contains(t, icsContent, "DTEND;TZID=America/New_York:20241025T070000")
}

func TestGenerateMeetingCancellationICS_WithRecurrence(t *testing.T) {
	generator := NewICSGenerator()
	startTime := time.Date(2024, 10, 25, 10, 0, 0, 0, time.UTC)

	// Weekly recurring meeting
	recurrence := &models.Recurrence{
		Type:           2, // Weekly
		RepeatInterval: 1,
		WeeklyDays:     "2,4", // Monday and Wednesday
	}

	params := ICSMeetingCancellationParams{
		MeetingUID:      "weekly-staff-cancel-456",
		MeetingTitle:    "Weekly Staff Meeting",
		StartTime:       startTime,
		DurationMinutes: 30,
		Timezone:        "UTC",
		RecipientEmail:  "staff@example.com",
		Recurrence:      recurrence,
		Sequence:        1,
	}

	icsContent, err := generator.GenerateMeetingCancellationICS(params)
	assert.NoError(t, err)

	// Check for recurrence rule and UID
	assert.Contains(t, icsContent, "UID:weekly-staff-cancel-456")
	assert.Contains(t, icsContent, "RRULE:FREQ=WEEKLY;BYDAY=MO,WE")
	assert.Contains(t, icsContent, "STATUS:CANCELLED")
	assert.Contains(t, icsContent, "SUMMARY:Weekly Staff Meeting (CANCELLED)")
}
