// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"errors"
	"log/slog"
	"strconv"

	"github.com/google/uuid"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/concurrent"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/constants"
)

// PastMeetingParticipantService implements the meetingsvc.Service interface and domain.MessageHandler
type PastMeetingParticipantService struct {
	MeetingRepository                domain.MeetingRepository
	PastMeetingRepository            domain.PastMeetingRepository
	PastMeetingParticipantRepository domain.PastMeetingParticipantRepository
	MessageBuilder                   domain.MessageBuilder
	Config                           ServiceConfig
}

// NewPastMeetingParticipantService creates a new PastMeetingParticipantService.
func NewPastMeetingParticipantService(
	meetingRepository domain.MeetingRepository,
	pastMeetingRepository domain.PastMeetingRepository,
	pastMeetingParticipantRepository domain.PastMeetingParticipantRepository,
	messageBuilder domain.MessageBuilder,
	config ServiceConfig,
) *PastMeetingParticipantService {
	return &PastMeetingParticipantService{
		Config:                           config,
		MeetingRepository:                meetingRepository,
		PastMeetingRepository:            pastMeetingRepository,
		PastMeetingParticipantRepository: pastMeetingParticipantRepository,
		MessageBuilder:                   messageBuilder,
	}
}

// ServiceReady checks if the service is ready for use.
func (s *PastMeetingParticipantService) ServiceReady() bool {
	return s.PastMeetingRepository != nil &&
		s.PastMeetingParticipantRepository != nil &&
		s.MessageBuilder != nil
}

// GetPastMeetingParticipants fetches all participants for a past meeting
func (s *PastMeetingParticipantService) GetPastMeetingParticipants(ctx context.Context, pastMeetingUID string) ([]*models.PastMeetingParticipant, error) {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "service not initialized", logging.PriorityCritical())
		return nil, domain.ErrServiceUnavailable
	}

	if pastMeetingUID == "" {
		slog.WarnContext(ctx, "past meeting UID is required")
		return nil, domain.ErrValidationFailed
	}

	ctx = logging.AppendCtx(ctx, slog.String("past_meeting_uid", pastMeetingUID))

	// Check if the past meeting exists
	exists, err := s.PastMeetingRepository.Exists(ctx, pastMeetingUID)
	if err != nil {
		slog.ErrorContext(ctx, "error checking if past meeting exists", logging.ErrKey, err)
		return nil, domain.ErrInternal
	}
	if !exists {
		slog.WarnContext(ctx, "past meeting not found")
		return nil, domain.ErrPastMeetingNotFound
	}

	// Get all participants for the past meeting
	participants, err := s.PastMeetingParticipantRepository.ListByPastMeeting(ctx, pastMeetingUID)
	if err != nil {
		slog.ErrorContext(ctx, "error getting past meeting participants", logging.ErrKey, err)
		return nil, domain.ErrInternal
	}

	slog.DebugContext(ctx, "returning past meeting participants", "count", len(participants))

	return participants, nil
}

func (s *PastMeetingParticipantService) validateCreateParticipantRequest(ctx context.Context, participant *models.PastMeetingParticipant) error {
	if participant == nil || participant.PastMeetingUID == "" {
		slog.WarnContext(ctx, "participant and past meeting UID are required")
		return domain.ErrValidationFailed
	}

	// Check if the past meeting exists
	exists, err := s.PastMeetingRepository.Exists(ctx, participant.PastMeetingUID)
	if err != nil {
		slog.ErrorContext(ctx, "error checking if past meeting exists", logging.ErrKey, err)
		return domain.ErrInternal
	}
	if !exists {
		slog.WarnContext(ctx, "past meeting not found", "past_meeting_uid", participant.PastMeetingUID)
		return domain.ErrPastMeetingNotFound
	}

	// Check that there isn't already a participant with the same email address for this past meeting.
	existingParticipant, err := s.PastMeetingParticipantRepository.GetByPastMeetingAndEmail(ctx, participant.PastMeetingUID, participant.Email)
	if err != nil && !errors.Is(err, domain.ErrPastMeetingParticipantNotFound) {
		slog.ErrorContext(ctx, "error checking for existing participant", logging.ErrKey, err)
		return domain.ErrInternal
	}
	if existingParticipant != nil {
		slog.WarnContext(ctx, "participant already exists for past meeting with same email address",
			"email", participant.Email,
			logging.ErrKey, domain.ErrPastMeetingParticipantAlreadyExists)
		return domain.ErrPastMeetingParticipantAlreadyExists
	}

	return nil
}

