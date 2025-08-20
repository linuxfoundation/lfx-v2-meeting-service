// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
)

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
	slog.InfoContext(ctx, "processing meeting started event", "payload", event.Payload)

	// Extract meeting information from payload
	meetingData, ok := event.Payload["object"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid meeting.started payload: missing object field")
	}

	meetingID, ok := meetingData["id"].(string)
	if !ok {
		// Try numeric ID
		if id, ok := meetingData["id"].(float64); ok {
			meetingID = fmt.Sprintf("%.0f", id)
		} else {
			return fmt.Errorf("invalid meeting.started payload: missing or invalid meeting ID")
		}
	}

	slog.InfoContext(ctx, "meeting started", "zoom_meeting_id", meetingID)

	// TODO: Add business logic for meeting started (e.g., update meeting status, send notifications)
	// This could include:
	// - Finding the meeting in our database by Zoom meeting ID
	// - Updating meeting status to "in_progress"
	// - Sending notifications to participants
	// - Recording start time

	return nil
}

// handleMeetingEndedEvent processes meeting.ended events
func (s *MeetingsService) handleMeetingEndedEvent(ctx context.Context, event models.ZoomWebhookEventMessage) error {
	slog.InfoContext(ctx, "processing meeting ended event", "payload", event.Payload)

	// Extract meeting information from payload
	meetingData, ok := event.Payload["object"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid meeting.ended payload: missing object field")
	}

	meetingID, ok := meetingData["id"].(string)
	if !ok {
		// Try numeric ID
		if id, ok := meetingData["id"].(float64); ok {
			meetingID = fmt.Sprintf("%.0f", id)
		} else {
			return fmt.Errorf("invalid meeting.ended payload: missing or invalid meeting ID")
		}
	}

	slog.InfoContext(ctx, "meeting ended", "zoom_meeting_id", meetingID)

	// TODO: Add business logic for meeting ended (e.g., update meeting status, process recordings)
	// This could include:
	// - Finding the meeting in our database by Zoom meeting ID
	// - Updating meeting status to "completed"
	// - Recording end time and duration
	// - Triggering post-meeting processes (recordings, transcripts, etc.)

	return nil
}

// handleMeetingDeletedEvent processes meeting.deleted events
func (s *MeetingsService) handleMeetingDeletedEvent(ctx context.Context, event models.ZoomWebhookEventMessage) error {
	slog.InfoContext(ctx, "processing meeting deleted event", "payload", event.Payload)

	// Extract meeting information from payload
	meetingData, ok := event.Payload["object"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid meeting.deleted payload: missing object field")
	}

	meetingID, ok := meetingData["id"].(string)
	if !ok {
		// Try numeric ID
		if id, ok := meetingData["id"].(float64); ok {
			meetingID = fmt.Sprintf("%.0f", id)
		} else {
			return fmt.Errorf("invalid meeting.deleted payload: missing or invalid meeting ID")
		}
	}

	slog.InfoContext(ctx, "meeting deleted", "zoom_meeting_id", meetingID)

	// TODO: Add business logic for meeting deleted (e.g., clean up related data)
	// This could include:
	// - Finding the meeting in our database by Zoom meeting ID
	// - Updating meeting status to "deleted" or removing it
	// - Cleaning up related registrations and data
	// - Sending notifications about cancellation

	return nil
}

// handleParticipantJoinedEvent processes meeting.participant_joined events
func (s *MeetingsService) handleParticipantJoinedEvent(ctx context.Context, event models.ZoomWebhookEventMessage) error {
	slog.InfoContext(ctx, "processing participant joined event", "payload", event.Payload)

	// Extract participant information from payload
	participantData, ok := event.Payload["object"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid participant_joined payload: missing object field")
	}

	meetingID, ok := participantData["id"].(string)
	if !ok {
		// Try numeric ID
		if id, ok := participantData["id"].(float64); ok {
			meetingID = fmt.Sprintf("%.0f", id)
		} else {
			return fmt.Errorf("invalid participant_joined payload: missing or invalid meeting ID")
		}
	}

	participant, ok := participantData["participant"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid participant_joined payload: missing participant field")
	}

	participantName, _ := participant["user_name"].(string)
	participantEmail, _ := participant["email"].(string)

	slog.InfoContext(ctx, "participant joined",
		"zoom_meeting_id", meetingID,
		"participant_name", participantName,
		"participant_email", participantEmail,
	)

	// TODO: Add business logic for participant joined (e.g., track attendance)
	// This could include:
	// - Recording participant join time
	// - Updating attendance records
	// - Sending welcome notifications
	// - Tracking participation metrics

	return nil
}

