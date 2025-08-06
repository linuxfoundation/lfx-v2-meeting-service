// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package store

import (
	"context"
	"encoding/json"
	"errors"
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

// NatsRegistrantRepository is the NATS KV store repository for registrants.
type NatsRegistrantRepository struct {
	MeetingRegistrants INatsKeyValue
}

// NewNatsRegistrantRepository creates a new NATS KV store repository for registrants.
func NewNatsRegistrantRepository(meetingRegistrants INatsKeyValue) *NatsRegistrantRepository {
	return &NatsRegistrantRepository{
		MeetingRegistrants: meetingRegistrants,
	}
}

func (s *NatsRegistrantRepository) CreateRegistrant(ctx context.Context, registrant *models.Registrant) error {
	if s.MeetingRegistrants == nil {
		return domain.ErrServiceUnavailable
	}

	// Check if registrant already exists
	exists, err := s.RegistrantExists(ctx, registrant.UID)
	if err != nil {
		return err
	}
	if exists {
		return domain.ErrRegistrantAlreadyExists
	}

	jsonData, err := json.Marshal(registrant)
	if err != nil {
		slog.ErrorContext(ctx, "error marshaling registrant", logging.ErrKey, err)
		return domain.ErrInternal
	}

	_, err = s.MeetingRegistrants.Put(ctx, registrant.UID, jsonData)
	if err != nil {
		slog.ErrorContext(ctx, "error putting registrant into NATS KV store", logging.ErrKey, err)
		return domain.ErrInternal
	}

	return nil
}

func (s *NatsRegistrantRepository) RegistrantExists(ctx context.Context, registrantUID string) (bool, error) {
	if s.MeetingRegistrants == nil {
		return false, domain.ErrServiceUnavailable
	}

	_, err := s.MeetingRegistrants.Get(ctx, registrantUID)
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (s *NatsRegistrantRepository) GetRegistrant(ctx context.Context, registrantUID string) (*models.Registrant, error) {
	if s.MeetingRegistrants == nil {
		return nil, domain.ErrServiceUnavailable
	}

	entry, err := s.MeetingRegistrants.Get(ctx, registrantUID)
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			slog.WarnContext(ctx, "registrant not found", logging.ErrKey, domain.ErrRegistrantNotFound)
			return nil, domain.ErrRegistrantNotFound
		}
		slog.ErrorContext(ctx, "error getting registrant from NATS KV", logging.ErrKey, err)
		return nil, err
	}

	var registrant models.Registrant
	err = json.Unmarshal(entry.Value(), &registrant)
	if err != nil {
		slog.ErrorContext(ctx, "error unmarshaling registrant", logging.ErrKey, err)
		return nil, domain.ErrUnmarshal
	}

	return &registrant, nil
}

func (s *NatsRegistrantRepository) GetRegistrantWithRevision(ctx context.Context, registrantUID string) (*models.Registrant, uint64, error) {
	if s.MeetingRegistrants == nil {
		return nil, 0, domain.ErrServiceUnavailable
	}

	entry, err := s.MeetingRegistrants.Get(ctx, registrantUID)
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

func (s *NatsRegistrantRepository) UpdateRegistrant(ctx context.Context, registrant *models.Registrant, revision uint64) error {
	if s.MeetingRegistrants == nil {
		return domain.ErrServiceUnavailable
	}

	jsonData, err := json.Marshal(registrant)
	if err != nil {
		slog.ErrorContext(ctx, "error marshaling registrant", logging.ErrKey, err)
		return domain.ErrInternal
	}

	_, err = s.MeetingRegistrants.Update(ctx, registrant.UID, jsonData, revision)
	if err != nil {
		if strings.Contains(err.Error(), "wrong last sequence") {
			slog.WarnContext(ctx, "revision mismatch", logging.ErrKey, err)
			return domain.ErrRevisionMismatch
		}
		slog.ErrorContext(ctx, "error updating registrant in NATS KV store", logging.ErrKey, err)
		return domain.ErrInternal
	}

	return nil
}

func (s *NatsRegistrantRepository) DeleteRegistrant(ctx context.Context, registrantUID string, revision uint64) error {
	if s.MeetingRegistrants == nil {
		return domain.ErrServiceUnavailable
	}

	err := s.MeetingRegistrants.Delete(ctx, registrantUID, jetstream.LastRevision(revision))
	if err != nil {
		if strings.Contains(err.Error(), "wrong last sequence") {
			slog.WarnContext(ctx, "revision mismatch", logging.ErrKey, err)
			return domain.ErrRevisionMismatch
		}
		slog.ErrorContext(ctx, "error deleting registrant from NATS KV store", logging.ErrKey, err)
		return domain.ErrInternal
	}

	return nil
}

func (s *NatsRegistrantRepository) ListMeetingRegistrants(ctx context.Context, meetingUID string) ([]*models.Registrant, error) {
	if s.MeetingRegistrants == nil {
		return nil, domain.ErrServiceUnavailable
	}

	lister, err := s.MeetingRegistrants.ListKeys(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "error listing registrant keys from NATS KV store", logging.ErrKey, err)
		return nil, domain.ErrInternal
	}

	var registrants []*models.Registrant
	for key := range lister.Keys() {
		registrant, err := s.GetRegistrant(ctx, key)
		if err != nil {
			slog.ErrorContext(ctx, "error getting registrant", "key", key, logging.ErrKey, err)
			continue
		}

		registrants = append(registrants, registrant)
	}

	return registrants, nil
}
