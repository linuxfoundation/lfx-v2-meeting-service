// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
	"github.com/nats-io/nats.go/jetstream"
)

// NATS Key-Value store bucket names.
const (
	// KVStoreNameMeetingRegistrants is the name of the KV store for meeting registrants.
	KVStoreNameMeetingRegistrants = "meeting-registrants"
)

// NATS Key-Value store keys.
var (
	KeyRegistrant   = "registrant"
	KeyIndex        = "index"
	KeyIndexMeeting = "meeting"
	KeyIndexEmail   = "email"
)

// NatsRegistrantRepository is the NATS KV store repository for registrants.
//
// TODO: refactor this to implement an interface for the repository that defines
// the functions that should be implemented by each repository regarding indices
// and CRUD operations on the NATS KV store. There will be more repositories in
// the future for past meeting, invitee, ai summary data, etc. so it should be
// easy to extend.
type NatsRegistrantRepository struct {
	MeetingRegistrants INatsKeyValue
}

// NewNatsRegistrantRepository creates a new NATS KV store repository for registrants.
func NewNatsRegistrantRepository(meetingRegistrants INatsKeyValue) *NatsRegistrantRepository {
	return &NatsRegistrantRepository{
		MeetingRegistrants: meetingRegistrants,
	}
}

func (s *NatsRegistrantRepository) IsReady(ctx context.Context) bool {
	return s.MeetingRegistrants != nil
}

func (s *NatsRegistrantRepository) getRegistrantKey(registrantUID string) string {
	key := fmt.Sprintf("%s/%s", KeyRegistrant, registrantUID)
	encodedKey, err := encodeKey(key)
	if err != nil {
		slog.Error("error encoding registrant key", logging.ErrKey, err, "key", key)
		return ""
	}
	return encodedKey
}

// ListByMeeting lists all registrants for a given meeting.
func (s *NatsRegistrantRepository) ListByMeeting(ctx context.Context, meetingUID string) ([]*models.Registrant, error) {
	if !s.IsReady(ctx) {
		return nil, domain.ErrServiceUnavailable
	}

	// Use the meeting index to efficiently find registrants for this meeting
	indexKey := fmt.Sprintf("/%s", s.formIndexKey(KeyIndexMeeting, meetingUID, ""))
	lister, err := s.MeetingRegistrants.ListKeys(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "error listing registrant index keys from NATS KV store", logging.ErrKey, err)
		return nil, domain.ErrInternal
	}

	var registrants []*models.Registrant
	for key := range lister.Keys() {
		decodedKey, err := decodeKey(key)
		if err != nil {
			continue
		}

		// To be able to extract the registrant UID from the index key,
		// we need to split the key into its four parts.
		parts := strings.Split(decodedKey, "/")
		if len(parts) != 5 {
			// It is not an index key if it doesn't have 5 parts.
			continue
		}
		if !strings.HasPrefix(decodedKey, indexKey) {
			continue
		}
		registrantUID := parts[4]

		registrant, err := s.Get(ctx, registrantUID)
		if err != nil {
			if errors.Is(err, domain.ErrRegistrantNotFound) {
				slog.WarnContext(ctx, "stale index entry found, registrant no longer exists", "registrantUID", registrantUID)
			} else {
				slog.ErrorContext(ctx, "error getting registrant", "registrantUID", registrantUID, logging.ErrKey, err)
			}
			continue
		}

		registrants = append(registrants, registrant)
	}

	return registrants, nil
}

