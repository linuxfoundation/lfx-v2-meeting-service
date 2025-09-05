// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/service"
)

// CommitteeHandlers handles committee-related messages and events.
type CommitteeHandlers struct {
	meetingService    *service.MeetingService
	registrantService *service.MeetingRegistrantService
	messageBuilder    domain.MessageBuilder
}

// NewCommitteeHandlers creates a new committee handlers instance.
func NewCommitteeHandlers(
	meetingService *service.MeetingService,
	registrantService *service.MeetingRegistrantService,
	messageBuilder domain.MessageBuilder,
) *CommitteeHandlers {
	return &CommitteeHandlers{
		meetingService:    meetingService,
		registrantService: registrantService,
		messageBuilder:    messageBuilder,
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

	// Parse the committee member created message
	var committeeMemberMsg models.CommitteeMemberCreatedMessage
	err := json.Unmarshal(msg.Data(), &committeeMemberMsg)
	if err != nil {
		slog.ErrorContext(ctx, "error unmarshaling committee member created message", logging.ErrKey, err)
		return nil, err
	}

	if committeeMemberMsg.CommitteeUID == "" || committeeMemberMsg.Email == "" {
		slog.WarnContext(ctx, "invalid committee member created message: missing required fields")
		return nil, fmt.Errorf("committee UID and member email are required")
	}

	ctx = logging.AppendCtx(ctx, slog.String("committee_uid", committeeMemberMsg.CommitteeUID))
	ctx = logging.AppendCtx(ctx, slog.String("member_email", committeeMemberMsg.Email))
	ctx = logging.AppendCtx(ctx, slog.String("voting_status", committeeMemberMsg.Voting.Status))

	slog.InfoContext(ctx, "processing new committee member, checking for relevant meetings")

	// Find meetings that include this committee and match the voting status
	err = h.addMemberToRelevantMeetings(ctx, &committeeMemberMsg)
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

	// Parse the committee member deleted message
	var committeeMemberMsg models.CommitteeMemberDeletedMessage
	err := json.Unmarshal(msg.Data(), &committeeMemberMsg)
	if err != nil {
		slog.ErrorContext(ctx, "error unmarshaling committee member deleted message", logging.ErrKey, err)
		return nil, err
	}

	if committeeMemberMsg.CommitteeUID == "" || committeeMemberMsg.Email == "" {
		slog.WarnContext(ctx, "invalid committee member deleted message: missing required fields")
		return nil, fmt.Errorf("committee UID and member email are required")
	}

	ctx = logging.AppendCtx(ctx, slog.String("committee_uid", committeeMemberMsg.CommitteeUID))
	ctx = logging.AppendCtx(ctx, slog.String("member_email", committeeMemberMsg.Email))
	ctx = logging.AppendCtx(ctx, slog.String("voting_status", committeeMemberMsg.Voting.Status))

	slog.InfoContext(ctx, "processing deleted committee member, checking for relevant meetings")

	// Find meetings that include this committee and remove/convert the member
	err = h.removeMemberFromRelevantMeetings(ctx, &committeeMemberMsg)
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
func (h *CommitteeHandlers) addMemberToRelevantMeetings(ctx context.Context, memberMsg *models.CommitteeMemberCreatedMessage) error {
	// TODO: Implement finding meetings with this committee and adding the member
	// This will involve:
	// 1. Search all meetings that include this committee UID in their committees list
	// 2. For each matching meeting:
	//    - Check if member's voting status is in the committee's AllowedVotingStatuses
	//    - If yes, check if registrant already exists (by email to avoid duplicates)
	//    - If not exists, create registrant record with:
	//      - email from memberMsg.MemberEmail
	//      - name from memberMsg.MemberName (or constructed from first/last name)
	//      - username from memberMsg.MemberUsername
	//      - type = "committee"
	//      - committee_uid = memberMsg.CommitteeUID
	//    - Send appropriate NATS messages for indexing/access control
	// 3. Handle any errors gracefully and log results

	slog.InfoContext(ctx, "committee member addition placeholder - implementation pending",
		"committee_uid", memberMsg.CommitteeUID,
		"member_email", memberMsg.Email,
		"voting_status", memberMsg.Voting.Status)

	// For now, just log what would be processed
	slog.DebugContext(ctx, "would search for meetings with committee and add member",
		"committee_uid", memberMsg.CommitteeUID,
		"member_username", memberMsg.Username,
		"member_email", memberMsg.Email,
		"member_first_name", memberMsg.FirstName,
		"member_last_name", memberMsg.LastName,
		"voting_status", memberMsg.Voting.Status)

	return nil
}

// removeMemberFromRelevantMeetings finds all meetings that include the specified committee
// and removes or converts the member's registrant based on meeting visibility.
func (h *CommitteeHandlers) removeMemberFromRelevantMeetings(ctx context.Context, memberMsg *models.CommitteeMemberDeletedMessage) error {
	// TODO: Implement finding meetings with this committee and removing/converting the member
	// This will involve:
	// 1. Search all meetings that include this committee UID in their committees list
	// 2. For each matching meeting:
	//    - Find registrants with type="committee" matching this member's email
	//    - Check if the registrant's committee_uid matches this committee (in case they're in multiple committees)
	//    - For each matching registrant:
	//      - If meeting is public (visibility="public"):
	//        * Update registrant type from "committee" to "direct"
	//        * Keep all other registrant data intact
	//        * Send update messages for indexing/access control
	//      - If meeting is private:
	//        * Delete the registrant entirely
	//        * Send delete messages for indexing/access control
	//        * Send email cancellation notification if configured
	// 3. Handle any errors gracefully and log results
	// 4. Consider edge case: member might be registered both as committee and direct registrant

	slog.InfoContext(ctx, "committee member removal placeholder - implementation pending",
		"committee_uid", memberMsg.CommitteeUID,
		"member_email", memberMsg.Email,
		"voting_status", memberMsg.Voting.Status)

	// For now, just log what would be processed
	slog.DebugContext(ctx, "would search for meetings with committee and remove/convert member",
		"committee_uid", memberMsg.CommitteeUID,
		"member_username", memberMsg.Username,
		"member_email", memberMsg.Email,
		"member_first_name", memberMsg.FirstName,
		"member_last_name", memberMsg.LastName,
		"voting_status", memberMsg.Voting.Status,
		"action_public_meetings", "convert to direct registrant",
		"action_private_meetings", "remove registrant entirely")

	return nil
}
