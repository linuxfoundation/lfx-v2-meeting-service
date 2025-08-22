// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/concurrent"
)

// ZoomPayloadForPastMeeting contains the essential Zoom webhook data for creating PastMeeting records
type ZoomPayloadForPastMeeting struct {
	UUID      string
	StartTime time.Time
	EndTime   *time.Time // nil for meeting.started, set for meeting.ended
	Timezone  string
}

// parseZoomWebhookEvent is a helper to parse webhook event messages
func (s *MeetingsService) parseZoomWebhookEvent(ctx context.Context, msg domain.Message) (*models.ZoomWebhookEventMessage, error) {
	var webhookEvent models.ZoomWebhookEventMessage
	if err := json.Unmarshal(msg.Data(), &webhookEvent); err != nil {
		slog.ErrorContext(ctx, "failed to unmarshal Zoom webhook event", logging.ErrKey, err)
		return nil, err
	}
	return &webhookEvent, nil
}

// HandleZoomMeetingStarted handles meeting.started webhook events
func (s *MeetingsService) HandleZoomMeetingStarted(ctx context.Context, msg domain.Message) ([]byte, error) {
	webhookEvent, err := s.parseZoomWebhookEvent(ctx, msg)
	if err != nil {
		return nil, err
	}

	ctx = logging.AppendCtx(ctx, slog.String("event_type", webhookEvent.EventType))
	err = s.handleMeetingStartedEvent(ctx, *webhookEvent)
	if err != nil {
		slog.ErrorContext(ctx, "failed to handle meeting started event", logging.ErrKey, err)
		return nil, err
	}

	slog.InfoContext(ctx, "successfully processed meeting started event")
	return nil, nil // No response needed for webhook events
}

// HandleZoomMeetingEnded handles meeting.ended webhook events
func (s *MeetingsService) HandleZoomMeetingEnded(ctx context.Context, msg domain.Message) ([]byte, error) {
	webhookEvent, err := s.parseZoomWebhookEvent(ctx, msg)
	if err != nil {
		return nil, err
	}

	ctx = logging.AppendCtx(ctx, slog.String("event_type", webhookEvent.EventType))
	err = s.handleMeetingEndedEvent(ctx, *webhookEvent)
	if err != nil {
		slog.ErrorContext(ctx, "failed to handle meeting ended event", logging.ErrKey, err)
		return nil, err
	}

	slog.InfoContext(ctx, "successfully processed meeting ended event")
	return nil, nil // No response needed for webhook events
}

// HandleZoomMeetingDeleted handles meeting.deleted webhook events
func (s *MeetingsService) HandleZoomMeetingDeleted(ctx context.Context, msg domain.Message) ([]byte, error) {
	webhookEvent, err := s.parseZoomWebhookEvent(ctx, msg)
	if err != nil {
		return nil, err
	}

	ctx = logging.AppendCtx(ctx, slog.String("event_type", webhookEvent.EventType))
	err = s.handleMeetingDeletedEvent(ctx, *webhookEvent)
	if err != nil {
		slog.ErrorContext(ctx, "failed to handle meeting deleted event", logging.ErrKey, err)
		return nil, err
	}

	slog.InfoContext(ctx, "successfully processed meeting deleted event")
	return nil, nil // No response needed for webhook events
}

// HandleZoomParticipantJoined handles meeting.participant_joined webhook events
func (s *MeetingsService) HandleZoomParticipantJoined(ctx context.Context, msg domain.Message) ([]byte, error) {
	webhookEvent, err := s.parseZoomWebhookEvent(ctx, msg)
	if err != nil {
		return nil, err
	}

	ctx = logging.AppendCtx(ctx, slog.String("event_type", webhookEvent.EventType))
	err = s.handleParticipantJoinedEvent(ctx, *webhookEvent)
	if err != nil {
		slog.ErrorContext(ctx, "failed to handle participant joined event", logging.ErrKey, err)
		return nil, err
	}

	slog.InfoContext(ctx, "successfully processed participant joined event")
	return nil, nil // No response needed for webhook events
}

