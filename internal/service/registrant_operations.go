// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"sync"

	"github.com/google/uuid"
	meetingsvc "github.com/linuxfoundation/lfx-v2-meeting-service/gen/meeting_service"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/constants"
)

func (s *MeetingsService) validateCreateMeetingRegistrantPayload(ctx context.Context, payload *meetingsvc.CreateMeetingRegistrantPayload) error {
	// Check if the meeting exists
	_, err := s.MeetingRepository.GetBase(ctx, payload.MeetingUID)
	if err != nil {
		if errors.Is(err, domain.ErrMeetingNotFound) {
			slog.WarnContext(ctx, "meeting not found", logging.ErrKey, err)
			return domain.ErrMeetingNotFound
		}
		slog.ErrorContext(ctx, "error getting meeting", logging.ErrKey, err)
		return domain.ErrInternal
	}

	// Check that there isn't already a registrant with the same email address for this meeting.
	registrants, err := s.RegistrantRepository.ListByEmail(ctx, payload.Email)
	if err != nil {
		slog.ErrorContext(ctx, "error listing registrants by email", logging.ErrKey, err)
		return domain.ErrInternal
	}
	for _, registrant := range registrants {
		if registrant.Email == payload.Email && registrant.MeetingUID == payload.MeetingUID {
			slog.WarnContext(ctx, "registrant already exists for meeting with same email address", logging.ErrKey, domain.ErrRegistrantAlreadyExists)
			return domain.ErrRegistrantAlreadyExists
		}
	}

	// TODO: add validation about occurrence ID once we occurrences calculated for meetings

	return nil
}

// createRegistrantContext creates a background context with registrant and meeting UID attributes for async operations
func createRegistrantContext(ctx context.Context, registrantUID, meetingUID string) context.Context {
	bgCtx := logging.AppendCtx(ctx, slog.String("registrant_uid", registrantUID))
	return logging.AppendCtx(bgCtx, slog.String("meeting_uid", meetingUID))
}

// CreateMeetingRegistrant creates a new registrant for a meeting
func (s *MeetingsService) CreateMeetingRegistrant(ctx context.Context, payload *meetingsvc.CreateMeetingRegistrantPayload) (*meetingsvc.Registrant, error) {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "NATS connection or store not initialized", logging.PriorityCritical())
		return nil, domain.ErrServiceUnavailable
	}

	if payload == nil {
		slog.WarnContext(ctx, "payload is required")
		return nil, domain.ErrValidationFailed
	}

	ctx = logging.AppendCtx(ctx, slog.String("meeting_uid", payload.MeetingUID))

	// Validate the payload
	err := s.validateCreateMeetingRegistrantPayload(ctx, payload)
	if err != nil {
		return nil, err
	}

	// Convert payload to domain model
	registrantDB := models.ToRegistrantDBModelFromCreatePayload(payload)
	if registrantDB == nil {
		// This should never happen since we validate the payload above.
		// Therefore we can return an internal error.
		return nil, domain.ErrInternal
	}

	// Generate UID for the registrant
	registrantDB.UID = uuid.New().String()
	registrantDB.MeetingUID = payload.MeetingUID

	ctx = logging.AppendCtx(ctx, slog.String("registrant_uid", registrantDB.UID))

	// Create the registrant
	err = s.RegistrantRepository.Create(ctx, registrantDB)
	if err != nil {
		if errors.Is(err, domain.ErrRegistrantAlreadyExists) {
			slog.WarnContext(ctx, "registrant already exists", logging.ErrKey, err)
			return nil, domain.ErrRegistrantAlreadyExists
		}
		slog.ErrorContext(ctx, "error creating registrant", logging.ErrKey, err)
		return nil, domain.ErrInternal
	}

	registrant := models.FromRegistrantDBModel(registrantDB)

	slog.DebugContext(ctx, "created registrant", "registrant", registrant)

	// Send NATS messages and invitation email asynchronously
	var wg sync.WaitGroup

	// Send indexing message for the new registrant
	wg.Add(1)
	go func() {
		defer wg.Done()
		msgCtx := createRegistrantContext(ctx, registrantDB.UID, registrantDB.MeetingUID)

		err := s.MessageBuilder.SendIndexMeetingRegistrant(msgCtx, models.ActionCreated, *registrantDB)
		if err != nil {
			slog.ErrorContext(msgCtx, "error sending indexing message for new registrant", logging.ErrKey, err)
		}
	}()

	// Send a message about the new registrant to the fga-sync service
	if registrantDB.Username != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			msgCtx := createRegistrantContext(ctx, registrantDB.UID, registrantDB.MeetingUID)

			err := s.MessageBuilder.SendPutMeetingRegistrantAccess(msgCtx, models.MeetingRegistrantAccessMessage{
				UID:        registrantDB.UID,
				Username:   registrantDB.Username,
				MeetingUID: registrantDB.MeetingUID,
				Host:       registrantDB.Host,
			})
			if err != nil {
				slog.ErrorContext(msgCtx, "error sending message about new registrant", logging.ErrKey, err)
			}
		}()
	} else {
		// This can happen when the registrant is not an LF user but rather a guest user.
		slog.DebugContext(ctx, "no username for registrant, skipping access message")
	}

	// Send invitation email to the registrant
	wg.Add(1)
	go func() {
		defer wg.Done()
		emailCtx := createRegistrantContext(ctx, registrantDB.UID, registrantDB.MeetingUID)

		err := s.sendRegistrantInvitationEmail(emailCtx, registrantDB)
		if err != nil {
			slog.ErrorContext(emailCtx, "failed to send invitation email", logging.ErrKey, err)
		}
	}()

	// Wait for all async operations to complete before returning
	wg.Wait()

	return registrant, nil
}

