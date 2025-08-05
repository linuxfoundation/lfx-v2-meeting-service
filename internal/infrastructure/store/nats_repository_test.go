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
	"github.com/nats-io/nats.go/jetstream"
)

// mockKeyValueEntry implements jetstream.KeyValueEntry for testing
type mockKeyValueEntry struct {
	key      string
	value    []byte
	revision uint64
}

func (m *mockKeyValueEntry) Key() string { return m.key }
func (m *mockKeyValueEntry) Value() []byte { return m.value }
func (m *mockKeyValueEntry) Revision() uint64 { return m.revision }
func (m *mockKeyValueEntry) Created() time.Time { return time.Now() }
func (m *mockKeyValueEntry) Delta() uint64 { return 0 }
func (m *mockKeyValueEntry) Operation() jetstream.KeyValueOp { return jetstream.KeyValuePut }
func (m *mockKeyValueEntry) Bucket() string { return "test-bucket" }

// mockKeyLister implements jetstream.KeyLister for testing
type mockKeyLister struct {
	keys []string
	index int
}

func (m *mockKeyLister) Next() (jetstream.KeyValueEntry, error) {
	if m.index >= len(m.keys) {
		return nil, errors.New("no more keys")
	}
	key := m.keys[m.index]
	m.index++
	return &mockKeyValueEntry{key: key, value: []byte(`{"uid":"` + key + `","title":"Test Meeting"}`)}, nil
}

func (m *mockKeyLister) Keys() <-chan string {
	ch := make(chan string)
	go func() {
		defer close(ch)
		for _, key := range m.keys {
			ch <- key
		}
	}()
	return ch
}

func (m *mockKeyLister) Stop() error { return nil }

// mockNatsKeyValue implements INatsKeyValue for testing
type mockNatsKeyValue struct {
	data      map[string][]byte
	revisions map[string]uint64
	putError  error
	getError  error
	deleteError error
	updateError error
}

func newMockNatsKeyValue() *mockNatsKeyValue {
	return &mockNatsKeyValue{
		data:      make(map[string][]byte),
		revisions: make(map[string]uint64),
	}
}

func (m *mockNatsKeyValue) ListKeys(ctx context.Context, opts ...jetstream.WatchOpt) (jetstream.KeyLister, error) {
	keys := make([]string, 0, len(m.data))
	for key := range m.data {
		keys = append(keys, key)
	}
	return &mockKeyLister{keys: keys}, nil
}

func (m *mockNatsKeyValue) Get(ctx context.Context, key string) (jetstream.KeyValueEntry, error) {
	if m.getError != nil {
		return nil, m.getError
	}
	value, exists := m.data[key]
	if !exists {
		return nil, jetstream.ErrKeyNotFound
	}
	revision := m.revisions[key]
	return &mockKeyValueEntry{key: key, value: value, revision: revision}, nil
}

func (m *mockNatsKeyValue) Put(ctx context.Context, key string, data []byte) (uint64, error) {
	if m.putError != nil {
		return 0, m.putError
	}
	m.data[key] = data
	revision := uint64(1)
	if existingRevision, exists := m.revisions[key]; exists {
		revision = existingRevision + 1
	}
	m.revisions[key] = revision
	return revision, nil
}

func (m *mockNatsKeyValue) Update(ctx context.Context, key string, data []byte, expectedRevision uint64) (uint64, error) {
	if m.updateError != nil {
		return 0, m.updateError
	}
	currentRevision, exists := m.revisions[key]
	if !exists {
		return 0, jetstream.ErrKeyNotFound
	}
	if currentRevision != expectedRevision {
		return 0, errors.New("wrong sequence")
	}
	m.data[key] = data
	newRevision := currentRevision + 1
	m.revisions[key] = newRevision
	return newRevision, nil
}

func (m *mockNatsKeyValue) Delete(ctx context.Context, key string, opts ...jetstream.KVDeleteOpt) error {
	if m.deleteError != nil {
		return m.deleteError
	}
	// Check if revision mismatch should be simulated
	// We need to check if the key exists and handle revision checking for delete operations
	if _, exists := m.data[key]; !exists {
		return jetstream.ErrKeyNotFound
	}
	delete(m.data, key)
	delete(m.revisions, key)
	return nil
}

func TestNewNatsRepository(t *testing.T) {
	meetings := newMockNatsKeyValue()
	registrants := newMockNatsKeyValue()
	
	repo := NewNatsRepository(meetings, registrants)
	
	if repo == nil {
		t.Fatal("expected non-nil repository")
	}
	if repo.Meetings != meetings {
		t.Error("expected meetings KV store to be set correctly")
	}
	if repo.MeetingRegistrants != registrants {
		t.Error("expected meeting registrants KV store to be set correctly")
	}
}

