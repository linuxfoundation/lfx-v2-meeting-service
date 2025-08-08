// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
)

func TestNewNatsRegistrantRepository(t *testing.T) {
	meetingRegistrants := newMockNatsKeyValue()

	repo := NewNatsRegistrantRepository(meetingRegistrants)

	if repo == nil {
		t.Fatal("expected repository to be created")
	}
	if repo.MeetingRegistrants != meetingRegistrants {
		t.Error("expected MeetingRegistrants to be set correctly")
	}
}

func TestNatsRegistrantRepository_Create(t *testing.T) {
	meetingRegistrants := newMockNatsKeyValue()
	repo := NewNatsRegistrantRepository(meetingRegistrants)

	now := time.Now()
	registrant := &models.Registrant{
		UID:        "registrant-123",
		MeetingUID: "meeting-123",
		Email:      "user@example.com",
		FirstName:  "John",
		LastName:   "Doe",
		CreatedAt:  &now,
		UpdatedAt:  &now,
	}

	err := repo.Create(context.Background(), registrant)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Verify the registrant was stored with encoded key
	// The repository uses getRegistrantKey which returns an already encoded key
	// So we need to match exactly what the repository passes to Put
	key := fmt.Sprintf("%s/%s", "registrant", registrant.UID)
	encodedKey, _ := encodeKey(key)
	storedData, exists := meetingRegistrants.data[encodedKey]
	if !exists {
		t.Error("expected registrant to be stored")
		return
	}

	var storedRegistrant models.Registrant
	if err := json.Unmarshal(storedData, &storedRegistrant); err != nil {
		t.Errorf("failed to unmarshal stored registrant: %v", err)
	}

	if storedRegistrant.UID != registrant.UID {
		t.Errorf("expected UID %s, got %s", registrant.UID, storedRegistrant.UID)
	}
	if storedRegistrant.Email != registrant.Email {
		t.Errorf("expected Email %s, got %s", registrant.Email, storedRegistrant.Email)
	}
}

func TestNatsRegistrantRepository_Create_AlreadyExists(t *testing.T) {
	meetingRegistrants := newMockNatsKeyValue()
	repo := NewNatsRegistrantRepository(meetingRegistrants)

	registrant := &models.Registrant{
		UID:        "existing-registrant",
		MeetingUID: "meeting-123",
		Email:      "user@example.com",
		FirstName:  "John",
		LastName:   "Doe",
	}

	// Add existing registrant with encoded key
	registrantData, _ := json.Marshal(registrant)
	key := fmt.Sprintf("registrant/%s", registrant.UID)
	encodedKey, _ := encodeKey(key)
	meetingRegistrants.data = map[string][]byte{
		encodedKey: registrantData,
	}

	err := repo.Create(context.Background(), registrant)
	if err == nil {
		t.Error("expected error but got nil")
	}
	if err != domain.ErrRegistrantAlreadyExists {
		t.Errorf("expected ErrRegistrantAlreadyExists, got %v", err)
	}
}

func TestNatsRegistrantRepository_Get(t *testing.T) {
	meetingRegistrants := newMockNatsKeyValue()
	repo := NewNatsRegistrantRepository(meetingRegistrants)

	now := time.Now()
	registrant := &models.Registrant{
		UID:        "registrant-123",
		MeetingUID: "meeting-123",
		Email:      "user@example.com",
		FirstName:  "John",
		LastName:   "Doe",
		CreatedAt:  &now,
		UpdatedAt:  &now,
	}

	registrantData, _ := json.Marshal(registrant)
	key := fmt.Sprintf("registrant/%s", registrant.UID)
	encodedKey, _ := encodeKey(key)
	meetingRegistrants.data = map[string][]byte{
		encodedKey: registrantData,
	}

	result, err := repo.Get(context.Background(), registrant.UID)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result.UID != registrant.UID {
		t.Errorf("expected UID %s, got %s", registrant.UID, result.UID)
	}
	if result.Email != registrant.Email {
		t.Errorf("expected Email %s, got %s", registrant.Email, result.Email)
	}
}

func TestNatsRegistrantRepository_Get_NotFound(t *testing.T) {
	meetingRegistrants := newMockNatsKeyValue()
	repo := NewNatsRegistrantRepository(meetingRegistrants)

	_, err := repo.Get(context.Background(), "non-existent")
	if err == nil {
		t.Error("expected error but got nil")
	}
	if err != domain.ErrRegistrantNotFound {
		t.Errorf("expected ErrRegistrantNotFound, got %v", err)
	}
}