// HandleZoomParticipantLeft handles meeting.participant_left webhook events
func (s *MeetingsService) HandleZoomParticipantLeft(ctx context.Context, msg domain.Message) ([]byte, error) {
	webhookEvent, err := s.parseZoomWebhookEvent(ctx, msg)
	if err != nil {
		return nil, err
	}

	ctx = logging.AppendCtx(ctx, slog.String("event_type", webhookEvent.EventType))
	err = s.handleParticipantLeftEvent(ctx, *webhookEvent)
	if err != nil {
		slog.ErrorContext(ctx, "failed to handle participant left event", logging.ErrKey, err)
		return nil, err
	}

	slog.InfoContext(ctx, "successfully processed participant left event")
	return nil, nil // No response needed for webhook events
}

// HandleZoomRecordingCompleted handles recording.completed webhook events
func (s *MeetingsService) HandleZoomRecordingCompleted(ctx context.Context, msg domain.Message) ([]byte, error) {
	webhookEvent, err := s.parseZoomWebhookEvent(ctx, msg)
	if err != nil {
		return nil, err
	}

	ctx = logging.AppendCtx(ctx, slog.String("event_type", webhookEvent.EventType))
	err = s.handleRecordingCompletedEvent(ctx, *webhookEvent)
	if err != nil {
		slog.ErrorContext(ctx, "failed to handle recording completed event", logging.ErrKey, err)
		return nil, err
	}

	slog.InfoContext(ctx, "successfully processed recording completed event")
	return nil, nil // No response needed for webhook events
}

// HandleZoomTranscriptCompleted handles recording.transcript_completed webhook events
func (s *MeetingsService) HandleZoomTranscriptCompleted(ctx context.Context, msg domain.Message) ([]byte, error) {
	webhookEvent, err := s.parseZoomWebhookEvent(ctx, msg)
	if err != nil {
		return nil, err
	}

	ctx = logging.AppendCtx(ctx, slog.String("event_type", webhookEvent.EventType))
	err = s.handleTranscriptCompletedEvent(ctx, *webhookEvent)
	if err != nil {
		slog.ErrorContext(ctx, "failed to handle transcript completed event", logging.ErrKey, err)
		return nil, err
	}

	slog.InfoContext(ctx, "successfully processed transcript completed event")
	return nil, nil // No response needed for webhook events
}

// HandleZoomSummaryCompleted handles meeting.summary_completed webhook events
func (s *MeetingsService) HandleZoomSummaryCompleted(ctx context.Context, msg domain.Message) ([]byte, error) {
	webhookEvent, err := s.parseZoomWebhookEvent(ctx, msg)
	if err != nil {
		return nil, err
	}

	ctx = logging.AppendCtx(ctx, slog.String("event_type", webhookEvent.EventType))
	err = s.handleSummaryCompletedEvent(ctx, *webhookEvent)
	if err != nil {
		slog.ErrorContext(ctx, "failed to handle summary completed event", logging.ErrKey, err)
		return nil, err
	}

	slog.InfoContext(ctx, "successfully processed summary completed event")
	return nil, nil // No response needed for webhook events
}

