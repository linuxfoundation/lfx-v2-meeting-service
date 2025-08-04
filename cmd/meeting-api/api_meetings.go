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
	case domain.ErrMeetingNotFound:
		return createResponse(http.StatusNotFound, domain.ErrMeetingNotFound)
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
	return nil, nil
}

// CreateMeetingRegistrant creates a new meeting registrant.
func (s *MeetingsAPI) CreateMeetingRegistrant(ctx context.Context, payload *meetingsvc.CreateMeetingRegistrantPayload) (*meetingsvc.Registrant, error) {
	return nil, nil
}

// GetMeetingRegistrant gets a single meeting registrant.
func (s *MeetingsAPI) GetMeetingRegistrant(ctx context.Context, payload *meetingsvc.GetMeetingRegistrantPayload) (*meetingsvc.GetMeetingRegistrantResult, error) {
	return nil, nil
}

// GetMeetingRegistrants gets all meeting registrants.
func (s *MeetingsAPI) UpdateMeetingRegistrant(ctx context.Context, payload *meetingsvc.UpdateMeetingRegistrantPayload) (*meetingsvc.Registrant, error) {
	return nil, nil
}

// DeleteMeetingRegistrant deletes a meeting registrant.
func (s *MeetingsAPI) DeleteMeetingRegistrant(ctx context.Context, payload *meetingsvc.DeleteMeetingRegistrantPayload) error {
	return nil
}
