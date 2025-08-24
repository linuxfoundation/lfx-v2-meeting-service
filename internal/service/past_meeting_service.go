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

func (s *PastMeetingService) validateCreatePastMeetingPayload(ctx context.Context, payload *meetingsvc.CreatePastMeetingPayload) error {
	// Validate that required fields are present
	if payload.MeetingUID == "" {
		return domain.ErrValidationFailed
	}
	if payload.ProjectUID == "" {
		return domain.ErrValidationFailed
	}
	if payload.Title == "" {
		return domain.ErrValidationFailed
	}
	if payload.Description == "" {
		return domain.ErrValidationFailed
	}
	if payload.Platform == "" {
		return domain.ErrValidationFailed
	}

	// Parse and validate timestamps
	scheduledStartTime, err := time.Parse(time.RFC3339, payload.ScheduledStartTime)
	if err != nil {
		slog.WarnContext(ctx, "invalid scheduled start time format", logging.ErrKey, err)
		return domain.ErrValidationFailed
	}

	scheduledEndTime, err := time.Parse(time.RFC3339, payload.ScheduledEndTime)
	if err != nil {
		slog.WarnContext(ctx, "invalid scheduled end time format", logging.ErrKey, err)
		return domain.ErrValidationFailed
	}

	// Validate that end time is after start time
	if scheduledEndTime.Before(scheduledStartTime) {
		slog.WarnContext(ctx, "scheduled end time cannot be before start time")
		return domain.ErrValidationFailed
	}

	return nil
}

func (s *PastMeetingService) CreatePastMeeting(ctx context.Context, payload *meetingsvc.CreatePastMeetingPayload) (*models.PastMeeting, error) {
	// Validate the payload
	if err := s.validateCreatePastMeetingPayload(ctx, payload); err != nil {
		return nil, err
	}

	// Check if the original meeting exists (optional validation)
	exists, err := s.MeetingRepository.Exists(ctx, payload.MeetingUID)
	if err != nil {
		slog.ErrorContext(ctx, "error checking if meeting exists", logging.ErrKey, err)
		return nil, domain.ErrInternal
	}
	if !exists {
		slog.WarnContext(ctx, "referenced meeting does not exist", "meeting_uid", payload.MeetingUID)
		// This is not an error - past meetings can be created for meetings that no longer exist
	}

	// Parse timestamps
	scheduledStartTime, _ := time.Parse(time.RFC3339, payload.ScheduledStartTime)
	scheduledEndTime, _ := time.Parse(time.RFC3339, payload.ScheduledEndTime)

	// Convert committees from DSL to domain models
	var committees []models.Committee
	if payload.Committees != nil {
		for _, committee := range payload.Committees {
			committees = append(committees, models.Committee{
				UID:                   committee.UID,
				AllowedVotingStatuses: committee.AllowedVotingStatuses,
			})
		}
	}

	// Convert recurrence from DSL to domain model
	var recurrence *models.Recurrence
	if payload.Recurrence != nil {
		recurrence = &models.Recurrence{
			Type:           payload.Recurrence.Type,
			RepeatInterval: payload.Recurrence.RepeatInterval,
		}

		if payload.Recurrence.WeeklyDays != nil {
			recurrence.WeeklyDays = *payload.Recurrence.WeeklyDays
		}
		if payload.Recurrence.MonthlyDay != nil {
			recurrence.MonthlyDay = *payload.Recurrence.MonthlyDay
		}
		if payload.Recurrence.MonthlyWeek != nil {
			recurrence.MonthlyWeek = *payload.Recurrence.MonthlyWeek
		}
		if payload.Recurrence.MonthlyWeekDay != nil {
			recurrence.MonthlyWeekDay = *payload.Recurrence.MonthlyWeekDay
		}
		if payload.Recurrence.EndTimes != nil {
			recurrence.EndTimes = *payload.Recurrence.EndTimes
		}
		if payload.Recurrence.EndDateTime != nil {
			endDateTime, err := time.Parse(time.RFC3339, *payload.Recurrence.EndDateTime)
			if err != nil {
				slog.WarnContext(ctx, "invalid recurrence end date time format", logging.ErrKey, err)
				return nil, domain.ErrValidationFailed
			}
			recurrence.EndDateTime = &endDateTime
		}
	}

	// Convert Zoom config from DSL to domain model
	var zoomConfig *models.ZoomConfig
	if payload.ZoomConfig != nil {
		zoomConfig = &models.ZoomConfig{}

		if payload.ZoomConfig.MeetingID != nil {
			zoomConfig.MeetingID = *payload.ZoomConfig.MeetingID
		}
		if payload.ZoomConfig.Passcode != nil {
			zoomConfig.Passcode = *payload.ZoomConfig.Passcode
		}
		if payload.ZoomConfig.AiCompanionEnabled != nil {
			zoomConfig.AICompanionEnabled = *payload.ZoomConfig.AiCompanionEnabled
		}
		if payload.ZoomConfig.AiSummaryRequireApproval != nil {
			zoomConfig.AISummaryRequireApproval = *payload.ZoomConfig.AiSummaryRequireApproval
		}
	}

	// Convert sessions from DSL to domain models
	var sessions []models.Session
	if payload.Sessions != nil {
		for _, session := range payload.Sessions {
			startTime, err := time.Parse(time.RFC3339, session.StartTime)
			if err != nil {
				slog.WarnContext(ctx, "invalid session start time format", logging.ErrKey, err)
				return nil, domain.ErrValidationFailed
			}

			domainSession := models.Session{
				UID:       session.UID,
				StartTime: startTime,
			}

			if session.EndTime != nil {
				endTime, err := time.Parse(time.RFC3339, *session.EndTime)
				if err != nil {
					slog.WarnContext(ctx, "invalid session end time format", logging.ErrKey, err)
					return nil, domain.ErrValidationFailed
				}
				domainSession.EndTime = &endTime
			}

			sessions = append(sessions, domainSession)
		}
	}

	// Create the domain model
	now := time.Now()
	pastMeeting := &models.PastMeeting{
		UID:                  uuid.New().String(),
		MeetingUID:           payload.MeetingUID,
		ProjectUID:           payload.ProjectUID,
		ScheduledStartTime:   scheduledStartTime,
		ScheduledEndTime:     scheduledEndTime,
		Duration:             payload.Duration,
		Timezone:             payload.Timezone,
		Recurrence:           recurrence,
		Title:                payload.Title,
		Description:          payload.Description,
		Committees:           committees,
		Platform:             payload.Platform,
		Restricted:           payload.Restricted,
		RecordingEnabled:     payload.RecordingEnabled,
		TranscriptEnabled:    payload.TranscriptEnabled,
		YoutubeUploadEnabled: payload.YoutubeUploadEnabled,
		ZoomConfig:           zoomConfig,
		Sessions:             sessions,
		CreatedAt:            &now,
		UpdatedAt:            &now,
	}

	// Set optional fields if provided
	if payload.OccurrenceID != nil {
		pastMeeting.OccurrenceID = *payload.OccurrenceID
	}
	if payload.PlatformMeetingID != nil {
		pastMeeting.PlatformMeetingID = *payload.PlatformMeetingID
	}
	if payload.EarlyJoinTimeMinutes != nil {
		pastMeeting.EarlyJoinTimeMinutes = *payload.EarlyJoinTimeMinutes
	}
	if payload.MeetingType != nil {
		pastMeeting.MeetingType = *payload.MeetingType
	}
	if payload.Visibility != nil {
		pastMeeting.Visibility = *payload.Visibility
	}
	if payload.ArtifactVisibility != nil {
		pastMeeting.ArtifactVisibility = *payload.ArtifactVisibility
	}
	if payload.PublicLink != nil {
		pastMeeting.PublicLink = *payload.PublicLink
	}

	// Save to repository
	if err := s.PastMeetingRepository.Create(ctx, pastMeeting); err != nil {
		slog.ErrorContext(ctx, "error creating past meeting", logging.ErrKey, err)
		return nil, domain.ErrInternal
	}

	return pastMeeting, nil
}