func TestNatsRegistrantRepository_GetWithRevision(t *testing.T) {
	meetingRegistrants := newMockNatsKeyValue()
	repo := NewNatsRegistrantRepository(meetingRegistrants)

	now := time.Now()
	registrant := &models.Registrant{
		UID:        "registrant-123",
		MeetingUID: "meeting-123",
		Email:      "user@example.com",
		FirstName:  "John",
		LastName:   "Doe",
		CreatedAt:  &now,
		UpdatedAt:  &now,
	}

	registrantData, _ := json.Marshal(registrant)
	expectedRevision := uint64(42)
	key := fmt.Sprintf("registrant/%s", registrant.UID)
	encodedKey, _ := encodeKey(key)
	meetingRegistrants.data = map[string][]byte{
		encodedKey: registrantData,
	}
	meetingRegistrants.revisions = map[string]uint64{
		encodedKey: expectedRevision,
	}

	result, revision, err := repo.GetWithRevision(context.Background(), registrant.UID)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if revision != expectedRevision {
		t.Errorf("expected revision %d, got %d", expectedRevision, revision)
	}
	if result.UID != registrant.UID {
		t.Errorf("expected UID %s, got %s", registrant.UID, result.UID)
	}
}

func TestNatsRegistrantRepository_Update(t *testing.T) {
	meetingRegistrants := newMockNatsKeyValue()
	repo := NewNatsRegistrantRepository(meetingRegistrants)

	now := time.Now()
	registrant := &models.Registrant{
		UID:        "registrant-123",
		MeetingUID: "meeting-123",
		Email:      "updated@example.com",
		FirstName:  "John",
		LastName:   "Doe",
		CreatedAt:  &now,
		UpdatedAt:  &now,
	}

	// Set up existing registrant with encoded key
	initialData, _ := json.Marshal(registrant)
	initialRevision := uint64(1)
	key := fmt.Sprintf("registrant/%s", registrant.UID)
	encodedKey, _ := encodeKey(key)
	meetingRegistrants.data = map[string][]byte{
		encodedKey: initialData,
	}
	meetingRegistrants.revisions = map[string]uint64{
		encodedKey: initialRevision,
	}

	err := repo.Update(context.Background(), registrant, initialRevision)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Verify the registrant was updated
	key = fmt.Sprintf("registrant/%s", registrant.UID)
	encodedKey, _ = encodeKey(key)
	revision := meetingRegistrants.revisions[encodedKey]
	if revision != initialRevision+1 {
		t.Errorf("expected revision to be incremented to %d, got %d", initialRevision+1, revision)
	}
}

func TestNatsRegistrantRepository_Update_RevisionMismatch(t *testing.T) {
	meetingRegistrants := newMockNatsKeyValue()
	repo := NewNatsRegistrantRepository(meetingRegistrants)

	registrant := &models.Registrant{
		UID:        "registrant-123",
		MeetingUID: "meeting-123",
		Email:      "updated@example.com",
		FirstName:  "John",
		LastName:   "Doe",
	}

	// Set up existing registrant with different revision
	initialData, _ := json.Marshal(registrant)
	initialRevision := uint64(1)
	key := fmt.Sprintf("registrant/%s", registrant.UID)
	encodedKey, _ := encodeKey(key)
	meetingRegistrants.data = map[string][]byte{
		encodedKey: initialData,
	}
	meetingRegistrants.revisions = map[string]uint64{
		encodedKey: initialRevision,
	}

	// Try to update with wrong revision
	wrongRevision := uint64(3)
	err := repo.Update(context.Background(), registrant, wrongRevision)
	if err == nil {
		t.Error("expected error but got nil")
	}
	if err != domain.ErrRevisionMismatch {
		t.Errorf("expected ErrRevisionMismatch, got %v", err)
	}
}

func TestNatsRegistrantRepository_Delete(t *testing.T) {
	meetingRegistrants := newMockNatsKeyValue()
	repo := NewNatsRegistrantRepository(meetingRegistrants)

	registrantUID := "registrant-123"
	revision := uint64(1)

	// Set up existing registrant with encoded key
	key := fmt.Sprintf("registrant/%s", registrantUID)
	encodedKey, _ := encodeKey(key)
	meetingRegistrants.data = map[string][]byte{
		encodedKey: []byte(`{"uid":"registrant-123","email":"user@example.com"}`),
	}

	err := repo.Delete(context.Background(), registrantUID, revision)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Verify registrant was deleted
	if _, exists := meetingRegistrants.data[encodedKey]; exists {
		t.Error("expected registrant to be deleted")
	}
}

