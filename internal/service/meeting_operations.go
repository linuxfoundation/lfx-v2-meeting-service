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
func (s *MeetingsService) GetMeetings(ctx context.Context) ([]*meetingsvc.MeetingFull, error) {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "NATS connection or store not initialized")
		return nil, domain.ErrServiceUnavailable
	}

	// Get all meetings from the store
	meetingsBase, meetingSettings, err := s.MeetingRepository.ListAll(ctx)
	if err != nil {
		return nil, err
	}

	// Build lookup for settings by UID to avoid index coupling
	settingsByUID := make(map[string]*models.MeetingSettings, len(meetingSettings))
	for _, s := range meetingSettings {
		if s != nil {
			settingsByUID[s.UID] = s
		}
	}

	meetings := make([]*meetingsvc.MeetingFull, len(meetingsBase))
	for i, meeting := range meetingsBase {
		var s *models.MeetingSettings
		if meeting != nil {
			s = settingsByUID[meeting.UID]
		}
		meetings[i] = models.ToMeetingFullServiceModel(meeting, s)
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
func (s *MeetingsService) CreateMeeting(ctx context.Context, payload *meetingsvc.CreateMeetingPayload) (*meetingsvc.MeetingFull, error) {
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
	if meetingDB == nil {
		// This should never happen since we validate the payload above.
		// Therefore we can return an internal error.
		return nil, domain.ErrInternal
	}

	// Generate UID for the meeting
	meetingDB.UID = uuid.New().String()

	now := time.Now().UTC()
	meetingSettingsDB := &models.MeetingSettings{
		UID:        meetingDB.UID,
		Organizers: payload.Organizers,
		CreatedAt:  &now,
		UpdatedAt:  &now,
	}

	// Create the meeting in the repository
	err := s.MeetingRepository.Create(ctx, meetingDB, meetingSettingsDB)
	if err != nil {
		return nil, domain.ErrInternal
	}

	g := new(errgroup.Group)
	g.Go(func() error {
		return s.MessageBuilder.SendIndexMeeting(ctx, models.ActionCreated, *meetingDB)
	})

	g.Go(func() error {
		return s.MessageBuilder.SendIndexMeetingSettings(ctx, models.ActionCreated, *meetingSettingsDB)
	})

	g.Go(func() error {
		// For the message we only need the committee UIDs.
		committees := make([]string, len(meetingDB.Committees))
		for i, committee := range meetingDB.Committees {
			committees[i] = committee.UID
		}

		return s.MessageBuilder.SendUpdateAccessMeeting(ctx, models.MeetingAccessMessage{
			UID:        meetingDB.UID,
			Public:     meetingDB.Visibility == "public",
			ProjectUID: meetingDB.ProjectUID,
			Organizers: meetingSettingsDB.Organizers,
			Committees: committees,
		})
	})

	if err := g.Wait(); err != nil {
		return nil, domain.ErrInternal
	}

	slog.DebugContext(ctx, "returning created meeting", "meeting_uid", meetingDB.UID)

	return models.ToMeetingFullServiceModel(meetingDB, meetingSettingsDB), nil
}

func (s *MeetingsService) GetMeetingBase(ctx context.Context, payload *meetingsvc.GetMeetingBasePayload) (*meetingsvc.MeetingBase, string, error) {
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
	meetingDB, revision, err := s.MeetingRepository.GetBaseWithRevision(ctx, *payload.UID)
	if err != nil {
		if errors.Is(err, domain.ErrMeetingNotFound) {
			slog.WarnContext(ctx, "meeting not found", logging.ErrKey, err)
			return nil, "", domain.ErrMeetingNotFound
		}
		slog.ErrorContext(ctx, "error getting meeting from store", logging.ErrKey, err)
		return nil, "", domain.ErrInternal
	}

	meeting := models.FromMeetingBaseDBModel(meetingDB)

	// Store the revision in context for the custom encoder to use
	revisionStr := strconv.FormatUint(revision, 10)
	ctx = context.WithValue(ctx, constants.ETagContextID, revisionStr)

	slog.DebugContext(ctx, "returning meeting", "meeting", meeting, "revision", revision)

	return meeting, revisionStr, nil
}

// GetMeetingSettings fetches settings for a specific meeting by ID
func (s *MeetingsService) GetMeetingSettings(ctx context.Context, payload *meetingsvc.GetMeetingSettingsPayload) (*meetingsvc.MeetingSettings, string, error) {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "NATS connection or store not initialized")
		return nil, "", domain.ErrServiceUnavailable
	}

	if payload == nil || payload.UID == nil {
		slog.WarnContext(ctx, "meeting UID is required")
		return nil, "", domain.ErrValidationFailed
	}

	ctx = logging.AppendCtx(ctx, slog.String("meeting_uid", *payload.UID))

	// Get meeting settings with revision from store
	settingsDB, revision, err := s.MeetingRepository.GetSettingsWithRevision(ctx, *payload.UID)
	if err != nil {
		if errors.Is(err, domain.ErrMeetingNotFound) {
			slog.WarnContext(ctx, "meeting settings not found", logging.ErrKey, err)
			return nil, "", domain.ErrMeetingNotFound
		}
		slog.ErrorContext(ctx, "error getting meeting settings from store", logging.ErrKey, err)
		return nil, "", domain.ErrInternal
	}

	settings := models.ToMeetingSettingsServiceModel(settingsDB)

	// Store the revision in context for the custom encoder to use
	revisionStr := strconv.FormatUint(revision, 10)
	ctx = context.WithValue(ctx, constants.ETagContextID, revisionStr)

	slog.DebugContext(ctx, "returning meeting settings", "settings", settings, "revision", revision)

	return settings, revisionStr, nil
}

