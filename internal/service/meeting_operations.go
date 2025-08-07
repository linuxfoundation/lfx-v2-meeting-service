// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"time"

	"github.com/google/uuid"
	meetingsvc "github.com/linuxfoundation/lfx-v2-meeting-service/gen/meeting_service"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/constants"
	"golang.org/x/sync/errgroup"
)

// GetMeetings fetches all meetings
func (s *MeetingsService) GetMeetings(ctx context.Context) ([]*meetingsvc.Meeting, error) {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "NATS connection or store not initialized")
		return nil, domain.ErrServiceUnavailable
	}

	// Get all meetings from the store
	meetingsBase, err := s.MeetingRepository.ListAll(ctx)
	if err != nil {
		return nil, err
	}

	meetings := make([]*meetingsvc.Meeting, len(meetingsBase))
	for i, meeting := range meetingsBase {
		meetings[i] = models.FromMeetingDBModel(meeting)
	}

	slog.DebugContext(ctx, "returning meetings", "meetings", meetings)

	return meetings, nil
}

func (s *MeetingsService) validateCreateMeetingPayload(ctx context.Context, payload *meetingsvc.CreateMeetingPayload) error {
	if payload == nil {
		return domain.ErrValidationFailed
	}

	startTime, err := time.Parse(time.RFC3339, payload.StartTime)
	if err != nil {
		return domain.ErrValidationFailed
	}

	if startTime.Before(time.Now().UTC()) {
		slog.WarnContext(ctx, "start time cannot be in the past", "start_time", payload.StartTime)
		return domain.ErrValidationFailed
	}

	return nil
}

// CreateMeeting creates a new meeting
func (s *MeetingsService) CreateMeeting(ctx context.Context, payload *meetingsvc.CreateMeetingPayload) (*meetingsvc.Meeting, error) {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "NATS connection or store not initialized")
		return nil, domain.ErrServiceUnavailable
	}

	if err := s.validateCreateMeetingPayload(ctx, payload); err != nil {
		return nil, err
	}

	// TODO: Check if project exists - integrate with project service
	// TODO: Check if committees exist once the committee service is implemented.

	// Convert payload to DB model
	meetingDB := models.ToMeetingDBModelFromCreatePayload(payload)

	// Generate UID for the meeting
	meetingDB.UID = uuid.New().String()

	// Create the meeting in the repository
	err := s.MeetingRepository.Create(ctx, meetingDB)
	if err != nil {
		return nil, domain.ErrInternal
	}

	g := new(errgroup.Group)
	g.Go(func() error {
		return s.MessageBuilder.SendIndexMeeting(ctx, models.ActionCreated, *meetingDB)
	})

	g.Go(func() error {
		return s.MessageBuilder.SendUpdateAccessMeeting(ctx, models.MeetingAccessMessage{
			UID: meetingDB.UID,
		})
	})

	if err := g.Wait(); err != nil {
		return nil, domain.ErrInternal
	}

	slog.DebugContext(ctx, "returning created meeting", "meeting_uid", meetingDB.UID)

	return models.FromMeetingDBModel(meetingDB), nil
}

func (s *MeetingsService) GetOneMeeting(ctx context.Context, payload *meetingsvc.GetMeetingPayload) (*meetingsvc.Meeting, string, error) {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "NATS connection or store not initialized")
		return nil, "", domain.ErrServiceUnavailable
	}

	if payload == nil || payload.UID == nil {
		slog.WarnContext(ctx, "meeting UID is required")
		return nil, "", domain.ErrValidationFailed
	}

	ctx = logging.AppendCtx(ctx, slog.String("meeting_uid", *payload.UID))

	// Get meeting with revision from store
	meetingDB, revision, err := s.MeetingRepository.GetWithRevision(ctx, *payload.UID)
	if err != nil {
		if errors.Is(err, domain.ErrMeetingNotFound) {
			slog.WarnContext(ctx, "meeting not found", logging.ErrKey, err)
			return nil, "", domain.ErrMeetingNotFound
		}
		slog.ErrorContext(ctx, "error getting meeting from store", logging.ErrKey, err)
		return nil, "", domain.ErrInternal
	}

	meeting := models.FromMeetingDBModel(meetingDB)

	// Store the revision in context for the custom encoder to use
	revisionStr := strconv.FormatUint(revision, 10)
	ctx = context.WithValue(ctx, constants.ETagContextID, revisionStr)

	slog.DebugContext(ctx, "returning meeting", "meeting", meeting, "revision", revision)

	return meeting, revisionStr, nil
}

