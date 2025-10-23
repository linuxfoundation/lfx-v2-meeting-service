// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/concurrent"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/constants"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/redaction"
)

// MeetingRegistrantService implements the meetingsvc.Service interface and domain.MessageHandler
type MeetingRegistrantService struct {
	meetingRepository    domain.MeetingRepository
	registrantRepository domain.RegistrantRepository
	emailService         domain.EmailService
	messageBuilder       domain.MessageBuilder
	occurrenceService    *OccurrenceService
	config               ServiceConfig
}

// NewMeetingRegistrantService creates a new MeetingRegistrantService.
func NewMeetingRegistrantService(
	meetingRepository domain.MeetingRepository,
	registrantRepository domain.RegistrantRepository,
	emailService domain.EmailService,
	messageBuilder domain.MessageBuilder,
	occurrenceService *OccurrenceService,
	config ServiceConfig,
) *MeetingRegistrantService {
	return &MeetingRegistrantService{
		config:               config,
		meetingRepository:    meetingRepository,
		registrantRepository: registrantRepository,
		emailService:         emailService,
		messageBuilder:       messageBuilder,
		occurrenceService:    occurrenceService,
	}
}

// ServiceReady checks if the service is ready for use.
func (s *MeetingRegistrantService) ServiceReady() bool {
	return s.meetingRepository != nil &&
		s.registrantRepository != nil &&
		s.messageBuilder != nil &&
		s.emailService != nil &&
		s.occurrenceService != nil
}

// ListMeetingRegistrants gets all registrants for a meeting
func (s *MeetingRegistrantService) ListMeetingRegistrants(ctx context.Context, uid string) ([]*models.Registrant, error) {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "service not initialized", logging.PriorityCritical())
		return nil, domain.NewUnavailableError("meeting registrant service is not ready")
	}

	ctx = logging.AppendCtx(ctx, slog.String("meeting_uid", uid))

	// Check if the meeting exists
	_, err := s.meetingRepository.GetBase(ctx, uid)
	if err != nil {
		return nil, err
	}

	slog.DebugContext(ctx, "meeting found", "meeting_uid", uid)

	// Get all registrants for the meeting
	registrants, err := s.registrantRepository.ListByMeeting(ctx, uid)
	if err != nil {
		return nil, err
	}

	slog.DebugContext(ctx, "returning meeting registrants", "count", len(registrants))

	return registrants, nil
}

// ListRegistrantsByEmail gets all registrants with a specific email address
func (s *MeetingRegistrantService) ListRegistrantsByEmail(ctx context.Context, email string) ([]*models.Registrant, error) {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "service not initialized", logging.PriorityCritical())
		return nil, domain.NewUnavailableError("meeting registrant service is not ready")
	}

	ctx = logging.AppendCtx(ctx, slog.String("email", redaction.RedactEmail(email)))

	// Get all registrants with this email
	registrants, err := s.registrantRepository.ListByEmail(ctx, email)
	if err != nil {
		return nil, err
	}

	slog.DebugContext(ctx, "returning registrants by email", "count", len(registrants))

	return registrants, nil
}

func (s *MeetingRegistrantService) validateCreateMeetingRegistrantRequest(ctx context.Context, reqRegistrant *models.Registrant) error {
	// Check if the meeting exists
	meeting, err := s.meetingRepository.GetBase(ctx, reqRegistrant.MeetingUID)
	if err != nil {
		return err
	}

	// Check that there isn't already a registrant with the same email address for this meeting.
	registrants, err := s.registrantRepository.ListByEmail(ctx, reqRegistrant.Email)
	if err != nil {
		return err
	}
	for _, registrant := range registrants {
		if registrant.Email == reqRegistrant.Email && registrant.MeetingUID == reqRegistrant.MeetingUID {
			return domain.NewConflictError("registrant with same email already exists for this meeting")
		}
	}

	// Validate occurrence ID if provided
	if reqRegistrant.OccurrenceID != "" {
		if err := s.occurrenceService.ValidateFutureOccurrenceID(meeting, reqRegistrant.OccurrenceID, 100); err != nil {
			return err
		}
	}

	return nil
}

