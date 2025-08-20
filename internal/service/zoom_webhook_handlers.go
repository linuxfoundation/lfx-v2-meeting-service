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
	// Convert to typed payload
	payload, err := event.ToMeetingStartedPayload()
	if err != nil {
		slog.ErrorContext(ctx, "failed to convert to typed meeting started payload", "error", err)
		return fmt.Errorf("failed to parse meeting started payload: %w", err)
	}

	slog.InfoContext(ctx, "processing meeting started event",
		"zoom_meeting_uuid", payload.Object.UUID,
		"zoom_meeting_id", payload.Object.ID,
		"topic", payload.Object.Topic,
		"start_time", payload.Object.StartTime,
	)

	return nil
}

// handleMeetingEndedEvent processes meeting.ended events
func (s *MeetingsService) handleMeetingEndedEvent(ctx context.Context, event models.ZoomWebhookEventMessage) error {
	// Convert to typed payload
	payload, err := event.ToMeetingEndedPayload()
	if err != nil {
		slog.ErrorContext(ctx, "failed to convert to typed meeting ended payload", "error", err)
		return fmt.Errorf("failed to parse meeting ended payload: %w", err)
	}

	slog.InfoContext(ctx, "processing meeting ended event",
		"zoom_meeting_uuid", payload.Object.UUID,
		"zoom_meeting_id", payload.Object.ID,
		"topic", payload.Object.Topic,
		"start_time", payload.Object.StartTime,
		"end_time", payload.Object.EndTime,
		"duration", payload.Object.Duration,
	)

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
