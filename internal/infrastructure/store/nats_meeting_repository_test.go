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

	repo := NewNatsMeetingRepository(meetings)

	if repo == nil {
		t.Fatal("expected repository to be created")
	}
	if repo.Meetings != meetings {
		t.Error("expected Meetings to be set correctly")
	}
}

func TestNatsMeetingRepository_Create(t *testing.T) {
	meetings := newMockNatsKeyValue()
	repo := NewNatsMeetingRepository(meetings)

	now := time.Now()
	meeting := &models.Meeting{
		UID:       "test-meeting-123",
		Title:     "Test Meeting",
		CreatedAt: &now,
		UpdatedAt: &now,
	}

	err := repo.Create(context.Background(), meeting)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Verify the meeting was stored
	storedData, exists := meetings.data[meeting.UID]
	if !exists {
		t.Error("expected meeting to be stored")
	}

	var storedMeeting models.Meeting
	if err := json.Unmarshal(storedData, &storedMeeting); err != nil {
		t.Errorf("failed to unmarshal stored meeting: %v", err)
	}

	if storedMeeting.UID != meeting.UID {
		t.Errorf("expected UID %s, got %s", meeting.UID, storedMeeting.UID)
	}
	if storedMeeting.Title != meeting.Title {
		t.Errorf("expected Title %s, got %s", meeting.Title, storedMeeting.Title)
	}
}

func TestNatsMeetingRepository_Create_Error(t *testing.T) {
	meetings := &mockNatsKeyValue{putError: errors.New("put failed")}
	repo := NewNatsMeetingRepository(meetings)

	meeting := &models.Meeting{UID: "test-meeting-123", Title: "Test Meeting"}

	err := repo.Create(context.Background(), meeting)
	if err == nil {
		t.Error("expected error but got nil")
	}
	if err != domain.ErrInternal {
		t.Errorf("expected ErrInternal, got %v", err)
	}
}

func TestNatsMeetingRepository_Exists(t *testing.T) {
	meetings := &mockNatsKeyValue{}
	repo := NewNatsMeetingRepository(meetings)

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
	repo := NewNatsMeetingRepository(meetings)

	now := time.Now()
	meeting := &models.Meeting{
		UID:       "test-meeting-123",
		Title:     "Test Meeting",
		CreatedAt: &now,
		UpdatedAt: &now,
	}

	meetingData, _ := json.Marshal(meeting)
	meetings.data = map[string][]byte{
		meeting.UID: meetingData,
	}

	result, err := repo.Get(context.Background(), meeting.UID)
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
	repo := NewNatsMeetingRepository(meetings)

	now := time.Now()
	meeting := &models.Meeting{
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

	result, revision, err := repo.GetWithRevision(context.Background(), meeting.UID)
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
	repo := NewNatsMeetingRepository(meetings)

	now := time.Now()
	meeting := &models.Meeting{
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

	err := repo.Update(context.Background(), meeting, initialRevision)
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
	repo := NewNatsMeetingRepository(meetings)

	meeting := &models.Meeting{
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
	err := repo.Update(context.Background(), meeting, wrongRevision)
	if err == nil {
		t.Error("expected error but got nil")
	}
	if err != domain.ErrRevisionMismatch {
		t.Errorf("expected ErrRevisionMismatch, got %v", err)
	}
}

func TestNatsMeetingRepository_Delete(t *testing.T) {
	meetings := &mockNatsKeyValue{}
	repo := NewNatsMeetingRepository(meetings)

	meetingUID := "test-meeting-123"
	revision := uint64(1)

	// Set up existing meeting
	meetings.data = map[string][]byte{
		meetingUID: []byte(`{"uid":"test-meeting-123","title":"Test Meeting"}`),
	}

	err := repo.Delete(context.Background(), meetingUID, revision)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Verify meeting was deleted
	if _, exists := meetings.data[meetingUID]; exists {
		t.Error("expected meeting to be deleted")
	}
}

func TestNatsMeetingRepository_Delete_RevisionMismatch(t *testing.T) {
	meetings := &mockNatsKeyValue{}
	repo := NewNatsMeetingRepository(meetings)

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
	if err != domain.ErrRevisionMismatch {
		t.Errorf("expected ErrRevisionMismatch, got %v", err)
	}
}

func TestNatsMeetingRepository_ListAll(t *testing.T) {
	meetings := &mockNatsKeyValue{}
	repo := NewNatsMeetingRepository(meetings)

	now := time.Now()
	meeting1 := &models.Meeting{
		UID:       "meeting-1",
		Title:     "First Meeting",
		CreatedAt: &now,
		UpdatedAt: &now,
	}
	meeting2 := &models.Meeting{
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

	result, err := repo.ListAll(context.Background())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(result) != 2 {
		t.Errorf("expected 2 meetings, got %d", len(result))
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
		t.Error("expected to find meeting-1")
	}
	if !foundMeeting2 {
		t.Error("expected to find meeting-2")
	}
}