// createRegistrantContext creates a background context with registrant and meeting UID attributes for async operations
func createRegistrantContext(ctx context.Context, registrantUID, meetingUID string) context.Context {
	bgCtx := logging.AppendCtx(ctx, slog.String("registrant_uid", registrantUID))
	return logging.AppendCtx(bgCtx, slog.String("meeting_uid", meetingUID))
}

// CreateMeetingRegistrant creates a new registrant for a meeting
func (s *MeetingRegistrantService) CreateMeetingRegistrant(ctx context.Context, reqRegistrant *models.Registrant) (*models.Registrant, error) {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "service not initialized", logging.PriorityCritical())
		return nil, domain.NewUnavailableError("meeting registrant service is not ready")
	}

	if reqRegistrant == nil {
		return nil, domain.NewValidationError("registrant payload is required")
	}

	ctx = logging.AppendCtx(ctx, slog.String("meeting_uid", reqRegistrant.MeetingUID))

	// Validate the payload
	err := s.validateCreateMeetingRegistrantRequest(ctx, reqRegistrant)
	if err != nil {
		return nil, err
	}

	// Generate UID for the registrant
	reqRegistrant.UID = uuid.New().String()

	ctx = logging.AppendCtx(ctx, slog.String("registrant_uid", reqRegistrant.UID))

	// Create the registrant
	err = s.registrantRepository.Create(ctx, reqRegistrant)
	if err != nil {
		return nil, err
	}

	slog.DebugContext(ctx, "created registrant", "registrant", reqRegistrant)

	// Build list of NATS messages and email tasks
	tasks := []func() error{
		// Send indexing message for the new registrant
		func() error {
			msgCtx := createRegistrantContext(ctx, reqRegistrant.UID, reqRegistrant.MeetingUID)
			err := s.messageBuilder.SendIndexMeetingRegistrant(msgCtx, models.ActionCreated, *reqRegistrant)
			if err != nil {
				slog.ErrorContext(msgCtx, "error sending indexing message for new registrant", logging.ErrKey, err)
			}
			return nil // Don't fail on messaging errors
		},
		// Send invitation email to the registrant
		func() error {
			emailCtx := createRegistrantContext(ctx, reqRegistrant.UID, reqRegistrant.MeetingUID)
			err := s.SendRegistrantInvitationEmail(emailCtx, reqRegistrant)
			if err != nil {
				slog.ErrorContext(emailCtx, "failed to send invitation email", logging.ErrKey, err)
			}
			return nil // Don't fail on email errors
		},
	}

	// Send a message about the new registrant to the fga-sync service if username exists
	if reqRegistrant.Username != "" {
		tasks = append(tasks, func() error {
			msgCtx := createRegistrantContext(ctx, reqRegistrant.UID, reqRegistrant.MeetingUID)
			err := s.messageBuilder.SendPutMeetingRegistrantAccess(msgCtx, models.MeetingRegistrantAccessMessage{
				UID:        reqRegistrant.UID,
				Username:   reqRegistrant.Username,
				MeetingUID: reqRegistrant.MeetingUID,
				Host:       reqRegistrant.Host,
			})
			if err != nil {
				slog.ErrorContext(msgCtx, "error sending message about new registrant", logging.ErrKey, err)
			}
			return nil // Don't fail on messaging errors
		})
	} else {
		// This can happen when the registrant is not an LF user but rather a guest user.
		slog.DebugContext(ctx, "no username for registrant, skipping access message")
	}

	// Use WorkerPool to execute tasks concurrently
	pool := concurrent.NewWorkerPool(len(tasks))
	if err := pool.Run(ctx, tasks...); err != nil {
		slog.ErrorContext(ctx, "error executing post-creation tasks", logging.ErrKey, err)
	}

	return reqRegistrant, nil
}

