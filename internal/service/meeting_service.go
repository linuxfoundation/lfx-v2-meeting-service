// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/concurrent"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/constants"
)

// MeetingsService implements the meetingsvc.Service interface and domain.MessageHandler
type MeetingService struct {
	MeetingRepository domain.MeetingRepository
	MessageBuilder    domain.MessageBuilder
	PlatformRegistry  domain.PlatformRegistry
	OccurrenceService domain.OccurrenceService
	Config            ServiceConfig
}

// NewMeetingsService creates a new MeetingsService.
func NewMeetingService(
	meetingRepository domain.MeetingRepository,
	messageBuilder domain.MessageBuilder,
	platformRegistry domain.PlatformRegistry,
	occurrenceService domain.OccurrenceService,
	config ServiceConfig,
) *MeetingService {
	return &MeetingService{
		MeetingRepository: meetingRepository,
		MessageBuilder:    messageBuilder,
		PlatformRegistry:  platformRegistry,
		OccurrenceService: occurrenceService,
		Config:            config,
	}
}

// detectMeetingBaseChanges compares two MeetingBase structs and returns a map of changes
func detectMeetingBaseChanges(oldMeeting, newMeeting *models.MeetingBase) map[string]any {
	changes := make(map[string]any)

	if oldMeeting.Title != newMeeting.Title {
		changes["Title"] = newMeeting.Title
	}
	if oldMeeting.Description != newMeeting.Description {
		changes["Description"] = newMeeting.Description
	}
	if !oldMeeting.StartTime.Equal(newMeeting.StartTime) {
		changes["Start Time"] = newMeeting.StartTime.Format("2006-01-02 15:04:05 MST")
	}
	if oldMeeting.Duration != newMeeting.Duration {
		changes["Duration"] = fmt.Sprintf("%d minutes", newMeeting.Duration)
	}
	if oldMeeting.Timezone != newMeeting.Timezone {
		changes["Timezone"] = newMeeting.Timezone
	}
	if oldMeeting.Visibility != newMeeting.Visibility {
		changes["Visibility"] = newMeeting.Visibility
	}
	if oldMeeting.JoinURL != newMeeting.JoinURL {
		changes["Join URL"] = newMeeting.JoinURL
	}

	// Compare platform-specific fields
	if oldMeeting.Platform != newMeeting.Platform {
		changes["Platform"] = newMeeting.Platform
	}

	// Check if Zoom config changed (basic comparison)
	switch {
	case oldMeeting.ZoomConfig != nil && newMeeting.ZoomConfig != nil:
		if oldMeeting.ZoomConfig.MeetingID != newMeeting.ZoomConfig.MeetingID {
			changes["Meeting ID"] = newMeeting.ZoomConfig.MeetingID
		}
	case oldMeeting.ZoomConfig == nil && newMeeting.ZoomConfig != nil:
		changes["Zoom Configuration"] = "Added Zoom configuration"
	case oldMeeting.ZoomConfig != nil && newMeeting.ZoomConfig == nil:
		changes["Zoom Configuration"] = "Removed Zoom configuration"
	}

	// Compare recurrence using cmp package for comprehensive comparison
	oldHasRecurrence := oldMeeting.Recurrence != nil
	newHasRecurrence := newMeeting.Recurrence != nil
	if oldHasRecurrence != newHasRecurrence {
		if newHasRecurrence {
			changes["Recurrence"] = "Added recurrence pattern"
		} else {
			changes["Recurrence"] = "Removed recurrence pattern"
		}
	} else if oldHasRecurrence && newHasRecurrence {
		if !cmp.Equal(oldMeeting.Recurrence, newMeeting.Recurrence) {
			changes["Recurrence"] = "Modified recurrence pattern"
		}
	}

	return changes
}

// ServiceReady checks if the service is ready for use.
func (s *MeetingService) ServiceReady() bool {
	return s.MeetingRepository != nil &&
		s.MessageBuilder != nil &&
		s.PlatformRegistry != nil &&
		s.OccurrenceService != nil
}

