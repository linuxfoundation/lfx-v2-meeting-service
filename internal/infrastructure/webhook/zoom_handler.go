// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	meetingsvc "github.com/linuxfoundation/lfx-v2-meeting-service/gen/meeting_service"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
)

// ZoomWebhookHandler implements domain.WebhookHandler for Zoom webhook events
type ZoomWebhookHandler struct {
	validator *ZoomWebhookValidator
}

// NewZoomWebhookHandler creates a new Zoom webhook handler
func NewZoomWebhookHandler() *ZoomWebhookHandler {
	secretToken := os.Getenv("ZOOM_WEBHOOK_SECRET_TOKEN")
	validator := NewZoomWebhookValidator(secretToken)

	return &ZoomWebhookHandler{
		validator: validator,
	}
}

// HandleWebhookEvent processes incoming Zoom webhook events (legacy method for API compatibility)
func (h *ZoomWebhookHandler) HandleWebhookEvent(ctx context.Context, payload *meetingsvc.ZoomWebhookPayload2) (*meetingsvc.ZoomWebhookResponse, error) {
	logger := slog.With("component", "zoom_webhook_handler")

	// Validate webhook signature if signature and timestamp are provided
	if payload.ZoomSignature != nil && payload.ZoomTimestamp != nil {
		// Marshal the body to get the raw bytes for signature validation
		bodyBytes, err := json.Marshal(payload.Body)
		if err != nil {
			logger.ErrorContext(ctx, "Failed to marshal webhook body for validation", logging.ErrKey, err)
			return nil, fmt.Errorf("invalid webhook payload: %w", err)
		}

		if err := h.validator.ValidateSignature(bodyBytes, *payload.ZoomSignature, *payload.ZoomTimestamp); err != nil {
			logger.WarnContext(ctx, "Webhook signature validation failed", logging.ErrKey, err)
			return nil, fmt.Errorf("unauthorized: %w", err)
		}

		logger.DebugContext(ctx, "Webhook signature validation passed")
	}

	// Validate event type
	if payload.Body == nil || payload.Body.Event == "" {
		logger.WarnContext(ctx, "Webhook payload missing event field")
		return nil, fmt.Errorf("invalid webhook payload: missing event field")
	}

	eventType := payload.Body.Event
	if !h.validator.IsValidEvent(eventType) {
		logger.WarnContext(ctx, "Unsupported webhook event type", "event_type", eventType)
		return nil, fmt.Errorf("unsupported event type: %s", eventType)
	}

	// Use the domain interface method
	if err := h.HandleEvent(ctx, eventType, payload.Body.Payload); err != nil {
		logger.ErrorContext(ctx, "Failed to process webhook event", logging.ErrKey, err)
		return nil, err
	}

	return &meetingsvc.ZoomWebhookResponse{
		Status:  "success",
		Message: stringPtr(fmt.Sprintf("Event %s processed successfully", eventType)),
	}, nil
}

// handleMeetingStarted processes meeting.started events
// TODO: This method is a placeholder for future implementation
func (h *ZoomWebhookHandler) handleMeetingStarted(ctx context.Context, payload *meetingsvc.ZoomWebhookPayload2) (*meetingsvc.ZoomWebhookResponse, error) {
	logger := slog.With("component", "webhook_handler", "event", "meeting.started")

	// Extract meeting information from payload
	// The payload.Body.Payload contains the actual event data
	logger.InfoContext(ctx, "Meeting started", "event_timestamp", payload.Body.EventTs)

	// TODO: Implement meeting started logic:
	// - Update meeting status to "in-progress"
	// - Send notifications to registered participants
	// - Update meeting analytics/tracking

	logger.DebugContext(ctx, "Meeting started event processed successfully")

	return &meetingsvc.ZoomWebhookResponse{
		Status:  "success",
		Message: stringPtr("Meeting started event processed"),
	}, nil
}

// handleMeetingEnded processes meeting.ended events
func (h *ZoomWebhookHandler) handleMeetingEnded(ctx context.Context, payload *meetingsvc.ZoomWebhookPayload2) (*meetingsvc.ZoomWebhookResponse, error) {
	logger := slog.With("component", "webhook_handler", "event", "meeting.ended")

	logger.InfoContext(ctx, "Meeting ended", "event_timestamp", payload.Body.EventTs)

	// TODO: Implement meeting ended logic:
	// - Update meeting status to "completed"
	// - Process attendance data
	// - Trigger post-meeting workflows (emails, reports, etc.)

	logger.DebugContext(ctx, "Meeting ended event processed successfully")

	return &meetingsvc.ZoomWebhookResponse{
		Status:  "success",
		Message: stringPtr("Meeting ended event processed"),
	}, nil
}

// handleMeetingDeleted processes meeting.deleted events
func (h *ZoomWebhookHandler) handleMeetingDeleted(ctx context.Context, payload *meetingsvc.ZoomWebhookPayload2) (*meetingsvc.ZoomWebhookResponse, error) {
	logger := slog.With("component", "webhook_handler", "event", "meeting.deleted")

	logger.InfoContext(ctx, "Meeting deleted", "event_timestamp", payload.Body.EventTs)

	// TODO: Implement meeting deleted logic:
	// - Update meeting status to "cancelled" or remove meeting
	// - Notify registered participants of cancellation
	// - Clean up associated resources

	logger.DebugContext(ctx, "Meeting deleted event processed successfully")

	return &meetingsvc.ZoomWebhookResponse{
		Status:  "success",
		Message: stringPtr("Meeting deleted event processed"),
	}, nil
}

