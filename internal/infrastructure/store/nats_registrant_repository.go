// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package store

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
)

// NatsRegistrantRepository is the NATS KV store repository for registrants.
// Note: This version uses the base repository with simplified indexing.
type NatsRegistrantRepository struct {
	*NatsBaseRepository[models.Registrant]
	keyBuilder *KeyBuilder
}

// NewNatsRegistrantRepository creates a new NATS KV store repository for registrants.
func NewNatsRegistrantRepository(kvStore INatsKeyValue) *NatsRegistrantRepository {
	baseRepo := NewNatsBaseRepository[models.Registrant](kvStore, "registrant")
	keyBuilder := NewKeyBuilder("")

	return &NatsRegistrantRepository{
		NatsBaseRepository: baseRepo,
		keyBuilder:         keyBuilder,
	}
}

// IsReady checks if the repository is ready
func (r *NatsRegistrantRepository) IsReady(ctx context.Context) bool {
	return r.NatsBaseRepository.IsReady()
}

// Create creates a new registrant with indexing
func (r *NatsRegistrantRepository) Create(ctx context.Context, registrant *models.Registrant) error {
	// Generate UID if not provided
	if registrant.UID == "" {
		registrant.UID = uuid.New().String()
	}

	key := r.keyBuilder.EntityKeyEncoded(KeyPrefixRegistrant, registrant.UID)
	err := r.NatsBaseRepository.Create(ctx, key, registrant)
	if err != nil {
		return err
	}

	// Create indices (simplified - in full implementation would need all indexing logic)
	if err := r.createIndices(ctx, registrant); err != nil {
		slog.WarnContext(ctx, "failed to create indices", logging.ErrKey, err, "registrant_uid", registrant.UID)
		// Don't fail the operation if indexing fails
	}

	return nil
}

// Exists checks if a registrant exists
func (r *NatsRegistrantRepository) Exists(ctx context.Context, registrantUID string) (bool, error) {
	key := r.keyBuilder.EntityKeyEncoded(KeyPrefixRegistrant, registrantUID)
	return r.NatsBaseRepository.Exists(ctx, key)
}

// Get retrieves a registrant by UID
func (r *NatsRegistrantRepository) Get(ctx context.Context, registrantUID string) (*models.Registrant, error) {
	key := r.keyBuilder.EntityKeyEncoded(KeyPrefixRegistrant, registrantUID)
	return r.NatsBaseRepository.Get(ctx, key)
}

// GetWithRevision retrieves a registrant with revision by UID
func (r *NatsRegistrantRepository) GetWithRevision(ctx context.Context, registrantUID string) (*models.Registrant, uint64, error) {
	key := r.keyBuilder.EntityKeyEncoded(KeyPrefixRegistrant, registrantUID)
	return r.NatsBaseRepository.GetWithRevision(ctx, key)
}

// Update updates an existing registrant
func (r *NatsRegistrantRepository) Update(ctx context.Context, registrant *models.Registrant, revision uint64) error {
	key := r.keyBuilder.EntityKeyEncoded(KeyPrefixRegistrant, registrant.UID)
	return r.NatsBaseRepository.Update(ctx, key, registrant, revision)
}

// Delete removes a registrant
func (r *NatsRegistrantRepository) Delete(ctx context.Context, registrantUID string, revision uint64) error {
	// Get registrant first for index cleanup
	registrant, err := r.Get(ctx, registrantUID)
	if err != nil {
		return err
	}

	// Delete indices (simplified)
	if err := r.deleteIndices(ctx, registrant); err != nil {
		slog.WarnContext(ctx, "failed to delete indices", logging.ErrKey, err, "registrant_uid", registrantUID)
		// Don't fail the operation if index cleanup fails
	}

	key := r.keyBuilder.EntityKeyEncoded(KeyPrefixRegistrant, registrantUID)
	return r.NatsBaseRepository.Delete(ctx, key, revision)
}

// ExistsByMeetingAndEmail checks if a registrant exists by meeting and email
func (r *NatsRegistrantRepository) ExistsByMeetingAndEmail(ctx context.Context, meetingUID, email string) (bool, error) {
	registrants, err := r.ListByMeeting(ctx, meetingUID)
	if err != nil {
		return false, err
	}

	for _, registrant := range registrants {
		if registrant.Email == email {
			return true, nil
		}
	}
	return false, nil
}

// GetByMeetingAndEmail retrieves a registrant by meeting and email
func (r *NatsRegistrantRepository) GetByMeetingAndEmail(ctx context.Context, meetingUID, email string) (*models.Registrant, uint64, error) {
	registrants, err := r.ListByMeeting(ctx, meetingUID)
	if err != nil {
		return nil, 0, err
	}

	for _, registrant := range registrants {
		if registrant.Email == email {
			// Get with revision
			return r.GetWithRevision(ctx, registrant.UID)
		}
	}

	return nil, 0, domain.NewNotFoundError(fmt.Sprintf("registrant with meeting '%s' and email '%s' not found", meetingUID, email))
}