// handleMeetingStartedEvent processes meeting.started events
func (s *MeetingsService) handleMeetingStartedEvent(ctx context.Context, event models.ZoomWebhookEventMessage) error {
	slog.InfoContext(ctx, "processing meeting started event")

	// Convert to typed payload
	payload, err := event.ToMeetingStartedPayload()
	if err != nil {
		slog.ErrorContext(ctx, "failed to parse meeting started payload", logging.ErrKey, err)
		return fmt.Errorf("invalid meeting.started payload: %w", err)
	}

	meetingObj := payload.Object

	slog.InfoContext(ctx, "meeting started",
		"zoom_meeting_id", meetingObj.ID,
		"zoom_meeting_uuid", meetingObj.UUID,
		"topic", meetingObj.Topic,
		"start_time", meetingObj.StartTime,
		"host_id", meetingObj.HostID,
	)

	// Find the meeting by Zoom meeting ID
	meeting, err := s.MeetingRepository.GetByZoomMeetingID(ctx, meetingObj.ID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to find meeting by Zoom ID", logging.ErrKey, err)
		return fmt.Errorf("meeting not found for Zoom ID %s: %w", meetingObj.ID, err)
	}

	// Create the past meeting record
	zoomData := ZoomPayloadForPastMeeting{
		UUID:      meetingObj.UUID,
		StartTime: meetingObj.StartTime,
		EndTime:   nil, // meeting.started events don't have end time
		Timezone:  meetingObj.Timezone,
	}
	pastMeeting, err := s.createPastMeetingRecord(ctx, meeting, zoomData)
	if err != nil {
		return fmt.Errorf("failed to create past meeting record: %w", err)
	}

	// Create participant records for all registrants
	err = s.createPastMeetingParticipants(ctx, pastMeeting, meeting)
	if err != nil {
		// Log the error but don't fail the entire webhook processing
		slog.ErrorContext(ctx, "failed to create past meeting participants", logging.ErrKey, err)
	}

	return nil
}

// handleMeetingEndedEvent processes meeting.ended events
func (s *MeetingsService) handleMeetingEndedEvent(ctx context.Context, event models.ZoomWebhookEventMessage) error {
	// Convert to typed payload
	payload, err := event.ToMeetingEndedPayload()
	if err != nil {
		slog.ErrorContext(ctx, "failed to convert to typed meeting ended payload", logging.ErrKey, err)
		return fmt.Errorf("failed to parse meeting ended payload: %w", err)
	}

	meetingObj := payload.Object
	slog.InfoContext(ctx, "processing meeting ended event",
		"zoom_meeting_uuid", meetingObj.UUID,
		"zoom_meeting_id", meetingObj.ID,
		"topic", meetingObj.Topic,
		"start_time", meetingObj.StartTime,
		"end_time", meetingObj.EndTime,
		"duration", meetingObj.Duration,
		"timezone", meetingObj.Timezone,
	)

	// Try to find existing PastMeeting record by platform meeting ID
	existingPastMeeting, err := s.PastMeetingRepository.GetByPlatformMeetingID(ctx, "Zoom", meetingObj.ID)
	if err != nil && err != domain.ErrMeetingNotFound {
		slog.ErrorContext(ctx, "error searching for existing past meeting", logging.ErrKey, err)
		return fmt.Errorf("failed to search for existing past meeting: %w", err)
	}

	if existingPastMeeting != nil {
		// Primary flow: Update existing PastMeeting session with end time
		err = s.updatePastMeetingSessionEndTime(ctx, existingPastMeeting, meetingObj.UUID, meetingObj.StartTime, meetingObj.EndTime)
		if err != nil {
			return fmt.Errorf("failed to update existing past meeting session: %w", err)
		}
	} else {
		// Fallback flow: Create PastMeeting since meeting.started might have been missed
		slog.WarnContext(ctx, "no existing past meeting found for ended event, creating as fallback",
			"zoom_meeting_id", meetingObj.ID,
			"zoom_meeting_uuid", meetingObj.UUID,
		)

		// Find the meeting by Zoom meeting ID first
		meeting, err := s.MeetingRepository.GetByZoomMeetingID(ctx, meetingObj.ID)
		if err != nil {
			slog.ErrorContext(ctx, "failed to find meeting by Zoom ID for fallback creation", logging.ErrKey, err)
			return fmt.Errorf("meeting not found for Zoom ID %s: %w", meetingObj.ID, err)
		}

		// Create PastMeeting with complete session (start and end times)
		zoomData := ZoomPayloadForPastMeeting{
			UUID:      meetingObj.UUID,
			StartTime: meetingObj.StartTime,
			EndTime:   &meetingObj.EndTime, // meeting.ended events have end time
			Timezone:  meetingObj.Timezone,
		}
		pastMeeting, err := s.createPastMeetingRecordForEndedEvent(ctx, meeting, zoomData)
		if err != nil {
			return fmt.Errorf("failed to create past meeting record for ended event: %w", err)
		}

		// Create participant records for all registrants
		err = s.createPastMeetingParticipants(ctx, pastMeeting, meeting)
		if err != nil {
			// Log the error but don't fail the entire webhook processing
			slog.ErrorContext(ctx, "failed to create past meeting participants for ended event", logging.ErrKey, err)
		}

		slog.InfoContext(ctx, "successfully created past meeting record from ended event",
			"past_meeting_uid", pastMeeting.UID,
			"meeting_uid", meeting.UID,
		)
	}

	return nil
}