// GetMeetings fetches all meetings
func (s *MeetingService) GetMeetings(ctx context.Context) ([]*models.MeetingFull, error) {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "service not initialized", logging.PriorityCritical())
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

	meetings := make([]*models.MeetingFull, len(meetingsBase))
	currentTime := time.Now()

	for i, meeting := range meetingsBase {
		var settings *models.MeetingSettings
		if meeting != nil {
			settings = settingsByUID[meeting.UID]
			// Calculate next 50 occurrences from current time
			meeting.Occurrences = s.OccurrenceService.CalculateOccurrencesFromDate(meeting, currentTime, 50)
		}
		meetings[i] = &models.MeetingFull{
			Base:     meeting,
			Settings: settings,
		}
	}

	slog.DebugContext(ctx, "returning meetings", "meetings", meetings)

	return meetings, nil
}

func (s *MeetingService) validateCreateMeetingPayload(ctx context.Context, payload *models.MeetingFull) error {
	if payload == nil || payload.Base == nil {
		return domain.ErrValidationFailed
	}

	if payload.Base.StartTime.Before(time.Now().UTC()) {
		slog.WarnContext(ctx, "start time cannot be in the past", "start_time", payload.Base.StartTime)
		return domain.ErrValidationFailed
	}

	return nil
}

// CreateMeeting creates a new meeting
func (s *MeetingService) CreateMeeting(ctx context.Context, reqMeeting *models.MeetingFull) (*models.MeetingFull, error) {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "service not initialized", logging.PriorityCritical())
		return nil, domain.ErrServiceUnavailable
	}

	if err := s.validateCreateMeetingPayload(ctx, reqMeeting); err != nil {
		return nil, err
	}

	// TODO: Check if project exists - integrate with project service
	// TODO: Check if committees exist once the committee service is implemented.

	// Generate UID for the meeting
	reqMeeting.Base.UID = uuid.New().String()
	reqMeeting.Settings.UID = reqMeeting.Base.UID

	// Generate password for the meeting
	reqMeeting.Base.Password = uuid.New().String()

	// Create meeting on external platform if configured
	if reqMeeting.Base.Platform != "" {
		provider, err := s.PlatformRegistry.GetProvider(reqMeeting.Base.Platform)
		if err != nil {
			slog.ErrorContext(ctx, "failed to get platform provider",
				"platform", reqMeeting.Base.Platform,
				logging.ErrKey, err)
			return nil, domain.ErrInternal
		}

		result, err := provider.CreateMeeting(ctx, reqMeeting.Base)
		if err != nil {
			slog.ErrorContext(ctx, "failed to create platform meeting",
				"platform", reqMeeting.Base.Platform,
				logging.ErrKey, err)
			return nil, domain.ErrInternal
		}

		// Store platform-specific data using the provider
		provider.StorePlatformData(reqMeeting.Base, result)

		slog.InfoContext(ctx, "created platform meeting",
			"platform", reqMeeting.Base.Platform,
			"platform_meeting_id", result.PlatformMeetingID)
	}

	// Calculate first 50 occurrences for the new meeting
	reqMeeting.Base.Occurrences = s.OccurrenceService.CalculateOccurrences(reqMeeting.Base, 50)

	// Create the meeting in the repository
	// TODO: handle rollbacks better
	err := s.MeetingRepository.Create(ctx, reqMeeting.Base, reqMeeting.Settings)
	if err != nil {
		// If repository creation fails and we created a platform meeting, attempt to clean it up
		if reqMeeting.Base.Platform != "" {
			if provider, provErr := s.PlatformRegistry.GetProvider(reqMeeting.Base.Platform); provErr == nil {
				if platformMeetingID := provider.GetPlatformMeetingID(reqMeeting.Base); platformMeetingID != "" {
					if delErr := provider.DeleteMeeting(ctx, platformMeetingID); delErr != nil {
						slog.ErrorContext(ctx, "failed to cleanup platform meeting after repository error",
							"platform", reqMeeting.Base.Platform,
							"platform_meeting_id", platformMeetingID,
							logging.ErrKey, delErr,
							logging.PriorityCritical())
					}
				}
			}
		}
		return nil, domain.ErrInternal
	}

	// Use WorkerPool for concurrent NATS message sending
	pool := concurrent.NewWorkerPool(3) // 3 messages to send

	messages := []func() error{
		func() error {
			return s.MessageBuilder.SendIndexMeeting(ctx, models.ActionCreated, *reqMeeting.Base)
		},
		func() error {
			return s.MessageBuilder.SendIndexMeetingSettings(ctx, models.ActionCreated, *reqMeeting.Settings)
		},
		func() error {
			// For the message we only need the committee UIDs.
			committees := make([]string, len(reqMeeting.Base.Committees))
			for i, committee := range reqMeeting.Base.Committees {
				committees[i] = committee.UID
			}

			return s.MessageBuilder.SendUpdateAccessMeeting(ctx, models.MeetingAccessMessage{
				UID:        reqMeeting.Base.UID,
				Public:     reqMeeting.Base.Visibility == "public",
				ProjectUID: reqMeeting.Base.ProjectUID,
				Organizers: reqMeeting.Settings.Organizers,
				Committees: committees,
			})
		},
	}

	if err := pool.Run(ctx, messages...); err != nil {
		slog.ErrorContext(ctx, "failed to send NATS messages for created meeting", logging.ErrKey, err)
		return nil, domain.ErrInternal
	}

	slog.DebugContext(ctx, "returning created meeting", "meeting_uid", reqMeeting.Base.UID)

	return reqMeeting, nil
}

