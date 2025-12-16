// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/service"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/concurrent"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/redaction"
)

// ZoomWebhookHandler handles Zoom webhook events.
type ZoomWebhookHandler struct {
	meetingService                *service.MeetingService
	registrantService             *service.MeetingRegistrantService
	pastMeetingService            *service.PastMeetingService
	pastMeetingParticipantService *service.PastMeetingParticipantService
	pastMeetingRecordingService   *service.PastMeetingRecordingService
	pastMeetingTranscriptService  *service.PastMeetingTranscriptService
	pastMeetingSummaryService     *service.PastMeetingSummaryService
	occurrenceService             domain.OccurrenceService
	WebhookValidator              domain.WebhookValidator
}

func NewZoomWebhookHandler(
	meetingService *service.MeetingService,
	registrantService *service.MeetingRegistrantService,
	pastMeetingService *service.PastMeetingService,
	pastMeetingParticipantService *service.PastMeetingParticipantService,
	pastMeetingRecordingService *service.PastMeetingRecordingService,
	pastMeetingTranscriptService *service.PastMeetingTranscriptService,
	pastMeetingSummaryService *service.PastMeetingSummaryService,
	occurrenceService domain.OccurrenceService,
	webhookValidator domain.WebhookValidator,
) *ZoomWebhookHandler {
	return &ZoomWebhookHandler{
		meetingService:                meetingService,
		registrantService:             registrantService,
		pastMeetingService:            pastMeetingService,
		pastMeetingParticipantService: pastMeetingParticipantService,
		pastMeetingRecordingService:   pastMeetingRecordingService,
		pastMeetingTranscriptService:  pastMeetingTranscriptService,
		pastMeetingSummaryService:     pastMeetingSummaryService,
		occurrenceService:             occurrenceService,
		WebhookValidator:              webhookValidator,
	}
}

func (s *ZoomWebhookHandler) HandlerReady() bool {
	return s.meetingService.ServiceReady() &&
		s.registrantService.ServiceReady() &&
		s.pastMeetingService.ServiceReady() &&
		s.pastMeetingParticipantService.ServiceReady() &&
		s.pastMeetingRecordingService.ServiceReady() &&
		s.pastMeetingTranscriptService.ServiceReady() &&
		s.pastMeetingSummaryService.ServiceReady()
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
	ParticipantUUID   string
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
	meeting, err := s.meetingService.GetMeetingByPlatformMeetingID(ctx, models.PlatformZoom, meetingObj.ID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to find meeting by Zoom meeting ID", logging.ErrKey, err)
		return fmt.Errorf("meeting not found for Zoom meeting ID %s: %w", meetingObj.ID, err)
	}

	// Calculate the closest occurrence ID based on the actual start time
	occurrenceID := s.findClosestOccurrenceID(ctx, meeting, meetingObj.StartTime)

	// Check if a past meeting already exists for this occurrence
	pastMeeting, err := s.pastMeetingService.GetByPlatformMeetingIDAndOccurrence(ctx, models.PlatformZoom, meetingObj.ID, occurrenceID)
	if err != nil && domain.GetErrorType(err) != domain.ErrorTypeNotFound {
		slog.ErrorContext(ctx, "failed to check for existing past meeting", logging.ErrKey, err)
		return fmt.Errorf("failed to check for existing past meeting: %w", err)
	}

	if pastMeeting == nil {
		slog.DebugContext(ctx, "creating new past meeting record",
			"meeting_uid", meeting.UID,
			"occurrence_id", occurrenceID,
			"zoom_meeting_id", meetingObj.ID,
			"session_uid", meetingObj.UUID,
		)

		zoomData := ZoomPayloadForPastMeeting{
			UUID:      meetingObj.UUID,
			StartTime: meetingObj.StartTime,
			EndTime:   nil, // Will be set when meeting ends
			Timezone:  meetingObj.Timezone,
		}

		newPastMeeting, err := s.createPastMeetingRecordWithSession(ctx, meeting, zoomData)
		if err != nil {
			slog.ErrorContext(ctx, "failed to create new past meeting record", logging.ErrKey, err)
			return fmt.Errorf("failed to create new past meeting record: %w", err)
		}

		err = s.createPastMeetingParticipants(ctx, newPastMeeting, meeting)
		if err != nil {
			// Log the error but don't fail the entire webhook processing
			slog.ErrorContext(ctx, "failed to create past meeting participants",
				logging.ErrKey, err,
				logging.PriorityCritical(),
			)
		}

		return nil
	}

	newSession := models.Session{
		UID:       meetingObj.UUID,
		StartTime: meetingObj.StartTime,
		EndTime:   nil, // Will be set when meeting ends
	}

	slog.DebugContext(ctx, "found existing past meeting, managing sessions",
		"past_meeting_uid", pastMeeting.UID,
		"occurrence_id", occurrenceID,
		"new_session_uid", newSession.UID,
		"existing_sessions_count", len(pastMeeting.Sessions),
	)

	// Add or update the session using helper function
	updatedSessions := s.addOrUpdateSession(pastMeeting.Sessions, newSession)
	pastMeeting.Sessions = updatedSessions

	// Update the past meeting with the new/updated session
	_, revision, err := s.pastMeetingService.GetPastMeeting(ctx, pastMeeting.UID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get past meeting revision", logging.ErrKey, err)
		return fmt.Errorf("failed to get past meeting revision: %w", err)
	}

	revisionUint, err := strconv.ParseUint(revision, 10, 64)
	if err != nil {
		slog.ErrorContext(ctx, "failed to parse past meeting revision", logging.ErrKey, err)
		return fmt.Errorf("failed to parse past meeting revision: %w", err)
	}

	err = s.pastMeetingService.UpdatePastMeeting(ctx, pastMeeting, revisionUint)
	if err != nil {
		slog.ErrorContext(ctx, "failed to update past meeting with new session", logging.ErrKey, err)
		return fmt.Errorf("failed to update past meeting with new session: %w", err)
	}

	slog.DebugContext(ctx, "successfully updated past meeting with new session",
		"past_meeting_uid", pastMeeting.UID,
		"session_uid", newSession.UID,
		"total_sessions", len(pastMeeting.Sessions),
	)

	return nil
}