// CreatePastMeetingParticipant creates a new participant for a past meeting
func (s *PastMeetingParticipantService) CreatePastMeetingParticipant(ctx context.Context, participant *models.PastMeetingParticipant) (*models.PastMeetingParticipant, error) {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "service not initialized", logging.PriorityCritical())
		return nil, domain.ErrServiceUnavailable
	}

	if participant == nil {
		slog.WarnContext(ctx, "participant is required")
		return nil, domain.ErrValidationFailed
	}

	ctx = logging.AppendCtx(ctx, slog.String("past_meeting_uid", participant.PastMeetingUID))

	// Validate the request
	err := s.validateCreateParticipantRequest(ctx, participant)
	if err != nil {
		return nil, err
	}

	// Get the past meeting to populate the MeetingUID
	pastMeeting, err := s.PastMeetingRepository.Get(ctx, participant.PastMeetingUID)
	if err != nil {
		if errors.Is(err, domain.ErrPastMeetingNotFound) {
			slog.WarnContext(ctx, "past meeting not found", logging.ErrKey, err)
			return nil, domain.ErrPastMeetingNotFound
		}
		slog.ErrorContext(ctx, "error getting past meeting", logging.ErrKey, err)
		return nil, domain.ErrInternal
	}
	participant.MeetingUID = pastMeeting.MeetingUID

	// Generate UID for the participant
	participant.UID = uuid.New().String()

	ctx = logging.AppendCtx(ctx, slog.String("participant_uid", participant.UID))

	// Create the participant
	err = s.PastMeetingParticipantRepository.Create(ctx, participant)
	if err != nil {
		if errors.Is(err, domain.ErrPastMeetingParticipantAlreadyExists) {
			slog.WarnContext(ctx, "participant already exists", logging.ErrKey, err)
			return nil, domain.ErrPastMeetingParticipantAlreadyExists
		}
		slog.ErrorContext(ctx, "error creating participant", logging.ErrKey, err)
		return nil, domain.ErrInternal
	}

	slog.DebugContext(ctx, "created past meeting participant",
		"participant_uid", participant.UID,
		"email", participant.Email)

	// Use WorkerPool for concurrent NATS message sending
	pool := concurrent.NewWorkerPool(2) // 2 messages to send
	messages := []func() error{
		func() error {
			return s.MessageBuilder.SendIndexPastMeetingParticipant(ctx, models.ActionCreated, *participant)
		},
		func() error {
			return s.MessageBuilder.SendPutPastMeetingParticipantAccess(ctx, models.PastMeetingParticipantAccessMessage{
				UID:            participant.UID,
				PastMeetingUID: participant.PastMeetingUID,
				Username:       participant.Username,
				Host:           participant.Host,
			})
		},
	}

	if err := pool.Run(ctx, messages...); err != nil {
		slog.ErrorContext(ctx, "failed to send NATS messages", logging.ErrKey, err)
		// Don't fail the operation if messaging fails
	}

	return participant, nil
}

