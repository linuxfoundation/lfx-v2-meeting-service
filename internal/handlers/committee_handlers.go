// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"sync/atomic"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/service"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/concurrent"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/redaction"
)

// CommitteeHandlers handles committee-related messages and events.
type CommitteeHandlers struct {
	meetingService       *service.MeetingService
	registrantService    *service.MeetingRegistrantService
	committeeSyncService *service.CommitteeSyncService
	messageBuilder       domain.MessageBuilder
}

// NewCommitteeHandlers creates a new committee handlers instance.
func NewCommitteeHandlers(
	meetingService *service.MeetingService,
	registrantService *service.MeetingRegistrantService,
	committeeSyncService *service.CommitteeSyncService,
	messageBuilder domain.MessageBuilder,
) *CommitteeHandlers {
	return &CommitteeHandlers{
		meetingService:       meetingService,
		registrantService:    registrantService,
		committeeSyncService: committeeSyncService,
		messageBuilder:       messageBuilder,
	}
}

func (h *CommitteeHandlers) HandlerReady() bool {
	return h.meetingService.ServiceReady() && h.registrantService.ServiceReady() &&
		h.committeeSyncService.ServiceReady()
}

// HandleMessage implements domain.MessageHandler interface
func (h *CommitteeHandlers) HandleMessage(ctx context.Context, msg domain.Message) {
	subject := msg.Subject()
	ctx = logging.AppendCtx(ctx, slog.String("subject", subject))
	slog.DebugContext(ctx, "handling committee NATS message")

	var response []byte
	var err error

	handlers := map[string]func(ctx context.Context, msg domain.Message) ([]byte, error){
		models.CommitteeMemberCreatedSubject: h.HandleCommitteeMemberCreated,
		models.CommitteeMemberDeletedSubject: h.HandleCommitteeMemberDeleted,
		models.CommitteeMemberUpdatedSubject: h.HandleCommitteeMemberUpdated,
	}

	handler, ok := handlers[subject]
	if !ok {
		slog.WarnContext(ctx, "unknown committee message subject", "subject", subject)
		return
	}

	response, err = handler(ctx, msg)
	if err != nil {
		slog.ErrorContext(ctx, "error handling committee message", logging.ErrKey, err)
	} else {
		slog.DebugContext(ctx, "committee message handled successfully", "response", string(response))
	}
}

// HandleCommitteeMemberCreated is the message handler for the committee-member-created subject.
// It processes new committee members and adds them to relevant meetings as registrants.
func (h *CommitteeHandlers) HandleCommitteeMemberCreated(ctx context.Context, msg domain.Message) ([]byte, error) {
	if !h.meetingService.ServiceReady() {
		slog.ErrorContext(ctx, "service not ready")
		return nil, fmt.Errorf("service not ready")
	}

	slog.DebugContext(ctx, "handling committee member created message", "message", string(msg.Data()))

	// Parse the committee member created message
	var committeeMemberMsg models.CommitteeEvent
	err := json.Unmarshal(msg.Data(), &committeeMemberMsg)
	if err != nil {
		slog.ErrorContext(ctx, "error unmarshaling committee member created message", logging.ErrKey, err)
		return nil, err
	}

	// The Data field comes as map[string]interface{} after JSON unmarshaling,
	// so we need to marshal it back to JSON and then unmarshal into the proper struct
	var committeeMemberMsgData models.CommitteeMember
	dataBytes, err := json.Marshal(committeeMemberMsg.Data)
	if err != nil {
		slog.ErrorContext(ctx, "error marshaling committee member data", logging.ErrKey, err)
		return nil, fmt.Errorf("failed to marshal committee member data: %w", err)
	}

	err = json.Unmarshal(dataBytes, &committeeMemberMsgData)
	if err != nil {
		slog.ErrorContext(ctx, "error unmarshaling committee member created message data", logging.ErrKey, err)
		return nil, fmt.Errorf("failed to unmarshal committee member data: %w", err)
	}

	if committeeMemberMsgData.CommitteeUID == "" || committeeMemberMsgData.Email == "" {
		slog.WarnContext(ctx, "invalid committee member created message: missing required fields")
		return nil, fmt.Errorf("committee UID and member email are required")
	}

	ctx = logging.AppendCtx(ctx, slog.String("committee_uid", committeeMemberMsgData.CommitteeUID))
	ctx = logging.AppendCtx(ctx, slog.String("member_email", redaction.RedactEmail(committeeMemberMsgData.Email)))
	ctx = logging.AppendCtx(ctx, slog.String("voting_status", committeeMemberMsgData.Voting.Status))

	slog.InfoContext(ctx, "processing new committee member, checking for relevant meetings")

	// Find meetings that include this committee and match the voting status
	err = h.addMemberToRelevantMeetings(ctx, &committeeMemberMsgData)
	if err != nil {
		// Log error but don't fail the entire handler - member addition is non-critical for other services
		slog.ErrorContext(ctx, "failed to add committee member to meetings", logging.ErrKey, err)
	} else {
		slog.InfoContext(ctx, "successfully processed new committee member for meetings")
	}

	return []byte("success"), nil
}

