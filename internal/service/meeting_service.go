// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/infrastructure/messaging"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/concurrent"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/constants"
)

// MeetingsService implements the meetingsvc.Service interface and domain.MessageHandler
type MeetingService struct {
	meetingRepository    domain.MeetingRepository
	registrantRepository domain.RegistrantRepository
	messageBuilder       domain.MessageBuilder
	platformRegistry     domain.PlatformRegistry
	occurrenceService    domain.OccurrenceService
	emailService         domain.EmailService
	config               ServiceConfig
}

// NewMeetingsService creates a new MeetingsService.
func NewMeetingService(
	meetingRepository domain.MeetingRepository,
	registrantRepository domain.RegistrantRepository,
	messageBuilder domain.MessageBuilder,
	platformRegistry domain.PlatformRegistry,
	occurrenceService domain.OccurrenceService,
	emailService domain.EmailService,
	config ServiceConfig,
) *MeetingService {
	return &MeetingService{
		meetingRepository:    meetingRepository,
		registrantRepository: registrantRepository,
		messageBuilder:       messageBuilder,
		platformRegistry:     platformRegistry,
		occurrenceService:    occurrenceService,
		emailService:         emailService,
		config:               config,
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
	return s.meetingRepository != nil &&
		s.messageBuilder != nil &&
		s.platformRegistry != nil &&
		s.occurrenceService != nil
}

// ListMeetings fetches all meetings
func (s *MeetingService) ListMeetings(ctx context.Context, includeCancelledOccurrences bool) ([]*models.MeetingFull, error) {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "service not initialized", logging.PriorityCritical())
		return nil, domain.NewUnavailableError("meeting service is not ready")
	}

	// Get all meetings from the store
	meetingsBase, meetingSettings, err := s.meetingRepository.ListAll(ctx)
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
			meeting.Occurrences = s.occurrenceService.CalculateOccurrencesFromDate(meeting, currentTime, 50)

			// Filter out cancelled occurrences unless explicitly requested
			if !includeCancelledOccurrences {
				nonCancelledOccurrences := make([]models.Occurrence, 0, len(meeting.Occurrences))
				for _, occ := range meeting.Occurrences {
					if !occ.IsCancelled {
						nonCancelledOccurrences = append(nonCancelledOccurrences, occ)
					}
				}
				meeting.Occurrences = nonCancelledOccurrences
			}
		}
		meetings[i] = &models.MeetingFull{
			Base:     meeting,
			Settings: settings,
		}
	}

	slog.DebugContext(ctx, "returning meetings", "meetings", meetings)

	return meetings, nil
}

// ListMeetingsByCommittee gets all meetings associated with a committee
func (s *MeetingService) ListMeetingsByCommittee(ctx context.Context, committeeUID string) ([]*models.MeetingBase, []*models.MeetingSettings, error) {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "service not initialized", logging.PriorityCritical())
		return nil, nil, domain.NewUnavailableError("meeting service is not ready")
	}

	ctx = logging.AppendCtx(ctx, slog.String("committee_uid", committeeUID))

	// Get meetings from repository
	meetings, settings, err := s.meetingRepository.ListByCommittee(ctx, committeeUID)
	if err != nil {
		slog.ErrorContext(ctx, "error getting meetings by committee", logging.ErrKey, err)
		return nil, nil, err
	}

	// Calculate occurrences for each meeting
	currentTime := time.Now()
	for _, meeting := range meetings {
		if meeting != nil {
			meeting.Occurrences = s.occurrenceService.CalculateOccurrencesFromDate(meeting, currentTime, 50)
		}
	}

	slog.DebugContext(ctx, "returning meetings by committee", "meeting_count", len(meetings))

	return meetings, settings, nil
}

func (s *MeetingService) validateCreateMeetingPayload(ctx context.Context, payload *models.MeetingFull) error {
	if payload == nil || payload.Base == nil {
		return domain.NewValidationError("meeting payload is required")
	}

	if payload.Base.StartTime.Before(time.Now().UTC()) {
		slog.WarnContext(ctx, "start time cannot be in the past", "start_time", payload.Base.StartTime)
		return domain.NewValidationError("meeting start time cannot be in the past")
	}

	return nil
}