func (s *MeetingsService) validateUpdateMeetingPayload(ctx context.Context, payload *meetingsvc.UpdateMeetingPayload) error {
	if payload == nil {
		return domain.ErrValidationFailed
	}

	startTime, err := time.Parse(time.RFC3339, payload.StartTime)
	if err != nil {
		return domain.ErrValidationFailed
	}

	if startTime.Before(time.Now().UTC()) {
		slog.WarnContext(ctx, "start time cannot be in the past", "start_time", payload.StartTime)
		return domain.ErrValidationFailed
	}

	return nil
}

// Update a meeting's base information.
func (s *MeetingsService) UpdateMeeting(ctx context.Context, payload *meetingsvc.UpdateMeetingPayload) (*meetingsvc.Meeting, error) {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "NATS connection or store not initialized")
		return nil, domain.ErrServiceUnavailable
	}

	if payload == nil || payload.UID == "" {
		slog.WarnContext(ctx, "meeting UID is required")
		return nil, domain.ErrValidationFailed
	}

	var revision uint64
	var err error
	if !s.Config.SkipEtagValidation {
		if payload.Etag == nil {
			slog.WarnContext(ctx, "ETag header is missing")
			return nil, domain.ErrValidationFailed
		}
		revision, err = strconv.ParseUint(*payload.Etag, 10, 64)
		if err != nil {
			slog.ErrorContext(ctx, "error parsing ETag", logging.ErrKey, err)
			return nil, domain.ErrValidationFailed
		}
	} else {
		// If skipping the Etag validation, we need to get the key revision from the store with a Get request.
		_, revision, err = s.MeetingRepository.GetWithRevision(ctx, payload.UID)
		if err != nil {
			if errors.Is(err, domain.ErrMeetingNotFound) {
				slog.WarnContext(ctx, "meeting not found", logging.ErrKey, err)
				return nil, domain.ErrMeetingNotFound
			}
			slog.ErrorContext(ctx, "error getting meeting from store", logging.ErrKey, err)
			return nil, domain.ErrInternal
		}
	}

	ctx = logging.AppendCtx(ctx, slog.String("meeting_uid", payload.UID))
	ctx = logging.AppendCtx(ctx, slog.String("etag", strconv.FormatUint(revision, 10)))

	// Check if the meeting exists and use some of the existing meeting data for the update.
	existingMeetingDB, err := s.MeetingRepository.Get(ctx, payload.UID)
	if err != nil {
		if errors.Is(err, domain.ErrMeetingNotFound) {
			slog.WarnContext(ctx, "meeting not found", logging.ErrKey, err)
			return nil, domain.ErrMeetingNotFound
		}
		slog.ErrorContext(ctx, "error checking if meeting exists", logging.ErrKey, err)
		return nil, domain.ErrInternal
	}

	if err := s.validateUpdateMeetingPayload(ctx, payload); err != nil {
		return nil, err
	}

	// TODO: Check if project exists - integrate with project service
	// TODO: Check if committees exist once the committee service is implemented.

	// Convert payload to DB model
	meetingDB := models.ToMeetingDBModelFromUpdatePayload(payload, existingMeetingDB)

	// Update the meeting in the repository
	err = s.MeetingRepository.Update(ctx, meetingDB, revision)
	if err != nil {
		if errors.Is(err, domain.ErrRevisionMismatch) {
			slog.WarnContext(ctx, "etag header is invalid", logging.ErrKey, err)
			return nil, domain.ErrRevisionMismatch
		}
		if errors.Is(err, domain.ErrInternal) {
			slog.ErrorContext(ctx, "error updating meeting in store", logging.ErrKey, err)
			return nil, domain.ErrInternal
		}
		return nil, domain.ErrInternal
	}

	g := new(errgroup.Group)
	g.Go(func() error {
		return s.MessageBuilder.SendIndexMeeting(ctx, models.ActionUpdated, *meetingDB)
	})

	g.Go(func() error {
		return s.MessageBuilder.SendUpdateAccessMeeting(ctx, models.MeetingAccessMessage{
			UID: meetingDB.UID,
		})
	})

	if err := g.Wait(); err != nil {
		// Return the first error from the goroutines.
		return nil, domain.ErrInternal
	}

	slog.DebugContext(ctx, "returning updated meeting", "meeting", meetingDB)

	meetingResp := models.FromMeetingDBModel(meetingDB)

	return meetingResp, nil
}

