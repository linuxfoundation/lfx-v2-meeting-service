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

// TestOccurrenceCalculator_QuarterlyCadenceChange is the regression test for LFXV2-2066.
// When a meeting's cadence is changed to quarterly via an all_following update, the
// occurrence calculator must:
//   - Stop emitting monthly occurrences at the old_occurrence_id boundary (not at new_occurrence_id).
//   - Emit quarterly occurrences starting from the new occurrence.
//   - Not emit any stale replaced occurrences.
//   - Return results sorted by start time.
func TestOccurrenceCalculator_QuarterlyCadenceChange(t *testing.T) {
	calc := NewOccurrenceCalculator(slog.Default())

	// Base meeting: monthly on the 1st Thursday, starting 2025-02-06 (1st Thu of Feb 2025)
	// The Zoom-style pattern is type=3 (Monthly), repeat_interval=1, monthly_week=1, monthly_week_day=5 (Thu).
	baseStart := time.Date(2025, 2, 6, 14, 0, 0, 0, time.UTC)

	// The cadence change to quarterly happens at the August 2025 occurrence.
	// old_occurrence_id: the original August 7 (1st Thu of Aug 2025) monthly slot, unix=1754571600
	// new_occurrence_id: August 7 at 14:00 (same date, adjusted time), unix=1754575200
	// After the change: repeat_interval=3, so occurrences are every 3 months (Nov, Feb, May …)
	augOldUnix := time.Date(2025, 8, 7, 13, 0, 0, 0, time.UTC).Unix() // original RRULE slot
	augNewUnix := time.Date(2025, 8, 7, 14, 0, 0, 0, time.UTC).Unix() // adjusted anchor
	quarterlyRecurrence := &models.ZoomMeetingRecurrence{
		Type:           3, // Monthly
		RepeatInterval: 3, // Every 3 months = quarterly
		MonthlyWeek:    1, // First week
		MonthlyWeekDay: 5, // Thursday
		EndTimes:       8, // 8 quarterly occurrences
	}

	meeting := models.MeetingEventData{
		ID:          "test-quarterly-meeting",
		Title:       "AAIF Outreach Committee Meeting",
		Description: "Monthly becoming quarterly",
		StartTime:   baseStart.Format(time.RFC3339),
		Timezone:    "UTC",
		Duration:    60,
		Recurrence: &models.ZoomMeetingRecurrence{
			Type:           3,
			RepeatInterval: 1, // Original: monthly
			MonthlyWeek:    1,
			MonthlyWeekDay: 5,
			EndTimes:       50, // Enough for the test
		},
		CancelledOccurrences: []string{},
		UpdatedOccurrences: []models.UpdatedOccurrence{
			{
				OldOccurrenceID: strconv.FormatInt(augOldUnix, 10),
				NewOccurrenceID: strconv.FormatInt(augNewUnix, 10),
				AllFollowing:    true,
				Duration:        60,
				Recurrence:      quarterlyRecurrence,
			},
		},
	}

	occurrences, err := calc.CalculateOccurrences(context.Background(), meeting, true, false, 100)
	require.NoError(t, err)
	require.NotEmpty(t, occurrences, "should have occurrences")

	// Occurrences must be sorted ascending by start time
	for i := 1; i < len(occurrences); i++ {
		prevID, _ := strconv.ParseInt(occurrences[i-1].OccurrenceID, 10, 64)
		currID, _ := strconv.ParseInt(occurrences[i].OccurrenceID, 10, 64)
		assert.Less(t, prevID, currID, "occurrences[%d] must be before occurrences[%d]", i-1, i)
	}

	// No stale replaced occurrence: the original August monthly slot (augOldUnix) must not appear.
	for _, occ := range occurrences {
		occUnix, _ := strconv.ParseInt(occ.OccurrenceID, 10, 64)
		assert.NotEqual(t, augOldUnix, occUnix,
			"stale replaced occurrence (old_occurrence_id) must not appear in the output")
	}

	// Find the anchor occurrence (augNewUnix) — it must be present.
	anchorID := strconv.FormatInt(augNewUnix, 10)
	var foundAnchor bool
	for _, occ := range occurrences {
		if occ.OccurrenceID == anchorID {
			foundAnchor = true
			break
		}
	}
	assert.True(t, foundAnchor, "anchor occurrence (new_occurrence_id) must be present")

	// All occurrences after the cadence change must be >= 3 months apart (quarterly).
	var postChangeOccs []models.Occurrence
	for _, occ := range occurrences {
		occUnix, _ := strconv.ParseInt(occ.OccurrenceID, 10, 64)
		if occUnix >= augNewUnix {
			postChangeOccs = append(postChangeOccs, occ)
		}
	}
	require.GreaterOrEqual(t, len(postChangeOccs), 2, "expected multiple quarterly occurrences after the cadence change")
	for i := 1; i < len(postChangeOccs); i++ {
		gap := postChangeOccs[i].StartTime.Sub(postChangeOccs[i-1].StartTime)
		// 3 months ≈ 88–92 days — assert at least 80 days between occurrences
		assert.GreaterOrEqual(t, gap.Hours(), float64(80*24),
			"quarterly occurrences should be ~3 months apart, got %v between occ %d and %d", gap, i-1, i)
	}
}

