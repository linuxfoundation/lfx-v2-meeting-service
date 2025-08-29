// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package domain

import (
	"time"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
)

// OccurrenceService defines the interface for calculating meeting occurrences
// based on recurrence patterns.
type OccurrenceService interface {
	// CalculateOccurrences calculates occurrences for a meeting starting from the meeting's start time.
	// This is typically used when creating a new meeting to get all future occurrences.
	CalculateOccurrences(meeting *models.MeetingBase, limit int) []models.Occurrence

	// CalculateOccurrencesFromDate calculates occurrences for a meeting starting from a specific date.
	// This is typically used when retrieving a meeting to get upcoming occurrences from the current time.
	CalculateOccurrencesFromDate(meeting *models.MeetingBase, fromDate time.Time, limit int) []models.Occurrence
}