// GetMeetingRegistrants gets all registrants for a meeting
func (s *MeetingsService) GetMeetingRegistrants(ctx context.Context, payload *meetingsvc.GetMeetingRegistrantsPayload) (*meetingsvc.GetMeetingRegistrantsResult, error) {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "NATS connection or store not initialized", logging.PriorityCritical())
		return nil, domain.ErrServiceUnavailable
	}

	if payload == nil || payload.UID == nil {
		slog.WarnContext(ctx, "meeting UID is required")
		return nil, domain.ErrValidationFailed
	}

	meetingUID := *payload.UID
	ctx = logging.AppendCtx(ctx, slog.String("meeting_uid", meetingUID))

	// Check if the meeting exists
	_, err := s.MeetingRepository.GetBase(ctx, meetingUID)
	if err != nil {
		if errors.Is(err, domain.ErrMeetingNotFound) {
			slog.WarnContext(ctx, "meeting not found", logging.ErrKey, err)
			return nil, domain.ErrMeetingNotFound
		}
		slog.ErrorContext(ctx, "error getting meeting", logging.ErrKey, err)
		return nil, domain.ErrInternal
	}

	slog.DebugContext(ctx, "meeting found", "meeting_uid", meetingUID)

	// Get all registrants for the meeting
	registrantsDB, err := s.RegistrantRepository.ListByMeeting(ctx, meetingUID)
	if err != nil {
		slog.ErrorContext(ctx, "error listing meeting registrants", logging.ErrKey, err)
		return nil, domain.ErrInternal
	}

	slog.DebugContext(ctx, "listing meeting registrants", "meeting_uid", meetingUID)

	registrants := make([]*meetingsvc.Registrant, len(registrantsDB))
	for i, registrantDB := range registrantsDB {
		registrants[i] = models.FromRegistrantDBModel(registrantDB)
	}

	result := &meetingsvc.GetMeetingRegistrantsResult{
		Registrants:  registrants,
		CacheControl: nil, // TODO: Add cache control logic if needed
	}

	slog.DebugContext(ctx, "returning meeting registrants", "count", len(registrants))

	return result, nil
}