// HandleCommitteeMemberUpdated is the message handler for the committee-member-updated subject.
// It processes updated committee members and handles email changes by updating registrations.
func (h *CommitteeHandlers) HandleCommitteeMemberUpdated(ctx context.Context, msg domain.Message) ([]byte, error) {
	if !h.meetingService.ServiceReady() {
		slog.ErrorContext(ctx, "service not ready")
		return nil, fmt.Errorf("service not ready")
	}

	slog.DebugContext(ctx, "handling committee member updated message", "message", string(msg.Data()))

	// Parse the committee member updated message
	var committeeMemberMsg models.CommitteeEvent
	err := json.Unmarshal(msg.Data(), &committeeMemberMsg)
	if err != nil {
		slog.ErrorContext(ctx, "error unmarshaling committee member updated message", logging.ErrKey, err)
		return nil, err
	}

	// The Data field comes as map[string]interface{} after JSON unmarshaling,
	// so we need to marshal it back to JSON and then unmarshal into the proper struct
	var updateEventData models.CommitteeMemberUpdateEventData
	dataBytes, err := json.Marshal(committeeMemberMsg.Data)
	if err != nil {
		slog.ErrorContext(ctx, "error marshaling committee member update data", logging.ErrKey, err)
		return nil, fmt.Errorf("failed to marshal committee member update data: %w", err)
	}

	err = json.Unmarshal(dataBytes, &updateEventData)
	if err != nil {
		slog.ErrorContext(ctx, "error unmarshaling committee member update data", logging.ErrKey, err)
		return nil, fmt.Errorf("failed to unmarshal committee member update data: %w", err)
	}

	// Validate required fields
	if updateEventData.Member == nil || updateEventData.OldMember == nil {
		slog.WarnContext(ctx, "invalid committee member updated message: missing member data")
		return nil, fmt.Errorf("both old and new member data are required")
	}

	if updateEventData.Member.CommitteeUID == "" || updateEventData.Member.Email == "" {
		slog.WarnContext(ctx, "invalid committee member updated message: missing required fields")
		return nil, fmt.Errorf("committee UID and member email are required")
	}

	oldMember := updateEventData.OldMember
	newMember := updateEventData.Member

	ctx = logging.AppendCtx(ctx, slog.String("member_uid", updateEventData.MemberUID))
	ctx = logging.AppendCtx(ctx, slog.String("committee_uid", newMember.CommitteeUID))

	slog.InfoContext(ctx, "processing updated committee member",
		"old_email", oldMember.Email,
		"new_email", newMember.Email)

	// Check if email changed
	if oldMember.Email != newMember.Email {
		slog.InfoContext(ctx, "committee member email changed, updating registrations",
			"old_email", oldMember.Email,
			"new_email", newMember.Email)
		err = h.handleMemberEmailChange(ctx, oldMember, newMember)
		if err != nil {
			// Log error but don't fail the entire handler - member update is non-critical for other services
			slog.ErrorContext(ctx, "failed to handle committee member email change", logging.ErrKey, err)
		} else {
			slog.InfoContext(ctx, "successfully processed committee member email change")
		}
	} else {
		slog.DebugContext(ctx, "no email change detected for committee member")
	}

	return []byte("success"), nil
}