func (s *MeetingService) validateCommittees(ctx context.Context, committees []models.Committee) error {
	if len(committees) == 0 {
		return nil
	}

	var invalidCommittees []string

	for _, committee := range committees {
		if committee.UID == "" {
			continue
		}

		_, err := s.messageBuilder.GetCommitteeName(ctx, committee.UID)
		if err != nil {
			var committeNotFoundErr *messaging.CommitteeNotFoundError
			if errors.As(err, &committeNotFoundErr) {
				invalidCommittees = append(invalidCommittees, committee.UID)
			} else {
				slog.ErrorContext(ctx, "error getting committee name", "committee_uid", committee.UID, logging.ErrKey, err)
				return domain.NewInternalError("failed to validate committee", err)
			}
		}
	}

	if len(invalidCommittees) > 0 {
		slog.WarnContext(ctx, "invalid committees provided", "invalid_committee_uids", strings.Join(invalidCommittees, ", "))
		return domain.NewValidationError("one or more committees do not exist: "+strings.Join(invalidCommittees, ", "), nil)
	}

	return nil
}

// validateProject validates that the project exists
func (s *MeetingService) validateProject(ctx context.Context, projectUID string) error {
	if projectUID == "" {
		return nil
	}

	_, err := s.messageBuilder.GetProjectName(ctx, projectUID)
	if err != nil {
		var projectNotFoundErr *messaging.ProjectNotFoundError
		if errors.As(err, &projectNotFoundErr) {
			slog.WarnContext(ctx, "invalid project provided", "project_uid", projectUID)
			return domain.NewValidationError("project does not exist", err)
		} else {
			slog.ErrorContext(ctx, "error getting project name", "project_uid", projectUID, logging.ErrKey, err)
			return domain.NewInternalError("failed to validate project", err)
		}
	}

	return nil
}

