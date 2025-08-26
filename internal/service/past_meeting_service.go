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
)

// PastMeetingService implements the meetingsvc.Service interface and domain.MessageHandler
type PastMeetingService struct {
	MeetingRepository     domain.MeetingRepository
	PastMeetingRepository domain.PastMeetingRepository
	MessageBuilder        domain.MessageBuilder
	Config                ServiceConfig
}

// NewPastMeetingService creates a new PastMeetingService.
func NewPastMeetingService(
	meetingRepository domain.MeetingRepository,
	pastMeetingRepository domain.PastMeetingRepository,
	messageBuilder domain.MessageBuilder,
	config ServiceConfig,
) *PastMeetingService {
	return &PastMeetingService{
		Config:                config,
		MeetingRepository:     meetingRepository,
		PastMeetingRepository: pastMeetingRepository,
		MessageBuilder:        messageBuilder,
	}
}

// ServiceReady checks if the service is ready for use.
func (s *PastMeetingService) ServiceReady() bool {
	return s.MeetingRepository != nil &&
		s.PastMeetingRepository != nil &&
		s.MessageBuilder != nil
}

func (s *PastMeetingService) validateCreatePastMeetingPayload(ctx context.Context, pastMeeting *models.PastMeeting) error {
	// Validate that required fields are present
	if pastMeeting == nil {
		return domain.ErrValidationFailed
	}
	if pastMeeting.MeetingUID == "" {
		return domain.ErrValidationFailed
	}
	if pastMeeting.ProjectUID == "" {
		return domain.ErrValidationFailed
	}
	if pastMeeting.Title == "" {
		return domain.ErrValidationFailed
	}
	if pastMeeting.Description == "" {
		return domain.ErrValidationFailed
	}
	if pastMeeting.Platform == "" {
		return domain.ErrValidationFailed
	}

	// Validate that end time is after start time
	if pastMeeting.ScheduledEndTime.Before(pastMeeting.ScheduledStartTime) {
		slog.WarnContext(ctx, "scheduled end time cannot be before start time")
		return domain.ErrValidationFailed
	}

	return nil
}

