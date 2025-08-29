// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"strconv"
	"strings"
	"time"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
)

// OccurrenceService implements the domain.OccurrenceService interface
type OccurrenceService struct{}

// NewOccurrenceService creates a new OccurrenceService
func NewOccurrenceService() *OccurrenceService {
	return &OccurrenceService{}
}

// CalculateOccurrences calculates occurrences for a meeting starting from the meeting's start time
func (s *OccurrenceService) CalculateOccurrences(meeting *models.MeetingBase, limit int) []models.Occurrence {
	if meeting == nil {
		return []models.Occurrence{}
	}

	return s.CalculateOccurrencesFromDate(meeting, meeting.StartTime, limit)
}

// CalculateOccurrencesFromDate calculates occurrences for a meeting starting from a specific date
func (s *OccurrenceService) CalculateOccurrencesFromDate(meeting *models.MeetingBase, fromDate time.Time, limit int) []models.Occurrence {
	if meeting == nil || limit <= 0 {
		return []models.Occurrence{}
	}

	// If the meeting has no recurrence, return just the original meeting if it's after fromDate
	if meeting.Recurrence == nil {
		if meeting.StartTime.Before(fromDate) {
			return []models.Occurrence{}
		}
		return []models.Occurrence{s.createOccurrence(meeting, meeting.StartTime)}
	}

	// Load the meeting timezone
	loc, err := time.LoadLocation(meeting.Timezone)
	if err != nil {
		loc = time.UTC
	}

	var occurrences []models.Occurrence
	current := meeting.StartTime.In(loc)
	endDate := s.getEndDate(meeting.Recurrence, loc)

	// Calculate occurrences based on recurrence type
	switch meeting.Recurrence.Type {
	case 1: // Daily
		occurrences = s.calculateDailyOccurrences(meeting, current, fromDate, endDate, limit)
	case 2: // Weekly
		occurrences = s.calculateWeeklyOccurrences(meeting, current, fromDate, endDate, limit, loc)
	case 3: // Monthly
		occurrences = s.calculateMonthlyOccurrences(meeting, current, fromDate, endDate, limit, loc)
	}

	// Apply EndTimes limit if specified
	if meeting.Recurrence.EndTimes > 0 && len(occurrences) > meeting.Recurrence.EndTimes {
		occurrences = occurrences[:meeting.Recurrence.EndTimes]
	}

	return occurrences
}

// calculateDailyOccurrences calculates occurrences for daily recurrence patterns
func (s *OccurrenceService) calculateDailyOccurrences(meeting *models.MeetingBase, startTime, fromDate, endDate time.Time, limit int) []models.Occurrence {
	var occurrences []models.Occurrence
	current := startTime

	for len(occurrences) < limit {
		// Check if we've exceeded the end date (use Before or Equal to exclude the end date)
		if !endDate.IsZero() && !current.Before(endDate) {
			break
		}

		// If this occurrence is on or after the fromDate, include it
		if !current.Before(fromDate) {
			occurrences = append(occurrences, s.createOccurrence(meeting, current))
		}

		// Move to next occurrence
		current = current.AddDate(0, 0, meeting.Recurrence.RepeatInterval)
	}

	return occurrences
}

