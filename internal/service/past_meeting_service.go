// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/concurrent"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/constants"
)

// PastMeetingService implements the meetingsvc.Service interface and domain.MessageHandler
type PastMeetingService struct {
	meetingRepository     domain.MeetingRepository
	pastMeetingRepository domain.PastMeetingRepository
	messageSender         domain.PastMeetingBasicMessageSender
	config                ServiceConfig
}

// NewPastMeetingService creates a new PastMeetingService.
func NewPastMeetingService(
	meetingRepository domain.MeetingRepository,
	pastMeetingRepository domain.PastMeetingRepository,
	messageSender domain.PastMeetingBasicMessageSender,
	config ServiceConfig,
) *PastMeetingService {
	return &PastMeetingService{
		config:                config,
		meetingRepository:     meetingRepository,
		pastMeetingRepository: pastMeetingRepository,
		messageSender:         messageSender,
	}
}

// ServiceReady checks if the service is ready for use.
func (s *PastMeetingService) ServiceReady() bool {
	return s.meetingRepository != nil &&
		s.pastMeetingRepository != nil &&
		s.messageSender != nil
}

func (s *PastMeetingService) ListPastMeetings(ctx context.Context) ([]*models.PastMeeting, error) {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "service not initialized", logging.PriorityCritical())
		return nil, domain.NewUnavailableError("service not initialized")
	}

	pastMeetings, err := s.pastMeetingRepository.ListAll(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "error listing past meetings", logging.ErrKey, err)
		return nil, err
	}

	return pastMeetings, nil
}

func (s *PastMeetingService) validateCreatePastMeetingPayload(ctx context.Context, pastMeeting *models.PastMeeting) error {
	// Validate that required fields are present
	if pastMeeting == nil {
		return domain.NewValidationError("past meeting payload is required")
	}
	if pastMeeting.MeetingUID == "" {
		return domain.NewValidationError("meeting UID is required")
	}
	if pastMeeting.ProjectUID == "" {
		return domain.NewValidationError("project UID is required")
	}
	if pastMeeting.Title == "" {
		return domain.NewValidationError("title is required")
	}
	if pastMeeting.Description == "" {
		return domain.NewValidationError("description is required")
	}
	if pastMeeting.Platform == "" {
		return domain.NewValidationError("platform is required")
	}

	// Validate that the meeting scheduled start time is not too far in the future
	// Allow up to MaxEarlyJoinTimeMinutes in the future to account for early join functionality
	maxFutureTime := time.Now().UTC().Add(time.Duration(constants.MaxEarlyJoinTimeMinutes) * time.Minute)
	if pastMeeting.ScheduledStartTime.After(maxFutureTime) {
		slog.WarnContext(ctx, "scheduled start time is too far in the future")
		return domain.NewValidationError(fmt.Sprintf("scheduled start time cannot be more than %d minutes in the future", constants.MaxEarlyJoinTimeMinutes))
	}

	// Validate that end time is after start time
	if pastMeeting.ScheduledEndTime.Before(pastMeeting.ScheduledStartTime) {
		slog.WarnContext(ctx, "scheduled end time cannot be before start time")
		return domain.NewValidationError("scheduled end time cannot be before start time")
	}

	// Validate that scheduled end time is not greater than the maximum duration from start time
	maxDuration := time.Duration(constants.MaxMeetingDurationMinutes) * time.Minute
	maxEndTime := pastMeeting.ScheduledStartTime.Add(maxDuration)
	if pastMeeting.ScheduledEndTime.After(maxEndTime) {
		slog.WarnContext(ctx, "scheduled end time is too far from start time")
		return domain.NewValidationError(fmt.Sprintf("scheduled end time cannot be more than %d minutes from start time", constants.MaxMeetingDurationMinutes))
	}

	return nil
}