func (s *PastMeetingService) CreatePastMeeting(ctx context.Context, pastMeetingReq *models.PastMeeting) (*models.PastMeeting, error) {
	// Check if service is ready
	if !s.ServiceReady() {
		return nil, domain.ErrServiceUnavailable
	}

	// Validate the payload
	if err := s.validateCreatePastMeetingPayload(ctx, pastMeetingReq); err != nil {
		return nil, err
	}

	// Check if the original meeting exists (optional validation)
	exists, err := s.MeetingRepository.Exists(ctx, pastMeetingReq.MeetingUID)
	if err != nil {
		slog.ErrorContext(ctx, "error checking if meeting exists", logging.ErrKey, err)
		return nil, domain.ErrInternal
	}
	if !exists {
		slog.WarnContext(ctx, "referenced meeting does not exist", "meeting_uid", pastMeetingReq.MeetingUID)
		// This is not an error - past meetings can be created for meetings that no longer exist
	}

	// Create the domain model
	pastMeetingReq.UID = uuid.New().String()

	// Save to repository
	if err := s.PastMeetingRepository.Create(ctx, pastMeetingReq); err != nil {
		slog.ErrorContext(ctx, "error creating past meeting", logging.ErrKey, err)
		return nil, domain.ErrInternal
	}

	// Use WorkerPool for concurrent NATS message sending
	pool := concurrent.NewWorkerPool(2) // 2 messages to send
	messages := []func() error{
		func() error {
			return s.MessageBuilder.SendIndexPastMeeting(ctx, models.ActionCreated, *pastMeetingReq)
		},
		func() error {
			// For the message we only need the committee UIDs.
			committees := make([]string, len(pastMeetingReq.Committees))
			for i, committee := range pastMeetingReq.Committees {
				committees[i] = committee.UID
			}
			// Determine if the meeting is public based on visibility
			isPublic := pastMeetingReq.Visibility == "public"

			return s.MessageBuilder.SendUpdateAccessPastMeeting(ctx, models.PastMeetingAccessMessage{
				UID:        pastMeetingReq.UID,
				MeetingUID: pastMeetingReq.MeetingUID,
				Public:     isPublic,
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

func (s *PastMeetingService) GetPastMeetings(ctx context.Context) ([]*models.PastMeeting, error) {
	if !s.ServiceReady() {
		return nil, domain.ErrServiceUnavailable
	}

	pastMeetings, err := s.PastMeetingRepository.ListAll(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "error listing past meetings", logging.ErrKey, err)
		return nil, domain.ErrInternal
	}

	return pastMeetings, nil
}

func (s *PastMeetingService) GetPastMeeting(ctx context.Context, uid string) (*models.PastMeeting, string, error) {
	if !s.ServiceReady() {
		return nil, "", domain.ErrServiceUnavailable
	}

	pastMeeting, revision, err := s.PastMeetingRepository.GetWithRevision(ctx, uid)
	if err != nil {
		if errors.Is(err, domain.ErrPastMeetingNotFound) {
			slog.WarnContext(ctx, "past meeting not found", logging.ErrKey, err)
			return nil, "", domain.ErrPastMeetingNotFound
		}
		slog.ErrorContext(ctx, "error getting past meeting", logging.ErrKey, err)
		return nil, "", domain.ErrInternal
	}

	return pastMeeting, strconv.FormatUint(revision, 10), nil
}

func (s *PastMeetingService) DeletePastMeeting(ctx context.Context, uid string, revision uint64) error {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "past meeting service not ready", logging.PriorityCritical())
		return domain.ErrServiceUnavailable
	}

	var err error
	if s.Config.SkipEtagValidation {
		// If skipping the Etag validation, we need to get the key revision from the store with a Get request.
		_, revision, err = s.PastMeetingRepository.GetWithRevision(ctx, uid)
		if err != nil {
			if errors.Is(err, domain.ErrPastMeetingNotFound) {
				slog.WarnContext(ctx, "past meeting not found", logging.ErrKey, err)
				return domain.ErrPastMeetingNotFound
			}
			slog.ErrorContext(ctx, "error getting meeting from store", logging.ErrKey, err)
			return domain.ErrInternal
		}
	}

	// Check if the past meeting exists
	exists, err := s.PastMeetingRepository.Exists(ctx, uid)
	if err != nil {
		slog.ErrorContext(ctx, "error checking if past meeting exists", logging.ErrKey, err)
		return domain.ErrInternal
	}
	if !exists {
		slog.WarnContext(ctx, "past meeting not found", "uid", uid)
		return domain.ErrPastMeetingNotFound
	}

	// Delete the past meeting
	if err := s.PastMeetingRepository.Delete(ctx, uid, revision); err != nil {
		if errors.Is(err, domain.ErrRevisionMismatch) {
			slog.WarnContext(ctx, "If-Match header is invalid", logging.ErrKey, err)
			return domain.ErrRevisionMismatch
		}
		if errors.Is(err, domain.ErrPastMeetingNotFound) {
			slog.WarnContext(ctx, "past meeting not found during deletion", logging.ErrKey, err)
			return domain.ErrPastMeetingNotFound
		}
		slog.ErrorContext(ctx, "error deleting past meeting", logging.ErrKey, err)
		return domain.ErrInternal
	}

	// Use WorkerPool for concurrent NATS deletion message sending
	pool := concurrent.NewWorkerPool(2) // 2 messages to send
	messages := []func() error{
		func() error {
			return s.MessageBuilder.SendDeleteIndexPastMeeting(ctx, uid)
		},
		func() error {
			return s.MessageBuilder.SendDeleteAllAccessPastMeeting(ctx, uid)
		},
	}

	if err := pool.Run(ctx, messages...); err != nil {
		slog.ErrorContext(ctx, "failed to send NATS deletion messages", logging.ErrKey, err)
		// Don't fail the operation if messaging fails - the deletion already succeeded
	}

	return nil
}
