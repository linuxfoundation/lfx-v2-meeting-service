// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"strconv"
	"testing"
	"time"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOccurrenceService_CalculateOccurrences(t *testing.T) {
	service := NewOccurrenceService()

	tests := []struct {
		name          string
		meeting       *models.MeetingBase
		limit         int
		expectedCount int
		validateDates []time.Time
		validateFirst bool // Only validate first occurrence
	}{
		{
			name:          "nil meeting",
			meeting:       nil,
			limit:         10,
			expectedCount: 0,
		},
		{
			name: "meeting without recurrence",
			meeting: &models.MeetingBase{
				UID:         "test-meeting",
				Title:       "Test Meeting",
				Description: "Test Description",
				StartTime:   time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC),
				Duration:    60,
				Timezone:    "UTC",
			},
			limit:         10,
			expectedCount: 1,
			validateDates: []time.Time{
				time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC),
			},
		},
		{
			name: "daily recurrence every day",
			meeting: &models.MeetingBase{
				Title:     "Daily Meeting",
				StartTime: time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC),
				Duration:  30,
				Timezone:  "UTC",
				Recurrence: &models.Recurrence{
					Type:           1, // Daily
					RepeatInterval: 1,
				},
			},
			limit:         5,
			expectedCount: 5,
			validateDates: []time.Time{
				time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC),
				time.Date(2024, 6, 2, 10, 0, 0, 0, time.UTC),
				time.Date(2024, 6, 3, 10, 0, 0, 0, time.UTC),
				time.Date(2024, 6, 4, 10, 0, 0, 0, time.UTC),
				time.Date(2024, 6, 5, 10, 0, 0, 0, time.UTC),
			},
		},
		{
			name: "daily recurrence every 3 days",
			meeting: &models.MeetingBase{
				StartTime: time.Date(2024, 6, 1, 14, 30, 0, 0, time.UTC),
				Duration:  45,
				Timezone:  "UTC",
				Recurrence: &models.Recurrence{
					Type:           1, // Daily
					RepeatInterval: 3,
				},
			},
			limit:         3,
			expectedCount: 3,
			validateDates: []time.Time{
				time.Date(2024, 6, 1, 14, 30, 0, 0, time.UTC),
				time.Date(2024, 6, 4, 14, 30, 0, 0, time.UTC),
				time.Date(2024, 6, 7, 14, 30, 0, 0, time.UTC),
			},
		},
		{
			name: "daily with end times",
			meeting: &models.MeetingBase{
				StartTime: time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC),
				Duration:  30,
				Timezone:  "UTC",
				Recurrence: &models.Recurrence{
					Type:           1,
					RepeatInterval: 1,
					EndTimes:       3,
				},
			},
			limit:         10,
			expectedCount: 3, // Limited by EndTimes
		},
		{
			name: "weekly on same day",
			meeting: &models.MeetingBase{
				StartTime: time.Date(2024, 6, 3, 10, 0, 0, 0, time.UTC), // Monday
				Duration:  60,
				Timezone:  "UTC",
				Recurrence: &models.Recurrence{
					Type:           2, // Weekly
					RepeatInterval: 1,
				},
			},
			limit:         3,
			expectedCount: 3,
			validateDates: []time.Time{
				time.Date(2024, 6, 3, 10, 0, 0, 0, time.UTC),  // Monday
				time.Date(2024, 6, 10, 10, 0, 0, 0, time.UTC), // Next Monday
				time.Date(2024, 6, 17, 10, 0, 0, 0, time.UTC), // Following Monday
			},
		},
		{
			name: "weekly on multiple days",
			meeting: &models.MeetingBase{
				StartTime: time.Date(2024, 6, 3, 9, 0, 0, 0, time.UTC), // Monday June 3, 2024
				Duration:  30,
				Timezone:  "UTC",
				Recurrence: &models.Recurrence{
					Type:           2, // Weekly
					RepeatInterval: 1,
					WeeklyDays:     "2,4,6", // Monday(2), Wednesday(4), Friday(6)
				},
			},
			limit:         6,
			expectedCount: 6,
			validateDates: []time.Time{
				time.Date(2024, 6, 3, 9, 0, 0, 0, time.UTC),  // Monday
				time.Date(2024, 6, 5, 9, 0, 0, 0, time.UTC),  // Wednesday
				time.Date(2024, 6, 7, 9, 0, 0, 0, time.UTC),  // Friday
				time.Date(2024, 6, 10, 9, 0, 0, 0, time.UTC), // Next Monday
				time.Date(2024, 6, 12, 9, 0, 0, 0, time.UTC), // Next Wednesday
				time.Date(2024, 6, 14, 9, 0, 0, 0, time.UTC), // Next Friday
			},
		},
		{
			name: "weekly every 2 weeks",
			meeting: &models.MeetingBase{
				StartTime: time.Date(2024, 6, 3, 10, 0, 0, 0, time.UTC), // Monday
				Duration:  60,
				Timezone:  "UTC",
				Recurrence: &models.Recurrence{
					Type:           2,
					RepeatInterval: 2,
				},
			},
			limit:         3,
			expectedCount: 3,
			validateDates: []time.Time{
				time.Date(2024, 6, 3, 10, 0, 0, 0, time.UTC),  // Monday
				time.Date(2024, 6, 17, 10, 0, 0, 0, time.UTC), // 2 weeks later
				time.Date(2024, 7, 1, 10, 0, 0, 0, time.UTC),  // 4 weeks later
			},
		},
		{
			name: "weekly start Thursday recur Monday",
			meeting: &models.MeetingBase{
				StartTime: time.Date(2026, 1, 2, 15, 4, 5, 0, time.UTC), // Thursday January 2nd, 2026
				Duration:  10,
				Timezone:  "UTC",
				Recurrence: &models.Recurrence{
					Type:           2, // Weekly
					RepeatInterval: 1,
					WeeklyDays:     "2", // Monday (2 in 1-7 system where 1=Sunday)
					EndTimes:       5,
				},
			},
			limit:         5,
			expectedCount: 5,
			validateDates: []time.Time{
				time.Date(2026, 1, 5, 15, 4, 5, 0, time.UTC),  // Monday January 5th (first Monday after Thursday start)
				time.Date(2026, 1, 12, 15, 4, 5, 0, time.UTC), // Monday January 12th
				time.Date(2026, 1, 19, 15, 4, 5, 0, time.UTC), // Monday January 19th
				time.Date(2026, 1, 26, 15, 4, 5, 0, time.UTC), // Monday January 26th
				time.Date(2026, 2, 2, 15, 4, 5, 0, time.UTC),  // Monday February 2nd
			},
		},
		{
			name: "monthly by day",
			meeting: &models.MeetingBase{
				StartTime: time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC),
				Duration:  60,
				Timezone:  "UTC",
				Recurrence: &models.Recurrence{
					Type:           3, // Monthly
					RepeatInterval: 1,
					MonthlyDay:     15,
				},
			},
			limit:         3,
			expectedCount: 3,
			validateDates: []time.Time{
				time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC),
				time.Date(2024, 7, 15, 10, 0, 0, 0, time.UTC),
				time.Date(2024, 8, 15, 10, 0, 0, 0, time.UTC),
			},
		},
		{
			name: "monthly by day - handle February",
			meeting: &models.MeetingBase{
				StartTime: time.Date(2024, 1, 31, 10, 0, 0, 0, time.UTC),
				Duration:  60,
				Timezone:  "UTC",
				Recurrence: &models.Recurrence{
					Type:           3,
					RepeatInterval: 1,
					MonthlyDay:     31,
				},
			},
			limit:         3,
			expectedCount: 3,
			validateDates: []time.Time{
				time.Date(2024, 1, 31, 10, 0, 0, 0, time.UTC),
				time.Date(2024, 2, 29, 10, 0, 0, 0, time.UTC), // Last day of February (leap year)
				time.Date(2024, 3, 31, 10, 0, 0, 0, time.UTC),
			},
		},
		{
			name: "monthly by week and day - 2nd Tuesday",
			meeting: &models.MeetingBase{
				StartTime: time.Date(2024, 6, 11, 14, 0, 0, 0, time.UTC), // 2nd Tuesday of June
				Duration:  90,
				Timezone:  "UTC",
				Recurrence: &models.Recurrence{
					Type:           3,
					RepeatInterval: 1,
					MonthlyWeek:    2, // 2nd week
					MonthlyWeekDay: 3, // Tuesday
				},
			},
			limit:         3,
			expectedCount: 3,
			validateDates: []time.Time{
				time.Date(2024, 6, 11, 14, 0, 0, 0, time.UTC), // 2nd Tuesday of June
				time.Date(2024, 7, 9, 14, 0, 0, 0, time.UTC),  // 2nd Tuesday of July
				time.Date(2024, 8, 13, 14, 0, 0, 0, time.UTC), // 2nd Tuesday of August
			},
		},
		{
			name: "monthly by week and day - last Friday",
			meeting: &models.MeetingBase{
				StartTime: time.Date(2024, 6, 28, 16, 0, 0, 0, time.UTC), // Last Friday of June
				Duration:  60,
				Timezone:  "UTC",
				Recurrence: &models.Recurrence{
					Type:           3,
					RepeatInterval: 1,
					MonthlyWeek:    -1, // Last week
					MonthlyWeekDay: 6,  // Friday
				},
			},
			limit:         3,
			expectedCount: 3,
			validateDates: []time.Time{
				time.Date(2024, 6, 28, 16, 0, 0, 0, time.UTC), // Last Friday of June
				time.Date(2024, 7, 26, 16, 0, 0, 0, time.UTC), // Last Friday of July
				time.Date(2024, 8, 30, 16, 0, 0, 0, time.UTC), // Last Friday of August
			},
		},
		{
			name: "monthly every 3 months",
			meeting: &models.MeetingBase{
				StartTime: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
				Duration:  60,
				Timezone:  "UTC",
				Recurrence: &models.Recurrence{
					Type:           3,
					RepeatInterval: 3,
					MonthlyDay:     15,
				},
			},
			limit:         3,
			expectedCount: 3,
			validateDates: []time.Time{
				time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
				time.Date(2024, 4, 15, 10, 0, 0, 0, time.UTC),
				time.Date(2024, 7, 15, 10, 0, 0, 0, time.UTC),
			},
		},
		{
			name: "end by date",
			meeting: &models.MeetingBase{
				StartTime: time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC),
				Duration:  30,
				Timezone:  "UTC",
				Recurrence: &models.Recurrence{
					Type:           1, // Daily
					RepeatInterval: 1,
					EndDateTime:    utils.TimePtr(time.Date(2024, 6, 10, 10, 0, 0, 0, time.UTC)),
				},
			},
			limit:         20,
			expectedCount: 9, // June 1-9
			validateFirst: true,
			validateDates: []time.Time{
				time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC),
			},
		},
		{
			name: "end by count",
			meeting: &models.MeetingBase{
				StartTime: time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC),
				Duration:  30,
				Timezone:  "UTC",
				Recurrence: &models.Recurrence{
					Type:           1,
					RepeatInterval: 1,
					EndTimes:       5,
				},
			},
			limit:         20,
			expectedCount: 5,
		},
		{
			name: "PST timezone",
			meeting: &models.MeetingBase{
				StartTime: time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC), // Store as UTC
				Duration:  60,
				Timezone:  "America/Los_Angeles",
				Recurrence: &models.Recurrence{
					Type:           1,
					RepeatInterval: 1,
				},
			},
			limit:         2,
			expectedCount: 2,
			validateDates: []time.Time{
				time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC),
				time.Date(2024, 6, 2, 10, 0, 0, 0, time.UTC),
			},
		},
		{
			name: "invalid timezone falls back to UTC",
			meeting: &models.MeetingBase{
				Title:     "Test Meeting",
				StartTime: time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC),
				Duration:  30,
				Timezone:  "Invalid/Timezone",
				Recurrence: &models.Recurrence{
					Type:           1,
					RepeatInterval: 1,
				},
			},
			limit:         1,
			expectedCount: 1,
		},
		{
			name: "unknown recurrence type",
			meeting: &models.MeetingBase{
				StartTime: time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC),
				Duration:  30,
				Timezone:  "UTC",
				Recurrence: &models.Recurrence{
					Type:           99, // Invalid type
					RepeatInterval: 1,
				},
			},
			limit:         5,
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			occurrences := service.CalculateOccurrences(tt.meeting, tt.limit)
			require.Len(t, occurrences, tt.expectedCount)

			if tt.validateDates != nil {
				if tt.validateFirst {
					// Only validate first occurrence
					if len(occurrences) > 0 && len(tt.validateDates) > 0 {
						assert.True(t, occurrences[0].StartTime.Equal(tt.validateDates[0]),
							"Expected first occurrence %s, got %s", tt.validateDates[0], occurrences[0].StartTime)
					}
				} else {
					// Validate all provided dates
					for i, expectedDate := range tt.validateDates {
						if i < len(occurrences) {
							assert.True(t, occurrences[i].StartTime.Equal(expectedDate),
								"Expected %s, got %s for occurrence %d", expectedDate, occurrences[i].StartTime, i)
						}
					}
				}
			}

			// Validate occurrence properties if we have occurrences
			if len(occurrences) > 0 && tt.meeting != nil {
				occ := occurrences[0]
				assert.Equal(t, tt.meeting.Title, occ.Title)
				assert.Equal(t, tt.meeting.Description, occ.Description)
				assert.Equal(t, tt.meeting.Duration, occ.Duration)
				assert.Equal(t, false, occ.IsCancelled)
			}
		})
	}
}

