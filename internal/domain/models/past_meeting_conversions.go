// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package models

import (
	"time"

	meetingservice "github.com/linuxfoundation/lfx-v2-meeting-service/gen/meeting_service"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/utils"
)

// ToPastMeetingServiceModel converts a domain PastMeeting to service model
func ToPastMeetingServiceModel(pastMeeting *PastMeeting) *meetingservice.PastMeeting {
	if pastMeeting == nil {
		return nil
	}

	result := &meetingservice.PastMeeting{
		UID:                  utils.StringPtr(pastMeeting.UID),
		MeetingUID:           utils.StringPtr(pastMeeting.MeetingUID),
		ProjectUID:           utils.StringPtr(pastMeeting.ProjectUID),
		ScheduledStartTime:   utils.StringPtr(pastMeeting.ScheduledStartTime.Format(time.RFC3339)),
		ScheduledEndTime:     utils.StringPtr(pastMeeting.ScheduledEndTime.Format(time.RFC3339)),
		Duration:             utils.IntPtr(pastMeeting.Duration),
		Timezone:             utils.StringPtr(pastMeeting.Timezone),
		Title:                utils.StringPtr(pastMeeting.Title),
		Description:          utils.StringPtr(pastMeeting.Description),
		Platform:             utils.StringPtr(pastMeeting.Platform),
		Restricted:           utils.BoolPtr(pastMeeting.Restricted),
		RecordingEnabled:     utils.BoolPtr(pastMeeting.RecordingEnabled),
		TranscriptEnabled:    utils.BoolPtr(pastMeeting.TranscriptEnabled),
		YoutubeUploadEnabled: utils.BoolPtr(pastMeeting.YoutubeUploadEnabled),
	}

	// Set optional string fields if they are not empty
	if pastMeeting.OccurrenceID != "" {
		result.OccurrenceID = utils.StringPtr(pastMeeting.OccurrenceID)
	}
	if pastMeeting.PlatformMeetingID != "" {
		result.PlatformMeetingID = utils.StringPtr(pastMeeting.PlatformMeetingID)
	}
	if pastMeeting.MeetingType != "" {
		result.MeetingType = utils.StringPtr(pastMeeting.MeetingType)
	}
	if pastMeeting.Visibility != "" {
		result.Visibility = utils.StringPtr(pastMeeting.Visibility)
	}
	if pastMeeting.ArtifactVisibility != "" {
		result.ArtifactVisibility = utils.StringPtr(pastMeeting.ArtifactVisibility)
	}
	if pastMeeting.PublicLink != "" {
		result.PublicLink = utils.StringPtr(pastMeeting.PublicLink)
	}

	// Set optional int fields if they are not zero
	if pastMeeting.EarlyJoinTimeMinutes != 0 {
		result.EarlyJoinTimeMinutes = utils.IntPtr(pastMeeting.EarlyJoinTimeMinutes)
	}

	// Convert recurrence
	if pastMeeting.Recurrence != nil {
		result.Recurrence = fromDBRecurrence(pastMeeting.Recurrence)
	}

	// Convert committees
	if len(pastMeeting.Committees) > 0 {
		var committees []*meetingservice.Committee
		for _, c := range pastMeeting.Committees {
			committees = append(committees, &meetingservice.Committee{
				UID:                   c.UID,
				AllowedVotingStatuses: c.AllowedVotingStatuses,
			})
		}
		result.Committees = committees
	}

	// Convert zoom config
	if pastMeeting.ZoomConfig != nil {
		result.ZoomConfig = fromDBZoomConfig(pastMeeting.ZoomConfig)
	}

	// Convert sessions
	if len(pastMeeting.Sessions) > 0 {
		var sessions []*meetingservice.Session
		for _, s := range pastMeeting.Sessions {
			session := &meetingservice.Session{
				UID:       s.UID,
				StartTime: s.StartTime.Format(time.RFC3339),
			}
			if s.EndTime != nil {
				endTime := s.EndTime.Format(time.RFC3339)
				session.EndTime = &endTime
			}
			sessions = append(sessions, session)
		}
		result.Sessions = sessions
	}

	// Convert timestamps
	if pastMeeting.CreatedAt != nil {
		result.CreatedAt = utils.StringPtr(pastMeeting.CreatedAt.Format(time.RFC3339))
	}
	if pastMeeting.UpdatedAt != nil {
		result.UpdatedAt = utils.StringPtr(pastMeeting.UpdatedAt.Format(time.RFC3339))
	}

	return result
}
