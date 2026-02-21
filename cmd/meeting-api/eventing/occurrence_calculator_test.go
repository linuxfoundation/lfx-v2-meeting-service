// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOccurrenceCalculator_NoRecurrence(t *testing.T) {
	calc := NewOccurrenceCalculator(slog.Default())
	startTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	occurrences, err := calc.CalculateOccurrences(startTime, 60, nil, nil, nil)
	require.NoError(t, err)
	require.Len(t, occurrences, 1)

	assert.Equal(t, "20240115T100000Z", occurrences[0].OccurrenceID)
	assert.Equal(t, startTime, occurrences[0].StartTime)
	assert.Equal(t, 60, occurrences[0].Duration)
	assert.Equal(t, "available", occurrences[0].Status)
}

func TestOccurrenceCalculator_DailyRecurrence(t *testing.T) {
	calc := NewOccurrenceCalculator(slog.Default())
	startTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	rec := &RecurrenceInput{
		Type:           1, // Daily
		RepeatInterval: 1,
		EndTimes:       5, // 5 occurrences
	}

	occurrences, err := calc.CalculateOccurrences(startTime, 60, rec, nil, nil)
	require.NoError(t, err)
	require.Len(t, occurrences, 5)

	// Check first occurrence
	assert.Equal(t, "20240115T100000Z", occurrences[0].OccurrenceID)
	assert.Equal(t, startTime, occurrences[0].StartTime)

	// Check second occurrence (next day)
	expectedSecond := startTime.Add(24 * time.Hour)
	assert.Equal(t, expectedSecond, occurrences[1].StartTime)

	// All should have same duration
	for _, occ := range occurrences {
		assert.Equal(t, 60, occ.Duration)
	}
}

func TestOccurrenceCalculator_WeeklyRecurrence(t *testing.T) {
	calc := NewOccurrenceCalculator(slog.Default())
	// Start on Monday, Jan 15, 2024
	startTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	rec := &RecurrenceInput{
		Type:           2,      // Weekly
		RepeatInterval: 1,      // Every week
		WeeklyDays:     "2,4",  // Monday and Wednesday (2=Mon, 4=Wed)
		EndTimes:       4,      // 4 occurrences
	}

	occurrences, err := calc.CalculateOccurrences(startTime, 60, rec, nil, nil)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(occurrences), 2, "Should have at least 2 occurrences")

	// First occurrence should be on Monday
	assert.Equal(t, time.Monday, occurrences[0].StartTime.Weekday())
}

func TestOccurrenceCalculator_MonthlyByDay(t *testing.T) {
	calc := NewOccurrenceCalculator(slog.Default())
	// Start on Jan 15, 2024
	startTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	rec := &RecurrenceInput{
		Type:           3,  // Monthly
		RepeatInterval: 1,  // Every month
		MonthlyDay:     15, // 15th of each month
		EndTimes:       3,  // 3 occurrences
	}

	occurrences, err := calc.CalculateOccurrences(startTime, 60, rec, nil, nil)
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
	rec := &RecurrenceInput{
		Type:           3, // Monthly
		RepeatInterval: 1, // Every month
		MonthlyWeek:    1, // First week
		MonthlyWeekDay: 3, // Tuesday (3=Tuesday)
		EndTimes:       3, // 3 occurrences
	}

	occurrences, err := calc.CalculateOccurrences(startTime, 60, rec, nil, nil)
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

	rec := &RecurrenceInput{
		Type:           1,                            // Daily
		RepeatInterval: 1,                            // Every day
		EndDateTime:    endTime.Format(time.RFC3339), // End on Jan 22
	}

	occurrences, err := calc.CalculateOccurrences(startTime, 60, rec, nil, nil)
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
	rec := &RecurrenceInput{
		Type:           1, // Daily
		RepeatInterval: 1,
		EndTimes:       5, // 5 occurrences
	}

	// Cancel the second occurrence
	secondOccTime := startTime.Add(24 * time.Hour)
	cancelledOccurrences := []string{secondOccTime.Format("20060102T150405Z")}

	occurrences, err := calc.CalculateOccurrences(startTime, 60, rec, cancelledOccurrences, nil)
	require.NoError(t, err)

	// Should have 4 occurrences (5 - 1 cancelled)
	assert.Len(t, occurrences, 4)

	// Second occurrence should not be in the list
	for _, occ := range occurrences {
		assert.NotEqual(t, secondOccTime.Format("20060102T150405Z"), occ.OccurrenceID)
	}
}