// handleParticipantLeftEvent processes meeting.participant_left events
func (s *MeetingsService) handleParticipantLeftEvent(ctx context.Context, event models.ZoomWebhookEventMessage) error {
	slog.InfoContext(ctx, "processing participant left event", "payload", event.Payload)

	// Extract participant information from payload
	participantData, ok := event.Payload["object"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid participant_left payload: missing object field")
	}

	meetingID, ok := participantData["id"].(string)
	if !ok {
		// Try numeric ID
		if id, ok := participantData["id"].(float64); ok {
			meetingID = fmt.Sprintf("%.0f", id)
		} else {
			return fmt.Errorf("invalid participant_left payload: missing or invalid meeting ID")
		}
	}

	participant, ok := participantData["participant"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid participant_left payload: missing participant field")
	}

	participantName, _ := participant["user_name"].(string)
	participantEmail, _ := participant["email"].(string)

	slog.InfoContext(ctx, "participant left",
		"zoom_meeting_id", meetingID,
		"participant_name", participantName,
		"participant_email", participantEmail,
	)

	// TODO: Add business logic for participant left (e.g., track attendance duration)
	// This could include:
	// - Recording participant leave time
	// - Calculating participation duration
	// - Updating attendance records
	// - Generating participation reports

	return nil
}

// handleRecordingCompletedEvent processes recording.completed events
func (s *MeetingsService) handleRecordingCompletedEvent(ctx context.Context, event models.ZoomWebhookEventMessage) error {
	slog.InfoContext(ctx, "processing recording completed event", "payload", event.Payload)

	// Extract recording information from payload
	recordingData, ok := event.Payload["object"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid recording.completed payload: missing object field")
	}

	meetingID, ok := recordingData["id"].(string)
	if !ok {
		// Try numeric ID
		if id, ok := recordingData["id"].(float64); ok {
			meetingID = fmt.Sprintf("%.0f", id)
		} else {
			return fmt.Errorf("invalid recording.completed payload: missing or invalid meeting ID")
		}
	}

	slog.InfoContext(ctx, "recording completed", "zoom_meeting_id", meetingID)

	// TODO: Add business logic for recording completed (e.g., process recording files)
	// This could include:
	// - Downloading and storing recording files
	// - Generating recording access links
	// - Sending recording notifications to participants
	// - Processing recording for transcription
	// - Updating meeting records with recording information

	return nil
}

// handleTranscriptCompletedEvent processes recording.transcript_completed events
func (s *MeetingsService) handleTranscriptCompletedEvent(ctx context.Context, event models.ZoomWebhookEventMessage) error {
	slog.InfoContext(ctx, "processing transcript completed event", "payload", event.Payload)

	// Extract transcript information from payload
	transcriptData, ok := event.Payload["object"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid transcript_completed payload: missing object field")
	}

	meetingID, ok := transcriptData["id"].(string)
	if !ok {
		// Try numeric ID
		if id, ok := transcriptData["id"].(float64); ok {
			meetingID = fmt.Sprintf("%.0f", id)
		} else {
			return fmt.Errorf("invalid transcript_completed payload: missing or invalid meeting ID")
		}
	}

	slog.InfoContext(ctx, "transcript completed", "zoom_meeting_id", meetingID)

	// TODO: Add business logic for transcript completed (e.g., process transcript files)
	// This could include:
	// - Downloading and storing transcript files
	// - Processing transcript for searchability
	// - Generating transcript access links
	// - Sending transcript notifications to participants
	// - Extracting key insights or action items

	return nil
}

// handleSummaryCompletedEvent processes meeting.summary_completed events
func (s *MeetingsService) handleSummaryCompletedEvent(ctx context.Context, event models.ZoomWebhookEventMessage) error {
	slog.InfoContext(ctx, "processing summary completed event", "payload", event.Payload)

	// Extract summary information from payload
	summaryData, ok := event.Payload["object"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid summary_completed payload: missing object field")
	}

	meetingID, ok := summaryData["id"].(string)
	if !ok {
		// Try numeric ID
		if id, ok := summaryData["id"].(float64); ok {
			meetingID = fmt.Sprintf("%.0f", id)
		} else {
			return fmt.Errorf("invalid summary_completed payload: missing or invalid meeting ID")
		}
	}

	slog.InfoContext(ctx, "summary completed", "zoom_meeting_id", meetingID)

	// TODO: Add business logic for summary completed (e.g., process meeting summaries)
	// This could include:
	// - Downloading and storing AI-generated summaries
	// - Extracting action items and key decisions
	// - Sending summary notifications to participants
	// - Integrating summaries with project management tools
	// - Generating follow-up reminders

	return nil
}
