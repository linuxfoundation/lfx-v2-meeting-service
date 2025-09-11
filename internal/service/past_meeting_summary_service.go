// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/constants"
)

// PastMeetingSummaryService implements the business logic for past meeting summaries.
type PastMeetingSummaryService struct {
	PastMeetingSummaryRepository domain.PastMeetingSummaryRepository
	PastMeetingRepository        domain.PastMeetingRepository
	RegistrantRepository         domain.RegistrantRepository
	MeetingRepository            domain.MeetingRepository
	EmailService                 domain.EmailService
	MessageBuilder               domain.MessageBuilder
	ServiceConfig                ServiceConfig
}

// NewPastMeetingSummaryService creates a new PastMeetingSummaryService.
func NewPastMeetingSummaryService(
	pastMeetingSummaryRepository domain.PastMeetingSummaryRepository,
	pastMeetingRepository domain.PastMeetingRepository,
	registrantRepository domain.RegistrantRepository,
	meetingRepository domain.MeetingRepository,
	emailService domain.EmailService,
	messageBuilder domain.MessageBuilder,
	serviceConfig ServiceConfig,
) *PastMeetingSummaryService {
	return &PastMeetingSummaryService{
		PastMeetingSummaryRepository: pastMeetingSummaryRepository,
		PastMeetingRepository:        pastMeetingRepository,
		RegistrantRepository:         registrantRepository,
		MeetingRepository:            meetingRepository,
		EmailService:                 emailService,
		MessageBuilder:               messageBuilder,
		ServiceConfig:                serviceConfig,
	}
}

// ServiceReady checks if the service is ready to serve requests.
func (s *PastMeetingSummaryService) ServiceReady() bool {
	return s.PastMeetingSummaryRepository != nil && s.MessageBuilder != nil
}

// ListSummariesByPastMeeting returns all summaries for a given past meeting.
func (s *PastMeetingSummaryService) ListSummariesByPastMeeting(ctx context.Context, pastMeetingUID string) ([]*models.PastMeetingSummary, error) {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "service not initialized", logging.PriorityCritical())
		return nil, domain.ErrServiceUnavailable
	}

	// Validate that the past meeting exists
	_, err := s.PastMeetingRepository.Get(ctx, pastMeetingUID)
	if err != nil {
		if err == domain.ErrPastMeetingNotFound {
			slog.WarnContext(ctx, "past meeting not found", "past_meeting_uid", pastMeetingUID)
			return nil, domain.ErrPastMeetingNotFound
		}
		slog.ErrorContext(ctx, "error checking past meeting existence", logging.ErrKey, err, "past_meeting_uid", pastMeetingUID)
		return nil, domain.ErrInternal
	}

	summaries, err := s.PastMeetingSummaryRepository.ListByPastMeeting(ctx, pastMeetingUID)
	if err != nil {
		slog.ErrorContext(ctx, "error listing summaries", logging.ErrKey, err, "past_meeting_uid", pastMeetingUID)
		return nil, domain.ErrInternal
	}

	return summaries, nil
}

// CreateSummary creates a new summary from a domain model.
func (s *PastMeetingSummaryService) CreateSummary(
	ctx context.Context,
	summary *models.PastMeetingSummary,
) (*models.PastMeetingSummary, error) {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "service not initialized", logging.PriorityCritical())
		return nil, domain.ErrServiceUnavailable
	}

	// Set system-generated fields
	now := time.Now().UTC()
	summary.UID = uuid.New().String()
	summary.CreatedAt = now
	summary.UpdatedAt = now

	// Create in repository
	err := s.PastMeetingSummaryRepository.Create(ctx, summary)
	if err != nil {
		slog.ErrorContext(ctx, "error creating summary", logging.ErrKey, err, "summary_uid", summary.UID)
		return nil, domain.ErrInternal
	}

	// Send indexing message for new summary
	err = s.MessageBuilder.SendIndexPastMeetingSummary(ctx, models.ActionCreated, *summary)
	if err != nil {
		slog.ErrorContext(ctx, "error sending index message for new summary", logging.ErrKey, err, "summary_uid", summary.UID)
		// Don't fail the operation if indexing fails
	}

	// Send email notifications to meeting hosts
	err = s.sendSummaryNotificationEmails(ctx, summary)
	if err != nil {
		slog.ErrorContext(ctx, "error sending summary notification emails", logging.ErrKey, err, "summary_uid", summary.UID)
		// Don't fail the operation if email notifications fail
	}

	slog.InfoContext(ctx, "created new past meeting summary",
		"summary_uid", summary.UID,
		"past_meeting_uid", summary.PastMeetingUID,
		"platform", summary.Platform,
	)

	return summary, nil
}