func (s *MeetingService) GetMeetingBase(ctx context.Context, uid string) (*models.MeetingBase, string, error) {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "service not initialized", logging.PriorityCritical())
		return nil, "", domain.ErrServiceUnavailable
	}

	ctx = logging.AppendCtx(ctx, slog.String("meeting_uid", uid))

	// Get meeting with revision from store
	meetingDB, revision, err := s.MeetingRepository.GetBaseWithRevision(ctx, uid)
	if err != nil {
		if errors.Is(err, domain.ErrMeetingNotFound) {
			slog.WarnContext(ctx, "meeting not found", logging.ErrKey, err)
			return nil, "", domain.ErrMeetingNotFound
		}
		slog.ErrorContext(ctx, "error getting meeting from store", logging.ErrKey, err)
		return nil, "", domain.ErrInternal
	}

	// Store the revision in context for the custom encoder to use
	revisionStr := strconv.FormatUint(revision, 10)
	ctx = context.WithValue(ctx, constants.ETagContextID, revisionStr)

	// Calculate next 50 occurrences from current time
	currentTime := time.Now()
	meetingDB.Occurrences = s.OccurrenceService.CalculateOccurrencesFromDate(meetingDB, currentTime, 50)

	slog.DebugContext(ctx, "returning meeting", "meeting", meetingDB, "revision", revision)

	return meetingDB, revisionStr, nil
}

// GetMeetingSettings fetches settings for a specific meeting by ID
func (s *MeetingService) GetMeetingSettings(ctx context.Context, uid string) (*models.MeetingSettings, string, error) {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "service not initialized", logging.PriorityCritical())
		return nil, "", domain.ErrServiceUnavailable
	}

	ctx = logging.AppendCtx(ctx, slog.String("meeting_uid", uid))

	// Get meeting settings with revision from store
	settingsDB, revision, err := s.MeetingRepository.GetSettingsWithRevision(ctx, uid)
	if err != nil {
		if errors.Is(err, domain.ErrMeetingNotFound) {
			slog.WarnContext(ctx, "meeting settings not found", logging.ErrKey, err)
			return nil, "", domain.ErrMeetingNotFound
		}
		slog.ErrorContext(ctx, "error getting meeting settings from store", logging.ErrKey, err)
		return nil, "", domain.ErrInternal
	}

	// Store the revision in context for the custom encoder to use
	revisionStr := strconv.FormatUint(revision, 10)
	ctx = context.WithValue(ctx, constants.ETagContextID, revisionStr)

	slog.DebugContext(ctx, "returning meeting settings", "settings", settingsDB, "revision", revision)

	return settingsDB, revisionStr, nil
}

// GetMeetingJoinURL fetches the join URL for a specific meeting by ID
func (s *MeetingService) GetMeetingJoinURL(ctx context.Context, uid string) (string, error) {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "service not initialized", logging.PriorityCritical())
		return "", domain.ErrServiceUnavailable
	}

	ctx = logging.AppendCtx(ctx, slog.String("meeting_uid", uid))

	// Get meeting base data from store
	meetingDB, _, err := s.MeetingRepository.GetBaseWithRevision(ctx, uid)
	if err != nil {
		if errors.Is(err, domain.ErrMeetingNotFound) {
			slog.WarnContext(ctx, "meeting not found", logging.ErrKey, err)
			return "", domain.ErrMeetingNotFound
		}
		slog.ErrorContext(ctx, "error getting meeting from store", logging.ErrKey, err)
		return "", domain.ErrInternal
	}

	// Return the join URL
	joinURL := meetingDB.JoinURL
	slog.DebugContext(ctx, "returning join URL", "join_url", joinURL)

	return joinURL, nil
}

