// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"errors"
	"log/slog"
	"strconv"

	"github.com/google/uuid"
	meetingsvc "github.com/linuxfoundation/lfx-v2-meeting-service/gen/meeting_service"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/constants"
)

// CreateMeetingRegistrant creates a new registrant for a meeting
func (s *MeetingsService) CreateMeetingRegistrant(ctx context.Context, payload *meetingsvc.CreateMeetingRegistrantPayload) (*meetingsvc.Registrant, error) {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "NATS connection or store not initialized")
		return nil, domain.ErrServiceUnavailable
	}

	if payload == nil {
		slog.WarnContext(ctx, "payload is required")
		return nil, domain.ErrValidationFailed
	}

	meetingUID := payload.MeetingUID
	if payload.UID != nil && *payload.UID != "" {
		meetingUID = *payload.UID
	}

	ctx = logging.AppendCtx(ctx, slog.String("meeting_uid", meetingUID))

	// Check if the meeting exists
	_, err := s.MeetingRepository.GetMeeting(ctx, meetingUID)
	if err != nil {
		if errors.Is(err, domain.ErrMeetingNotFound) {
			slog.WarnContext(ctx, "meeting not found", logging.ErrKey, err)
			return nil, domain.ErrMeetingNotFound
		}
		slog.ErrorContext(ctx, "error getting meeting", logging.ErrKey, err)
		return nil, domain.ErrInternal
	}

	// Convert payload to domain model
	registrantDB := models.ToRegistrantDBModelFromCreatePayload(payload)

	// Generate UID for the registrant
	registrantDB.UID = uuid.New().String()
	registrantDB.MeetingUID = meetingUID

	ctx = logging.AppendCtx(ctx, slog.String("registrant_uid", registrantDB.UID))

	// Create the registrant
	err = s.RegistrantRepository.CreateRegistrant(ctx, registrantDB)
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

	return registrant, nil
}

// GetMeetingRegistrants gets all registrants for a meeting
func (s *MeetingsService) GetMeetingRegistrants(ctx context.Context, payload *meetingsvc.GetMeetingRegistrantsPayload) (*meetingsvc.GetMeetingRegistrantsResult, error) {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "NATS connection or store not initialized")
		return nil, domain.ErrServiceUnavailable
	}

	if payload == nil || payload.UID == nil {
		slog.WarnContext(ctx, "meeting UID is required")
		return nil, domain.ErrValidationFailed
	}

	meetingUID := *payload.UID
	ctx = logging.AppendCtx(ctx, slog.String("meeting_uid", meetingUID))

	// Check if the meeting exists
	_, err := s.MeetingRepository.GetMeeting(ctx, meetingUID)
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
	registrantsDB, err := s.RegistrantRepository.ListMeetingRegistrants(ctx, meetingUID)
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
		slog.ErrorContext(ctx, "NATS connection or store not initialized")
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

	// Get registrant with revision from store
	registrantDB, revision, err := s.RegistrantRepository.GetRegistrantWithRevision(ctx, registrantUID)
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

// UpdateMeetingRegistrant updates an existing registrant
func (s *MeetingsService) UpdateMeetingRegistrant(ctx context.Context, payload *meetingsvc.UpdateMeetingRegistrantPayload) (*meetingsvc.Registrant, error) {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "NATS connection or store not initialized")
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
		if payload.Etag == nil {
			slog.WarnContext(ctx, "ETag header is missing")
			return nil, domain.ErrValidationFailed
		}
		revision, err = strconv.ParseUint(*payload.Etag, 10, 64)
		if err != nil {
			slog.ErrorContext(ctx, "error parsing ETag", logging.ErrKey, err)
			return nil, domain.ErrValidationFailed
		}
	} else {
		// If skipping the Etag validation, we need to get the key revision from the store with a Get request.
		_, revision, err = s.RegistrantRepository.GetRegistrantWithRevision(ctx, registrantUID)
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
	existingRegistrantDB, err := s.RegistrantRepository.GetRegistrant(ctx, registrantUID)
	if err != nil {
		if errors.Is(err, domain.ErrRegistrantNotFound) {
			slog.WarnContext(ctx, "registrant not found", logging.ErrKey, err)
			return nil, domain.ErrRegistrantNotFound
		}
		slog.ErrorContext(ctx, "error checking if registrant exists", logging.ErrKey, err)
		return nil, domain.ErrInternal
	}

	// Convert payload to domain model
	registrantDB := models.ToRegistrantDBModelFromUpdatePayload(payload, existingRegistrantDB)

	// Update the registrant
	err = s.RegistrantRepository.UpdateRegistrant(ctx, registrantDB, revision)
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

	return registrant, nil
}

// DeleteMeetingRegistrant deletes a registrant from a meeting
func (s *MeetingsService) DeleteMeetingRegistrant(ctx context.Context, payload *meetingsvc.DeleteMeetingRegistrantPayload) error {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "NATS connection or store not initialized")
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
		if payload.Etag == nil {
			slog.WarnContext(ctx, "ETag header is missing")
			return domain.ErrValidationFailed
		}
		revision, err = strconv.ParseUint(*payload.Etag, 10, 64)
		if err != nil {
			slog.ErrorContext(ctx, "error parsing ETag", logging.ErrKey, err)
			return domain.ErrValidationFailed
		}
	} else {
		// If skipping the Etag validation, we need to get the key revision from the store with a Get request.
		_, revision, err = s.RegistrantRepository.GetRegistrantWithRevision(ctx, registrantUID)
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

	// Delete the registrant with revision check
	err = s.RegistrantRepository.DeleteRegistrant(ctx, registrantUID, revision)
	if err != nil {
		if errors.Is(err, domain.ErrRevisionMismatch) {
			slog.WarnContext(ctx, "revision mismatch", logging.ErrKey, err)
			return domain.ErrRevisionMismatch
		}
		if errors.Is(err, domain.ErrRegistrantNotFound) {
			slog.WarnContext(ctx, "registrant not found", logging.ErrKey, err)
			return domain.ErrRegistrantNotFound
		}
		slog.ErrorContext(ctx, "error deleting registrant", logging.ErrKey, err)
		return domain.ErrInternal
	}

	slog.DebugContext(ctx, "deleted registrant")

	return nil
}
