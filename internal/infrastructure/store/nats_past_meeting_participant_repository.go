// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package store

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
	"github.com/nats-io/nats.go/jetstream"
)

// NATS Key-Value store bucket name for past meeting participants.
const (
	// KVStoreNamePastMeetingParticipants is the name of the KV store for past meeting participants.
	KVStoreNamePastMeetingParticipants = "past-meeting-participants"
)

// NatsPastMeetingParticipantRepository is the NATS KV store repository for past meeting participants.
type NatsPastMeetingParticipantRepository struct {
	PastMeetingParticipants INatsKeyValue
}

// NewNatsPastMeetingParticipantRepository creates a new NATS KV store repository for past meeting participants.
func NewNatsPastMeetingParticipantRepository(pastMeetingParticipants INatsKeyValue) *NatsPastMeetingParticipantRepository {
	return &NatsPastMeetingParticipantRepository{
		PastMeetingParticipants: pastMeetingParticipants,
	}
}

func (s *NatsPastMeetingParticipantRepository) get(ctx context.Context, participantUID string) (jetstream.KeyValueEntry, error) {
	return s.PastMeetingParticipants.Get(ctx, participantUID)
}

func (s *NatsPastMeetingParticipantRepository) unmarshal(ctx context.Context, entry jetstream.KeyValueEntry) (*models.PastMeetingParticipant, error) {
	var participant models.PastMeetingParticipant
	err := json.Unmarshal(entry.Value(), &participant)
	if err != nil {
		slog.ErrorContext(ctx, "error unmarshaling past meeting participant", logging.ErrKey, err)
		return nil, err
	}

	return &participant, nil
}

func (s *NatsPastMeetingParticipantRepository) Get(ctx context.Context, participantUID string) (*models.PastMeetingParticipant, error) {
	participant, _, err := s.GetWithRevision(ctx, participantUID)
	if err != nil {
		return nil, err
	}
	return participant, nil
}

func (s *NatsPastMeetingParticipantRepository) GetWithRevision(ctx context.Context, participantUID string) (*models.PastMeetingParticipant, uint64, error) {
	entry, err := s.get(ctx, participantUID)
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			slog.WarnContext(ctx, "past meeting participant not found", logging.ErrKey, domain.ErrPastMeetingParticipantNotFound)
			return nil, 0, domain.ErrPastMeetingParticipantNotFound
		}
		slog.ErrorContext(ctx, "error getting past meeting participant from NATS KV", logging.ErrKey, err)
		return nil, 0, domain.ErrInternal
	}

	participant, err := s.unmarshal(ctx, entry)
	if err != nil {
		return nil, 0, domain.ErrUnmarshal
	}

	return participant, entry.Revision(), nil
}

func (s *NatsPastMeetingParticipantRepository) Exists(ctx context.Context, participantUID string) (bool, error) {
	_, err := s.get(ctx, participantUID)
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			return false, nil
		}
		return false, domain.ErrInternal
	}
	return true, nil
}

func (s *NatsPastMeetingParticipantRepository) Create(ctx context.Context, participant *models.PastMeetingParticipant) error {
	if s.PastMeetingParticipants == nil {
		return domain.ErrServiceUnavailable
	}

	// Generate a new UID if not provided
	if participant.UID == "" {
		participant.UID = uuid.New().String()
	}

	// Set timestamps
	now := time.Now()
	participant.CreatedAt = &now
	participant.UpdatedAt = &now

	// Marshal the participant
	participantBytes, err := json.Marshal(participant)
	if err != nil {
		slog.ErrorContext(ctx, "error marshaling past meeting participant", logging.ErrKey, err)
		return domain.ErrMarshal
	}

	// Store in NATS KV
	_, err = s.PastMeetingParticipants.Put(ctx, participant.UID, participantBytes)
	if err != nil {
		slog.ErrorContext(ctx, "error storing past meeting participant in NATS KV", logging.ErrKey, err)
		return domain.ErrInternal
	}

	return nil
}

func (s *NatsPastMeetingParticipantRepository) Update(ctx context.Context, participant *models.PastMeetingParticipant, revision uint64) error {
	if s.PastMeetingParticipants == nil {
		return domain.ErrServiceUnavailable
	}

	// Update timestamp
	now := time.Now()
	participant.UpdatedAt = &now

	// Marshal the participant
	participantBytes, err := json.Marshal(participant)
	if err != nil {
		slog.ErrorContext(ctx, "error marshaling past meeting participant", logging.ErrKey, err)
		return domain.ErrMarshal
	}

	// Update in NATS KV with revision check
	_, err = s.PastMeetingParticipants.Update(ctx, participant.UID, participantBytes, revision)
	if err != nil {
		if strings.Contains(err.Error(), "wrong last sequence") {
			slog.WarnContext(ctx, "revision mismatch", logging.ErrKey, err)
			return domain.ErrRevisionMismatch
		}
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			return domain.ErrPastMeetingParticipantNotFound
		}
		slog.ErrorContext(ctx, "error updating past meeting participant in NATS KV", logging.ErrKey, err)
		return domain.ErrInternal
	}

	return nil
}