// GetMeetingRegistrant gets a specific registrant by UID
func (s *MeetingRegistrantService) GetMeetingRegistrant(ctx context.Context, meetingUID, registrantUID string) (*models.Registrant, string, error) {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "service not initialized", logging.PriorityCritical())
		return nil, "", domain.NewUnavailableError("meeting registrant service is not ready")
	}

	ctx = logging.AppendCtx(ctx, slog.String("meeting_uid", meetingUID))
	ctx = logging.AppendCtx(ctx, slog.String("registrant_uid", registrantUID))

	// Check that meeting exists
	exists, err := s.meetingRepository.Exists(ctx, meetingUID)
	if err != nil {
		return nil, "", err
	}
	if !exists {
		return nil, "", domain.NewNotFoundError("meeting not found")
	}

	// Get registrant with revision from store
	registrant, revision, err := s.registrantRepository.GetWithRevision(ctx, registrantUID)
	if err != nil {
		return nil, "", err
	}

	// Ensure the registrant belongs to the requested meeting
	if registrant.MeetingUID != meetingUID {
		return nil, "", domain.NewNotFoundError("registrant does not belong to the specified meeting")
	}

	// Store the revision in context for the custom encoder to use
	revisionStr := strconv.FormatUint(revision, 10)
	ctx = context.WithValue(ctx, constants.ETagContextID, revisionStr)

	slog.DebugContext(ctx, "returning registrant", "registrant", registrant, "revision", revision)

	return registrant, revisionStr, nil
}

func (s *MeetingRegistrantService) validateUpdateMeetingRegistrantRequest(ctx context.Context, reqRegistrant *models.Registrant, existingRegistrant *models.Registrant) error {
	// Check that the meeting exists and get it for occurrence validation
	meeting, err := s.meetingRepository.GetBase(ctx, existingRegistrant.MeetingUID)
	if err != nil {
		return err
	}

	if existingRegistrant.Email != reqRegistrant.Email {
		// If changing the email address, check that there isn't already a registrant for this meeting with the new email address.
		registrants, err := s.registrantRepository.ListByEmail(ctx, reqRegistrant.Email)
		if err != nil {
			return err
		}
		for _, registrant := range registrants {
			if registrant.Email == reqRegistrant.Email && registrant.MeetingUID == existingRegistrant.MeetingUID {
				return domain.NewConflictError("registrant with same email already exists for this meeting")
			}
		}
	}

	// Validate occurrence ID if provided and different from existing
	if reqRegistrant.OccurrenceID != "" && reqRegistrant.OccurrenceID != existingRegistrant.OccurrenceID {
		if err := s.occurrenceService.ValidateFutureOccurrenceID(meeting, reqRegistrant.OccurrenceID, 100); err != nil {
			return err
		}
	}

	return nil
}

// UpdateMeetingRegistrant updates an existing registrant
func (s *MeetingRegistrantService) UpdateMeetingRegistrant(ctx context.Context, reqRegistrant *models.Registrant, revision uint64) (*models.Registrant, error) {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "service not initialized", logging.PriorityCritical())
		return nil, domain.NewUnavailableError("meeting registrant service is not ready")
	}

	ctx = logging.AppendCtx(ctx, slog.String("meeting_uid", reqRegistrant.MeetingUID))
	ctx = logging.AppendCtx(ctx, slog.String("registrant_uid", reqRegistrant.UID))

	var err error
	if s.config.SkipEtagValidation {
		// If skipping the Etag validation, we need to get the key revision from the store with a Get request.
		_, revision, err = s.registrantRepository.GetWithRevision(ctx, reqRegistrant.UID)
		if err != nil {
			return nil, err
		}
	}

	ctx = logging.AppendCtx(ctx, slog.String("etag", strconv.FormatUint(revision, 10)))

	// Check if the registrant exists and get existing data for the update
	existingRegistrant, err := s.registrantRepository.Get(ctx, reqRegistrant.UID)
	if err != nil {
		return nil, err
	}

	// Validate the request
	err = s.validateUpdateMeetingRegistrantRequest(ctx, reqRegistrant, existingRegistrant)
	if err != nil {
		return nil, err
	}

	reqRegistrant = models.MergeUpdateRegistrantRequest(reqRegistrant, existingRegistrant)

	// Update the registrant
	err = s.registrantRepository.Update(ctx, reqRegistrant, revision)
	if err != nil {
		return nil, err
	}

	slog.DebugContext(ctx, "updated registrant", "registrant", reqRegistrant)

	// Build list of NATS messages tasks
	tasks := []func() error{
		// Send indexing message for the updated registrant
		func() error {
			msgCtx := createRegistrantContext(ctx, reqRegistrant.UID, reqRegistrant.MeetingUID)
			err := s.messageBuilder.SendIndexMeetingRegistrant(msgCtx, models.ActionUpdated, *reqRegistrant)
			if err != nil {
				slog.ErrorContext(msgCtx, "error sending indexing message for updated registrant", logging.ErrKey, err)
			}
			return nil // Don't fail on messaging errors
		},
	}

	// Send a message about the updated registrant to the fga-sync service if username exists
	if reqRegistrant.Username != "" {
		tasks = append(tasks, func() error {
			msgCtx := createRegistrantContext(ctx, reqRegistrant.UID, reqRegistrant.MeetingUID)
			err := s.messageBuilder.SendPutMeetingRegistrantAccess(msgCtx, models.MeetingRegistrantAccessMessage{
				UID:        reqRegistrant.UID,
				Username:   reqRegistrant.Username,
				MeetingUID: reqRegistrant.MeetingUID,
				Host:       reqRegistrant.Host,
			})
			if err != nil {
				slog.ErrorContext(msgCtx, "error sending message about updated registrant", logging.ErrKey, err)
			}
			return nil // Don't fail on messaging errors
		})
	} else {
		// This can happen when the registrant is not an LF user but rather a guest user.
		slog.DebugContext(ctx, "no username for registrant, skipping access message")
	}

	// Use WorkerPool to execute tasks concurrently
	pool := concurrent.NewWorkerPool(len(tasks))
	if err := pool.Run(ctx, tasks...); err != nil {
		slog.ErrorContext(ctx, "error executing post-update tasks", logging.ErrKey, err)
	}

	return reqRegistrant, nil
}

