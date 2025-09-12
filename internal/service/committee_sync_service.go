// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/concurrent"
)

// CommitteeChanges represents the differences between old and new committee configurations
type CommitteeChanges struct {
	Added   []models.Committee
	Removed []models.Committee
	Changed []CommitteeChange
}

// CommitteeChange represents a committee whose voting statuses have changed
type CommitteeChange struct {
	Old models.Committee
	New models.Committee
}

// CommitteeSyncService handles committee member synchronization logic
type CommitteeSyncService struct {
	registrantRepository domain.RegistrantRepository
	registrantService    *MeetingRegistrantService
	messageBuilder       domain.MessageBuilder
}

// NewCommitteeSyncService creates a new committee sync service
func NewCommitteeSyncService(
	registrantRepository domain.RegistrantRepository,
	registrantService *MeetingRegistrantService,
	messageBuilder domain.MessageBuilder,
) *CommitteeSyncService {
	return &CommitteeSyncService{
		registrantRepository: registrantRepository,
		registrantService:    registrantService,
		messageBuilder:       messageBuilder,
	}
}

// ServiceReady checks if the service is ready for use.
func (s *CommitteeSyncService) ServiceReady() bool {
	return s.registrantRepository != nil &&
		s.registrantService != nil &&
		s.messageBuilder != nil
}

// SyncCommittees synchronizes committee members between old and new committee configurations
func (s *CommitteeSyncService) SyncCommittees(
	ctx context.Context,
	meetingUID string,
	oldCommittees []models.Committee,
	newCommittees []models.Committee,
	isPublicMeeting bool,
) error {
	changes := s.calculateCommitteeChanges(oldCommittees, newCommittees)

	// Early return if no changes
	if len(changes.Added) == 0 && len(changes.Removed) == 0 && len(changes.Changed) == 0 {
		slog.DebugContext(ctx, "no committee changes detected")
		return nil
	}

	slog.InfoContext(ctx, "committee changes detected, processing member sync",
		"meeting_uid", meetingUID,
		"added_committees", len(changes.Added),
		"removed_committees", len(changes.Removed),
		"changed_committees", len(changes.Changed),
		"is_public_meeting", isPublicMeeting)

	// Process changes concurrently but with error collection
	var allErrors []error

	// Handle added committees
	if len(changes.Added) > 0 {
		if err := s.addCommitteeMembers(ctx, meetingUID, changes.Added); err != nil {
			allErrors = append(allErrors, fmt.Errorf("failed to add committee members: %w", err))
		}
	}

	// Handle removed committees
	if len(changes.Removed) > 0 {
		if err := s.removeCommitteeMembers(ctx, meetingUID, changes.Removed, isPublicMeeting); err != nil {
			allErrors = append(allErrors, fmt.Errorf("failed to remove committee members: %w", err))
		}
	}

	// Handle changed committees
	if len(changes.Changed) > 0 {
		if err := s.updateCommitteeMembers(ctx, meetingUID, changes.Changed, isPublicMeeting); err != nil {
			allErrors = append(allErrors, fmt.Errorf("failed to update committee members: %w", err))
		}
	}

	// Return aggregated errors
	if len(allErrors) > 0 {
		return fmt.Errorf("committee sync encountered %d error(s): %v", len(allErrors), allErrors)
	}

	slog.InfoContext(ctx, "committee synchronization completed successfully")
	return nil
}

// calculateCommitteeChanges compares old and new committee lists to determine changes
func (s *CommitteeSyncService) calculateCommitteeChanges(
	oldCommittees []models.Committee,
	newCommittees []models.Committee,
) CommitteeChanges {
	changes := CommitteeChanges{}

	// Create maps for easier comparison
	oldCommitteeMap := make(map[string]models.Committee)
	for _, committee := range oldCommittees {
		oldCommitteeMap[committee.UID] = committee
	}

	newCommitteeMap := make(map[string]models.Committee)
	for _, committee := range newCommittees {
		newCommitteeMap[committee.UID] = committee
	}

	// Check for added committees
	for _, committee := range newCommittees {
		if _, exists := oldCommitteeMap[committee.UID]; !exists {
			changes.Added = append(changes.Added, committee)
		}
	}

	// Check for removed committees
	for _, committee := range oldCommittees {
		if _, exists := newCommitteeMap[committee.UID]; !exists {
			changes.Removed = append(changes.Removed, committee)
		}
	}

	// Check for committees with changed voting statuses
	for _, newCommittee := range newCommittees {
		if oldCommittee, exists := oldCommitteeMap[newCommittee.UID]; exists {
			if !slices.Equal(oldCommittee.AllowedVotingStatuses, newCommittee.AllowedVotingStatuses) {
				changes.Changed = append(changes.Changed, CommitteeChange{
					Old: oldCommittee,
					New: newCommittee,
				})
			}
		}
	}

	return changes
}