func (s *MeetingService) validateUpdateMeetingRequest(ctx context.Context, req *models.MeetingBase) error {
	if req == nil {
		return domain.ErrValidationFailed
	}

	if req.StartTime.Before(time.Now().UTC()) {
		slog.WarnContext(ctx, "start time cannot be in the past", "start_time", req.StartTime)
		return domain.ErrValidationFailed
	}

	return nil
}

// Update a meeting's base information.
func (s *MeetingService) UpdateMeetingBase(ctx context.Context, reqMeeting *models.MeetingBase, revision uint64) (*models.MeetingBase, error) {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "service not initialized", logging.PriorityCritical())
		return nil, domain.ErrServiceUnavailable
	}

	if reqMeeting == nil || reqMeeting.UID == "" {
		slog.WarnContext(ctx, "meeting UID is required")
		return nil, domain.ErrValidationFailed
	}

	var err error
	if s.Config.SkipEtagValidation {
		// If skipping the Etag validation, we need to get the key revision from the store with a Get request.
		_, revision, err = s.MeetingRepository.GetBaseWithRevision(ctx, reqMeeting.UID)
		if err != nil {
			if errors.Is(err, domain.ErrMeetingNotFound) {
				slog.WarnContext(ctx, "meeting not found", logging.ErrKey, err)
				return nil, domain.ErrMeetingNotFound
			}
			slog.ErrorContext(ctx, "error getting meeting from store", logging.ErrKey, err)
			return nil, domain.ErrInternal
		}
	}

	ctx = logging.AppendCtx(ctx, slog.String("meeting_uid", reqMeeting.UID))
	ctx = logging.AppendCtx(ctx, slog.String("etag", strconv.FormatUint(revision, 10)))

	// Check if the meeting exists and use some of the existing meeting data for the update.
	existingMeetingDB, err := s.MeetingRepository.GetBase(ctx, reqMeeting.UID)
	if err != nil {
		if errors.Is(err, domain.ErrMeetingNotFound) {
			slog.WarnContext(ctx, "meeting not found", logging.ErrKey, err)
			return nil, domain.ErrMeetingNotFound
		}
		slog.ErrorContext(ctx, "error checking if meeting exists", logging.ErrKey, err)
		return nil, domain.ErrInternal
	}

	if err := s.validateUpdateMeetingRequest(ctx, reqMeeting); err != nil {
		return nil, err
	}

	reqMeeting = models.MergeUpdateMeetingRequest(reqMeeting, existingMeetingDB)

	// TODO: Check if project exists - integrate with project service
	// TODO: Check if committees exist once the committee service is implemented.

	// Update meeting on external platform if configured
	if reqMeeting.Platform != "" {
		provider, err := s.PlatformRegistry.GetProvider(reqMeeting.Platform)
		if err != nil {
			slog.ErrorContext(ctx, "failed to get platform provider",
				"platform", reqMeeting.Platform,
				logging.ErrKey, err)
			return nil, domain.ErrInternal
		}

		platformMeetingID := provider.GetPlatformMeetingID(reqMeeting)
		slog.DebugContext(ctx, "checking if meeting has platform ID",
			"platform", reqMeeting.Platform,
			"platform_meeting_id", platformMeetingID,
		)

		if platformMeetingID != "" {
			if err := provider.UpdateMeeting(ctx, platformMeetingID, reqMeeting); err != nil {
				slog.ErrorContext(ctx, "failed to update platform meeting",
					"platform", reqMeeting.Platform,
					"platform_meeting_id", platformMeetingID,
					logging.ErrKey, err)
				// Continue with local update even if platform update fails
				// This ensures data consistency - local is source of truth
			} else {
				slog.InfoContext(ctx, "updated platform meeting",
					"platform", reqMeeting.Platform,
					"platform_meeting_id", platformMeetingID)
			}
		}
	}

	// Detect changes before updating
	changes := detectMeetingBaseChanges(existingMeetingDB, reqMeeting)

	// Update the meeting in the repository
	err = s.MeetingRepository.UpdateBase(ctx, reqMeeting, revision)
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

	// Use WorkerPool for concurrent NATS message sending (always 3 messages now)
	pool := concurrent.NewWorkerPool(3)

	messages := []func() error{
		func() error {
			return s.MessageBuilder.SendIndexMeeting(ctx, models.ActionUpdated, *reqMeeting)
		},
		func() error {
			// Get the meeting settings to retrieve organizers
			settingsDB, err := s.MeetingRepository.GetSettings(ctx, reqMeeting.UID)
			if err != nil {
				// If we can't get settings, use empty organizers array rather than failing
				slog.WarnContext(ctx, "could not retrieve meeting settings for access message", logging.ErrKey, err)
				settingsDB = &models.MeetingSettings{Organizers: []string{}}
			}

			// For the message we only need the committee UIDs.
			committees := make([]string, len(reqMeeting.Committees))
			for i, committee := range reqMeeting.Committees {
				committees[i] = committee.UID
			}

			return s.MessageBuilder.SendUpdateAccessMeeting(ctx, models.MeetingAccessMessage{
				UID:        reqMeeting.UID,
				Public:     reqMeeting.Visibility == "public",
				ProjectUID: reqMeeting.ProjectUID,
				Organizers: settingsDB.Organizers,
				Committees: committees,
			})
		},
		func() error {
			// Always send meeting updated message for other services to consume
			return s.MessageBuilder.SendMeetingUpdated(ctx, models.MeetingUpdatedMessage{
				MeetingUID: reqMeeting.UID,
				Changes:    changes,
			})
		},
	}

	if err := pool.Run(ctx, messages...); err != nil {
		slog.ErrorContext(ctx, "failed to send NATS messages for updated meeting", logging.ErrKey, err)
		return nil, domain.ErrInternal
	}

	// Calculate occurrences for the updated meeting
	currentTime := time.Now()
	reqMeeting.Occurrences = s.OccurrenceService.CalculateOccurrencesFromDate(reqMeeting, currentTime, 50)

	slog.DebugContext(ctx, "returning updated meeting", "meeting", reqMeeting)

	return reqMeeting, nil
}