// handleMeetingDeletedEvent processes meeting.deleted events
func (s *MeetingsService) handleMeetingDeletedEvent(ctx context.Context, event models.ZoomWebhookEventMessage) error {
	// Convert to typed payload
	payload, err := event.ToMeetingDeletedPayload()
	if err != nil {
		slog.ErrorContext(ctx, "failed to convert to typed meeting deleted payload", "error", err)
		return fmt.Errorf("failed to parse meeting deleted payload: %w", err)
	}

	slog.InfoContext(ctx, "processing meeting deleted event",
		"zoom_meeting_uuid", payload.Object.UUID,
		"zoom_meeting_id", payload.Object.ID,
		"topic", payload.Object.Topic,
	)

	return nil
}

// handleParticipantJoinedEvent processes meeting.participant_joined events
func (s *MeetingsService) handleParticipantJoinedEvent(ctx context.Context, event models.ZoomWebhookEventMessage) error {
	// Convert to typed payload
	payload, err := event.ToParticipantJoinedPayload()
	if err != nil {
		slog.ErrorContext(ctx, "failed to convert to typed participant joined payload", "error", err)
		return fmt.Errorf("failed to parse participant joined payload: %w", err)
	}

	slog.InfoContext(ctx, "processing participant joined event",
		"zoom_meeting_uuid", payload.Object.UUID,
		"zoom_meeting_id", payload.Object.ID,
		"participant_id", payload.Object.Participant.ID,
		"participant_name", payload.Object.Participant.UserName,
		"participant_email", payload.Object.Participant.Email,
		"join_time", payload.Object.Participant.JoinTime,
	)

	return nil
}

// handleParticipantLeftEvent processes meeting.participant_left events
func (s *MeetingsService) handleParticipantLeftEvent(ctx context.Context, event models.ZoomWebhookEventMessage) error {
	// Convert to typed payload
	payload, err := event.ToParticipantLeftPayload()
	if err != nil {
		slog.ErrorContext(ctx, "failed to convert to typed participant left payload", "error", err)
		return fmt.Errorf("failed to parse participant left payload: %w", err)
	}

	slog.InfoContext(ctx, "processing participant left event",
		"zoom_meeting_uuid", payload.Object.UUID,
		"zoom_meeting_id", payload.Object.ID,
		"participant_id", payload.Object.Participant.ID,
		"participant_name", payload.Object.Participant.UserName,
		"participant_email", payload.Object.Participant.Email,
		"leave_time", payload.Object.Participant.LeaveTime,
		"duration", payload.Object.Participant.Duration,
	)

	return nil
}

// handleRecordingCompletedEvent processes recording.completed events
func (s *MeetingsService) handleRecordingCompletedEvent(ctx context.Context, event models.ZoomWebhookEventMessage) error {
	// Convert to typed payload
	payload, err := event.ToRecordingCompletedPayload()
	if err != nil {
		slog.ErrorContext(ctx, "failed to convert to typed recording completed payload", "error", err)
		return fmt.Errorf("failed to parse recording completed payload: %w", err)
	}

	slog.InfoContext(ctx, "processing recording completed event",
		"zoom_meeting_uuid", payload.Object.UUID,
		"zoom_meeting_id", payload.Object.ID,
		"topic", payload.Object.Topic,
		"total_size", payload.Object.TotalSize,
		"recording_count", payload.Object.RecordingCount,
	)

	return nil
}