// addCommitteeMembers adds committee members as registrants for the specified committees
func (s *CommitteeSyncService) addCommitteeMembers(
	ctx context.Context,
	meetingUID string,
	committees []models.Committee,
) error {
	if len(committees) == 0 {
		return nil
	}

	slog.InfoContext(ctx, "adding committee members as registrants",
		"meeting_uid", meetingUID,
		"committee_count", len(committees))

	// Create functions for worker pool
	var tasks []func() error
	var errors []error

	// Process each committee
	for _, committee := range committees {
		committee := committee // Capture loop variable
		tasks = append(tasks, func() error {
			return s.addCommitteeMembersForCommittee(ctx, meetingUID, committee)
		})
	}

	// Execute with worker pool
	workerPool := concurrent.NewWorkerPool(5) // Limit to 5 concurrent committee requests
	err := workerPool.Run(ctx, tasks...)
	if err != nil {
		errors = append(errors, err)
	}

	// Return aggregated errors
	if len(errors) > 0 {
		return fmt.Errorf("failed to add members from %d committees: %v", len(errors), errors)
	}

	slog.InfoContext(ctx, "successfully added committee members as registrants",
		"meeting_uid", meetingUID,
		"committee_count", len(committees))

	return nil
}

// addCommitteeMembersForCommittee processes a single committee's members
func (s *CommitteeSyncService) addCommitteeMembersForCommittee(
	ctx context.Context,
	meetingUID string,
	committee models.Committee,
) error {
	// Fetch committee members from committee-api
	members, err := s.messageBuilder.GetCommitteeMembers(ctx, committee.UID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to fetch committee members",
			"committee_uid", committee.UID,
			logging.ErrKey, err)
		return fmt.Errorf("failed to fetch committee members for %s: %w", committee.UID, err)
	}

	if len(members) == 0 {
		slog.DebugContext(ctx, "committee has no members", "committee_uid", committee.UID)
		return nil
	}

	// Filter members by allowed voting statuses
	eligibleMembers := s.filterMembersByVotingStatus(members, committee.AllowedVotingStatuses)

	if len(eligibleMembers) == 0 {
		slog.DebugContext(ctx, "no committee members match allowed voting statuses",
			"committee_uid", committee.UID,
			"allowed_voting_statuses", committee.AllowedVotingStatuses)
		return nil
	}

	slog.InfoContext(ctx, "processing committee members for registration",
		"committee_uid", committee.UID,
		"total_members", len(members),
		"eligible_members", len(eligibleMembers))

	// Add eligible members as registrants using worker pool for concurrent processing
	return s.AddCommitteeMembersAsRegistrants(ctx, meetingUID, committee.UID, eligibleMembers)
}

// filterMembersByVotingStatus filters committee members by their voting status
func (s *CommitteeSyncService) filterMembersByVotingStatus(
	members []models.CommitteeMember,
	allowedVotingStatuses []string,
) []models.CommitteeMember {
	if len(allowedVotingStatuses) == 0 {
		// If no specific voting statuses are required, include all members
		return members
	}

	var eligible []models.CommitteeMember
	for _, member := range members {
		if slices.Contains(allowedVotingStatuses, member.Voting.Status) {
			eligible = append(eligible, member)
		}
	}

	return eligible
}