func TestOccurrenceService_CalculateOccurrencesFromDate(t *testing.T) {
	service := NewOccurrenceService()

	tests := []struct {
		name          string
		meeting       *models.MeetingBase
		fromDate      time.Time
		limit         int
		expectedCount int
		validateDates []time.Time
	}{
		{
			name:          "nil meeting",
			meeting:       nil,
			fromDate:      time.Now(),
			limit:         10,
			expectedCount: 0,
		},
		{
			name: "zero limit",
			meeting: &models.MeetingBase{
				StartTime: time.Now(),
			},
			fromDate:      time.Now(),
			limit:         0,
			expectedCount: 0,
		},
		{
			name: "single meeting in the past",
			meeting: &models.MeetingBase{
				StartTime: time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC),
				Title:     "Past Meeting",
				Duration:  60,
				Timezone:  "UTC",
			},
			fromDate:      time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC),
			limit:         10,
			expectedCount: 0,
		},
		{
			name: "filter past occurrences",
			meeting: &models.MeetingBase{
				StartTime: time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC),
				Duration:  30,
				Timezone:  "UTC",
				Recurrence: &models.Recurrence{
					Type:           1,
					RepeatInterval: 1,
				},
			},
			fromDate:      time.Date(2024, 6, 5, 0, 0, 0, 0, time.UTC),
			limit:         3,
			expectedCount: 3,
			validateDates: []time.Time{
				time.Date(2024, 6, 5, 10, 0, 0, 0, time.UTC),
				time.Date(2024, 6, 6, 10, 0, 0, 0, time.UTC),
				time.Date(2024, 6, 7, 10, 0, 0, 0, time.UTC),
			},
		},
		{
			name: "weekly recurrence from future date",
			meeting: &models.MeetingBase{
				StartTime: time.Date(2024, 6, 3, 10, 0, 0, 0, time.UTC), // Monday June 3
				Duration:  60,
				Timezone:  "UTC",
				Recurrence: &models.Recurrence{
					Type:           2, // Weekly
					RepeatInterval: 1,
					WeeklyDays:     "2,4", // Monday(2), Wednesday(4)
				},
			},
			fromDate:      time.Date(2024, 6, 11, 0, 0, 0, 0, time.UTC), // Start from Tuesday June 11
			limit:         4,
			expectedCount: 4,
			validateDates: []time.Time{
				time.Date(2024, 6, 12, 10, 0, 0, 0, time.UTC), // Wednesday June 12
				time.Date(2024, 6, 17, 10, 0, 0, 0, time.UTC), // Monday June 17
				time.Date(2024, 6, 19, 10, 0, 0, 0, time.UTC), // Wednesday June 19
				time.Date(2024, 6, 24, 10, 0, 0, 0, time.UTC), // Monday June 24
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			occurrences := service.CalculateOccurrencesFromDate(tt.meeting, tt.fromDate, tt.limit)
			require.Len(t, occurrences, tt.expectedCount)

			if tt.validateDates != nil {
				for i, expectedDate := range tt.validateDates {
					if i < len(occurrences) {
						assert.True(t, occurrences[i].StartTime.Equal(expectedDate),
							"Expected %s, got %s for occurrence %d", expectedDate, occurrences[i].StartTime, i)
					}
				}
			}
		})
	}
}