// TestOccurrenceCalculator_AllFollowingNoRecurrenceChange verifies that an all_following
// update that changes only title (not recurrence) is handled correctly —
// the inherited recurrence keeps generating occurrences at the original cadence.
// Boundary behaviour: the base segment stops at the anchor occurrence's old_occurrence_id,
// and the all_following segment starts there and generates occurrences with the inherited
// recurrence. If the recurrence has EndTimes=N, the all_following segment generates N
// occurrences from its own start (matching ITX canonical behaviour).
func TestOccurrenceCalculator_AllFollowingNoRecurrenceChange(t *testing.T) {
	calc := NewOccurrenceCalculator(slog.Default())

	// Use EndDateTime so the total number of occurrences is deterministic regardless of which
	// segment inherits it — both base and all_following respect the same terminal date.
	startTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	endDate := time.Date(2024, 1, 19, 23, 59, 59, 0, time.UTC) // 5 daily occurrences total

	// Third occurrence (day 3): all_following update — title changes, no recurrence change.
	thirdOcc := startTime.Add(2 * 24 * time.Hour)
	thirdOccID := strconv.FormatInt(thirdOcc.Unix(), 10)

	meeting := models.MeetingEventData{
		ID:        "test-all-following-no-rec",
		Title:     "Original Title",
		StartTime: startTime.Format(time.RFC3339),
		Timezone:  "UTC",
		Duration:  60,
		Recurrence: &models.ZoomMeetingRecurrence{
			Type:           1, // Daily
			RepeatInterval: 1,
			EndDateTime:    endDate.Format(time.RFC3339),
		},
		CancelledOccurrences: []string{},
		UpdatedOccurrences: []models.UpdatedOccurrence{
			{
				OldOccurrenceID: thirdOccID,
				NewOccurrenceID: thirdOccID,
				AllFollowing:    true,
				Duration:        60,
				Title:           "Updated Title",
				Recurrence:      nil, // No recurrence change — inherit base recurrence
			},
		},
	}

	occurrences, err := calc.CalculateOccurrences(context.Background(), meeting, true, false, 100)
	require.NoError(t, err)
	require.Len(t, occurrences, 5, "should have 5 daily occurrences bounded by EndDateTime")

	// Occurrences must be sorted
	for i := 1; i < len(occurrences); i++ {
		prevID, _ := strconv.ParseInt(occurrences[i-1].OccurrenceID, 10, 64)
		currID, _ := strconv.ParseInt(occurrences[i].OccurrenceID, 10, 64)
		assert.Less(t, prevID, currID, "occurrences must be sorted")
	}

	// Occurrences before the update retain the original title
	assert.Equal(t, "Original Title", occurrences[0].Title)
	assert.Equal(t, "Original Title", occurrences[1].Title)
	// Occurrences from the anchor onward have the updated title
	assert.Equal(t, "Updated Title", occurrences[2].Title)
	assert.Equal(t, "Updated Title", occurrences[3].Title)
	assert.Equal(t, "Updated Title", occurrences[4].Title)
}