// GetMeetingRegistrant gets a specific registrant by UID
func (s *MeetingsService) GetMeetingRegistrant(ctx context.Context, payload *meetingsvc.GetMeetingRegistrantPayload) (*meetingsvc.GetMeetingRegistrantResult, error) {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "NATS connection or store not initialized", logging.PriorityCritical())
		return nil, domain.ErrServiceUnavailable
	}

	if payload == nil || payload.MeetingUID == nil || payload.UID == nil {
		slog.WarnContext(ctx, "meeting UID and registrant UID are required")
		return nil, domain.ErrValidationFailed
	}

	meetingUID := *payload.MeetingUID
	registrantUID := *payload.UID

	ctx = logging.AppendCtx(ctx, slog.String("meeting_uid", meetingUID))
	ctx = logging.AppendCtx(ctx, slog.String("registrant_uid", registrantUID))

	// Check that meeting exists
	exists, err := s.MeetingRepository.Exists(ctx, meetingUID)
	if err != nil {
		slog.ErrorContext(ctx, "error checking if meeting exists", logging.ErrKey, err)
		return nil, domain.ErrInternal
	}
	if !exists {
		slog.WarnContext(ctx, "meeting not found", logging.ErrKey, domain.ErrMeetingNotFound)
		return nil, domain.ErrMeetingNotFound
	}

	// Get registrant with revision from store
	registrantDB, revision, err := s.RegistrantRepository.GetWithRevision(ctx, registrantUID)
	if err != nil {
		if errors.Is(err, domain.ErrRegistrantNotFound) {
			slog.WarnContext(ctx, "registrant not found", logging.ErrKey, err)
			return nil, domain.ErrRegistrantNotFound
		}
		slog.ErrorContext(ctx, "error getting registrant from store", logging.ErrKey, err)
		return nil, domain.ErrInternal
	}

	registrant := models.FromRegistrantDBModel(registrantDB)

	// Store the revision in context for the custom encoder to use
	revisionStr := strconv.FormatUint(revision, 10)
	ctx = context.WithValue(ctx, constants.ETagContextID, revisionStr)

	result := &meetingsvc.GetMeetingRegistrantResult{
		Registrant: registrant,
		Etag:       &revisionStr,
	}

	slog.DebugContext(ctx, "returning registrant", "registrant", registrant, "revision", revision)

	return result, nil
}

func (s *MeetingsService) validateUpdateMeetingRegistrantPayload(ctx context.Context, payload *meetingsvc.UpdateMeetingRegistrantPayload, existingRegistrant *models.Registrant) error {
	// Check that the meeting exists
	exists, err := s.MeetingRepository.Exists(ctx, existingRegistrant.MeetingUID)
	if err != nil {
		slog.ErrorContext(ctx, "error checking if meeting exists", logging.ErrKey, err)
		return domain.ErrInternal
	}
	if !exists {
		slog.WarnContext(ctx, "meeting not found", logging.ErrKey, domain.ErrMeetingNotFound)
		return domain.ErrMeetingNotFound
	}

	if existingRegistrant.Email != payload.Email {
		// If changing the email address, check that there isn't already a registrant for this meeting with the new email address.
		registrants, err := s.RegistrantRepository.ListByEmail(ctx, payload.Email)
		if err != nil {
			slog.ErrorContext(ctx, "error listing registrants by email", logging.ErrKey, err)
			return domain.ErrInternal
		}
		for _, registrant := range registrants {
			if registrant.Email == payload.Email && registrant.MeetingUID == existingRegistrant.MeetingUID {
				slog.WarnContext(ctx, "registrant already exists for meeting with same email address", logging.ErrKey, domain.ErrRegistrantAlreadyExists)
				return domain.ErrRegistrantAlreadyExists
			}
		}
	}

	// TODO: add validation about occurrence ID once we occurrences calculated for meetings

	return nil
}