func TestNatsRegistrantRepository_Delete_RevisionMismatch(t *testing.T) {
	meetingRegistrants := newMockNatsKeyValue()
	meetingRegistrants.deleteError = errors.New("wrong last sequence")
	repo := NewNatsRegistrantRepository(meetingRegistrants)

	registrantUID := "registrant-123"
	wrongRevision := uint64(3)

	// Set up existing registrant so it can be found first with encoded key
	key := fmt.Sprintf("registrant/%s", registrantUID)
	encodedKey, _ := encodeKey(key)
	meetingRegistrants.data = map[string][]byte{
		encodedKey: []byte(`{"uid":"registrant-123","email":"user@example.com"}`),
	}

	err := repo.Delete(context.Background(), registrantUID, wrongRevision)
	if err == nil {
		t.Error("expected error but got nil")
	}
	if err != domain.ErrRevisionMismatch {
		t.Errorf("expected ErrRevisionMismatch, got %v", err)
	}
}

func TestNatsRegistrantRepository_ListByMeeting(t *testing.T) {
	meetingRegistrants := newMockNatsKeyValue()
	repo := NewNatsRegistrantRepository(meetingRegistrants)

	now := time.Now()
	registrant1 := &models.Registrant{
		UID:        "registrant-1",
		MeetingUID: "meeting-123",
		Email:      "user1@example.com",
		FirstName:  "John",
		LastName:   "Doe",
		CreatedAt:  &now,
		UpdatedAt:  &now,
	}
	registrant2 := &models.Registrant{
		UID:        "registrant-2",
		MeetingUID: "meeting-123",
		Email:      "user2@example.com",
		FirstName:  "Jane",
		LastName:   "Smith",
		CreatedAt:  &now,
		UpdatedAt:  &now,
	}

	// Set up registrants with encoded keys and indices
	registrant1Data, _ := json.Marshal(registrant1)
	registrant2Data, _ := json.Marshal(registrant2)

	// Encode registrant keys
	reg1Key, _ := encodeKey(fmt.Sprintf("registrant/%s", registrant1.UID))
	reg2Key, _ := encodeKey(fmt.Sprintf("registrant/%s", registrant2.UID))

	// Encode index keys for meeting
	index1Key, _ := encodeKey(fmt.Sprintf("index/meeting/%s/%s", registrant1.MeetingUID, registrant1.UID))
	index2Key, _ := encodeKey(fmt.Sprintf("index/meeting/%s/%s", registrant2.MeetingUID, registrant2.UID))

	meetingRegistrants.data = map[string][]byte{
		reg1Key:   registrant1Data,
		reg2Key:   registrant2Data,
		index1Key: {},
		index2Key: {},
	}

	result, err := repo.ListByMeeting(context.Background(), "meeting-123")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(result) != 2 {
		t.Errorf("expected 2 registrants, got %d", len(result))
	}

	// Check if both registrants are present
	foundRegistrant1 := false
	foundRegistrant2 := false
	for _, registrant := range result {
		if registrant.UID == "registrant-1" && registrant.Email == "user1@example.com" {
			foundRegistrant1 = true
		}
		if registrant.UID == "registrant-2" && registrant.Email == "user2@example.com" {
			foundRegistrant2 = true
		}
	}

	if !foundRegistrant1 {
		t.Error("expected to find registrant-1")
	}
	if !foundRegistrant2 {
		t.Error("expected to find registrant-2")
	}
}

func TestNatsRegistrantRepository_Exists(t *testing.T) {
	meetingRegistrants := newMockNatsKeyValue()
	repo := NewNatsRegistrantRepository(meetingRegistrants)

	// Test non-existing registrant
	exists, err := repo.Exists(context.Background(), "non-existent")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if exists {
		t.Error("expected registrant to not exist")
	}

	// Add a registrant with encoded key
	registrantData := `{"uid":"existing-registrant","email":"user@example.com"}`
	key := fmt.Sprintf("registrant/%s", "existing-registrant")
	encodedKey, _ := encodeKey(key)
	meetingRegistrants.data = map[string][]byte{
		encodedKey: []byte(registrantData),
	}

	// Test existing registrant
	exists, err = repo.Exists(context.Background(), "existing-registrant")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !exists {
		t.Error("expected registrant to exist")
	}
}
