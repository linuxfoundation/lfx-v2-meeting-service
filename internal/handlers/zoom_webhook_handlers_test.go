// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// This file contains unit tests for Zoom webhook event handlers.
// These tests verify:
// 1. Proper parsing of webhook event messages from NATS
// 2. Correct conversion of generic webhook events to typed payload structs
// 3. Accurate parsing of participant names from user display names
// 4. Error handling for invalid event types
//
// The tests demonstrate expected behavior for:
// - meeting.started events (PastMeeting creation with participant records)
// - meeting.ended events (session end time updates)
// - meeting.participant_joined events (attendance tracking and session creation)
// - meeting.participant_left events (session completion with leave time/reason)

package handlers

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/mocks"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	serviceMocks "github.com/linuxfoundation/lfx-v2-meeting-service/internal/service/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseZoomWebhookEvent tests the webhook event parsing
func TestParseZoomWebhookEvent(t *testing.T) {
	ctx := context.Background()
	handler := &ZoomWebhookHandler{}

	tests := []struct {
		name        string
		input       models.ZoomWebhookEventMessage
		shouldError bool
	}{
		{
			name: "valid meeting.started event",
			input: models.ZoomWebhookEventMessage{
				EventType: "meeting.started",
				EventTS:   time.Now().Unix(),
				Payload: map[string]interface{}{
					"object": map[string]interface{}{
						"uuid":       "test-zoom-uuid",
						"id":         "123456789",
						"host_id":    "host-123",
						"topic":      "Test Meeting",
						"type":       2,
						"start_time": time.Now(),
						"timezone":   "UTC",
						"duration":   60,
					},
				},
			},
			shouldError: false,
		},
		{
			name: "valid meeting.ended event",
			input: models.ZoomWebhookEventMessage{
				EventType: "meeting.ended",
				EventTS:   time.Now().Unix(),
				Payload: map[string]interface{}{
					"object": map[string]interface{}{
						"uuid":       "test-zoom-uuid",
						"id":         "123456789",
						"host_id":    "host-123",
						"topic":      "Test Meeting",
						"type":       2,
						"start_time": time.Now().Add(-1 * time.Hour),
						"end_time":   time.Now(),
						"timezone":   "UTC",
						"duration":   60,
					},
				},
			},
			shouldError: false,
		},
		{
			name: "valid participant.joined event",
			input: models.ZoomWebhookEventMessage{
				EventType: "meeting.participant_joined",
				EventTS:   time.Now().Unix(),
				Payload: map[string]interface{}{
					"object": map[string]interface{}{
						"uuid":       "test-zoom-uuid",
						"id":         "123456789",
						"host_id":    "host-123",
						"topic":      "Test Meeting",
						"type":       2,
						"start_time": time.Now().Add(-30 * time.Minute),
						"timezone":   "UTC",
						"participant": map[string]interface{}{
							"user_id":             "user-123",
							"user_name":           "John Doe",
							"id":                  "participant-session-123",
							"join_time":           time.Now(),
							"email":               "user@example.com",
							"participant_user_id": "participant-user-123",
						},
					},
				},
			},
			shouldError: false,
		},
		{
			name: "valid participant.left event",
			input: models.ZoomWebhookEventMessage{
				EventType: "meeting.participant_left",
				EventTS:   time.Now().Unix(),
				Payload: map[string]interface{}{
					"object": map[string]interface{}{
						"uuid":       "test-zoom-uuid",
						"id":         "123456789",
						"host_id":    "host-123",
						"topic":      "Test Meeting",
						"type":       2,
						"start_time": time.Now().Add(-1 * time.Hour),
						"timezone":   "UTC",
						"participant": map[string]interface{}{
							"user_id":             "user-123",
							"user_name":           "John Doe",
							"id":                  "participant-session-123",
							"leave_time":          time.Now(),
							"duration":            1800,
							"email":               "user@example.com",
							"participant_user_id": "participant-user-123",
							"leave_reason":        "left normally",
						},
					},
				},
			},
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Marshal the input
			msgData, err := json.Marshal(tt.input)
			require.NoError(t, err)

			// Create a mock message
			mockMsg := mocks.NewMockMessage(msgData, "")

			// Parse the event
			event, err := handler.parseZoomWebhookEvent(ctx, mockMsg)

			if tt.shouldError {
				assert.Error(t, err)
				assert.Nil(t, event)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, event)
				assert.Equal(t, tt.input.EventType, event.EventType)
				assert.Equal(t, tt.input.EventTS, event.EventTS)
			}
		})
	}
}