func (s *MeetingsService) validateUpdateMeetingPayload(ctx context.Context, payload *meetingsvc.UpdateMeetingBasePayload) error {
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
func (s *MeetingsService) UpdateMeetingBase(ctx context.Context, payload *meetingsvc.UpdateMeetingBasePayload) (*meetingsvc.MeetingBase, error) {
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
		if payload.IfMatch == nil {
			slog.WarnContext(ctx, "If-Match header is missing")
			return nil, domain.ErrValidationFailed
		}
		revision, err = strconv.ParseUint(*payload.IfMatch, 10, 64)
		if err != nil {
			slog.ErrorContext(ctx, "error parsing If-Match header", logging.ErrKey, err)
			return nil, domain.ErrValidationFailed
		}
	} else {
		// If skipping the Etag validation, we need to get the key revision from the store with a Get request.
		_, revision, err = s.MeetingRepository.GetBaseWithRevision(ctx, payload.UID)
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
	existingMeetingDB, err := s.MeetingRepository.GetBase(ctx, payload.UID)
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
	meetingDB := models.ToMeetingBaseDBModelFromUpdatePayload(payload, existingMeetingDB)
	if meetingDB == nil {
		// This should never happen since we validate the payload above.
		// Therefore we can return an internal error.
		return nil, domain.ErrInternal
	}

	// Update the meeting in the repository
	err = s.MeetingRepository.UpdateBase(ctx, meetingDB, revision)
	if err != nil {
		if errors.Is(err, domain.ErrRevisionMismatch) {
			slog.WarnContext(ctx, "If-Match header is invalid", logging.ErrKey, err)
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
		// Get the meeting settings to retrieve organizers
		settingsDB, err := s.MeetingRepository.GetSettings(ctx, meetingDB.UID)
		if err != nil {
			// If we can't get settings, use empty organizers array rather than failing
			slog.WarnContext(ctx, "could not retrieve meeting settings for access message", logging.ErrKey, err)
			settingsDB = &models.MeetingSettings{Organizers: []string{}}
		}

		// For the message we only need the committee UIDs.
		committees := make([]string, len(meetingDB.Committees))
		for i, committee := range meetingDB.Committees {
			committees[i] = committee.UID
		}

		return s.MessageBuilder.SendUpdateAccessMeeting(ctx, models.MeetingAccessMessage{
			UID:        meetingDB.UID,
			Public:     meetingDB.Visibility == "public",
			ProjectUID: meetingDB.ProjectUID,
			Organizers: settingsDB.Organizers,
			Committees: committees,
		})
	})

	if err := g.Wait(); err != nil {
		// Return the first error from the goroutines.
		return nil, domain.ErrInternal
	}

	slog.DebugContext(ctx, "returning updated meeting", "meeting", meetingDB)

	meetingResp := models.FromMeetingBaseDBModel(meetingDB)

	return meetingResp, nil
}

// UpdateMeetingSettings updates a meeting's settings
func (s *MeetingsService) UpdateMeetingSettings(ctx context.Context, payload *meetingsvc.UpdateMeetingSettingsPayload) (*meetingsvc.MeetingSettings, error) {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "NATS connection or store not initialized")
		return nil, domain.ErrServiceUnavailable
	}

	if payload == nil || payload.UID == nil {
		slog.WarnContext(ctx, "meeting UID is required")
		return nil, domain.ErrValidationFailed
	}

	var revision uint64
	var err error
	if !s.Config.SkipEtagValidation {
		if payload.IfMatch == nil {
			slog.WarnContext(ctx, "If-Match header is missing")
			return nil, domain.ErrValidationFailed
		}
		revision, err = strconv.ParseUint(*payload.IfMatch, 10, 64)
		if err != nil {
			slog.ErrorContext(ctx, "error parsing If-Match header", logging.ErrKey, err)
			return nil, domain.ErrValidationFailed
		}
	} else {
		// If skipping the Etag validation, we need to get the key revision from the store with a Get request.
		_, revision, err = s.MeetingRepository.GetSettingsWithRevision(ctx, *payload.UID)
		if err != nil {
			if errors.Is(err, domain.ErrMeetingNotFound) {
				slog.WarnContext(ctx, "meeting settings not found", logging.ErrKey, err)
				return nil, domain.ErrMeetingNotFound
			}
			slog.ErrorContext(ctx, "error getting meeting settings from store", logging.ErrKey, err)
			return nil, domain.ErrInternal
		}
	}

	ctx = logging.AppendCtx(ctx, slog.String("meeting_uid", *payload.UID))
	ctx = logging.AppendCtx(ctx, slog.String("etag", strconv.FormatUint(revision, 10)))

	// Check if the meeting settings exist and use some of the existing data for the update.
	existingSettingsDB, err := s.MeetingRepository.GetSettings(ctx, *payload.UID)
	if err != nil {
		if errors.Is(err, domain.ErrMeetingNotFound) {
			slog.WarnContext(ctx, "meeting settings not found", logging.ErrKey, err)
			return nil, domain.ErrMeetingNotFound
		}
		slog.ErrorContext(ctx, "error checking if meeting settings exist", logging.ErrKey, err)
		return nil, domain.ErrInternal
	}

	// Update the settings with new data
	now := time.Now().UTC()
	updatedSettingsDB := &models.MeetingSettings{
		UID:        *payload.UID,
		Organizers: payload.Organizers,
		CreatedAt:  existingSettingsDB.CreatedAt,
		UpdatedAt:  &now,
	}

	// Update the meeting settings in the repository
	err = s.MeetingRepository.UpdateSettings(ctx, updatedSettingsDB, revision)
	if err != nil {
		if errors.Is(err, domain.ErrRevisionMismatch) {
			slog.WarnContext(ctx, "If-Match header is invalid", logging.ErrKey, err)
			return nil, domain.ErrRevisionMismatch
		}
		if errors.Is(err, domain.ErrInternal) {
			slog.ErrorContext(ctx, "error updating meeting settings in store", logging.ErrKey, err)
			return nil, domain.ErrInternal
		}
		return nil, domain.ErrInternal
	}

	g := new(errgroup.Group)
	g.Go(func() error {
		return s.MessageBuilder.SendIndexMeetingSettings(ctx, models.ActionUpdated, *updatedSettingsDB)
	})

	g.Go(func() error {
		// Get the meeting base data to send access update message
		meetingDB, err := s.MeetingRepository.GetBase(ctx, *payload.UID)
		if err != nil {
			// Don't fail the message if we can't get the meeting base data
			// since the settings were already updated.
			slog.WarnContext(ctx, "could not retrieve meeting base data for access message", logging.ErrKey, err)
			return nil
		}

		// For the message we only need the committee UIDs.
		committees := make([]string, len(meetingDB.Committees))
		for i, committee := range meetingDB.Committees {
			committees[i] = committee.UID
		}

		return s.MessageBuilder.SendUpdateAccessMeeting(ctx, models.MeetingAccessMessage{
			UID:        meetingDB.UID,
			Public:     meetingDB.Visibility == "public",
			ProjectUID: meetingDB.ProjectUID,
			Organizers: updatedSettingsDB.Organizers,
			Committees: committees,
		})
	})

	if err := g.Wait(); err != nil {
		// Return the first error from the goroutines.
		return nil, domain.ErrInternal
	}

	slog.DebugContext(ctx, "returning updated meeting settings", "settings", updatedSettingsDB)

	settingsResp := models.ToMeetingSettingsServiceModel(updatedSettingsDB)

	return settingsResp, nil
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
		if payload.IfMatch == nil {
			slog.WarnContext(ctx, "If-Match header is missing")
			return domain.ErrValidationFailed
		}
		revision, err = strconv.ParseUint(*payload.IfMatch, 10, 64)
		if err != nil {
			slog.ErrorContext(ctx, "error parsing If-Match header", logging.ErrKey, err)
			return domain.ErrValidationFailed
		}
	} else {
		// If skipping the Etag validation, we need to get the key revision from the store with a Get request.
		_, revision, err = s.MeetingRepository.GetBaseWithRevision(ctx, *payload.UID)
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
			slog.WarnContext(ctx, "If-Match header is invalid", logging.ErrKey, err)
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
		return s.MessageBuilder.SendDeleteIndexMeetingSettings(ctx, *payload.UID)
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