// DeleteRegistrantWithCleanup is an internal helper that deletes a registrant and sends cleanup messages.
// It can optionally skip revision checking when skipRevisionCheck is true (useful for bulk cleanup operations).
func (s *MeetingRegistrantService) DeleteRegistrantWithCleanup(
	ctx context.Context,
	registrant *models.Registrant,
	meeting *models.MeetingBase,
	revision uint64,
	skipRevisionCheck bool,
) error {
	ctx = logging.AppendCtx(ctx, slog.String("registrant_uid", registrant.UID))

	// Delete the registrant from the database
	var err error
	if skipRevisionCheck {
		// Use revision 0 to skip revision checking for bulk cleanup operations
		err = s.registrantRepository.Delete(ctx, registrant.UID, 0)
	} else {
		err = s.registrantRepository.Delete(ctx, registrant.UID, revision)
	}

	if err != nil {
		// For bulk cleanup operations, we might encounter not found errors which are acceptable
		if skipRevisionCheck && domain.GetErrorType(err) == domain.ErrorTypeNotFound {
			slog.DebugContext(ctx, "registrant already deleted, skipping")
			return nil
		}
		return err
	}

	slog.DebugContext(ctx, "deleted registrant")

	// Send cleanup messages and cancellation email asynchronously using WorkerPool
	var functions []func() error

	// Send indexing delete message for the registrant
	functions = append(functions, func() error {
		msgCtx := createRegistrantContext(ctx, registrant.UID, registrant.MeetingUID)

		err := s.messageBuilder.SendDeleteIndexMeetingRegistrant(msgCtx, registrant.UID)
		if err != nil {
			slog.ErrorContext(msgCtx, "error sending delete indexing message for registrant", logging.ErrKey, err, logging.PriorityCritical())
		}
		return nil // Don't propagate messaging errors as they shouldn't fail the operation
	})

	// Send access removal message if the registrant has a username
	if registrant.Username != "" {
		functions = append(functions, func() error {
			msgCtx := createRegistrantContext(ctx, registrant.UID, registrant.MeetingUID)

			err := s.messageBuilder.SendRemoveMeetingRegistrantAccess(msgCtx, models.MeetingRegistrantAccessMessage{
				UID:        registrant.UID,
				Username:   registrant.Username,
				MeetingUID: registrant.MeetingUID,
				Host:       registrant.Host,
			})
			if err != nil {
				slog.ErrorContext(msgCtx, "error sending message about deleted registrant", logging.ErrKey, err, logging.PriorityCritical())
			}
			return nil // Don't propagate messaging errors as they shouldn't fail the operation
		})
	} else {
		// This can happen when the registrant is not an LF user but rather a guest user.
		slog.DebugContext(ctx, "no username for registrant, skipping access message")
	}

	// Send cancellation email to the registrant
	functions = append(functions, func() error {
		emailCtx := createRegistrantContext(ctx, registrant.UID, registrant.MeetingUID)

		err := s.SendRegistrantCancellationEmail(emailCtx, registrant, meeting)
		if err != nil {
			slog.ErrorContext(emailCtx, "failed to send cancellation email", logging.ErrKey, err)
		}
		return nil // Don't propagate email errors as they shouldn't fail the operation
	})

	// Execute all functions concurrently using WorkerPool
	pool := concurrent.NewWorkerPool(3) // Use 3 workers for the async operations
	err = pool.Run(ctx, functions...)
	if err != nil {
		// Log the error but don't fail the operation since messaging/email errors are non-critical
		slog.WarnContext(ctx, "some async operations failed during registrant cleanup", logging.ErrKey, err)
	}

	return nil
}