// Delete a meeting.
func (s *MeetingsService) DeleteMeeting(ctx context.Context, payload *meetingsvc.DeleteMeetingPayload) error {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "NATS connection or store not initialized")
		return domain.ErrServiceUnavailable
	}

	if payload == nil || payload.UID == nil {
		slog.WarnContext(ctx, "meeting UID is required")
		return domain.ErrValidationFailed
	}

	var revision uint64
	var err error
	if !s.Config.SkipEtagValidation {
		if payload.Etag == nil {
			slog.WarnContext(ctx, "ETag header is missing")
			return domain.ErrValidationFailed
		}
		revision, err = strconv.ParseUint(*payload.Etag, 10, 64)
		if err != nil {
			slog.ErrorContext(ctx, "error parsing ETag", logging.ErrKey, err)
			return domain.ErrValidationFailed
		}
	} else {
		// If skipping the Etag validation, we need to get the key revision from the store with a Get request.
		_, revision, err = s.MeetingRepository.GetWithRevision(ctx, *payload.UID)
		if err != nil {
			if errors.Is(err, domain.ErrMeetingNotFound) {
				slog.WarnContext(ctx, "meeting not found", logging.ErrKey, err)
				return domain.ErrMeetingNotFound
			}
			slog.ErrorContext(ctx, "error getting meeting from store", logging.ErrKey, err)
			return domain.ErrInternal
		}
	}

	ctx = logging.AppendCtx(ctx, slog.String("meeting_uid", *payload.UID))
	ctx = logging.AppendCtx(ctx, slog.String("etag", strconv.FormatUint(revision, 10)))

	// Delete the meeting using the store
	err = s.MeetingRepository.Delete(ctx, *payload.UID, revision)
	if err != nil {
		if errors.Is(err, domain.ErrRevisionMismatch) {
			slog.WarnContext(ctx, "etag header is invalid", logging.ErrKey, err)
			return domain.ErrRevisionMismatch
		}
		if errors.Is(err, domain.ErrMeetingNotFound) {
			slog.WarnContext(ctx, "meeting not found", logging.ErrKey, err)
			return domain.ErrMeetingNotFound
		}
		if errors.Is(err, domain.ErrInternal) {
			slog.ErrorContext(ctx, "error deleting meeting from store", logging.ErrKey, err)
			return domain.ErrInternal
		}
		return domain.ErrInternal
	}

	g := new(errgroup.Group)
	g.Go(func() error {
		return s.MessageBuilder.SendDeleteIndexMeeting(ctx, *payload.UID)
	})

	g.Go(func() error {
		return s.MessageBuilder.SendDeleteAllAccessMeeting(ctx, *payload.UID)
	})

	if err := g.Wait(); err != nil {
		// Return the first error from the goroutines.
		return domain.ErrInternal
	}

	slog.DebugContext(ctx, "deleted meeting", "meeting_uid", *payload.UID)
	return nil
}