// handleTranscriptCompletedEvent processes recording.transcript_completed events
func (s *MeetingsService) handleTranscriptCompletedEvent(ctx context.Context, event models.ZoomWebhookEventMessage) error {
	// Convert to typed payload
	payload, err := event.ToTranscriptCompletedPayload()
	if err != nil {
		slog.ErrorContext(ctx, "failed to convert to typed transcript completed payload", "error", err)
		return fmt.Errorf("failed to parse transcript completed payload: %w", err)
	}

	slog.InfoContext(ctx, "processing transcript completed event",
		"zoom_meeting_uuid", payload.Object.UUID,
		"zoom_meeting_id", payload.Object.ID,
		"topic", payload.Object.Topic,
		"duration", payload.Object.Duration,
	)

	return nil
}

// handleSummaryCompletedEvent processes meeting.summary_completed events
func (s *MeetingsService) handleSummaryCompletedEvent(ctx context.Context, event models.ZoomWebhookEventMessage) error {
	// Convert to typed payload
	payload, err := event.ToSummaryCompletedPayload()
	if err != nil {
		slog.ErrorContext(ctx, "failed to convert to typed summary completed payload", "error", err)
		return fmt.Errorf("failed to parse summary completed payload: %w", err)
	}

	slog.InfoContext(ctx, "processing summary completed event",
		"zoom_meeting_uuid", payload.Object.UUID,
		"zoom_meeting_id", payload.Object.ID,
		"topic", payload.Object.Topic,
		"start_time", payload.Object.StartTime,
		"end_time", payload.Object.EndTime,
		"duration", payload.Object.Duration,
	)

	return nil
}

// createPastMeetingRecord creates a historical record for a meeting that has started
func (s *MeetingsService) createPastMeetingRecord(ctx context.Context, meeting *models.MeetingBase, zoomData ZoomPayloadForPastMeeting) (*models.PastMeeting, error) {
	return s.createPastMeetingRecordWithSession(ctx, meeting, zoomData)
}

// createPastMeetingParticipants creates participant records for all registrants of a meeting
func (s *MeetingsService) createPastMeetingParticipants(ctx context.Context, pastMeeting *models.PastMeeting, meeting *models.MeetingBase) error {
	slog.InfoContext(ctx, "creating past meeting participant records",
		"past_meeting_uid", pastMeeting.UID,
		"meeting_uid", meeting.UID,
	)

	// Get all registrants for this meeting
	registrants, err := s.RegistrantRepository.ListByMeeting(ctx, meeting.UID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get registrants for meeting", logging.ErrKey, err)
		return fmt.Errorf("failed to get registrants: %w", err)
	}

	// Track successful and failed creations with thread-safe counters
	var successCount, failedCount int
	var mu sync.Mutex
	var failedEmails []string

	tasks := []func() error{}

	// Create PastMeetingParticipant records for all registrants
	for _, registrant := range registrants {
		// Capture registrant in closure
		r := registrant
		tasks = append(tasks, func() error {
			participant := &models.PastMeetingParticipant{
				UID:            uuid.New().String(),
				PastMeetingUID: pastMeeting.UID,
				MeetingUID:     meeting.UID,
				Email:          r.Email,
				FirstName:      r.FirstName,
				LastName:       r.LastName,
				IsInvited:      true,
				IsAttended:     false, // Will be set to true when they join
				// Sessions will be updated when participants join/leave
			}

			err := s.PastMeetingParticipantRepository.Create(ctx, participant)

			// Use mutex to protect shared counters
			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				slog.ErrorContext(ctx, "failed to create past meeting participant record",
					logging.ErrKey, err,
					"past_meeting_uid", pastMeeting.UID,
					"meeting_uid", meeting.UID,
					"email", r.Email,
				)
				failedCount++
				failedEmails = append(failedEmails, r.Email)
				// Continue creating other participants even if one fails
			} else {
				successCount++
			}
			return nil
		})
	}

	errWorkerPool := concurrent.NewWorkerPool(10).Run(ctx, tasks...)
	if errWorkerPool != nil {
		slog.ErrorContext(ctx, "failed to create some past meeting participant records",
			logging.ErrKey, errWorkerPool,
			"meeting_uid", meeting.UID,
			"past_meeting_uid", pastMeeting.UID,
		)
	}

	slog.InfoContext(ctx, "completed creating past meeting participant records",
		"past_meeting_uid", pastMeeting.UID,
		"meeting_uid", meeting.UID,
		"total_registrants", len(registrants),
		"successful", successCount,
		"failed", failedCount,
		"failed_emails", failedEmails,
	)

	// Return error if all creations failed
	if failedCount > 0 && successCount == 0 {
		return fmt.Errorf("failed to create any participant records")
	}

	return nil
}

