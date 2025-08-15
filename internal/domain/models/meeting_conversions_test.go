// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package models

import (
	"testing"
	"time"

	meetingservice "github.com/linuxfoundation/lfx-v2-meeting-service/gen/meeting_service"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/utils"
)

func TestToMeetingBaseDBModel(t *testing.T) {
	// Test with nil input
	result := ToMeetingBaseDBModel(nil)
	if result != nil {
		t.Error("expected nil result for nil input")
	}

	// Test with valid Goa meeting
	now := time.Now()
	startTimeStr := now.Format(time.RFC3339)
	createdAtStr := now.Format(time.RFC3339)
	updatedAtStr := now.Format(time.RFC3339)

	goaMeeting := &meetingservice.MeetingBase{
		UID:                             utils.StringPtr("test-uid"),
		ProjectUID:                      utils.StringPtr("project-uid"),
		Title:                           utils.StringPtr("Test Meeting"),
		Description:                     utils.StringPtr("Test Description"),
		Timezone:                        utils.StringPtr("UTC"),
		Platform:                        utils.StringPtr("Zoom"),
		Duration:                        utils.IntPtr(60),
		StartTime:                       &startTimeStr,
		EarlyJoinTimeMinutes:            utils.IntPtr(15),
		MeetingType:                     utils.StringPtr("standard"),
		Visibility:                      utils.StringPtr("public"),
		Restricted:                      utils.BoolPtr(false),
		ArtifactVisibility:              utils.StringPtr("public"),
		PublicLink:                      utils.StringPtr("https://example.com"),
		EmailDeliveryErrorCount:         utils.IntPtr(0),
		RecordingEnabled:                utils.BoolPtr(true),
		TranscriptEnabled:               utils.BoolPtr(true),
		YoutubeUploadEnabled:            utils.BoolPtr(false),
		RegistrantCount:                 utils.IntPtr(10),
		RegistrantResponseDeclinedCount: utils.IntPtr(2),
		RegistrantResponseAcceptedCount: utils.IntPtr(8),
		CreatedAt:                       &createdAtStr,
		UpdatedAt:                       &updatedAtStr,
	}

	meeting := ToMeetingBaseDBModel(goaMeeting)
	if meeting == nil {
		t.Fatal("expected non-nil meeting result")
	}

	// Test basic fields
	if meeting.UID != "test-uid" {
		t.Errorf("expected UID 'test-uid', got %q", meeting.UID)
	}
	if meeting.ProjectUID != "project-uid" {
		t.Errorf("expected ProjectUID 'project-uid', got %q", meeting.ProjectUID)
	}
	if meeting.Title != "Test Meeting" {
		t.Errorf("expected Title 'Test Meeting', got %q", meeting.Title)
	}
	if meeting.Duration != 60 {
		t.Errorf("expected Duration 60, got %d", meeting.Duration)
	}
	if meeting.Platform != "Zoom" {
		t.Errorf("expected Platform 'Zoom', got %q", meeting.Platform)
	}

	// Test time conversion
	if !meeting.StartTime.Equal(now.Truncate(time.Second)) {
		t.Errorf("expected StartTime %v, got %v", now.Truncate(time.Second), meeting.StartTime)
	}

	// Test boolean fields
	if meeting.Restricted != false {
		t.Errorf("expected Restricted false, got %t", meeting.Restricted)
	}
	if meeting.RecordingEnabled != true {
		t.Errorf("expected RecordingEnabled true, got %t", meeting.RecordingEnabled)
	}
}

