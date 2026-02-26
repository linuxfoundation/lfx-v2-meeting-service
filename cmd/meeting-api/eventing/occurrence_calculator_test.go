// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"log/slog"
	"strconv"
	"testing"
	"time"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOccurrenceCalculator_NoRecurrence(t *testing.T) {
	calc := NewOccurrenceCalculator(slog.Default())
	startTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	meeting := models.MeetingEventData{
		ID:                   "test-meeting-1",
		Title:                "Test Meeting",
		Description:          "Test Description",
		StartTime:            startTime.Format(time.RFC3339),
		Timezone:             "UTC",
		Duration:             60,
		Recurrence:           nil, // No recurrence
		CancelledOccurrences: []string{},
		UpdatedOccurrences:   []models.UpdatedOccurrence{},
	}

	occurrences, err := calc.CalculateOccurrences(context.Background(), meeting, false, false, 100)
	require.NoError(t, err)
	require.Len(t, occurrences, 0) // No occurrences for non-recurring meetings
}

func TestOccurrenceCalculator_DailyRecurrence(t *testing.T) {
	calc := NewOccurrenceCalculator(slog.Default())
	startTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	meeting := models.MeetingEventData{
		ID:          "test-meeting-1",
		Title:       "Test Meeting",
		Description: "Test Description",
		StartTime:   startTime.Format(time.RFC3339),
		Timezone:    "UTC",
		Duration:    60,
		Recurrence: &models.ZoomMeetingRecurrence{
			Type:           1, // Daily
			RepeatInterval: 1,
			EndTimes:       5, // 5 occurrences
		},
		CancelledOccurrences: []string{},
		UpdatedOccurrences:   []models.UpdatedOccurrence{},
	}

	// Include past occurrences for testing
	occurrences, err := calc.CalculateOccurrences(context.Background(), meeting, true, false, 100)
	require.NoError(t, err)
	require.Len(t, occurrences, 5)

	// Check first occurrence
	assert.Equal(t, startTime.Unix(), parseOccurrenceID(t, occurrences[0].OccurrenceID))
	assert.Equal(t, startTime.UTC(), occurrences[0].StartTime.UTC())

	// Check second occurrence (next day)
	expectedSecond := startTime.Add(24 * time.Hour)
	assert.Equal(t, expectedSecond.UTC(), occurrences[1].StartTime.UTC())

	// All should have same duration
	for _, occ := range occurrences {
		assert.Equal(t, 60, occ.Duration)
	}
}

func TestOccurrenceCalculator_WeeklyRecurrence(t *testing.T) {
	calc := NewOccurrenceCalculator(slog.Default())
	// Start on Monday, Jan 15, 2024
	startTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	meeting := models.MeetingEventData{
		ID:          "test-meeting-1",
		Title:       "Test Meeting",
		Description: "Test Description",
		StartTime:   startTime.Format(time.RFC3339),
		Timezone:    "UTC",
		Duration:    60,
		Recurrence: &models.ZoomMeetingRecurrence{
			Type:           2,     // Weekly
			RepeatInterval: 1,     // Every week
			WeeklyDays:     "2,4", // Monday and Wednesday (2=Mon, 4=Wed)
			EndTimes:       4,     // 4 occurrences
		},
		CancelledOccurrences: []string{},
		UpdatedOccurrences:   []models.UpdatedOccurrence{},
	}

	occurrences, err := calc.CalculateOccurrences(context.Background(), meeting, true, false, 100)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(occurrences), 2, "Should have at least 2 occurrences")

	// First occurrence should be on Monday
	assert.Equal(t, time.Monday, occurrences[0].StartTime.Weekday())
}

func TestOccurrenceCalculator_MonthlyByDay(t *testing.T) {
	calc := NewOccurrenceCalculator(slog.Default())
	// Start on Jan 15, 2024
	startTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	meeting := models.MeetingEventData{
		ID:          "test-meeting-1",
		Title:       "Test Meeting",
		Description: "Test Description",
		StartTime:   startTime.Format(time.RFC3339),
		Timezone:    "UTC",
		Duration:    60,
		Recurrence: &models.ZoomMeetingRecurrence{
			Type:           3,  // Monthly
			RepeatInterval: 1,  // Every month
			MonthlyDay:     15, // 15th of each month
			EndTimes:       3,  // 3 occurrences
		},
		CancelledOccurrences: []string{},
		UpdatedOccurrences:   []models.UpdatedOccurrence{},
	}

	occurrences, err := calc.CalculateOccurrences(context.Background(), meeting, true, false, 100)
	require.NoError(t, err)
	require.Len(t, occurrences, 3)

	// Check that each occurrence is on the 15th
	for i, occ := range occurrences {
		assert.Equal(t, 15, occ.StartTime.Day(), "Occurrence %d should be on 15th", i)
	}
}

