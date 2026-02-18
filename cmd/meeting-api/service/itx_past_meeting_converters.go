// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	meetingservice "github.com/linuxfoundation/lfx-v2-meeting-service/gen/meeting_service"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/models/itx"
)

// ConvertUpdatePastMeetingPayload converts Goa payload to ITX update past meeting request
func ConvertUpdatePastMeetingPayload(payload *meetingservice.UpdateItxPastMeetingPayload) *itx.CreatePastMeetingRequest {
	req := &itx.CreatePastMeetingRequest{}

	// Optional fields - only set if provided
	if payload.MeetingID != nil {
		req.MeetingID = *payload.MeetingID
	}
	if payload.OccurrenceID != nil {
		req.OccurrenceID = *payload.OccurrenceID
	}
	if payload.ProjectUID != nil {
		req.ProjectID = *payload.ProjectUID
	}
	if payload.StartTime != nil {
		req.StartTime = *payload.StartTime
	}
	if payload.Duration != nil {
		req.Duration = *payload.Duration
	}
	if payload.Timezone != nil {
		req.Timezone = *payload.Timezone
	}
	if payload.Title != nil {
		req.Topic = *payload.Title
	}
	if payload.Description != nil {
		req.Agenda = *payload.Description
	}
	if payload.Restricted != nil {
		req.Restricted = *payload.Restricted
	}
	if payload.MeetingType != nil {
		req.MeetingType = *payload.MeetingType
	}
	if payload.Visibility != nil {
		req.Visibility = *payload.Visibility
	}
	if payload.RecordingEnabled != nil {
		req.RecordingEnabled = *payload.RecordingEnabled
	}
	if payload.TranscriptEnabled != nil {
		req.TranscriptEnabled = *payload.TranscriptEnabled
	}
	if payload.ArtifactVisibility != nil {
		req.RecordingAccess = *payload.ArtifactVisibility
		req.TranscriptAccess = *payload.ArtifactVisibility
	}

	if payload.Committees != nil {
		req.Committees = make([]itx.Committee, 0, len(payload.Committees))
		for _, c := range payload.Committees {
			if c == nil || c.UID == nil {
				continue
			}
			req.Committees = append(req.Committees, itx.Committee{
				ID:      *c.UID,
				Filters: c.AllowedVotingStatuses,
			})
		}
	}

	return req
}

// ConvertCreatePastMeetingPayload converts Goa payload to ITX create past meeting request
func ConvertCreatePastMeetingPayload(payload *meetingservice.CreateItxPastMeetingPayload) *itx.CreatePastMeetingRequest {
	req := &itx.CreatePastMeetingRequest{
		MeetingID:    payload.MeetingID,
		OccurrenceID: payload.OccurrenceID,
		ProjectID:    payload.ProjectUID,
		StartTime:    payload.StartTime,
		Duration:     payload.Duration,
		Timezone:     payload.Timezone,
	}

	// Optional fields
	if payload.Title != nil {
		req.Topic = *payload.Title
	}
	if payload.Description != nil {
		req.Agenda = *payload.Description
	}
	if payload.Restricted != nil {
		req.Restricted = *payload.Restricted
	}
	if payload.MeetingType != nil {
		req.MeetingType = *payload.MeetingType
	}
	if payload.Visibility != nil {
		req.Visibility = *payload.Visibility
	}
	if payload.RecordingEnabled != nil {
		req.RecordingEnabled = *payload.RecordingEnabled
	}
	if payload.TranscriptEnabled != nil {
		req.TranscriptEnabled = *payload.TranscriptEnabled
	}
	if payload.ArtifactVisibility != nil {
		req.RecordingAccess = *payload.ArtifactVisibility
		req.TranscriptAccess = *payload.ArtifactVisibility
	}

	if payload.Committees != nil {
		req.Committees = make([]itx.Committee, 0, len(payload.Committees))
		for _, c := range payload.Committees {
			if c == nil || c.UID == nil {
				continue
			}
			req.Committees = append(req.Committees, itx.Committee{
				ID:      *c.UID,
				Filters: c.AllowedVotingStatuses,
			})
		}
	}

	return req
}

// ConvertPastMeetingToGoa converts ITX past meeting response to Goa type
func ConvertPastMeetingToGoa(resp *itx.PastMeetingResponse) *meetingservice.ITXPastZoomMeeting {
	goaResp := &meetingservice.ITXPastZoomMeeting{
		// Identifiers
		ID:           ptrIfNotEmpty(resp.PastMeetingID),
		MeetingID:    ptrIfNotEmpty(resp.MeetingID),
		OccurrenceID: ptrIfNotEmpty(resp.OccurrenceID),

		// Project association
		ProjectUID: ptrIfNotEmpty(resp.ProjectID),

		// Meeting details
		Title:       ptrIfNotEmpty(resp.Topic),
		Description: ptrIfNotEmpty(resp.Agenda),
		StartTime:   ptrIfNotEmpty(resp.StartTime),
		Timezone:    ptrIfNotEmpty(resp.Timezone),
		Duration:    &resp.Duration,
		Visibility:  ptrIfNotEmpty(resp.Visibility),
		Restricted:  &resp.Restricted,
		MeetingType: ptrIfNotEmpty(resp.MeetingType),

		// Recording/Transcript settings
		RecordingEnabled:   &resp.RecordingEnabled,
		TranscriptEnabled:  &resp.TranscriptEnabled,
		ArtifactVisibility: ptrIfNotEmpty(resp.RecordingAccess),

		IsManuallyCreated: &resp.IsManuallyCreated,
	}

	// Convert committees
	if resp.Committees != nil {
		goaResp.Committees = make([]*meetingservice.Committee, len(resp.Committees))
		for i, c := range resp.Committees {
			uid := c.ID
			goaResp.Committees[i] = &meetingservice.Committee{
				UID:                   &uid,
				AllowedVotingStatuses: c.Filters,
			}
		}
	}

	return goaResp
}
