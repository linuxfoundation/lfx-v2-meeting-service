// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/service"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/concurrent"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/utils"
)

// ZoomWebhookHandler handles Zoom webhook events.
type MeetingHandler struct {
	meetingService                *service.MeetingService
	registrantService             *service.MeetingRegistrantService
	pastMeetingService            *service.PastMeetingService
	pastMeetingParticipantService *service.PastMeetingParticipantService
}

func NewMeetingHandler(
	meetingService *service.MeetingService,
	registrantService *service.MeetingRegistrantService,
	pastMeetingService *service.PastMeetingService,
	pastMeetingParticipantService *service.PastMeetingParticipantService,
) *MeetingHandler {
	return &MeetingHandler{
		meetingService:                meetingService,
		registrantService:             registrantService,
		pastMeetingService:            pastMeetingService,
		pastMeetingParticipantService: pastMeetingParticipantService,
	}
}

func (s *MeetingHandler) HandlerReady() bool {
	return s.meetingService.ServiceReady() &&
		s.registrantService.ServiceReady() &&
		s.pastMeetingService.ServiceReady() &&
		s.pastMeetingParticipantService.ServiceReady()
}

// HandleMessage implements domain.MessageHandler interface
func (s *MeetingHandler) HandleMessage(ctx context.Context, msg domain.Message) {
	subject := msg.Subject()
	ctx = logging.AppendCtx(ctx, slog.String("subject", subject))
	slog.DebugContext(ctx, "handling NATS message")

	var response []byte
	var err error

	handlers := map[string]func(ctx context.Context, msg domain.Message) ([]byte, error){
		models.MeetingGetTitleSubject: s.HandleMeetingGetTitle,
		models.MeetingDeletedSubject:  s.HandleMeetingDeleted,
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

func (s *MeetingHandler) handleMeetingGetAttribute(ctx context.Context, msg domain.Message, subject, getAttribute string) ([]byte, error) {
	if !s.meetingService.ServiceReady() {
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

	meeting, err := s.meetingService.MeetingRepository.GetBase(ctx, meetingUID)
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
func (s *MeetingHandler) HandleMeetingGetTitle(ctx context.Context, msg domain.Message) ([]byte, error) {
	return s.handleMeetingGetAttribute(ctx, msg, models.MeetingGetTitleSubject, "title")
}

// HandleMeetingDeleted is the message handler for the meeting-deleted subject.
// It cleans up all registrants associated with the deleted meeting.
func (s *MeetingHandler) HandleMeetingDeleted(ctx context.Context, msg domain.Message) ([]byte, error) {
	if !s.meetingService.ServiceReady() {
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
	registrants, err := s.registrantService.RegistrantRepository.ListByMeeting(ctx, meetingUID)
	if err != nil {
		slog.ErrorContext(ctx, "error getting registrants for deleted meeting", logging.ErrKey, err)
		return nil, err
	}

	if len(registrants) == 0 {
		slog.DebugContext(ctx, "no registrants to clean up for deleted meeting")
		return []byte("success"), nil
	}

	slog.InfoContext(ctx, "cleaning up registrants for deleted meeting", "registrant_count", len(registrants))

	// Process registrants concurrently using WorkerPool
	var tasks []func() error
	for _, registrant := range registrants {
		reg := registrant // capture loop variable
		tasks = append(tasks, func() error {
			// Use the shared helper with skipRevisionCheck=true for bulk cleanup
			err := s.registrantService.DeleteRegistrantWithCleanup(ctx, reg, 0, true)
			if err != nil {
				slog.ErrorContext(ctx, "error deleting registrant",
					"registrant_uid", reg.UID,
					logging.ErrKey, err,
					logging.PriorityCritical())
				return err
			}
			slog.DebugContext(ctx, "successfully cleaned up registrant", "registrant_uid", reg.UID)
			return nil
		})
	}

	// Execute all cleanup operations concurrently using WorkerPool
	pool := concurrent.NewWorkerPool(10) // Use 10 workers, same concurrency as before
	err = pool.Run(ctx, tasks...)
	if err != nil {
		slog.ErrorContext(ctx, "some registrant cleanup operations failed",
			"total_registrants", len(registrants),
			logging.ErrKey, err,
			logging.PriorityCritical())
		return nil, fmt.Errorf("failed to clean up registrants: %w", err)
	}

	slog.InfoContext(ctx, "successfully cleaned up all registrants for deleted meeting", "registrant_count", len(registrants))
	return []byte("success"), nil
}
