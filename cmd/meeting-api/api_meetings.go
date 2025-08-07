// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"net/http"

	meetingsvc "github.com/linuxfoundation/lfx-v2-meeting-service/gen/meeting_service"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
)

// handleError converts domain errors to HTTP errors.
func handleError(err error) error {
	switch err {
	case domain.ErrServiceUnavailable:
		return createResponse(http.StatusServiceUnavailable, domain.ErrServiceUnavailable)
	case domain.ErrValidationFailed:
		return createResponse(http.StatusBadRequest, domain.ErrValidationFailed)
	case domain.ErrRevisionMismatch:
		return createResponse(http.StatusBadRequest, domain.ErrRevisionMismatch)
	case domain.ErrRegistrantAlreadyExists:
		return createResponse(http.StatusBadRequest, domain.ErrRegistrantAlreadyExists)
	case domain.ErrMeetingNotFound:
		return createResponse(http.StatusNotFound, domain.ErrMeetingNotFound)
	case domain.ErrRegistrantNotFound:
		return createResponse(http.StatusNotFound, domain.ErrRegistrantNotFound)
	case domain.ErrInternal, domain.ErrUnmarshal:
		return createResponse(http.StatusInternalServerError, domain.ErrInternal)
	}
	return err
}

// GetMeetings fetches all meetings
func (s *MeetingsAPI) GetMeetings(ctx context.Context, payload *meetingsvc.GetMeetingsPayload) (*meetingsvc.GetMeetingsResult, error) {
	meetings, err := s.service.GetMeetings(ctx)
	if err != nil {
		return nil, handleError(err)
	}

	return &meetingsvc.GetMeetingsResult{
		Meetings:     meetings,
		CacheControl: nil,
	}, nil
}

// CreateMeeting creates a new meeting.
func (s *MeetingsAPI) CreateMeeting(ctx context.Context, payload *meetingsvc.CreateMeetingPayload) (*meetingsvc.Meeting, error) {
	meeting, err := s.service.CreateMeeting(ctx, payload)
	if err != nil {
		return nil, handleError(err)
	}
	return meeting, nil
}

// GetMeeting gets a single meeting's base information.
func (s *MeetingsAPI) GetMeeting(ctx context.Context, payload *meetingsvc.GetMeetingPayload) (*meetingsvc.GetMeetingResult, error) {
	meeting, revision, err := s.service.GetOneMeeting(ctx, payload)
	if err != nil {
		return nil, handleError(err)
	}
	return &meetingsvc.GetMeetingResult{
		Meeting: meeting,
		Etag:    &revision,
	}, nil
}

// UpdateMeeting updates a meeting's base information.
func (s *MeetingsAPI) UpdateMeeting(ctx context.Context, payload *meetingsvc.UpdateMeetingPayload) (*meetingsvc.Meeting, error) {
	updatedMeeting, err := s.service.UpdateMeeting(ctx, payload)
	if err != nil {
		return nil, handleError(err)
	}
	return updatedMeeting, nil
}

// DeleteMeeting deletes a meeting.
func (s *MeetingsAPI) DeleteMeeting(ctx context.Context, payload *meetingsvc.DeleteMeetingPayload) error {
	err := s.service.DeleteMeeting(ctx, payload)
	if err != nil {
		return handleError(err)
	}
	return nil
}

// GetMeetingRegistrants gets all meeting registrants.
func (s *MeetingsAPI) GetMeetingRegistrants(ctx context.Context, payload *meetingsvc.GetMeetingRegistrantsPayload) (*meetingsvc.GetMeetingRegistrantsResult, error) {
	registrants, err := s.service.GetMeetingRegistrants(ctx, payload)
	if err != nil {
		return nil, handleError(err)
	}
	return registrants, nil
}

// CreateMeetingRegistrant creates a new meeting registrant.
func (s *MeetingsAPI) CreateMeetingRegistrant(ctx context.Context, payload *meetingsvc.CreateMeetingRegistrantPayload) (*meetingsvc.Registrant, error) {
	registrant, err := s.service.CreateMeetingRegistrant(ctx, payload)
	if err != nil {
		return nil, handleError(err)
	}
	return registrant, nil
}

// GetMeetingRegistrant gets a single meeting registrant.
func (s *MeetingsAPI) GetMeetingRegistrant(ctx context.Context, payload *meetingsvc.GetMeetingRegistrantPayload) (*meetingsvc.GetMeetingRegistrantResult, error) {
	registrant, err := s.service.GetMeetingRegistrant(ctx, payload)
	if err != nil {
		return nil, handleError(err)
	}
	return registrant, nil
}

// GetMeetingRegistrants gets all meeting registrants.
func (s *MeetingsAPI) UpdateMeetingRegistrant(ctx context.Context, payload *meetingsvc.UpdateMeetingRegistrantPayload) (*meetingsvc.Registrant, error) {
	registrant, err := s.service.UpdateMeetingRegistrant(ctx, payload)
	if err != nil {
		return nil, handleError(err)
	}
	return registrant, nil
}

// DeleteMeetingRegistrant deletes a meeting registrant.
func (s *MeetingsAPI) DeleteMeetingRegistrant(ctx context.Context, payload *meetingsvc.DeleteMeetingRegistrantPayload) error {
	err := s.service.DeleteMeetingRegistrant(ctx, payload)
	if err != nil {
		return handleError(err)
	}
	return nil
}
