// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/service"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/concurrent"
)

// ZoomWebhookHandler handles Zoom webhook events.
type ZoomWebhookHandler struct {
	meetingService                *service.MeetingService
	registrantService             *service.MeetingRegistrantService
	pastMeetingService            *service.PastMeetingService
	pastMeetingParticipantService *service.PastMeetingParticipantService
	pastMeetingRecordingService   *service.PastMeetingRecordingService
	WebhookValidator              domain.WebhookValidator
}

func NewZoomWebhookHandler(
	meetingService *service.MeetingService,
	registrantService *service.MeetingRegistrantService,
	pastMeetingService *service.PastMeetingService,
	pastMeetingParticipantService *service.PastMeetingParticipantService,
	pastMeetingRecordingService *service.PastMeetingRecordingService,
	webhookValidator domain.WebhookValidator,
) *ZoomWebhookHandler {
	return &ZoomWebhookHandler{
		meetingService:                meetingService,
		registrantService:             registrantService,
		pastMeetingService:            pastMeetingService,
		pastMeetingParticipantService: pastMeetingParticipantService,
		pastMeetingRecordingService:   pastMeetingRecordingService,
		WebhookValidator:              webhookValidator,
	}
}

func (s *ZoomWebhookHandler) HandlerReady() bool {
	return s.meetingService.ServiceReady() &&
		s.registrantService.ServiceReady() &&
		s.pastMeetingService.ServiceReady() &&
		s.pastMeetingParticipantService.ServiceReady() &&
		s.pastMeetingRecordingService.ServiceReady()
}

// ZoomPayloadForPastMeeting contains the essential Zoom webhook data for creating PastMeeting records
type ZoomPayloadForPastMeeting struct {
	UUID      string
	StartTime time.Time
	EndTime   *time.Time // nil for meeting.started, set for meeting.ended
	Timezone  string
}

// ZoomPayloadForParticipant represents participant data from Zoom webhook events
type ZoomPayloadForParticipant struct {
	UserID            string
	UserName          string
	ID                string
	JoinTime          time.Time
	Email             string
	ParticipantUserID string
	LeaveTime         time.Time
	LeaveReason       string
}

// HandleMessage implements [domain.MessageHandler] interface
func (s *ZoomWebhookHandler) HandleMessage(ctx context.Context, msg domain.Message) {
	subject := msg.Subject()
	ctx = logging.AppendCtx(ctx, slog.String("subject", subject))
	slog.DebugContext(ctx, "handling NATS message")

	var response []byte
	var err error

	handlers := map[string]func(ctx context.Context, msg domain.Message) ([]byte, error){
		models.ZoomWebhookMeetingStartedSubject:               s.HandleZoomMeetingStarted,
		models.ZoomWebhookMeetingEndedSubject:                 s.HandleZoomMeetingEnded,
		models.ZoomWebhookMeetingDeletedSubject:               s.HandleZoomMeetingDeleted,
		models.ZoomWebhookMeetingParticipantJoinedSubject:     s.HandleZoomParticipantJoined,
		models.ZoomWebhookMeetingParticipantLeftSubject:       s.HandleZoomParticipantLeft,
		models.ZoomWebhookRecordingCompletedSubject:           s.HandleZoomRecordingCompleted,
		models.ZoomWebhookRecordingTranscriptCompletedSubject: s.HandleZoomTranscriptCompleted,
		models.ZoomWebhookMeetingSummaryCompletedSubject:      s.HandleZoomSummaryCompleted,
	}

	handler, ok := handlers[subject]
	if !ok {
		slog.WarnContext(ctx, "unknown subject")
		if msg.HasReply() {
			err = msg.Respond(nil)
			if err != nil {
				slog.ErrorContext(ctx, "error responding to NATS message", logging.ErrKey, err)
			}
		}
		return
	}

	response, err = handler(ctx, msg)
	if err != nil {
		slog.ErrorContext(ctx, "error handling message",
			logging.ErrKey, err,
		)
		if msg.HasReply() {
			err = msg.Respond(nil)
			if err != nil {
				slog.ErrorContext(ctx, "error responding to NATS message", logging.ErrKey, err)
			}
		}
		return
	}

	if msg.HasReply() {
		err = msg.Respond(response)
		if err != nil {
			slog.ErrorContext(ctx, "error responding to NATS message", logging.ErrKey, err)
			return
		}
		slog.DebugContext(ctx, "responded to NATS message", "response", response)
	} else {
		slog.DebugContext(ctx, "handled NATS message (no reply expected)")
	}
}

// parseZoomWebhookEvent is a helper to parse webhook event messages
func (s *ZoomWebhookHandler) parseZoomWebhookEvent(ctx context.Context, msg domain.Message) (*models.ZoomWebhookEventMessage, error) {
	var webhookEvent models.ZoomWebhookEventMessage
	if err := json.Unmarshal(msg.Data(), &webhookEvent); err != nil {
		slog.ErrorContext(ctx, "failed to unmarshal Zoom webhook event", logging.ErrKey, err)
		return nil, err
	}
	return &webhookEvent, nil
}