// UpdateMeetingRegistrant updates an existing registrant
func (s *MeetingsService) UpdateMeetingRegistrant(ctx context.Context, payload *meetingsvc.UpdateMeetingRegistrantPayload) (*meetingsvc.Registrant, error) {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "NATS connection or store not initialized", logging.PriorityCritical())
		return nil, domain.ErrServiceUnavailable
	}

	if payload == nil || payload.UID == nil {
		slog.WarnContext(ctx, "registrant UID is required")
		return nil, domain.ErrValidationFailed
	}

	meetingUID := payload.MeetingUID
	registrantUID := *payload.UID

	ctx = logging.AppendCtx(ctx, slog.String("meeting_uid", meetingUID))
	ctx = logging.AppendCtx(ctx, slog.String("registrant_uid", registrantUID))

	var revision uint64
	var err error
	if !s.Config.SkipEtagValidation {
		if payload.IfMatch == nil {
			slog.WarnContext(ctx, "If-Match header is missing")
			return nil, domain.ErrValidationFailed
		}
		revision, err = strconv.ParseUint(*payload.IfMatch, 10, 64)
		if err != nil {
			slog.ErrorContext(ctx, "error parsing If-Match header", logging.ErrKey, err)
			return nil, domain.ErrValidationFailed
		}
	} else {
		// If skipping the Etag validation, we need to get the key revision from the store with a Get request.
		_, revision, err = s.RegistrantRepository.GetWithRevision(ctx, registrantUID)
		if err != nil {
			if errors.Is(err, domain.ErrRegistrantNotFound) {
				slog.WarnContext(ctx, "registrant not found", logging.ErrKey, err)
				return nil, domain.ErrRegistrantNotFound
			}
			slog.ErrorContext(ctx, "error getting registrant from store", logging.ErrKey, err)
			return nil, domain.ErrInternal
		}
	}

	ctx = logging.AppendCtx(ctx, slog.String("etag", strconv.FormatUint(revision, 10)))

	// Check if the registrant exists and get existing data for the update
	existingRegistrantDB, err := s.RegistrantRepository.Get(ctx, registrantUID)
	if err != nil {
		if errors.Is(err, domain.ErrRegistrantNotFound) {
			slog.WarnContext(ctx, "registrant not found", logging.ErrKey, err)
			return nil, domain.ErrRegistrantNotFound
		}
		slog.ErrorContext(ctx, "error checking if registrant exists", logging.ErrKey, err)
		return nil, domain.ErrInternal
	}

	// Validate the payload
	err = s.validateUpdateMeetingRegistrantPayload(ctx, payload, existingRegistrantDB)
	if err != nil {
		return nil, err
	}

	// Convert payload to domain model
	registrantDB := models.ToRegistrantDBModelFromUpdatePayload(payload, existingRegistrantDB)
	if registrantDB == nil {
		// This should never happen since we validate the payload above.
		// Therefore we can return an internal error.
		return nil, domain.ErrInternal
	}

	// Update the registrant
	err = s.RegistrantRepository.Update(ctx, registrantDB, revision)
	if err != nil {
		if errors.Is(err, domain.ErrRevisionMismatch) {
			slog.WarnContext(ctx, "revision mismatch", logging.ErrKey, err)
			return nil, domain.ErrRevisionMismatch
		}
		slog.ErrorContext(ctx, "error updating registrant", logging.ErrKey, err)
		return nil, domain.ErrInternal
	}

	registrant := models.FromRegistrantDBModel(registrantDB)

	slog.DebugContext(ctx, "updated registrant", "registrant", registrant)

	// Send NATS messages asynchronously
	var wg sync.WaitGroup

	// Send indexing message for the updated registrant
	wg.Add(1)
	go func() {
		defer wg.Done()
		msgCtx := createRegistrantContext(ctx, registrantDB.UID, registrantDB.MeetingUID)

		err := s.MessageBuilder.SendIndexMeetingRegistrant(msgCtx, models.ActionUpdated, *registrantDB)
		if err != nil {
			slog.ErrorContext(msgCtx, "error sending indexing message for updated registrant", logging.ErrKey, err)
		}
	}()

	if registrantDB.Username != "" {
		// Send a message about the updated registrant to the fga-sync service
		wg.Add(1)
		go func() {
			defer wg.Done()
			msgCtx := createRegistrantContext(ctx, registrantDB.UID, registrantDB.MeetingUID)

			err := s.MessageBuilder.SendPutMeetingRegistrantAccess(msgCtx, models.MeetingRegistrantAccessMessage{
				UID:        registrantDB.UID,
				Username:   registrantDB.Username,
				MeetingUID: registrantDB.MeetingUID,
				Host:       registrantDB.Host,
			})
			if err != nil {
				slog.ErrorContext(msgCtx, "error sending message about updated registrant", logging.ErrKey, err)
			}
		}()
	} else {
		// This can happen when the registrant is not an LF user but rather a guest user.
		slog.DebugContext(ctx, "no username for registrant, skipping access message")
	}

	// Wait for all async operations to complete before returning
	wg.Wait()

	return registrant, nil
}

