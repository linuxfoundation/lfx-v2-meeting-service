// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/utils"
)

// HandleMessage implements domain.MessageHandler interface
func (s *MeetingsService) HandleMessage(ctx context.Context, msg domain.Message) {
	subject := msg.Subject()
	ctx = logging.AppendCtx(ctx, slog.String("subject", subject))
	slog.DebugContext(ctx, "handling NATS message")

	var response []byte
	var err error

	handlers := map[string]func(ctx context.Context, msg domain.Message) ([]byte, error){
		models.MeetingGetTitleSubject:                         s.HandleMeetingGetTitle,
		models.MeetingDeletedSubject:                          s.HandleMeetingDeleted,
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

func (s *MeetingsService) handleMeetingGetAttribute(ctx context.Context, msg domain.Message, subject, getAttribute string) ([]byte, error) {

	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "NATS KV store not initialized")
		return nil, fmt.Errorf("NATS KV store not initialized")
	}

	meetingUID := string(msg.Data())

	ctx = logging.AppendCtx(ctx, slog.String("meeting_id", meetingUID))
	ctx = logging.AppendCtx(ctx, slog.String("subject", subject))

	// Validate that the meeting ID is a valid UUID.
	_, err := uuid.Parse(meetingUID)
	if err != nil {
		slog.ErrorContext(ctx, "error parsing meeting ID", logging.ErrKey, err)
		return nil, err
	}

	meeting, err := s.MeetingRepository.GetBase(ctx, meetingUID)
	if err != nil {
		slog.ErrorContext(ctx, "error getting meeting from NATS KV", logging.ErrKey, err)
		return nil, err
	}

	value, ok := utils.FieldByTag(meeting, "json", getAttribute)
	if !ok {
		slog.ErrorContext(ctx, "error getting meeting attribute", logging.ErrKey, fmt.Errorf("attribute %s not found", getAttribute))
		return nil, fmt.Errorf("attribute %s not found", getAttribute)
	}

	strValue, ok := value.(string)
	if !ok {
		slog.ErrorContext(ctx, "meeting attribute is not a string", logging.ErrKey, fmt.Errorf("attribute %s is not a string", getAttribute))
		return nil, fmt.Errorf("attribute %s is not a string", getAttribute)
	}

	return []byte(strValue), nil
}

// HandleMeetingGetTitle is the message handler for the meeting-get-title subject.
func (s *MeetingsService) HandleMeetingGetTitle(ctx context.Context, msg domain.Message) ([]byte, error) {
	return s.handleMeetingGetAttribute(ctx, msg, models.MeetingGetTitleSubject, "title")
}

// HandleMeetingDeleted is the message handler for the meeting-deleted subject.
// It cleans up all registrants associated with the deleted meeting.
func (s *MeetingsService) HandleMeetingDeleted(ctx context.Context, msg domain.Message) ([]byte, error) {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "service not ready")
		return nil, fmt.Errorf("service not ready")
	}

	// Parse the meeting deletion message
	var meetingDeletedMsg models.MeetingDeletedMessage
	err := json.Unmarshal(msg.Data(), &meetingDeletedMsg)
	if err != nil {
		slog.ErrorContext(ctx, "error unmarshaling meeting deleted message", logging.ErrKey, err)
		return nil, err
	}

	meetingUID := meetingDeletedMsg.MeetingUID
	if meetingUID == "" {
		slog.WarnContext(ctx, "meeting UID is empty in deletion message")
		return nil, fmt.Errorf("meeting UID is required")
	}

	ctx = logging.AppendCtx(ctx, slog.String("meeting_uid", meetingUID))
	slog.InfoContext(ctx, "processing meeting deletion, cleaning up registrants")

	// Get all registrants for the meeting
	registrants, err := s.RegistrantRepository.ListByMeeting(ctx, meetingUID)
	if err != nil {
		slog.ErrorContext(ctx, "error getting registrants for deleted meeting", logging.ErrKey, err)
		return nil, err
	}

	if len(registrants) == 0 {
		slog.DebugContext(ctx, "no registrants to clean up for deleted meeting")
		return []byte("success"), nil
	}

	slog.InfoContext(ctx, "cleaning up registrants for deleted meeting", "registrant_count", len(registrants))

	// Process registrants concurrently using goroutines for better performance
	// Use a channel to collect errors from concurrent operations
	type registrantError struct {
		registrantUID string
		err           error
	}

	errorChan := make(chan registrantError, len(registrants))
	semaphore := make(chan struct{}, 10) // Limit concurrency to 10 goroutines

	// Launch goroutines for concurrent registrant deletion
	for _, registrant := range registrants {
		go func(reg *models.Registrant) {
			semaphore <- struct{}{}        // Acquire semaphore
			defer func() { <-semaphore }() // Release semaphore

			// Use the shared helper with skipRevisionCheck=true for bulk cleanup
			err := s.deleteRegistrantWithCleanup(ctx, reg, 0, true)
			if err != nil {
				slog.ErrorContext(ctx, "error deleting registrant",
					"registrant_uid", reg.UID,
					logging.ErrKey, err,
					logging.PriorityCritical())
				errorChan <- registrantError{registrantUID: reg.UID, err: err}
			} else {
				slog.DebugContext(ctx, "successfully cleaned up registrant", "registrant_uid", reg.UID)
				errorChan <- registrantError{registrantUID: reg.UID, err: nil}
			}
		}(registrant)
	}

	// Collect results from all goroutines
	var cleanupErrors []error
	successCount := 0
	for range len(registrants) {
		result := <-errorChan
		if result.err != nil {
			cleanupErrors = append(cleanupErrors, result.err)
		} else {
			successCount++
		}
	}
	close(errorChan)

	if len(cleanupErrors) > 0 {
		slog.ErrorContext(ctx, "some registrant cleanup operations failed",
			"total_registrants", len(registrants),
			"successful_count", successCount,
			"failed_count", len(cleanupErrors),
			logging.PriorityCritical())
		return nil, fmt.Errorf("failed to clean up %d out of %d registrants", len(cleanupErrors), len(registrants))
	}

	slog.InfoContext(ctx, "successfully cleaned up all registrants for deleted meeting", "registrant_count", len(registrants))
	return []byte("success"), nil
}
