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

// HandleMeetingCreated is the message handler for the meeting-created subject.
// It performs post-creation tasks like committee member synchronization.
func (s *MeetingHandler) HandleMeetingCreated(ctx context.Context, msg domain.Message) ([]byte, error) {
	if !s.meetingService.ServiceReady() {
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

		err := s.syncCommitteeMembers(ctx, &meetingCreatedMsg)
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

// syncCommitteeMembers handles the synchronization of committee members as registrants.
// This is a placeholder implementation that will be completed when committee-api contract is available.
func (s *MeetingHandler) syncCommitteeMembers(ctx context.Context, meetingMsg *models.MeetingCreatedMessage) error {
	// TODO: Implement committee member fetching and registrant creation
	// This will involve:
	// 1. For each committee in meetingMsg.Base.Committees:
	//    - Send request to committee-api to get members filtered by AllowedVotingStatuses
	//    - Parse response and extract member details
	// 2. For each committee member:
	//    - Check if registrant already exists (by email to avoid duplicates)
	//    - Create registrant record if not exists
	//    - Send appropriate NATS messages for indexing/access control

	slog.InfoContext(ctx, "committee member sync placeholder - implementation pending committee-api contract",
		"committee_count", len(meetingMsg.Base.Committees))

	// For now, just log the committees that would be processed
	for i, committee := range meetingMsg.Base.Committees {
		slog.DebugContext(ctx, "would process committee",
			"committee_index", i,
			"committee_uid", committee.UID,
			"allowed_voting_statuses", committee.AllowedVotingStatuses)
	}

	return nil
}

// HandleMeetingUpdated is the message handler for the meeting-updated subject.
// It performs post-update tasks like committee member synchronization changes.
func (s *MeetingHandler) HandleMeetingUpdated(ctx context.Context, msg domain.Message) ([]byte, error) {
	if !s.meetingService.ServiceReady() {
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

	if meetingUpdatedMsg.MeetingUID == "" || meetingUpdatedMsg.UpdatedBase == nil || meetingUpdatedMsg.PreviousBase == nil {
		slog.WarnContext(ctx, "invalid meeting updated message: missing required fields")
		return nil, fmt.Errorf("meeting UID, updated base, and previous base data are required")
	}

	ctx = logging.AppendCtx(ctx, slog.String("meeting_uid", meetingUpdatedMsg.MeetingUID))
	slog.InfoContext(ctx, "processing meeting update post-tasks")

	// Check if committees have changed and handle committee member sync
	err = s.handleCommitteeChanges(ctx, &meetingUpdatedMsg)
	if err != nil {
		// Log error but don't fail the entire handler - committee sync is non-critical
		slog.ErrorContext(ctx, "committee change handling failed", logging.ErrKey, err)
	}

	return []byte("success"), nil
}

// handleCommitteeChanges manages committee member synchronization when committees change.
func (s *MeetingHandler) handleCommitteeChanges(ctx context.Context, meetingMsg *models.MeetingUpdatedMessage) error {
	oldCommittees := meetingMsg.PreviousBase.Committees
	newCommittees := meetingMsg.UpdatedBase.Committees

	// Create maps for easier comparison
	oldCommitteeMap := make(map[string]models.Committee)
	for _, committee := range oldCommittees {
		oldCommitteeMap[committee.UID] = committee
	}

	newCommitteeMap := make(map[string]models.Committee)
	for _, committee := range newCommittees {
		newCommitteeMap[committee.UID] = committee
	}

	// Check if there are any changes
	if len(oldCommittees) == 0 && len(newCommittees) == 0 {
		slog.DebugContext(ctx, "no committees in old or new meeting - no changes to process")
		return nil
	}

	hasChanges := false

	// Check for added committees
	var addedCommittees []models.Committee
	for _, committee := range newCommittees {
		if _, exists := oldCommitteeMap[committee.UID]; !exists {
			addedCommittees = append(addedCommittees, committee)
			hasChanges = true
		}
	}

	// Check for removed committees
	var removedCommittees []models.Committee
	for _, committee := range oldCommittees {
		if _, exists := newCommitteeMap[committee.UID]; !exists {
			removedCommittees = append(removedCommittees, committee)
			hasChanges = true
		}
	}

	// Check for committees with changed voting statuses
	var changedCommittees []struct {
		Old models.Committee
		New models.Committee
	}
	for _, newCommittee := range newCommittees {
		if oldCommittee, exists := oldCommitteeMap[newCommittee.UID]; exists {
			// Compare voting statuses
			if !equalStringSlices(oldCommittee.AllowedVotingStatuses, newCommittee.AllowedVotingStatuses) {
				changedCommittees = append(changedCommittees, struct {
					Old models.Committee
					New models.Committee
				}{Old: oldCommittee, New: newCommittee})
				hasChanges = true
			}
		}
	}

	if !hasChanges {
		slog.DebugContext(ctx, "no committee changes detected")
		return nil
	}

	isPublicMeeting := meetingMsg.UpdatedBase.Visibility == "public"

	slog.InfoContext(ctx, "committee changes detected, processing member sync",
		"added_committees", len(addedCommittees),
		"removed_committees", len(removedCommittees),
		"changed_committees", len(changedCommittees),
		"is_public_meeting", isPublicMeeting)

	// Handle added committees
	if len(addedCommittees) > 0 {
		err := s.handleAddedCommittees(ctx, meetingMsg, addedCommittees)
		if err != nil {
			slog.ErrorContext(ctx, "failed to handle added committees", logging.ErrKey, err)
		}
	}

	// Handle removed committees
	if len(removedCommittees) > 0 {
		err := s.handleRemovedCommittees(ctx, meetingMsg, removedCommittees, isPublicMeeting)
		if err != nil {
			slog.ErrorContext(ctx, "failed to handle removed committees", logging.ErrKey, err)
		}
	}

	// Handle committees with changed voting statuses
	if len(changedCommittees) > 0 {
		err := s.handleChangedCommittees(ctx, meetingMsg, changedCommittees)
		if err != nil {
			slog.ErrorContext(ctx, "failed to handle changed committees", logging.ErrKey, err)
		}
	}

	return nil
}

// handleAddedCommittees processes committees that were added to the meeting.
func (s *MeetingHandler) handleAddedCommittees(ctx context.Context, meetingMsg *models.MeetingUpdatedMessage, addedCommittees []models.Committee) error {
	// TODO: Implement adding committee members as registrants
	// This will involve:
	// 1. For each added committee:
	//    - Send request to committee-api to get members filtered by AllowedVotingStatuses
	//    - Parse response and extract member details
	// 2. For each committee member:
	//    - Check if registrant already exists (by email to avoid duplicates)
	//    - Create registrant record with type="committee" if not exists
	//    - Send appropriate NATS messages for indexing/access control

	slog.InfoContext(ctx, "added committees sync placeholder - implementation pending committee-api contract",
		"added_committee_count", len(addedCommittees))

	for i, committee := range addedCommittees {
		slog.DebugContext(ctx, "would add committee members",
			"committee_index", i,
			"committee_uid", committee.UID,
			"allowed_voting_statuses", committee.AllowedVotingStatuses)
	}

	return nil
}

// handleRemovedCommittees processes committees that were removed from the meeting.
func (s *MeetingHandler) handleRemovedCommittees(ctx context.Context, meetingMsg *models.MeetingUpdatedMessage, removedCommittees []models.Committee, isPublicMeeting bool) error {
	// TODO: Implement removing/updating committee members as registrants
	// This will involve:
	// 1. For each removed committee:
	//    - Find all registrants with type="committee" for this committee
	// 2. For each committee member registrant:
	//    - If meeting is public: update registrant to type="direct" (keep them registered)
	//    - If meeting is private: remove registrant entirely
	//    - Send appropriate NATS messages for indexing/access control

	action := "remove"
	if isPublicMeeting {
		action = "convert to direct"
	}

	slog.InfoContext(ctx, "removed committees sync placeholder - implementation pending committee-api contract",
		"removed_committee_count", len(removedCommittees),
		"action", action)

	for i, committee := range removedCommittees {
		slog.DebugContext(ctx, "would process removed committee",
			"committee_index", i,
			"committee_uid", committee.UID,
			"action", action)
	}

	return nil
}

// handleChangedCommittees processes committees that had their voting statuses changed.
func (s *MeetingHandler) handleChangedCommittees(ctx context.Context, meetingMsg *models.MeetingUpdatedMessage, changedCommittees []struct{ Old, New models.Committee }) error {
	// TODO: Implement updating committee member registrants for voting status changes
	// This will involve:
	// 1. For each changed committee:
	//    - Get current committee members with old voting statuses
	//    - Get new committee members with new voting statuses
	//    - Find members that should be removed (no longer match voting status)
	//    - Find members that should be added (now match voting status)
	//    - Process removals and additions similar to removed/added committees

	slog.InfoContext(ctx, "changed committees sync placeholder - implementation pending committee-api contract",
		"changed_committee_count", len(changedCommittees))

	for i, change := range changedCommittees {
		slog.DebugContext(ctx, "would process changed committee",
			"committee_index", i,
			"committee_uid", change.New.UID,
			"old_voting_statuses", change.Old.AllowedVotingStatuses,
			"new_voting_statuses", change.New.AllowedVotingStatuses)
	}

	return nil
}

// equalStringSlices compares two string slices for equality (order-independent).
func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	// Create frequency maps
	mapA := make(map[string]int)
	mapB := make(map[string]int)

	for _, str := range a {
		mapA[str]++
	}
	for _, str := range b {
		mapB[str]++
	}

	// Compare maps
	for key, count := range mapA {
		if mapB[key] != count {
			return false
		}
	}

	return true
}