// AddCommitteeMembersAsRegistrants creates registrants for the eligible committee members
func (s *CommitteeSyncService) AddCommitteeMembersAsRegistrants(
	ctx context.Context,
	meetingUID string,
	committeeUID string,
	members []models.CommitteeMember,
) error {
	// Create functions for worker pool
	var tasks []func() error
	var errors []error

	// Process each member
	for _, member := range members {
		member := member // Capture loop variable
		tasks = append(tasks, func() error {
			return s.createRegistrantForCommitteeMember(ctx, meetingUID, committeeUID, member)
		})
	}

	// Execute with worker pool
	workerPool := concurrent.NewWorkerPool(10) // Allow more concurrent registrant operations
	err := workerPool.Run(ctx, tasks...)
	if err != nil {
		// WorkerPool.Run returns first error, but we want to count successes
		// For now, we'll treat any error as a failure and log it
		errors = append(errors, err)
	}

	// Log summary
	successCount := len(members) - len(errors)
	slog.InfoContext(ctx, "committee member registration summary",
		"meeting_uid", meetingUID,
		"committee_uid", committeeUID,
		"total_members", len(members),
		"successful_registrations", successCount,
		"failed_registrations", len(errors))

	// Return aggregated errors if any
	if len(errors) > 0 {
		return fmt.Errorf("failed to create %d registrants from committee %s: %v", len(errors), committeeUID, errors)
	}

	return nil
}

// updateRegistrantToCommitteeType updates an existing registrant to committee type
func (s *CommitteeSyncService) updateRegistrantToCommitteeType(
	ctx context.Context,
	existingRegistrant *models.Registrant,
	revision uint64,
	committeeUID string,
) error {
	// Check if registrant is already a committee type
	if existingRegistrant.Type == models.RegistrantTypeCommittee {
		slog.DebugContext(ctx, "registrant already has committee type",
			"meeting_uid", existingRegistrant.MeetingUID,
			"registrant_uid", existingRegistrant.UID,
			"email", existingRegistrant.Email,
			"committee_uid", committeeUID)
		return nil
	}

	// Create update request with only the fields we want to change
	updateRequest := &models.Registrant{
		UID:          existingRegistrant.UID,
		MeetingUID:   existingRegistrant.MeetingUID,
		Email:        existingRegistrant.Email,
		Type:         models.RegistrantTypeCommittee,
		CommitteeUID: &committeeUID,
	}

	// Use the registrant service to update, which handles NATS messages and FGA sync
	_, err := s.registrantService.UpdateMeetingRegistrant(ctx, updateRequest, revision)
	if err != nil {
		slog.ErrorContext(ctx, "failed to update registrant to committee type",
			"meeting_uid", existingRegistrant.MeetingUID,
			"registrant_uid", existingRegistrant.UID,
			"email", existingRegistrant.Email,
			"committee_uid", committeeUID,
			logging.ErrKey, err)
		return fmt.Errorf("failed to update registrant to committee type: %w", err)
	}

	slog.InfoContext(ctx, "updated registrant to committee type",
		"meeting_uid", existingRegistrant.MeetingUID,
		"registrant_uid", existingRegistrant.UID,
		"email", existingRegistrant.Email,
		"committee_uid", committeeUID)

	return nil
}

// createRegistrantForCommitteeMember creates a registrant record for a committee member
func (s *CommitteeSyncService) createRegistrantForCommitteeMember(
	ctx context.Context,
	meetingUID string,
	committeeUID string,
	member models.CommitteeMember,
) error {
	// Check if registrant already exists by email
	existingRegistrant, revision, err := s.registrantRepository.GetByMeetingAndEmail(ctx, meetingUID, member.Email)
	if err != nil && err != domain.NewNotFoundError("registrant not found", nil) {
		slog.ErrorContext(ctx, "failed to check for existing registrant",
			"meeting_uid", meetingUID,
			"email", member.Email,
			logging.ErrKey, err)
		return fmt.Errorf("failed to check for existing registrant: %w", err)
	}

	if existingRegistrant != nil {
		// Update existing registrant to committee type if needed
		return s.updateRegistrantToCommitteeType(ctx, existingRegistrant, revision, committeeUID)
	}

	// Create registrant for committee member using registrant service (includes email invitations)
	registrant := &models.Registrant{
		MeetingUID:   meetingUID,
		Email:        member.Email,
		FirstName:    member.FirstName,
		LastName:     member.LastName,
		Username:     member.Username,
		Type:         models.RegistrantTypeCommittee,
		CommitteeUID: &committeeUID,
		OrgName:      member.Organization.Name,
		JobTitle:     member.JobTitle,
	}

	createdRegistrant, err := s.registrantService.CreateMeetingRegistrant(ctx, registrant)
	if err != nil {
		if errors.Is(err, domain.NewConflictError("registrant already exists", nil)) {
			// This shouldn't happen since we check ExistsByMeetingAndEmail above,
			// but handle gracefully if it does
			slog.DebugContext(ctx, "registrant already exists (race condition), skipping committee member",
				"meeting_uid", meetingUID,
				"email", member.Email,
				"committee_uid", committeeUID)
			return nil
		}
		slog.ErrorContext(ctx, "failed to create registrant for committee member",
			"meeting_uid", meetingUID,
			"email", member.Email,
			"committee_uid", committeeUID,
			logging.ErrKey, err)
		return fmt.Errorf("failed to create registrant: %w", err)
	}

	slog.DebugContext(ctx, "successfully created registrant for committee member",
		"meeting_uid", meetingUID,
		"registrant_uid", createdRegistrant.UID,
		"email", member.Email,
		"committee_uid", committeeUID)

	return nil
}