// ListByEmail lists all registrants for a given email address.
func (s *NatsRegistrantRepository) ListByEmail(ctx context.Context, email string) ([]*models.Registrant, error) {
	if !s.IsReady(ctx) {
		return nil, domain.ErrServiceUnavailable
	}

	// Use the meeting index to efficiently find registrants for this meeting
	indexKey := fmt.Sprintf("/%s", s.formIndexKey(KeyIndexEmail, email, ""))
	lister, err := s.MeetingRegistrants.ListKeys(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "error listing registrant index keys from NATS KV store", logging.ErrKey, err)
		return nil, domain.ErrInternal
	}

	var registrants []*models.Registrant
	for key := range lister.Keys() {
		decodedKey, err := decodeKey(key)
		if err != nil {
			continue
		}

		// To be able to extract the registrant UID from the index key,
		// we need to split the key into its four parts.
		parts := strings.Split(decodedKey, "/")
		if len(parts) != 5 {
			// It is not an index key if it doesn't have 5 parts.
			continue
		}
		if !strings.HasPrefix(decodedKey, indexKey) {
			slog.Debug("registrant bucket: index key does not match", "indexKey", indexKey, "decodedKey", decodedKey)
			continue
		}
		slog.Debug("registrant bucket: determined registrant UID", "registrantUID", parts[4])
		registrantUID := parts[4]

		registrant, err := s.Get(ctx, registrantUID)
		if err != nil {
			if errors.Is(err, domain.ErrRegistrantNotFound) {
				slog.WarnContext(ctx, "stale index entry found, registrant no longer exists", "registrantUID", registrantUID)
			} else {
				slog.ErrorContext(ctx, "error getting registrant", "registrantUID", registrantUID, logging.ErrKey, err)
			}
			continue
		}

		registrants = append(registrants, registrant)
	}

	return registrants, nil
}

func (s *NatsRegistrantRepository) put(ctx context.Context, registrant *models.Registrant) error {
	jsonData, err := json.Marshal(registrant)
	if err != nil {
		slog.ErrorContext(ctx, "error marshaling registrant", logging.ErrKey, err)
		return domain.ErrInternal
	}

	key := s.getRegistrantKey(registrant.UID)
	_, err = s.MeetingRegistrants.Put(ctx, key, jsonData)
	if err != nil {
		slog.ErrorContext(ctx, "error putting registrant into NATS KV store", logging.ErrKey, err)
		return domain.ErrInternal
	}

	return nil
}

func (s *NatsRegistrantRepository) putIndex(ctx context.Context, indexKey string) error {
	encodedKey, err := encodeKey(indexKey)
	if err != nil {
		slog.ErrorContext(ctx, "error encoding index key", logging.ErrKey, err, "key", indexKey)
		return domain.ErrInternal
	}
	_, err = s.MeetingRegistrants.Put(ctx, encodedKey, []byte{})
	if err != nil {
		slog.ErrorContext(ctx, "error putting index into NATS KV store", logging.ErrKey, err,
			"indexKey", indexKey,
			"encodedKey", encodedKey,
		)
		return domain.ErrInternal
	}

	return nil
}

func (s *NatsRegistrantRepository) formIndexKey(indexType string, indexValue string, objectUID string) string {
	return fmt.Sprintf("%s/%s/%s/%s", KeyIndex, indexType, indexValue, objectUID)
}

func (s *NatsRegistrantRepository) putIndexMeeting(ctx context.Context, registrantUID string, meetingUID string) error {
	indexKey := s.formIndexKey("meeting", meetingUID, registrantUID)
	err := s.putIndex(ctx, indexKey)
	if err != nil {
		slog.ErrorContext(ctx, "error putting meeting index into NATS KV store", logging.ErrKey, err,
			"key", indexKey,
		)
		return err
	}
	slog.Info("registrant bucket: created meeting index key", "key", indexKey)

	return nil
}

func (s *NatsRegistrantRepository) putIndexEmail(ctx context.Context, registrantUID string, email string) error {
	indexKey := s.formIndexKey("email", email, registrantUID)
	err := s.putIndex(ctx, indexKey)
	if err != nil {
		slog.ErrorContext(ctx, "error putting email index into NATS KV store", logging.ErrKey, err,
			"key", indexKey,
		)
		return err
	}
	slog.Info("registrant bucket: created email index key", "key", indexKey)

	return nil
}