func TestFromMeetingBaseDBModel(t *testing.T) {
	// Test with nil input
	result := FromMeetingBaseDBModel(nil)
	if result != nil {
		t.Error("expected nil result for nil input")
	}

	// Test with valid domain meeting
	now := time.Now()
	meeting := &MeetingBase{
		UID:                             "test-uid",
		ProjectUID:                      "project-uid",
		Title:                           "Test Meeting",
		Description:                     "Test Description",
		Timezone:                        "UTC",
		Platform:                        "Zoom",
		Duration:                        60,
		StartTime:                       now,
		EarlyJoinTimeMinutes:            15,
		MeetingType:                     "standard",
		Visibility:                      "public",
		Restricted:                      false,
		ArtifactVisibility:              "public",
		PublicLink:                      "https://example.com",
		EmailDeliveryErrorCount:         0,
		RecordingEnabled:                true,
		TranscriptEnabled:               true,
		YoutubeUploadEnabled:            false,
		RegistrantCount:                 10,
		RegistrantResponseDeclinedCount: 2,
		RegistrantResponseAcceptedCount: 8,
		CreatedAt:                       &now,
		UpdatedAt:                       &now,
	}

	goaMeeting := FromMeetingBaseDBModel(meeting)
	if goaMeeting == nil {
		t.Fatal("expected non-nil goa meeting result")
	}

	// Test basic fields
	if utils.StringValue(goaMeeting.UID) != "test-uid" {
		t.Errorf("expected UID 'test-uid', got %q", utils.StringValue(goaMeeting.UID))
	}
	if utils.StringValue(goaMeeting.ProjectUID) != "project-uid" {
		t.Errorf("expected ProjectUID 'project-uid', got %q", utils.StringValue(goaMeeting.ProjectUID))
	}
	if utils.StringValue(goaMeeting.Title) != "Test Meeting" {
		t.Errorf("expected Title 'Test Meeting', got %q", utils.StringValue(goaMeeting.Title))
	}
	if utils.IntValue(goaMeeting.Duration) != 60 {
		t.Errorf("expected Duration 60, got %d", utils.IntValue(goaMeeting.Duration))
	}

	// Test time conversion
	if goaMeeting.StartTime == nil {
		t.Error("expected StartTime to not be nil")
	} else {
		parsedTime, err := time.Parse(time.RFC3339, *goaMeeting.StartTime)
		if err != nil {
			t.Errorf("failed to parse StartTime: %v", err)
		} else if !parsedTime.Equal(now.Truncate(time.Second)) {
			t.Errorf("expected StartTime %v, got %v", now.Truncate(time.Second), parsedTime)
		}
	}

	// Test boolean fields
	if utils.BoolValue(goaMeeting.Restricted) != false {
		t.Errorf("expected Restricted false, got %t", utils.BoolValue(goaMeeting.Restricted))
	}
	if utils.BoolValue(goaMeeting.RecordingEnabled) != true {
		t.Errorf("expected RecordingEnabled true, got %t", utils.BoolValue(goaMeeting.RecordingEnabled))
	}
}

func TestConversionRoundTrip(t *testing.T) {
	// Test round trip conversion: Domain -> Goa -> Domain
	now := time.Now().Truncate(time.Second) // Truncate to avoid precision issues
	originalMeeting := &MeetingBase{
		UID:               "round-trip-uid",
		ProjectUID:        "project-123",
		Title:             "Round Trip Test",
		Description:       "Testing round trip conversion",
		Timezone:          "America/New_York",
		Platform:          "Zoom",
		Duration:          90,
		StartTime:         now,
		RecordingEnabled:  true,
		TranscriptEnabled: false,
		Restricted:        true,
		CreatedAt:         &now,
		UpdatedAt:         &now,
	}

	// Convert to Goa type
	goaMeeting := FromMeetingBaseDBModel(originalMeeting)
	if goaMeeting == nil {
		t.Fatal("failed to convert to Goa meeting")
	}

	// Convert back to domain type
	convertedMeeting := ToMeetingBaseDBModel(goaMeeting)
	if convertedMeeting == nil {
		t.Fatal("failed to convert back to domain meeting")
	}

	// Compare key fields
	if convertedMeeting.UID != originalMeeting.UID {
		t.Errorf("UID mismatch: expected %q, got %q", originalMeeting.UID, convertedMeeting.UID)
	}
	if convertedMeeting.Title != originalMeeting.Title {
		t.Errorf("Title mismatch: expected %q, got %q", originalMeeting.Title, convertedMeeting.Title)
	}
	if convertedMeeting.Duration != originalMeeting.Duration {
		t.Errorf("Duration mismatch: expected %d, got %d", originalMeeting.Duration, convertedMeeting.Duration)
	}
	if !convertedMeeting.StartTime.Equal(originalMeeting.StartTime) {
		t.Errorf("StartTime mismatch: expected %v, got %v", originalMeeting.StartTime, convertedMeeting.StartTime)
	}
	if convertedMeeting.RecordingEnabled != originalMeeting.RecordingEnabled {
		t.Errorf("RecordingEnabled mismatch: expected %t, got %t", originalMeeting.RecordingEnabled, convertedMeeting.RecordingEnabled)
	}
	if convertedMeeting.Restricted != originalMeeting.Restricted {
		t.Errorf("Restricted mismatch: expected %t, got %t", originalMeeting.Restricted, convertedMeeting.Restricted)
	}
}