// HandleCommitteeMemberDeleted is the message handler for the committee-member-deleted subject.
// It processes deleted committee members and removes/converts them in relevant meetings.
func (h *CommitteeHandlers) HandleCommitteeMemberDeleted(ctx context.Context, msg domain.Message) ([]byte, error) {
	if !h.meetingService.ServiceReady() {
		slog.ErrorContext(ctx, "service not ready")
		return nil, fmt.Errorf("service not ready")
	}

	slog.DebugContext(ctx, "handling committee member deleted message", "message", string(msg.Data()))

	// Parse the committee member deleted message
	var committeeMemberMsg models.CommitteeEvent
	err := json.Unmarshal(msg.Data(), &committeeMemberMsg)
	if err != nil {
		slog.ErrorContext(ctx, "error unmarshaling committee member deleted message", logging.ErrKey, err)
		return nil, err
	}

	// The Data field comes as map[string]interface{} after JSON unmarshaling,
	// so we need to marshal it back to JSON and then unmarshal into the proper struct
	var committeeMemberMsgData models.CommitteeMember
	dataBytes, err := json.Marshal(committeeMemberMsg.Data)
	if err != nil {
		slog.ErrorContext(ctx, "error marshaling committee member data", logging.ErrKey, err)
		return nil, fmt.Errorf("failed to marshal committee member data: %w", err)
	}

	err = json.Unmarshal(dataBytes, &committeeMemberMsgData)
	if err != nil {
		slog.ErrorContext(ctx, "error unmarshaling committee member deleted message data", logging.ErrKey, err)
		return nil, fmt.Errorf("failed to unmarshal committee member data: %w", err)
	}

	if committeeMemberMsgData.CommitteeUID == "" || committeeMemberMsgData.Email == "" {
		slog.WarnContext(ctx, "invalid committee member deleted message: missing required fields")
		return nil, fmt.Errorf("committee UID and member email are required")
	}

	ctx = logging.AppendCtx(ctx, slog.String("committee_uid", committeeMemberMsgData.CommitteeUID))
	ctx = logging.AppendCtx(ctx, slog.String("member_email", redaction.RedactEmail(committeeMemberMsgData.Email)))
	ctx = logging.AppendCtx(ctx, slog.String("voting_status", committeeMemberMsgData.Voting.Status))

	slog.InfoContext(ctx, "processing deleted committee member, checking for relevant meetings")

	// Find meetings that include this committee and remove/convert the member
	err = h.removeMemberFromRelevantMeetings(ctx, &committeeMemberMsgData)
	if err != nil {
		// Log error but don't fail the entire handler - member removal is non-critical for other services
		slog.ErrorContext(ctx, "failed to remove committee member from meetings", logging.ErrKey, err)
	} else {
		slog.InfoContext(ctx, "successfully processed deleted committee member for meetings")
	}

	return []byte("success"), nil
}

// addMemberToRelevantMeetings finds all meetings that include the specified committee
// and adds the new member as a registrant if their voting status matches.
func (h *CommitteeHandlers) addMemberToRelevantMeetings(ctx context.Context, memberMsg *models.CommitteeMember) error {
	// Get meetings that contain this committee
	meetings, _, err := h.meetingService.MeetingRepository.ListByCommittee(ctx, memberMsg.CommitteeUID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to list meetings by committee", logging.ErrKey, err)
		return fmt.Errorf("failed to list meetings by committee: %w", err)
	}

	if len(meetings) == 0 {
		slog.InfoContext(ctx, "no meetings found for committee",
			"committee_uid", memberMsg.CommitteeUID,
			"member_email", memberMsg.Email)
		return nil
	}

	slog.InfoContext(ctx, "found meetings for committee member addition",
		"committee_uid", memberMsg.CommitteeUID,
		"member_email", memberMsg.Email,
		"voting_status", memberMsg.Voting.Status,
		"meetings_count", len(meetings))

	// Create committee member for sync service
	committeeMember := models.CommitteeMember{
		UID:           memberMsg.UID,
		Username:      memberMsg.Username,
		Email:         memberMsg.Email,
		FirstName:     memberMsg.FirstName,
		LastName:      memberMsg.LastName,
		JobTitle:      memberMsg.JobTitle,
		Organization:  memberMsg.Organization,
		CommitteeUID:  memberMsg.CommitteeUID,
		CommitteeName: memberMsg.CommitteeName,
		Voting:        memberMsg.Voting,
	}

	// Process each meeting that contains this committee
	var successCount int64

	tasks := make([]func() error, len(meetings))
	for i := range meetings {
		meeting := meetings[i]
		tasks = append(tasks, func() error {
			err := h.tryAddMemberToMeeting(ctx, meeting, &committeeMember)
			if err != nil {
				return err
			}
			atomic.AddInt64(&successCount, 1)
			return nil
		})
	}

	workerPool := concurrent.NewWorkerPool(10)
	errors := workerPool.RunAll(ctx, tasks...)
	if len(errors) > 0 {
		// Log the errors but don't fail the entire operation
		slog.ErrorContext(ctx, "failed to add committee member to meetings",
			"committee_uid", memberMsg.CommitteeUID,
			"member_email", memberMsg.Email,
			"errors", errors,
			"errors_count", len(errors),
		)
	}

	slog.InfoContext(ctx, "completed committee member addition to meetings",
		"committee_uid", memberMsg.CommitteeUID,
		"member_email", memberMsg.Email,
		"total_meetings", len(meetings),
		"successful_additions", atomic.LoadInt64(&successCount),
		"failed_additions", len(errors))

	return nil
}