// TestGetEffectiveRecurrence verifies that getEffectiveRecurrence returns the correct
// recurrence rule for the LFXV2-2066 scenario: a meeting with a quarterly cadence change.
func TestGetEffectiveRecurrence(t *testing.T) {
	baseRecurrence := &models.ZoomMeetingRecurrence{
		Type:           3,
		RepeatInterval: 1, // monthly
		MonthlyWeek:    1,
		MonthlyWeekDay: 5,
	}
	quarterlyRecurrence := &models.ZoomMeetingRecurrence{
		Type:           3,
		RepeatInterval: 3, // quarterly
		MonthlyWeek:    1,
		MonthlyWeekDay: 5,
	}

	// Cadence change happened in August 2025
	changeUnix := time.Date(2025, 8, 7, 14, 0, 0, 0, time.UTC).Unix()

	meeting := models.MeetingEventData{
		StartTime:  time.Date(2025, 2, 6, 14, 0, 0, 0, time.UTC).Format(time.RFC3339),
		Recurrence: baseRecurrence,
		UpdatedOccurrences: []models.UpdatedOccurrence{
			{
				OldOccurrenceID: strconv.FormatInt(changeUnix-3600, 10),
				NewOccurrenceID: strconv.FormatInt(changeUnix, 10),
				AllFollowing:    true,
				Recurrence:      quarterlyRecurrence,
			},
		},
	}

	// Before the cadence change: should return the base monthly rule
	beforeChange := time.Date(2025, 5, 1, 0, 0, 0, 0, time.UTC)
	eff := getEffectiveRecurrence(meeting, beforeChange)
	assert.Equal(t, 1, eff.RepeatInterval, "before the change: should return monthly (interval=1)")

	// After the cadence change: should return the quarterly rule
	afterChange := time.Date(2025, 10, 1, 0, 0, 0, 0, time.UTC)
	eff = getEffectiveRecurrence(meeting, afterChange)
	assert.Equal(t, 3, eff.RepeatInterval, "after the change: should return quarterly (interval=3)")

	// Meeting with no all_following updates: always returns the base rule
	noUpdates := models.MeetingEventData{
		Recurrence:         baseRecurrence,
		UpdatedOccurrences: []models.UpdatedOccurrence{},
	}
	eff = getEffectiveRecurrence(noUpdates, afterChange)
	assert.Equal(t, 1, eff.RepeatInterval, "no updates: should always return base rule")

	// nil recurrence: should return nil
	nilMeeting := models.MeetingEventData{Recurrence: nil}
	assert.Nil(t, getEffectiveRecurrence(nilMeeting, afterChange))
}

// TestOccurrenceCalculator_OutputIsSorted verifies that the calculator always returns
// occurrences in ascending start-time order regardless of segment iteration order.
func TestOccurrenceCalculator_OutputIsSorted(t *testing.T) {
	calc := NewOccurrenceCalculator(slog.Default())
	startTime := time.Date(2024, 3, 1, 9, 0, 0, 0, time.UTC)

	meeting := models.MeetingEventData{
		ID:        "test-sort",
		StartTime: startTime.Format(time.RFC3339),
		Timezone:  "UTC",
		Duration:  30,
		Recurrence: &models.ZoomMeetingRecurrence{
			Type:           1, // Daily
			RepeatInterval: 1,
			EndTimes:       10,
		},
		CancelledOccurrences: []string{},
		UpdatedOccurrences:   []models.UpdatedOccurrence{},
	}

	occurrences, err := calc.CalculateOccurrences(context.Background(), meeting, true, false, 100)
	require.NoError(t, err)
	require.Len(t, occurrences, 10)

	for i := 1; i < len(occurrences); i++ {
		assert.True(t, occurrences[i].StartTime.After(occurrences[i-1].StartTime),
			"occurrence %d (%v) should be after occurrence %d (%v)",
			i, occurrences[i].StartTime, i-1, occurrences[i-1].StartTime)
	}
}

// Helper function to parse occurrence ID (unix timestamp string) to int64
func parseOccurrenceID(t *testing.T, occurrenceID string) int64 {
	ts, err := strconv.ParseInt(occurrenceID, 10, 64)
	require.NoError(t, err)
	return ts
}
