// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package models

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/utils"
)

func TestMeeting_JSONSerialization(t *testing.T) {
	now := time.Now().UTC()
	meeting := MeetingBase{
		UID:                             "test-uid",
		ProjectUID:                      "project-uid",
		StartTime:                       now,
		Duration:                        60,
		Timezone:                        "UTC",
		Title:                           "Test Meeting",
		Description:                     "Test Description",
		Platform:                        "Zoom",
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

	// Test JSON marshaling
	data, err := json.Marshal(meeting)
	if err != nil {
		t.Errorf("failed to marshal meeting: %v", err)
	}

	// Test JSON unmarshaling
	var unmarshaled MeetingBase
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Errorf("failed to unmarshal meeting: %v", err)
	}

	// Compare key fields
	if unmarshaled.UID != meeting.UID {
		t.Errorf("expected UID %q, got %q", meeting.UID, unmarshaled.UID)
	}
	if unmarshaled.Title != meeting.Title {
		t.Errorf("expected Title %q, got %q", meeting.Title, unmarshaled.Title)
	}
	if unmarshaled.Duration != meeting.Duration {
		t.Errorf("expected Duration %d, got %d", meeting.Duration, unmarshaled.Duration)
	}
}

func TestCommittee_JSONSerialization(t *testing.T) {
	committee := Committee{
		UID:                   "committee-uid",
		AllowedVotingStatuses: []string{"active", "pending"},
	}

	data, err := json.Marshal(committee)
	if err != nil {
		t.Errorf("failed to marshal committee: %v", err)
	}

	var unmarshaled Committee
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Errorf("failed to unmarshal committee: %v", err)
	}

	if unmarshaled.UID != committee.UID {
		t.Errorf("expected UID %q, got %q", committee.UID, unmarshaled.UID)
	}
	if len(unmarshaled.AllowedVotingStatuses) != len(committee.AllowedVotingStatuses) {
		t.Errorf("expected %d voting statuses, got %d", len(committee.AllowedVotingStatuses), len(unmarshaled.AllowedVotingStatuses))
	}
}

func TestRecurrence_JSONSerialization(t *testing.T) {
	recurrence := Recurrence{
		Type:           1,
		RepeatInterval: 2,
		WeeklyDays:     "1,3,5",
		MonthlyDay:     15,
		MonthlyWeek:    2,
		MonthlyWeekDay: 3,
		EndTimes:       10,
		EndDateTime:    utils.TimePtr(time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC)),
	}

	data, err := json.Marshal(recurrence)
	if err != nil {
		t.Errorf("failed to marshal recurrence: %v", err)
	}

	var unmarshaled Recurrence
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Errorf("failed to unmarshal recurrence: %v", err)
	}

	if unmarshaled.Type != recurrence.Type {
		t.Errorf("expected Type %d, got %d", recurrence.Type, unmarshaled.Type)
	}
	if unmarshaled.RepeatInterval != recurrence.RepeatInterval {
		t.Errorf("expected RepeatInterval %d, got %d", recurrence.RepeatInterval, unmarshaled.RepeatInterval)
	}
	if unmarshaled.WeeklyDays != recurrence.WeeklyDays {
		t.Errorf("expected WeeklyDays %q, got %q", recurrence.WeeklyDays, unmarshaled.WeeklyDays)
	}
}

func TestOccurrence_JSONSerialization(t *testing.T) {
	recurrence := &Recurrence{
		Type:           1,
		RepeatInterval: 1,
	}

	occurrence := Occurrence{
		OccurrenceID:     "occurrence-123",
		StartTime:        utils.TimePtr(time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)),
		Title:            "Occurrence Title",
		Description:      "Occurrence Description",
		Duration:         60,
		Recurrence:       recurrence,
		RegistrantCount:  5,
		ResponseCountNo:  1,
		ResponseCountYes: 4,
		Status:           "scheduled",
	}

	data, err := json.Marshal(occurrence)
	if err != nil {
		t.Errorf("failed to marshal occurrence: %v", err)
	}

	var unmarshaled Occurrence
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Errorf("failed to unmarshal occurrence: %v", err)
	}

	if unmarshaled.OccurrenceID != occurrence.OccurrenceID {
		t.Errorf("expected OccurrenceID %q, got %q", occurrence.OccurrenceID, unmarshaled.OccurrenceID)
	}
	if unmarshaled.StartTime == nil || occurrence.StartTime == nil || !unmarshaled.StartTime.Equal(*occurrence.StartTime) {
		t.Errorf("expected StartTime %q, got %q", occurrence.StartTime, unmarshaled.StartTime)
	}
	if unmarshaled.Recurrence == nil {
		t.Error("expected Recurrence to not be nil")
	} else if unmarshaled.Recurrence.Type != recurrence.Type {
		t.Errorf("expected Recurrence.Type %d, got %d", recurrence.Type, unmarshaled.Recurrence.Type)
	}
}