func TestOccurrenceCalculator_MonthlyByWeekDay(t *testing.T) {
	calc := NewOccurrenceCalculator(slog.Default())
	// Start on first Tuesday of Jan 2024 (Jan 2)
	startTime := time.Date(2024, 1, 2, 10, 0, 0, 0, time.UTC)

	meeting := models.MeetingEventData{
		ID:          "test-meeting-1",
		Title:       "Test Meeting",
		Description: "Test Description",
		StartTime:   startTime.Format(time.RFC3339),
		Timezone:    "UTC",
		Duration:    60,
		Recurrence: &models.ZoomMeetingRecurrence{
			Type:           3, // Monthly
			RepeatInterval: 1, // Every month
			MonthlyWeek:    1, // First week
			MonthlyWeekDay: 3, // Tuesday (3=Tuesday)
			EndTimes:       3, // 3 occurrences
		},
		CancelledOccurrences: []string{},
		UpdatedOccurrences:   []models.UpdatedOccurrence{},
	}

	occurrences, err := calc.CalculateOccurrences(context.Background(), meeting, true, false, 100)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(occurrences), 1, "Should have at least 1 occurrence")

	// All occurrences should be on Tuesday
	for i, occ := range occurrences {
		assert.Equal(t, time.Tuesday, occ.StartTime.Weekday(), "Occurrence %d should be on Tuesday", i)
	}
}

func TestOccurrenceCalculator_WithEndDate(t *testing.T) {
	calc := NewOccurrenceCalculator(slog.Default())
	startTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	endTime := time.Date(2024, 1, 22, 10, 0, 0, 0, time.UTC)

	meeting := models.MeetingEventData{
		ID:          "test-meeting-1",
		Title:       "Test Meeting",
		Description: "Test Description",
		StartTime:   startTime.Format(time.RFC3339),
		Timezone:    "UTC",
		Duration:    60,
		Recurrence: &models.ZoomMeetingRecurrence{
			Type:           1,                            // Daily
			RepeatInterval: 1,                            // Every day
			EndDateTime:    endTime.Format(time.RFC3339), // End on Jan 22
		},
		CancelledOccurrences: []string{},
		UpdatedOccurrences:   []models.UpdatedOccurrence{},
	}

	occurrences, err := calc.CalculateOccurrences(context.Background(), meeting, true, false, 100)
	require.NoError(t, err)

	// Should have occurrences from Jan 15 to Jan 22 (8 days)
	assert.LessOrEqual(t, len(occurrences), 8)

	// Last occurrence should not be after end date
	if len(occurrences) > 0 {
		lastOcc := occurrences[len(occurrences)-1]
		assert.True(t, lastOcc.StartTime.Before(endTime) || lastOcc.StartTime.Equal(endTime),
			"Last occurrence should not be after end date")
	}
}

func TestOccurrenceCalculator_CancelledOccurrences(t *testing.T) {
	calc := NewOccurrenceCalculator(slog.Default())
	startTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	// Cancel the second occurrence (using Unix timestamp as occurrence ID)
	secondOccTime := startTime.Add(24 * time.Hour)
	cancelledOccurrenceID := strconv.FormatInt(secondOccTime.Unix(), 10)

	meeting := models.MeetingEventData{
		ID:          "test-meeting-1",
		Title:       "Test Meeting",
		Description: "Test Description",
		StartTime:   startTime.Format(time.RFC3339),
		Timezone:    "UTC",
		Duration:    60,
		Recurrence: &models.ZoomMeetingRecurrence{
			Type:           1, // Daily
			RepeatInterval: 1,
			EndTimes:       5, // 5 occurrences
		},
		CancelledOccurrences: []string{cancelledOccurrenceID},
		UpdatedOccurrences:   []models.UpdatedOccurrence{},
	}

	occurrences, err := calc.CalculateOccurrences(context.Background(), meeting, true, false, 100)
	require.NoError(t, err)

	// Should have 4 occurrences (5 - 1 cancelled)
	assert.Len(t, occurrences, 4)

	// Second occurrence should not be in the list
	for _, occ := range occurrences {
		assert.NotEqual(t, cancelledOccurrenceID, occ.OccurrenceID)
	}
}

func TestOccurrenceCalculator_BiWeeklyRecurrence(t *testing.T) {
	calc := NewOccurrenceCalculator(slog.Default())
	startTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	meeting := models.MeetingEventData{
		ID:          "test-meeting-1",
		Title:       "Test Meeting",
		Description: "Test Description",
		StartTime:   startTime.Format(time.RFC3339),
		Timezone:    "UTC",
		Duration:    60,
		Recurrence: &models.ZoomMeetingRecurrence{
			Type:           2,   // Weekly
			RepeatInterval: 2,   // Every 2 weeks
			WeeklyDays:     "2", // Monday
			EndTimes:       4,   // 4 occurrences
		},
		CancelledOccurrences: []string{},
		UpdatedOccurrences:   []models.UpdatedOccurrence{},
	}

	occurrences, err := calc.CalculateOccurrences(context.Background(), meeting, true, false, 100)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(occurrences), 2, "Should have at least 2 occurrences")

	if len(occurrences) >= 2 {
		// Check that occurrences are 2 weeks apart
		diff := occurrences[1].StartTime.Sub(occurrences[0].StartTime)
		assert.Equal(t, 14*24*time.Hour, diff, "Occurrences should be 14 days apart")
	}
}

func TestParseByDay(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"1", "SU"},
		{"2", "MO"},
		{"1,3,5", "SU,TU,TH"},
		{"2,3,4,5,6", "MO,TU,WE,TH,FR"},
		{"1,7", "SU,SA"},
	}

	for _, tt := range tests {
		result, err := parseByDay(tt.input)
		require.NoError(t, err)
		assert.Equal(t, tt.expected, result, "Input: %s", tt.input)
	}
}

// Helper function to parse occurrence ID (unix timestamp string) to int64
func parseOccurrenceID(t *testing.T, occurrenceID string) int64 {
	ts, err := strconv.ParseInt(occurrenceID, 10, 64)
	require.NoError(t, err)
	return ts
}