func (s *PastMeetingService) CreatePastMeeting(ctx context.Context, pastMeetingReq *models.PastMeeting) (*models.PastMeeting, error) {
	// Check if service is ready
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "service not initialized", logging.PriorityCritical())
		return nil, domain.NewUnavailableError("service not initialized")
	}

	// Validate the payload
	if err := s.validateCreatePastMeetingPayload(ctx, pastMeetingReq); err != nil {
		return nil, err
	}

	// Check if the original meeting exists (optional validation)
	exists, err := s.meetingRepository.Exists(ctx, pastMeetingReq.MeetingUID)
	if err != nil {
		slog.ErrorContext(ctx, "error checking if meeting exists", logging.ErrKey, err)
		return nil, err
	}
	if !exists {
		slog.WarnContext(ctx, "referenced meeting does not exist", "meeting_uid", pastMeetingReq.MeetingUID)
		// This is not an error - past meetings can be created for meetings that no longer exist
	}

	// Create the domain model
	pastMeetingReq.UID = uuid.New().String()

	// Save to repository
	if err := s.pastMeetingRepository.Create(ctx, pastMeetingReq); err != nil {
		slog.ErrorContext(ctx, "error creating past meeting", logging.ErrKey, err)
		return nil, err
	}

	// Use WorkerPool for concurrent NATS message sending
	pool := concurrent.NewWorkerPool(2) // 2 messages to send
	messages := []func() error{
		func() error {
			return s.messageSender.SendIndexPastMeeting(ctx, models.ActionCreated, *pastMeetingReq)
		},
		func() error {
			// For the message we only need the committee UIDs.
			committees := make([]string, len(pastMeetingReq.Committees))
			for i, committee := range pastMeetingReq.Committees {
				committees[i] = committee.UID
			}

			return s.messageSender.SendUpdateAccessPastMeeting(ctx, models.PastMeetingAccessMessage{
				UID:        pastMeetingReq.UID,
				MeetingUID: pastMeetingReq.MeetingUID,
				Public:     pastMeetingReq.IsPublic(),
				ProjectUID: pastMeetingReq.ProjectUID,
				Committees: committees,
			})
		},
	}

	if err := pool.Run(ctx, messages...); err != nil {
		slog.ErrorContext(ctx, "failed to send NATS messages", logging.ErrKey, err)
		// Don't fail the operation if messaging fails
	}

	return pastMeetingReq, nil
}

func (s *PastMeetingService) GetPastMeeting(ctx context.Context, uid string) (*models.PastMeeting, string, error) {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "service not initialized", logging.PriorityCritical())
		return nil, "", domain.NewUnavailableError("service not initialized")
	}

	pastMeeting, revision, err := s.pastMeetingRepository.GetWithRevision(ctx, uid)
	if err != nil {
		slog.ErrorContext(ctx, "error getting past meeting", logging.ErrKey, err)
		return nil, "", err
	}

	return pastMeeting, strconv.FormatUint(revision, 10), nil
}

// GetByPlatformMeetingIDAndOccurrence gets a past meeting by platform meeting ID and occurrence
func (s *PastMeetingService) GetByPlatformMeetingIDAndOccurrence(ctx context.Context, platform, platformMeetingID, occurrenceID string) (*models.PastMeeting, error) {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "service not initialized", logging.PriorityCritical())
		return nil, domain.NewUnavailableError("service not initialized")
	}

	ctx = logging.AppendCtx(ctx,
		slog.String("platform", platform))
	ctx = logging.AppendCtx(ctx,
		slog.String("platform_meeting_id", platformMeetingID))
	ctx = logging.AppendCtx(ctx,
		slog.String("occurrence_id", occurrenceID))

	pastMeeting, err := s.pastMeetingRepository.GetByPlatformMeetingIDAndOccurrence(ctx, platform, platformMeetingID, occurrenceID)
	if err != nil {
		return nil, err
	}

	slog.DebugContext(ctx, "returning past meeting by platform meeting ID and occurrence", "past_meeting_uid", pastMeeting.UID)

	return pastMeeting, nil
}