func TestOccurrenceService_parseWeeklyDays(t *testing.T) {
	service := NewOccurrenceService()

	tests := []struct {
		name     string
		input    string
		expected []int
	}{
		{
			name:     "empty string",
			input:    "",
			expected: []int{},
		},
		{
			name:     "single day Sunday",
			input:    "1",
			expected: []int{0}, // Sunday = 0 in Go
		},
		{
			name:     "multiple days",
			input:    "1,3,5",
			expected: []int{0, 2, 4}, // Sunday=0, Tuesday=2, Thursday=4
		},
		{
			name:     "Saturday as 7",
			input:    "7",
			expected: []int{6}, // Saturday = 6 in Go
		},
		{
			name:     "invalid day filtered out",
			input:    "1,3,8",
			expected: []int{0, 2}, // Sunday=0, Tuesday=2, 8 is invalid
		},
		{
			name:     "whitespace handling",
			input:    "  2 , 4  ",
			expected: []int{1, 3}, // Monday=1, Wednesday=3
		},
		{
			name:     "invalid format",
			input:    "abc,2,def",
			expected: []int{1}, // Monday=1
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.parseWeeklyDays(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestOccurrenceService_ValidateFutureOccurrenceID(t *testing.T) {
	service := NewOccurrenceService()
	now := time.Now()
	futureTime := now.Add(24 * time.Hour)
	pastTime := now.Add(-24 * time.Hour)

	// Test meeting with future occurrences
	futureMeeting := &models.MeetingBase{
		UID:       "meeting-future",
		Title:     "Future Meeting",
		StartTime: futureTime,
		Duration:  60,
		Timezone:  "UTC",
	}

	// Test meeting with past occurrences
	pastMeeting := &models.MeetingBase{
		UID:       "meeting-past",
		Title:     "Past Meeting",
		StartTime: pastTime,
		Duration:  60,
		Timezone:  "UTC",
	}

	// Generate valid occurrence IDs based on the actual calculation
	futureOccurrenceID := strconv.FormatInt(futureTime.Unix(), 10)
	pastOccurrenceID := strconv.FormatInt(pastTime.Unix(), 10)

	tests := []struct {
		name                  string
		meeting               *models.MeetingBase
		occurrenceID          string
		maxOccurrencesToCheck int
		expectError           bool
		errorMessage          string
	}{
		{
			name:                  "nil meeting",
			meeting:               nil,
			occurrenceID:          "test-occurrence",
			maxOccurrencesToCheck: 10,
			expectError:           true,
			errorMessage:          "meeting and occurrence ID are required",
		},
		{
			name:                  "empty occurrence ID",
			meeting:               futureMeeting,
			occurrenceID:          "",
			maxOccurrencesToCheck: 10,
			expectError:           true,
			errorMessage:          "meeting and occurrence ID are required",
		},
		{
			name:                  "zero max occurrences",
			meeting:               futureMeeting,
			occurrenceID:          "test-occurrence",
			maxOccurrencesToCheck: 0,
			expectError:           true,
			errorMessage:          "maxOccurrencesToCheck must be greater than 0",
		},
		{
			name:                  "negative max occurrences",
			meeting:               futureMeeting,
			occurrenceID:          "test-occurrence",
			maxOccurrencesToCheck: -1,
			expectError:           true,
			errorMessage:          "maxOccurrencesToCheck must be greater than 0",
		},
		{
			name:                  "valid future occurrence",
			meeting:               futureMeeting,
			occurrenceID:          futureOccurrenceID,
			maxOccurrencesToCheck: 10,
			expectError:           false,
		},
		{
			name:                  "past occurrence should fail",
			meeting:               pastMeeting,
			occurrenceID:          pastOccurrenceID,
			maxOccurrencesToCheck: 10,
			expectError:           true,
			errorMessage:          "cannot register for past occurrences",
		},
		{
			name:                  "nonexistent occurrence ID",
			meeting:               futureMeeting,
			occurrenceID:          "nonexistent-occurrence",
			maxOccurrencesToCheck: 10,
			expectError:           true,
			errorMessage:          "occurrence not found for this meeting",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.ValidateFutureOccurrenceID(tt.meeting, tt.occurrenceID, tt.maxOccurrencesToCheck)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMessage != "" {
					assert.Contains(t, err.Error(), tt.errorMessage)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