func (h *CommitteeHandlers) tryAddMemberToMeeting(ctx context.Context, meeting *models.MeetingBase, member *models.CommitteeMember) error {
	if meeting == nil {
		return nil
	}

	// Find the committee configuration for this meeting
	var committeeConfig *models.Committee
	for i, committee := range meeting.Committees {
		if committee.UID == member.CommitteeUID {
			committeeConfig = &meeting.Committees[i]
			break
		}
	}

	if committeeConfig == nil {
		// This shouldn't happen since we filtered by committee UID
		slog.WarnContext(ctx, "committee not found in meeting",
			"meeting_uid", meeting.UID,
			"committee_uid", member.CommitteeUID)
		return nil
	}

	// Check if member's voting status matches allowed statuses
	if len(committeeConfig.AllowedVotingStatuses) > 0 &&
		!slices.Contains(committeeConfig.AllowedVotingStatuses, member.Voting.Status) {
		slog.DebugContext(ctx, "member voting status not allowed for meeting",
			"meeting_uid", meeting.UID,
			"committee_uid", member.CommitteeUID,
			"member_voting_status", member.Voting.Status,
			"allowed_voting_statuses", committeeConfig.AllowedVotingStatuses)
		return nil
	}

	// Add the member to this meeting using the committee sync service
	err := h.committeeSyncService.AddCommitteeMembersAsRegistrants(
		ctx,
		meeting.UID,
		member.CommitteeUID,
		[]models.CommitteeMember{*member},
	)
	if err != nil {
		slog.ErrorContext(ctx, "failed to add committee member to meeting",
			"meeting_uid", meeting.UID,
			"committee_uid", member.CommitteeUID,
			"member_email", member.Email,
			logging.ErrKey, err)
		return fmt.Errorf("failed to add member to meeting %s: %w", meeting.UID, err)
	} else {
		slog.InfoContext(ctx, "successfully added committee member to meeting",
			"meeting_uid", meeting.UID,
			"committee_uid", member.CommitteeUID,
			"member_email", member.Email)
	}

	return nil
}

// removeMemberFromRelevantMeetings finds all meetings that include the specified committee
// and removes or converts the member's registrant based on meeting visibility.
func (h *CommitteeHandlers) removeMemberFromRelevantMeetings(ctx context.Context, member *models.CommitteeMember) error {
	// Get meetings that contain this committee
	meetings, _, err := h.meetingService.MeetingRepository.ListByCommittee(ctx, member.CommitteeUID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to list meetings by committee", logging.ErrKey, err)
		return fmt.Errorf("failed to list meetings by committee: %w", err)
	}

	if len(meetings) == 0 {
		slog.InfoContext(ctx, "no meetings found for committee",
			"committee_uid", member.CommitteeUID,
			"member_email", member.Email)
		return nil
	}

	slog.InfoContext(ctx, "found meetings for committee member removal",
		"committee_uid", member.CommitteeUID,
		"member_email", member.Email,
		"voting_status", member.Voting.Status,
		"meetings_count", len(meetings))

	// Process each meeting that contains this committee
	var successCount int64

	tasks := make([]func() error, len(meetings))
	for i := range meetings {
		meeting := meetings[i]
		tasks = append(tasks, func() error {
			err := h.tryRemoveMemberFromMeeting(ctx, meeting, member)
			if err != nil {
				return err
			}
			atomic.AddInt64(&successCount, 1)
			return nil
		})
	}

	workerPool := concurrent.NewWorkerPool(10)
	errors := workerPool.RunAll(ctx, tasks...)
	if len(errors) > 0 {
		// Log the errors but don't fail the entire operation
		slog.ErrorContext(ctx, "failed to remove committee member from meetings",
			"committee_uid", member.CommitteeUID,
			"member_email", member.Email,
			"errors", errors,
			"errors_count", len(errors),
		)
	}

	slog.InfoContext(ctx, "completed committee member removal from meetings",
		"committee_uid", member.CommitteeUID,
		"member_email", member.Email,
		"total_meetings", len(meetings),
		"successful_removals", atomic.LoadInt64(&successCount),
		"failed_removals", len(errors))

	return nil
}