// GetPastMeetingParticipant fetches a specific participant by UID
func (s *PastMeetingParticipantService) GetPastMeetingParticipant(ctx context.Context, pastMeetingUID, participantUID string) (*models.PastMeetingParticipant, string, error) {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "service not initialized", logging.PriorityCritical())
		return nil, "", domain.ErrServiceUnavailable
	}

	if pastMeetingUID == "" || participantUID == "" {
		slog.WarnContext(ctx, "past meeting UID and participant UID are required")
		return nil, "", domain.ErrValidationFailed
	}

	ctx = logging.AppendCtx(ctx, slog.String("past_meeting_uid", pastMeetingUID))
	ctx = logging.AppendCtx(ctx, slog.String("participant_uid", participantUID))

	// Check if the past meeting exists
	exists, err := s.PastMeetingRepository.Exists(ctx, pastMeetingUID)
	if err != nil {
		slog.ErrorContext(ctx, "error checking if past meeting exists", logging.ErrKey, err)
		return nil, "", domain.ErrInternal
	}
	if !exists {
		slog.WarnContext(ctx, "past meeting not found")
		return nil, "", domain.ErrPastMeetingNotFound
	}

	// Get the participant with revision
	participant, revision, err := s.PastMeetingParticipantRepository.GetWithRevision(ctx, participantUID)
	if err != nil {
		if errors.Is(err, domain.ErrPastMeetingParticipantNotFound) {
			slog.WarnContext(ctx, "participant not found", logging.ErrKey, err)
			return nil, "", domain.ErrPastMeetingParticipantNotFound
		}
		slog.ErrorContext(ctx, "error getting participant from store", logging.ErrKey, err)
		return nil, "", domain.ErrInternal
	}

	// Verify the participant belongs to the specified past meeting
	if participant.PastMeetingUID != pastMeetingUID {
		slog.WarnContext(ctx, "participant does not belong to the specified past meeting",
			"expected_past_meeting_uid", pastMeetingUID,
			"actual_past_meeting_uid", participant.PastMeetingUID)
		return nil, "", domain.ErrPastMeetingParticipantNotFound
	}

	// Convert revision to string for ETag
	revisionStr := strconv.FormatUint(revision, 10)
	ctx = context.WithValue(ctx, constants.ETagContextID, revisionStr)

	slog.DebugContext(ctx, "returning participant", "participant_uid", participant.UID, "revision", revision)

	return participant, revisionStr, nil
}

func (s *PastMeetingParticipantService) validateUpdateParticipantRequest(ctx context.Context, participant *models.PastMeetingParticipant) error {
	// Check if the past meeting exists
	exists, err := s.PastMeetingRepository.Exists(ctx, participant.PastMeetingUID)
	if err != nil {
		slog.ErrorContext(ctx, "error checking if past meeting exists", logging.ErrKey, err)
		return domain.ErrInternal
	}
	if !exists {
		slog.WarnContext(ctx, "past meeting not found")
		return domain.ErrPastMeetingNotFound
	}

	// Check that there isn't already another participant with the same email address for this past meeting
	// (unless it's the same participant being updated)
	existingParticipant, err := s.PastMeetingParticipantRepository.GetByPastMeetingAndEmail(ctx, participant.PastMeetingUID, participant.Email)
	if err != nil && !errors.Is(err, domain.ErrPastMeetingParticipantNotFound) {
		slog.ErrorContext(ctx, "error checking for existing participant", logging.ErrKey, err)
		return domain.ErrInternal
	}
	if existingParticipant != nil && existingParticipant.UID != participant.UID {
		slog.WarnContext(ctx, "another participant already exists for past meeting with same email address",
			"email", participant.Email,
			logging.ErrKey, domain.ErrPastMeetingParticipantAlreadyExists)
		return domain.ErrPastMeetingParticipantAlreadyExists
	}

	return nil
}