// DeleteMeetingRegistrant deletes a registrant from a meeting
func (s *MeetingRegistrantService) DeleteMeetingRegistrant(ctx context.Context, meetingUID, registrantUID string, revision uint64) error {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "service not initialized", logging.PriorityCritical())
		return domain.NewUnavailableError("meeting registrant service is not ready")
	}

	ctx = logging.AppendCtx(ctx, slog.String("meeting_uid", meetingUID))
	ctx = logging.AppendCtx(ctx, slog.String("registrant_uid", registrantUID))

	var err error
	if s.config.SkipEtagValidation {
		// If skipping the Etag validation, we need to get the key revision from the store with a Get request.
		_, revision, err = s.registrantRepository.GetWithRevision(ctx, registrantUID)
		if err != nil {
			return err
		}
	}

	ctx = logging.AppendCtx(ctx, slog.String("etag", strconv.FormatUint(revision, 10)))

	// Get the meeting for cleanup process and to check for existence
	meeting, err := s.meetingRepository.GetBase(ctx, meetingUID)
	if err != nil {
		return err
	}

	// Get the registrant for cleanup process and to check for existence
	registrant, err := s.registrantRepository.Get(ctx, registrantUID)
	if err != nil {
		return err
	}

	// Use the helper to delete the registrant with cleanup
	return s.DeleteRegistrantWithCleanup(ctx, registrant, meeting, revision, false)
}

// SendRegistrantEmailChangeNotifications sends notification emails when a registrant's email changes
// It sends a cancellation email to the old address and an invitation email to the new address
// Returns an error if either email fails to send, allowing the caller to decide how to handle failures
func (s *MeetingRegistrantService) SendRegistrantEmailChangeNotifications(
	ctx context.Context,
	meeting *models.MeetingBase,
	oldRegistrant *models.Registrant,
	newRegistrant *models.Registrant,
	oldEmail string,
	newEmail string,
) error {
	var errors []error

	// Send cancellation email to old email address
	oldEmailRegistrant := *oldRegistrant
	oldEmailRegistrant.Email = oldEmail
	err := s.SendRegistrantCancellationEmail(ctx, &oldEmailRegistrant, meeting)
	if err != nil {
		slog.ErrorContext(ctx, "failed to send cancellation email to old address",
			"old_email", redaction.RedactEmail(oldEmail),
			logging.ErrKey, err)
		errors = append(errors, fmt.Errorf("failed to send cancellation email to %s: %w", oldEmail, err))
	}

	// Send invitation email to new email address
	newEmailRegistrant := *newRegistrant
	newEmailRegistrant.Email = newEmail
	err = s.SendRegistrantInvitationEmail(ctx, &newEmailRegistrant)
	if err != nil {
		slog.ErrorContext(ctx, "failed to send invitation email to new address",
			"new_email", redaction.RedactEmail(newEmail),
			logging.ErrKey, err)
		errors = append(errors, fmt.Errorf("failed to send invitation email to %s: %w", newEmail, err))
	}

	if len(errors) > 0 {
		return fmt.Errorf("email notification errors: %v", errors)
	}

	slog.InfoContext(ctx, "sent email change notifications",
		"meeting_uid", meeting.UID,
		"registrant_uid", oldRegistrant.UID,
		"old_email", redaction.RedactEmail(oldEmail),
		"new_email", redaction.RedactEmail(newEmail))

	return nil
}

