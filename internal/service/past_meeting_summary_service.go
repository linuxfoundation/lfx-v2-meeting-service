// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/concurrent"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/constants"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/redaction"
)

// PastMeetingSummaryService implements the business logic for past meeting summaries.
type PastMeetingSummaryService struct {
	pastMeetingSummaryRepository     domain.PastMeetingSummaryRepository
	pastMeetingRepository            domain.PastMeetingRepository
	pastMeetingParticipantRepository domain.PastMeetingParticipantRepository
	registrantRepository             domain.RegistrantRepository
	meetingRepository                domain.MeetingRepository
	emailService                     domain.EmailService
	messageBuilder                   domain.MessageBuilder
	config                           ServiceConfig
}

// NewPastMeetingSummaryService creates a new PastMeetingSummaryService.
func NewPastMeetingSummaryService(
	pastMeetingSummaryRepository domain.PastMeetingSummaryRepository,
	pastMeetingRepository domain.PastMeetingRepository,
	pastMeetingParticipantRepository domain.PastMeetingParticipantRepository,
	registrantRepository domain.RegistrantRepository,
	meetingRepository domain.MeetingRepository,
	emailService domain.EmailService,
	messageBuilder domain.MessageBuilder,
	serviceConfig ServiceConfig,
) *PastMeetingSummaryService {
	return &PastMeetingSummaryService{
		pastMeetingSummaryRepository:     pastMeetingSummaryRepository,
		pastMeetingRepository:            pastMeetingRepository,
		pastMeetingParticipantRepository: pastMeetingParticipantRepository,
		registrantRepository:             registrantRepository,
		meetingRepository:                meetingRepository,
		emailService:                     emailService,
		messageBuilder:                   messageBuilder,
		config:                           serviceConfig,
	}
}

// ServiceReady checks if the service is ready to serve requests.
func (s *PastMeetingSummaryService) ServiceReady() bool {
	return s.pastMeetingSummaryRepository != nil &&
		s.pastMeetingRepository != nil &&
		s.pastMeetingParticipantRepository != nil &&
		s.registrantRepository != nil &&
		s.meetingRepository != nil &&
		s.emailService != nil &&
		s.messageBuilder != nil
}

// ListSummariesByPastMeeting returns all summaries for a given past meeting.
func (s *PastMeetingSummaryService) ListSummariesByPastMeeting(ctx context.Context, pastMeetingUID string) ([]*models.PastMeetingSummary, error) {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "service not initialized", logging.PriorityCritical())
		return nil, domain.NewUnavailableError("service not initialized")
	}

	// Validate that the past meeting exists
	_, err := s.pastMeetingRepository.Get(ctx, pastMeetingUID)
	if err != nil {
		if domain.GetErrorType(err) == domain.ErrorTypeNotFound {
			slog.WarnContext(ctx, "past meeting not found", "past_meeting_uid", pastMeetingUID)
			return nil, domain.NewNotFoundError("past meeting not found")
		}
		slog.ErrorContext(ctx, "error checking past meeting existence", logging.ErrKey, err, "past_meeting_uid", pastMeetingUID)
		return nil, domain.NewInternalError("error checking past meeting existence", err)
	}

	summaries, err := s.pastMeetingSummaryRepository.ListByPastMeeting(ctx, pastMeetingUID)
	if err != nil {
		slog.ErrorContext(ctx, "error listing summaries", logging.ErrKey, err, "past_meeting_uid", pastMeetingUID)
		return nil, domain.NewInternalError("error listing summaries", err)
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
		return nil, domain.NewUnavailableError("service not initialized")
	}

	// Set system-generated fields
	now := time.Now().UTC()
	summary.UID = uuid.New().String()
	summary.CreatedAt = now
	summary.UpdatedAt = now

	// Send email notifications first to determine EmailSent status
	err := s.sendSummaryNotificationEmails(ctx, summary)
	if err != nil {
		slog.ErrorContext(ctx, "error sending summary notification emails", logging.ErrKey, err, "summary_uid", summary.UID)
		summary.EmailSent = false
	} else {
		summary.EmailSent = true
	}

	// Create in repository with correct EmailSent status
	err = s.pastMeetingSummaryRepository.Create(ctx, summary)
	if err != nil {
		slog.ErrorContext(ctx, "error creating summary", logging.ErrKey, err, "summary_uid", summary.UID)
		return nil, domain.NewInternalError("error creating summary", err)
	}

	// Use WorkerPool for concurrent NATS message sending
	pool := concurrent.NewWorkerPool(2) // 2 messages to send
	messages := []func() error{
		func() error {
			return s.messageBuilder.SendIndexPastMeetingSummary(ctx, models.ActionCreated, *summary)
		},
		func() error {
			// Get past meeting to retrieve artifact visibility
			pastMeeting, err := s.pastMeetingRepository.Get(ctx, summary.PastMeetingUID)
			if err != nil {
				return err
			}

			// Get participants for the past meeting
			participantPointers, err := s.pastMeetingParticipantRepository.ListByPastMeeting(ctx, summary.PastMeetingUID)
			if err != nil {
				return err
			}

			// Convert to simplified access participants
			participants := make([]models.AccessParticipant, len(participantPointers))
			for i, p := range participantPointers {
				participants[i] = models.AccessParticipant{
					Username: p.Username,
					Host:     p.Host,
				}
			}

			return s.messageBuilder.SendUpdateAccessPastMeetingSummary(ctx, models.PastMeetingSummaryAccessMessage{
				UID:                summary.UID,
				PastMeetingUID:     summary.PastMeetingUID,
				ArtifactVisibility: pastMeeting.ArtifactVisibility,
				Participants:       participants,
			})
		},
	}

	if err := pool.Run(ctx, messages...); err != nil {
		slog.ErrorContext(ctx, "failed to send NATS messages", logging.ErrKey, err)
		// Don't fail the operation if messaging fails
	}

	slog.InfoContext(ctx, "created new past meeting summary",
		"summary_uid", summary.UID,
		"past_meeting_uid", summary.PastMeetingUID,
		"platform", summary.Platform,
		"email_sent", summary.EmailSent,
	)

	return summary, nil
}