// deleteRegistrantWithCleanup is an internal helper that deletes a registrant and sends cleanup messages.
// It can optionally skip revision checking when skipRevisionCheck is true (useful for bulk cleanup operations).
func (s *MeetingsService) deleteRegistrantWithCleanup(ctx context.Context, registrantDB *models.Registrant, revision uint64, skipRevisionCheck bool) error {
	ctx = logging.AppendCtx(ctx, slog.String("registrant_uid", registrantDB.UID))

	// Delete the registrant from the database
	var err error
	if skipRevisionCheck {
		// Use revision 0 to skip revision checking for bulk cleanup operations
		err = s.RegistrantRepository.Delete(ctx, registrantDB.UID, 0)
	} else {
		err = s.RegistrantRepository.Delete(ctx, registrantDB.UID, revision)
	}

	if err != nil {
		if errors.Is(err, domain.ErrRevisionMismatch) && !skipRevisionCheck {
			slog.WarnContext(ctx, "revision mismatch", logging.ErrKey, err)
			return domain.ErrRevisionMismatch
		}
		if errors.Is(err, domain.ErrRegistrantNotFound) {
			// If skipping revision check, this is expected during cleanup
			if skipRevisionCheck {
				slog.DebugContext(ctx, "registrant already deleted, skipping")
				return nil
			}
			slog.WarnContext(ctx, "registrant not found", logging.ErrKey, err)
			return domain.ErrRegistrantNotFound
		}
		slog.ErrorContext(ctx, "error deleting registrant", logging.ErrKey, err)
		return domain.ErrInternal
	}

	slog.DebugContext(ctx, "deleted registrant")

	// Send cleanup messages and cancellation email asynchronously
	var wg sync.WaitGroup

	// Send indexing delete message for the registrant
	wg.Add(1)
	go func() {
		defer wg.Done()
		msgCtx := createRegistrantContext(ctx, registrantDB.UID, registrantDB.MeetingUID)

		err := s.MessageBuilder.SendDeleteIndexMeetingRegistrant(msgCtx, registrantDB.UID)
		if err != nil {
			slog.ErrorContext(msgCtx, "error sending delete indexing message for registrant", logging.ErrKey, err, logging.PriorityCritical())
		}
	}()

	// Send access removal message if the registrant has a username
	if registrantDB.Username != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			msgCtx := createRegistrantContext(ctx, registrantDB.UID, registrantDB.MeetingUID)

			err := s.MessageBuilder.SendRemoveMeetingRegistrantAccess(msgCtx, models.MeetingRegistrantAccessMessage{
				UID:        registrantDB.UID,
				Username:   registrantDB.Username,
				MeetingUID: registrantDB.MeetingUID,
				Host:       registrantDB.Host,
			})
			if err != nil {
				slog.ErrorContext(msgCtx, "error sending message about deleted registrant", logging.ErrKey, err, logging.PriorityCritical())
			}
		}()
	} else {
		// This can happen when the registrant is not an LF user but rather a guest user.
		slog.DebugContext(ctx, "no username for registrant, skipping access message")
	}

	// Send cancellation email to the registrant
	wg.Add(1)
	go func() {
		defer wg.Done()
		emailCtx := createRegistrantContext(ctx, registrantDB.UID, registrantDB.MeetingUID)

		err := s.sendRegistrantCancellationEmail(emailCtx, registrantDB)
		if err != nil {
			slog.ErrorContext(emailCtx, "failed to send cancellation email", logging.ErrKey, err)
		}
	}()

	// Wait for all async operations to complete before returning
	wg.Wait()

	return nil
}