func TestConversionWithComplexStructures(t *testing.T) {
	// Test conversion with nested structures
	now := time.Now().Truncate(time.Second)
	meeting := &MeetingBase{
		UID:        "complex-uid",
		ProjectUID: "project-456",
		Title:      "Complex Meeting",
		StartTime:  now,
		Duration:   120,
		Committees: []Committee{
			{
				UID:                   "committee-1",
				AllowedVotingStatuses: []string{"active", "pending"},
			},
		},
		ZoomConfig: &ZoomConfig{
			MeetingID:                "zoom-123",
			AICompanionEnabled:       true,
			AISummaryRequireApproval: false,
		},
		Recurrence: &Recurrence{
			Type:           1,
			RepeatInterval: 2,
			WeeklyDays:     "1,3,5",
		},
		CreatedAt: &now,
		UpdatedAt: &now,
	}

	// Convert to Goa and back
	goaMeeting := FromMeetingBaseDBModel(meeting)
	if goaMeeting == nil {
		t.Fatal("failed to convert complex meeting to Goa")
	}

	convertedMeeting := ToMeetingBaseDBModel(goaMeeting)
	if convertedMeeting == nil {
		t.Fatal("failed to convert complex Goa meeting back to domain")
	}

	// Test basic fields
	if convertedMeeting.UID != meeting.UID {
		t.Errorf("UID mismatch: expected %q, got %q", meeting.UID, convertedMeeting.UID)
	}
	if convertedMeeting.Title != meeting.Title {
		t.Errorf("Title mismatch: expected %q, got %q", meeting.Title, convertedMeeting.Title)
	}

	// Note: Complex structures like Committees, ZoomConfig, and Recurrence
	// would need specific conversion logic implemented in the actual conversion functions
	// This test serves as a placeholder for when that functionality is added
}

func TestToMeetingSettingsServiceModel(t *testing.T) {
	now := time.Now()

	// Test with nil input
	result := ToMeetingSettingsServiceModel(nil)
	if result != nil {
		t.Error("expected nil result for nil input")
	}

	// Test with valid MeetingSettings
	settings := &MeetingSettings{
		UID:        "meeting-123",
		Organizers: []string{"org1", "org2", "org3"},
		CreatedAt:  &now,
		UpdatedAt:  &now,
	}

	result = ToMeetingSettingsServiceModel(settings)
	if result == nil {
		t.Fatal("expected non-nil result for valid input")
	}

	// Check UID
	if result.UID == nil || *result.UID != settings.UID {
		t.Errorf("UID mismatch: expected %q, got %v", settings.UID, result.UID)
	}

	// Check Organizers
	if len(result.Organizers) != len(settings.Organizers) {
		t.Errorf("Organizers length mismatch: expected %d, got %d", len(settings.Organizers), len(result.Organizers))
	}
	for i, org := range settings.Organizers {
		if i < len(result.Organizers) && result.Organizers[i] != org {
			t.Errorf("Organizer[%d] mismatch: expected %q, got %q", i, org, result.Organizers[i])
		}
	}

	// Check CreatedAt
	if result.CreatedAt == nil {
		t.Error("expected CreatedAt to be set")
	} else {
		parsedTime, err := time.Parse(time.RFC3339, *result.CreatedAt)
		if err != nil {
			t.Errorf("failed to parse CreatedAt: %v", err)
		} else if parsedTime.Unix() != now.Unix() {
			t.Errorf("CreatedAt mismatch: expected %v, got %v", now, parsedTime)
		}
	}

	// Check UpdatedAt
	if result.UpdatedAt == nil {
		t.Error("expected UpdatedAt to be set")
	} else {
		parsedTime, err := time.Parse(time.RFC3339, *result.UpdatedAt)
		if err != nil {
			t.Errorf("failed to parse UpdatedAt: %v", err)
		} else if parsedTime.Unix() != now.Unix() {
			t.Errorf("UpdatedAt mismatch: expected %v, got %v", now, parsedTime)
		}
	}
}