// addOrUpdateSession adds a new session to the list or updates an existing session with the same UID.
// It loops through ALL sessions to check for duplicates (not just the last one).
func (s *ZoomWebhookHandler) addOrUpdateSession(existingSessions []models.Session, newSession models.Session) []models.Session {
	// Check all existing sessions for a matching UID
	for i, session := range existingSessions {
		if session.UID == newSession.UID {
			// Found duplicate UID - overwrite the existing session
			existingSessions[i] = newSession
			return existingSessions
		}
	}

	// No duplicate found - append the new session
	return append(existingSessions, newSession)
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

	// First, get the meeting from database to calculate the correct occurrence ID
	meeting, err := s.meetingService.GetMeetingByPlatformMeetingID(ctx, models.PlatformZoom, meetingObj.ID)
	if err != nil {
		if domain.GetErrorType(err) == domain.ErrorTypeNotFound {
			slog.WarnContext(ctx, "meeting not found in database for ended event, skipping",
				"zoom_meeting_id", meetingObj.ID,
			)
			return nil
		}
		slog.ErrorContext(ctx, "error getting meeting from database", logging.ErrKey, err)
		return fmt.Errorf("failed to get meeting: %w", err)
	}

	// Calculate the closest occurrence ID based on the actual start time
	occurrenceID := s.findClosestOccurrenceID(ctx, meeting, meetingObj.StartTime)

	// Try to find existing PastMeeting record by platform meeting ID and occurrence ID
	existingPastMeeting, err := s.pastMeetingService.GetByPlatformMeetingIDAndOccurrence(ctx, models.PlatformZoom, meetingObj.ID, occurrenceID)
	if err != nil && domain.GetErrorType(err) != domain.ErrorTypeNotFound {
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
			"occurrence_id", occurrenceID,
			"zoom_meeting_id", meetingObj.ID,
			"zoom_meeting_uuid", meetingObj.UUID,
		)

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
		"participant_id", participant.ParticipantUUID,
		"participant_name", participant.UserName,
		"participant_email", redaction.RedactEmail(participant.Email),
		"join_time", participant.JoinTime,
	)

	// First, get the meeting from database to calculate the correct occurrence ID
	meeting, err := s.meetingService.GetMeetingByPlatformMeetingID(ctx, models.PlatformZoom, meetingObj.ID)
	if err != nil {
		if domain.GetErrorType(err) == domain.ErrorTypeNotFound {
			slog.WarnContext(ctx, "meeting not found in database for participant joined event, skipping",
				"zoom_meeting_id", meetingObj.ID,
			)
			return nil
		}
		slog.ErrorContext(ctx, "error getting meeting from database", logging.ErrKey, err)
		return fmt.Errorf("failed to get meeting: %w", err)
	}

	// Calculate the closest occurrence ID based on the actual start time
	occurrenceID := s.findClosestOccurrenceID(ctx, meeting, meetingObj.StartTime)

	// Find the PastMeeting record by platform meeting ID and occurrence ID
	pastMeeting, err := s.pastMeetingService.GetByPlatformMeetingIDAndOccurrence(ctx, models.PlatformZoom, meetingObj.ID, occurrenceID)
	if err != nil {
		if domain.GetErrorType(err) == domain.ErrorTypeNotFound {
			slog.WarnContext(ctx, "no past meeting found for participant joined event, skipping",
				"zoom_meeting_id", meetingObj.ID,
				"participant_email", redaction.RedactEmail(participant.Email),
			)
			return nil // Don't fail the webhook processing
		}
		slog.ErrorContext(ctx, "error searching for past meeting", logging.ErrKey, err)
		return fmt.Errorf("failed to search for past meeting: %w", err)
	}

	// Try to find existing participant by email first, then by name
	existingParticipant, err := s.findExistingParticipant(ctx, pastMeeting.UID, participant.Email, participant.UserName)
	if err != nil {
		return fmt.Errorf("failed to find existing participant: %w", err)
	}

	if existingParticipant != nil {
		// Update existing participant to mark as attended and add new session
		err = s.updateParticipantAttendance(ctx, existingParticipant, participant.ParticipantUUID, participant.JoinTime)
		if err != nil {
			return fmt.Errorf("failed to update participant attendance: %w", err)
		}
		slog.InfoContext(ctx, "updated existing participant attendance",
			"past_meeting_uid", pastMeeting.UID,
			"participant_uid", existingParticipant.UID,
			"email", redaction.RedactEmail(participant.Email),
			"session_uid", participant.ParticipantUUID,
		)
		return nil
	}

	slog.DebugContext(ctx, "no existing participant found, creating new participant",
		"participant_email", redaction.RedactEmail(participant.Email),
		"zoom_meeting_id", meetingObj.ID,
	)

	// Create new participant record
	zoomParticipant := ZoomPayloadForParticipant{
		UserID:            participant.UserID,
		UserName:          participant.UserName,
		ParticipantUUID:   participant.ParticipantUUID,
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
		"past_meeting_uid", pastMeeting.UID,
		"participant_uid", newParticipant.UID,
		"email", redaction.RedactEmail(participant.Email),
		"is_invited", newParticipant.IsInvited,
	)

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
		"participant_id", participant.ParticipantUUID,
		"participant_name", participant.UserName,
		"participant_email", redaction.RedactEmail(participant.Email),
		"leave_time", participant.LeaveTime,
		"duration", participant.Duration,
	)

	// First, get the meeting from database to calculate the correct occurrence ID
	meeting, err := s.meetingService.GetMeetingByPlatformMeetingID(ctx, models.PlatformZoom, meetingObj.ID)
	if err != nil {
		if domain.GetErrorType(err) == domain.ErrorTypeNotFound {
			slog.WarnContext(ctx, "meeting not found in database for participant left event, skipping",
				"zoom_meeting_id", meetingObj.ID,
			)
			return nil
		}
		slog.ErrorContext(ctx, "error getting meeting from database", logging.ErrKey, err)
		return fmt.Errorf("failed to get meeting: %w", err)
	}

	// Calculate the closest occurrence ID based on the actual start time
	occurrenceID := s.findClosestOccurrenceID(ctx, meeting, meetingObj.StartTime)

	// Find the PastMeeting record by platform meeting ID and occurrence ID
	pastMeeting, err := s.pastMeetingService.GetByPlatformMeetingIDAndOccurrence(ctx, models.PlatformZoom, meetingObj.ID, occurrenceID)
	if err != nil {
		if domain.GetErrorType(err) == domain.ErrorTypeNotFound {
			slog.WarnContext(ctx, "no past meeting found for participant left event, skipping",
				"zoom_meeting_id", meetingObj.ID,
				"participant_email", redaction.RedactEmail(participant.Email),
			)
			return nil // Don't fail the webhook processing
		}
		slog.ErrorContext(ctx, "error searching for past meeting", logging.ErrKey, err)
		return fmt.Errorf("failed to search for past meeting: %w", err)
	}

	// Try to find existing participant by email first, then by name
	existingParticipant, err := s.findExistingParticipant(ctx, pastMeeting.UID, participant.Email, participant.UserName)
	if err != nil {
		return fmt.Errorf("failed to find existing participant: %w", err)
	}

	if existingParticipant != nil {
		// Update existing participant's session with leave time
		err = s.updateParticipantSessionLeaveTime(ctx, existingParticipant, participant.ParticipantUUID, participant.LeaveTime, participant.LeaveReason)
		if err != nil {
			return fmt.Errorf("failed to update participant session leave time: %w", err)
		}
		slog.InfoContext(ctx, "updated participant session leave time",
			"past_meeting_uid", pastMeeting.UID,
			"participant_uid", existingParticipant.UID,
			"email", redaction.RedactEmail(participant.Email),
			"session_uid", participant.ParticipantUUID,
			"duration", participant.Duration,
		)
		return nil
	}

	// Create new participant record with completed session (they joined and left but we missed the joined event)
	slog.WarnContext(ctx, "no existing participant found for left event, creating new record",
		"participant_email", redaction.RedactEmail(participant.Email),
		"zoom_meeting_id", meetingObj.ID,
	)

	zoomParticipant := ZoomPayloadForParticipant{
		UserID:            participant.UserID,
		UserName:          participant.UserName,
		ParticipantUUID:   participant.ParticipantUUID,
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
		"past_meeting_uid", pastMeeting.UID,
		"participant_uid", newParticipant.UID,
		"email", redaction.RedactEmail(participant.Email),
		"calculated_join_time", zoomParticipant.JoinTime,
		"leave_time", participant.LeaveTime,
	)

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
		"start_time", payload.Object.StartTime,
		"total_size", payload.Object.TotalSize,
		"recording_count", payload.Object.RecordingCount,
	)

	// Get the base meeting to calculate the occurrence ID
	meetingIDStr := strconv.FormatInt(payload.Object.ID, 10)
	meeting, err := s.meetingService.GetMeetingByPlatformMeetingID(ctx, models.PlatformZoom, meetingIDStr)
	if err != nil {
		if domain.GetErrorType(err) == domain.ErrorTypeNotFound {
			slog.WarnContext(ctx, "no meeting found for recording, cannot determine occurrence",
				"zoom_meeting_id", payload.Object.ID,
			)
			return nil
		}
		slog.ErrorContext(ctx, "error finding meeting for recording", logging.ErrKey, err,
			"zoom_meeting_id", payload.Object.ID,
		)
		return fmt.Errorf("failed to find meeting: %w", err)
	}

	// Calculate the occurrence ID based on the recording start time
	occurrenceID := s.findClosestOccurrenceID(ctx, meeting, payload.Object.StartTime)

	// Find the PastMeeting record by platform meeting ID AND occurrence ID
	pastMeeting, err := s.pastMeetingService.GetByPlatformMeetingIDAndOccurrence(ctx, models.PlatformZoom, meetingIDStr, occurrenceID)
	if err != nil {
		if domain.GetErrorType(err) == domain.ErrorTypeNotFound {
			slog.WarnContext(ctx, "no past meeting found for recording completed event, skipping",
				"zoom_meeting_id", payload.Object.ID,
				"zoom_meeting_uuid", payload.Object.UUID,
				"occurrence_id", occurrenceID,
				"topic", payload.Object.Topic,
			)
			return nil // Not an error - we just don't have a past meeting record yet
		}
		slog.ErrorContext(ctx, "error finding past meeting for recording", logging.ErrKey, err,
			"zoom_meeting_id", payload.Object.ID,
			"occurrence_id", occurrenceID,
		)
		return fmt.Errorf("failed to find past meeting: %w", err)
	}

	// Check if a recording already exists for this Zoom UUID (for idempotency)
	existingRecording, err := s.pastMeetingRecordingService.GetRecordingByPlatformMeetingInstanceID(ctx, models.PlatformZoom, payload.Object.UUID)
	if err != nil && domain.GetErrorType(err) != domain.ErrorTypeNotFound {
		slog.ErrorContext(ctx, "error checking for existing recording by instance ID", logging.ErrKey, err,
			"zoom_meeting_uuid", payload.Object.UUID,
		)
		return fmt.Errorf("failed to check for existing recording: %w", err)
	}

	// If recording already exists for this Zoom UUID, skip creation (idempotent)
	if existingRecording != nil {
		slog.InfoContext(ctx, "recording already exists for this Zoom UUID, skipping creation",
			"recording_uid", existingRecording.UID,
			"past_meeting_uid", pastMeeting.UID,
			"zoom_meeting_id", payload.Object.ID,
			"zoom_meeting_uuid", payload.Object.UUID,
		)
		return nil
	}

	// Create domain model from Zoom payload - each Zoom UUID gets its own recording
	recordingFromPayload := s.createRecordingFromZoomPayload(pastMeeting.UID, *payload)

	// Create new recording for this Zoom UUID
	recording, err := s.pastMeetingRecordingService.CreateRecording(ctx, recordingFromPayload)
	if err != nil {
		slog.ErrorContext(ctx, "error creating recording", logging.ErrKey, err,
			"past_meeting_uid", pastMeeting.UID,
			"zoom_meeting_id", payload.Object.ID,
			"zoom_meeting_uuid", payload.Object.UUID,
		)
		return fmt.Errorf("failed to create recording: %w", err)
	}
	slog.InfoContext(ctx, "successfully created new recording",
		"recording_uid", recording.UID,
		"past_meeting_uid", pastMeeting.UID,
		"zoom_meeting_id", payload.Object.ID,
		"zoom_meeting_uuid", payload.Object.UUID,
		"total_files", len(recording.RecordingFiles),
		"total_size", recording.TotalSize,
	)

	// Add recording UID to past meeting's RecordingUIDs array
	if !slices.Contains(pastMeeting.RecordingUIDs, recording.UID) {
		// Get the latest version with revision to update
		updatedPastMeeting, revision, err := s.pastMeetingService.GetPastMeeting(ctx, pastMeeting.UID)
		if err != nil {
			slog.WarnContext(ctx, "failed to get past meeting for recording UID update", logging.ErrKey, err,
				"past_meeting_uid", pastMeeting.UID,
			)
		} else {
			updatedPastMeeting.RecordingUIDs = append(updatedPastMeeting.RecordingUIDs, recording.UID)
			revisionUint, _ := strconv.ParseUint(revision, 10, 64)
			if err := s.pastMeetingService.UpdatePastMeeting(ctx, updatedPastMeeting, revisionUint); err != nil {
				slog.WarnContext(ctx, "failed to update past meeting with recording UID", logging.ErrKey, err,
					"past_meeting_uid", pastMeeting.UID,
					"recording_uid", recording.UID,
				)
				// Don't fail the entire operation if this update fails
			}
		}
	}

	return nil
}