// DeleteMeetingRegistrant deletes a registrant from a meeting
func (s *MeetingsService) DeleteMeetingRegistrant(ctx context.Context, payload *meetingsvc.DeleteMeetingRegistrantPayload) error {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "NATS connection or store not initialized", logging.PriorityCritical())
		return domain.ErrServiceUnavailable
	}

	if payload == nil || payload.MeetingUID == nil || payload.UID == nil {
		slog.WarnContext(ctx, "meeting UID and registrant UID are required")
		return domain.ErrValidationFailed
	}

	meetingUID := *payload.MeetingUID
	registrantUID := *payload.UID

	ctx = logging.AppendCtx(ctx, slog.String("meeting_uid", meetingUID))
	ctx = logging.AppendCtx(ctx, slog.String("registrant_uid", registrantUID))

	var revision uint64
	var err error
	if !s.Config.SkipEtagValidation {
		if payload.IfMatch == nil {
			slog.WarnContext(ctx, "If-Match header is missing")
			return domain.ErrValidationFailed
		}
		revision, err = strconv.ParseUint(*payload.IfMatch, 10, 64)
		if err != nil {
			slog.ErrorContext(ctx, "error parsing If-Match header", logging.ErrKey, err)
			return domain.ErrValidationFailed
		}
	} else {
		// If skipping the Etag validation, we need to get the key revision from the store with a Get request.
		_, revision, err = s.RegistrantRepository.GetWithRevision(ctx, registrantUID)
		if err != nil {
			if errors.Is(err, domain.ErrRegistrantNotFound) {
				slog.WarnContext(ctx, "registrant not found", logging.ErrKey, err)
				return domain.ErrRegistrantNotFound
			}
			slog.ErrorContext(ctx, "error getting registrant from store", logging.ErrKey, err)
			return domain.ErrInternal
		}
	}

	ctx = logging.AppendCtx(ctx, slog.String("etag", strconv.FormatUint(revision, 10)))

	// Check that meeting exists
	exists, err := s.MeetingRepository.Exists(ctx, meetingUID)
	if err != nil {
		slog.ErrorContext(ctx, "error checking if meeting exists", logging.ErrKey, err)
		return domain.ErrInternal
	}
	if !exists {
		slog.WarnContext(ctx, "meeting not found", logging.ErrKey, domain.ErrMeetingNotFound)
		return domain.ErrMeetingNotFound
	}

	// Check that the registrant exists, but also get the registrant data for the access deletion message
	registrantDB, err := s.RegistrantRepository.Get(ctx, registrantUID)
	if err != nil {
		if errors.Is(err, domain.ErrRegistrantNotFound) {
			slog.WarnContext(ctx, "registrant not found", logging.ErrKey, err)
			return domain.ErrRegistrantNotFound
		}
		slog.ErrorContext(ctx, "error getting registrant from store", logging.ErrKey, err)
		return domain.ErrInternal
	}

	// Use the helper to delete the registrant with cleanup
	return s.deleteRegistrantWithCleanup(ctx, registrantDB, revision, false)
}

// sendRegistrantInvitationEmail sends an invitation email to a newly created registrant
func (s *MeetingsService) sendRegistrantInvitationEmail(ctx context.Context, registrant *models.Registrant) error {
	// Get meeting details for the email
	meetingDB, err := s.MeetingRepository.GetBase(ctx, registrant.MeetingUID)
	if err != nil {
		return fmt.Errorf("failed to get meeting details: %w", err)
	}

	// Format recipient name
	recipientName := fmt.Sprintf("%s %s", registrant.FirstName, registrant.LastName)
	if recipientName == " " {
		recipientName = "" // If both names are empty, use empty string
	}

	// Construct join link if available
	joinLink := meetingDB.PublicLink
	if joinLink == "" && meetingDB.ZoomConfig != nil && meetingDB.ZoomConfig.MeetingID != "" {
		// Construct Zoom link if meeting ID is available
		joinLink = fmt.Sprintf("https://zoom.us/j/%s", meetingDB.ZoomConfig.MeetingID)
	}

	// Create email invitation
	invitation := domain.EmailInvitation{
		RecipientEmail: registrant.Email,
		RecipientName:  recipientName,
		MeetingTitle:   meetingDB.Title,
		StartTime:      meetingDB.StartTime,
		Duration:       meetingDB.Duration,
		Timezone:       meetingDB.Timezone,
		Description:    meetingDB.Description,
		JoinLink:       joinLink,
		ProjectName:    "", // TODO: Add project name once project service integration is available
	}

	// Send the email
	return s.EmailService.SendRegistrantInvitation(ctx, invitation)
}

// sendRegistrantCancellationEmail sends a cancellation email to a deleted registrant
func (s *MeetingsService) sendRegistrantCancellationEmail(ctx context.Context, registrant *models.Registrant) error {
	// Get meeting details for the email
	meetingDB, err := s.MeetingRepository.GetBase(ctx, registrant.MeetingUID)
	if err != nil {
		return fmt.Errorf("failed to get meeting details: %w", err)
	}

	// Format recipient name
	recipientName := fmt.Sprintf("%s %s", registrant.FirstName, registrant.LastName)
	if recipientName == " " {
		recipientName = "" // If both names are empty, use empty string
	}

	// Create email cancellation
	cancellation := domain.EmailCancellation{
		RecipientEmail: registrant.Email,
		RecipientName:  recipientName,
		MeetingTitle:   meetingDB.Title,
		StartTime:      meetingDB.StartTime,
		Duration:       meetingDB.Duration,
		Timezone:       meetingDB.Timezone,
		Description:    meetingDB.Description,
		ProjectName:    "", // TODO: Add project name once project service integration is available
		Reason:         "Your registration has been removed from this meeting.",
	}

	// Send the email
	return s.EmailService.SendRegistrantCancellation(ctx, cancellation)
}