// UpdatePastMeetingParticipant updates an existing participant
func (s *PastMeetingParticipantService) UpdatePastMeetingParticipant(ctx context.Context, participant *models.PastMeetingParticipant, revision uint64) (*models.PastMeetingParticipant, error) {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "service not initialized", logging.PriorityCritical())
		return nil, domain.ErrServiceUnavailable
	}

	if participant == nil || participant.UID == "" {
		slog.WarnContext(ctx, "participant and participant UID are required")
		return nil, domain.ErrValidationFailed
	}

	var err error
	if s.Config.SkipEtagValidation {
		// If skipping the Etag validation, we need to get the key revision from the store with a Get request.
		_, revision, err = s.PastMeetingParticipantRepository.GetWithRevision(ctx, participant.UID)
		if err != nil {
			if errors.Is(err, domain.ErrPastMeetingParticipantNotFound) {
				slog.WarnContext(ctx, "participant not found", logging.ErrKey, err)
				return nil, domain.ErrPastMeetingParticipantNotFound
			}
			slog.ErrorContext(ctx, "error getting participant from store", logging.ErrKey, err)
			return nil, domain.ErrInternal
		}
	}

	ctx = logging.AppendCtx(ctx, slog.String("past_meeting_uid", participant.PastMeetingUID))
	ctx = logging.AppendCtx(ctx, slog.String("participant_uid", participant.UID))
	ctx = logging.AppendCtx(ctx, slog.String("etag", strconv.FormatUint(revision, 10)))

	// Check if the participant exists and get existing data
	existingParticipant, err := s.PastMeetingParticipantRepository.Get(ctx, participant.UID)
	if err != nil {
		if errors.Is(err, domain.ErrPastMeetingParticipantNotFound) {
			slog.WarnContext(ctx, "participant not found", logging.ErrKey, err)
			return nil, domain.ErrPastMeetingParticipantNotFound
		}
		slog.ErrorContext(ctx, "error checking if participant exists", logging.ErrKey, err)
		return nil, domain.ErrInternal
	}

	// Preserve fields that shouldn't be changed
	participant.PastMeetingUID = existingParticipant.PastMeetingUID
	participant.MeetingUID = existingParticipant.MeetingUID
	participant.CreatedAt = existingParticipant.CreatedAt

	// Validate the update request
	err = s.validateUpdateParticipantRequest(ctx, participant)
	if err != nil {
		return nil, err
	}

	// Update the participant
	err = s.PastMeetingParticipantRepository.Update(ctx, participant, revision)
	if err != nil {
		if errors.Is(err, domain.ErrRevisionMismatch) {
			slog.WarnContext(ctx, "If-Match header is invalid", logging.ErrKey, err)
			return nil, domain.ErrRevisionMismatch
		}
		if errors.Is(err, domain.ErrPastMeetingParticipantNotFound) {
			slog.WarnContext(ctx, "participant not found", logging.ErrKey, err)
			return nil, domain.ErrPastMeetingParticipantNotFound
		}
		slog.ErrorContext(ctx, "error updating participant", logging.ErrKey, err)
		return nil, domain.ErrInternal
	}

	slog.DebugContext(ctx, "updated past meeting participant", "participant_uid", participant.UID)

	// Use WorkerPool for concurrent NATS message sending
	pool := concurrent.NewWorkerPool(2) // 2 messages to send
	messages := []func() error{
		func() error {
			return s.MessageBuilder.SendIndexPastMeetingParticipant(ctx, models.ActionUpdated, *participant)
		},
		func() error {
			return s.MessageBuilder.SendPutPastMeetingParticipantAccess(ctx, models.PastMeetingParticipantAccessMessage{
				UID:            participant.UID,
				PastMeetingUID: participant.PastMeetingUID,
				Username:       participant.Username,
				Host:           participant.Host,
			})
		},
	}

	if err := pool.Run(ctx, messages...); err != nil {
		slog.ErrorContext(ctx, "failed to send NATS messages", logging.ErrKey, err)
		// Don't fail the operation if messaging fails
	}

	return participant, nil
}

