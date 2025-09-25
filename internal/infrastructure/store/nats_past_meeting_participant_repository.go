// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package store

import (
	"context"
	"log/slog"

	"github.com/google/uuid"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
)

// NatsPastMeetingParticipantRepository is the NATS KV store repository for past meeting participants.
type NatsPastMeetingParticipantRepository struct {
	*NatsBaseRepository[models.PastMeetingParticipant]
}

// NewNatsPastMeetingParticipantRepository creates a new NATS KV store repository for past meeting participants.
func NewNatsPastMeetingParticipantRepository(kvStore INatsKeyValue) *NatsPastMeetingParticipantRepository {
	baseRepo := NewNatsBaseRepository[models.PastMeetingParticipant](kvStore, "past meeting participant")

	return &NatsPastMeetingParticipantRepository{
		NatsBaseRepository: baseRepo,
	}
}

// Get retrieves a past meeting participant by UID
func (r *NatsPastMeetingParticipantRepository) Get(ctx context.Context, participantUID string) (*models.PastMeetingParticipant, error) {
	return r.NatsBaseRepository.Get(ctx, participantUID)
}

// GetWithRevision retrieves a past meeting participant with revision by UID
func (r *NatsPastMeetingParticipantRepository) GetWithRevision(ctx context.Context, participantUID string) (*models.PastMeetingParticipant, uint64, error) {
	return r.NatsBaseRepository.GetWithRevision(ctx, participantUID)
}

// Exists checks if a past meeting participant exists
func (r *NatsPastMeetingParticipantRepository) Exists(ctx context.Context, participantUID string) (bool, error) {
	return r.NatsBaseRepository.Exists(ctx, participantUID)
}

// Create creates a new past meeting participant
func (r *NatsPastMeetingParticipantRepository) Create(ctx context.Context, participant *models.PastMeetingParticipant) error {
	// Generate a new UID if not provided
	if participant.UID == "" {
		participant.UID = uuid.New().String()
	}

	return r.NatsBaseRepository.Create(ctx, participant.UID, participant)
}

// Update updates an existing past meeting participant
func (r *NatsPastMeetingParticipantRepository) Update(ctx context.Context, participant *models.PastMeetingParticipant, revision uint64) error {
	return r.NatsBaseRepository.Update(ctx, participant.UID, participant, revision)
}

// Delete removes a past meeting participant
func (r *NatsPastMeetingParticipantRepository) Delete(ctx context.Context, participantUID string, revision uint64) error {
	return r.NatsBaseRepository.Delete(ctx, participantUID, revision)
}

// ListByPastMeeting retrieves all past meeting participants for a given past meeting UID
func (r *NatsPastMeetingParticipantRepository) ListByPastMeeting(ctx context.Context, pastMeetingUID string) ([]*models.PastMeetingParticipant, error) {
	allParticipants, err := r.ListAll(ctx)
	if err != nil {
		return nil, err
	}

	var matchingParticipants []*models.PastMeetingParticipant
	for _, participant := range allParticipants {
		if participant.PastMeetingUID == pastMeetingUID {
			matchingParticipants = append(matchingParticipants, participant)
		}
	}

	return matchingParticipants, nil
}

// ListByEmail retrieves all past meeting participants with a specific email
func (r *NatsPastMeetingParticipantRepository) ListByEmail(ctx context.Context, email string) ([]*models.PastMeetingParticipant, error) {
	allParticipants, err := r.ListAll(ctx)
	if err != nil {
		return nil, err
	}

	var matchingParticipants []*models.PastMeetingParticipant
	for _, participant := range allParticipants {
		if participant.Email == email {
			matchingParticipants = append(matchingParticipants, participant)
		}
	}

	return matchingParticipants, nil
}

// GetByPastMeetingAndEmail retrieves a past meeting participant by past meeting UID and email
func (r *NatsPastMeetingParticipantRepository) GetByPastMeetingAndEmail(ctx context.Context, pastMeetingUID, email string) (*models.PastMeetingParticipant, error) {
	participants, err := r.ListByPastMeeting(ctx, pastMeetingUID)
	if err != nil {
		return nil, err
	}

	for _, participant := range participants {
		if participant.Email == email {
			return participant, nil
		}
	}

	slog.DebugContext(ctx, "participant not found", "past_meeting_uid", pastMeetingUID, "email", email)
	return nil, domain.NewNotFoundError("participant not found")
}

// ListAll lists all past meeting participants
func (r *NatsPastMeetingParticipantRepository) ListAll(ctx context.Context) ([]*models.PastMeetingParticipant, error) {
	return r.ListEntities(ctx, "")
}