// CreateMeeting creates a new meeting
func (s *MeetingService) CreateMeeting(ctx context.Context, reqMeeting *models.MeetingFull) (*models.MeetingFull, error) {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "service not initialized", logging.PriorityCritical())
		return nil, domain.NewUnavailableError("meeting service is not ready")
	}

	if err := s.validateCreateMeetingPayload(ctx, reqMeeting); err != nil {
		return nil, err
	}

	// Validate project exists
	if err := s.validateProject(ctx, reqMeeting.Base.ProjectUID); err != nil {
		return nil, err
	}

	// Validate committees exist
	if err := s.validateCommittees(ctx, reqMeeting.Base.Committees); err != nil {
		return nil, err
	}

	// Generate UID for the meeting
	reqMeeting.Base.UID = uuid.New().String()
	reqMeeting.Settings.UID = reqMeeting.Base.UID

	// Generate password for the meeting
	reqMeeting.Base.Password = uuid.New().String()

	// Create meeting on external platform if configured
	if reqMeeting.Base.Platform != "" {
		provider, err := s.platformRegistry.GetProvider(reqMeeting.Base.Platform)
		if err != nil {
			slog.ErrorContext(ctx, "failed to get platform provider",
				"platform", reqMeeting.Base.Platform,
				logging.ErrKey, err)
			return nil, domain.NewInternalError("failed to initialize meeting platform", err)
		}

		if provider == nil {
			slog.ErrorContext(ctx, "platform provider is nil")
			return nil, domain.NewInternalError("platform provider is nil", nil)
		}

		result, err := provider.CreateMeeting(ctx, reqMeeting.Base)
		if err != nil {
			slog.ErrorContext(ctx, "failed to create platform meeting",
				"platform", reqMeeting.Base.Platform,
				logging.ErrKey, err)
			return nil, domain.NewInternalError("failed to create meeting on external platform", err)
		}

		// Store platform-specific data using the provider
		provider.StorePlatformData(reqMeeting.Base, result)

		slog.InfoContext(ctx, "created platform meeting",
			"platform", reqMeeting.Base.Platform,
			"platform_meeting_id", result.PlatformMeetingID)
	}

	// Calculate first 50 occurrences for the new meeting
	reqMeeting.Base.Occurrences = s.occurrenceService.CalculateOccurrences(reqMeeting.Base, 50)

	// Create the meeting in the repository
	// TODO: handle rollbacks better
	err := s.meetingRepository.Create(ctx, reqMeeting.Base, reqMeeting.Settings)
	if err != nil {
		// If repository creation fails and we created a platform meeting, attempt to clean it up
		if reqMeeting.Base.Platform != "" {
			if provider, provErr := s.platformRegistry.GetProvider(reqMeeting.Base.Platform); provErr == nil && provider != nil {
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
		return nil, err
	}

	// Use WorkerPool for concurrent NATS message sending
	pool := concurrent.NewWorkerPool(4) // 4 messages to send

	messages := []func() error{
		func() error {
			return s.messageBuilder.SendIndexMeeting(ctx, models.ActionCreated, *reqMeeting.Base)
		},
		func() error {
			return s.messageBuilder.SendIndexMeetingSettings(ctx, models.ActionCreated, *reqMeeting.Settings)
		},
		func() error {
			// For the message we only need the committee UIDs.
			committees := make([]string, len(reqMeeting.Base.Committees))
			for i, committee := range reqMeeting.Base.Committees {
				committees[i] = committee.UID
			}

			return s.messageBuilder.SendUpdateAccessMeeting(ctx, models.MeetingAccessMessage{
				UID:        reqMeeting.Base.UID,
				Public:     reqMeeting.Base.IsPublic(),
				ProjectUID: reqMeeting.Base.ProjectUID,
				Organizers: reqMeeting.Settings.Organizers,
				Committees: committees,
			})
		},
		func() error {
			return s.messageBuilder.SendMeetingCreated(ctx, models.MeetingCreatedMessage{
				MeetingUID: reqMeeting.Base.UID,
				Base:       reqMeeting.Base,
				Settings:   reqMeeting.Settings,
			})
		},
	}

	if err := pool.Run(ctx, messages...); err != nil {
		slog.ErrorContext(ctx, "failed to send NATS messages for created meeting", logging.ErrKey, err)
		return nil, domain.NewInternalError("failed to publish meeting creation events", err)
	}

	slog.DebugContext(ctx, "returning created meeting", "meeting_uid", reqMeeting.Base.UID)

	return reqMeeting, nil
}

func (s *MeetingService) GetMeetingBase(ctx context.Context, uid string, includeCancelledOccurrences bool) (*models.MeetingBase, string, error) {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "service not initialized", logging.PriorityCritical())
		return nil, "", domain.NewUnavailableError("meeting service is not ready")
	}

	ctx = logging.AppendCtx(ctx, slog.String("meeting_uid", uid))

	meetingDB, revision, err := s.meetingRepository.GetBaseWithRevision(ctx, uid)
	if err != nil {
		return nil, "", err
	}

	// Store the revision in context for the custom encoder to use
	revisionStr := strconv.FormatUint(revision, 10)
	ctx = context.WithValue(ctx, constants.ETagContextID, revisionStr)

	// Calculate next 50 occurrences from current time
	currentTime := time.Now()
	meetingDB.Occurrences = s.occurrenceService.CalculateOccurrencesFromDate(meetingDB, currentTime, 50)

	// Filter out cancelled occurrences unless explicitly requested
	if !includeCancelledOccurrences {
		nonCancelledOccurrences := make([]models.Occurrence, 0, len(meetingDB.Occurrences))
		for _, occ := range meetingDB.Occurrences {
			if !occ.IsCancelled {
				nonCancelledOccurrences = append(nonCancelledOccurrences, occ)
			}
		}
		meetingDB.Occurrences = nonCancelledOccurrences
	}

	slog.DebugContext(ctx, "returning meeting", "meeting", meetingDB, "revision", revision)

	return meetingDB, revisionStr, nil
}