func TestZoomConfig_JSONSerialization(t *testing.T) {
	zoomConfig := ZoomConfig{
		MeetingID:                "zoom-meeting-123",
		AICompanionEnabled:       true,
		AISummaryRequireApproval: false,
	}

	data, err := json.Marshal(zoomConfig)
	if err != nil {
		t.Errorf("failed to marshal zoom config: %v", err)
	}

	var unmarshaled ZoomConfig
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Errorf("failed to unmarshal zoom config: %v", err)
	}

	if unmarshaled.MeetingID != zoomConfig.MeetingID {
		t.Errorf("expected MeetingID %q, got %q", zoomConfig.MeetingID, unmarshaled.MeetingID)
	}
	if unmarshaled.AICompanionEnabled != zoomConfig.AICompanionEnabled {
		t.Errorf("expected AICompanionEnabled %t, got %t", zoomConfig.AICompanionEnabled, unmarshaled.AICompanionEnabled)
	}
	if unmarshaled.AISummaryRequireApproval != zoomConfig.AISummaryRequireApproval {
		t.Errorf("expected AISummaryRequireApproval %t, got %t", zoomConfig.AISummaryRequireApproval, unmarshaled.AISummaryRequireApproval)
	}
}

func TestMeeting_WithComplexStructures(t *testing.T) {
	now := time.Now().UTC()
	meeting := MeetingBase{
		UID:         "complex-meeting",
		ProjectUID:  "project-123",
		StartTime:   now,
		Duration:    90,
		Timezone:    "America/New_York",
		Title:       "Complex Meeting",
		Description: "Meeting with all structures",
		Platform:    "Zoom",
		Recurrence: &Recurrence{
			Type:           2,
			RepeatInterval: 1,
			WeeklyDays:     "1,3,5",
		},
		Committees: []Committee{
			{
				UID:                   "committee-1",
				AllowedVotingStatuses: []string{"active"},
			},
			{
				UID:                   "committee-2",
				AllowedVotingStatuses: []string{"active", "pending"},
			},
		},
		ZoomConfig: &ZoomConfig{
			MeetingID:                "zoom-123",
			AICompanionEnabled:       true,
			AISummaryRequireApproval: true,
		},
		Occurrences: []Occurrence{
			{
				OccurrenceID:     "occ-1",
				StartTime:        utils.TimePtr(time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)),
				Title:            "First Occurrence",
				Duration:         90,
				RegistrantCount:  10,
				ResponseCountYes: 8,
				ResponseCountNo:  2,
				Status:           "scheduled",
			},
		},
		CreatedAt: &now,
		UpdatedAt: &now,
	}

	data, err := json.Marshal(meeting)
	if err != nil {
		t.Errorf("failed to marshal complex meeting: %v", err)
	}

	var unmarshaled MeetingBase
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Errorf("failed to unmarshal complex meeting: %v", err)
	}

	// Test nested structures
	if unmarshaled.Recurrence == nil {
		t.Error("expected Recurrence to not be nil")
	}
	if len(unmarshaled.Committees) != 2 {
		t.Errorf("expected 2 committees, got %d", len(unmarshaled.Committees))
	}
	if unmarshaled.ZoomConfig == nil {
		t.Error("expected ZoomConfig to not be nil")
	}
	if len(unmarshaled.Occurrences) != 1 {
		t.Errorf("expected 1 occurrence, got %d", len(unmarshaled.Occurrences))
	}
}