func TestFromMeetingSettingsServiceModel(t *testing.T) {
	now := time.Now()
	nowStr := now.Format(time.RFC3339)

	// Test with nil input
	result := FromMeetingSettingsServiceModel(nil)
	if result != nil {
		t.Error("expected nil result for nil input")
	}

	// Test with valid service model
	serviceSettings := &meetingservice.MeetingSettings{
		UID:        utils.StringPtr("meeting-456"),
		Organizers: []string{"user1", "user2"},
		CreatedAt:  &nowStr,
		UpdatedAt:  &nowStr,
	}

	result = FromMeetingSettingsServiceModel(serviceSettings)
	if result == nil {
		t.Fatal("expected non-nil result for valid input")
	}

	// Check UID
	if result.UID != *serviceSettings.UID {
		t.Errorf("UID mismatch: expected %q, got %q", *serviceSettings.UID, result.UID)
	}

	// Check Organizers
	if len(result.Organizers) != len(serviceSettings.Organizers) {
		t.Errorf("Organizers length mismatch: expected %d, got %d", len(serviceSettings.Organizers), len(result.Organizers))
	}
	for i, org := range serviceSettings.Organizers {
		if i < len(result.Organizers) && result.Organizers[i] != org {
			t.Errorf("Organizer[%d] mismatch: expected %q, got %q", i, org, result.Organizers[i])
		}
	}

	// Check CreatedAt
	if result.CreatedAt == nil {
		t.Error("expected CreatedAt to be set")
	} else if result.CreatedAt.Unix() != now.Unix() {
		t.Errorf("CreatedAt mismatch: expected %v, got %v", now, *result.CreatedAt)
	}

	// Check UpdatedAt
	if result.UpdatedAt == nil {
		t.Error("expected UpdatedAt to be set")
	} else if result.UpdatedAt.Unix() != now.Unix() {
		t.Errorf("UpdatedAt mismatch: expected %v, got %v", now, *result.UpdatedAt)
	}
}

