// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package store

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
)

func TestNewNatsMeetingRepository(t *testing.T) {
	meetings := &mockNatsKeyValue{}
	meetingSettings := &mockNatsKeyValue{}

	repo := NewNatsMeetingRepository(meetings, meetingSettings)

	if repo == nil {
		t.Fatal("expected repository to be created")
	}
	if repo.Meetings != meetings {
		t.Error("expected Meetings to be set correctly")
	}
}

func TestNatsMeetingRepository_Create(t *testing.T) {
	meetings := newMockNatsKeyValue()
	meetingSettings := newMockNatsKeyValue()
	repo := NewNatsMeetingRepository(meetings, meetingSettings)

	now := time.Now()
	meeting := &models.MeetingBase{
		UID:       "test-meeting-123",
		Title:     "Test Meeting",
		CreatedAt: &now,
		UpdatedAt: &now,
	}
	settings := &models.MeetingSettings{
		UID:        "test-meeting-123",
		Organizers: []string{"test-organizer-123"},
		CreatedAt:  &now,
		UpdatedAt:  &now,
	}

	err := repo.Create(context.Background(), meeting, settings)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Verify the meeting was stored
	storedData, exists := meetings.data[meeting.UID]
	if !exists {
		t.Error("expected meeting to be stored")
	}

	var storedMeeting models.MeetingBase
	if err := json.Unmarshal(storedData, &storedMeeting); err != nil {
		t.Errorf("failed to unmarshal stored meeting: %v", err)
	}

	if storedMeeting.UID != meeting.UID {
		t.Errorf("expected UID %s, got %s", meeting.UID, storedMeeting.UID)
	}
	if storedMeeting.Title != meeting.Title {
		t.Errorf("expected Title %s, got %s", meeting.Title, storedMeeting.Title)
	}

	// Verify the meeting settings were stored
	storedSettingsData, exists := meetingSettings.data[meeting.UID]
	if !exists {
		t.Error("expected meeting settings to be stored")
	}

	var storedSettings models.MeetingSettings
	if err := json.Unmarshal(storedSettingsData, &storedSettings); err != nil {
		t.Errorf("failed to unmarshal stored meeting settings: %v", err)
	}

	if storedSettings.UID != settings.UID {
		t.Errorf("expected UID %s, got %s", settings.UID, storedSettings.UID)
	}
	if len(storedSettings.Organizers) != len(settings.Organizers) {
		t.Errorf("expected %d organizers, got %d", len(settings.Organizers), len(storedSettings.Organizers))
	}
	for i, organizer := range storedSettings.Organizers {
		if organizer != settings.Organizers[i] {
			t.Errorf("expected organizer %q, got %q", settings.Organizers[i], organizer)
		}
	}
}

func TestNatsMeetingRepository_Create_Error(t *testing.T) {
	meetings := &mockNatsKeyValue{putError: errors.New("put failed")}
	meetingSettings := &mockNatsKeyValue{}
	repo := NewNatsMeetingRepository(meetings, meetingSettings)

	now := time.Now()
	meeting := &models.MeetingBase{
		UID:       "test-meeting-123",
		Title:     "Test Meeting",
		CreatedAt: &now,
		UpdatedAt: &now,
	}

	settings := &models.MeetingSettings{
		UID:        "test-meeting-123",
		Organizers: []string{"test-organizer-123"},
		CreatedAt:  &now,
		UpdatedAt:  &now,
	}
	err := repo.Create(context.Background(), meeting, settings)
	if err == nil {
		t.Error("expected error but got nil")
	}
	if domain.GetErrorType(err) != domain.ErrorTypeInternal {
		t.Errorf("expected Internal error, got %v", err)
	}
}

func TestNatsMeetingRepository_Exists(t *testing.T) {
	meetings := &mockNatsKeyValue{}
	meetingSettings := &mockNatsKeyValue{}
	repo := NewNatsMeetingRepository(meetings, meetingSettings)

	// Test non-existing meeting
	exists, err := repo.Exists(context.Background(), "non-existent")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if exists {
		t.Error("expected meeting to not exist")
	}

	// Add a meeting
	meetingData := `{"uid":"existing-meeting","title":"Test Meeting"}`
	meetings.data = map[string][]byte{
		"existing-meeting": []byte(meetingData),
	}

	// Test existing meeting
	exists, err = repo.Exists(context.Background(), "existing-meeting")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !exists {
		t.Error("expected meeting to exist")
	}
}