func TestNatsRepository_CreateMeeting(t *testing.T) {
	meetings := newMockNatsKeyValue()
	repo := NewNatsRepository(meetings, nil)
	
	meeting := &models.Meeting{
		UID:         "test-meeting-uid",
		Title:       "Test Meeting",
		ProjectUID:  "project-123",
		Description: "Test Description",
	}
	
	ctx := context.Background()
	err := repo.CreateMeeting(ctx, meeting)
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	
	// Verify the meeting was stored
	data, exists := meetings.data[meeting.UID]
	if !exists {
		t.Error("expected meeting to be stored")
	}
	
	var storedMeeting models.Meeting
	err = json.Unmarshal(data, &storedMeeting)
	if err != nil {
		t.Errorf("failed to unmarshal stored meeting: %v", err)
	}
	
	if storedMeeting.UID != meeting.UID {
		t.Errorf("expected UID %q, got %q", meeting.UID, storedMeeting.UID)
	}
	if storedMeeting.Title != meeting.Title {
		t.Errorf("expected Title %q, got %q", meeting.Title, storedMeeting.Title)
	}
}

func TestNatsRepository_CreateMeeting_Error(t *testing.T) {
	meetings := newMockNatsKeyValue()
	meetings.putError = errors.New("put failed")
	repo := NewNatsRepository(meetings, nil)
	
	meeting := &models.Meeting{UID: "test-uid", Title: "Test"}
	
	ctx := context.Background()
	err := repo.CreateMeeting(ctx, meeting)
	if err == nil {
		t.Error("expected error but got none")
	}
}

func TestNatsRepository_MeetingExists(t *testing.T) {
	meetings := newMockNatsKeyValue()
	repo := NewNatsRepository(meetings, nil)
	
	ctx := context.Background()
	
	// Test non-existent meeting
	exists, err := repo.MeetingExists(ctx, "non-existent")
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	if exists {
		t.Error("expected meeting to not exist")
	}
	
	// Add a meeting
	meetings.data["existing-meeting"] = []byte(`{"uid":"existing-meeting"}`)
	meetings.revisions["existing-meeting"] = 1
	
	// Test existing meeting
	exists, err = repo.MeetingExists(ctx, "existing-meeting")
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	if !exists {
		t.Error("expected meeting to exist")
	}
}

func TestNatsRepository_GetMeeting(t *testing.T) {
	meetings := newMockNatsKeyValue()
	repo := NewNatsRepository(meetings, nil)
	
	ctx := context.Background()
	
	// Test non-existent meeting
	_, err := repo.GetMeeting(ctx, "non-existent")
	if err != domain.ErrMeetingNotFound {
		t.Errorf("expected ErrMeetingNotFound, got: %v", err)
	}
	
	// Add a meeting
	meeting := &models.Meeting{
		UID:   "test-meeting",
		Title: "Test Meeting",
	}
	data, _ := json.Marshal(meeting)
	meetings.data["test-meeting"] = data
	meetings.revisions["test-meeting"] = 1
	
	// Test existing meeting
	retrieved, err := repo.GetMeeting(ctx, "test-meeting")
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	if retrieved == nil {
		t.Fatal("expected non-nil meeting")
	}
	if retrieved.UID != meeting.UID {
		t.Errorf("expected UID %q, got %q", meeting.UID, retrieved.UID)
	}
	if retrieved.Title != meeting.Title {
		t.Errorf("expected Title %q, got %q", meeting.Title, retrieved.Title)
	}
}

func TestNatsRepository_GetMeetingWithRevision(t *testing.T) {
	meetings := newMockNatsKeyValue()
	repo := NewNatsRepository(meetings, nil)
	
	ctx := context.Background()
	
	// Add a meeting
	meeting := &models.Meeting{
		UID:   "test-meeting",
		Title: "Test Meeting",
	}
	data, _ := json.Marshal(meeting)
	meetings.data["test-meeting"] = data
	meetings.revisions["test-meeting"] = 5
	
	// Test getting meeting with revision
	retrieved, revision, err := repo.GetMeetingWithRevision(ctx, "test-meeting")
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	if retrieved == nil {
		t.Fatal("expected non-nil meeting")
	}
	if revision != 5 {
		t.Errorf("expected revision 5, got %d", revision)
	}
	if retrieved.UID != meeting.UID {
		t.Errorf("expected UID %q, got %q", meeting.UID, retrieved.UID)
	}
}