// GetSummary retrieves a summary by UID with ETag support.
func (s *PastMeetingSummaryService) GetSummary(ctx context.Context, summaryUID string) (*models.PastMeetingSummary, string, error) {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "service not initialized", logging.PriorityCritical())
		return nil, "", domain.NewUnavailableError("service not initialized")
	}

	if summaryUID == "" {
		slog.WarnContext(ctx, "summary UID is required")
		return nil, "", domain.NewValidationError("summary UID is required")
	}

	ctx = logging.AppendCtx(ctx, slog.String("summary_uid", summaryUID))

	// Get the summary with revision
	summary, revision, err := s.pastMeetingSummaryRepository.GetWithRevision(ctx, summaryUID)
	if err != nil {
		if domain.GetErrorType(err) == domain.ErrorTypeNotFound {
			slog.DebugContext(ctx, "summary not found", "summary_uid", summaryUID)
			return nil, "", err
		}
		slog.ErrorContext(ctx, "error getting summary with revision", logging.ErrKey, err, "summary_uid", summaryUID)
		return nil, "", domain.NewInternalError("error getting summary with revision", err)
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
		return nil, domain.NewUnavailableError("service not initialized")
	}

	// Get current summary
	currentSummary, currentRevision, err := s.pastMeetingSummaryRepository.GetWithRevision(ctx, summary.UID)
	if err != nil {
		if domain.GetErrorType(err) == domain.ErrorTypeNotFound {
			slog.DebugContext(ctx, "summary not found for update", "summary_uid", summary.UID)
			return nil, err
		}
		slog.ErrorContext(ctx, "error getting summary for update", logging.ErrKey, err, "summary_uid", summary.UID)
		return nil, domain.NewInternalError("error getting summary for update", err)
	}

	// Verify revision matches
	if revision != currentRevision {
		slog.WarnContext(ctx, "revision mismatch during update",
			"expected_revision", revision,
			"current_revision", currentRevision,
			"summary_uid", summary.UID)
		return nil, domain.NewValidationError("revision mismatch")
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
	err = s.pastMeetingSummaryRepository.Update(ctx, &updatedSummary, revision)
	if err != nil {
		slog.ErrorContext(ctx, "error updating summary", logging.ErrKey, err, "summary_uid", summary.UID)
		return nil, domain.NewInternalError("error updating summary", err)
	}

	// Use WorkerPool for concurrent NATS message sending
	pool := concurrent.NewWorkerPool(2) // 2 messages to send
	messages := []func() error{
		func() error {
			return s.messageBuilder.SendIndexPastMeetingSummary(ctx, models.ActionUpdated, updatedSummary)
		},
		func() error {
			// Get past meeting to retrieve artifact visibility
			pastMeeting, err := s.pastMeetingRepository.Get(ctx, updatedSummary.PastMeetingUID)
			if err != nil {
				return err
			}

			// Get participants for the past meeting
			participantPointers, err := s.pastMeetingParticipantRepository.ListByPastMeeting(ctx, updatedSummary.PastMeetingUID)
			if err != nil {
				return err
			}

			// Convert to simplified access participants
			participants := make([]models.AccessParticipant, len(participantPointers))
			for i, p := range participantPointers {
				participants[i] = models.AccessParticipant{
					Username: p.Username,
					Host:     p.Host,
				}
			}

			return s.messageBuilder.SendUpdateAccessPastMeetingSummary(ctx, models.PastMeetingSummaryAccessMessage{
				UID:                updatedSummary.UID,
				PastMeetingUID:     updatedSummary.PastMeetingUID,
				ArtifactVisibility: pastMeeting.ArtifactVisibility,
				Participants:       participants,
			})
		},
	}

	if err := pool.Run(ctx, messages...); err != nil {
		slog.ErrorContext(ctx, "failed to send NATS messages", logging.ErrKey, err)
		// Don't fail the operation if messaging fails
	}

	slog.InfoContext(ctx, "updated existing past meeting summary",
		"summary_uid", summary.UID,
		"past_meeting_uid", updatedSummary.PastMeetingUID,
	)

	return &updatedSummary, nil
}