// GetMeetingByPlatformMeetingID gets a meeting by its platform meeting ID
func (s *MeetingService) GetMeetingByPlatformMeetingID(ctx context.Context, platform, platformMeetingID string) (*models.MeetingBase, error) {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "service not initialized", logging.PriorityCritical())
		return nil, domain.NewUnavailableError("meeting service is not ready")
	}

	ctx = logging.AppendCtx(ctx, slog.String("platform_meeting_id", platformMeetingID))

	switch platform {
	case models.PlatformZoom:
		meeting, err := s.meetingRepository.GetByZoomMeetingID(ctx, platformMeetingID)
		if err != nil {
			slog.ErrorContext(ctx, "failed to find meeting by Zoom meeting ID", logging.ErrKey, err)
			return nil, err
		}

		// Calculate next 50 occurrences from current time
		currentTime := time.Now()
		meeting.Occurrences = s.occurrenceService.CalculateOccurrencesFromDate(meeting, currentTime, 50)

		slog.DebugContext(ctx, "returning meeting by Zoom meeting ID", "meeting_uid", meeting.UID)
		return meeting, nil
	default:
		return nil, domain.NewNotFoundError(fmt.Sprintf("meeting with platform '%s' and meeting ID '%s' not found", platform, platformMeetingID), nil)
	}
}

// GetMeetingSettings fetches settings for a specific meeting by ID
func (s *MeetingService) GetMeetingSettings(ctx context.Context, uid string) (*models.MeetingSettings, string, error) {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "service not initialized", logging.PriorityCritical())
		return nil, "", domain.NewUnavailableError("meeting service is not ready")
	}

	ctx = logging.AppendCtx(ctx, slog.String("meeting_uid", uid))

	settingsDB, revision, err := s.meetingRepository.GetSettingsWithRevision(ctx, uid)
	if err != nil {
		return nil, "", err
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
		return "", domain.NewUnavailableError("meeting service is not ready")
	}

	ctx = logging.AppendCtx(ctx, slog.String("meeting_uid", uid))

	meetingDB, _, err := s.meetingRepository.GetBaseWithRevision(ctx, uid)
	if err != nil {
		return "", err
	}

	return meetingDB.JoinURL, nil
}

func (s *MeetingService) validateUpdateMeetingRequest(ctx context.Context, req *models.MeetingBase, existingMeeting *models.MeetingBase) error {
	if req == nil {
		return domain.NewValidationError("meeting update request is required")
	}

	// Only validate start time is in the future if the start time is being changed
	if existingMeeting != nil && !req.StartTime.Equal(existingMeeting.StartTime) {
		if req.StartTime.Before(time.Now().UTC()) {
			slog.WarnContext(ctx, "start time cannot be in the past", "start_time", req.StartTime)
			return domain.NewValidationError("meeting start time cannot be in the past")
		}
	}

	return nil
}

