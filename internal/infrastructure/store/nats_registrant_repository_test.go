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

func TestNewNatsRegistrantRepository(t *testing.T) {
	meetingRegistrants := &mockNatsKeyValue{}

	repo := NewNatsRegistrantRepository(meetingRegistrants)

	if repo == nil {
		t.Fatal("expected repository to be created")
	}
	if repo.MeetingRegistrants != meetingRegistrants {
		t.Error("expected MeetingRegistrants to be set correctly")
	}
}

func TestNatsRegistrantRepository_CreateRegistrant(t *testing.T) {
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

	err := repo.CreateRegistrant(context.Background(), registrant)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Verify the registrant was stored
	key := registrant.UID
	storedData, exists := meetingRegistrants.data[key]
	if !exists {
		t.Error("expected registrant to be stored")
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

func TestNatsRegistrantRepository_CreateRegistrant_AlreadyExists(t *testing.T) {
	meetingRegistrants := &mockNatsKeyValue{}
	repo := NewNatsRegistrantRepository(meetingRegistrants)

	registrant := &models.Registrant{
		UID:        "existing-registrant",
		MeetingUID: "meeting-123",
		Email:      "user@example.com",
		FirstName:  "John",
		LastName:   "Doe",
	}

	// Add existing registrant
	registrantData, _ := json.Marshal(registrant)
	meetingRegistrants.data = map[string][]byte{
		registrant.UID: registrantData,
	}

	err := repo.CreateRegistrant(context.Background(), registrant)
	if err == nil {
		t.Error("expected error but got nil")
	}
	if err != domain.ErrRegistrantAlreadyExists {
		t.Errorf("expected ErrRegistrantAlreadyExists, got %v", err)
	}
}

func TestNatsRegistrantRepository_GetRegistrant(t *testing.T) {
	meetingRegistrants := &mockNatsKeyValue{}
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
	meetingRegistrants.data = map[string][]byte{
		registrant.UID: registrantData,
	}

	result, err := repo.GetRegistrant(context.Background(), registrant.UID)
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

func TestNatsRegistrantRepository_GetRegistrant_NotFound(t *testing.T) {
	meetingRegistrants := &mockNatsKeyValue{}
	repo := NewNatsRegistrantRepository(meetingRegistrants)

	_, err := repo.GetRegistrant(context.Background(), "non-existent")
	if err == nil {
		t.Error("expected error but got nil")
	}
	if err != domain.ErrRegistrantNotFound {
		t.Errorf("expected ErrRegistrantNotFound, got %v", err)
	}
}

func TestNatsRegistrantRepository_GetRegistrantWithRevision(t *testing.T) {
	meetingRegistrants := &mockNatsKeyValue{}
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
	meetingRegistrants.data = map[string][]byte{
		registrant.UID: registrantData,
	}
	meetingRegistrants.revisions = map[string]uint64{
		registrant.UID: expectedRevision,
	}

	result, revision, err := repo.GetRegistrantWithRevision(context.Background(), registrant.UID)
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

func TestNatsRegistrantRepository_UpdateRegistrant(t *testing.T) {
	meetingRegistrants := &mockNatsKeyValue{}
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

	// Set up existing registrant
	initialData, _ := json.Marshal(registrant)
	initialRevision := uint64(1)
	meetingRegistrants.data = map[string][]byte{
		registrant.UID: initialData,
	}
	meetingRegistrants.revisions = map[string]uint64{
		registrant.UID: initialRevision,
	}

	err := repo.UpdateRegistrant(context.Background(), registrant, initialRevision)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Verify the registrant was updated
	revision := meetingRegistrants.revisions[registrant.UID]
	if revision != initialRevision+1 {
		t.Errorf("expected revision to be incremented to %d, got %d", initialRevision+1, revision)
	}
}

func TestNatsRegistrantRepository_UpdateRegistrant_RevisionMismatch(t *testing.T) {
	meetingRegistrants := &mockNatsKeyValue{}
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
	meetingRegistrants.data = map[string][]byte{
		registrant.UID: initialData,
	}
	meetingRegistrants.revisions = map[string]uint64{
		registrant.UID: initialRevision,
	}

	// Try to update with wrong revision
	wrongRevision := uint64(3)
	err := repo.UpdateRegistrant(context.Background(), registrant, wrongRevision)
	if err == nil {
		t.Error("expected error but got nil")
	}
	if err != domain.ErrRevisionMismatch {
		t.Errorf("expected ErrRevisionMismatch, got %v", err)
	}
}

func TestNatsRegistrantRepository_DeleteRegistrant(t *testing.T) {
	meetingRegistrants := &mockNatsKeyValue{}
	repo := NewNatsRegistrantRepository(meetingRegistrants)

	registrantUID := "registrant-123"
	revision := uint64(1)

	// Set up existing registrant
	meetingRegistrants.data = map[string][]byte{
		registrantUID: []byte(`{"uid":"registrant-123","email":"user@example.com"}`),
	}

	err := repo.DeleteRegistrant(context.Background(), registrantUID, revision)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Verify registrant was deleted
	if _, exists := meetingRegistrants.data[registrantUID]; exists {
		t.Error("expected registrant to be deleted")
	}
}

func TestNatsRegistrantRepository_DeleteRegistrant_RevisionMismatch(t *testing.T) {
	meetingRegistrants := &mockNatsKeyValue{deleteError: errors.New("wrong last sequence")}
	repo := NewNatsRegistrantRepository(meetingRegistrants)

	registrantUID := "registrant-123"
	wrongRevision := uint64(3)

	err := repo.DeleteRegistrant(context.Background(), registrantUID, wrongRevision)
	if err == nil {
		t.Error("expected error but got nil")
	}
	if err != domain.ErrRevisionMismatch {
		t.Errorf("expected ErrRevisionMismatch, got %v", err)
	}
}

func TestNatsRegistrantRepository_ListMeetingRegistrants(t *testing.T) {
	meetingRegistrants := &mockNatsKeyValue{}
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

	// Set up registrants
	registrant1Data, _ := json.Marshal(registrant1)
	registrant2Data, _ := json.Marshal(registrant2)
	meetingRegistrants.data = map[string][]byte{
		"registrant-1": registrant1Data,
		"registrant-2": registrant2Data,
	}

	result, err := repo.ListMeetingRegistrants(context.Background(), "meeting-123")
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

func TestNatsRegistrantRepository_RegistrantExists(t *testing.T) {
	meetingRegistrants := &mockNatsKeyValue{}
	repo := NewNatsRegistrantRepository(meetingRegistrants)

	// Test non-existing registrant
	exists, err := repo.RegistrantExists(context.Background(), "non-existent")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if exists {
		t.Error("expected registrant to not exist")
	}

	// Add a registrant
	registrantData := `{"uid":"existing-registrant","email":"user@example.com"}`
	meetingRegistrants.data = map[string][]byte{
		"existing-registrant": []byte(registrantData),
	}

	// Test existing registrant
	exists, err = repo.RegistrantExists(context.Background(), "existing-registrant")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !exists {
		t.Error("expected registrant to exist")
	}
}