// UpdateMeetingSettings updates a meeting's settings
func (s *MeetingService) UpdateMeetingSettings(ctx context.Context, reqSettings *models.MeetingSettings, revision uint64) (*models.MeetingSettings, error) {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "service not initialized", logging.PriorityCritical())
		return nil, domain.ErrServiceUnavailable
	}

	if reqSettings == nil || reqSettings.UID == "" {
		slog.WarnContext(ctx, "meeting UID is required")
		return nil, domain.ErrValidationFailed
	}

	var err error
	if s.Config.SkipEtagValidation {
		// If skipping the Etag validation, we need to get the key revision from the store with a Get request.
		_, revision, err = s.MeetingRepository.GetSettingsWithRevision(ctx, reqSettings.UID)
		if err != nil {
			if errors.Is(err, domain.ErrMeetingNotFound) {
				slog.WarnContext(ctx, "meeting settings not found", logging.ErrKey, err)
				return nil, domain.ErrMeetingNotFound
			}
			slog.ErrorContext(ctx, "error getting meeting settings from store", logging.ErrKey, err)
			return nil, domain.ErrInternal
		}
	}

	ctx = logging.AppendCtx(ctx, slog.String("meeting_uid", reqSettings.UID))
	ctx = logging.AppendCtx(ctx, slog.String("etag", strconv.FormatUint(revision, 10)))

	// Check if the meeting settings exist and use some of the existing data for the update.
	existingSettingsDB, err := s.MeetingRepository.GetSettings(ctx, reqSettings.UID)
	if err != nil {
		if errors.Is(err, domain.ErrMeetingNotFound) {
			slog.WarnContext(ctx, "meeting settings not found", logging.ErrKey, err)
			return nil, domain.ErrMeetingNotFound
		}
		slog.ErrorContext(ctx, "error checking if meeting settings exist", logging.ErrKey, err)
		return nil, domain.ErrInternal
	}

	reqSettings = models.MergeUpdateMeetingSettingsRequest(reqSettings, existingSettingsDB)

	// Update the meeting settings in the repository
	err = s.MeetingRepository.UpdateSettings(ctx, reqSettings, revision)
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

	// Use WorkerPool for concurrent NATS message sending
	pool := concurrent.NewWorkerPool(2) // 2 messages to send

	messages := []func() error{
		func() error {
			return s.MessageBuilder.SendIndexMeetingSettings(ctx, models.ActionUpdated, *reqSettings)
		},
		func() error {
			// Get the meeting base data to send access update message
			meetingDB, err := s.MeetingRepository.GetBase(ctx, reqSettings.UID)
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
				Organizers: reqSettings.Organizers,
				Committees: committees,
			})
		},
	}

	if err := pool.Run(ctx, messages...); err != nil {
		slog.ErrorContext(ctx, "failed to send NATS messages for updated meeting settings", logging.ErrKey, err)
		return nil, domain.ErrInternal
	}

	slog.DebugContext(ctx, "returning updated meeting settings", "settings", reqSettings)

	return reqSettings, nil
}

