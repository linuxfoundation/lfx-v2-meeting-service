// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
)

const (
	MaxWeeksCount  = 1000
	MaxMonthsCount = 500
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

// ValidateFutureOccurrenceID validates that an occurrence ID exists for a meeting and is in the future
func (s *OccurrenceService) ValidateFutureOccurrenceID(meeting *models.MeetingBase, occurrenceID string, maxOccurrencesToCheck int) error {
	if meeting == nil || occurrenceID == "" {
		return domain.NewValidationError("meeting and occurrence ID are required")
	}

	if maxOccurrencesToCheck <= 0 {
		return domain.NewValidationError("maxOccurrencesToCheck must be greater than 0")
	}

	// Calculate occurrences for the meeting up to the specified limit
	occurrences := s.CalculateOccurrences(meeting, maxOccurrencesToCheck)

	// Check if the provided occurrence ID exists
	var foundOccurrence *models.Occurrence
	for i := range occurrences {
		slog.Debug("checking occurrence", "occurrence_id", occurrences[i].OccurrenceID)
		if occurrences[i].OccurrenceID == occurrenceID {
			foundOccurrence = &occurrences[i]
			break
		}
	}

	if foundOccurrence == nil {
		return domain.NewValidationError("invalid occurrence ID: occurrence not found for this meeting")
	}

	// Check if the occurrence is in the future
	if foundOccurrence.StartTime != nil && foundOccurrence.StartTime.Before(time.Now()) {
		return domain.NewValidationError("invalid occurrence ID: cannot register for past occurrences")
	}

	return nil
}

// CalculateOccurrencesFromDate calculates occurrences for a meeting starting from a specific date
func (s *OccurrenceService) CalculateOccurrencesFromDate(meeting *models.MeetingBase, fromDate time.Time, limit int) []models.Occurrence {
	if meeting == nil || limit <= 0 {
		return []models.Occurrence{}
	}

	// If the meeting has no recurrence, return just the original meeting if it's still relevant
	if meeting.Recurrence == nil {
		if s.isOccurrenceRelevant(meeting.StartTime, meeting.Duration, fromDate) {
			return []models.Occurrence{s.createOccurrence(meeting, meeting.StartTime)}
		}
		return []models.Occurrence{}
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

// isOccurrenceRelevant checks if an occurrence should be included based on:
// - If it starts on or after fromDate
// - If it's currently ongoing (started before fromDate but ends after it)
// - If it ended within the last 40 minutes before fromDate
func (s *OccurrenceService) isOccurrenceRelevant(occurrenceStart time.Time, duration int, fromDate time.Time) bool {
	// Buffer period after meeting ends (40 minutes)
	const bufferMinutes = 40

	// Calculate when the occurrence ends (including buffer)
	occurrenceEndWithBuffer := occurrenceStart.Add(time.Duration(duration+bufferMinutes) * time.Minute)

	// Consider the occurrence relevant if starts in the future or is ongoing/ended within the buffer period
	return !occurrenceStart.Before(fromDate) || occurrenceEndWithBuffer.After(fromDate)
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

		// Include this occurrence if it's still relevant (ongoing, future, or within buffer)
		if s.isOccurrenceRelevant(current, meeting.Duration, fromDate) {
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

	// Find the first valid occurrence date that's on or after startTime
	firstOccurrence := s.findFirstWeeklyOccurrence(startTime, weeklyDays, loc)

	// Start from the week containing the first valid occurrence
	weekStart := s.getStartOfWeek(firstOccurrence)

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

			// Include this occurrence if it's still relevant (ongoing, future, or within buffer)
			if s.isOccurrenceRelevant(occurrenceDate, meeting.Duration, fromDate) && len(occurrences) < limit {
				occurrences = append(occurrences, s.createOccurrence(meeting, occurrenceDate))
			}
		}

		weekCount++

		// Safety check to prevent infinite loops
		if weekCount > MaxWeeksCount {
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

		switch {
		case meeting.Recurrence.MonthlyDay > 0:
			// Monthly by day of month (e.g., 15th of every month)
			occurrenceDate = s.calculateMonthlyByDay(startTime, monthCount, meeting.Recurrence.MonthlyDay, meeting.Recurrence.RepeatInterval, loc)
		case meeting.Recurrence.MonthlyWeek != 0 && meeting.Recurrence.MonthlyWeekDay > 0:
			// Monthly by week and day (e.g., 2nd Tuesday of every month)
			occurrenceDate = s.calculateMonthlyByWeekDay(startTime, monthCount, meeting.Recurrence.MonthlyWeek, meeting.Recurrence.MonthlyWeekDay, meeting.Recurrence.RepeatInterval, loc)
		default:
			// Default to same day of month as start time
			occurrenceDate = s.calculateMonthlyByDay(startTime, monthCount, startTime.Day(), meeting.Recurrence.RepeatInterval, loc)
		}

		// Check if we've exceeded the end date
		if !endDate.IsZero() && !occurrenceDate.Before(endDate) {
			break
		}

		// Include this occurrence if it's still relevant (ongoing, future, or within buffer)
		if s.isOccurrenceRelevant(occurrenceDate, meeting.Duration, fromDate) {
			occurrences = append(occurrences, s.createOccurrence(meeting, occurrenceDate))
		}

		monthCount++
		current = occurrenceDate

		// Safety check to prevent infinite loops
		if monthCount > MaxMonthsCount {
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

	candidate := firstOccurrence.AddDate(0, 0, (week-1)*7)
	if candidate.Month() != targetMonth.Month() {
		lastOfMonth := time.Date(targetMonth.Year(), targetMonth.Month()+1, 0, startTime.Hour(), startTime.Minute(), startTime.Second(), startTime.Nanosecond(), loc)
		return s.findLastWeekdayOfMonth(lastOfMonth, targetWeekday)
	}
	return candidate
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
			goWeekday := day - 1
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

// findFirstWeeklyOccurrence finds the first occurrence date that's on or after startTime for the given weekly days
func (s *OccurrenceService) findFirstWeeklyOccurrence(startTime time.Time, weeklyDays []int, loc *time.Location) time.Time {
	// Start from the meeting start time
	current := startTime.In(loc)

	// Check up to 7 days ahead to find the first matching weekday
	for i := 0; i < 7; i++ {
		checkDate := current.AddDate(0, 0, i)
		weekday := int(checkDate.Weekday())

		// Check if this weekday is in our list of weekly days
		for _, targetDay := range weeklyDays {
			if weekday == targetDay {
				// Found a matching day, return the time with the original hour/minute/second
				return time.Date(
					checkDate.Year(), checkDate.Month(), checkDate.Day(),
					startTime.Hour(), startTime.Minute(), startTime.Second(), startTime.Nanosecond(),
					loc,
				)
			}
		}
	}

	// Fallback to start time if no matching day found (shouldn't happen)
	return startTime
}

// findLastWeekdayOfMonth finds the last occurrence of a weekday in a month
func (s *OccurrenceService) findLastWeekdayOfMonth(lastOfMonth time.Time, targetWeekday time.Weekday) time.Time {
	daysBack := (int(lastOfMonth.Weekday()) - int(targetWeekday) + 7) % 7
	return lastOfMonth.AddDate(0, 0, -daysBack)
}

// createOccurrence creates an occurrence model from meeting and start time.
// Note: Response counts are initialized to 0 and will be calculated later based on RSVPs.
func (s *OccurrenceService) createOccurrence(meeting *models.MeetingBase, startTime time.Time) models.Occurrence {
	var occurrenceInMeeting models.Occurrence
	for _, occurrence := range meeting.Occurrences {
		if occurrence.OccurrenceID == strconv.FormatInt(startTime.Unix(), 10) {
			occurrenceInMeeting = occurrence
		}
	}

	return models.Occurrence{
		OccurrenceID:       strconv.FormatInt(startTime.Unix(), 10),
		StartTime:          &startTime,
		Title:              meeting.Title,
		Description:        meeting.Description,
		Duration:           meeting.Duration,
		Recurrence:         nil, // Occurrences don't have recurrence patterns - reserved for future use
		RegistrantCount:    meeting.RegistrantCount,
		ResponseCountNo:    0,
		ResponseCountYes:   0,
		ResponseCountMaybe: 0,
		IsCancelled:        occurrenceInMeeting.IsCancelled,
	}
}

// Compile-time interface check
var _ domain.OccurrenceService = (*OccurrenceService)(nil)
