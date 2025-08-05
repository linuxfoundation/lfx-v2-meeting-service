// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package models

import (
	"testing"
	"time"

	meetingservice "github.com/linuxfoundation/lfx-v2-meeting-service/gen/meeting_service"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/utils"
)

func TestToMeetingDBModel(t *testing.T) {
	// Test with nil input
	result := ToMeetingDBModel(nil)
	if result != nil {
		t.Error("expected nil result for nil input")
	}

	// Test with valid Goa meeting
	now := time.Now()
	startTimeStr := now.Format(time.RFC3339)
	createdAtStr := now.Format(time.RFC3339)
	updatedAtStr := now.Format(time.RFC3339)

	goaMeeting := &meetingservice.Meeting{
		UID:                             utils.StringPtr("test-uid"),
		ProjectUID:                      utils.StringPtr("project-uid"),
		Title:                           utils.StringPtr("Test Meeting"),
		Description:                     utils.StringPtr("Test Description"),
		Timezone:                        utils.StringPtr("UTC"),
		Platform:                        utils.StringPtr("zoom"),
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

	meeting := ToMeetingDBModel(goaMeeting)
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
	if meeting.Platform != "zoom" {
		t.Errorf("expected Platform 'zoom', got %q", meeting.Platform)
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

func TestFromMeetingDBModel(t *testing.T) {
	// Test with nil input
	result := FromMeetingDBModel(nil)
	if result != nil {
		t.Error("expected nil result for nil input")
	}

	// Test with valid domain meeting
	now := time.Now()
	meeting := &Meeting{
		UID:                             "test-uid",
		ProjectUID:                      "project-uid",
		Title:                           "Test Meeting",
		Description:                     "Test Description",
		Timezone:                        "UTC",
		Platform:                        "zoom",
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

	goaMeeting := FromMeetingDBModel(meeting)
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
	originalMeeting := &Meeting{
		UID:               "round-trip-uid",
		ProjectUID:        "project-123",
		Title:             "Round Trip Test",
		Description:       "Testing round trip conversion",
		Timezone:          "America/New_York",
		Platform:          "zoom",
		Duration:          90,
		StartTime:         now,
		RecordingEnabled:  true,
		TranscriptEnabled: false,
		Restricted:        true,
		CreatedAt:         &now,
		UpdatedAt:         &now,
	}

	// Convert to Goa type
	goaMeeting := FromMeetingDBModel(originalMeeting)
	if goaMeeting == nil {
		t.Fatal("failed to convert to Goa meeting")
	}

	// Convert back to domain type
	convertedMeeting := ToMeetingDBModel(goaMeeting)
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
	meeting := &Meeting{
		UID:        "complex-uid",
		ProjectUID: "project-456",
		Title:      "Complex Meeting",
		StartTime:  now,
		Duration:   120,
		Committees: []Committee{
			{
				UID: "committee-1",
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
	goaMeeting := FromMeetingDBModel(meeting)
	if goaMeeting == nil {
		t.Fatal("failed to convert complex meeting to Goa")
	}

	convertedMeeting := ToMeetingDBModel(goaMeeting)
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