// handleParticipantJoined processes meeting.participant_joined events
func (h *ZoomWebhookHandler) handleParticipantJoined(ctx context.Context, payload *meetingsvc.ZoomWebhookPayload2) (*meetingsvc.ZoomWebhookResponse, error) {
	logger := slog.With("component", "webhook_handler", "event", "meeting.participant_joined")

	logger.InfoContext(ctx, "Participant joined", "event_timestamp", payload.Body.EventTs)

	// TODO: Implement participant joined logic:
	// - Track participant attendance
	// - Update meeting participant list
	// - Send join notifications if required

	logger.DebugContext(ctx, "Participant joined event processed successfully")

	return &meetingsvc.ZoomWebhookResponse{
		Status:  "success",
		Message: stringPtr("Participant joined event processed"),
	}, nil
}

// handleParticipantLeft processes meeting.participant_left events
func (h *ZoomWebhookHandler) handleParticipantLeft(ctx context.Context, payload *meetingsvc.ZoomWebhookPayload2) (*meetingsvc.ZoomWebhookResponse, error) {
	logger := slog.With("component", "webhook_handler", "event", "meeting.participant_left")

	logger.InfoContext(ctx, "Participant left", "event_timestamp", payload.Body.EventTs)

	// TODO: Implement participant left logic:
	// - Update participant attendance duration
	// - Update meeting participant list
	// - Process attendance data

	logger.DebugContext(ctx, "Participant left event processed successfully")

	return &meetingsvc.ZoomWebhookResponse{
		Status:  "success",
		Message: stringPtr("Participant left event processed"),
	}, nil
}

// handleRecordingCompleted processes recording.completed events
func (h *ZoomWebhookHandler) handleRecordingCompleted(ctx context.Context, payload *meetingsvc.ZoomWebhookPayload2) (*meetingsvc.ZoomWebhookResponse, error) {
	logger := slog.With("component", "webhook_handler", "event", "recording.completed")

	logger.InfoContext(ctx, "Recording completed", "event_timestamp", payload.Body.EventTs)

	// TODO: Implement recording completed logic:
	// - Download/process recording files
	// - Update meeting with recording links
	// - Trigger post-processing workflows (transcription, upload to YouTube, etc.)
	// - Send recording notification to participants

	logger.DebugContext(ctx, "Recording completed event processed successfully")

	return &meetingsvc.ZoomWebhookResponse{
		Status:  "success",
		Message: stringPtr("Recording completed event processed"),
	}, nil
}

// handleTranscriptCompleted processes recording.transcript_completed events
func (h *ZoomWebhookHandler) handleTranscriptCompleted(ctx context.Context, payload *meetingsvc.ZoomWebhookPayload2) (*meetingsvc.ZoomWebhookResponse, error) {
	logger := slog.With("component", "webhook_handler", "event", "recording.transcript_completed")

	logger.InfoContext(ctx, "Transcript completed", "event_timestamp", payload.Body.EventTs)

	// TODO: Implement transcript completed logic:
	// - Download/process transcript files
	// - Update meeting with transcript links
	// - Process transcript for summary generation
	// - Send transcript notification to participants

	logger.DebugContext(ctx, "Transcript completed event processed successfully")

	return &meetingsvc.ZoomWebhookResponse{
		Status:  "success",
		Message: stringPtr("Transcript completed event processed"),
	}, nil
}

// handleSummaryCompleted processes meeting.summary_completed events
func (h *ZoomWebhookHandler) handleSummaryCompleted(ctx context.Context, payload *meetingsvc.ZoomWebhookPayload2) (*meetingsvc.ZoomWebhookResponse, error) {
	logger := slog.With("component", "webhook_handler", "event", "meeting.summary_completed")

	logger.InfoContext(ctx, "Summary completed", "event_timestamp", payload.Body.EventTs)

	// TODO: Implement summary completed logic:
	// - Download/process AI-generated meeting summary
	// - Update meeting with summary content
	// - Send summary to participants

	logger.DebugContext(ctx, "Summary completed event processed successfully")

	return &meetingsvc.ZoomWebhookResponse{
		Status:  "success",
		Message: stringPtr("Summary completed event processed"),
	}, nil
}

// stringPtr returns a pointer to the given string
func stringPtr(s string) *string {
	return &s
}

// HandleEvent implements domain.WebhookHandler interface
func (h *ZoomWebhookHandler) HandleEvent(ctx context.Context, eventType string, payload interface{}) error {
	logger := slog.With("component", "zoom_webhook_handler", "event_type", eventType)

	// Validate event type
	if !h.validator.IsValidEvent(eventType) {
		return fmt.Errorf("unsupported event type: %s", eventType)
	}

	logger.InfoContext(ctx, "Processing Zoom webhook event")

	// TODO: Process the event based on eventType and payload
	// This is where we would implement the actual business logic for each event type
	// For now, just log the event
	logger.InfoContext(ctx, "Zoom webhook event processed successfully")

	return nil
}

// ValidateSignature implements domain.WebhookHandler interface
func (h *ZoomWebhookHandler) ValidateSignature(body []byte, signature, timestamp string) error {
	return h.validator.ValidateSignature(body, signature, timestamp)
}

// SupportedEvents implements domain.WebhookHandler interface
func (h *ZoomWebhookHandler) SupportedEvents() []string {
	return []string{
		"meeting.started",
		"meeting.ended",
		"meeting.deleted",
		"meeting.participant_joined",
		"meeting.participant_left",
		"recording.completed",
		"recording.transcript_completed",
		"meeting.summary_completed",
	}
}