// calculateWeeklyOccurrences calculates occurrences for weekly recurrence patterns
func (s *OccurrenceService) calculateWeeklyOccurrences(meeting *models.MeetingBase, startTime, fromDate, endDate time.Time, limit int, loc *time.Location) []models.Occurrence {
	var occurrences []models.Occurrence

	// Parse weekly days
	weeklyDays := s.parseWeeklyDays(meeting.Recurrence.WeeklyDays)
	if len(weeklyDays) == 0 {
		// If no weekly days specified, use the start time's weekday
		weeklyDays = []int{int(startTime.Weekday())}
	}

	// Start from the beginning of the week containing startTime
	weekStart := s.getStartOfWeek(startTime)

	weekCount := 0
	for len(occurrences) < limit {
		currentWeek := weekStart.AddDate(0, 0, weekCount*7*meeting.Recurrence.RepeatInterval)

		// Check if we've exceeded the end date
		if !endDate.IsZero() && !currentWeek.Before(endDate) {
			break
		}

		// Generate occurrences for each specified day of this week
		for _, dayOfWeek := range weeklyDays {
			dayOffset := (dayOfWeek - int(currentWeek.Weekday()) + 7) % 7
			occurrenceDate := currentWeek.AddDate(0, 0, dayOffset)

			// Preserve the original time
			occurrenceDate = time.Date(
				occurrenceDate.Year(), occurrenceDate.Month(), occurrenceDate.Day(),
				startTime.Hour(), startTime.Minute(), startTime.Second(), startTime.Nanosecond(),
				loc,
			)

			// Check if we've exceeded the end date
			if !endDate.IsZero() && !occurrenceDate.Before(endDate) {
				continue
			}

			// If this occurrence is on or after the fromDate, include it
			if !occurrenceDate.Before(fromDate) && len(occurrences) < limit {
				occurrences = append(occurrences, s.createOccurrence(meeting, occurrenceDate))
			}
		}

		weekCount++

		// Safety check to prevent infinite loops
		if weekCount > 1000 {
			break
		}
	}

	return occurrences
}

// calculateMonthlyOccurrences calculates occurrences for monthly recurrence patterns
func (s *OccurrenceService) calculateMonthlyOccurrences(meeting *models.MeetingBase, startTime, fromDate, endDate time.Time, limit int, loc *time.Location) []models.Occurrence {
	var occurrences []models.Occurrence
	current := startTime

	monthCount := 0
	for len(occurrences) < limit {
		// Check if we've exceeded the end date
		if !endDate.IsZero() && !current.Before(endDate) {
			break
		}

		var occurrenceDate time.Time

		if meeting.Recurrence.MonthlyDay > 0 {
			// Monthly by day of month (e.g., 15th of every month)
			occurrenceDate = s.calculateMonthlyByDay(startTime, monthCount, meeting.Recurrence.MonthlyDay, meeting.Recurrence.RepeatInterval, loc)
		} else if meeting.Recurrence.MonthlyWeek != 0 && meeting.Recurrence.MonthlyWeekDay > 0 {
			// Monthly by week and day (e.g., 2nd Tuesday of every month)
			occurrenceDate = s.calculateMonthlyByWeekDay(startTime, monthCount, meeting.Recurrence.MonthlyWeek, meeting.Recurrence.MonthlyWeekDay, meeting.Recurrence.RepeatInterval, loc)
		} else {
			// Default to same day of month as start time
			occurrenceDate = s.calculateMonthlyByDay(startTime, monthCount, startTime.Day(), meeting.Recurrence.RepeatInterval, loc)
		}

		// Check if we've exceeded the end date
		if !endDate.IsZero() && !occurrenceDate.Before(endDate) {
			break
		}

		// If this occurrence is on or after the fromDate, include it
		if !occurrenceDate.Before(fromDate) {
			occurrences = append(occurrences, s.createOccurrence(meeting, occurrenceDate))
		}

		monthCount++
		current = occurrenceDate

		// Safety check to prevent infinite loops
		if monthCount > 1000 {
			break
		}
	}

	return occurrences
}

// calculateMonthlyByDay calculates monthly occurrence by day of month
func (s *OccurrenceService) calculateMonthlyByDay(startTime time.Time, monthCount, dayOfMonth, interval int, loc *time.Location) time.Time {
	// Calculate target year and month
	totalMonths := monthCount * interval
	year := startTime.Year()
	month := int(startTime.Month()) + totalMonths

	// Normalize month/year (e.g., month 13 -> year+1, month 1)
	for month > 12 {
		year++
		month -= 12
	}

	// Handle case where target day doesn't exist in target month (e.g., Feb 31)
	lastDayOfMonth := time.Date(year, time.Month(month)+1, 0, 0, 0, 0, 0, loc).Day()
	actualDay := dayOfMonth
	if actualDay > lastDayOfMonth {
		actualDay = lastDayOfMonth
	}

	return time.Date(
		year, time.Month(month), actualDay,
		startTime.Hour(), startTime.Minute(), startTime.Second(), startTime.Nanosecond(),
		loc,
	)
}

