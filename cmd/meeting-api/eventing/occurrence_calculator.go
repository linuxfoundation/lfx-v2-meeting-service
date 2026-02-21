// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/teambition/rrule-go"
)

// NOTE: This occurrence calculation logic is duplicated from the itx-service-zoom repository.
// Ideally, this should be imported as a shared package from itx-service-zoom, but that package
// is currently private. In the future, the occurrence calculation logic should be made public
// and exported from itx-service-zoom so it can be reused across services without duplication.
// See: github.com/linuxfoundation/itx-service-zoom (private)

// OccurrenceCalculator calculates meeting occurrences based on RRULE recurrence patterns
type OccurrenceCalculator struct {
	logger *slog.Logger
}

// NewOccurrenceCalculator creates a new occurrence calculator
func NewOccurrenceCalculator(logger *slog.Logger) *OccurrenceCalculator {
	return &OccurrenceCalculator{
		logger: logger,
	}
}

// Occurrence represents a single meeting occurrence
type Occurrence struct {
	OccurrenceID string
	StartTime    time.Time
	Duration     int
	Status       string
}

// RecurrenceInput contains the recurrence configuration
type RecurrenceInput struct {
	Type           int    // 1=Daily, 2=Weekly, 3=Monthly
	RepeatInterval int    // How often to repeat
	WeeklyDays     string // Days of week for weekly recurrence (e.g., "1,3,5" for Mon,Wed,Fri)
	MonthlyDay     int    // Day of month for monthly recurrence
	MonthlyWeek    int    // Week of month (-1=last, 1=first, 2=second, etc.)
	MonthlyWeekDay int    // Day of week for monthly week recurrence (1=Sun, 2=Mon, etc.)
	EndTimes       int    // Number of occurrences (0 = until end date)
	EndDateTime    string // End date/time for recurrence
}

// OccurrenceUpdate represents an update to a specific occurrence
type OccurrenceUpdate struct {
	StartTime time.Time
	Duration  int
}

// CalculateOccurrences generates meeting occurrences based on RRULE pattern
// Returns up to 100 occurrences to prevent excessive data
func (c *OccurrenceCalculator) CalculateOccurrences(
	startTime time.Time,
	duration int,
	recurrence *RecurrenceInput,
	cancelledOccurrences []string,
	updatedOccurrences map[string]OccurrenceUpdate,
) ([]Occurrence, error) {
	if recurrence == nil {
		// Single meeting, no recurrence
		return []Occurrence{
			{
				OccurrenceID: startTime.Format("20060102T150405Z"),
				StartTime:    startTime,
				Duration:     duration,
				Status:       "available",
			},
		}, nil
	}

	// Build RRULE from recurrence input
	rruleStr, err := c.buildRRule(startTime, recurrence)
	if err != nil {
		return nil, fmt.Errorf("failed to build rrule: %w", err)
	}

	c.logger.Debug("generated rrule", "rrule", rruleStr)

	// Parse RRULE
	rule, err := rrule.StrToRRule(rruleStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse rrule: %w", err)
	}

	// Generate occurrences (limit to 100)
	dtStart := startTime
	count := 100
	if recurrence.EndTimes > 0 && recurrence.EndTimes < 100 {
		count = recurrence.EndTimes
	}

	occurrenceTimes := rule.Between(dtStart, dtStart.AddDate(10, 0, 0), true) // Max 10 years ahead
	if len(occurrenceTimes) > count {
		occurrenceTimes = occurrenceTimes[:count]
	}

	// Convert to Occurrence objects
	occurrences := make([]Occurrence, 0, len(occurrenceTimes))
	cancelledMap := make(map[string]bool)
	for _, cancelled := range cancelledOccurrences {
		cancelledMap[cancelled] = true
	}

	for _, occTime := range occurrenceTimes {
		occurrenceID := occTime.Format("20060102T150405Z")

		// Skip cancelled occurrences
		if cancelledMap[occurrenceID] {
			continue
		}

		// Apply updates if present
		occStartTime := occTime
		occDuration := duration
		if update, exists := updatedOccurrences[occurrenceID]; exists {
			if !update.StartTime.IsZero() {
				occStartTime = update.StartTime
			}
			if update.Duration > 0 {
				occDuration = update.Duration
			}
		}

		occurrences = append(occurrences, Occurrence{
			OccurrenceID: occurrenceID,
			StartTime:    occStartTime,
			Duration:     occDuration,
			Status:       "available",
		})
	}

	c.logger.Debug("calculated occurrences",
		"total", len(occurrences),
		"cancelled", len(cancelledOccurrences),
		"updated", len(updatedOccurrences),
	)

	return occurrences, nil
}