func TestNatsMeetingRepository_Get(t *testing.T) {
	meetings := &mockNatsKeyValue{}
	meetingSettings := &mockNatsKeyValue{}
	repo := NewNatsMeetingRepository(meetings, meetingSettings)

	now := time.Now()
	meeting := &models.MeetingBase{
		UID:       "test-meeting-123",
		Title:     "Test Meeting",
		CreatedAt: &now,
		UpdatedAt: &now,
	}

	meetingData, _ := json.Marshal(meeting)
	meetings.data = map[string][]byte{
		meeting.UID: meetingData,
	}

	result, err := repo.GetBase(context.Background(), meeting.UID)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result.UID != meeting.UID {
		t.Errorf("expected UID %s, got %s", meeting.UID, result.UID)
	}
	if result.Title != meeting.Title {
		t.Errorf("expected Title %s, got %s", meeting.Title, result.Title)
	}
}

func TestNatsMeetingRepository_GetWithRevision(t *testing.T) {
	meetings := &mockNatsKeyValue{}
	meetingSettings := &mockNatsKeyValue{}
	repo := NewNatsMeetingRepository(meetings, meetingSettings)

	now := time.Now()
	meeting := &models.MeetingBase{
		UID:       "test-meeting-123",
		Title:     "Test Meeting",
		CreatedAt: &now,
		UpdatedAt: &now,
	}

	meetingData, _ := json.Marshal(meeting)
	expectedRevision := uint64(42)
	meetings.data = map[string][]byte{
		meeting.UID: meetingData,
	}
	meetings.revisions = map[string]uint64{
		meeting.UID: expectedRevision,
	}

	result, revision, err := repo.GetBaseWithRevision(context.Background(), meeting.UID)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if revision != expectedRevision {
		t.Errorf("expected revision %d, got %d", expectedRevision, revision)
	}
	if result.UID != meeting.UID {
		t.Errorf("expected UID %s, got %s", meeting.UID, result.UID)
	}
}

func TestNatsMeetingRepository_Update(t *testing.T) {
	meetings := &mockNatsKeyValue{}
	meetingSettings := &mockNatsKeyValue{}
	repo := NewNatsMeetingRepository(meetings, meetingSettings)

	now := time.Now()
	meeting := &models.MeetingBase{
		UID:       "test-meeting-123",
		Title:     "Updated Meeting",
		CreatedAt: &now,
		UpdatedAt: &now,
	}

	// Set up existing meeting
	initialData, _ := json.Marshal(meeting)
	initialRevision := uint64(1)
	meetings.data = map[string][]byte{
		meeting.UID: initialData,
	}
	meetings.revisions = map[string]uint64{
		meeting.UID: initialRevision,
	}

	err := repo.UpdateBase(context.Background(), meeting, initialRevision)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Verify the meeting was updated
	revision := meetings.revisions[meeting.UID]
	if revision != initialRevision+1 {
		t.Errorf("expected revision to be incremented to %d, got %d", initialRevision+1, revision)
	}
}

func TestNatsMeetingRepository_Update_RevisionMismatch(t *testing.T) {
	meetings := &mockNatsKeyValue{}
	meetingSettings := &mockNatsKeyValue{}
	repo := NewNatsMeetingRepository(meetings, meetingSettings)

	meeting := &models.MeetingBase{
		UID:   "test-meeting-123",
		Title: "Updated Meeting",
	}

	// Set up existing meeting with different revision
	initialData, _ := json.Marshal(meeting)
	initialRevision := uint64(1)
	meetings.data = map[string][]byte{
		meeting.UID: initialData,
	}
	meetings.revisions = map[string]uint64{
		meeting.UID: initialRevision,
	}

	// Try to update with wrong revision
	wrongRevision := uint64(3)
	err := repo.UpdateBase(context.Background(), meeting, wrongRevision)
	if err == nil {
		t.Error("expected error but got nil")
	}
	if domain.GetErrorType(err) != domain.ErrorTypeConflict {
		t.Errorf("expected Conflict error, got %v", err)
	}
}

