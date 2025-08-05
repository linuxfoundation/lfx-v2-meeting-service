// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package domain

import (
	"context"
	"testing"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
)

// mockMeetingRepository implements the MeetingRepository interface for testing
type mockMeetingRepository struct {
	meetings  map[string]*models.Meeting
	revisions map[string]uint64
}

func newMockMeetingRepository() *mockMeetingRepository {
	return &mockMeetingRepository{
		meetings:  make(map[string]*models.Meeting),
		revisions: make(map[string]uint64),
	}
}

func (m *mockMeetingRepository) CreateMeeting(ctx context.Context, meeting *models.Meeting) error {
	m.meetings[meeting.UID] = meeting
	m.revisions[meeting.UID] = 1
	return nil
}

func (m *mockMeetingRepository) MeetingExists(ctx context.Context, meetingUID string) (bool, error) {
	_, exists := m.meetings[meetingUID]
	return exists, nil
}

func (m *mockMeetingRepository) DeleteMeeting(ctx context.Context, meetingUID string, revision uint64) error {
	if m.revisions[meetingUID] != revision {
		return ErrRevisionMismatch
	}
	delete(m.meetings, meetingUID)
	delete(m.revisions, meetingUID)
	return nil
}

func (m *mockMeetingRepository) GetMeeting(ctx context.Context, meetingUID string) (*models.Meeting, error) {
	meeting, exists := m.meetings[meetingUID]
	if !exists {
		return nil, ErrMeetingNotFound
	}
	return meeting, nil
}

func (m *mockMeetingRepository) GetMeetingWithRevision(ctx context.Context, meetingUID string) (*models.Meeting, uint64, error) {
	meeting, exists := m.meetings[meetingUID]
	if !exists {
		return nil, 0, ErrMeetingNotFound
	}
	revision := m.revisions[meetingUID]
	return meeting, revision, nil
}

func (m *mockMeetingRepository) UpdateMeeting(ctx context.Context, meeting *models.Meeting, revision uint64) error {
	if m.revisions[meeting.UID] != revision {
		return ErrRevisionMismatch
	}
	m.meetings[meeting.UID] = meeting
	m.revisions[meeting.UID] = revision + 1
	return nil
}

func (m *mockMeetingRepository) ListAllMeetings(ctx context.Context) ([]*models.Meeting, error) {
	meetings := make([]*models.Meeting, 0, len(m.meetings))
	for _, meeting := range m.meetings {
		meetings = append(meetings, meeting)
	}
	return meetings, nil
}

func TestMeetingRepository_CreateMeeting(t *testing.T) {
	ctx := context.Background()
	repo := newMockMeetingRepository()

	meeting := &models.Meeting{
		UID:   "test-uid",
		Title: "Test Meeting",
	}

	err := repo.CreateMeeting(ctx, meeting)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	exists, err := repo.MeetingExists(ctx, "test-uid")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if !exists {
		t.Error("expected meeting to exist after creation")
	}
}

func TestMeetingRepository_GetMeeting(t *testing.T) {
	ctx := context.Background()
	repo := newMockMeetingRepository()

	meeting := &models.Meeting{
		UID:   "test-uid",
		Title: "Test Meeting",
	}

	// Test getting non-existent meeting
	_, err := repo.GetMeeting(ctx, "non-existent")
	if err != ErrMeetingNotFound {
		t.Errorf("expected ErrMeetingNotFound, got %v", err)
	}

	// Create and get meeting
	err = repo.CreateMeeting(ctx, meeting)
	if err != nil {
		t.Errorf("expected no error creating meeting, got %v", err)
	}

	retrieved, err := repo.GetMeeting(ctx, "test-uid")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if retrieved.UID != meeting.UID {
		t.Errorf("expected UID %q, got %q", meeting.UID, retrieved.UID)
	}
}

func TestMeetingRepository_GetMeetingWithRevision(t *testing.T) {
	ctx := context.Background()
	repo := newMockMeetingRepository()

	meeting := &models.Meeting{
		UID:   "test-uid",
		Title: "Test Meeting",
	}

	// Create meeting
	err := repo.CreateMeeting(ctx, meeting)
	if err != nil {
		t.Errorf("expected no error creating meeting, got %v", err)
	}

	retrieved, revision, err := repo.GetMeetingWithRevision(ctx, "test-uid")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if retrieved.UID != meeting.UID {
		t.Errorf("expected UID %q, got %q", meeting.UID, retrieved.UID)
	}
	if revision != 1 {
		t.Errorf("expected revision 1, got %d", revision)
	}
}

func TestMeetingRepository_UpdateMeeting(t *testing.T) {
	ctx := context.Background()
	repo := newMockMeetingRepository()

	meeting := &models.Meeting{
		UID:   "test-uid",
		Title: "Test Meeting",
	}

	// Create meeting
	err := repo.CreateMeeting(ctx, meeting)
	if err != nil {
		t.Errorf("expected no error creating meeting, got %v", err)
	}

	// Update with wrong revision should fail
	meeting.Title = "Updated Meeting"
	err = repo.UpdateMeeting(ctx, meeting, 999)
	if err != ErrRevisionMismatch {
		t.Errorf("expected ErrRevisionMismatch, got %v", err)
	}

	// Update with correct revision should succeed
	err = repo.UpdateMeeting(ctx, meeting, 1)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	// Verify update
	updated, revision, err := repo.GetMeetingWithRevision(ctx, "test-uid")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if updated.Title != "Updated Meeting" {
		t.Errorf("expected title 'Updated Meeting', got %q", updated.Title)
	}
	if revision != 2 {
		t.Errorf("expected revision 2, got %d", revision)
	}
}

func TestMeetingRepository_DeleteMeeting(t *testing.T) {
	ctx := context.Background()
	repo := newMockMeetingRepository()

	meeting := &models.Meeting{
		UID:   "test-uid",
		Title: "Test Meeting",
	}

	// Create meeting
	err := repo.CreateMeeting(ctx, meeting)
	if err != nil {
		t.Errorf("expected no error creating meeting, got %v", err)
	}

	// Delete with wrong revision should fail
	err = repo.DeleteMeeting(ctx, "test-uid", 999)
	if err != ErrRevisionMismatch {
		t.Errorf("expected ErrRevisionMismatch, got %v", err)
	}

	// Delete with correct revision should succeed
	err = repo.DeleteMeeting(ctx, "test-uid", 1)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	// Verify deletion
	exists, err := repo.MeetingExists(ctx, "test-uid")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if exists {
		t.Error("expected meeting to not exist after deletion")
	}
}

func TestMeetingRepository_ListAllMeetings(t *testing.T) {
	ctx := context.Background()
	repo := newMockMeetingRepository()

	// Empty list
	meetings, err := repo.ListAllMeetings(ctx)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if len(meetings) != 0 {
		t.Errorf("expected empty list, got %d meetings", len(meetings))
	}

	// Add meetings
	meeting1 := &models.Meeting{UID: "uid1", Title: "Meeting 1"}
	meeting2 := &models.Meeting{UID: "uid2", Title: "Meeting 2"}

	err = repo.CreateMeeting(ctx, meeting1)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	err = repo.CreateMeeting(ctx, meeting2)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	meetings, err = repo.ListAllMeetings(ctx)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if len(meetings) != 2 {
		t.Errorf("expected 2 meetings, got %d", len(meetings))
	}
}