// Delete a meeting.
func (s *MeetingService) DeleteMeeting(ctx context.Context, uid string, revision uint64) error {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "service not initialized", logging.PriorityCritical())
		return domain.ErrServiceUnavailable
	}

	var err error
	if s.Config.SkipEtagValidation {
		// If skipping the Etag validation, we need to get the key revision from the store with a Get request.
		_, revision, err = s.MeetingRepository.GetBaseWithRevision(ctx, uid)
		if err != nil {
			if errors.Is(err, domain.ErrMeetingNotFound) {
				slog.WarnContext(ctx, "meeting not found", logging.ErrKey, err)
				return domain.ErrMeetingNotFound
			}
			slog.ErrorContext(ctx, "error getting meeting from store", logging.ErrKey, err)
			return domain.ErrInternal
		}
	}

	ctx = logging.AppendCtx(ctx, slog.String("meeting_uid", uid))
	ctx = logging.AppendCtx(ctx, slog.String("etag", strconv.FormatUint(revision, 10)))

	// Get the meeting to check if it has a Zoom meeting ID
	meetingDB, err := s.MeetingRepository.GetBase(ctx, uid)
	if err != nil {
		if errors.Is(err, domain.ErrMeetingNotFound) {
			slog.WarnContext(ctx, "meeting not found", logging.ErrKey, err)
			return domain.ErrMeetingNotFound
		}
		slog.ErrorContext(ctx, "error getting meeting from store", logging.ErrKey, err)
		return domain.ErrInternal
	}

	// Delete the meeting using the store first
	err = s.MeetingRepository.Delete(ctx, uid, revision)
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

	// Delete meeting from external platform if configured
	// We do this after successfully deleting from repository to ensure consistency
	if meetingDB.Platform != "" {
		provider, err := s.PlatformRegistry.GetProvider(meetingDB.Platform)
		if err != nil {
			slog.ErrorContext(ctx, "failed to get platform provider for deletion",
				"platform", meetingDB.Platform,
				logging.ErrKey, err)
			// Continue anyway - meeting is already deleted from our system
		} else {
			platformMeetingID := provider.GetPlatformMeetingID(meetingDB)
			if platformMeetingID != "" {
				if err := provider.DeleteMeeting(ctx, platformMeetingID); err != nil {
					slog.ErrorContext(ctx, "failed to delete platform meeting",
						"platform", meetingDB.Platform,
						"platform_meeting_id", platformMeetingID,
						logging.ErrKey, err)
					// Continue anyway - meeting is already deleted from our system
				} else {
					slog.InfoContext(ctx, "deleted platform meeting",
						"platform", meetingDB.Platform,
						"platform_meeting_id", platformMeetingID)
				}
			}
		}
	}

	// Send meeting deletion message to trigger registrant cleanup
	err = s.MessageBuilder.SendMeetingDeleted(ctx, models.MeetingDeletedMessage{
		MeetingUID: uid,
	})
	if err != nil {
		slog.ErrorContext(ctx, "error sending meeting deleted message", logging.ErrKey, err, logging.PriorityCritical())
		// Don't return error - this is for internal processing but main deletion already succeeded
	}

	// Use WorkerPool for concurrent NATS message sending
	pool := concurrent.NewWorkerPool(3) // 3 messages to send

	messages := []func() error{
		func() error {
			return s.MessageBuilder.SendDeleteIndexMeeting(ctx, uid)
		},
		func() error {
			return s.MessageBuilder.SendDeleteIndexMeetingSettings(ctx, uid)
		},
		func() error {
			return s.MessageBuilder.SendDeleteAllAccessMeeting(ctx, uid)
		},
	}

	if err := pool.Run(ctx, messages...); err != nil {
		slog.ErrorContext(ctx, "failed to send NATS deletion messages", logging.ErrKey, err)
		return domain.ErrInternal
	}

	slog.DebugContext(ctx, "deleted meeting", "meeting_uid", uid)
	return nil
}
