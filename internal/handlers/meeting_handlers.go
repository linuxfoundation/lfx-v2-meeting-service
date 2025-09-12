// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/service"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/concurrent"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/constants"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/utils"
)

// MeetingHandler handles meeting-related messages and events.
type MeetingHandler struct {
	meetingService                *service.MeetingService
	registrantService             *service.MeetingRegistrantService
	pastMeetingService            *service.PastMeetingService
	pastMeetingParticipantService *service.PastMeetingParticipantService
	committeeSyncService          *service.CommitteeSyncService
}

func NewMeetingHandler(
	meetingService *service.MeetingService,
	registrantService *service.MeetingRegistrantService,
	pastMeetingService *service.PastMeetingService,
	pastMeetingParticipantService *service.PastMeetingParticipantService,
	committeeSyncService *service.CommitteeSyncService,
) *MeetingHandler {
	return &MeetingHandler{
		meetingService:                meetingService,
		registrantService:             registrantService,
		pastMeetingService:            pastMeetingService,
		pastMeetingParticipantService: pastMeetingParticipantService,
		committeeSyncService:          committeeSyncService,
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
		models.MeetingCreatedSubject:  s.HandleMeetingCreated,
		models.MeetingUpdatedSubject:  s.HandleMeetingUpdated,
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
	errors := pool.RunAll(ctx, tasks...)
	if len(errors) > 0 {
		slog.ErrorContext(ctx, "some registrant cleanup operations failed",
			"meeting_uid", meetingUID,
			"total_registrants", len(registrants),
			"errors_count", len(errors),
			"errors", errors,
			logging.PriorityCritical())
		return nil, fmt.Errorf("failed to clean up registrants: %w", errors)
	}

	slog.InfoContext(ctx, "successfully cleaned up all registrants for deleted meeting", "registrant_count", len(registrants))
	return []byte("success"), nil
}

// HandleMeetingCreated is the message handler for the meeting-created subject.
// It performs post-creation tasks like committee member synchronization.
func (s *MeetingHandler) HandleMeetingCreated(ctx context.Context, msg domain.Message) ([]byte, error) {
	if !s.meetingService.ServiceReady() || !s.committeeSyncService.ServiceReady() {
		slog.ErrorContext(ctx, "service not ready")
		return nil, fmt.Errorf("service not ready")
	}

	// Parse the meeting created message
	var meetingCreatedMsg models.MeetingCreatedMessage
	err := json.Unmarshal(msg.Data(), &meetingCreatedMsg)
	if err != nil {
		slog.ErrorContext(ctx, "error unmarshaling meeting created message", logging.ErrKey, err)
		return nil, err
	}

	if meetingCreatedMsg.MeetingUID == "" || meetingCreatedMsg.Base == nil {
		slog.WarnContext(ctx, "invalid meeting created message: missing required fields")
		return nil, fmt.Errorf("meeting UID and base data are required")
	}

	ctx = logging.AppendCtx(ctx, slog.String("meeting_uid", meetingCreatedMsg.MeetingUID))
	slog.InfoContext(ctx, "processing meeting creation post-tasks")

	// Check if the meeting has committees that need syncing
	if len(meetingCreatedMsg.Base.Committees) > 0 {
		slog.InfoContext(ctx, "meeting has committees, starting committee member sync",
			"committee_count", len(meetingCreatedMsg.Base.Committees))

		// Use the new CommitteeSyncService to handle committee member sync
		// For meeting creation, we sync from empty committees to new committees
		isPublicMeeting := meetingCreatedMsg.Base.IsPublic()
		err := s.committeeSyncService.SyncCommittees(
			ctx,
			meetingCreatedMsg.MeetingUID,
			[]models.Committee{}, // No old committees for creation
			meetingCreatedMsg.Base.Committees,
			isPublicMeeting,
		)
		if err != nil {
			// Log error but don't fail the entire handler - committee sync is non-critical
			slog.ErrorContext(ctx, "committee member sync failed", logging.ErrKey, err)
		} else {
			slog.InfoContext(ctx, "committee member sync completed successfully")
		}
	} else {
		slog.DebugContext(ctx, "no committees to sync for this meeting")
	}

	return []byte("success"), nil
}

// HandleMeetingUpdated is the message handler for the meeting-updated subject.
// It performs post-update tasks like committee member synchronization changes.
func (s *MeetingHandler) HandleMeetingUpdated(ctx context.Context, msg domain.Message) ([]byte, error) {
	if !s.meetingService.ServiceReady() || !s.committeeSyncService.ServiceReady() {
		slog.ErrorContext(ctx, "service not ready")
		return nil, fmt.Errorf("service not ready")
	}

	// Parse the meeting updated message
	var meetingUpdatedMsg models.MeetingUpdatedMessage
	err := json.Unmarshal(msg.Data(), &meetingUpdatedMsg)
	if err != nil {
		slog.ErrorContext(ctx, "error unmarshaling meeting updated message", logging.ErrKey, err)
		return nil, err
	}

	if meetingUpdatedMsg.MeetingUID == "" {
		slog.WarnContext(ctx, "invalid meeting updated message: missing meeting UID")
		return nil, fmt.Errorf("meeting UID is required")
	}

	ctx = logging.AppendCtx(ctx, slog.String("meeting_uid", meetingUpdatedMsg.MeetingUID))
	slog.InfoContext(ctx, "processing meeting update post-tasks")

	// Handle update notifications to registrants
	if len(meetingUpdatedMsg.Changes) > 0 {
		slog.DebugContext(ctx, "meaningful changes detected, notifying registrants", "changes", meetingUpdatedMsg.Changes)
		err = s.meetingUpdatedInvitations(ctx, meetingUpdatedMsg)
		if err != nil {
			slog.ErrorContext(ctx, "error sending update notifications to registrants", logging.ErrKey, err)
			return nil, fmt.Errorf("failed to send update notifications: %w", err)
		}
	}

	// Handle committee changes using the new CommitteeSyncService (only if we have the required fields)
	if meetingUpdatedMsg.UpdatedBase != nil && meetingUpdatedMsg.PreviousBase != nil {
		isPublicMeeting := meetingUpdatedMsg.UpdatedBase.IsPublic()
		err = s.committeeSyncService.SyncCommittees(
			ctx,
			meetingUpdatedMsg.MeetingUID,
			meetingUpdatedMsg.PreviousBase.Committees,
			meetingUpdatedMsg.UpdatedBase.Committees,
			isPublicMeeting,
		)
		if err != nil {
			// Log error but don't fail the entire handler - committee sync is non-critical
			slog.ErrorContext(ctx, "committee change handling failed", logging.ErrKey, err)
		}
	}

	return []byte("success"), nil
}

func (s *MeetingHandler) meetingUpdatedInvitations(ctx context.Context, msg models.MeetingUpdatedMessage) error {
	// Get all registrants for the meeting
	registrants, err := s.registrantService.RegistrantRepository.ListByMeeting(ctx, msg.MeetingUID)
	if err != nil {
		slog.ErrorContext(ctx, "error getting registrants for updated meeting", logging.ErrKey, err)
		return err
	}

	if len(registrants) == 0 {
		slog.DebugContext(ctx, "no registrants to notify for updated meeting")
		return nil
	}

	slog.DebugContext(ctx, "sending update notifications to registrants", "registrant_count", len(registrants))

	// Process registrants concurrently using WorkerPool
	var tasks []func() error
	for _, registrant := range registrants {
		reg := registrant // capture loop variable
		tasks = append(tasks, func() error {
			// Get meeting details for the email
			meeting, err := s.meetingService.MeetingRepository.GetBase(ctx, msg.MeetingUID)
			if err != nil {
				slog.ErrorContext(ctx, "error getting meeting details for update notification",
					"registrant_uid", reg.UID, logging.ErrKey, err)
				return err
			}

			// Build recipient name from first and last name
			var recipientName string
			if reg.FirstName != "" || reg.LastName != "" {
				recipientName = strings.TrimSpace(fmt.Sprintf("%s %s", reg.FirstName, reg.LastName))
			}

			// Extract meeting ID and passcode from Zoom config
			var meetingID, passcode string
			if meeting.ZoomConfig != nil {
				meetingID = meeting.ZoomConfig.MeetingID
				passcode = meeting.ZoomConfig.Passcode
			}

			// Send update notification email to registrant
			updatedInvitation := domain.EmailUpdatedInvitation{
				MeetingUID:     msg.MeetingUID,
				RecipientEmail: reg.Email,
				RecipientName:  recipientName,
				MeetingTitle:   meeting.Title,
				StartTime:      meeting.StartTime,
				Duration:       meeting.Duration,
				Timezone:       meeting.Timezone,
				Description:    meeting.Description,
				JoinLink:       constants.GenerateLFXMeetingURL(meeting.UID, meeting.Password),
				MeetingID:      meetingID,
				Passcode:       passcode,
				Recurrence:     meeting.Recurrence,
				Changes:        msg.Changes,
			}

			err = s.registrantService.EmailService.SendRegistrantUpdatedInvitation(ctx, updatedInvitation)
			if err != nil {
				slog.ErrorContext(ctx, "error sending update notification email",
					"registrant_uid", reg.UID, "email", reg.Email, logging.ErrKey, err)
				return err
			}

			slog.DebugContext(ctx, "update notification sent successfully", "registrant_uid", reg.UID, "email", reg.Email)
			return nil
		})
	}

	// Execute all notification operations concurrently using WorkerPool
	pool := concurrent.NewWorkerPool(10) // Use 10 workers for concurrent processing
	errors := pool.RunAll(ctx, tasks...)
	if len(errors) > 0 {
		slog.ErrorContext(ctx, "some notification operations failed",
			"meeting_uid", msg.MeetingUID,
			"total_registrants", len(registrants),
			"errors_count", len(errors),
			"errors", errors,
			logging.PriorityCritical())
		return fmt.Errorf("failed to send update notifications: %w", errors)
	}

	slog.InfoContext(ctx, "successfully sent update notifications to all registrants", "registrant_count", len(registrants))
	return nil
}