// sendSummaryNotificationEmails sends email notifications to meeting host registrants
func (s *PastMeetingSummaryService) sendSummaryNotificationEmails(ctx context.Context, summary *models.PastMeetingSummary) error {
	// Get the past meeting to retrieve meeting details
	pastMeeting, err := s.pastMeetingRepository.Get(ctx, summary.PastMeetingUID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get past meeting for summary notification", logging.ErrKey, err, "past_meeting_uid", summary.PastMeetingUID)
		return err
	}

	// Get the original meeting to get project context
	meetingBase, err := s.meetingRepository.GetBase(ctx, summary.MeetingUID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get meeting base for summary notification", logging.ErrKey, err, "meeting_uid", summary.MeetingUID)
		return err
	}

	// Get all registrants for the meeting
	registrants, err := s.registrantRepository.ListByMeeting(ctx, summary.MeetingUID)
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

	// Get project details for email
	projectName, _ := s.messageBuilder.GetProjectName(ctx, meetingBase.ProjectUID)
	projectLogo, _ := s.messageBuilder.GetProjectLogo(ctx, meetingBase.ProjectUID)
	projectSlug, _ := s.messageBuilder.GetProjectSlug(ctx, meetingBase.ProjectUID)

	// Send email to each host
	successCount := 0
	for _, registrant := range hostRegistrants {
		notification := domain.EmailSummaryNotification{
			RecipientEmail:     registrant.Email,
			RecipientName:      strings.TrimSpace(strings.Join([]string{registrant.FirstName, registrant.LastName}, " ")),
			MeetingTitle:       pastMeeting.Title,
			MeetingDate:        pastMeeting.ScheduledStartTime,
			ProjectName:        projectName,
			ProjectLogo:        projectLogo,
			SummaryContent:     summary.SummaryData.Content,
			SummaryTitle:       summary.SummaryData.Title,
			MeetingDetailsLink: constants.GenerateLFXMeetingDetailsURL(projectSlug, pastMeeting.UID, s.config.LFXEnvironment),
		}

		err := s.emailService.SendSummaryNotification(ctx, notification)
		if err != nil {
			slog.ErrorContext(ctx, "failed to send summary notification email",
				logging.ErrKey, err,
				"registrant_uid", registrant.UID,
				"email", redaction.RedactEmail(registrant.Email),
				"meeting_uid", summary.MeetingUID,
			)
			// Continue with other recipients even if one fails
		} else {
			successCount++
			slog.DebugContext(ctx, "summary notification email sent successfully",
				"registrant_uid", registrant.UID,
				"email", redaction.RedactEmail(registrant.Email),
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
		return domain.NewInternalError("error sending summary notification emails")
	}

	return nil
}