// createRecordingFromZoomPayload converts a Zoom recording completed payload to a domain model.
func (s *ZoomWebhookHandler) createRecordingFromZoomPayload(pastMeetingUID string, payload models.ZoomRecordingCompletedPayload) *models.PastMeetingRecording {
	recording := &models.PastMeetingRecording{
		PastMeetingUID:            pastMeetingUID,
		Platform:                  models.PlatformZoom,
		PlatformMeetingID:         strconv.FormatInt(payload.Object.ID, 10),
		PlatformMeetingInstanceID: payload.Object.UUID, // Zoom meeting UUID uniquely identifies this recording instance
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
	// Exclude transcript files as they are now handled by the transcript service
	var recordingFiles []models.RecordingFileData
	for _, zoomFile := range payload.Object.RecordingFiles {
		// Skip transcript-related files - these are handled by the transcript service
		if zoomFile.FileType == "TRANSCRIPT" || zoomFile.FileType == "TIMELINE" {
			continue
		}

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

	recording.AddRecordingFiles(recordingFiles)

	return recording
}

// createTranscriptFromZoomPayload converts a Zoom transcript completed payload to a transcript domain model.
func (s *ZoomWebhookHandler) createTranscriptFromZoomPayload(pastMeetingUID string, payload models.ZoomTranscriptCompletedPayload) *models.PastMeetingTranscript {
	transcript := &models.PastMeetingTranscript{
		PastMeetingUID:            pastMeetingUID,
		Platform:                  models.PlatformZoom,
		PlatformMeetingID:         strconv.FormatInt(payload.Object.ID, 10),
		PlatformMeetingInstanceID: payload.Object.UUID, // Zoom meeting UUID uniquely identifies this transcript instance
	}

	session := models.TranscriptSession{
		UUID:      payload.Object.UUID,
		ShareURL:  payload.Object.ShareURL,
		TotalSize: payload.Object.TotalSize,
		StartTime: payload.Object.StartTime,
	}
	transcript.Sessions = append(transcript.Sessions, session)

	// Convert Zoom transcript files to domain model format - only transcript files
	var transcriptFiles []models.TranscriptFileData
	for _, zoomFile := range payload.Object.RecordingFiles {
		// Only process transcript-related files
		if zoomFile.FileType == "TRANSCRIPT" || zoomFile.FileType == "TIMELINE" {
			transcriptFiles = append(transcriptFiles, models.TranscriptFileData{
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
	}

	transcript.TranscriptFiles = transcriptFiles
	transcript.TranscriptCount = len(transcriptFiles)
	transcript.TotalSize = 0
	for _, file := range transcriptFiles {
		transcript.TotalSize += file.FileSize
	}

	return transcript
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
		"start_time", payload.Object.StartTime,
		"duration", payload.Object.Duration,
		"recording_files", len(payload.Object.RecordingFiles),
	)

	// Get the base meeting to calculate the occurrence ID
	meetingIDStr := strconv.FormatInt(payload.Object.ID, 10)
	meeting, err := s.meetingService.GetMeetingByPlatformMeetingID(ctx, models.PlatformZoom, meetingIDStr)
	if err != nil {
		if domain.GetErrorType(err) == domain.ErrorTypeNotFound {
			slog.WarnContext(ctx, "no meeting found for transcript, cannot determine occurrence",
				"zoom_meeting_id", payload.Object.ID,
			)
			return nil
		}
		slog.ErrorContext(ctx, "error finding meeting for transcript", logging.ErrKey, err,
			"zoom_meeting_id", payload.Object.ID,
		)
		return fmt.Errorf("failed to find meeting: %w", err)
	}

	// Calculate the occurrence ID based on the transcript start time
	occurrenceID := s.findClosestOccurrenceID(ctx, meeting, payload.Object.StartTime)

	// Find the PastMeeting record by platform meeting ID AND occurrence ID
	pastMeeting, err := s.pastMeetingService.GetByPlatformMeetingIDAndOccurrence(ctx, models.PlatformZoom, meetingIDStr, occurrenceID)
	if err != nil {
		if domain.GetErrorType(err) == domain.ErrorTypeNotFound {
			slog.WarnContext(ctx, "no past meeting found for transcript completed event, skipping",
				"zoom_meeting_id", payload.Object.ID,
				"zoom_meeting_uuid", payload.Object.UUID,
				"occurrence_id", occurrenceID,
				"topic", payload.Object.Topic,
			)
			return nil // Not an error - we just don't have a past meeting record yet
		}
		slog.ErrorContext(ctx, "error finding past meeting for transcript", logging.ErrKey, err,
			"zoom_meeting_id", payload.Object.ID,
			"occurrence_id", occurrenceID,
		)
		return fmt.Errorf("failed to find past meeting: %w", err)
	}

	// Check if a transcript already exists for this Zoom UUID (for idempotency)
	existingTranscript, err := s.pastMeetingTranscriptService.GetTranscriptByPlatformMeetingInstanceID(ctx, models.PlatformZoom, payload.Object.UUID)
	if err != nil && domain.GetErrorType(err) != domain.ErrorTypeNotFound {
		slog.ErrorContext(ctx, "error checking for existing transcript by instance ID", logging.ErrKey, err,
			"zoom_meeting_uuid", payload.Object.UUID,
		)
		return fmt.Errorf("failed to check for existing transcript: %w", err)
	}

	// If transcript already exists for this Zoom UUID, skip creation (idempotent)
	if existingTranscript != nil {
		slog.InfoContext(ctx, "transcript already exists for this Zoom UUID, skipping creation",
			"transcript_uid", existingTranscript.UID,
			"past_meeting_uid", pastMeeting.UID,
			"zoom_meeting_id", payload.Object.ID,
			"zoom_meeting_uuid", payload.Object.UUID,
		)
		return nil
	}

	// Create domain model from Zoom transcript payload - each Zoom UUID gets its own transcript
	transcriptFromPayload := s.createTranscriptFromZoomPayload(pastMeeting.UID, *payload)

	// Create new transcript for this Zoom UUID
	transcript, err := s.pastMeetingTranscriptService.CreateTranscript(ctx, transcriptFromPayload)
	if err != nil {
		slog.ErrorContext(ctx, "error creating transcript", logging.ErrKey, err,
			"past_meeting_uid", pastMeeting.UID,
			"zoom_meeting_id", payload.Object.ID,
			"zoom_meeting_uuid", payload.Object.UUID,
		)
		return fmt.Errorf("failed to create transcript: %w", err)
	}
	slog.InfoContext(ctx, "successfully created transcript with files",
		"transcript_uid", transcript.UID,
		"past_meeting_uid", pastMeeting.UID,
		"zoom_meeting_id", payload.Object.ID,
		"zoom_meeting_uuid", payload.Object.UUID,
		"total_files", len(transcript.TranscriptFiles),
		"total_size", transcript.TotalSize,
	)

	// Add transcript UID to past meeting's TranscriptUIDs array
	if !slices.Contains(pastMeeting.TranscriptUIDs, transcript.UID) {
		// Get the latest version with revision to update
		updatedPastMeeting, revision, err := s.pastMeetingService.GetPastMeeting(ctx, pastMeeting.UID)
		if err != nil {
			slog.WarnContext(ctx, "failed to get past meeting for transcript UID update", logging.ErrKey, err,
				"past_meeting_uid", pastMeeting.UID,
			)
		} else {
			updatedPastMeeting.TranscriptUIDs = append(updatedPastMeeting.TranscriptUIDs, transcript.UID)
			revisionUint, _ := strconv.ParseUint(revision, 10, 64)
			if err := s.pastMeetingService.UpdatePastMeeting(ctx, updatedPastMeeting, revisionUint); err != nil {
				slog.WarnContext(ctx, "failed to update past meeting with transcript UID", logging.ErrKey, err,
					"past_meeting_uid", pastMeeting.UID,
					"transcript_uid", transcript.UID,
				)
				// Don't fail the entire operation if this update fails
			}
		}
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
		"zoom_meeting_uuid", payload.Object.MeetingUUID,
		"zoom_meeting_id", payload.Object.MeetingID,
		"topic", payload.Object.MeetingTopic,
		"start_time", payload.Object.MeetingStartTime,
		"end_time", payload.Object.MeetingEndTime,
	)

	// Find the meeting by Zoom meeting ID first to get occurrence information
	zoomMeetingID := fmt.Sprintf("%d", payload.Object.MeetingID)
	meeting, err := s.meetingService.GetMeetingByPlatformMeetingID(ctx, models.PlatformZoom, zoomMeetingID)
	if err != nil {
		slog.WarnContext(ctx, "failed to find meeting by Zoom ID for summary",
			"zoom_meeting_id", zoomMeetingID,
			"zoom_meeting_uuid", payload.Object.MeetingUUID,
			"error", err,
		)
		// Don't fail the webhook if no meeting is found - it might have been deleted
		return nil
	}

	// Calculate the closest occurrence ID based on the meeting start time
	occurrenceID := s.findClosestOccurrenceID(ctx, meeting, payload.Object.MeetingStartTime)

	// Find the past meeting by platform meeting ID AND occurrence ID
	pastMeeting, err := s.pastMeetingService.GetByPlatformMeetingIDAndOccurrence(ctx, models.PlatformZoom, zoomMeetingID, occurrenceID)
	if err != nil {
		if domain.GetErrorType(err) == domain.ErrorTypeNotFound {
			slog.WarnContext(ctx, "no past meeting found for summary",
				"zoom_meeting_id", zoomMeetingID,
				"zoom_meeting_uuid", payload.Object.MeetingUUID,
				"occurrence_id", occurrenceID,
			)
			return nil // Don't fail the webhook if no past meeting is found
		}
		slog.ErrorContext(ctx, "error finding past meeting for summary", logging.ErrKey, err,
			"zoom_meeting_id", zoomMeetingID,
			"occurrence_id", occurrenceID,
		)
		return fmt.Errorf("failed to find past meeting: %w", err)
	}

	// Check if a summary already exists for this Zoom meeting UUID
	// We need to check all summaries for this past meeting and see if any have the same Zoom UUID
	existingSummaries, err := s.pastMeetingSummaryService.ListSummariesByPastMeeting(ctx, pastMeeting.UID)
	if err != nil {
		slog.ErrorContext(ctx, "error checking for existing summaries", logging.ErrKey, err,
			"past_meeting_uid", pastMeeting.UID,
		)
		return fmt.Errorf("failed to check existing summaries: %w", err)
	}

	// Check if any existing summary has the same Zoom UUID
	for _, existingSummary := range existingSummaries {
		if existingSummary.ZoomConfig != nil && existingSummary.ZoomConfig.MeetingUUID == payload.Object.MeetingUUID {
			slog.InfoContext(ctx, "summary already exists for this Zoom meeting UUID",
				"existing_summary_uid", existingSummary.UID,
				"zoom_meeting_uuid", payload.Object.MeetingUUID,
				"past_meeting_uid", pastMeeting.UID,
			)
			return nil // Summary already exists, nothing to do
		}
	}

	// Create the summary record since none exists for this UUID
	summary := &models.PastMeetingSummary{
		PastMeetingUID: pastMeeting.UID,
		MeetingUID:     pastMeeting.MeetingUID,
		Platform:       models.PlatformZoom,
		Password:       uuid.NewString(),
		ZoomConfig: &models.PastMeetingSummaryZoomConfig{
			MeetingID:   zoomMeetingID,
			MeetingUUID: payload.Object.MeetingUUID,
		},
		SummaryData: models.SummaryData{
			StartTime:     payload.Object.SummaryStartTime,
			EndTime:       payload.Object.SummaryEndTime,
			Title:         payload.Object.SummaryTitle,
			Content:       payload.Object.SummaryContent,
			DocURL:        payload.Object.SummaryDocURL,
			EditedContent: "", // Empty until user edits
		},
		RequiresApproval: pastMeeting.ZoomConfig != nil && pastMeeting.ZoomConfig.AISummaryRequireApproval,
		Approved:         false, // Default to false until manually approved
		EmailSent:        false, // Default to false until emails are sent
	}

	// Create the summary using the service
	createdSummary, err := s.pastMeetingSummaryService.CreateSummary(ctx, summary)
	if err != nil {
		slog.ErrorContext(ctx, "failed to create past meeting summary", logging.ErrKey, err,
			"past_meeting_uid", pastMeeting.UID,
			"zoom_meeting_id", zoomMeetingID,
		)
		return fmt.Errorf("failed to create summary: %w", err)
	}

	slog.InfoContext(ctx, "successfully created past meeting summary",
		"summary_uid", createdSummary.UID,
		"past_meeting_uid", pastMeeting.UID,
		"meeting_uid", pastMeeting.MeetingUID,
		"occurrence_id", occurrenceID,
		"zoom_meeting_id", zoomMeetingID,
		"zoom_meeting_uuid", payload.Object.MeetingUUID,
	)

	// Add summary UID to past meeting's SummaryUIDs array
	if !slices.Contains(pastMeeting.SummaryUIDs, createdSummary.UID) {
		// Get the latest version with revision to update
		updatedPastMeeting, revision, err := s.pastMeetingService.GetPastMeeting(ctx, pastMeeting.UID)
		if err != nil {
			slog.WarnContext(ctx, "failed to get past meeting for summary UID update", logging.ErrKey, err,
				"past_meeting_uid", pastMeeting.UID,
			)
		} else {
			updatedPastMeeting.SummaryUIDs = append(updatedPastMeeting.SummaryUIDs, createdSummary.UID)
			revisionUint, _ := strconv.ParseUint(revision, 10, 64)
			if err := s.pastMeetingService.UpdatePastMeeting(ctx, updatedPastMeeting, revisionUint); err != nil {
				slog.WarnContext(ctx, "failed to update past meeting with summary UID", logging.ErrKey, err,
					"past_meeting_uid", pastMeeting.UID,
					"summary_uid", createdSummary.UID,
				)
				// Don't fail the entire operation if this update fails
			}
		}
	}

	return nil
}

// createPastMeetingParticipants creates participant records for all registrants of a meeting
func (s *ZoomWebhookHandler) createPastMeetingParticipants(ctx context.Context, pastMeeting *models.PastMeeting, meeting *models.MeetingBase) error {
	slog.DebugContext(ctx, "creating past meeting participant records",
		"past_meeting_uid", pastMeeting.UID,
		"meeting_uid", meeting.UID,
	)

	// Get all registrants for this meeting
	registrants, err := s.registrantService.ListMeetingRegistrants(ctx, meeting.UID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get registrants for meeting", logging.ErrKey, err)
		return fmt.Errorf("failed to get registrants: %w", err)
	}

	// Track successful and failed creations with thread-safe counters
	var successCount, failedCount int64
	var mu sync.Mutex
	var failedEmails []string

	tasks := []func() error{}

	// Create PastMeetingParticipant records for all registrants
	for _, registrant := range registrants {
		// Capture registrant in closure
		r := registrant
		tasks = append(tasks, func() error {
			participant := &models.PastMeetingParticipant{
				UID:                uuid.New().String(),
				PastMeetingUID:     pastMeeting.UID,
				MeetingUID:         meeting.UID,
				Email:              r.Email,
				FirstName:          r.FirstName,
				LastName:           r.LastName,
				JobTitle:           r.JobTitle,
				OrgName:            r.OrgName,
				OrgIsMember:        r.OrgIsMember,
				OrgIsProjectMember: r.OrgIsProjectMember,
				AvatarURL:          r.AvatarURL,
				LinkedInProfile:    r.LinkedInProfile,
				Username:           r.Username,
				Host:               r.Host,
				IsInvited:          true,
				IsAttended:         false, // Will be set to true when they join
				// Sessions will be updated when participants join/leave
			}

			_, err := s.pastMeetingParticipantService.CreatePastMeetingParticipant(ctx, participant)

			if err != nil {
				slog.ErrorContext(ctx, "failed to create past meeting participant record",
					logging.ErrKey, err,
					"past_meeting_uid", pastMeeting.UID,
					"meeting_uid", meeting.UID,
					"email", redaction.RedactEmail(r.Email),
				)
				atomic.AddInt64(&failedCount, 1)
				// Use mutex only for slice access
				mu.Lock()
				failedEmails = append(failedEmails, redaction.RedactEmail(r.Email))
				mu.Unlock()
				// Continue creating other participants even if one fails
			} else {
				atomic.AddInt64(&successCount, 1)
			}
			return nil
		})
	}

	errorsWorkerPool := concurrent.NewWorkerPool(10).RunAll(ctx, tasks...)
	if len(errorsWorkerPool) > 0 {
		slog.ErrorContext(ctx, "failed to create some past meeting participant records",
			"errors_count", len(errorsWorkerPool),
			"errors", errorsWorkerPool,
			"meeting_uid", meeting.UID,
			"past_meeting_uid", pastMeeting.UID,
		)
	}

	slog.DebugContext(ctx, "completed creating past meeting participant records",
		"past_meeting_uid", pastMeeting.UID,
		"meeting_uid", meeting.UID,
		"total_registrants", len(registrants),
		"successful", atomic.LoadInt64(&successCount),
		"failed", atomic.LoadInt64(&failedCount),
		"failed_emails", failedEmails,
	)

	// Return error if all creations failed
	if atomic.LoadInt64(&failedCount) > 0 && atomic.LoadInt64(&successCount) == 0 {
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

	_, revision, err := s.pastMeetingService.GetPastMeeting(ctx, pastMeeting.UID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get past meeting revision for update", logging.ErrKey, err)
		return fmt.Errorf("failed to get past meeting revision: %w", err)
	}

	revisionUint, err := strconv.ParseUint(revision, 10, 64)
	if err != nil {
		slog.ErrorContext(ctx, "failed to parse past meeting revision", logging.ErrKey, err)
		return fmt.Errorf("failed to parse past meeting revision: %w", err)
	}

	err = s.pastMeetingService.UpdatePastMeeting(ctx, pastMeeting, revisionUint)
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

// findClosestOccurrenceID finds the occurrence ID closest to the given start time
// For recurring meetings: calculates occurrences around the actual start time and returns the ID of the closest one
// For non-recurring meetings: returns the meeting's scheduled start time as unix timestamp string
func (s *ZoomWebhookHandler) findClosestOccurrenceID(ctx context.Context, meeting *models.MeetingBase, actualStartTime time.Time) string {
	// For non-recurring meetings, use the scheduled start time from the meeting object
	if meeting.Recurrence == nil {
		occurrenceID := strconv.FormatInt(meeting.StartTime.Unix(), 10)
		slog.DebugContext(ctx, "non-recurring meeting, using scheduled start time as unix timestamp",
			"meeting_uid", meeting.UID,
			"occurrence_id", occurrenceID,
			"scheduled_start_time", meeting.StartTime,
			"actual_start_time", actualStartTime,
		)
		return occurrenceID
	}

	// For recurring meetings, use the OccurrenceService to calculate occurrences
	// around the actual start time and find the closest match

	// Calculate search start (look back 1 month from actual start time)
	searchStart := actualStartTime.AddDate(0, -1, 0)

	// Calculate occurrences starting from 1 month before the actual start time
	occurrences := s.occurrenceService.CalculateOccurrencesFromDate(meeting, searchStart, 100)

	// If no occurrences found, fallback to scheduled start time
	if len(occurrences) == 0 {
		occurrenceID := strconv.FormatInt(meeting.StartTime.Unix(), 10)
		slog.DebugContext(ctx, "no occurrences found for recurring meeting, using scheduled start time as unix timestamp",
			"meeting_uid", meeting.UID,
			"occurrence_id", occurrenceID,
			"scheduled_start_time", meeting.StartTime,
			"actual_start_time", actualStartTime,
			"search_start", searchStart,
		)
		return occurrenceID
	}

	// Find the occurrence with the closest start time
	var closestOccurrence *models.Occurrence
	minDiff := time.Duration(math.MaxInt64 - 1)

	for i := range occurrences {
		occ := &occurrences[i]
		if occ.StartTime == nil {
			continue
		}

		// Calculate absolute time difference
		diff := actualStartTime.Sub(*occ.StartTime)
		if diff < 0 {
			diff = -diff
		}

		// Update if this occurrence is closer
		if diff < minDiff {
			minDiff = diff
			closestOccurrence = occ
		}
	}

	// If we found a close occurrence, use its ID
	if closestOccurrence != nil && closestOccurrence.OccurrenceID != "" {
		slog.DebugContext(ctx, "found closest occurrence for recurring meeting using OccurrenceService",
			"meeting_uid", meeting.UID,
			"occurrence_id", closestOccurrence.OccurrenceID,
			"occurrence_start_time", closestOccurrence.StartTime,
			"actual_start_time", actualStartTime,
			"occurrences_searched", len(occurrences),
		)
		return closestOccurrence.OccurrenceID
	}

	// Fallback to scheduled start time unix timestamp if no valid occurrence found
	occurrenceID := strconv.FormatInt(meeting.StartTime.Unix(), 10)
	slog.WarnContext(ctx, "no valid occurrence found for recurring meeting, using scheduled start time as unix timestamp",
		"meeting_uid", meeting.UID,
		"occurrence_id", occurrenceID,
		"scheduled_start_time", meeting.StartTime,
		"actual_start_time", actualStartTime,
		"occurrences_checked", len(occurrences),
	)
	return occurrenceID
}

// createPastMeetingRecordWithSession creates a PastMeeting record with the specified session details
func (s *ZoomWebhookHandler) createPastMeetingRecordWithSession(ctx context.Context, meeting *models.MeetingBase, zoomData ZoomPayloadForPastMeeting) (*models.PastMeeting, error) {
	contextType := "creating past meeting record"
	if zoomData.EndTime != nil {
		contextType = "creating past meeting record for ended event (fallback)"
	}

	// Calculate occurrence ID based on meeting type and occurrences
	occurrenceID := s.findClosestOccurrenceID(ctx, meeting, zoomData.StartTime)

	// Parse occurrence ID to get the scheduled start time for this occurrence
	// The occurrence ID is a unix timestamp string
	scheduledStartTime := meeting.StartTime.UTC() // Default to meeting start time in UTC
	if occurrenceUnix, err := strconv.ParseInt(occurrenceID, 10, 64); err == nil {
		scheduledStartTime = time.Unix(occurrenceUnix, 0).UTC()
	} else {
		slog.WarnContext(ctx, "failed to parse occurrence ID as unix timestamp, using meeting start time",
			"occurrence_id", occurrenceID,
			"error", err,
		)
	}

	// Calculate scheduled end time based on duration from the scheduled start time (will be in UTC)
	scheduledEndTime := scheduledStartTime.Add(time.Duration(meeting.Duration) * time.Minute)

	logFields := []any{
		"meeting_uid", meeting.UID,
		"zoom_meeting_uuid", zoomData.UUID,
		"actual_start_time", zoomData.StartTime,
		"scheduled_start_time", scheduledStartTime,
		"scheduled_end_time", scheduledEndTime,
		"timezone", zoomData.Timezone,
		"occurrence_id", occurrenceID,
		"is_recurring", meeting.Recurrence != nil,
		"occurrences_count", len(meeting.Occurrences),
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
		OccurrenceID:         occurrenceID,
		ProjectUID:           meeting.ProjectUID,
		ScheduledStartTime:   scheduledStartTime, // Scheduled time for this specific occurrence
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
	_, err := s.pastMeetingService.CreatePastMeeting(ctx, pastMeeting)
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
		"email", redaction.RedactEmail(participant.Email),
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
	_, revision, err := s.pastMeetingParticipantService.GetPastMeetingParticipant(ctx, participant.PastMeetingUID, participant.UID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get participant revision for attendance update", logging.ErrKey, err)
		return fmt.Errorf("failed to get participant revision: %w", err)
	}

	revisionUint, err := strconv.ParseUint(revision, 10, 64)
	if err != nil {
		slog.ErrorContext(ctx, "failed to parse participant revision", logging.ErrKey, err)
		return fmt.Errorf("failed to parse participant revision: %w", err)
	}

	// Update the participant record
	_, err = s.pastMeetingParticipantService.UpdatePastMeetingParticipant(ctx, participant, revisionUint)
	if err != nil {
		slog.ErrorContext(ctx, "failed to update participant attendance", logging.ErrKey, err)
		return fmt.Errorf("failed to update participant: %w", err)
	}

	slog.DebugContext(ctx, "successfully updated participant attendance and session",
		"participant_uid", participant.UID,
		"email", redaction.RedactEmail(participant.Email),
		"session_uid", sessionUID,
	)

	return nil
}

// createParticipantFromJoinedEvent creates a new participant record from a participant_joined event
func (s *ZoomWebhookHandler) createParticipantFromJoinedEvent(ctx context.Context, pastMeeting *models.PastMeeting, participant ZoomPayloadForParticipant) (*models.PastMeetingParticipant, error) {
	slog.DebugContext(ctx, "creating participant from joined event",
		"past_meeting_uid", pastMeeting.UID,
		"participant_email", redaction.RedactEmail(participant.Email),
		"participant_name", participant.UserName,
	)

	// Create session for joined event
	session := models.ParticipantSession{
		UID:      participant.ParticipantUUID,
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

// cleanZoomUserName removes organization information in parentheses from Zoom user names
// e.g., "First Last (The Linux Foundation)" becomes "First Last"
func cleanZoomUserName(userName string) string {
	if userName == "" {
		return ""
	}

	// Find the first opening parenthesis
	if idx := strings.Index(userName, "("); idx != -1 {
		// Remove everything from the opening parenthesis onwards
		userName = userName[:idx]
	}

	// Trim any trailing whitespace
	return strings.TrimSpace(userName)
}

// findExistingParticipant finds an existing participant by email first, then by name
// Returns nil if not found (not an error), only returns errors for system failures
func (s *ZoomWebhookHandler) findExistingParticipant(ctx context.Context, pastMeetingUID, email, userName string) (*models.PastMeetingParticipant, error) {
	// Get all participants for this past meeting to search through
	participants, err := s.pastMeetingParticipantService.ListPastMeetingParticipants(ctx, pastMeetingUID)
	if err != nil {
		slog.ErrorContext(ctx, "error getting participants for past meeting", logging.ErrKey, err)
		return nil, fmt.Errorf("failed to get participants for past meeting: %w", err)
	}

	// Try to find existing PastMeetingParticipant record by email first
	if email != "" {
		for _, p := range participants {
			if p.Email != "" && strings.EqualFold(email, p.Email) {
				slog.DebugContext(ctx, "found existing participant by email match",
					"participant_uid", p.UID,
					"email", redaction.RedactEmail(p.Email))
				return p, nil
			}
		}
	}

	// If no existing participant found by email, try to find by name
	if userName != "" {
		// Clean the Zoom name to remove organization info in parentheses
		cleanedZoomName := cleanZoomUserName(userName)
		for _, p := range participants {
			existingFullName := p.GetFullName()
			if existingFullName != "" && strings.EqualFold(cleanedZoomName, existingFullName) {
				slog.DebugContext(ctx, "found existing participant by name match",
					"participant_uid", p.UID,
					"zoom_name", userName,
					"cleaned_zoom_name", cleanedZoomName,
					"existing_name", existingFullName)
				return p, nil
			}
		}
	}

	return nil, nil // No existing participant found
}

// updateParticipantSessionLeaveTime updates a participant's session with the leave time and reason
func (s *ZoomWebhookHandler) updateParticipantSessionLeaveTime(
	ctx context.Context,
	participant *models.PastMeetingParticipant,
	sessionUID string,
	leaveTime time.Time,
	leaveReason string,
) error {
	slog.DebugContext(ctx, "updating participant session leave time",
		"participant_uid", participant.UID,
		"email", redaction.RedactEmail(participant.Email),
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
	_, revision, err := s.pastMeetingParticipantService.GetPastMeetingParticipant(ctx, participant.PastMeetingUID, participant.UID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get participant revision for session update", logging.ErrKey, err)
		return fmt.Errorf("failed to get participant revision: %w", err)
	}

	revisionUint, err := strconv.ParseUint(revision, 10, 64)
	if err != nil {
		slog.ErrorContext(ctx, "failed to parse participant revision", logging.ErrKey, err)
		return fmt.Errorf("failed to parse participant revision: %w", err)
	}

	// Update the participant record
	_, err = s.pastMeetingParticipantService.UpdatePastMeetingParticipant(ctx, participant, revisionUint)
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
		"participant_email", redaction.RedactEmail(participant.Email),
		"participant_name", participant.UserName,
		"join_time", participant.JoinTime,
		"leave_time", participant.LeaveTime,
	)

	session := models.ParticipantSession{
		UID:         participant.ParticipantUUID,
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
	registrants, err := s.registrantService.ListRegistrantsByEmail(ctx, participant.Email)
	if err != nil {
		slog.WarnContext(ctx, "could not check registrant records for participant",
			logging.ErrKey, err,
			"participant_email", redaction.RedactEmail(participant.Email),
		)
		// Continue without failing - we'll treat as non-invited
	} else {
		// Check if any registrant is for this meeting
		for _, registrant := range registrants {
			if registrant.MeetingUID == pastMeeting.MeetingUID {
				matchingRegistrant = registrant
				slog.DebugContext(ctx, "participant was invited (found registrant record)",
					"participant_email", redaction.RedactEmail(participant.Email),
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
			"email", redaction.RedactEmail(matchingRegistrant.Email),
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
			LinkedInProfile:    matchingRegistrant.LinkedInProfile,
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
	_, err = s.pastMeetingParticipantService.CreatePastMeetingParticipant(ctx, newParticipant)
	if err != nil {
		slog.ErrorContext(ctx, "failed to create participant record",
			logging.ErrKey, err,
		)
		return nil, fmt.Errorf("failed to create participant: %w", err)
	}

	// Log success with appropriate details
	logFields := []any{
		"participant_uid", newParticipant.UID,
		"email", redaction.RedactEmail(participant.Email),
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