// Update a meeting's base information.
func (s *MeetingService) UpdateMeetingBase(ctx context.Context, reqMeeting *models.MeetingBase, revision uint64) (*models.MeetingBase, error) {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "service not initialized", logging.PriorityCritical())
		return nil, domain.NewUnavailableError("meeting service is not ready")
	}

	if reqMeeting == nil || reqMeeting.UID == "" {
		slog.WarnContext(ctx, "meeting UID is required")
		return nil, domain.NewValidationError("meeting UID is required for update")
	}

	var err error
	if s.config.SkipEtagValidation {
		// If skipping the Etag validation, we need to get the key revision from the store with a Get request.
		_, revision, err = s.meetingRepository.GetBaseWithRevision(ctx, reqMeeting.UID)
		if err != nil {
			return nil, err
		}
	}

	ctx = logging.AppendCtx(ctx, slog.String("meeting_uid", reqMeeting.UID))
	ctx = logging.AppendCtx(ctx, slog.String("etag", strconv.FormatUint(revision, 10)))

	// Check if the meeting exists and use some of the existing meeting data for the update.
	existingMeetingDB, err := s.meetingRepository.GetBase(ctx, reqMeeting.UID)
	if err != nil {
		return nil, err
	}

	if err := s.validateUpdateMeetingRequest(ctx, reqMeeting, existingMeetingDB); err != nil {
		return nil, err
	}

	reqMeeting = models.MergeUpdateMeetingRequest(reqMeeting, existingMeetingDB)

	// Validate project exists
	if err := s.validateProject(ctx, reqMeeting.ProjectUID); err != nil {
		return nil, err
	}

	// Validate committees exist
	if err := s.validateCommittees(ctx, reqMeeting.Committees); err != nil {
		return nil, err
	}

	// Update meeting on external platform if configured
	if reqMeeting.Platform != "" {
		provider, err := s.platformRegistry.GetProvider(reqMeeting.Platform)
		if err != nil {
			slog.ErrorContext(ctx, "failed to get platform provider",
				"platform", reqMeeting.Platform,
				logging.ErrKey, err)
			return nil, domain.NewInternalError("failed to initialize meeting platform", err)
		}

		if provider == nil {
			slog.ErrorContext(ctx, "platform provider is nil")
			return nil, domain.NewInternalError("platform provider is nil", nil)
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

	// Increment ICS sequence for calendar updates
	reqMeeting.IcsSequence = existingMeetingDB.IcsSequence + 1

	// Detect changes before updating
	changes := detectMeetingBaseChanges(existingMeetingDB, reqMeeting)

	err = s.meetingRepository.UpdateBase(ctx, reqMeeting, revision)
	if err != nil {
		return nil, err
	}

	// Calculate occurrences for the updated meeting before sending NATS messages
	// This ensures the indexer receives the meeting with updated occurrences
	currentTime := time.Now()
	reqMeeting.Occurrences = s.occurrenceService.CalculateOccurrencesFromDate(reqMeeting, currentTime, 50)

	// Get the meeting settings to retrieve organizers for the updated message
	settingsDB, err := s.meetingRepository.GetSettings(ctx, reqMeeting.UID)
	if err != nil {
		// If we can't get settings, use empty organizers array rather than failing
		slog.WarnContext(ctx, "could not retrieve meeting settings for messages", logging.ErrKey, err)
		settingsDB = &models.MeetingSettings{Organizers: []string{}}
	}

	// Use WorkerPool for concurrent NATS message sending
	pool := concurrent.NewWorkerPool(3)

	messages := []func() error{
		func() error {
			return s.messageBuilder.SendIndexMeeting(ctx, models.ActionUpdated, *reqMeeting)
		},
		func() error {
			// For the message we only need the committee UIDs.
			committees := make([]string, len(reqMeeting.Committees))
			for i, committee := range reqMeeting.Committees {
				committees[i] = committee.UID
			}

			return s.messageBuilder.SendUpdateAccessMeeting(ctx, models.MeetingAccessMessage{
				UID:        reqMeeting.UID,
				Public:     reqMeeting.IsPublic(),
				ProjectUID: reqMeeting.ProjectUID,
				Organizers: settingsDB.Organizers,
				Committees: committees,
			})
		},
		func() error {
			return s.messageBuilder.SendMeetingUpdated(ctx, models.MeetingUpdatedMessage{
				MeetingUID:   reqMeeting.UID,
				UpdatedBase:  reqMeeting,
				PreviousBase: existingMeetingDB,
				Settings:     settingsDB,
				Changes:      changes,
			})
		},
	}

	if err := pool.Run(ctx, messages...); err != nil {
		slog.ErrorContext(ctx, "failed to send NATS messages for updated meeting", logging.ErrKey, err)
		return nil, domain.NewInternalError("failed to publish meeting update events", err)
	}

	slog.DebugContext(ctx, "returning updated meeting", "meeting", reqMeeting)

	return reqMeeting, nil
}

// UpdateMeetingSettings updates a meeting's settings
func (s *MeetingService) UpdateMeetingSettings(ctx context.Context, reqSettings *models.MeetingSettings, revision uint64) (*models.MeetingSettings, error) {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "service not initialized", logging.PriorityCritical())
		return nil, domain.NewUnavailableError("meeting service is not ready")
	}

	if reqSettings == nil || reqSettings.UID == "" {
		slog.WarnContext(ctx, "meeting UID is required")
		return nil, domain.NewValidationError("meeting UID is required for update")
	}

	var err error
	if s.config.SkipEtagValidation {
		// If skipping the Etag validation, we need to get the key revision from the store with a Get request.
		_, revision, err = s.meetingRepository.GetSettingsWithRevision(ctx, reqSettings.UID)
		if err != nil {
			return nil, err
		}
	}

	ctx = logging.AppendCtx(ctx, slog.String("meeting_uid", reqSettings.UID))
	ctx = logging.AppendCtx(ctx, slog.String("etag", strconv.FormatUint(revision, 10)))

	// Check if the meeting settings exist and use some of the existing data for the update.
	existingSettingsDB, err := s.meetingRepository.GetSettings(ctx, reqSettings.UID)
	if err != nil {
		return nil, err
	}

	reqSettings = models.MergeUpdateMeetingSettingsRequest(reqSettings, existingSettingsDB)

	// Update the meeting settings in the repository
	err = s.meetingRepository.UpdateSettings(ctx, reqSettings, revision)
	if err != nil {
		return nil, err
	}

	// Use WorkerPool for concurrent NATS message sending
	pool := concurrent.NewWorkerPool(2) // 2 messages to send

	messages := []func() error{
		func() error {
			return s.messageBuilder.SendIndexMeetingSettings(ctx, models.ActionUpdated, *reqSettings)
		},
		func() error {
			// Get the meeting base data to send access update message
			meetingDB, err := s.meetingRepository.GetBase(ctx, reqSettings.UID)
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

			return s.messageBuilder.SendUpdateAccessMeeting(ctx, models.MeetingAccessMessage{
				UID:        meetingDB.UID,
				Public:     meetingDB.IsPublic(),
				ProjectUID: meetingDB.ProjectUID,
				Organizers: reqSettings.Organizers,
				Committees: committees,
			})
		},
	}

	if err := pool.Run(ctx, messages...); err != nil {
		slog.ErrorContext(ctx, "failed to send NATS messages for updated meeting settings", logging.ErrKey, err)
		return nil, domain.NewInternalError("failed to publish meeting settings update events", err)
	}

	slog.DebugContext(ctx, "returning updated meeting settings", "settings", reqSettings)

	return reqSettings, nil
}