// DeletePastMeetingParticipant deletes a participant from a past meeting
func (s *PastMeetingParticipantService) DeletePastMeetingParticipant(ctx context.Context, pastMeetingUID, participantUID string, revision uint64) error {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "service not initialized", logging.PriorityCritical())
		return domain.ErrServiceUnavailable
	}

	if pastMeetingUID == "" || participantUID == "" {
		slog.WarnContext(ctx, "past meeting UID and participant UID are required")
		return domain.ErrValidationFailed
	}

	var err error
	if s.Config.SkipEtagValidation {
		// If skipping the Etag validation, we need to get the key revision from the store with a Get request.
		_, revision, err = s.PastMeetingParticipantRepository.GetWithRevision(ctx, participantUID)
		if err != nil {
			if errors.Is(err, domain.ErrPastMeetingParticipantNotFound) {
				slog.WarnContext(ctx, "participant not found", logging.ErrKey, err)
				return domain.ErrPastMeetingParticipantNotFound
			}
			slog.ErrorContext(ctx, "error getting participant from store", logging.ErrKey, err)
			return domain.ErrInternal
		}
	}

	ctx = logging.AppendCtx(ctx, slog.String("past_meeting_uid", pastMeetingUID))
	ctx = logging.AppendCtx(ctx, slog.String("participant_uid", participantUID))
	ctx = logging.AppendCtx(ctx, slog.String("etag", strconv.FormatUint(revision, 10)))

	// Check if the past meeting exists
	exists, err := s.PastMeetingRepository.Exists(ctx, pastMeetingUID)
	if err != nil {
		slog.ErrorContext(ctx, "error checking if past meeting exists", logging.ErrKey, err)
		return domain.ErrInternal
	}
	if !exists {
		slog.WarnContext(ctx, "past meeting not found")
		return domain.ErrPastMeetingNotFound
	}

	// Check if the participant exists and belongs to the specified past meeting
	participant, err := s.PastMeetingParticipantRepository.Get(ctx, participantUID)
	if err != nil {
		if errors.Is(err, domain.ErrPastMeetingParticipantNotFound) {
			slog.WarnContext(ctx, "participant not found", logging.ErrKey, err)
			return domain.ErrPastMeetingParticipantNotFound
		}
		slog.ErrorContext(ctx, "error getting participant", logging.ErrKey, err)
		return domain.ErrInternal
	}

	// Verify the participant belongs to the specified past meeting
	if participant.PastMeetingUID != pastMeetingUID {
		slog.WarnContext(ctx, "participant does not belong to the specified past meeting",
			"expected_past_meeting_uid", pastMeetingUID,
			"actual_past_meeting_uid", participant.PastMeetingUID)
		return domain.ErrPastMeetingParticipantNotFound
	}

	// Delete the participant
	err = s.PastMeetingParticipantRepository.Delete(ctx, participantUID, revision)
	if err != nil {
		if errors.Is(err, domain.ErrRevisionMismatch) {
			slog.WarnContext(ctx, "If-Match header is invalid", logging.ErrKey, err)
			return domain.ErrRevisionMismatch
		}
		if errors.Is(err, domain.ErrPastMeetingParticipantNotFound) {
			slog.WarnContext(ctx, "participant not found", logging.ErrKey, err)
			return domain.ErrPastMeetingParticipantNotFound
		}
		slog.ErrorContext(ctx, "error deleting participant from store", logging.ErrKey, err)
		return domain.ErrInternal
	}

	slog.DebugContext(ctx, "deleted past meeting participant", "participant_uid", participantUID)

	// Use WorkerPool for concurrent NATS deletion message sending
	pool := concurrent.NewWorkerPool(2) // 2 messages to send
	messages := []func() error{
		func() error {
			return s.MessageBuilder.SendDeleteIndexPastMeetingParticipant(ctx, participantUID)
		},
		func() error {
			return s.MessageBuilder.SendRemovePastMeetingParticipantAccess(ctx, models.PastMeetingParticipantAccessMessage{
				UID:            participantUID,
				PastMeetingUID: participant.PastMeetingUID,
				Username:       participant.Username,
				Host:           participant.Host,
			})
		},
	}

	if err := pool.Run(ctx, messages...); err != nil {
		slog.ErrorContext(ctx, "failed to send NATS deletion messages", logging.ErrKey, err)
		// Don't fail the operation if messaging fails - the deletion already succeeded
	}

	return nil
}
