// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package mocks

import (
	"time"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/stretchr/testify/mock"
)

// MockOccurrenceService is a mock implementation of domain.OccurrenceService
type MockOccurrenceService struct {
	mock.Mock
}

func (m *MockOccurrenceService) CalculateOccurrences(meeting *models.MeetingBase, limit int) []models.Occurrence {
	args := m.Called(meeting, limit)
	return args.Get(0).([]models.Occurrence)
}

func (m *MockOccurrenceService) CalculateOccurrencesFromDate(meeting *models.MeetingBase, fromDate time.Time, limit int) []models.Occurrence {
	args := m.Called(meeting, fromDate, limit)
	return args.Get(0).([]models.Occurrence)
}

func (m *MockOccurrenceService) GetSeriesEndDate(meeting *models.MeetingBase) *time.Time {
	args := m.Called(meeting)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*time.Time)
}

// Ensure MockOccurrenceService implements domain.OccurrenceService
var _ domain.OccurrenceService = (*MockOccurrenceService)(nil)