// TestZoomPayloadConversions tests the conversion methods for typed payloads
func TestZoomPayloadConversions(t *testing.T) {

	t.Run("ToMeetingStartedPayload", func(t *testing.T) {
		startTime := time.Now()
		event := &models.ZoomWebhookEventMessage{
			EventType: "meeting.started",
			EventTS:   time.Now().Unix(),
			Payload: map[string]interface{}{
				"object": map[string]interface{}{
					"uuid":       "test-zoom-uuid",
					"id":         "123456789",
					"host_id":    "host-123",
					"topic":      "Test Meeting",
					"type":       2,
					"start_time": startTime,
					"timezone":   "UTC",
					"duration":   60,
				},
			},
		}

		payload, err := event.ToMeetingStartedPayload()
		assert.NoError(t, err)
		assert.NotNil(t, payload)
		assert.Equal(t, "test-zoom-uuid", payload.Object.UUID)
		assert.Equal(t, "123456789", payload.Object.ID)
		assert.Equal(t, "Test Meeting", payload.Object.Topic)
		assert.WithinDuration(t, startTime, payload.Object.StartTime, time.Second)
	})

	t.Run("ToMeetingEndedPayload", func(t *testing.T) {
		startTime := time.Now().Add(-1 * time.Hour)
		endTime := time.Now()
		event := &models.ZoomWebhookEventMessage{
			EventType: "meeting.ended",
			EventTS:   time.Now().Unix(),
			Payload: map[string]interface{}{
				"object": map[string]interface{}{
					"uuid":       "test-zoom-uuid",
					"id":         "123456789",
					"host_id":    "host-123",
					"topic":      "Test Meeting",
					"type":       2,
					"start_time": startTime,
					"end_time":   endTime,
					"timezone":   "UTC",
					"duration":   60,
				},
			},
		}

		payload, err := event.ToMeetingEndedPayload()
		assert.NoError(t, err)
		assert.NotNil(t, payload)
		assert.Equal(t, "test-zoom-uuid", payload.Object.UUID)
		assert.Equal(t, "123456789", payload.Object.ID)
		assert.WithinDuration(t, startTime, payload.Object.StartTime, time.Second)
		assert.WithinDuration(t, endTime, payload.Object.EndTime, time.Second)
	})

	t.Run("ToParticipantJoinedPayload", func(t *testing.T) {
		joinTime := time.Now()
		event := &models.ZoomWebhookEventMessage{
			EventType: "meeting.participant_joined",
			EventTS:   time.Now().Unix(),
			Payload: map[string]interface{}{
				"object": map[string]interface{}{
					"uuid":       "test-zoom-uuid",
					"id":         "123456789",
					"host_id":    "host-123",
					"topic":      "Test Meeting",
					"type":       2,
					"start_time": time.Now().Add(-30 * time.Minute),
					"timezone":   "UTC",
					"participant": map[string]interface{}{
						"user_id":             "user-123",
						"user_name":           "John Doe",
						"id":                  "participant-session-123",
						"join_time":           joinTime,
						"email":               "user@example.com",
						"participant_user_id": "participant-user-123",
					},
				},
			},
		}

		payload, err := event.ToParticipantJoinedPayload()
		assert.NoError(t, err)
		assert.NotNil(t, payload)
		assert.Equal(t, "test-zoom-uuid", payload.Object.UUID)
		assert.Equal(t, "123456789", payload.Object.ID)
		assert.Equal(t, "user@example.com", payload.Object.Participant.Email)
		assert.Equal(t, "John Doe", payload.Object.Participant.UserName)
		assert.WithinDuration(t, joinTime, payload.Object.Participant.JoinTime, time.Second)
	})

	t.Run("ToParticipantLeftPayload", func(t *testing.T) {
		leaveTime := time.Now()
		event := &models.ZoomWebhookEventMessage{
			EventType: "meeting.participant_left",
			EventTS:   time.Now().Unix(),
			Payload: map[string]interface{}{
				"object": map[string]interface{}{
					"uuid":       "test-zoom-uuid",
					"id":         "123456789",
					"host_id":    "host-123",
					"topic":      "Test Meeting",
					"type":       2,
					"start_time": time.Now().Add(-1 * time.Hour),
					"timezone":   "UTC",
					"participant": map[string]interface{}{
						"user_id":             "user-123",
						"user_name":           "John Doe",
						"id":                  "participant-session-123",
						"leave_time":          leaveTime,
						"duration":            1800,
						"email":               "user@example.com",
						"participant_user_id": "participant-user-123",
						"leave_reason":        "left normally",
					},
				},
			},
		}

		payload, err := event.ToParticipantLeftPayload()
		assert.NoError(t, err)
		assert.NotNil(t, payload)
		assert.Equal(t, "test-zoom-uuid", payload.Object.UUID)
		assert.Equal(t, "123456789", payload.Object.ID)
		assert.Equal(t, "user@example.com", payload.Object.Participant.Email)
		assert.WithinDuration(t, leaveTime, payload.Object.Participant.LeaveTime, time.Second)
		assert.Equal(t, 1800, payload.Object.Participant.Duration)
		assert.Equal(t, "left normally", payload.Object.Participant.LeaveReason)
	})

	t.Run("Wrong event type returns error", func(t *testing.T) {
		event := &models.ZoomWebhookEventMessage{
			EventType: "meeting.ended",
			EventTS:   time.Now().Unix(),
			Payload:   map[string]interface{}{},
		}

		// Try to convert to wrong type
		_, err := event.ToMeetingStartedPayload()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid event type")
	})
}