func TestNatsMeetingRepository_Delete(t *testing.T) {
	meetings := newMockNatsKeyValue()
	meetingSettings := newMockNatsKeyValue()
	repo := NewNatsMeetingRepository(meetings, meetingSettings)

	meetingUID := "test-meeting-123"
	revision := uint64(1)

	// Set up existing meeting
	meetings.data = map[string][]byte{
		meetingUID: []byte(`{"uid":"test-meeting-123","title":"Test Meeting"}`),
	}
	// Set up existing meeting settings
	meetingSettings.data = map[string][]byte{
		meetingUID: []byte(`{"uid":"test-meeting-123","organizers":["org1"]}`),
	}

	err := repo.Delete(context.Background(), meetingUID, revision)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Verify meeting was deleted
	if _, exists := meetings.data[meetingUID]; exists {
		t.Error("expected meeting to be deleted")
	}
	// Verify settings were deleted
	if _, exists := meetingSettings.data[meetingUID]; exists {
		t.Error("expected meeting settings to be deleted")
	}
}

func TestNatsMeetingRepository_Delete_RevisionMismatch(t *testing.T) {
	meetings := newMockNatsKeyValue()
	meetingSettings := newMockNatsKeyValue()
	repo := NewNatsMeetingRepository(meetings, meetingSettings)

	meetingUID := "test-meeting-123"

	// Set up existing meeting
	meetings.data = map[string][]byte{
		meetingUID: []byte(`{"uid":"test-meeting-123","title":"Test Meeting"}`),
	}

	// Set up the mock to return revision mismatch error
	meetings.deleteError = errors.New("wrong last sequence")

	wrongRevision := uint64(3)
	err := repo.Delete(context.Background(), meetingUID, wrongRevision)
	if err == nil {
		t.Error("expected error but got nil")
	}
	if domain.GetErrorType(err) != domain.ErrorTypeConflict {
		t.Errorf("expected Conflict error, got %v", err)
	}
}

func TestNatsMeetingRepository_ListAll(t *testing.T) {
	meetings := newMockNatsKeyValue()
	meetingSettings := newMockNatsKeyValue()
	repo := NewNatsMeetingRepository(meetings, meetingSettings)

	now := time.Now()
	meeting1 := &models.MeetingBase{
		UID:       "meeting-1",
		Title:     "First Meeting",
		CreatedAt: &now,
		UpdatedAt: &now,
	}
	meeting2 := &models.MeetingBase{
		UID:       "meeting-2",
		Title:     "Second Meeting",
		CreatedAt: &now,
		UpdatedAt: &now,
	}

	// Set up meetings
	meeting1Data, _ := json.Marshal(meeting1)
	meeting2Data, _ := json.Marshal(meeting2)
	meetings.data = map[string][]byte{
		"meeting-1": meeting1Data,
		"meeting-2": meeting2Data,
	}

	// Set up meeting settings
	settings1 := &models.MeetingSettings{
		UID:        "meeting-1",
		Organizers: []string{"org1"},
		CreatedAt:  &now,
		UpdatedAt:  &now,
	}
	settings2 := &models.MeetingSettings{
		UID:        "meeting-2",
		Organizers: []string{"org2"},
		CreatedAt:  &now,
		UpdatedAt:  &now,
	}
	settings1Data, _ := json.Marshal(settings1)
	settings2Data, _ := json.Marshal(settings2)
	meetingSettings.data = map[string][]byte{
		"meeting-1": settings1Data,
		"meeting-2": settings2Data,
	}

	result, settings, err := repo.ListAll(context.Background())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(result) != 2 {
		t.Errorf("expected 2 meetings, got %d", len(result))
	} else if len(settings) != 2 {
		t.Errorf("expected 2 meeting settings, got %d", len(settings))
	}

	// Check if both meetings are present
	foundMeeting1 := false
	foundMeeting2 := false
	for _, meeting := range result {
		if meeting.UID == "meeting-1" && meeting.Title == "First Meeting" {
			foundMeeting1 = true
		}
		if meeting.UID == "meeting-2" && meeting.Title == "Second Meeting" {
			foundMeeting2 = true
		}
	}

	if !foundMeeting1 {
		t.Error("expected to find meeting-1 base")
	}
	if !foundMeeting2 {
		t.Error("expected to find meeting-2 base")
	}

	// Check if both meeting settings are present
	foundMeetingSetting1 := false
	foundMeetingSetting2 := false
	for _, setting := range settings {
		if setting.UID == "meeting-1" {
			foundMeetingSetting1 = true
		}
		if setting.UID == "meeting-2" {
			foundMeetingSetting2 = true
		}
	}
	if !foundMeetingSetting1 {
		t.Error("expected to find meeting-1 settings")
	}
	if !foundMeetingSetting2 {
		t.Error("expected to find meeting-2 settings")
	}
}