// SendRegistrantInvitationEmail sends an invitation email to a newly created registrant
func (s *MeetingRegistrantService) SendRegistrantInvitationEmail(ctx context.Context, registrant *models.Registrant) error {
	meetingDB, err := s.meetingRepository.GetBase(ctx, registrant.MeetingUID)
	if err != nil {
		return fmt.Errorf("failed to get meeting details: %w", err)
	}

	recipientName := fmt.Sprintf("%s %s", registrant.FirstName, registrant.LastName)
	if recipientName == " " {
		recipientName = ""
	}

	var meetingID, passcode string
	if meetingDB.ZoomConfig != nil {
		meetingID = meetingDB.ZoomConfig.MeetingID
		passcode = meetingDB.ZoomConfig.Passcode
	}

	projectName, _ := s.messageBuilder.GetProjectName(ctx, meetingDB.ProjectUID)
	projectLogo, _ := s.messageBuilder.GetProjectLogo(ctx, meetingDB.ProjectUID)
	projectSlug, _ := s.messageBuilder.GetProjectSlug(ctx, meetingDB.ProjectUID)

	// Try to get the project logo as a PNG image.
	// If there is a project logo and it is in .svg format, then use the .png format of the logo image instead.
	if projectLogo != "" && strings.HasSuffix(projectLogo, ".svg") {
		projectLogoUrl := fmt.Sprintf("%s/%s.png", s.config.ProjectLogoBaseURL, meetingDB.ProjectUID)

		// Check that the project logo is reachable
		resp, err := http.Get(projectLogoUrl)
		if err == nil && resp != nil && resp.StatusCode == http.StatusOK {
			projectLogo = projectLogoUrl
		} else {
			slog.WarnContext(ctx, "unable to confirm that the project logo url is accessible", "url", projectLogoUrl, "status_code", resp.StatusCode)
		}
		if resp != nil {
			_ = resp.Body.Close()
		}
	}

	invitation := domain.EmailInvitation{
		MeetingUID:         meetingDB.UID,
		RecipientEmail:     registrant.Email,
		RecipientName:      recipientName,
		MeetingTitle:       meetingDB.Title,
		StartTime:          meetingDB.StartTime,
		Duration:           meetingDB.Duration,
		Timezone:           meetingDB.Timezone,
		Description:        meetingDB.Description,
		Visibility:         meetingDB.Visibility,
		MeetingType:        meetingDB.MeetingType,
		JoinLink:           constants.GenerateLFXMeetingURL(meetingDB.UID, meetingDB.Password, s.config.LFXEnvironment),
		MeetingDetailsLink: constants.GenerateLFXMeetingDetailsURL(projectSlug, meetingDB.UID, s.config.LFXEnvironment),
		ProjectName:        projectName,
		ProjectLogo:        projectLogo,
		Platform:           meetingDB.Platform,
		MeetingID:          meetingID,
		Passcode:           passcode,
		Recurrence:         meetingDB.Recurrence,
		IcsSequence:        meetingDB.IcsSequence,
	}

	return s.emailService.SendRegistrantInvitation(ctx, invitation)
}

