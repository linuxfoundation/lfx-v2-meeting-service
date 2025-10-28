// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package store

import (
	"context"
	"fmt"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
)

// NatsMeetingRSVPRepository is the NATS KV store repository for meeting RSVP responses.
type NatsMeetingRSVPRepository struct {
	*NatsBaseRepository[models.RSVPResponse]
}

// NewNatsMeetingRSVPRepository creates a new NATS KV store repository for meeting RSVP responses.
func NewNatsMeetingRSVPRepository(kvStore INatsKeyValue) *NatsMeetingRSVPRepository {
	baseRepo := NewNatsBaseRepository[models.RSVPResponse](kvStore, "meeting rsvp")

	return &NatsMeetingRSVPRepository{
		NatsBaseRepository: baseRepo,
	}
}

// Create creates a new RSVP response
func (r *NatsMeetingRSVPRepository) Create(ctx context.Context, rsvp *models.RSVPResponse) error {
	if rsvp.ID == "" {
		return domain.NewValidationError("rsvp ID is required")
	}

	return r.NatsBaseRepository.Create(ctx, rsvp.ID, rsvp)
}

// Exists checks if an RSVP response exists
func (r *NatsMeetingRSVPRepository) Exists(ctx context.Context, rsvpID string) (bool, error) {
	return r.NatsBaseRepository.Exists(ctx, rsvpID)
}

// Delete removes an RSVP response
func (r *NatsMeetingRSVPRepository) Delete(ctx context.Context, rsvpID string, revision uint64) error {
	return r.NatsBaseRepository.Delete(ctx, rsvpID, revision)
}

// Get retrieves an RSVP response by ID
func (r *NatsMeetingRSVPRepository) Get(ctx context.Context, rsvpID string) (*models.RSVPResponse, error) {
	return r.NatsBaseRepository.Get(ctx, rsvpID)
}

// GetWithRevision retrieves an RSVP response with revision by ID
func (r *NatsMeetingRSVPRepository) GetWithRevision(ctx context.Context, rsvpID string) (*models.RSVPResponse, uint64, error) {
	return r.NatsBaseRepository.GetWithRevision(ctx, rsvpID)
}

// Update updates an existing RSVP response
func (r *NatsMeetingRSVPRepository) Update(ctx context.Context, rsvp *models.RSVPResponse, revision uint64) error {
	return r.NatsBaseRepository.Update(ctx, rsvp.ID, rsvp, revision)
}

// ListByMeeting retrieves all RSVP responses for a meeting
func (r *NatsMeetingRSVPRepository) ListByMeeting(ctx context.Context, meetingUID string) ([]*models.RSVPResponse, error) {
	allRSVPs, err := r.ListAll(ctx)
	if err != nil {
		return nil, err
	}

	var matchingRSVPs []*models.RSVPResponse
	for _, rsvp := range allRSVPs {
		if rsvp.MeetingUID == meetingUID {
			matchingRSVPs = append(matchingRSVPs, rsvp)
		}
	}

	return matchingRSVPs, nil
}

// GetByMeetingAndRegistrant retrieves an RSVP response by meeting and registrant
// Since "most recent wins", only one RSVP per registrant per meeting is stored
func (r *NatsMeetingRSVPRepository) GetByMeetingAndRegistrant(ctx context.Context, meetingUID, registrantID string) (*models.RSVPResponse, uint64, error) {
	rsvps, err := r.ListByMeeting(ctx, meetingUID)
	if err != nil {
		return nil, 0, err
	}

	for _, rsvp := range rsvps {
		if rsvp.RegistrantID == registrantID {
			// Get with revision
			return r.GetWithRevision(ctx, rsvp.ID)
		}
	}

	return nil, 0, domain.NewNotFoundError(fmt.Sprintf("rsvp with meeting '%s' and registrant '%s' not found", meetingUID, registrantID))
}

// ListAll lists all RSVP responses
func (r *NatsMeetingRSVPRepository) ListAll(ctx context.Context) ([]*models.RSVPResponse, error) {
	return r.ListEntities(ctx, "")
}