// updatePastMeetingSessionEndTime updates the end time for the matching session in an existing PastMeeting
func (s *MeetingsService) updatePastMeetingSessionEndTime(ctx context.Context, pastMeeting *models.PastMeeting, sessionUUID string, startTime, endTime time.Time) error {
	slog.InfoContext(ctx, "updating past meeting session end time",
		"past_meeting_uid", pastMeeting.UID,
		"session_uuid", sessionUUID,
		"end_time", endTime,
	)

	// Find the session with matching UUID and update its end time
	sessionFound := false
	for i := range pastMeeting.Sessions {
		if pastMeeting.Sessions[i].UID == sessionUUID {
			// Update end time
			pastMeeting.Sessions[i].EndTime = &endTime

			// If the session doesn't have a start time (zero value), use the one from the payload
			if pastMeeting.Sessions[i].StartTime.IsZero() {
				pastMeeting.Sessions[i].StartTime = startTime
				slog.InfoContext(ctx, "session missing start time, using start time from payload",
					"session_uuid", sessionUUID,
					"start_time_from_payload", startTime,
				)
			}

			sessionFound = true
			slog.InfoContext(ctx, "found and updated session end time",
				"session_uuid", sessionUUID,
				"start_time", pastMeeting.Sessions[i].StartTime,
				"end_time", endTime,
			)
			break
		}
	}

	if !sessionFound {
		slog.WarnContext(ctx, "session UUID not found in past meeting, adding new session",
			"session_uuid", sessionUUID,
			"past_meeting_uid", pastMeeting.UID,
		)
		// If session not found, add a new session with both start and end times
		// This could happen if there was a problem with the original session creation
		pastMeeting.Sessions = append(pastMeeting.Sessions, models.Session{
			UID:       sessionUUID,
			StartTime: startTime,
			EndTime:   &endTime,
		})
	}

	// Update the PastMeeting record in the repository
	// We need to get the current revision first
	_, revision, err := s.PastMeetingRepository.GetWithRevision(ctx, pastMeeting.UID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get past meeting revision for update", logging.ErrKey, err)
		return fmt.Errorf("failed to get past meeting revision: %w", err)
	}

	err = s.PastMeetingRepository.Update(ctx, pastMeeting, revision)
	if err != nil {
		slog.ErrorContext(ctx, "failed to update past meeting with session end time", logging.ErrKey, err)
		return fmt.Errorf("failed to update past meeting: %w", err)
	}

	slog.InfoContext(ctx, "successfully updated past meeting session end time",
		"past_meeting_uid", pastMeeting.UID,
		"session_uuid", sessionUUID,
	)

	return nil
}