// GetSummary retrieves a summary by UID with ETag support.
func (s *PastMeetingSummaryService) GetSummary(ctx context.Context, summaryUID string) (*models.PastMeetingSummary, string, error) {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "service not initialized", logging.PriorityCritical())
		return nil, "", domain.ErrServiceUnavailable
	}

	if summaryUID == "" {
		slog.WarnContext(ctx, "summary UID is required")
		return nil, "", domain.ErrValidationFailed
	}

	ctx = logging.AppendCtx(ctx, slog.String("summary_uid", summaryUID))

	// Get the summary with revision
	summary, revision, err := s.PastMeetingSummaryRepository.GetWithRevision(ctx, summaryUID)
	if err != nil {
		if err == domain.ErrPastMeetingSummaryNotFound {
			slog.DebugContext(ctx, "summary not found", "summary_uid", summaryUID)
			return nil, "", err
		}
		slog.ErrorContext(ctx, "error getting summary with revision", logging.ErrKey, err, "summary_uid", summaryUID)
		return nil, "", domain.ErrInternal
	}

	// Convert revision to string for ETag
	revisionStr := strconv.FormatUint(revision, 10)
	ctx = context.WithValue(ctx, constants.ETagContextID, revisionStr)

	slog.DebugContext(ctx, "returning summary", "summary_uid", summary.UID, "revision", revision)

	return summary, revisionStr, nil
}

// UpdateSummary updates an existing summary.
func (s *PastMeetingSummaryService) UpdateSummary(
	ctx context.Context,
	summary *models.PastMeetingSummary,
	revision uint64,
) (*models.PastMeetingSummary, error) {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "service not initialized", logging.PriorityCritical())
		return nil, domain.ErrServiceUnavailable
	}

	// Get current summary
	currentSummary, currentRevision, err := s.PastMeetingSummaryRepository.GetWithRevision(ctx, summary.UID)
	if err != nil {
		if err == domain.ErrPastMeetingSummaryNotFound {
			slog.DebugContext(ctx, "summary not found for update", "summary_uid", summary.UID)
			return nil, err
		}
		slog.ErrorContext(ctx, "error getting summary for update", logging.ErrKey, err, "summary_uid", summary.UID)
		return nil, domain.ErrInternal
	}

	// Verify revision matches
	if revision != currentRevision {
		slog.WarnContext(ctx, "revision mismatch during update",
			"expected_revision", revision,
			"current_revision", currentRevision,
			"summary_uid", summary.UID)
		return nil, domain.ErrRevisionMismatch
	}

	// Merge the update fields with the current summary
	updatedSummary := *currentSummary
	updatedSummary.UpdatedAt = time.Now().UTC()

	// Update only the editable fields from the request
	if summary.SummaryData.EditedContent != "" {
		updatedSummary.SummaryData.EditedContent = summary.SummaryData.EditedContent
	}

	// Update approval status
	updatedSummary.Approved = summary.Approved

	// Update in repository
	err = s.PastMeetingSummaryRepository.Update(ctx, &updatedSummary, revision)
	if err != nil {
		slog.ErrorContext(ctx, "error updating summary", logging.ErrKey, err, "summary_uid", summary.UID)
		return nil, domain.ErrInternal
	}

	// Send indexing message for updated summary
	err = s.MessageBuilder.SendIndexPastMeetingSummary(ctx, models.ActionUpdated, updatedSummary)
	if err != nil {
		slog.ErrorContext(ctx, "error sending index message for updated summary", logging.ErrKey, err, "summary_uid", summary.UID)
		// Don't fail the operation if indexing fails
	}

	slog.InfoContext(ctx, "updated existing past meeting summary",
		"summary_uid", summary.UID,
		"past_meeting_uid", updatedSummary.PastMeetingUID,
	)

	return &updatedSummary, nil
}

