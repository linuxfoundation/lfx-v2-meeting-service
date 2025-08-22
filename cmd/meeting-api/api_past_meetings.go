// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"log/slog"
	"strconv"

	meetingsvc "github.com/linuxfoundation/lfx-v2-meeting-service/gen/meeting_service"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/utils"
)

// GetPastMeetings implements the Goa service interface for listing past meetings
func (s *MeetingsAPI) GetPastMeetings(ctx context.Context, payload *meetingsvc.GetPastMeetingsPayload) (*meetingsvc.GetPastMeetingsResult, error) {
	if !s.service.ServiceReady() {
		slog.ErrorContext(ctx, "NATS connection or store not initialized", logging.PriorityCritical())
		return nil, domain.ErrServiceUnavailable
	}

	pastMeetings, err := s.service.GetPastMeetings(ctx)
	if err != nil {
		return nil, err
	}

	var goaPastMeetings []*meetingsvc.PastMeeting
	for _, pastMeeting := range pastMeetings {
		goaPastMeetings = append(goaPastMeetings, models.ToPastMeetingServiceModel(pastMeeting))
	}

	result := &meetingsvc.GetPastMeetingsResult{
		PastMeetings: goaPastMeetings,
		CacheControl: utils.StringPtr("public, max-age=300"),
	}

	return result, nil
}

// CreatePastMeeting implements the Goa service interface for creating past meetings
func (s *MeetingsAPI) CreatePastMeeting(ctx context.Context, payload *meetingsvc.CreatePastMeetingPayload) (*meetingsvc.PastMeeting, error) {
	if !s.service.ServiceReady() {
		slog.ErrorContext(ctx, "NATS connection or store not initialized", logging.PriorityCritical())
		return nil, domain.ErrServiceUnavailable
	}

	pastMeeting, err := s.service.CreatePastMeeting(ctx, payload)
	if err != nil {
		return nil, err
	}

	return models.ToPastMeetingServiceModel(pastMeeting), nil
}

// GetPastMeeting implements the Goa service interface for getting a single past meeting
func (s *MeetingsAPI) GetPastMeeting(ctx context.Context, payload *meetingsvc.GetPastMeetingPayload) (*meetingsvc.GetPastMeetingResult, error) {
	if !s.service.ServiceReady() {
		slog.ErrorContext(ctx, "NATS connection or store not initialized", logging.PriorityCritical())
		return nil, domain.ErrServiceUnavailable
	}

	pastMeeting, revision, err := s.service.GetPastMeeting(ctx, *payload.UID)
	if err != nil {
		return nil, err
	}

	result := &meetingsvc.GetPastMeetingResult{
		PastMeeting: models.ToPastMeetingServiceModel(pastMeeting),
		Etag:        utils.StringPtr(strconv.FormatUint(revision, 10)),
	}

	return result, nil
}

// DeletePastMeeting implements the Goa service interface for deleting past meetings
func (s *MeetingsAPI) DeletePastMeeting(ctx context.Context, payload *meetingsvc.DeletePastMeetingPayload) error {
	if !s.service.ServiceReady() {
		slog.ErrorContext(ctx, "NATS connection or store not initialized", logging.PriorityCritical())
		return domain.ErrServiceUnavailable
	}

	// Parse the revision from If-Match header
	revision, err := strconv.ParseUint(*payload.IfMatch, 10, 64)
	if err != nil {
		slog.WarnContext(ctx, "invalid If-Match header", logging.ErrKey, err)
		return domain.ErrValidationFailed
	}

	return s.service.DeletePastMeeting(ctx, *payload.UID, revision)
}
