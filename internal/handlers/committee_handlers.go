// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/service"
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
	return h.meetingService.ServiceReady() && h.registrantService.ServiceReady()
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
	ctx = logging.AppendCtx(ctx, slog.String("member_email", committeeMemberMsgData.Email))
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
	ctx = logging.AppendCtx(ctx, slog.String("member_email", committeeMemberMsgData.Email))
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
	var allErrors []error
	successCount := 0

	for _, meeting := range meetings {
		if meeting == nil {
			continue
		}

		// Find the committee configuration for this meeting
		var committeeConfig *models.Committee
		for _, committee := range meeting.Committees {
			if committee.UID == memberMsg.CommitteeUID {
				committeeConfig = &committee
				break
			}
		}

		if committeeConfig == nil {
			// This shouldn't happen since we filtered by committee UID
			slog.WarnContext(ctx, "committee not found in meeting",
				"meeting_uid", meeting.UID,
				"committee_uid", memberMsg.CommitteeUID)
			continue
		}

		// Check if member's voting status matches allowed statuses
		if len(committeeConfig.AllowedVotingStatuses) > 0 &&
			!slices.Contains(committeeConfig.AllowedVotingStatuses, memberMsg.Voting.Status) {
			slog.DebugContext(ctx, "member voting status not allowed for meeting",
				"meeting_uid", meeting.UID,
				"committee_uid", memberMsg.CommitteeUID,
				"member_voting_status", memberMsg.Voting.Status,
				"allowed_voting_statuses", committeeConfig.AllowedVotingStatuses)
			continue
		}

		// Add the member to this meeting using the committee sync service
		err := h.committeeSyncService.AddCommitteeMembersAsRegistrants(
			ctx,
			meeting.UID,
			memberMsg.CommitteeUID,
			[]models.CommitteeMember{committeeMember},
		)
		if err != nil {
			slog.ErrorContext(ctx, "failed to add committee member to meeting",
				"meeting_uid", meeting.UID,
				"committee_uid", memberMsg.CommitteeUID,
				"member_email", memberMsg.Email,
				logging.ErrKey, err)
			allErrors = append(allErrors, fmt.Errorf("failed to add member to meeting %s: %w", meeting.UID, err))
		} else {
			successCount++
			slog.InfoContext(ctx, "successfully added committee member to meeting",
				"meeting_uid", meeting.UID,
				"committee_uid", memberMsg.CommitteeUID,
				"member_email", memberMsg.Email)
		}
	}

	slog.InfoContext(ctx, "completed committee member addition to meetings",
		"committee_uid", memberMsg.CommitteeUID,
		"member_email", memberMsg.Email,
		"total_meetings", len(meetings),
		"successful_additions", successCount,
		"failed_additions", len(allErrors))

	if len(allErrors) > 0 {
		return fmt.Errorf("failed to add committee member to %d meetings: %v", len(allErrors), allErrors)
	}

	return nil
}

// removeMemberFromRelevantMeetings finds all meetings that include the specified committee
// and removes or converts the member's registrant based on meeting visibility.
func (h *CommitteeHandlers) removeMemberFromRelevantMeetings(ctx context.Context, memberMsg *models.CommitteeMember) error {
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

	slog.InfoContext(ctx, "found meetings for committee member removal",
		"committee_uid", memberMsg.CommitteeUID,
		"member_email", memberMsg.Email,
		"voting_status", memberMsg.Voting.Status,
		"meetings_count", len(meetings))

	// Process each meeting that contains this committee
	var allErrors []error
	successCount := 0

	for _, meeting := range meetings {
		if meeting == nil {
			continue
		}

		// Find the committee configuration for this meeting to get voting status requirements
		var committeeConfig *models.Committee
		for _, committee := range meeting.Committees {
			if committee.UID == memberMsg.CommitteeUID {
				committeeConfig = &committee
				break
			}
		}

		if committeeConfig == nil {
			// This shouldn't happen since we filtered by committee UID
			slog.WarnContext(ctx, "committee not found in meeting",
				"meeting_uid", meeting.UID,
				"committee_uid", memberMsg.CommitteeUID)
			continue
		}

		// Check if this member would have been eligible (had the right voting status)
		// If they weren't eligible, they wouldn't be registered, so skip
		if len(committeeConfig.AllowedVotingStatuses) > 0 &&
			!slices.Contains(committeeConfig.AllowedVotingStatuses, memberMsg.Voting.Status) {
			slog.DebugContext(ctx, "member voting status was not allowed for meeting, skipping",
				"meeting_uid", meeting.UID,
				"committee_uid", memberMsg.CommitteeUID,
				"member_voting_status", memberMsg.Voting.Status,
				"allowed_voting_statuses", committeeConfig.AllowedVotingStatuses)
			continue
		}

		// Find and remove/convert registrants for this committee member
		err := h.removeCommitteeMemberFromMeeting(ctx, meeting, memberMsg)
		if err != nil {
			slog.ErrorContext(ctx, "failed to remove committee member from meeting",
				"meeting_uid", meeting.UID,
				"committee_uid", memberMsg.CommitteeUID,
				"member_email", memberMsg.Email,
				logging.ErrKey, err)
			allErrors = append(allErrors, fmt.Errorf("failed to remove member from meeting %s: %w", meeting.UID, err))
		} else {
			successCount++
			slog.InfoContext(ctx, "successfully processed committee member removal from meeting",
				"meeting_uid", meeting.UID,
				"committee_uid", memberMsg.CommitteeUID,
				"member_email", memberMsg.Email)
		}
	}

	slog.InfoContext(ctx, "completed committee member removal from meetings",
		"committee_uid", memberMsg.CommitteeUID,
		"member_email", memberMsg.Email,
		"total_meetings", len(meetings),
		"successful_removals", successCount,
		"failed_removals", len(allErrors))

	if len(allErrors) > 0 {
		return fmt.Errorf("failed to remove committee member from %d meetings: %v", len(allErrors), allErrors)
	}

	return nil
}

// removeCommitteeMemberFromMeeting removes or converts a specific committee member from a meeting
func (h *CommitteeHandlers) removeCommitteeMemberFromMeeting(ctx context.Context, meeting *models.MeetingBase, memberMsg *models.CommitteeMember) error {
	isPublicMeeting := meeting.Visibility == "public"

	// Use the committee sync service's proper method for removing a committee member
	err := h.committeeSyncService.RemoveCommitteeMemberFromMeeting(
		ctx,
		meeting.UID,
		memberMsg.CommitteeUID,
		memberMsg.Email,
		isPublicMeeting,
	)
	if err != nil {
		return fmt.Errorf("failed to remove committee member from meeting: %w", err)
	}

	return nil
}