func (s *PastMeetingService) UpdatePastMeeting(ctx context.Context, pastMeeting *models.PastMeeting, revision uint64) error {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "service not initialized", logging.PriorityCritical())
		return domain.NewUnavailableError("service not initialized")
	}

	if pastMeeting == nil || pastMeeting.UID == "" {
		slog.WarnContext(ctx, "past meeting UID is required")
		return domain.NewValidationError("past meeting UID is required for update")
	}

	var err error
	if s.config.SkipEtagValidation {
		// If skipping the Etag validation, we need to get the key revision from the store with a Get request.
		_, revision, err = s.pastMeetingRepository.GetWithRevision(ctx, pastMeeting.UID)
		if err != nil {
			slog.ErrorContext(ctx, "error getting past meeting from store", logging.ErrKey, err)
			return err
		}
	}

	// Update the past meeting
	if err := s.pastMeetingRepository.Update(ctx, pastMeeting, revision); err != nil {
		slog.ErrorContext(ctx, "error updating past meeting", logging.ErrKey, err)
		return err
	}

	// Use WorkerPool for concurrent NATS message sending
	pool := concurrent.NewWorkerPool(2) // 2 messages to send
	messages := []func() error{
		func() error {
			return s.messageSender.SendIndexPastMeeting(ctx, models.ActionUpdated, *pastMeeting)
		},
		func() error {
			// For the message we only need the committee UIDs.
			committees := make([]string, len(pastMeeting.Committees))
			for i, committee := range pastMeeting.Committees {
				committees[i] = committee.UID
			}

			return s.messageSender.SendUpdateAccessPastMeeting(ctx, models.PastMeetingAccessMessage{
				UID:        pastMeeting.UID,
				MeetingUID: pastMeeting.MeetingUID,
				Public:     pastMeeting.IsPublic(),
				ProjectUID: pastMeeting.ProjectUID,
				Committees: committees,
			})
		},
	}

	if err := pool.Run(ctx, messages...); err != nil {
		slog.ErrorContext(ctx, "failed to send NATS messages", logging.ErrKey, err)
		// Don't fail the operation if messaging fails
	}

	return nil
}

func (s *PastMeetingService) DeletePastMeeting(ctx context.Context, uid string, revision uint64) error {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "service not initialized", logging.PriorityCritical())
		return domain.NewUnavailableError("service not initialized")
	}

	var err error
	if s.config.SkipEtagValidation {
		// If skipping the Etag validation, we need to get the key revision from the store with a Get request.
		_, revision, err = s.pastMeetingRepository.GetWithRevision(ctx, uid)
		if err != nil {
			slog.ErrorContext(ctx, "error getting meeting from store", logging.ErrKey, err)
			return err
		}
	}

	// Check if the past meeting exists
	exists, err := s.pastMeetingRepository.Exists(ctx, uid)
	if err != nil {
		slog.ErrorContext(ctx, "error checking if past meeting exists", logging.ErrKey, err)
		return err
	}
	if !exists {
		slog.WarnContext(ctx, "past meeting not found", "uid", uid)
		return domain.NewNotFoundError("past meeting not found")
	}

	// Delete the past meeting
	if err := s.pastMeetingRepository.Delete(ctx, uid, revision); err != nil {
		slog.ErrorContext(ctx, "error deleting past meeting", logging.ErrKey, err)
		return err
	}

	// Use WorkerPool for concurrent NATS deletion message sending
	pool := concurrent.NewWorkerPool(2) // 2 messages to send
	messages := []func() error{
		func() error {
			return s.messageSender.SendDeleteIndexPastMeeting(ctx, uid)
		},
		func() error {
			return s.messageSender.SendDeleteAllAccessPastMeeting(ctx, uid)
		},
	}

	if err := pool.Run(ctx, messages...); err != nil {
		slog.ErrorContext(ctx, "failed to send NATS deletion messages", logging.ErrKey, err)
		// Don't fail the operation if messaging fails - the deletion already succeeded
	}

	return nil
}