func (h *CommitteeHandlers) tryRemoveMemberFromMeeting(ctx context.Context, meeting *models.MeetingBase, member *models.CommitteeMember) error {
	if meeting == nil {
		return nil
	}

	// Find the committee configuration for this meeting to get voting status requirements
	var committeeConfig *models.Committee
	for i, committee := range meeting.Committees {
		if committee.UID == member.CommitteeUID {
			committeeConfig = &meeting.Committees[i]
			break
		}
	}

	if committeeConfig == nil {
		// This shouldn't happen since we filtered by committee UID
		slog.WarnContext(ctx, "committee not found in meeting",
			"meeting_uid", meeting.UID,
			"committee_uid", member.CommitteeUID)
		return nil
	}

	// Check if this member would have been eligible (had the right voting status)
	// If they weren't eligible, they wouldn't be registered, so skip
	if len(committeeConfig.AllowedVotingStatuses) > 0 &&
		!slices.Contains(committeeConfig.AllowedVotingStatuses, member.Voting.Status) {
		slog.DebugContext(ctx, "member voting status was not allowed for meeting, skipping",
			"meeting_uid", meeting.UID,
			"committee_uid", member.CommitteeUID,
			"member_voting_status", member.Voting.Status,
			"allowed_voting_statuses", committeeConfig.AllowedVotingStatuses)
		return nil
	}

	// Find and remove/convert registrants for this committee member
	isPublicMeeting := meeting.IsPublic()
	err := h.committeeSyncService.RemoveCommitteeMemberFromMeeting(
		ctx,
		meeting,
		member.CommitteeUID,
		member.Email,
		isPublicMeeting,
	)
	if err != nil {
		slog.ErrorContext(ctx, "failed to remove committee member from meeting",
			"meeting_uid", meeting.UID,
			"committee_uid", member.CommitteeUID,
			"member_email", member.Email,
			logging.ErrKey, err)
		return fmt.Errorf("failed to remove committee member from meeting %s: %w", meeting.UID, err)
	}

	slog.InfoContext(ctx, "successfully processed committee member removal from meeting",
		"meeting_uid", meeting.UID,
		"committee_uid", member.CommitteeUID,
		"member_email", member.Email)
	return nil
}

// handleMemberEmailChange processes email changes for committee members across all relevant meetings
func (h *CommitteeHandlers) handleMemberEmailChange(ctx context.Context, oldMember, newMember *models.CommitteeMember) error {
	// Use the existing optimized method from committee sync service
	err := h.committeeSyncService.HandleCommitteeMemberEmailChangeForMeetings(
		ctx,
		oldMember.Email,
		newMember.Email,
		newMember.CommitteeUID,
		newMember,
	)
	if err != nil {
		slog.ErrorContext(ctx, "failed to handle committee member email change",
			"committee_uid", newMember.CommitteeUID,
			"old_email", oldMember.Email,
			"new_email", newMember.Email,
			logging.ErrKey, err)
		return fmt.Errorf("failed to handle committee member email change: %w", err)
	}

	slog.InfoContext(ctx, "successfully processed committee member email change",
		"committee_uid", newMember.CommitteeUID,
		"old_email", oldMember.Email,
		"new_email", newMember.Email)

	return nil
}