func TestOccurrenceCalculator_UpdatedOccurrences(t *testing.T) {
	calc := NewOccurrenceCalculator(slog.Default())
	startTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	rec := &RecurrenceInput{
		Type:           1, // Daily
		RepeatInterval: 1,
		EndTimes:       3, // 3 occurrences
	}

	// Update the second occurrence
	secondOccTime := startTime.Add(24 * time.Hour)
	updatedOccurrences := map[string]OccurrenceUpdate{
		secondOccTime.Format("20060102T150405Z"): {
			StartTime: secondOccTime.Add(2 * time.Hour), // Move 2 hours later
			Duration:  90,                                // Change duration to 90 minutes
		},
	}

	occurrences, err := calc.CalculateOccurrences(startTime, 60, rec, nil, updatedOccurrences)
	require.NoError(t, err)
	require.Len(t, occurrences, 3)

	// First occurrence should be unchanged
	assert.Equal(t, startTime, occurrences[0].StartTime)
	assert.Equal(t, 60, occurrences[0].Duration)

	// Second occurrence should be updated
	assert.Equal(t, secondOccTime.Add(2*time.Hour), occurrences[1].StartTime)
	assert.Equal(t, 90, occurrences[1].Duration)

	// Third occurrence should be unchanged
	assert.Equal(t, 60, occurrences[2].Duration)
}

func TestOccurrenceCalculator_ApplyAllFollowingUpdates(t *testing.T) {
	calc := NewOccurrenceCalculator(slog.Default())

	// Create initial occurrences
	startTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	occurrences := []Occurrence{
		{OccurrenceID: "20240115T100000Z", StartTime: startTime, Duration: 60},
		{OccurrenceID: "20240116T100000Z", StartTime: startTime.Add(24 * time.Hour), Duration: 60},
		{OccurrenceID: "20240117T100000Z", StartTime: startTime.Add(48 * time.Hour), Duration: 60},
		{OccurrenceID: "20240118T100000Z", StartTime: startTime.Add(72 * time.Hour), Duration: 60},
	}

	// Update from second occurrence onwards
	newStartTime := startTime.Add(24 * time.Hour).Add(2 * time.Hour) // 2 hours later
	updated := calc.ApplyAllFollowingUpdates(occurrences, "20240116T100000Z", newStartTime, 90)

	require.Len(t, updated, 4)

	// First occurrence should be unchanged
	assert.Equal(t, startTime, updated[0].StartTime)
	assert.Equal(t, 60, updated[0].Duration)

	// All following occurrences should be updated
	assert.Equal(t, newStartTime, updated[1].StartTime)
	assert.Equal(t, 90, updated[1].Duration)

	assert.Equal(t, newStartTime, updated[2].StartTime)
	assert.Equal(t, 90, updated[2].Duration)

	assert.Equal(t, newStartTime, updated[3].StartTime)
	assert.Equal(t, 90, updated[3].Duration)
}

func TestOccurrenceCalculator_BiWeeklyRecurrence(t *testing.T) {
	calc := NewOccurrenceCalculator(slog.Default())
	startTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	rec := &RecurrenceInput{
		Type:           2,     // Weekly
		RepeatInterval: 2,     // Every 2 weeks
		WeeklyDays:     "2",   // Monday
		EndTimes:       4,     // 4 occurrences
	}

	occurrences, err := calc.CalculateOccurrences(startTime, 60, rec, nil, nil)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(occurrences), 2, "Should have at least 2 occurrences")

	if len(occurrences) >= 2 {
		// Check that occurrences are 2 weeks apart
		diff := occurrences[1].StartTime.Sub(occurrences[0].StartTime)
		assert.Equal(t, 14*24*time.Hour, diff, "Occurrences should be 14 days apart")
	}
}

func TestOccurrenceCalculator_MaxOccurrencesLimit(t *testing.T) {
	calc := NewOccurrenceCalculator(slog.Default())
	startTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	rec := &RecurrenceInput{
		Type:           1,   // Daily
		RepeatInterval: 1,   // Every day
		EndTimes:       200, // Request 200 occurrences
	}

	occurrences, err := calc.CalculateOccurrences(startTime, 60, rec, nil, nil)
	require.NoError(t, err)

	// Should be limited to 100 occurrences
	assert.LessOrEqual(t, len(occurrences), 100, "Should not exceed 100 occurrences")
}

func TestBuildRRule_InvalidType(t *testing.T) {
	calc := NewOccurrenceCalculator(slog.Default())
	startTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	rec := &RecurrenceInput{
		Type: 99, // Invalid type
	}

	_, err := calc.buildRRule(startTime, rec)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid recurrence type")
}

func TestConvertWeeklyDays(t *testing.T) {
	calc := NewOccurrenceCalculator(slog.Default())

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
		result := calc.convertWeeklyDays(tt.input)
		assert.Equal(t, tt.expected, result, "Input: %s", tt.input)
	}
}

func TestConvertDayOfWeek(t *testing.T) {
	calc := NewOccurrenceCalculator(slog.Default())

	tests := []struct {
		input    int
		expected string
	}{
		{1, "SU"},
		{2, "MO"},
		{3, "TU"},
		{4, "WE"},
		{5, "TH"},
		{6, "FR"},
		{7, "SA"},
	}

	for _, tt := range tests {
		result := calc.convertDayOfWeek(tt.input)
		assert.Equal(t, tt.expected, result, "Input: %d", tt.input)
	}
}