// calculateMonthlyByWeekDay calculates monthly occurrence by week and weekday
func (s *OccurrenceService) calculateMonthlyByWeekDay(startTime time.Time, monthCount, week, weekDay, interval int, loc *time.Location) time.Time {
	targetMonth := startTime.AddDate(0, monthCount*interval, 0)
	firstOfMonth := time.Date(targetMonth.Year(), targetMonth.Month(), 1, startTime.Hour(), startTime.Minute(), startTime.Second(), startTime.Nanosecond(), loc)

	if week == -1 {
		// Last occurrence of weekday in month
		lastOfMonth := time.Date(targetMonth.Year(), targetMonth.Month()+1, 0, startTime.Hour(), startTime.Minute(), startTime.Second(), startTime.Nanosecond(), loc)
		return s.findLastWeekdayOfMonth(lastOfMonth, time.Weekday(weekDay-1))
	}

	// Find the first occurrence of the weekday in the month
	targetWeekday := time.Weekday(weekDay - 1) // Convert from 1-7 to 0-6
	daysToAdd := (int(targetWeekday) - int(firstOfMonth.Weekday()) + 7) % 7
	firstOccurrence := firstOfMonth.AddDate(0, 0, daysToAdd)

	// Add weeks to get to the target week
	return firstOccurrence.AddDate(0, 0, (week-1)*7)
}

// Helper functions

// getEndDate determines the end date based on recurrence settings
func (s *OccurrenceService) getEndDate(recurrence *models.Recurrence, loc *time.Location) time.Time {
	if recurrence.EndDateTime != nil {
		return recurrence.EndDateTime.In(loc)
	}
	// If EndTimes is specified, we'll handle it later by limiting the slice
	// For now, return zero time to indicate no end date
	return time.Time{}
}

// parseWeeklyDays parses the weekly days string (e.g., "1,3,5") into day integers
func (s *OccurrenceService) parseWeeklyDays(weeklyDays string) []int {
	if weeklyDays == "" {
		return []int{}
	}

	dayStrings := strings.Split(weeklyDays, ",")
	var days []int

	for _, dayStr := range dayStrings {
		dayStr = strings.TrimSpace(dayStr)
		if day, err := strconv.Atoi(dayStr); err == nil && day >= 1 && day <= 7 {
			// Convert from 1=Sunday, 2=Monday to Go's 0=Sunday, 1=Monday format
			// day 1 (Sunday) -> 0, day 2 (Monday) -> 1, etc.
			goWeekday := (day - 1) % 7
			days = append(days, goWeekday)
		}
	}

	return days
}

// getStartOfWeek gets the start of the week (Sunday) for a given date
func (s *OccurrenceService) getStartOfWeek(date time.Time) time.Time {
	weekday := int(date.Weekday())
	return date.AddDate(0, 0, -weekday)
}

// findLastWeekdayOfMonth finds the last occurrence of a weekday in a month
func (s *OccurrenceService) findLastWeekdayOfMonth(lastOfMonth time.Time, targetWeekday time.Weekday) time.Time {
	daysBack := (int(lastOfMonth.Weekday()) - int(targetWeekday) + 7) % 7
	return lastOfMonth.AddDate(0, 0, -daysBack)
}

// createOccurrence creates an occurrence model from meeting and start time
func (s *OccurrenceService) createOccurrence(meeting *models.MeetingBase, startTime time.Time) models.Occurrence {
	return models.Occurrence{
		OccurrenceID:     strconv.FormatInt(startTime.Unix(), 10),
		StartTime:        &startTime,
		Title:            meeting.Title,
		Description:      meeting.Description,
		Duration:         meeting.Duration,
		Recurrence:       nil, // Occurrences don't have recurrence patterns - reserved for future use
		RegistrantCount:  meeting.RegistrantCount,
		ResponseCountNo:  meeting.RegistrantResponseDeclinedCount,
		ResponseCountYes: meeting.RegistrantResponseAcceptedCount,
		IsCancelled:      false, // Default to not cancelled for calculated occurrences
	}
}

// Compile-time interface check
var _ domain.OccurrenceService = (*OccurrenceService)(nil)
