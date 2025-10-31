// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/utils"
)

// ZoomWebhookService handles Zoom webhook event processing
type ZoomWebhookService struct {
	messageSender    domain.WebhookEventSender
	webhookValidator domain.WebhookValidator
}

// WebhookRequest represents the webhook processing request
type WebhookRequest struct {
	Event     string
	EventTS   int64
	Payload   any
	Signature string
	Timestamp string
	RawBody   []byte
}

// WebhookResponse represents the webhook processing response
type WebhookResponse struct {
	Status         *string
	Message        *string
	PlainToken     *string
	EncryptedToken *string
}

// NewZoomWebhookService creates a new ZoomWebhookService
func NewZoomWebhookService(
	messageSender domain.WebhookEventSender,
	webhookValidator domain.WebhookValidator,
) *ZoomWebhookService {
	return &ZoomWebhookService{
		messageSender:    messageSender,
		webhookValidator: webhookValidator,
	}
}

// ServiceReady checks if the service is ready to process requests
func (s *ZoomWebhookService) ServiceReady() bool {
	return s.messageSender != nil && s.webhookValidator != nil
}

// ProcessWebhookEvent processes a Zoom webhook event
func (s *ZoomWebhookService) ProcessWebhookEvent(ctx context.Context, req WebhookRequest) (*WebhookResponse, error) {
	// Validate request
	if err := s.validateRequest(req); err != nil {
		return nil, err
	}

	// Validate webhook signature
	if err := s.validateSignature(req); err != nil {
		return nil, err
	}

	// Handle special endpoint validation event
	if req.Event == "endpoint.url_validation" {
		return s.handleEndpointValidation(ctx, req)
	}

	// Process regular webhook event
	return s.processRegularEvent(ctx, req)
}

// validateRequest validates the webhook request structure
func (s *ZoomWebhookService) validateRequest(req WebhookRequest) error {
	if req.Event == "" {
		return domain.NewValidationError("missing event field")
	}

	if req.Payload == nil {
		return domain.NewValidationError("missing payload field")
	}

	if req.Signature == "" || req.Timestamp == "" {
		return domain.NewValidationError("missing signature headers")
	}

	return nil
}

// validateSignature validates the webhook signature
func (s *ZoomWebhookService) validateSignature(req WebhookRequest) error {
	if err := s.webhookValidator.ValidateSignature(req.RawBody, req.Signature, req.Timestamp); err != nil {
		return domain.NewValidationError("invalid webhook signature", err)
	}
	return nil
}

// handleEndpointValidation handles the special endpoint.url_validation event
func (s *ZoomWebhookService) handleEndpointValidation(ctx context.Context, req WebhookRequest) (*WebhookResponse, error) {
	logger := slog.With("component", "zoom_webhook_service", "method", "handleEndpointValidation")

	// Extract plainToken from payload
	payloadMap, ok := req.Payload.(map[string]any)
	if !ok {
		logger.ErrorContext(ctx, "Webhook payload is not a valid map for validation", "payload_type", fmt.Sprintf("%T", req.Payload))
		return nil, domain.NewValidationError("invalid validation payload format")
	}

	plainToken, ok := payloadMap["plainToken"].(string)
	if !ok || plainToken == "" {
		logger.ErrorContext(ctx, "Missing plainToken in validation payload")
		return nil, domain.NewValidationError("missing plainToken in validation payload")
	}

	// Generate encrypted token using HMAC SHA-256
	secretToken := s.webhookValidator.GetSecretToken()
	if secretToken == "" {
		logger.ErrorContext(ctx, "Zoom webhook validator not properly configured")
		return nil, domain.NewInternalError("webhook validation not configured")
	}

	h := hmac.New(sha256.New, []byte(secretToken))
	h.Write([]byte(plainToken))
	encryptedToken := hex.EncodeToString(h.Sum(nil))

	logger.InfoContext(ctx, "Zoom webhook endpoint validation completed successfully")

	return &WebhookResponse{
		PlainToken:     utils.StringPtr(plainToken),
		EncryptedToken: utils.StringPtr(encryptedToken),
	}, nil
}

// processRegularEvent processes regular webhook events by publishing to NATS
func (s *ZoomWebhookService) processRegularEvent(ctx context.Context, req WebhookRequest) (*WebhookResponse, error) {
	logger := slog.With("component", "zoom_webhook_service", "method", "processRegularEvent")

	// Map event type to NATS subject
	subject := getZoomWebhookSubject(req.Event)
	if subject == "" {
		logger.WarnContext(ctx, "Unsupported Zoom webhook event type", "event_type", req.Event)
		return nil, domain.NewValidationError(fmt.Sprintf("unsupported event type: %s", req.Event), nil)
	}

	// Create webhook event message for NATS
	payloadMap, ok := req.Payload.(map[string]any)
	if !ok {
		logger.ErrorContext(ctx, "Webhook payload is not a valid map", "payload_type", fmt.Sprintf("%T", req.Payload))
		return nil, domain.NewValidationError("invalid webhook payload format")
	}

	webhookMessage := models.ZoomWebhookEventMessage{
		EventType: req.Event,
		EventTS:   req.EventTS,
		Payload:   payloadMap,
	}

	// Publish to NATS for async processing
	if err := s.messageSender.PublishZoomWebhookEvent(ctx, subject, webhookMessage); err != nil {
		logger.ErrorContext(ctx, "Failed to publish webhook event to NATS", "error", err, "event_type", req.Event, "subject", subject)
		return nil, domain.NewInternalError("failed to process webhook event", err)
	}

	logger.InfoContext(ctx, "Zoom webhook event published to NATS successfully", "event_type", req.Event, "subject", subject)

	return &WebhookResponse{
		Status:  utils.StringPtr("success"),
		Message: utils.StringPtr(fmt.Sprintf("Event %s queued for processing", req.Event)),
	}, nil
}

// getZoomWebhookSubject maps Zoom event types to NATS subjects
func getZoomWebhookSubject(eventType string) string {
	eventSubjectMap := map[string]string{
		"meeting.started":                models.ZoomWebhookMeetingStartedSubject,
		"meeting.ended":                  models.ZoomWebhookMeetingEndedSubject,
		"meeting.deleted":                models.ZoomWebhookMeetingDeletedSubject,
		"meeting.participant_joined":     models.ZoomWebhookMeetingParticipantJoinedSubject,
		"meeting.participant_left":       models.ZoomWebhookMeetingParticipantLeftSubject,
		"recording.completed":            models.ZoomWebhookRecordingCompletedSubject,
		"recording.transcript_completed": models.ZoomWebhookRecordingTranscriptCompletedSubject,
		"meeting.summary_completed":      models.ZoomWebhookMeetingSummaryCompletedSubject,
	}

	return eventSubjectMap[eventType]
}