func (s *NatsRegistrantRepository) createIndices(ctx context.Context, registrant *models.Registrant) error {
	err := s.putIndexMeeting(ctx, registrant.UID, registrant.MeetingUID)
	if err != nil {
		return err
	}

	// Only create email index if email is not empty
	if registrant.Email != "" {
		err = s.putIndexEmail(ctx, registrant.UID, registrant.Email)
		if err != nil {
			return err
		}
	} else {
		slog.DebugContext(ctx, "skipping email index creation for registrant with empty email", "registrant_uid", registrant.UID)
	}

	return nil
}

// Create creates a new registrant.
func (s *NatsRegistrantRepository) Create(ctx context.Context, registrant *models.Registrant) error {
	if !s.IsReady(ctx) {
		return domain.ErrServiceUnavailable
	}

	// Check if registrant already exists
	exists, err := s.Exists(ctx, registrant.UID)
	if err != nil {
		return err
	}
	if exists {
		return domain.ErrRegistrantAlreadyExists
	}

	// TODO: handle atomicity of the put and index operations.
	err = s.put(ctx, registrant)
	if err != nil {
		return err
	}

	err = s.createIndices(ctx, registrant)
	if err != nil {
		return err
	}

	return nil
}

// RegistrantExists checks if a registrant exists.
func (s *NatsRegistrantRepository) Exists(ctx context.Context, registrantUID string) (bool, error) {
	if !s.IsReady(ctx) {
		return false, domain.ErrServiceUnavailable
	}

	key := s.getRegistrantKey(registrantUID)
	_, err := s.MeetingRegistrants.Get(ctx, key)
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// ExistsByMeetingAndEmail checks if a registrant exists for a given meeting and email.
func (s *NatsRegistrantRepository) ExistsByMeetingAndEmail(ctx context.Context, meetingUID, email string) (bool, error) {
	if !s.IsReady(ctx) {
		return false, domain.ErrServiceUnavailable
	}

	// List all registrants for the meeting
	registrants, err := s.ListByMeeting(ctx, meetingUID)
	if err != nil {
		return false, err
	}

	// Check if any registrant has the given email
	for _, registrant := range registrants {
		if registrant.Email == email {
			return true, nil
		}
	}

	return false, nil
}

// Get gets a registrant by its UID.
func (s *NatsRegistrantRepository) Get(ctx context.Context, registrantUID string) (*models.Registrant, error) {
	registrant, _, err := s.GetWithRevision(ctx, registrantUID)
	if err != nil {
		return nil, err
	}
	return registrant, nil
}

// GetWithRevision gets a registrant by its UID and returns the revision.
func (s *NatsRegistrantRepository) GetWithRevision(ctx context.Context, registrantUID string) (*models.Registrant, uint64, error) {
	if !s.IsReady(ctx) {
		return nil, 0, domain.ErrServiceUnavailable
	}

	key := s.getRegistrantKey(registrantUID)
	entry, err := s.MeetingRegistrants.Get(ctx, key)
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			slog.WarnContext(ctx, "registrant not found", logging.ErrKey, domain.ErrRegistrantNotFound)
			return nil, 0, domain.ErrRegistrantNotFound
		}
		slog.ErrorContext(ctx, "error getting registrant from NATS KV", logging.ErrKey, err)
		return nil, 0, err
	}

	var registrant models.Registrant
	err = json.Unmarshal(entry.Value(), &registrant)
	if err != nil {
		slog.ErrorContext(ctx, "error unmarshaling registrant", logging.ErrKey, err)
		return nil, 0, domain.ErrUnmarshal
	}

	return &registrant, entry.Revision(), nil
}