// removeCommitteeMembers removes or converts committee members based on meeting visibility
func (s *CommitteeSyncService) removeCommitteeMembers(
	ctx context.Context,
	meetingUID string,
	committees []models.Committee,
	isPublicMeeting bool,
) error {
	if len(committees) == 0 {
		return nil
	}

	action := "remove"
	if isPublicMeeting {
		action = "convert to direct"
	}

	slog.InfoContext(ctx, "removing committee members from meeting",
		"meeting_uid", meetingUID,
		"committee_count", len(committees),
		"action", action,
		"is_public_meeting", isPublicMeeting)

	// Process each committee
	var errors []error
	for _, committee := range committees {
		if err := s.removeCommitteeMembersFromMeeting(ctx, meetingUID, committee.UID, isPublicMeeting); err != nil {
			errors = append(errors, err)
		}
	}

	// Return aggregated errors
	if len(errors) > 0 {
		return fmt.Errorf("failed to remove members from %d committees: %v", len(errors), errors)
	}

	slog.InfoContext(ctx, "successfully processed committee member removal",
		"meeting_uid", meetingUID,
		"committee_count", len(committees))

	return nil
}

// removeCommitteeMembersFromMeeting removes members of a committee from a meeting
func (s *CommitteeSyncService) removeCommitteeMembersFromMeeting(
	ctx context.Context,
	meetingUID string,
	committeeUID string,
	isPublicMeeting bool,
) error {
	// Get all registrants for this meeting
	registrants, err := s.registrantRepository.ListByMeeting(ctx, meetingUID)
	if err != nil {
		return fmt.Errorf("failed to list registrants for meeting %s: %w", meetingUID, err)
	}

	// Filter to committee members for this specific committee
	var committeeRegistrants []*models.Registrant
	for _, registrant := range registrants {
		if registrant.Type == models.RegistrantTypeCommittee &&
			registrant.CommitteeUID != nil &&
			*registrant.CommitteeUID == committeeUID {
			committeeRegistrants = append(committeeRegistrants, registrant)
		}
	}

	if len(committeeRegistrants) == 0 {
		slog.DebugContext(ctx, "no committee registrants found to remove",
			"committee_uid", committeeUID)
		return nil
	}

	slog.InfoContext(ctx, "processing committee registrant removal",
		"committee_uid", committeeUID,
		"registrant_count", len(committeeRegistrants),
		"is_public_meeting", isPublicMeeting)

	// Process each registrant
	var errors []error
	for _, registrant := range committeeRegistrants {
		if err := s.processCommitteeRegistrantRemoval(ctx, registrant, isPublicMeeting); err != nil {
			errors = append(errors, err)
		}
	}

	// Return aggregated errors
	if len(errors) > 0 {
		return fmt.Errorf("failed to remove %d committee registrants from committee %s: %v", len(errors), committeeUID, errors)
	}

	slog.InfoContext(ctx, "successfully processed committee registrant removal",
		"committee_uid", committeeUID,
		"registrant_count", len(committeeRegistrants))

	return nil
}