func TestMeetingSettingsConversionRoundTrip(t *testing.T) {
	now := time.Now()

	// Original domain model
	originalSettings := &MeetingSettings{
		UID:        "test-meeting-789",
		Organizers: []string{"admin", "moderator", "host"},
		CreatedAt:  &now,
		UpdatedAt:  &now,
	}

	// Convert to service model and back
	serviceModel := ToMeetingSettingsServiceModel(originalSettings)
	if serviceModel == nil {
		t.Fatal("failed to convert to service model")
	}

	backToDomain := FromMeetingSettingsServiceModel(serviceModel)
	if backToDomain == nil {
		t.Fatal("failed to convert back to domain model")
	}

	// Verify all fields match
	if backToDomain.UID != originalSettings.UID {
		t.Errorf("Round-trip UID mismatch: expected %q, got %q", originalSettings.UID, backToDomain.UID)
	}

	if len(backToDomain.Organizers) != len(originalSettings.Organizers) {
		t.Errorf("Round-trip Organizers length mismatch: expected %d, got %d",
			len(originalSettings.Organizers), len(backToDomain.Organizers))
	}

	for i, org := range originalSettings.Organizers {
		if i < len(backToDomain.Organizers) && backToDomain.Organizers[i] != org {
			t.Errorf("Round-trip Organizer[%d] mismatch: expected %q, got %q", i, org, backToDomain.Organizers[i])
		}
	}

	// Note: Time comparison allows for small differences due to formatting precision
	if backToDomain.CreatedAt == nil || backToDomain.CreatedAt.Unix() != originalSettings.CreatedAt.Unix() {
		t.Errorf("Round-trip CreatedAt mismatch: expected %v, got %v",
			originalSettings.CreatedAt, backToDomain.CreatedAt)
	}

	if backToDomain.UpdatedAt == nil || backToDomain.UpdatedAt.Unix() != originalSettings.UpdatedAt.Unix() {
		t.Errorf("Round-trip UpdatedAt mismatch: expected %v, got %v",
			originalSettings.UpdatedAt, backToDomain.UpdatedAt)
	}
}

func TestMergeZoomConfig(t *testing.T) {
	tests := []struct {
		name     string
		existing *ZoomConfig
		payload  *meetingservice.ZoomConfigPost
		expected *ZoomConfig
	}{
		{
			name:     "both nil",
			existing: nil,
			payload:  nil,
			expected: nil,
		},
		{
			name:     "existing nil, payload provided",
			existing: nil,
			payload: &meetingservice.ZoomConfigPost{
				AiCompanionEnabled:       utils.BoolPtr(true),
				AiSummaryRequireApproval: utils.BoolPtr(false),
			},
			expected: &ZoomConfig{
				MeetingID:                "",
				AICompanionEnabled:       true,
				AISummaryRequireApproval: false,
			},
		},
		{
			name: "existing provided, payload nil",
			existing: &ZoomConfig{
				MeetingID:                "12345",
				AICompanionEnabled:       true,
				AISummaryRequireApproval: false,
			},
			payload: nil,
			expected: &ZoomConfig{
				MeetingID:                "12345",
				AICompanionEnabled:       true,
				AISummaryRequireApproval: false,
			},
		},
		{
			name: "merge existing with payload - preserve MeetingID",
			existing: &ZoomConfig{
				MeetingID:                "zoom-meeting-123",
				AICompanionEnabled:       false,
				AISummaryRequireApproval: true,
			},
			payload: &meetingservice.ZoomConfigPost{
				AiCompanionEnabled:       utils.BoolPtr(true),
				AiSummaryRequireApproval: utils.BoolPtr(false),
			},
			expected: &ZoomConfig{
				MeetingID:                "zoom-meeting-123", // Should be preserved
				AICompanionEnabled:       true,               // Should be updated
				AISummaryRequireApproval: false,              // Should be updated
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mergeZoomConfig(tt.existing, tt.payload)

			if tt.expected == nil {
				if result != nil {
					t.Errorf("expected nil result, got %+v", result)
				}
				return
			}

			if result == nil {
				t.Fatalf("expected non-nil result, got nil")
			}

			if result.MeetingID != tt.expected.MeetingID {
				t.Errorf("MeetingID mismatch: expected %q, got %q", tt.expected.MeetingID, result.MeetingID)
			}

			if result.AICompanionEnabled != tt.expected.AICompanionEnabled {
				t.Errorf("AICompanionEnabled mismatch: expected %v, got %v", tt.expected.AICompanionEnabled, result.AICompanionEnabled)
			}

			if result.AISummaryRequireApproval != tt.expected.AISummaryRequireApproval {
				t.Errorf("AISummaryRequireApproval mismatch: expected %v, got %v", tt.expected.AISummaryRequireApproval, result.AISummaryRequireApproval)
			}
		})
	}
}