func (s *NatsPastMeetingParticipantRepository) Delete(ctx context.Context, participantUID string, revision uint64) error {
	if s.PastMeetingParticipants == nil {
		return domain.ErrServiceUnavailable
	}

	err := s.PastMeetingParticipants.Delete(ctx, participantUID, jetstream.LastRevision(revision))
	if err != nil {
		if strings.Contains(err.Error(), "wrong last sequence") {
			slog.WarnContext(ctx, "revision mismatch", logging.ErrKey, err)
			return domain.ErrRevisionMismatch
		}
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			return domain.ErrPastMeetingParticipantNotFound
		}
		slog.ErrorContext(ctx, "error deleting past meeting participant from NATS KV", logging.ErrKey, err)
		return domain.ErrInternal
	}

	return nil
}

func (s *NatsPastMeetingParticipantRepository) ListByPastMeeting(ctx context.Context, pastMeetingUID string) ([]*models.PastMeetingParticipant, error) {
	if s.PastMeetingParticipants == nil {
		return nil, domain.ErrServiceUnavailable
	}

	keysLister, err := s.PastMeetingParticipants.ListKeys(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "error listing past meeting participant keys from NATS KV store", logging.ErrKey, err)
		return nil, domain.ErrInternal
	}

	var participants []*models.PastMeetingParticipant
	for key := range keysLister.Keys() {
		entry, err := s.get(ctx, key)
		if err != nil {
			if !errors.Is(err, jetstream.ErrKeyNotFound) {
				slog.ErrorContext(ctx, "error getting past meeting participant from NATS KV store", logging.ErrKey, err, "key", key)
			}
			continue
		}

		participant, err := s.unmarshal(ctx, entry)
		if err != nil {
			slog.ErrorContext(ctx, "error unmarshaling past meeting participant", logging.ErrKey, err, "key", key)
			continue
		}

		if participant.PastMeetingUID == pastMeetingUID {
			participants = append(participants, participant)
		}
	}

	return participants, nil
}

func (s *NatsPastMeetingParticipantRepository) ListByEmail(ctx context.Context, email string) ([]*models.PastMeetingParticipant, error) {
	if s.PastMeetingParticipants == nil {
		return nil, domain.ErrServiceUnavailable
	}

	keysLister, err := s.PastMeetingParticipants.ListKeys(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "error listing past meeting participant keys from NATS KV store", logging.ErrKey, err)
		return nil, domain.ErrInternal
	}

	email = strings.ToLower(email)

	var participants []*models.PastMeetingParticipant
	for key := range keysLister.Keys() {
		entry, err := s.get(ctx, key)
		if err != nil {
			if !errors.Is(err, jetstream.ErrKeyNotFound) {
				slog.ErrorContext(ctx, "error getting past meeting participant from NATS KV store", logging.ErrKey, err, "key", key)
			}
			continue
		}

		participant, err := s.unmarshal(ctx, entry)
		if err != nil {
			slog.ErrorContext(ctx, "error unmarshaling past meeting participant", logging.ErrKey, err, "key", key)
			continue
		}

		if strings.ToLower(participant.Email) == email {
			participants = append(participants, participant)
		}
	}

	return participants, nil
}

func (s *NatsPastMeetingParticipantRepository) GetByPastMeetingAndEmail(ctx context.Context, pastMeetingUID, email string) (*models.PastMeetingParticipant, error) {
	if s.PastMeetingParticipants == nil {
		return nil, domain.ErrServiceUnavailable
	}

	keysLister, err := s.PastMeetingParticipants.ListKeys(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "error listing past meeting participant keys from NATS KV store", logging.ErrKey, err)
		return nil, domain.ErrInternal
	}

	email = strings.ToLower(email)

	for key := range keysLister.Keys() {
		entry, err := s.get(ctx, key)
		if err != nil {
			if !errors.Is(err, jetstream.ErrKeyNotFound) {
				slog.ErrorContext(ctx, "error getting past meeting participant from NATS KV store", logging.ErrKey, err, "key", key)
			}
			continue
		}

		participant, err := s.unmarshal(ctx, entry)
		if err != nil {
			slog.ErrorContext(ctx, "error unmarshaling past meeting participant", logging.ErrKey, err, "key", key)
			continue
		}

		if participant.PastMeetingUID == pastMeetingUID && strings.ToLower(participant.Email) == email {
			return participant, nil
		}
	}

	return nil, domain.ErrPastMeetingParticipantNotFound
}

// Ensure NatsPastMeetingParticipantRepository implements domain.PastMeetingParticipantRepository
var _ domain.PastMeetingParticipantRepository = (*NatsPastMeetingParticipantRepository)(nil)