// processCommitteeRegistrantRemoval handles removal or conversion of a single committee registrant
func (s *CommitteeSyncService) processCommitteeRegistrantRemoval(
	ctx context.Context,
	registrant *models.Registrant,
	isPublicMeeting bool,
) error {
	if isPublicMeeting {
		// Convert to direct registrant (keep them registered)
		return s.convertRegistrantToDirect(ctx, registrant)
	} else {
		// Remove registrant entirely using registrant service (includes email notifications)
		return s.registrantService.DeleteRegistrantWithCleanup(ctx, registrant, 0, true)
	}
}

// convertRegistrantToDirect converts a committee registrant to a direct registrant
func (s *CommitteeSyncService) convertRegistrantToDirect(
	ctx context.Context,
	registrant *models.Registrant,
) error {
	// Get current revision
	_, revision, err := s.registrantRepository.GetWithRevision(ctx, registrant.UID)
	if err != nil {
		return fmt.Errorf("failed to get registrant revision: %w", err)
	}

	// Update registrant to be direct type
	registrant.Type = models.RegistrantTypeDirect
	registrant.CommitteeUID = nil

	err = s.registrantRepository.Update(ctx, registrant, revision)
	if err != nil {
		return fmt.Errorf("failed to convert registrant to direct: %w", err)
	}

	// Send indexing message
	err = s.messageBuilder.SendIndexMeetingRegistrant(ctx, models.ActionUpdated, *registrant)
	if err != nil {
		slog.ErrorContext(ctx, "failed to send indexing message for converted registrant",
			"registrant_uid", registrant.UID,
			logging.ErrKey, err)
	}

	slog.DebugContext(ctx, "converted committee registrant to direct registrant",
		"registrant_uid", registrant.UID,
		"email", registrant.Email)

	return nil
}

// updateCommitteeMembers handles committees with changed voting statuses
func (s *CommitteeSyncService) updateCommitteeMembers(
	ctx context.Context,
	meetingUID string,
	changes []CommitteeChange,
	isPublicMeeting bool,
) error {
	if len(changes) == 0 {
		return nil
	}

	slog.InfoContext(ctx, "updating committee members for voting status changes",
		"meeting_uid", meetingUID,
		"changed_committee_count", len(changes))

	var errors []error
	for _, change := range changes {
		if err := s.updateCommitteeMembersForCommittee(ctx, meetingUID, change, isPublicMeeting); err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to update members for %d committees: %v", len(errors), errors)
	}

	return nil
}

// updateCommitteeMembersForCommittee processes voting status changes for a single committee
func (s *CommitteeSyncService) updateCommitteeMembersForCommittee(
	ctx context.Context,
	meetingUID string,
	change CommitteeChange,
	isPublicMeeting bool,
) error {
	// Fetch current committee members
	members, err := s.messageBuilder.GetCommitteeMembers(ctx, change.New.UID)
	if err != nil {
		return fmt.Errorf("failed to fetch committee members for %s: %w", change.New.UID, err)
	}

	// Filter members by old and new voting statuses
	oldEligibleMembers := s.filterMembersByVotingStatus(members, change.Old.AllowedVotingStatuses)
	newEligibleMembers := s.filterMembersByVotingStatus(members, change.New.AllowedVotingStatuses)

	// Create email sets for easier comparison
	oldEmails := make(map[string]bool)
	for _, member := range oldEligibleMembers {
		oldEmails[member.Email] = true
	}

	newEmails := make(map[string]bool)
	for _, member := range newEligibleMembers {
		newEmails[member.Email] = true
	}

	// Find members to add (in new but not in old)
	var membersToAdd []models.CommitteeMember
	for _, member := range newEligibleMembers {
		if !oldEmails[member.Email] {
			membersToAdd = append(membersToAdd, member)
		}
	}

	// Find members to remove (in old but not in new)
	var membersToRemove []models.CommitteeMember
	for _, member := range oldEligibleMembers {
		if !newEmails[member.Email] {
			membersToRemove = append(membersToRemove, member)
		}
	}

	slog.InfoContext(ctx, "processing voting status change for committee",
		"committee_uid", change.New.UID,
		"old_voting_statuses", change.Old.AllowedVotingStatuses,
		"new_voting_statuses", change.New.AllowedVotingStatuses,
		"members_to_add", len(membersToAdd),
		"members_to_remove", len(membersToRemove))

	// Add new eligible members
	if len(membersToAdd) > 0 {
		err := s.AddCommitteeMembersAsRegistrants(ctx, meetingUID, change.New.UID, membersToAdd)
		if err != nil {
			return fmt.Errorf("failed to add new eligible members: %w", err)
		}
	}

	// Remove no longer eligible members
	if len(membersToRemove) > 0 {
		err := s.removeSpecificCommitteeMembers(ctx, meetingUID, change.New.UID, membersToRemove, isPublicMeeting)
		if err != nil {
			return fmt.Errorf("failed to remove no longer eligible members: %w", err)
		}
	}

	return nil
}

