// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
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
		models.MeetingGetTitleSubject: s.HandleMeetingGetTitle,
	}

	handler, ok := handlers[subject]
	if !ok {
		slog.WarnContext(ctx, "unknown subject")
		err = msg.Respond(nil)
		if err != nil {
			slog.ErrorContext(ctx, "error responding to NATS message", logging.ErrKey, err)
			return
		}
		return
	}

	response, err = handler(ctx, msg)
	if err != nil {
		slog.ErrorContext(ctx, "error handling message",
			logging.ErrKey, err,
			"subject", subject,
		)
		err = msg.Respond(nil)
		if err != nil {
			slog.ErrorContext(ctx, "error responding to NATS message", logging.ErrKey, err)
		}
		return
	}
	err = msg.Respond(response)
	if err != nil {
		slog.ErrorContext(ctx, "error responding to NATS message", logging.ErrKey, err)
		return
	}

	slog.DebugContext(ctx, "responded to NATS message", "response", response)
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

	meeting, err := s.MeetingRepository.Get(ctx, meetingUID)
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