// SendRegistrantUpdatedInvitation sends an updated invitation email to a registrant
func (s *MeetingRegistrantService) SendRegistrantUpdatedInvitation(ctx context.Context, registrant *models.Registrant, meeting *models.MeetingBase, oldMeeting *models.MeetingBase, changes map[string]any, meetingID, passcode string) error {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "service not initialized", logging.PriorityCritical())
		return domain.NewUnavailableError("meeting registrant service is not ready")
	}

	if oldMeeting == nil {
		slog.ErrorContext(ctx, "old meeting object missing; unable to send updated invitation")
		return errors.New("old meeting object missing")
	}

	ctx = logging.AppendCtx(ctx, slog.String("registrant_uid", registrant.UID))
	ctx = logging.AppendCtx(ctx, slog.String("meeting_uid", meeting.UID))
	ctx = logging.AppendCtx(ctx, slog.String("email", redaction.RedactEmail(registrant.Email)))

	recipientName := fmt.Sprintf("%s %s", registrant.FirstName, registrant.LastName)
	if recipientName == " " {
		recipientName = ""
	}

	projectName, _ := s.messageBuilder.GetProjectName(ctx, meeting.ProjectUID)
	projectLogo, _ := s.messageBuilder.GetProjectLogo(ctx, meeting.ProjectUID)
	projectSlug, _ := s.messageBuilder.GetProjectSlug(ctx, meeting.ProjectUID)

	updatedInvitation := domain.EmailUpdatedInvitation{
		MeetingUID:         meeting.UID,
		RecipientEmail:     registrant.Email,
		RecipientName:      recipientName,
		MeetingTitle:       meeting.Title,
		StartTime:          meeting.StartTime,
		Duration:           meeting.Duration,
		Timezone:           meeting.Timezone,
		Description:        meeting.Description,
		Visibility:         meeting.Visibility,
		MeetingType:        meeting.MeetingType,
		JoinLink:           constants.GenerateLFXMeetingURL(meeting.UID, meeting.Password, s.config.LFXEnvironment),
		MeetingDetailsLink: constants.GenerateLFXMeetingDetailsURL(projectSlug, meeting.UID, s.config.LFXEnvironment),
		Platform:           meeting.Platform,
		MeetingID:          meetingID,
		Passcode:           passcode,
		Recurrence:         meeting.Recurrence,
		Changes:            changes,
		ProjectName:        projectName,
		ProjectLogo:        projectLogo,
		IcsSequence:        meeting.IcsSequence,
		// Previous meeting data
		OldStartTime:   oldMeeting.StartTime,
		OldDuration:    oldMeeting.Duration,
		OldTimezone:    oldMeeting.Timezone,
		OldRecurrence:  oldMeeting.Recurrence,
		OldDescription: oldMeeting.Description,
	}

	return s.emailService.SendRegistrantUpdatedInvitation(ctx, updatedInvitation)
}

// SendRegistrantCancellationEmail sends a cancellation email to a deleted registrant
func (s *MeetingRegistrantService) SendRegistrantCancellationEmail(
	ctx context.Context,
	registrant *models.Registrant,
	meeting *models.MeetingBase,
) error {
	if meeting == nil {
		slog.WarnContext(ctx, "meeting object missing; unable to send cancellation email")
		return errors.New("meeting object missing")
	}

	recipientName := fmt.Sprintf("%s %s", registrant.FirstName, registrant.LastName)
	if recipientName == " " {
		recipientName = ""
	}

	projectName, _ := s.messageBuilder.GetProjectName(ctx, meeting.ProjectUID)
	projectLogo, _ := s.messageBuilder.GetProjectLogo(ctx, meeting.ProjectUID)
	projectSlug, _ := s.messageBuilder.GetProjectSlug(ctx, meeting.ProjectUID)

	cancellation := domain.EmailCancellation{
		MeetingUID:         meeting.UID,
		RecipientEmail:     registrant.Email,
		RecipientName:      recipientName,
		MeetingTitle:       meeting.Title,
		StartTime:          meeting.StartTime,
		Duration:           meeting.Duration,
		Timezone:           meeting.Timezone,
		Description:        meeting.Description,
		Visibility:         meeting.Visibility,
		MeetingType:        meeting.MeetingType,
		Platform:           meeting.Platform,
		MeetingDetailsLink: constants.GenerateLFXMeetingDetailsURL(projectSlug, meeting.UID, s.config.LFXEnvironment),
		ProjectName:        projectName,
		ProjectLogo:        projectLogo,
		Reason:             "Your registration has been removed from this meeting.",
		Recurrence:         meeting.Recurrence,
		IcsSequence:        meeting.IcsSequence,
	}

	return s.emailService.SendRegistrantCancellation(ctx, cancellation)
}