func (s *PastMeetingService) GetPastMeetings(ctx context.Context) ([]*models.PastMeeting, error) {
	pastMeetings, err := s.PastMeetingRepository.ListAll(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "error listing past meetings", logging.ErrKey, err)
		return nil, domain.ErrInternal
	}

	return pastMeetings, nil
}

func (s *PastMeetingService) GetPastMeeting(ctx context.Context, uid string) (*models.PastMeeting, string, error) {
	pastMeeting, revision, err := s.PastMeetingRepository.GetWithRevision(ctx, uid)
	if err != nil {
		if errors.Is(err, domain.ErrMeetingNotFound) {
			slog.WarnContext(ctx, "past meeting not found", logging.ErrKey, err)
			return nil, "", domain.ErrMeetingNotFound
		}
		slog.ErrorContext(ctx, "error getting past meeting", logging.ErrKey, err)
		return nil, "", domain.ErrInternal
	}

	return pastMeeting, strconv.FormatUint(revision, 10), nil
}

func (s *PastMeetingService) DeletePastMeeting(ctx context.Context, uid string, revision uint64) error {
	// Check if the past meeting exists
	exists, err := s.PastMeetingRepository.Exists(ctx, uid)
	if err != nil {
		slog.ErrorContext(ctx, "error checking if past meeting exists", logging.ErrKey, err)
		return domain.ErrInternal
	}
	if !exists {
		slog.WarnContext(ctx, "past meeting not found", "uid", uid)
		return domain.ErrMeetingNotFound
	}

	// Delete the past meeting
	if err := s.PastMeetingRepository.Delete(ctx, uid, revision); err != nil {
		if errors.Is(err, domain.ErrMeetingNotFound) {
			slog.WarnContext(ctx, "past meeting not found during deletion", logging.ErrKey, err)
			return domain.ErrMeetingNotFound
		}
		slog.ErrorContext(ctx, "error deleting past meeting", logging.ErrKey, err)
		return domain.ErrInternal
	}

	return nil
}
