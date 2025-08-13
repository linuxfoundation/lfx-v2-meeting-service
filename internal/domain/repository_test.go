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
	meetings  map[string]*models.MeetingBase
	revisions map[string]uint64
}

func newMockMeetingRepository() *mockMeetingRepository {
	return &mockMeetingRepository{
		meetings:  make(map[string]*models.MeetingBase),
		revisions: make(map[string]uint64),
	}
}

func (m *mockMeetingRepository) Create(ctx context.Context, meeting *models.MeetingBase) error {
	m.meetings[meeting.UID] = meeting
	m.revisions[meeting.UID] = 1
	return nil
}

func (m *mockMeetingRepository) Exists(ctx context.Context, meetingUID string) (bool, error) {
	_, exists := m.meetings[meetingUID]
	return exists, nil
}

func (m *mockMeetingRepository) Delete(ctx context.Context, meetingUID string, revision uint64) error {
	if m.revisions[meetingUID] != revision {
		return ErrRevisionMismatch
	}
	delete(m.meetings, meetingUID)
	delete(m.revisions, meetingUID)
	return nil
}

func (m *mockMeetingRepository) GetBase(ctx context.Context, meetingUID string) (*models.MeetingBase, error) {
	meeting, exists := m.meetings[meetingUID]
	if !exists {
		return nil, ErrMeetingNotFound
	}
	return meeting, nil
}

func (m *mockMeetingRepository) GetBaseWithRevision(ctx context.Context, meetingUID string) (*models.MeetingBase, uint64, error) {
	meeting, exists := m.meetings[meetingUID]
	if !exists {
		return nil, 0, ErrMeetingNotFound
	}
	revision := m.revisions[meetingUID]
	return meeting, revision, nil
}

func (m *mockMeetingRepository) UpdateBase(ctx context.Context, meeting *models.MeetingBase, revision uint64) error {
	if m.revisions[meeting.UID] != revision {
		return ErrRevisionMismatch
	}
	m.meetings[meeting.UID] = meeting
	m.revisions[meeting.UID] = revision + 1
	return nil
}

func (m *mockMeetingRepository) ListAll(ctx context.Context) ([]*models.MeetingBase, error) {
	meetings := make([]*models.MeetingBase, 0, len(m.meetings))
	for _, meeting := range m.meetings {
		meetings = append(meetings, meeting)
	}
	return meetings, nil
}

func TestMeetingRepository_Create(t *testing.T) {
	ctx := context.Background()
	repo := newMockMeetingRepository()

	meeting := &models.MeetingBase{
		UID:   "test-uid",
		Title: "Test Meeting",
	}

	err := repo.Create(ctx, meeting)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	exists, err := repo.Exists(ctx, "test-uid")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if !exists {
		t.Error("expected meeting to exist after creation")
	}
}

func TestMeetingRepository_GetBase(t *testing.T) {
	ctx := context.Background()
	repo := newMockMeetingRepository()

	meeting := &models.MeetingBase{
		UID:   "test-uid",
		Title: "Test Meeting",
	}

	// Test getting non-existent meeting
	_, err := repo.GetBase(ctx, "non-existent")
	if err != ErrMeetingNotFound {
		t.Errorf("expected ErrMeetingNotFound, got %v", err)
	}

	// Create and get meeting
	err = repo.Create(ctx, meeting)
	if err != nil {
		t.Errorf("expected no error creating meeting, got %v", err)
	}

	retrieved, err := repo.GetBase(ctx, "test-uid")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if retrieved.UID != meeting.UID {
		t.Errorf("expected UID %q, got %q", meeting.UID, retrieved.UID)
	}
}

func TestMeetingRepository_GetBaseWithRevision(t *testing.T) {
	ctx := context.Background()
	repo := newMockMeetingRepository()

	meeting := &models.MeetingBase{
		UID:   "test-uid",
		Title: "Test Meeting",
	}

	// Create meeting
	err := repo.Create(ctx, meeting)
	if err != nil {
		t.Errorf("expected no error creating meeting, got %v", err)
	}

	retrieved, revision, err := repo.GetBaseWithRevision(ctx, "test-uid")
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

func TestMeetingRepository_UpdateBase(t *testing.T) {
	ctx := context.Background()
	repo := newMockMeetingRepository()

	meeting := &models.MeetingBase{
		UID:   "test-uid",
		Title: "Test Meeting",
	}

	// Create meeting
	err := repo.Create(ctx, meeting)
	if err != nil {
		t.Errorf("expected no error creating meeting, got %v", err)
	}

	// Update with wrong revision should fail
	meeting.Title = "Updated Meeting"
	err = repo.UpdateBase(ctx, meeting, 999)
	if err != ErrRevisionMismatch {
		t.Errorf("expected ErrRevisionMismatch, got %v", err)
	}

	// Update with correct revision should succeed
	err = repo.UpdateBase(ctx, meeting, 1)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	// Verify update
	updated, revision, err := repo.GetBaseWithRevision(ctx, "test-uid")
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

func TestMeetingRepository_Delete(t *testing.T) {
	ctx := context.Background()
	repo := newMockMeetingRepository()

	meeting := &models.MeetingBase{
		UID:   "test-uid",
		Title: "Test Meeting",
	}

	// Create meeting
	err := repo.Create(ctx, meeting)
	if err != nil {
		t.Errorf("expected no error creating meeting, got %v", err)
	}

	// Delete with wrong revision should fail
	err = repo.Delete(ctx, "test-uid", 999)
	if err != ErrRevisionMismatch {
		t.Errorf("expected ErrRevisionMismatch, got %v", err)
	}

	// Delete with correct revision should succeed
	err = repo.Delete(ctx, "test-uid", 1)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	// Verify deletion
	exists, err := repo.Exists(ctx, "test-uid")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if exists {
		t.Error("expected meeting to not exist after deletion")
	}
}

func TestMeetingRepository_ListAll(t *testing.T) {
	ctx := context.Background()
	repo := newMockMeetingRepository()

	// Empty list
	meetings, err := repo.ListAll(ctx)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if len(meetings) != 0 {
		t.Errorf("expected empty list, got %d meetings", len(meetings))
	}

	// Add meetings
	meeting1 := &models.MeetingBase{UID: "uid1", Title: "Meeting 1"}
	meeting2 := &models.MeetingBase{UID: "uid2", Title: "Meeting 2"}

	err = repo.Create(ctx, meeting1)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	err = repo.Create(ctx, meeting2)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	meetings, err = repo.ListAll(ctx)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if len(meetings) != 2 {
		t.Errorf("expected 2 meetings, got %d", len(meetings))
	}
}