// Update updates a registrant.
func (s *NatsRegistrantRepository) Update(ctx context.Context, registrant *models.Registrant, revision uint64) error {
	if !s.IsReady(ctx) {
		return domain.ErrServiceUnavailable
	}

	// Get the old registrant to check if indexes need updating
	oldRegistrant, _, err := s.GetWithRevision(ctx, registrant.UID)
	if err != nil {
		return err
	}

	jsonData, err := json.Marshal(registrant)
	if err != nil {
		slog.ErrorContext(ctx, "error marshaling registrant", logging.ErrKey, err)
		return domain.ErrInternal
	}

	key := s.getRegistrantKey(registrant.UID)
	_, err = s.MeetingRegistrants.Update(ctx, key, jsonData, revision)
	if err != nil {
		if strings.Contains(err.Error(), "wrong last sequence") {
			slog.WarnContext(ctx, "revision mismatch", logging.ErrKey, err)
			return domain.ErrRevisionMismatch
		}
		slog.ErrorContext(ctx, "error updating registrant in NATS KV store", logging.ErrKey, err)
		return domain.ErrInternal
	}

	// Update indexes if email or meeting changed
	// TODO: handle atomicity of update and index operations
	if oldRegistrant.Email != registrant.Email || oldRegistrant.MeetingUID != registrant.MeetingUID {
		// Delete old indexes
		err = s.deleteIndices(ctx, oldRegistrant)
		if err != nil {
			return err
		}

		// Create new indexes
		err = s.createIndices(ctx, registrant)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *NatsRegistrantRepository) deleteIndices(ctx context.Context, registrant *models.Registrant) error {
	// Delete meeting index
	meetingIndexKey := s.formIndexKey("meeting", registrant.MeetingUID, registrant.UID)
	encodedMeetingIndexKey, err := encodeKey(meetingIndexKey)
	if err != nil {
		slog.ErrorContext(ctx, "error encoding meeting index key", logging.ErrKey, err, "raw_key", meetingIndexKey)
		return domain.ErrInternal
	}

	err = s.MeetingRegistrants.Delete(ctx, encodedMeetingIndexKey)
	if err != nil && !errors.Is(err, jetstream.ErrKeyNotFound) {
		slog.ErrorContext(ctx, "error deleting meeting index",
			logging.ErrKey, err,
			"raw_key", meetingIndexKey,
			"encoded_key", encodedMeetingIndexKey,
		)
		return domain.ErrInternal
	}

	// Only delete email index if email is not empty
	if registrant.Email != "" {
		emailIndexKey := s.formIndexKey("email", registrant.Email, registrant.UID)
		encodedEmailIndexKey, err := encodeKey(emailIndexKey)
		if err != nil {
			slog.ErrorContext(ctx, "error encoding email index key", logging.ErrKey, err, "raw_key", emailIndexKey)
			return domain.ErrInternal
		}

		err = s.MeetingRegistrants.Delete(ctx, encodedEmailIndexKey)
		if err != nil && !errors.Is(err, jetstream.ErrKeyNotFound) {
			slog.ErrorContext(ctx, "error deleting email index",
				logging.ErrKey, err,
				"raw_key", emailIndexKey,
				"encoded_key", encodedEmailIndexKey,
			)
			return domain.ErrInternal
		}
	} else {
		slog.DebugContext(ctx, "skipping email index deletion for registrant with empty email", "registrant_uid", registrant.UID)
	}

	return nil
}

// Delete deletes a registrant.
func (s *NatsRegistrantRepository) Delete(ctx context.Context, registrantUID string, revision uint64) error {
	if !s.IsReady(ctx) {
		return domain.ErrServiceUnavailable
	}

	// Get registrant first to clean up indexes
	registrant, _, err := s.GetWithRevision(ctx, registrantUID)
	if err != nil {
		return err
	}

	key := s.getRegistrantKey(registrantUID)
	err = s.MeetingRegistrants.Delete(ctx, key, jetstream.LastRevision(revision))
	if err != nil {
		if strings.Contains(err.Error(), "wrong last sequence") {
			slog.WarnContext(ctx, "revision mismatch", logging.ErrKey, err)
			return domain.ErrRevisionMismatch
		}
		slog.ErrorContext(ctx, "error deleting registrant from NATS KV store", logging.ErrKey, err)
		return domain.ErrInternal
	}

	// Clean up indexes
	// TODO: handle atomicity of delete and index cleanup operations
	err = s.deleteIndices(ctx, registrant)
	if err != nil {
		// Log but don't fail the delete since the main data is already deleted
		slog.ErrorContext(ctx, "error cleaning up registrant indexes", logging.ErrKey, err)
	}

	return nil
}