// TestParseNameFromUserName tests the name parsing helper
func TestParseNameFromUserName(t *testing.T) {
	tests := []struct {
		input     string
		firstName string
		lastName  string
	}{
		{"John Doe", "John", "Doe"},
		{"Jane", "Jane", ""},
		{"Mary Jane Watson", "Mary", "Jane Watson"},
		{"", "", ""},
		{"  John  ", "John", ""},
		{"John  Doe  Jr.", "John", "Doe Jr."},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			firstName, lastName := parseNameFromUserName(tt.input)
			assert.Equal(t, tt.firstName, firstName)
			assert.Equal(t, tt.lastName, lastName)
		})
	}
}

// TestFindClosestOccurrenceID tests the occurrence ID calculation logic
func TestFindClosestOccurrenceID(t *testing.T) {
	scheduledTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	actualTime := time.Date(2024, 1, 15, 10, 5, 0, 0, time.UTC) // 5 minutes later

	tests := []struct {
		name              string
		meeting           *models.MeetingBase
		actualStart       time.Time
		expectedID        string
		description       string
		mockOccurrences   []models.Occurrence
		expectServiceCall bool
	}{
		{
			name: "non-recurring meeting returns scheduled start time unix timestamp",
			meeting: &models.MeetingBase{
				UID:        "meeting-1",
				StartTime:  scheduledTime,
				Recurrence: nil,
			},
			actualStart:       actualTime,
			expectedID:        "1705312800", // Unix timestamp of scheduledTime (not actualTime)
			description:       "Non-recurring meetings should use scheduled start time unix timestamp as occurrence ID",
			expectServiceCall: false, // No service call for non-recurring meetings
		},
		{
			name: "recurring meeting without calculated occurrences returns scheduled start time unix timestamp",
			meeting: &models.MeetingBase{
				UID:       "meeting-2",
				StartTime: scheduledTime,
				Recurrence: &models.Recurrence{
					Type:           1,
					RepeatInterval: 1,
				},
			},
			actualStart:       actualTime,
			expectedID:        "1705312800",
			description:       "Recurring meetings without calculated occurrences should fall back to scheduled start time unix timestamp",
			mockOccurrences:   []models.Occurrence{}, // Empty slice from service
			expectServiceCall: true,
		},
		{
			name: "recurring meeting finds exact matching occurrence",
			meeting: &models.MeetingBase{
				UID:       "meeting-3",
				StartTime: scheduledTime,
				Recurrence: &models.Recurrence{
					Type:           1,
					RepeatInterval: 1,
				},
			},
			actualStart: scheduledTime, // Exactly matches the occurrence start time
			expectedID:  "1705312800",  // Unix timestamp of scheduledTime
			description: "Should return the occurrence ID that exactly matches the actual start time",
			mockOccurrences: []models.Occurrence{
				{
					OccurrenceID: "1705312800", // Unix timestamp of scheduledTime
					StartTime:    &scheduledTime,
				},
				{
					OccurrenceID: "1705399200", // Unix timestamp of scheduledTime + 24 hours
					StartTime: func() *time.Time {
						t := scheduledTime.Add(24 * time.Hour)
						return &t
					}(),
				},
			},
			expectServiceCall: true,
		},
		{
			name: "recurring meeting finds closest occurrence",
			meeting: &models.MeetingBase{
				UID:       "meeting-4",
				StartTime: scheduledTime,
				Recurrence: &models.Recurrence{
					Type:           1,
					RepeatInterval: 1,
				},
			},
			actualStart: scheduledTime.Add(25 * time.Hour), // Closer to second occurrence
			expectedID:  "1705399200",                      // Unix timestamp of scheduledTime + 24 hours
			description: "Should return the occurrence ID closest to the actual start time",
			mockOccurrences: []models.Occurrence{
				{
					OccurrenceID: "1705312800", // Unix timestamp of scheduledTime
					StartTime:    &scheduledTime,
				},
				{
					OccurrenceID: "1705399200", // Unix timestamp of scheduledTime + 24 hours
					StartTime: func() *time.Time {
						t := scheduledTime.Add(24 * time.Hour)
						return &t
					}(),
				},
				{
					OccurrenceID: "1705485600", // Unix timestamp of scheduledTime + 48 hours
					StartTime: func() *time.Time {
						t := scheduledTime.Add(48 * time.Hour)
						return &t
					}(),
				},
			},
			expectServiceCall: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock occurrence service
			mockOccurrenceService := &serviceMocks.MockOccurrenceService{}

			if tt.expectServiceCall {
				// Set up mock to return the expected occurrences when called with search window
				mockOccurrenceService.On(
					"CalculateOccurrencesFromDate",
					tt.meeting,
					tt.actualStart.AddDate(0, -1, 0), // search start (1 month back)
					100,
				).Return(tt.mockOccurrences)
			}

			// Create handler with mock service
			handler := &ZoomWebhookHandler{
				occurrenceService: mockOccurrenceService,
			}

			result := handler.findClosestOccurrenceID(tt.meeting, tt.actualStart)
			assert.Equal(t, tt.expectedID, result, tt.description)

			// Verify mock expectations
			mockOccurrenceService.AssertExpectations(t)
		})
	}
}