// GetByMeetingAndUsername retrieves a registrant by meeting and username
func (r *NatsRegistrantRepository) GetByMeetingAndUsername(ctx context.Context, meetingUID, username string) (*models.Registrant, uint64, error) {
	registrants, err := r.ListByMeeting(ctx, meetingUID)
	if err != nil {
		return nil, 0, err
	}

	for _, registrant := range registrants {
		if registrant.Username == username {
			// Get with revision
			return r.GetWithRevision(ctx, registrant.UID)
		}
	}

	return nil, 0, domain.NewNotFoundError(fmt.Sprintf("registrant with meeting '%s' and username '%s' not found", meetingUID, username))
}

// ListByMeeting retrieves all registrants for a meeting
func (r *NatsRegistrantRepository) ListByMeeting(ctx context.Context, meetingUID string) ([]*models.Registrant, error) {
	allRegistrants, err := r.ListAll(ctx)
	if err != nil {
		return nil, err
	}

	var matchingRegistrants []*models.Registrant
	for _, registrant := range allRegistrants {
		if registrant.MeetingUID == meetingUID {
			matchingRegistrants = append(matchingRegistrants, registrant)
		}
	}

	return matchingRegistrants, nil
}

// ListByEmail retrieves all registrants with a specific email
func (r *NatsRegistrantRepository) ListByEmail(ctx context.Context, email string) ([]*models.Registrant, error) {
	allRegistrants, err := r.ListAll(ctx)
	if err != nil {
		return nil, err
	}

	var matchingRegistrants []*models.Registrant
	for _, registrant := range allRegistrants {
		if registrant.Email == email {
			matchingRegistrants = append(matchingRegistrants, registrant)
		}
	}

	return matchingRegistrants, nil
}

// ListByEmailAndCommittee retrieves all registrants with a specific email and committee
func (r *NatsRegistrantRepository) ListByEmailAndCommittee(ctx context.Context, email string, committeeUID string) ([]*models.Registrant, error) {
	registrantsByEmail, err := r.ListByEmail(ctx, email)
	if err != nil {
		return nil, err
	}

	var matchingRegistrants []*models.Registrant
	for _, registrant := range registrantsByEmail {
		if registrant.Type == models.RegistrantTypeCommittee &&
			registrant.CommitteeUID != nil &&
			*registrant.CommitteeUID == committeeUID {
			matchingRegistrants = append(matchingRegistrants, registrant)
		}
	}

	return matchingRegistrants, nil
}

// ListAll lists all registrants
func (r *NatsRegistrantRepository) ListAll(ctx context.Context) ([]*models.Registrant, error) {
	pattern := KeyPrefixRegistrant + "/"
	return r.ListEntitiesEncoded(ctx, pattern, r.keyBuilder)
}

func (r *NatsRegistrantRepository) createIndices(ctx context.Context, registrant *models.Registrant) error {
	// Create meeting index
	meetingIndexKey := r.keyBuilder.IndexKeyEncoded(KeyPrefixIndexMeeting, registrant.MeetingUID, registrant.UID)
	if _, err := r.kvStore.Put(ctx, meetingIndexKey, []byte{}); err != nil {
		return err
	}

	if registrant.Email != "" {
		// Create email index
		emailIndexKey := r.keyBuilder.IndexKeyEncoded(KeyPrefixIndexEmail, registrant.Email, registrant.UID)
		if _, err := r.kvStore.Put(ctx, emailIndexKey, []byte{}); err != nil {
			return err
		}
	} else {
		slog.DebugContext(ctx, "skipping email index creation for registrant with empty email", "registrant_uid", registrant.UID)
	}

	return nil
}

func (r *NatsRegistrantRepository) deleteIndices(ctx context.Context, registrant *models.Registrant) error {
	// Delete meeting index
	meetingIndexKey := r.keyBuilder.IndexKeyEncoded(KeyPrefixIndexMeeting, registrant.MeetingUID, registrant.UID)
	if err := r.kvStore.Delete(ctx, meetingIndexKey); err != nil {
		slog.WarnContext(ctx, "failed to delete meeting index", logging.ErrKey, err)
	}

	if registrant.Email != "" {
		// Delete email index
		emailIndexKey := r.keyBuilder.IndexKeyEncoded(KeyPrefixIndexEmail, registrant.Email, registrant.UID)
		if err := r.kvStore.Delete(ctx, emailIndexKey); err != nil {
			slog.WarnContext(ctx, "failed to delete email index", logging.ErrKey, err)
		}
	}

	return nil
}