// createPastMeetingRecordForEndedEvent creates a PastMeeting record when meeting.ended is received without a prior meeting.started
func (s *MeetingsService) createPastMeetingRecordForEndedEvent(ctx context.Context, meeting *models.MeetingBase, zoomData ZoomPayloadForPastMeeting) (*models.PastMeeting, error) {
	return s.createPastMeetingRecordWithSession(ctx, meeting, zoomData)
}

// createPastMeetingRecordWithSession creates a PastMeeting record with the specified session details
func (s *MeetingsService) createPastMeetingRecordWithSession(ctx context.Context, meeting *models.MeetingBase, zoomData ZoomPayloadForPastMeeting) (*models.PastMeeting, error) {
	contextType := "creating past meeting record"
	if zoomData.EndTime != nil {
		contextType = "creating past meeting record for ended event (fallback)"
	}

	logFields := []any{
		"meeting_uid", meeting.UID,
		"zoom_meeting_uuid", zoomData.UUID,
		"actual_start_time", zoomData.StartTime,
		"timezone", zoomData.Timezone,
	}
	if zoomData.EndTime != nil {
		logFields = append(logFields, "actual_end_time", *zoomData.EndTime)
	}

	slog.InfoContext(ctx, contextType, logFields...)

	// Get platform meeting ID from Zoom config
	platformMeetingID := ""
	if meeting.ZoomConfig != nil {
		platformMeetingID = meeting.ZoomConfig.MeetingID
	}

	// Calculate scheduled end time based on duration
	scheduledEndTime := meeting.StartTime.Add(time.Duration(meeting.Duration) * time.Minute)

	// Create session with appropriate end time
	session := models.Session{
		UID:       zoomData.UUID,
		StartTime: zoomData.StartTime,
		EndTime:   zoomData.EndTime, // nil for started events, set for ended events
	}

	// Create PastMeeting record with current meeting attributes and actual webhook data
	pastMeeting := &models.PastMeeting{
		UID:                  uuid.New().String(),
		MeetingUID:           meeting.UID,
		OccurrenceID:         "", // TODO: set occurrence ID once we have occurrences figured out
		ProjectUID:           meeting.ProjectUID,
		ScheduledStartTime:   meeting.StartTime, // Scheduled time from our meeting
		ScheduledEndTime:     scheduledEndTime,
		Duration:             meeting.Duration,
		Timezone:             zoomData.Timezone, // Use timezone from webhook payload
		Recurrence:           meeting.Recurrence,
		Title:                meeting.Title,
		Description:          meeting.Description,
		Committees:           meeting.Committees,
		Platform:             meeting.Platform,
		PlatformMeetingID:    platformMeetingID,
		EarlyJoinTimeMinutes: meeting.EarlyJoinTimeMinutes,
		MeetingType:          meeting.MeetingType,
		Visibility:           meeting.Visibility,
		Restricted:           meeting.Restricted,
		ArtifactVisibility:   meeting.ArtifactVisibility,
		PublicLink:           meeting.PublicLink,
		RecordingEnabled:     meeting.RecordingEnabled,
		TranscriptEnabled:    meeting.TranscriptEnabled,
		YoutubeUploadEnabled: meeting.YoutubeUploadEnabled,
		ZoomConfig:           meeting.ZoomConfig,
		Sessions:             []models.Session{session},
	}

	// Create the PastMeeting record
	err := s.PastMeetingRepository.Create(ctx, pastMeeting)
	if err != nil {
		slog.ErrorContext(ctx, "failed to create past meeting record", logging.ErrKey, err)
		return nil, fmt.Errorf("failed to create past meeting record: %w", err)
	}

	successLogFields := []any{
		"past_meeting_uid", pastMeeting.UID,
		"meeting_uid", meeting.UID,
	}
	if zoomData.EndTime != nil {
		successLogFields = append(successLogFields, "session_duration", zoomData.EndTime.Sub(zoomData.StartTime))
	}

	slog.InfoContext(ctx, "successfully created past meeting record", successLogFields...)

	return pastMeeting, nil
}