// Delete a meeting.
func (s *MeetingService) DeleteMeeting(ctx context.Context, uid string, revision uint64) error {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "service not initialized", logging.PriorityCritical())
		return domain.NewUnavailableError("meeting service is not ready")
	}

	var err error
	if s.config.SkipEtagValidation {
		// If skipping the Etag validation, we need to get the key revision from the store with a Get request.
		_, revision, err = s.meetingRepository.GetBaseWithRevision(ctx, uid)
		if err != nil {
			return err
		}
	}

	ctx = logging.AppendCtx(ctx, slog.String("meeting_uid", uid))
	ctx = logging.AppendCtx(ctx, slog.String("etag", strconv.FormatUint(revision, 10)))

	// Get the meeting to check if it has a Zoom meeting ID
	meetingDB, err := s.meetingRepository.GetBase(ctx, uid)
	if err != nil {
		return err
	}

	// Delete the meeting using the store first
	err = s.meetingRepository.Delete(ctx, uid, revision)
	if err != nil {
		return err
	}

	// Delete meeting from external platform if configured
	// We do this after successfully deleting from repository to ensure consistency
	if meetingDB.Platform != "" {
		provider, err := s.platformRegistry.GetProvider(meetingDB.Platform)
		if err != nil {
			slog.ErrorContext(ctx, "failed to get platform provider for deletion",
				"platform", meetingDB.Platform,
				logging.ErrKey, err)
			// Continue anyway - meeting is already deleted from our system
		} else if provider != nil {
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
	err = s.messageBuilder.SendMeetingDeleted(ctx, models.MeetingDeletedMessage{
		MeetingUID: uid,
		Meeting:    meetingDB,
	})
	if err != nil {
		slog.ErrorContext(ctx, "error sending meeting deleted message", logging.ErrKey, err, logging.PriorityCritical())
		// Don't return error - this is for internal processing but main deletion already succeeded
	}

	// Use WorkerPool for concurrent NATS message sending
	pool := concurrent.NewWorkerPool(3)

	messages := []func() error{
		func() error {
			return s.messageBuilder.SendDeleteIndexMeeting(ctx, uid)
		},
		func() error {
			return s.messageBuilder.SendDeleteIndexMeetingSettings(ctx, uid)
		},
		func() error {
			return s.messageBuilder.SendDeleteAllAccessMeeting(ctx, uid)
		},
	}

	if err := pool.Run(ctx, messages...); err != nil {
		slog.ErrorContext(ctx, "failed to send NATS deletion messages", logging.ErrKey, err)
		return domain.NewInternalError("failed to publish meeting deletion events", err)
	}

	slog.DebugContext(ctx, "deleted meeting", "meeting_uid", uid)
	return nil
}

// CancelMeetingOccurrence cancels a specific occurrence of a meeting by setting its IsCancelled field to true
func (s *MeetingService) CancelMeetingOccurrence(ctx context.Context, meetingUID string, occurrenceID string, revision uint64) error {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "service not initialized", logging.PriorityCritical())
		return domain.NewUnavailableError("meeting service is not ready")
	}

	if meetingUID == "" {
		slog.WarnContext(ctx, "meeting UID is required")
		return domain.NewValidationError("meeting UID is required")
	}

	if occurrenceID == "" {
		slog.WarnContext(ctx, "occurrence ID is required")
		return domain.NewValidationError("occurrence ID is required")
	}

	var err error
	if s.config.SkipEtagValidation {
		// If skipping the Etag validation, we need to get the key revision from the store with a Get request.
		_, revision, err = s.meetingRepository.GetBaseWithRevision(ctx, meetingUID)
		if err != nil {
			return err
		}
	}

	ctx = logging.AppendCtx(ctx, slog.String("meeting_uid", meetingUID))
	ctx = logging.AppendCtx(ctx, slog.String("occurrence_id", occurrenceID))
	ctx = logging.AppendCtx(ctx, slog.String("etag", strconv.FormatUint(revision, 10)))

	// Get the meeting from repository
	meetingDB, err := s.meetingRepository.GetBase(ctx, meetingUID)
	if err != nil {
		return err
	}

	// Calculate current occurrences
	// We calculate up to 50 occurrences which should be sufficient for most use cases
	currentTime := time.Now()
	calculatedOccurrences := s.occurrenceService.CalculateOccurrencesFromDate(meetingDB, currentTime, 50)

	// Find the occurrence and check if it's already cancelled
	var cancelledOccurrence *models.Occurrence
	for i, occ := range calculatedOccurrences {
		if occ.OccurrenceID == occurrenceID {
			// Check if the occurrence is already cancelled
			if occ.IsCancelled {
				slog.WarnContext(ctx, "occurrence is already cancelled", "meeting_uid", meetingUID, "occurrence_id", occurrenceID)
				return domain.NewConflictError("occurrence is already cancelled")
			}
			// Capture the occurrence for email notifications
			cancelledOccurrence = &occ
			// Mark the occurrence as cancelled
			calculatedOccurrences[i].IsCancelled = true
			slog.InfoContext(ctx, "marked occurrence as cancelled", "occurrence_id", occurrenceID)
			break
		}
	}

	if cancelledOccurrence == nil {
		slog.WarnContext(ctx, "occurrence not found for meeting", "meeting_uid", meetingUID, "occurrence_id", occurrenceID)
		return domain.NewNotFoundError("occurrence not found for this meeting")
	}

	// Replace the entire occurrences array with the calculated occurrences (including the cancelled one)
	meetingDB.Occurrences = calculatedOccurrences

	// Update the meeting in the repository
	err = s.meetingRepository.UpdateBase(ctx, meetingDB, revision)
	if err != nil {
		slog.ErrorContext(ctx, "failed to update meeting in repository", logging.ErrKey, err)
		return err
	}

	// Send update to indexer
	err = s.messageBuilder.SendIndexMeeting(ctx, models.ActionUpdated, *meetingDB)
	if err != nil {
		slog.ErrorContext(ctx, "failed to send NATS message for cancelled occurrence", logging.ErrKey, err)
		return domain.NewInternalError("failed to publish occurrence cancellation event", err)
	}

	// Send email notifications to all registrants (async operations that don't fail the cancellation)
	registrants, err := s.registrantRepository.ListByMeeting(ctx, meetingUID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to fetch registrants for occurrence cancellation emails", logging.ErrKey, err)
		// Don't fail the operation if we can't fetch registrants
	}

	if len(registrants) == 0 {
		slog.InfoContext(ctx, "successfully cancelled meeting occurrence", "meeting_uid", meetingUID, "occurrence_id", occurrenceID)
		return nil
	}

	// Get project information for email branding
	projectName, _ := s.messageBuilder.GetProjectName(ctx, meetingDB.ProjectUID)
	projectLogo, _ := s.messageBuilder.GetProjectLogo(ctx, meetingDB.ProjectUID)
	projectSlug, _ := s.messageBuilder.GetProjectSlug(ctx, meetingDB.ProjectUID)

	// Build functions to send emails to each registrant
	var emailFunctions []func() error
	for _, registrant := range registrants {
		// Capture loop variables
		r := registrant
		occ := *cancelledOccurrence

		emailFunctions = append(emailFunctions, func() error {
			emailCtx := logging.AppendCtx(ctx, slog.String("registrant_uid", r.UID))
			emailCtx = logging.AppendCtx(emailCtx, slog.String("registrant_email", r.Email))

			// Get the occurrence start time
			occurrenceStartTime := meetingDB.StartTime
			if occ.StartTime != nil {
				occurrenceStartTime = *occ.StartTime
			}

			recipientName := r.GetFullName()

			cancellation := domain.EmailOccurrenceCancellation{
				MeetingUID:          meetingDB.UID,
				RecipientEmail:      r.Email,
				RecipientName:       recipientName,
				MeetingTitle:        meetingDB.Title,
				OccurrenceID:        occ.OccurrenceID,
				OccurrenceStartTime: occurrenceStartTime,
				Duration:            meetingDB.Duration,
				Timezone:            meetingDB.Timezone,
				Description:         meetingDB.Description,
				Visibility:          meetingDB.Visibility,
				MeetingType:         meetingDB.MeetingType,
				Platform:            meetingDB.Platform,
				MeetingDetailsLink:  constants.GenerateLFXMeetingDetailsURL(projectSlug, meetingDB.UID, s.config.LFXEnvironment),
				ProjectName:         projectName,
				ProjectLogo:         projectLogo,
				Reason:              "This specific occurrence of the recurring meeting has been cancelled by an organizer.",
				Recurrence:          meetingDB.Recurrence,
				IcsSequence:         meetingDB.IcsSequence,
			}

			err := s.emailService.SendOccurrenceCancellation(emailCtx, cancellation)
			if err != nil {
				slog.ErrorContext(emailCtx, "failed to send occurrence cancellation email", logging.ErrKey, err)
			}
			return nil // Don't propagate email errors as they shouldn't fail the operation
		})
	}

	// Execute all email functions concurrently using WorkerPool
	pool := concurrent.NewWorkerPool(5) // Use 5 workers for email operations
	err = pool.Run(ctx, emailFunctions...)
	if err != nil {
		// Log the error but don't fail the operation since email errors are non-critical
		slog.WarnContext(ctx, "some email operations failed during occurrence cancellation", logging.ErrKey, err)
	}

	slog.InfoContext(ctx, "successfully cancelled meeting occurrence", "meeting_uid", meetingUID, "occurrence_id", occurrenceID)
	return nil
}