// removeSpecificCommitteeMembers removes specific committee members by email
func (s *CommitteeSyncService) removeSpecificCommitteeMembers(
	ctx context.Context,
	meetingUID string,
	committeeUID string,
	members []models.CommitteeMember,
	isPublicMeeting bool,
) error {
	// Get all registrants for this meeting
	registrants, err := s.registrantRepository.ListByMeeting(ctx, meetingUID)
	if err != nil {
		return fmt.Errorf("failed to list registrants: %w", err)
	}

	// Create email set for members to remove
	emailsToRemove := make(map[string]bool)
	for _, member := range members {
		emailsToRemove[member.Email] = true
	}

	// Find registrants to remove
	var registrantsToRemove []*models.Registrant
	for _, registrant := range registrants {
		if registrant.Type == models.RegistrantTypeCommittee &&
			registrant.CommitteeUID != nil &&
			*registrant.CommitteeUID == committeeUID &&
			emailsToRemove[registrant.Email] {
			registrantsToRemove = append(registrantsToRemove, registrant)
		}
	}

	// Process removal for each registrant
	for _, registrant := range registrantsToRemove {
		err := s.processCommitteeRegistrantRemoval(ctx, registrant, isPublicMeeting)
		if err != nil {
			slog.ErrorContext(ctx, "failed to remove specific committee member",
				"registrant_uid", registrant.UID,
				"email", registrant.Email,
				logging.ErrKey, err)
		}
	}

	return nil
}

// RemoveCommitteeMemberFromMeeting removes a specific committee member from a meeting
// by finding their registrant records and removing/converting them based on meeting visibility
func (s *CommitteeSyncService) RemoveCommitteeMemberFromMeeting(
	ctx context.Context,
	meetingUID string,
	committeeUID string,
	memberEmail string,
	isPublicMeeting bool,
) error {
	// Get all registrants for this meeting
	registrants, err := s.registrantRepository.ListByMeeting(ctx, meetingUID)
	if err != nil {
		return fmt.Errorf("failed to list registrants for meeting %s: %w", meetingUID, err)
	}

	// Find registrants that match this committee member
	var matchingRegistrants []*models.Registrant
	for _, registrant := range registrants {
		if registrant.Type == models.RegistrantTypeCommittee &&
			registrant.CommitteeUID != nil &&
			*registrant.CommitteeUID == committeeUID &&
			registrant.Email == memberEmail {
			matchingRegistrants = append(matchingRegistrants, registrant)
		}
	}

	if len(matchingRegistrants) == 0 {
		slog.DebugContext(ctx, "no matching committee registrants found for removal",
			"meeting_uid", meetingUID,
			"committee_uid", committeeUID,
			"member_email", memberEmail)
		return nil
	}

	slog.InfoContext(ctx, "found matching committee registrants for removal",
		"meeting_uid", meetingUID,
		"committee_uid", committeeUID,
		"member_email", memberEmail,
		"matching_registrants", len(matchingRegistrants),
		"is_public_meeting", isPublicMeeting)

	// Process each matching registrant using existing private methods
	for _, registrant := range matchingRegistrants {
		err := s.processCommitteeRegistrantRemoval(ctx, registrant, isPublicMeeting)
		if err != nil {
			return fmt.Errorf("failed to process registrant removal for %s: %w", registrant.UID, err)
		}
	}

	return nil
}