// HandleZoomMeetingStarted handles meeting.started webhook events
func (s *ZoomWebhookHandler) HandleZoomMeetingStarted(ctx context.Context, msg domain.Message) ([]byte, error) {
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
func (s *ZoomWebhookHandler) HandleZoomMeetingEnded(ctx context.Context, msg domain.Message) ([]byte, error) {
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
func (s *ZoomWebhookHandler) HandleZoomMeetingDeleted(ctx context.Context, msg domain.Message) ([]byte, error) {
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
func (s *ZoomWebhookHandler) HandleZoomParticipantJoined(ctx context.Context, msg domain.Message) ([]byte, error) {
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
func (s *ZoomWebhookHandler) HandleZoomParticipantLeft(ctx context.Context, msg domain.Message) ([]byte, error) {
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
func (s *ZoomWebhookHandler) HandleZoomRecordingCompleted(ctx context.Context, msg domain.Message) ([]byte, error) {
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
func (s *ZoomWebhookHandler) HandleZoomTranscriptCompleted(ctx context.Context, msg domain.Message) ([]byte, error) {
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
func (s *ZoomWebhookHandler) HandleZoomSummaryCompleted(ctx context.Context, msg domain.Message) ([]byte, error) {
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
func (s *ZoomWebhookHandler) handleMeetingStartedEvent(ctx context.Context, event models.ZoomWebhookEventMessage) error {
	slog.InfoContext(ctx, "processing meeting started event")

	// Convert to typed payload
	payload, err := event.ToMeetingStartedPayload()
	if err != nil {
		slog.ErrorContext(ctx, "failed to parse meeting started payload", logging.ErrKey, err)
		return fmt.Errorf("invalid meeting.started payload: %w", err)
	}

	meetingObj := payload.Object

	slog.DebugContext(ctx, "meeting started",
		"zoom_meeting_id", meetingObj.ID,
		"zoom_meeting_uuid", meetingObj.UUID,
		"topic", meetingObj.Topic,
		"start_time", meetingObj.StartTime,
		"host_id", meetingObj.HostID,
	)

	// Find the meeting by Zoom meeting ID
	meeting, err := s.meetingService.MeetingRepository.GetByZoomMeetingID(ctx, meetingObj.ID)
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
		slog.ErrorContext(ctx, "failed to create past meeting record",
			logging.ErrKey, err,
			logging.PriorityCritical(),
		)
		return fmt.Errorf("failed to create past meeting record: %w", err)
	}

	// Create participant records for all registrants
	err = s.createPastMeetingParticipants(ctx, pastMeeting, meeting)
	if err != nil {
		// Log the error but don't fail the entire webhook processing
		slog.ErrorContext(ctx, "failed to create past meeting participants",
			logging.ErrKey, err,
			logging.PriorityCritical(),
		)
	}

	return nil
}

// handleMeetingEndedEvent processes meeting.ended events
func (s *ZoomWebhookHandler) handleMeetingEndedEvent(ctx context.Context, event models.ZoomWebhookEventMessage) error {
	// Convert to typed payload
	payload, err := event.ToMeetingEndedPayload()
	if err != nil {
		slog.ErrorContext(ctx, "failed to convert to typed meeting ended payload", logging.ErrKey, err)
		return fmt.Errorf("failed to parse meeting ended payload: %w", err)
	}

	meetingObj := payload.Object
	slog.DebugContext(ctx, "processing meeting ended event",
		"zoom_meeting_uuid", meetingObj.UUID,
		"zoom_meeting_id", meetingObj.ID,
		"topic", meetingObj.Topic,
		"start_time", meetingObj.StartTime,
		"end_time", meetingObj.EndTime,
		"duration", meetingObj.Duration,
		"timezone", meetingObj.Timezone,
	)

	// Try to find existing PastMeeting record by platform meeting ID
	existingPastMeeting, err := s.pastMeetingService.PastMeetingRepository.GetByPlatformMeetingID(ctx, "Zoom", meetingObj.ID)
	if err != nil && err != domain.ErrPastMeetingNotFound {
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
		meeting, err := s.meetingService.MeetingRepository.GetByZoomMeetingID(ctx, meetingObj.ID)
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
			slog.ErrorContext(ctx, "failed to create past meeting record for ended event",
				logging.ErrKey, err,
				logging.PriorityCritical(),
			)
			return fmt.Errorf("failed to create past meeting record for ended event: %w", err)
		}

		// Create participant records for all registrants
		err = s.createPastMeetingParticipants(ctx, pastMeeting, meeting)
		if err != nil {
			// Log the error but don't fail the entire webhook processing
			slog.ErrorContext(ctx, "failed to create past meeting participants for ended event",
				logging.ErrKey, err,
				logging.PriorityCritical(),
			)
		}

		slog.InfoContext(ctx, "successfully created past meeting record from ended event",
			"past_meeting_uid", pastMeeting.UID,
			"meeting_uid", meeting.UID,
		)
	}

	return nil
}

// handleMeetingDeletedEvent processes meeting.deleted events
func (s *ZoomWebhookHandler) handleMeetingDeletedEvent(ctx context.Context, event models.ZoomWebhookEventMessage) error {
	// Convert to typed payload
	payload, err := event.ToMeetingDeletedPayload()
	if err != nil {
		slog.ErrorContext(ctx, "failed to convert to typed meeting deleted payload", "error", err)
		return fmt.Errorf("failed to parse meeting deleted payload: %w", err)
	}

	slog.DebugContext(ctx, "processing meeting deleted event",
		"zoom_meeting_uuid", payload.Object.UUID,
		"zoom_meeting_id", payload.Object.ID,
		"topic", payload.Object.Topic,
	)

	return nil
}

// handleParticipantJoinedEvent processes meeting.participant_joined events
func (s *ZoomWebhookHandler) handleParticipantJoinedEvent(ctx context.Context, event models.ZoomWebhookEventMessage) error {
	// Convert to typed payload
	payload, err := event.ToParticipantJoinedPayload()
	if err != nil {
		slog.ErrorContext(ctx, "failed to convert to typed participant joined payload", logging.ErrKey, err)
		return fmt.Errorf("failed to parse participant joined payload: %w", err)
	}

	meetingObj := payload.Object
	participant := meetingObj.Participant

	slog.DebugContext(ctx, "processing participant joined event",
		"zoom_meeting_uuid", meetingObj.UUID,
		"zoom_meeting_id", meetingObj.ID,
		"participant_id", participant.ID,
		"participant_name", participant.UserName,
		"participant_email", participant.Email,
		"join_time", participant.JoinTime,
	)

	// Find the PastMeeting record by platform meeting ID
	pastMeeting, err := s.pastMeetingService.PastMeetingRepository.GetByPlatformMeetingID(ctx, "Zoom", meetingObj.ID)
	if err != nil {
		if err == domain.ErrPastMeetingNotFound {
			slog.WarnContext(ctx, "no past meeting found for participant joined event, skipping",
				"zoom_meeting_id", meetingObj.ID,
				"participant_email", participant.Email,
			)
			return nil // Don't fail the webhook processing
		}
		slog.ErrorContext(ctx, "error searching for past meeting", logging.ErrKey, err)
		return fmt.Errorf("failed to search for past meeting: %w", err)
	}

	// Try to find existing PastMeetingParticipant record
	existingParticipant, err := s.pastMeetingParticipantService.PastMeetingParticipantRepository.GetByPastMeetingAndEmail(ctx, pastMeeting.UID, participant.Email)
	if err != nil && err != domain.ErrPastMeetingParticipantNotFound {
		slog.ErrorContext(ctx, "error searching for existing participant", logging.ErrKey, err)
		return fmt.Errorf("failed to search for existing participant: %w", err)
	}

	if existingParticipant != nil {
		// Update existing participant to mark as attended and add new session
		err = s.updateParticipantAttendance(ctx, existingParticipant, participant.ID, participant.JoinTime)
		if err != nil {
			return fmt.Errorf("failed to update participant attendance: %w", err)
		}
		slog.InfoContext(ctx, "updated existing participant attendance",
			"participant_uid", existingParticipant.UID,
			"email", participant.Email,
			"session_uid", participant.ID,
		)
	} else {
		// Create new participant record
		zoomParticipant := ZoomPayloadForParticipant{
			UserID:            participant.UserID,
			UserName:          participant.UserName,
			ID:                participant.ID,
			JoinTime:          participant.JoinTime,
			Email:             participant.Email,
			ParticipantUserID: participant.ParticipantUserID,
		}
		newParticipant, err := s.createParticipantFromJoinedEvent(ctx, pastMeeting, zoomParticipant)
		if err != nil {
			slog.ErrorContext(ctx, "failed to create participant from joined event",
				logging.ErrKey, err,
				logging.PriorityCritical(),
			)
			return fmt.Errorf("failed to create participant from joined event: %w", err)
		}
		slog.InfoContext(ctx, "created new participant from joined event",
			"participant_uid", newParticipant.UID,
			"email", participant.Email,
			"is_invited", newParticipant.IsInvited,
		)
	}

	return nil
}

// handleParticipantLeftEvent processes meeting.participant_left events
func (s *ZoomWebhookHandler) handleParticipantLeftEvent(ctx context.Context, event models.ZoomWebhookEventMessage) error {
	// Convert to typed payload
	payload, err := event.ToParticipantLeftPayload()
	if err != nil {
		slog.ErrorContext(ctx, "failed to convert to typed participant left payload", logging.ErrKey, err)
		return fmt.Errorf("failed to parse participant left payload: %w", err)
	}

	meetingObj := payload.Object
	participant := meetingObj.Participant

	slog.DebugContext(ctx, "processing participant left event",
		"zoom_meeting_uuid", meetingObj.UUID,
		"zoom_meeting_id", meetingObj.ID,
		"participant_id", participant.ID,
		"participant_name", participant.UserName,
		"participant_email", participant.Email,
		"leave_time", participant.LeaveTime,
		"duration", participant.Duration,
	)

	// Find the PastMeeting record by platform meeting ID
	pastMeeting, err := s.pastMeetingService.PastMeetingRepository.GetByPlatformMeetingID(ctx, "Zoom", meetingObj.ID)
	if err != nil {
		if err == domain.ErrPastMeetingNotFound {
			slog.WarnContext(ctx, "no past meeting found for participant left event, skipping",
				"zoom_meeting_id", meetingObj.ID,
				"participant_email", participant.Email,
			)
			return nil // Don't fail the webhook processing
		}
		slog.ErrorContext(ctx, "error searching for past meeting", logging.ErrKey, err)
		return fmt.Errorf("failed to search for past meeting: %w", err)
	}

	// Try to find existing PastMeetingParticipant record
	existingParticipant, err := s.pastMeetingParticipantService.PastMeetingParticipantRepository.GetByPastMeetingAndEmail(ctx, pastMeeting.UID, participant.Email)
	if err != nil && err != domain.ErrPastMeetingParticipantNotFound {
		slog.ErrorContext(ctx, "error searching for existing participant", logging.ErrKey, err)
		return fmt.Errorf("failed to search for existing participant: %w", err)
	}

	if existingParticipant != nil {
		// Update existing participant's session with leave time
		err = s.updateParticipantSessionLeaveTime(ctx, existingParticipant, participant.ID, participant.LeaveTime, participant.LeaveReason)
		if err != nil {
			return fmt.Errorf("failed to update participant session leave time: %w", err)
		}
		slog.InfoContext(ctx, "updated participant session leave time",
			"participant_uid", existingParticipant.UID,
			"email", participant.Email,
			"session_uid", participant.ID,
			"duration", participant.Duration,
		)
	} else {
		// Create new participant record with completed session (they joined and left but we missed the joined event)
		slog.WarnContext(ctx, "no existing participant found for left event, creating new record",
			"participant_email", participant.Email,
			"zoom_meeting_id", meetingObj.ID,
		)

		zoomParticipant := ZoomPayloadForParticipant{
			UserID:            participant.UserID,
			UserName:          participant.UserName,
			ID:                participant.ID,
			JoinTime:          participant.LeaveTime.Add(-time.Duration(participant.Duration) * time.Second), // Calculate join time from leave time and duration
			Email:             participant.Email,
			ParticipantUserID: participant.ParticipantUserID,
			LeaveTime:         participant.LeaveTime,
			LeaveReason:       participant.LeaveReason,
		}

		newParticipant, err := s.createParticipantFromLeftEvent(ctx, pastMeeting, zoomParticipant)
		if err != nil {
			slog.ErrorContext(ctx, "failed to create participant from left event",
				logging.ErrKey, err,
				logging.PriorityCritical(),
			)
			return fmt.Errorf("failed to create participant from left event: %w", err)
		}
		slog.InfoContext(ctx, "created participant record from left event",
			"participant_uid", newParticipant.UID,
			"email", participant.Email,
			"calculated_join_time", zoomParticipant.JoinTime,
			"leave_time", participant.LeaveTime,
		)
	}

	return nil
}

// handleRecordingCompletedEvent processes recording.completed events
func (s *ZoomWebhookHandler) handleRecordingCompletedEvent(ctx context.Context, event models.ZoomWebhookEventMessage) error {
	// Convert to typed payload
	payload, err := event.ToRecordingCompletedPayload()
	if err != nil {
		slog.ErrorContext(ctx, "failed to convert to typed recording completed payload", "error", err)
		return fmt.Errorf("failed to parse recording completed payload: %w", err)
	}

	slog.DebugContext(ctx, "processing recording completed event",
		"zoom_meeting_uuid", payload.Object.UUID,
		"zoom_meeting_id", payload.Object.ID,
		"topic", payload.Object.Topic,
		"total_size", payload.Object.TotalSize,
		"recording_count", payload.Object.RecordingCount,
	)

	// Find the PastMeeting record by platform meeting ID
	meetingIDStr := strconv.FormatInt(payload.Object.ID, 10)
	pastMeeting, err := s.pastMeetingService.PastMeetingRepository.GetByPlatformMeetingID(ctx, "Zoom", meetingIDStr)
	if err != nil {
		if err == domain.ErrPastMeetingNotFound {
			slog.WarnContext(ctx, "no past meeting found for recording completed event, skipping",
				"zoom_meeting_id", payload.Object.ID,
				"zoom_meeting_uuid", payload.Object.UUID,
				"topic", payload.Object.Topic,
			)
			return nil // Not an error - we just don't have a past meeting record yet
		}
		slog.ErrorContext(ctx, "error finding past meeting for recording", logging.ErrKey, err,
			"zoom_meeting_id", payload.Object.ID,
		)
		return fmt.Errorf("failed to find past meeting: %w", err)
	}

	// Check if a recording already exists for this past meeting
	existingRecording, err := s.pastMeetingRecordingService.GetRecordingByPastMeetingUID(ctx, pastMeeting.UID)
	if err != nil && err != domain.ErrPastMeetingRecordingNotFound {
		slog.ErrorContext(ctx, "error checking for existing recording", logging.ErrKey, err,
			"past_meeting_uid", pastMeeting.UID,
			"zoom_meeting_id", payload.Object.ID,
		)
		return fmt.Errorf("failed to check for existing recording: %w", err)
	}

	// Create domain model from Zoom payload
	recordingFromPayload := s.createRecordingFromZoomPayload(pastMeeting.UID, *payload)

	var recording *models.PastMeetingRecording
	if existingRecording != nil {
		// Update existing recording with new files
		recording, err = s.pastMeetingRecordingService.UpdateRecording(ctx, existingRecording.UID, recordingFromPayload)
		if err != nil {
			slog.ErrorContext(ctx, "error updating recording", logging.ErrKey, err,
				"recording_uid", existingRecording.UID,
				"past_meeting_uid", pastMeeting.UID,
				"zoom_meeting_id", payload.Object.ID,
			)
			return fmt.Errorf("failed to update recording: %w", err)
		}
		slog.InfoContext(ctx, "successfully updated existing recording",
			"recording_uid", recording.UID,
			"past_meeting_uid", pastMeeting.UID,
			"zoom_meeting_id", payload.Object.ID,
			"total_files", len(recording.RecordingFiles),
			"total_size", recording.TotalSize,
		)
	} else {
		// Create new recording
		recording, err = s.pastMeetingRecordingService.CreateRecording(ctx, recordingFromPayload)
		if err != nil {
			slog.ErrorContext(ctx, "error creating recording", logging.ErrKey, err,
				"past_meeting_uid", pastMeeting.UID,
				"zoom_meeting_id", payload.Object.ID,
			)
			return fmt.Errorf("failed to create recording: %w", err)
		}
		slog.InfoContext(ctx, "successfully created new recording",
			"recording_uid", recording.UID,
			"past_meeting_uid", pastMeeting.UID,
			"zoom_meeting_id", payload.Object.ID,
			"total_files", len(recording.RecordingFiles),
			"total_size", recording.TotalSize,
		)
	}

	return nil
}

// createRecordingFromZoomPayload converts a Zoom recording completed payload to a domain model.
// This conversion logic belongs in the application layer (handler) rather than the domain model.
func (s *ZoomWebhookHandler) createRecordingFromZoomPayload(pastMeetingUID string, payload models.ZoomRecordingCompletedPayload) *models.PastMeetingRecording {
	recording := &models.PastMeetingRecording{
		PastMeetingUID:    pastMeetingUID,
		Platform:          "Zoom",
		PlatformMeetingID: strconv.FormatInt(payload.Object.ID, 10),
	}

	// Create recording session from payload
	session := models.RecordingSession{
		UUID:      payload.Object.UUID,
		ShareURL:  payload.Object.ShareURL,
		TotalSize: payload.Object.TotalSize,
		StartTime: payload.Object.StartTime,
	}
	recording.AddRecordingSession(session)

	// Convert Zoom recording files to domain model format
	var recordingFiles []models.RecordingFileData
	for _, zoomFile := range payload.Object.RecordingFiles {
		recordingFiles = append(recordingFiles, models.RecordingFileData{
			ID:                zoomFile.ID,
			PlatformMeetingID: zoomFile.MeetingID,
			RecordingStart:    zoomFile.RecordingStart,
			RecordingEnd:      zoomFile.RecordingEnd,
			FileType:          zoomFile.FileType,
			FileSize:          zoomFile.FileSize,
			PlayURL:           zoomFile.PlayURL,
			DownloadURL:       zoomFile.DownloadURL,
			Status:            zoomFile.Status,
			RecordingType:     zoomFile.RecordingType,
		})
	}

	// Add files to the recording (this will calculate total size and count)
	recording.AddRecordingFiles(recordingFiles)

	return recording
}

// createRecordingFromZoomTranscriptPayload converts a Zoom transcript completed payload to a domain model.
// This conversion logic belongs in the application layer (handler) rather than the domain model.
func (s *ZoomWebhookHandler) createRecordingFromZoomTranscriptPayload(pastMeetingUID string, payload models.ZoomTranscriptCompletedPayload) *models.PastMeetingRecording {
	recording := &models.PastMeetingRecording{
		PastMeetingUID:    pastMeetingUID,
		Platform:          "Zoom",
		PlatformMeetingID: strconv.FormatInt(payload.Object.ID, 10),
	}

	session := models.RecordingSession{
		UUID:      payload.Object.UUID,
		ShareURL:  payload.Object.ShareURL,
		TotalSize: payload.Object.TotalSize,
		StartTime: payload.Object.StartTime,
	}
	recording.AddRecordingSession(session)

	// Convert Zoom transcript files to domain model format
	var recordingFiles []models.RecordingFileData
	for _, zoomFile := range payload.Object.RecordingFiles {
		recordingFiles = append(recordingFiles, models.RecordingFileData{
			ID:                zoomFile.ID,
			PlatformMeetingID: zoomFile.MeetingID,
			RecordingStart:    zoomFile.RecordingStart,
			RecordingEnd:      zoomFile.RecordingEnd,
			FileType:          zoomFile.FileType,
			FileSize:          zoomFile.FileSize,
			PlayURL:           zoomFile.PlayURL,
			DownloadURL:       zoomFile.DownloadURL,
			Status:            zoomFile.Status,
			RecordingType:     zoomFile.RecordingType,
		})
	}

	// Add files to the recording (this will calculate total size and count)
	recording.AddRecordingFiles(recordingFiles)

	return recording
}

// handleTranscriptCompletedEvent processes recording.transcript_completed events
func (s *ZoomWebhookHandler) handleTranscriptCompletedEvent(ctx context.Context, event models.ZoomWebhookEventMessage) error {
	// Convert to typed payload
	payload, err := event.ToTranscriptCompletedPayload()
	if err != nil {
		slog.ErrorContext(ctx, "failed to convert to typed transcript completed payload", "error", err)
		return fmt.Errorf("failed to parse transcript completed payload: %w", err)
	}

	slog.DebugContext(ctx, "processing transcript completed event",
		"zoom_meeting_uuid", payload.Object.UUID,
		"zoom_meeting_id", payload.Object.ID,
		"topic", payload.Object.Topic,
		"duration", payload.Object.Duration,
		"recording_files", len(payload.Object.RecordingFiles),
	)

	// Find the PastMeeting record by platform meeting ID
	meetingIDStr := strconv.FormatInt(payload.Object.ID, 10)
	pastMeeting, err := s.pastMeetingService.PastMeetingRepository.GetByPlatformMeetingID(ctx, "Zoom", meetingIDStr)
	if err != nil {
		if err == domain.ErrPastMeetingNotFound {
			slog.WarnContext(ctx, "no past meeting found for transcript completed event, skipping",
				"zoom_meeting_id", payload.Object.ID,
				"zoom_meeting_uuid", payload.Object.UUID,
				"topic", payload.Object.Topic,
			)
			return nil // Not an error - we just don't have a past meeting record yet
		}
		slog.ErrorContext(ctx, "error finding past meeting for transcript", logging.ErrKey, err,
			"zoom_meeting_id", payload.Object.ID,
		)
		return fmt.Errorf("failed to find past meeting: %w", err)
	}

	// Check if a recording already exists for this past meeting
	existingRecording, err := s.pastMeetingRecordingService.GetRecordingByPastMeetingUID(ctx, pastMeeting.UID)
	if err != nil && err != domain.ErrPastMeetingRecordingNotFound {
		slog.ErrorContext(ctx, "error checking for existing recording", logging.ErrKey, err,
			"past_meeting_uid", pastMeeting.UID,
			"zoom_meeting_id", payload.Object.ID,
		)
		return fmt.Errorf("failed to check for existing recording: %w", err)
	}

	// Create domain model from Zoom transcript payload
	recordingFromPayload := s.createRecordingFromZoomTranscriptPayload(pastMeeting.UID, *payload)

	var recording *models.PastMeetingRecording
	if existingRecording != nil {
		// Update existing recording with new transcript files
		recording, err = s.pastMeetingRecordingService.UpdateRecording(ctx, existingRecording.UID, recordingFromPayload)
		if err != nil {
			slog.ErrorContext(ctx, "error updating recording with transcript", logging.ErrKey, err,
				"recording_uid", existingRecording.UID,
				"past_meeting_uid", pastMeeting.UID,
				"zoom_meeting_id", payload.Object.ID,
			)
			return fmt.Errorf("failed to update recording: %w", err)
		}
		slog.InfoContext(ctx, "successfully updated recording with transcript files",
			"recording_uid", recording.UID,
			"past_meeting_uid", pastMeeting.UID,
			"zoom_meeting_id", payload.Object.ID,
			"total_files", len(recording.RecordingFiles),
			"total_size", recording.TotalSize,
		)
	} else {
		// Create new recording with transcript files
		recording, err = s.pastMeetingRecordingService.CreateRecording(ctx, recordingFromPayload)
		if err != nil {
			slog.ErrorContext(ctx, "error creating recording with transcript", logging.ErrKey, err,
				"past_meeting_uid", pastMeeting.UID,
				"zoom_meeting_id", payload.Object.ID,
			)
			return fmt.Errorf("failed to create recording: %w", err)
		}
		slog.InfoContext(ctx, "successfully created recording with transcript files",
			"recording_uid", recording.UID,
			"past_meeting_uid", pastMeeting.UID,
			"zoom_meeting_id", payload.Object.ID,
			"total_files", len(recording.RecordingFiles),
			"total_size", recording.TotalSize,
		)
	}

	return nil
}

// handleSummaryCompletedEvent processes meeting.summary_completed events
func (s *ZoomWebhookHandler) handleSummaryCompletedEvent(ctx context.Context, event models.ZoomWebhookEventMessage) error {
	// Convert to typed payload
	payload, err := event.ToSummaryCompletedPayload()
	if err != nil {
		slog.ErrorContext(ctx, "failed to convert to typed summary completed payload", "error", err)
		return fmt.Errorf("failed to parse summary completed payload: %w", err)
	}

	slog.DebugContext(ctx, "processing summary completed event",
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
func (s *ZoomWebhookHandler) createPastMeetingRecord(ctx context.Context, meeting *models.MeetingBase, zoomData ZoomPayloadForPastMeeting) (*models.PastMeeting, error) {
	return s.createPastMeetingRecordWithSession(ctx, meeting, zoomData)
}

// createPastMeetingParticipants creates participant records for all registrants of a meeting
func (s *ZoomWebhookHandler) createPastMeetingParticipants(ctx context.Context, pastMeeting *models.PastMeeting, meeting *models.MeetingBase) error {
	slog.DebugContext(ctx, "creating past meeting participant records",
		"past_meeting_uid", pastMeeting.UID,
		"meeting_uid", meeting.UID,
	)

	// Get all registrants for this meeting
	registrants, err := s.registrantService.RegistrantRepository.ListByMeeting(ctx, meeting.UID)
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

			err := s.pastMeetingParticipantService.PastMeetingParticipantRepository.Create(ctx, participant)

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

	slog.DebugContext(ctx, "completed creating past meeting participant records",
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
func (s *ZoomWebhookHandler) updatePastMeetingSessionEndTime(ctx context.Context, pastMeeting *models.PastMeeting, sessionUUID string, startTime, endTime time.Time) error {
	slog.DebugContext(ctx, "updating past meeting session end time",
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
				slog.WarnContext(ctx, "session missing start time, using start time from payload",
					"session_uuid", sessionUUID,
					"start_time_from_payload", startTime,
				)
			}

			sessionFound = true
			slog.DebugContext(ctx, "found and updated session end time",
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
	_, revision, err := s.pastMeetingService.PastMeetingRepository.GetWithRevision(ctx, pastMeeting.UID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get past meeting revision for update", logging.ErrKey, err)
		return fmt.Errorf("failed to get past meeting revision: %w", err)
	}

	err = s.pastMeetingService.PastMeetingRepository.Update(ctx, pastMeeting, revision)
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
func (s *ZoomWebhookHandler) createPastMeetingRecordForEndedEvent(ctx context.Context, meeting *models.MeetingBase, zoomData ZoomPayloadForPastMeeting) (*models.PastMeeting, error) {
	return s.createPastMeetingRecordWithSession(ctx, meeting, zoomData)
}

// createPastMeetingRecordWithSession creates a PastMeeting record with the specified session details
func (s *ZoomWebhookHandler) createPastMeetingRecordWithSession(ctx context.Context, meeting *models.MeetingBase, zoomData ZoomPayloadForPastMeeting) (*models.PastMeeting, error) {
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

	slog.DebugContext(ctx, contextType, logFields...)

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
	err := s.pastMeetingService.PastMeetingRepository.Create(ctx, pastMeeting)
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

	slog.DebugContext(ctx, "successfully created past meeting record", successLogFields...)

	return pastMeeting, nil
}

// updateParticipantAttendance updates an existing participant record to mark them as attended and add a new session
func (s *ZoomWebhookHandler) updateParticipantAttendance(ctx context.Context, participant *models.PastMeetingParticipant, sessionUID string, joinTime time.Time) error {
	slog.DebugContext(ctx, "updating participant attendance and adding session",
		"participant_uid", participant.UID,
		"email", participant.Email,
		"session_uid", sessionUID,
		"join_time", joinTime,
	)

	// Mark as attended
	participant.IsAttended = true

	// Add new session for this join event
	newSession := models.ParticipantSession{
		UID:      sessionUID,
		JoinTime: joinTime,
		// LeaveTime will be set when participant leaves
	}

	// Check if this session already exists (shouldn't happen, but be safe)
	sessionExists := false
	for _, session := range participant.Sessions {
		if session.UID == sessionUID {
			sessionExists = true
			slog.WarnContext(ctx, "session already exists for participant, skipping",
				"participant_uid", participant.UID,
				"session_uid", sessionUID,
			)
			break
		}
	}

	if !sessionExists {
		participant.Sessions = append(participant.Sessions, newSession)
		slog.DebugContext(ctx, "added new session to participant",
			"participant_uid", participant.UID,
			"session_uid", sessionUID,
			"total_sessions", len(participant.Sessions),
		)
	}

	// Get current revision for update
	_, revision, err := s.pastMeetingParticipantService.PastMeetingParticipantRepository.GetWithRevision(ctx, participant.UID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get participant revision for attendance update", logging.ErrKey, err)
		return fmt.Errorf("failed to get participant revision: %w", err)
	}

	// Update the participant record
	err = s.pastMeetingParticipantService.PastMeetingParticipantRepository.Update(ctx, participant, revision)
	if err != nil {
		slog.ErrorContext(ctx, "failed to update participant attendance", logging.ErrKey, err)
		return fmt.Errorf("failed to update participant: %w", err)
	}

	slog.DebugContext(ctx, "successfully updated participant attendance and session",
		"participant_uid", participant.UID,
		"email", participant.Email,
		"session_uid", sessionUID,
	)

	return nil
}

// createParticipantFromJoinedEvent creates a new participant record from a participant_joined event
func (s *ZoomWebhookHandler) createParticipantFromJoinedEvent(ctx context.Context, pastMeeting *models.PastMeeting, participant ZoomPayloadForParticipant) (*models.PastMeetingParticipant, error) {
	slog.DebugContext(ctx, "creating participant from joined event",
		"past_meeting_uid", pastMeeting.UID,
		"participant_email", participant.Email,
		"participant_name", participant.UserName,
	)

	// Create session for joined event
	session := models.ParticipantSession{
		UID:      participant.ID,
		JoinTime: participant.JoinTime,
		// LeaveTime will be set when participant leaves
	}

	return s.createParticipantRecord(ctx, pastMeeting, participant, session)
}

// parseNameFromUserName attempts to parse first and last name from a display name
func parseNameFromUserName(userName string) (firstName, lastName string) {
	if userName == "" {
		return "", ""
	}

	parts := strings.Fields(strings.TrimSpace(userName))
	if len(parts) == 0 {
		return "", ""
	}

	if len(parts) == 1 {
		return parts[0], ""
	}

	// First part is first name, everything else is last name
	firstName = parts[0]
	lastName = strings.Join(parts[1:], " ")
	return firstName, lastName
}

// updateParticipantSessionLeaveTime updates a participant's session with the leave time and reason
func (s *ZoomWebhookHandler) updateParticipantSessionLeaveTime(ctx context.Context, participant *models.PastMeetingParticipant, sessionUID string, leaveTime time.Time, leaveReason string) error {
	slog.DebugContext(ctx, "updating participant session leave time",
		"participant_uid", participant.UID,
		"email", participant.Email,
		"session_uid", sessionUID,
		"leave_time", leaveTime,
	)

	// Find the session with matching UID and update its leave time
	sessionFound := false
	for i := range participant.Sessions {
		if participant.Sessions[i].UID == sessionUID {
			participant.Sessions[i].LeaveTime = &leaveTime
			participant.Sessions[i].LeaveReason = leaveReason
			sessionFound = true

			// Calculate duration if we have both join and leave times
			duration := leaveTime.Sub(participant.Sessions[i].JoinTime)
			slog.DebugContext(ctx, "found and updated session leave time",
				"session_uid", sessionUID,
				"join_time", participant.Sessions[i].JoinTime,
				"leave_time", leaveTime,
				"duration", duration,
			)
			break
		}
	}

	if !sessionFound {
		// Session not found - this could happen if we missed the join event
		// Create a session with just the leave time (we'll estimate join time if possible)
		slog.WarnContext(ctx, "session not found for participant, creating new session with leave time",
			"participant_uid", participant.UID,
			"session_uid", sessionUID,
		)

		newSession := models.ParticipantSession{
			UID:         sessionUID,
			LeaveTime:   &leaveTime,
			LeaveReason: leaveReason,
			// JoinTime will be zero value - we don't have this information
			// Could potentially be estimated from duration if provided
		}
		participant.Sessions = append(participant.Sessions, newSession)
	}

	// Get current revision for update
	_, revision, err := s.pastMeetingParticipantService.PastMeetingParticipantRepository.GetWithRevision(ctx, participant.UID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get participant revision for session update", logging.ErrKey, err)
		return fmt.Errorf("failed to get participant revision: %w", err)
	}

	// Update the participant record
	err = s.pastMeetingParticipantService.PastMeetingParticipantRepository.Update(ctx, participant, revision)
	if err != nil {
		slog.ErrorContext(ctx, "failed to update participant session", logging.ErrKey, err)
		return fmt.Errorf("failed to update participant: %w", err)
	}

	slog.DebugContext(ctx, "successfully updated participant session leave time",
		"participant_uid", participant.UID,
		"session_uid", sessionUID,
	)

	return nil
}

// createParticipantFromLeftEvent creates a new participant record from a participant_left event
func (s *ZoomWebhookHandler) createParticipantFromLeftEvent(ctx context.Context, pastMeeting *models.PastMeeting, participant ZoomPayloadForParticipant) (*models.PastMeetingParticipant, error) {
	slog.DebugContext(ctx, "creating participant from left event",
		"past_meeting_uid", pastMeeting.UID,
		"participant_email", participant.Email,
		"participant_name", participant.UserName,
		"join_time", participant.JoinTime,
		"leave_time", participant.LeaveTime,
	)

	session := models.ParticipantSession{
		UID:         participant.ID,
		JoinTime:    participant.JoinTime,
		LeaveTime:   &participant.LeaveTime,
		LeaveReason: participant.LeaveReason,
	}

	return s.createParticipantRecord(ctx, pastMeeting, participant, session)
}

// createParticipantRecord creates a participant record with the given session data
func (s *ZoomWebhookHandler) createParticipantRecord(ctx context.Context, pastMeeting *models.PastMeeting, participant ZoomPayloadForParticipant, session models.ParticipantSession) (*models.PastMeetingParticipant, error) {
	// Check if this participant was invited (has a registrant record for this meeting)
	var matchingRegistrant *models.Registrant
	registrants, err := s.registrantService.RegistrantRepository.ListByEmail(ctx, participant.Email)
	if err != nil {
		slog.WarnContext(ctx, "could not check registrant records for participant",
			logging.ErrKey, err,
			"participant_email", participant.Email,
		)
		// Continue without failing - we'll treat as non-invited
	} else {
		// Check if any registrant is for this meeting
		for _, registrant := range registrants {
			if registrant.MeetingUID == pastMeeting.MeetingUID {
				matchingRegistrant = registrant
				slog.DebugContext(ctx, "participant was invited (found registrant record)",
					"participant_email", participant.Email,
					"registrant_uid", registrant.UID,
				)
				break
			}
		}
	}

	// Create new participant record based on whether they're a registrant
	var newParticipant *models.PastMeetingParticipant

	if matchingRegistrant != nil {
		// Use registrant information for accurate data
		slog.DebugContext(ctx, "creating participant from registrant data",
			"registrant_uid", matchingRegistrant.UID,
			"email", matchingRegistrant.Email,
		)

		newParticipant = &models.PastMeetingParticipant{
			UID:                uuid.New().String(),
			PastMeetingUID:     pastMeeting.UID,
			MeetingUID:         pastMeeting.MeetingUID,
			Email:              participant.Email,
			FirstName:          matchingRegistrant.FirstName,
			LastName:           matchingRegistrant.LastName,
			JobTitle:           matchingRegistrant.JobTitle,
			OrgName:            matchingRegistrant.OrgName,
			OrgIsMember:        matchingRegistrant.OrgIsMember,
			OrgIsProjectMember: matchingRegistrant.OrgIsProjectMember,
			AvatarURL:          matchingRegistrant.AvatarURL,
			Username:           matchingRegistrant.Username,
			Host:               matchingRegistrant.Host,
			IsInvited:          true,
			IsAttended:         true,
			Sessions:           []models.ParticipantSession{session},
		}
	} else {
		// Parse name from Zoom data for non-registered participants
		firstName, lastName := parseNameFromUserName(participant.UserName)
		slog.DebugContext(ctx, "creating participant from Zoom data (not registered)",
			"zoom_username", participant.UserName,
			"parsed_first_name", firstName,
			"parsed_last_name", lastName,
		)

		newParticipant = &models.PastMeetingParticipant{
			UID:            uuid.New().String(),
			PastMeetingUID: pastMeeting.UID,
			MeetingUID:     pastMeeting.MeetingUID,
			Email:          participant.Email,
			FirstName:      firstName,
			LastName:       lastName,
			IsInvited:      false,
			IsAttended:     true,
			Sessions:       []models.ParticipantSession{session},
		}
	}

	// Create the participant record
	err = s.pastMeetingParticipantService.PastMeetingParticipantRepository.Create(ctx, newParticipant)
	if err != nil {
		slog.ErrorContext(ctx, "failed to create participant record",
			logging.ErrKey, err,
		)
		return nil, fmt.Errorf("failed to create participant: %w", err)
	}

	// Log success with appropriate details
	logFields := []any{
		"participant_uid", newParticipant.UID,
		"email", participant.Email,
		"is_invited", newParticipant.IsInvited,
	}

	// Add session duration if we have both join and leave times
	if session.LeaveTime != nil && !session.JoinTime.IsZero() {
		duration := session.LeaveTime.Sub(session.JoinTime)
		logFields = append(logFields, "session_duration", duration)
	}

	slog.DebugContext(ctx, "successfully created participant record", logFields...)

	return newParticipant, nil
}