func TestNatsRepository_UpdateMeeting(t *testing.T) {
	meetings := newMockNatsKeyValue()
	repo := NewNatsRepository(meetings, nil)
	
	ctx := context.Background()
	
	// Add initial meeting
	meeting := &models.Meeting{
		UID:   "test-meeting",
		Title: "Original Title",
	}
	data, _ := json.Marshal(meeting)
	meetings.data["test-meeting"] = data
	meetings.revisions["test-meeting"] = 1
	
	// Update the meeting
	meeting.Title = "Updated Title"
	err := repo.UpdateMeeting(ctx, meeting, 1)
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	
	// Verify the update
	updatedData := meetings.data["test-meeting"]
	var updatedMeeting models.Meeting
	err = json.Unmarshal(updatedData, &updatedMeeting)
	if err != nil {
		t.Errorf("failed to unmarshal updated meeting: %v", err)
	}
	
	if updatedMeeting.Title != "Updated Title" {
		t.Errorf("expected updated title 'Updated Title', got %q", updatedMeeting.Title)
	}
	
	// Check revision was incremented
	if meetings.revisions["test-meeting"] != 2 {
		t.Errorf("expected revision 2, got %d", meetings.revisions["test-meeting"])
	}
}

func TestNatsRepository_UpdateMeeting_RevisionMismatch(t *testing.T) {
	meetings := newMockNatsKeyValue()
	repo := NewNatsRepository(meetings, nil)
	
	ctx := context.Background()
	
	// Add initial meeting
	meeting := &models.Meeting{UID: "test-meeting", Title: "Original"}
	data, _ := json.Marshal(meeting)
	meetings.data["test-meeting"] = data
	meetings.revisions["test-meeting"] = 5
	
	// Set up mock to return wrong sequence error for update
	meetings.updateError = errors.New("wrong last sequence")
	
	// Try to update with wrong revision
	meeting.Title = "Updated"
	err := repo.UpdateMeeting(ctx, meeting, 3) // Wrong revision
	if err != domain.ErrRevisionMismatch {
		t.Errorf("expected ErrRevisionMismatch, got: %v", err)
	}
}

func TestNatsRepository_DeleteMeeting(t *testing.T) {
	meetings := newMockNatsKeyValue()
	repo := NewNatsRepository(meetings, nil)
	
	ctx := context.Background()
	
	// Add a meeting
	meetings.data["test-meeting"] = []byte(`{"uid":"test-meeting"}`)
	meetings.revisions["test-meeting"] = 3
	
	// Delete the meeting
	err := repo.DeleteMeeting(ctx, "test-meeting", 3)
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	
	// Verify deletion
	_, exists := meetings.data["test-meeting"]
	if exists {
		t.Error("expected meeting to be deleted")
	}
}

func TestNatsRepository_DeleteMeeting_RevisionMismatch(t *testing.T) {
	meetings := newMockNatsKeyValue()
	repo := NewNatsRepository(meetings, nil)
	
	ctx := context.Background()
	
	// Add a meeting
	meetings.data["test-meeting"] = []byte(`{"uid":"test-meeting"}`)
	meetings.revisions["test-meeting"] = 5
	
	// Set up mock to return wrong sequence error
	meetings.deleteError = errors.New("wrong last sequence")
	
	// Try to delete with wrong revision
	err := repo.DeleteMeeting(ctx, "test-meeting", 3)
	if err != domain.ErrRevisionMismatch {
		t.Errorf("expected ErrRevisionMismatch, got: %v", err)
	}
	
	// Verify not deleted
	_, exists := meetings.data["test-meeting"]
	if !exists {
		t.Error("expected meeting to still exist")
	}
}

func TestNatsRepository_ListAllMeetings(t *testing.T) {
	meetings := newMockNatsKeyValue()
	repo := NewNatsRepository(meetings, nil)
	
	ctx := context.Background()
	
	// Test empty list
	meetingList, err := repo.ListAllMeetings(ctx)
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	if len(meetingList) != 0 {
		t.Errorf("expected empty list, got %d meetings", len(meetingList))
	}
	
	// Add some meetings
	meeting1 := &models.Meeting{UID: "meeting-1", Title: "Meeting 1"}
	meeting2 := &models.Meeting{UID: "meeting-2", Title: "Meeting 2"}
	
	data1, _ := json.Marshal(meeting1)
	data2, _ := json.Marshal(meeting2)
	
	meetings.data["meeting-1"] = data1
	meetings.data["meeting-2"] = data2
	
	// Test list with meetings
	meetingList, err = repo.ListAllMeetings(ctx)
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	if len(meetingList) != 2 {
		t.Errorf("expected 2 meetings, got %d", len(meetingList))
	}
}