// buildRRule constructs an RRULE string from recurrence input
func (c *OccurrenceCalculator) buildRRule(startTime time.Time, rec *RecurrenceInput) (string, error) {
	dtStart := startTime.Format("20060102T150405Z")
	rrule := fmt.Sprintf("DTSTART:%s\nRRULE:", dtStart)

	// Frequency
	var freq string
	switch rec.Type {
	case 1:
		freq = "DAILY"
	case 2:
		freq = "WEEKLY"
	case 3:
		freq = "MONTHLY"
	default:
		return "", fmt.Errorf("invalid recurrence type: %d", rec.Type)
	}
	rrule += "FREQ=" + freq

	// Interval
	if rec.RepeatInterval > 1 {
		rrule += fmt.Sprintf(";INTERVAL=%d", rec.RepeatInterval)
	}

	// Weekly days
	if rec.Type == 2 && rec.WeeklyDays != "" {
		days := c.convertWeeklyDays(rec.WeeklyDays)
		if days != "" {
			rrule += ";BYDAY=" + days
		}
	}

	// Monthly configuration
	if rec.Type == 3 {
		if rec.MonthlyWeek != 0 && rec.MonthlyWeekDay > 0 {
			// Monthly by week and day (e.g., "2nd Tuesday")
			weekPrefix := fmt.Sprintf("%d", rec.MonthlyWeek)
			dayOfWeek := c.convertDayOfWeek(rec.MonthlyWeekDay)
			rrule += fmt.Sprintf(";BYDAY=%s%s", weekPrefix, dayOfWeek)
		} else if rec.MonthlyDay > 0 {
			// Monthly by day of month
			rrule += fmt.Sprintf(";BYMONTHDAY=%d", rec.MonthlyDay)
		}
	}

	// End condition
	if rec.EndTimes > 0 {
		rrule += fmt.Sprintf(";COUNT=%d", rec.EndTimes)
	} else if rec.EndDateTime != "" {
		// Parse end date/time
		endTime, err := time.Parse(time.RFC3339, rec.EndDateTime)
		if err != nil {
			// Try alternative formats
			endTime, err = time.Parse("2006-01-02T15:04:05Z", rec.EndDateTime)
			if err != nil {
				endTime, err = time.Parse("2006-01-02 15:04:05", rec.EndDateTime)
				if err != nil {
					return "", fmt.Errorf("failed to parse end date/time: %w", err)
				}
			}
		}
		rrule += ";UNTIL=" + endTime.Format("20060102T150405Z")
	}

	return rrule, nil
}

// convertWeeklyDays converts comma-separated day numbers to RRULE BYDAY format
// Input: "1,3,5" (1=Sunday in Zoom API)
// Output: "SU,TU,TH"
func (c *OccurrenceCalculator) convertWeeklyDays(weeklyDays string) string {
	dayMap := map[string]string{
		"1": "SU", // Sunday
		"2": "MO", // Monday
		"3": "TU", // Tuesday
		"4": "WE", // Wednesday
		"5": "TH", // Thursday
		"6": "FR", // Friday
		"7": "SA", // Saturday
	}

	days := ""
	for i, day := range weeklyDays {
		if i > 0 && day != ',' {
			days += ","
		}
		if day != ',' {
			if rruleDay, ok := dayMap[string(day)]; ok {
				days += rruleDay
			}
		}
	}
	return days
}

// convertDayOfWeek converts Zoom day number to RRULE day abbreviation
// 1=Sunday, 2=Monday, etc.
func (c *OccurrenceCalculator) convertDayOfWeek(day int) string {
	dayMap := map[int]string{
		1: "SU",
		2: "MO",
		3: "TU",
		4: "WE",
		5: "TH",
		6: "FR",
		7: "SA",
	}
	return dayMap[day]
}

// ApplyAllFollowingUpdates applies updates to all occurrences from a given occurrence onwards
// This is used when a user updates "this and all following" occurrences
func (c *OccurrenceCalculator) ApplyAllFollowingUpdates(
	occurrences []Occurrence,
	fromOccurrenceID string,
	startTime time.Time,
	duration int,
) []Occurrence {
	foundStart := false
	updated := make([]Occurrence, 0, len(occurrences))

	for _, occ := range occurrences {
		if !foundStart && occ.OccurrenceID == fromOccurrenceID {
			foundStart = true
		}

		if foundStart {
			// Apply updates to this and all following
			if !startTime.IsZero() {
				occ.StartTime = startTime
			}
			if duration > 0 {
				occ.Duration = duration
			}
		}

		updated = append(updated, occ)
	}

	return updated
}