// DeleteSummary deletes a summary.
func (s *PastMeetingSummaryService) DeleteSummary(ctx context.Context, summaryUID string) error {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "service not initialized", logging.PriorityCritical())
		return domain.ErrServiceUnavailable
	}

	// Get the summary first to send delete message
	summary, revision, err := s.PastMeetingSummaryRepository.GetWithRevision(ctx, summaryUID)
	if err != nil {
		if err == domain.ErrPastMeetingSummaryNotFound {
			slog.DebugContext(ctx, "summary not found for deletion", "summary_uid", summaryUID)
			return err
		}
		slog.ErrorContext(ctx, "error getting summary for deletion", logging.ErrKey, err, "summary_uid", summaryUID)
		return domain.ErrInternal
	}

	// Delete from repository
	err = s.PastMeetingSummaryRepository.Delete(ctx, summaryUID, revision)
	if err != nil {
		slog.ErrorContext(ctx, "error deleting summary", logging.ErrKey, err, "summary_uid", summaryUID)
		return domain.ErrInternal
	}

	// Send indexing message for deleted summary
	err = s.MessageBuilder.SendIndexPastMeetingSummary(ctx, models.ActionDeleted, *summary)
	if err != nil {
		slog.ErrorContext(ctx, "error sending index message for deleted summary", logging.ErrKey, err, "summary_uid", summaryUID)
		// Don't fail the operation if indexing fails
	}

	slog.InfoContext(ctx, "deleted past meeting summary", "summary_uid", summaryUID)
	return nil
}

// sendSummaryNotificationEmails sends email notifications to meeting host registrants
func (s *PastMeetingSummaryService) sendSummaryNotificationEmails(ctx context.Context, summary *models.PastMeetingSummary) error {
	// Get the past meeting to retrieve meeting details
	pastMeeting, err := s.PastMeetingRepository.Get(ctx, summary.PastMeetingUID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get past meeting for summary notification", logging.ErrKey, err, "past_meeting_uid", summary.PastMeetingUID)
		return err
	}

	// Get the original meeting to get project context
	meetingBase, err := s.MeetingRepository.GetBase(ctx, summary.MeetingUID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get meeting base for summary notification", logging.ErrKey, err, "meeting_uid", summary.MeetingUID)
		return err
	}

	// Get all registrants for the meeting
	registrants, err := s.RegistrantRepository.ListByMeeting(ctx, summary.MeetingUID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get registrants for summary notification", logging.ErrKey, err, "meeting_uid", summary.MeetingUID)
		return err
	}

	// Filter to only host registrants
	hostRegistrants := make([]*models.Registrant, 0)
	for _, registrant := range registrants {
		if registrant.Host {
			hostRegistrants = append(hostRegistrants, registrant)
		}
	}

	if len(hostRegistrants) == 0 {
		slog.InfoContext(ctx, "no host registrants found for summary notification", "meeting_uid", summary.MeetingUID)
		return nil
	}

	// Send email to each host
	successCount := 0
	for _, registrant := range hostRegistrants {
		notification := domain.EmailSummaryNotification{
			RecipientEmail: registrant.Email,
			RecipientName:  registrant.FirstName + " " + registrant.LastName,
			MeetingTitle:   pastMeeting.Title,
			MeetingDate:    pastMeeting.ScheduledStartTime,
			ProjectName:    meetingBase.ProjectUID, // Using ProjectUID as project context
			SummaryContent: summary.SummaryData.Content,
			SummaryDocURL:  summary.SummaryData.DocURL,
			SummaryTitle:   summary.SummaryData.Title,
		}

		err := s.EmailService.SendSummaryNotification(ctx, notification)
		if err != nil {
			slog.ErrorContext(ctx, "failed to send summary notification email",
				logging.ErrKey, err,
				"registrant_uid", registrant.UID,
				"email", registrant.Email,
				"meeting_uid", summary.MeetingUID,
			)
			// Continue with other recipients even if one fails
		} else {
			successCount++
			slog.DebugContext(ctx, "summary notification email sent successfully",
				"registrant_uid", registrant.UID,
				"email", registrant.Email,
			)
		}
	}

	slog.InfoContext(ctx, "summary notification emails sent",
		"total_hosts", len(hostRegistrants),
		"successful_sends", successCount,
		"summary_uid", summary.UID,
	)

	// Return error only if no emails were sent successfully
	if successCount == 0 && len(hostRegistrants) > 0 {
		return domain.ErrInternal
	}

	return nil
}
