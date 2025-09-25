// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
)

// NatsPastMeetingRepository is the NATS KV store repository for past meetings
type NatsPastMeetingRepository struct {
	*NatsBaseRepository[models.PastMeeting]
}

// NewNatsPastMeetingRepository creates a new NATS KV store repository for past meetings
func NewNatsPastMeetingRepository(pastMeetings INatsKeyValue) *NatsPastMeetingRepository {
	baseRepo := NewNatsBaseRepository[models.PastMeeting](pastMeetings, "past meeting")

	return &NatsPastMeetingRepository{
		NatsBaseRepository: baseRepo,
	}
}

// Get retrieves a past meeting by UID
func (s *NatsPastMeetingRepository) Get(ctx context.Context, pastMeetingUID string) (*models.PastMeeting, error) {
	return s.NatsBaseRepository.Get(ctx, pastMeetingUID)
}

// GetWithRevision retrieves a past meeting with revision by UID
func (s *NatsPastMeetingRepository) GetWithRevision(ctx context.Context, pastMeetingUID string) (*models.PastMeeting, uint64, error) {
	return s.NatsBaseRepository.GetWithRevision(ctx, pastMeetingUID)
}

// Exists checks if a past meeting exists
func (s *NatsPastMeetingRepository) Exists(ctx context.Context, pastMeetingUID string) (bool, error) {
	return s.NatsBaseRepository.Exists(ctx, pastMeetingUID)
}

// Create creates a new past meeting
func (s *NatsPastMeetingRepository) Create(ctx context.Context, pastMeeting *models.PastMeeting) error {
	// Generate UID if not provided
	if pastMeeting.UID == "" {
		pastMeeting.UID = uuid.New().String()
	}

	// Set timestamps
	now := time.Now()
	pastMeeting.CreatedAt = &now
	pastMeeting.UpdatedAt = &now

	// Calculate scheduled end time if not set
	if pastMeeting.ScheduledEndTime.IsZero() && pastMeeting.Duration > 0 {
		pastMeeting.ScheduledEndTime = pastMeeting.ScheduledStartTime.Add(time.Duration(pastMeeting.Duration) * time.Minute)
	}

	return s.NatsBaseRepository.Create(ctx, pastMeeting.UID, pastMeeting)
}

// Update updates an existing past meeting
func (s *NatsPastMeetingRepository) Update(ctx context.Context, pastMeeting *models.PastMeeting, revision uint64) error {
	return s.NatsBaseRepository.Update(ctx, pastMeeting.UID, pastMeeting, revision)
}

// Delete removes a past meeting
func (s *NatsPastMeetingRepository) Delete(ctx context.Context, pastMeetingUID string, revision uint64) error {
	return s.NatsBaseRepository.Delete(ctx, pastMeetingUID, revision)
}

// ListAll lists all past meetings
func (s *NatsPastMeetingRepository) ListAll(ctx context.Context) ([]*models.PastMeeting, error) {
	return s.ListEntities(ctx, "")
}

// GetByPlatformMeetingID gets a past meeting by platform meeting ID
func (s *NatsPastMeetingRepository) GetByPlatformMeetingID(ctx context.Context, platform, platformMeetingID string) (*models.PastMeeting, error) {
	allPastMeetings, err := s.ListAll(ctx)
	if err != nil {
		return nil, err
	}

	for _, pastMeeting := range allPastMeetings {
		if pastMeeting.Platform == platform && pastMeeting.PlatformMeetingID == platformMeetingID {
			return pastMeeting, nil
		}
	}

	return nil, domain.NewNotFoundError(
		fmt.Sprintf("past meeting with platform '%s' and meeting ID '%s' not found", platform, platformMeetingID))
}

// GetByPlatformMeetingIDAndOccurrence gets a past meeting by platform meeting ID and occurrence
func (s *NatsPastMeetingRepository) GetByPlatformMeetingIDAndOccurrence(ctx context.Context, platform, platformMeetingID, occurrenceID string) (*models.PastMeeting, error) {
	allPastMeetings, err := s.ListAll(ctx)
	if err != nil {
		return nil, err
	}

	for _, pastMeeting := range allPastMeetings {
		if pastMeeting.Platform == platform &&
			pastMeeting.PlatformMeetingID == platformMeetingID &&
			pastMeeting.OccurrenceID == occurrenceID {
			return pastMeeting, nil
		}
	}

	return nil, domain.NewNotFoundError(
		fmt.Sprintf("past meeting with platform '%s', meeting ID '%s' and occurrence ID '%s' not found",
			platform, platformMeetingID, occurrenceID))
}
