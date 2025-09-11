// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"log/slog"
	"time"

	meetingservice "github.com/linuxfoundation/lfx-v2-meeting-service/gen/meeting_service"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/utils"
)

// ConvertCreateMeetingPayloadToDomain converts a Goa CreateMeetingPayload to domain model
func ConvertCreateMeetingPayloadToDomain(payload *meetingservice.CreateMeetingPayload) (*models.MeetingFull, error) {
	// Convert payload to domain - split into Base and Settings
	base, err := convertCreateMeetingBasePayloadToDomain(payload)
	if err != nil {
		return nil, domain.ErrValidationFailed
	}
	settings := convertCreateMeetingSettingsPayloadToDomain(payload)

	request := &models.MeetingFull{
		Base:     base,
		Settings: settings,
	}

	return request, nil
}

func convertCreateMeetingBasePayloadToDomain(payload *meetingservice.CreateMeetingPayload) (*models.MeetingBase, error) {
	if payload == nil {
		return nil, domain.ErrValidationFailed
	}

	startTime, err := time.Parse(time.RFC3339, payload.StartTime)
	if err != nil {
		slog.Error("failed to parse start time", logging.ErrKey, err,
			"start_time", payload.StartTime,
		)
		return nil, domain.ErrValidationFailed
	}

	now := time.Now().UTC()
	meeting := &models.MeetingBase{
		ProjectUID:           payload.ProjectUID,
		StartTime:            startTime,
		Duration:             payload.Duration,
		Timezone:             payload.Timezone,
		Recurrence:           convertRecurrenceToDomain(payload.Recurrence),
		Title:                payload.Title,
		Description:          payload.Description,
		Committees:           convertCommitteesToDomain(payload.Committees),
		Platform:             utils.StringValue(payload.Platform),
		EarlyJoinTimeMinutes: utils.IntValue(payload.EarlyJoinTimeMinutes),
		MeetingType:          utils.StringValue(payload.MeetingType),
		Visibility:           utils.StringValue(payload.Visibility),
		Restricted:           utils.BoolValue(payload.Restricted),
		ArtifactVisibility:   utils.StringValue(payload.ArtifactVisibility),
		RecordingEnabled:     utils.BoolValue(payload.RecordingEnabled),
		TranscriptEnabled:    utils.BoolValue(payload.TranscriptEnabled),
		YoutubeUploadEnabled: utils.BoolValue(payload.YoutubeUploadEnabled),
		ZoomConfig:           convertZoomConfigPostToDomain(payload.ZoomConfig),
		CreatedAt:            &now,
		UpdatedAt:            &now,
	}

	return meeting, nil
}

func convertCreateMeetingSettingsPayloadToDomain(payload *meetingservice.CreateMeetingPayload) *models.MeetingSettings {
	now := time.Now().UTC()
	return &models.MeetingSettings{
		UID:        "", // This will get populated by the service
		Organizers: payload.Organizers,
		CreatedAt:  &now,
		UpdatedAt:  &now,
	}
}

func convertZoomConfigPostToDomain(z *meetingservice.ZoomConfigPost) *models.ZoomConfig {
	if z == nil {
		return nil
	}

	return &models.ZoomConfig{
		MeetingID:                "", // TODO: replace with actual zoom meeting ID once we have zoom integration
		AICompanionEnabled:       utils.BoolValue(z.AiCompanionEnabled),
		AISummaryRequireApproval: utils.BoolValue(z.AiSummaryRequireApproval),
	}
}

func convertCommitteesToDomain(committees []*meetingservice.Committee) []models.Committee {
	dbCommittees := make([]models.Committee, 0, len(committees))
	for _, c := range committees {
		if c != nil {
			dbCommittees = append(dbCommittees, convertCommitteeToDomain(c))
		}
	}
	return dbCommittees
}

func convertCommitteeToDomain(c *meetingservice.Committee) models.Committee {
	if c == nil {
		return models.Committee{}
	}

	return models.Committee{
		UID:                   c.UID,
		AllowedVotingStatuses: c.AllowedVotingStatuses,
	}
}

func convertRecurrenceToDomain(r *meetingservice.Recurrence) *models.Recurrence {
	if r == nil {
		return nil
	}

	recurrence := &models.Recurrence{
		Type:           r.Type,
		RepeatInterval: r.RepeatInterval,
		WeeklyDays:     utils.StringValue(r.WeeklyDays),
		MonthlyDay:     utils.IntValue(r.MonthlyDay),
		MonthlyWeek:    utils.IntValue(r.MonthlyWeek),
		MonthlyWeekDay: utils.IntValue(r.MonthlyWeekDay),
		EndTimes:       utils.IntValue(r.EndTimes),
	}

	// Convert EndDateTime
	if r.EndDateTime != nil {
		endDateTime, err := time.Parse(time.RFC3339, *r.EndDateTime)
		if err == nil {
			recurrence.EndDateTime = &endDateTime
		}
	}

	return recurrence
}

// ConvertMeetingUpdatePayloadToDomain converts a Goa UpdateMeetingBasePayload to domain model
func ConvertMeetingUpdatePayloadToDomain(payload *meetingservice.UpdateMeetingBasePayload) (*models.MeetingBase, error) {
	startTime, err := time.Parse(time.RFC3339, payload.StartTime)
	if err != nil {
		slog.Error("failed to parse start time", logging.ErrKey, err,
			"start_time", payload.StartTime,
		)
		return nil, domain.ErrValidationFailed
	}

	now := time.Now().UTC()
	meeting := &models.MeetingBase{
		UID:                  payload.UID,
		ProjectUID:           payload.ProjectUID,
		StartTime:            startTime,
		Duration:             payload.Duration,
		Timezone:             payload.Timezone,
		Recurrence:           convertRecurrenceToDomain(payload.Recurrence),
		Title:                payload.Title,
		Description:          payload.Description,
		Committees:           convertCommitteesToDomain(payload.Committees),
		Platform:             utils.StringValue(payload.Platform),
		EarlyJoinTimeMinutes: utils.IntValue(payload.EarlyJoinTimeMinutes),
		MeetingType:          utils.StringValue(payload.MeetingType),
		Visibility:           utils.StringValue(payload.Visibility),
		Restricted:           utils.BoolValue(payload.Restricted),
		ArtifactVisibility:   utils.StringValue(payload.ArtifactVisibility),
		PublicLink:           "", // This will get populated by the service from the existing meeting
		JoinURL:              "", // This will get populated by the service from the existing meeting
		RecordingEnabled:     utils.BoolValue(payload.RecordingEnabled),
		TranscriptEnabled:    utils.BoolValue(payload.TranscriptEnabled),
		YoutubeUploadEnabled: utils.BoolValue(payload.YoutubeUploadEnabled),
		ZoomConfig:           convertZoomConfigPostToDomain(payload.ZoomConfig),
		CreatedAt:            nil, // This will get populated by the service from the existing meeting
		UpdatedAt:            &now,
	}

	return meeting, nil
}

// ConvertUpdateSettingsPayloadToDomain converts a Goa UpdateMeetingSettingsPayload to domain model
func ConvertUpdateSettingsPayloadToDomain(payload *meetingservice.UpdateMeetingSettingsPayload) *models.MeetingSettings {
	now := time.Now().UTC()
	result := &models.MeetingSettings{
		UID:        utils.StringValue(payload.UID),
		Organizers: payload.Organizers,
		CreatedAt:  nil, // This will get populated by the service from the existing meeting
		UpdatedAt:  &now,
	}

	return result
}

// ConvertCreateRegistrantPayloadToDomain converts a Goa CreateMeetingRegistrantPayload type to the domain Registrant model for database storage
func ConvertCreateRegistrantPayloadToDomain(goaRegistrant *meetingservice.CreateMeetingRegistrantPayload) *models.Registrant {
	now := time.Now().UTC()
	registrant := &models.Registrant{
		UID:                "", // This will get populated by the service
		MeetingUID:         goaRegistrant.MeetingUID,
		Email:              goaRegistrant.Email,
		FirstName:          utils.StringValue(goaRegistrant.FirstName),
		LastName:           utils.StringValue(goaRegistrant.LastName),
		Host:               utils.BoolValue(goaRegistrant.Host),
		Type:               models.RegistrantTypeDirect, // Creating a registrant via the API must be direct type
		JobTitle:           utils.StringValue(goaRegistrant.JobTitle),
		OccurrenceID:       utils.StringValue(goaRegistrant.OccurrenceID),
		OrgName:            utils.StringValue(goaRegistrant.OrgName),
		OrgIsMember:        false, // This will get populated by the service
		OrgIsProjectMember: false, // This will get populated by the service
		AvatarURL:          utils.StringValue(goaRegistrant.AvatarURL),
		Username:           utils.StringValue(goaRegistrant.Username),
		CreatedAt:          &now,
		UpdatedAt:          &now,
	}

	return registrant
}

// ConvertUpdateRegistrantPayloadToDomain converts a Goa UpdateMeetingRegistrantPayload to a domain Registrant model
func ConvertUpdateRegistrantPayloadToDomain(payload *meetingservice.UpdateMeetingRegistrantPayload) *models.Registrant {
	now := time.Now().UTC()
	registrant := &models.Registrant{
		UID:                *payload.UID,
		MeetingUID:         payload.MeetingUID,
		Email:              payload.Email,
		FirstName:          utils.StringValue(payload.FirstName),
		LastName:           utils.StringValue(payload.LastName),
		Host:               utils.BoolValue(payload.Host),
		Type:               "", // This will be populated by the service because it depends on the existing value for the registrant
		JobTitle:           utils.StringValue(payload.JobTitle),
		OccurrenceID:       utils.StringValue(payload.OccurrenceID),
		OrgName:            utils.StringValue(payload.OrgName),
		OrgIsMember:        false, // This will get populated by the service
		OrgIsProjectMember: false, // This will get populated by the service
		AvatarURL:          utils.StringValue(payload.AvatarURL),
		Username:           utils.StringValue(payload.Username),
		CreatedAt:          nil, // This will get populated by the service
		UpdatedAt:          &now,
	}

	return registrant
}

// ConvertCreatePastMeetingParticipantPayloadToDomain converts a CreatePastMeetingParticipantPayload type to the domain PastMeetingParticipant model for database storage
func ConvertCreatePastMeetingParticipantPayloadToDomain(payload *meetingservice.CreatePastMeetingParticipantPayload) *models.PastMeetingParticipant {
	now := time.Now().UTC()
	participant := &models.PastMeetingParticipant{
		UID:                "", // This will get populated by the service
		PastMeetingUID:     utils.StringValue(payload.UID),
		MeetingUID:         "", // This will need to be populated by the service from the past meeting
		Email:              payload.Email,
		FirstName:          utils.StringValue(payload.FirstName),
		LastName:           utils.StringValue(payload.LastName),
		Host:               utils.BoolValue(payload.Host),
		JobTitle:           utils.StringValue(payload.JobTitle),
		OrgName:            utils.StringValue(payload.OrgName),
		OrgIsMember:        false, // This will get populated by the service
		OrgIsProjectMember: false, // This will get populated by the service
		AvatarURL:          utils.StringValue(payload.AvatarURL),
		Username:           utils.StringValue(payload.Username),
		IsInvited:          utils.BoolValue(payload.IsInvited),
		IsAttended:         utils.BoolValue(payload.IsAttended),
		CreatedAt:          &now,
		UpdatedAt:          &now,
	}

	return participant
}

// ConvertUpdatePastMeetingParticipantPayloadToDomain converts an UpdatePastMeetingParticipantPayload to a domain PastMeetingParticipant model
func ConvertUpdatePastMeetingParticipantPayloadToDomain(payload *meetingservice.UpdatePastMeetingParticipantPayload) *models.PastMeetingParticipant {
	now := time.Now().UTC()
	participant := &models.PastMeetingParticipant{
		UID:                utils.StringValue(payload.UID),
		PastMeetingUID:     payload.PastMeetingUID,
		MeetingUID:         "", // This will get populated by the service
		Email:              payload.Email,
		FirstName:          utils.StringValue(payload.FirstName),
		LastName:           utils.StringValue(payload.LastName),
		Host:               utils.BoolValue(payload.Host),
		JobTitle:           utils.StringValue(payload.JobTitle),
		OrgName:            utils.StringValue(payload.OrgName),
		OrgIsMember:        false, // This will get populated by the service
		OrgIsProjectMember: false, // This will get populated by the service
		AvatarURL:          utils.StringValue(payload.AvatarURL),
		Username:           utils.StringValue(payload.Username),
		IsInvited:          utils.BoolValue(payload.IsInvited),
		IsAttended:         utils.BoolValue(payload.IsAttended),
		CreatedAt:          nil, // This will get populated by the service
		UpdatedAt:          &now,
	}

	return participant
}

// ConvertCreatePastMeetingPayloadToDomain converts a CreatePastMeetingPayload type to the domain PastMeeting model
func ConvertCreatePastMeetingPayloadToDomain(payload *meetingservice.CreatePastMeetingPayload) *models.PastMeeting {
	scheduledStartTime, err := time.Parse(time.RFC3339, payload.ScheduledStartTime)
	if err != nil {
		slog.Error("failed to parse scheduled start time", logging.ErrKey, err,
			"scheduled_start_time", payload.ScheduledStartTime,
		)
		return nil
	}

	scheduledEndTime, err := time.Parse(time.RFC3339, payload.ScheduledEndTime)
	if err != nil {
		slog.Error("failed to parse scheduled end time", logging.ErrKey, err,
			"scheduled_end_time", payload.ScheduledEndTime,
		)
		return nil
	}

	now := time.Now().UTC()
	pastMeeting := &models.PastMeeting{
		UID:                  "", // This will get populated by the service
		MeetingUID:           payload.MeetingUID,
		OccurrenceID:         utils.StringValue(payload.OccurrenceID),
		ProjectUID:           payload.ProjectUID,
		ScheduledStartTime:   scheduledStartTime,
		ScheduledEndTime:     scheduledEndTime,
		Duration:             payload.Duration,
		Timezone:             payload.Timezone,
		Recurrence:           convertRecurrenceToDomain(payload.Recurrence),
		Title:                payload.Title,
		Description:          payload.Description,
		Committees:           convertCommitteesToDomain(payload.Committees),
		Platform:             payload.Platform,
		PlatformMeetingID:    utils.StringValue(payload.PlatformMeetingID),
		EarlyJoinTimeMinutes: utils.IntValue(payload.EarlyJoinTimeMinutes),
		MeetingType:          utils.StringValue(payload.MeetingType),
		Visibility:           utils.StringValue(payload.Visibility),
		Restricted:           utils.BoolValue(payload.Restricted),
		ArtifactVisibility:   utils.StringValue(payload.ArtifactVisibility),
		PublicLink:           utils.StringValue(payload.PublicLink),
		RecordingEnabled:     utils.BoolValue(payload.RecordingEnabled),
		TranscriptEnabled:    utils.BoolValue(payload.TranscriptEnabled),
		YoutubeUploadEnabled: utils.BoolValue(payload.YoutubeUploadEnabled),
		ZoomConfig:           convertZoomConfigFullToDomain(payload.ZoomConfig),
		Sessions:             convertSessionsFullToDomain(payload.Sessions),
		CreatedAt:            &now,
		UpdatedAt:            &now,
	}

	return pastMeeting
}

// convertZoomConfigFullToDomain converts a ZoomConfigFull to domain ZoomConfig
func convertZoomConfigFullToDomain(z *meetingservice.ZoomConfigFull) *models.ZoomConfig {
	if z == nil {
		return nil
	}

	return &models.ZoomConfig{
		MeetingID:                utils.StringValue(z.MeetingID),
		Passcode:                 utils.StringValue(z.Passcode),
		AICompanionEnabled:       utils.BoolValue(z.AiCompanionEnabled),
		AISummaryRequireApproval: utils.BoolValue(z.AiSummaryRequireApproval),
	}
}

// convertSessionsFullToDomain converts Sessions to domain Sessions
func convertSessionsFullToDomain(sessions []*meetingservice.Session) []models.Session {
	if sessions == nil {
		return nil
	}

	var result []models.Session
	for _, session := range sessions {
		if session == nil {
			continue
		}

		startTime, err := time.Parse(time.RFC3339, session.StartTime)
		if err != nil {
			slog.Error("failed to parse session start time", logging.ErrKey, err,
				"start_time", session.StartTime,
			)
			continue
		}

		domainSession := models.Session{
			UID:       session.UID,
			StartTime: startTime,
		}

		if session.EndTime != nil {
			endTime, err := time.Parse(time.RFC3339, *session.EndTime)
			if err != nil {
				slog.Error("failed to parse session end time", logging.ErrKey, err,
					"end_time", *session.EndTime,
				)
			} else {
				domainSession.EndTime = &endTime
			}
		}

		result = append(result, domainSession)
	}

	return result
}

// ConvertUpdatePastMeetingSummaryPayloadToDomain converts an update past meeting summary payload to domain model.
func ConvertUpdatePastMeetingSummaryPayloadToDomain(payload *meetingservice.UpdatePastMeetingSummaryPayload) *models.PastMeetingSummary {
	if payload == nil {
		return nil
	}

	summary := &models.PastMeetingSummary{
		UID:            payload.SummaryUID,
		PastMeetingUID: payload.PastMeetingUID,
	}

	// Set SummaryData with only the editable fields
	summaryData := models.SummaryData{}

	if payload.EditedOverview != nil {
		summaryData.EditedOverview = *payload.EditedOverview
	}

	if payload.EditedDetails != nil {
		editedDetails := make([]models.SummaryDetail, len(payload.EditedDetails))
		for i, detail := range payload.EditedDetails {
			editedDetails[i] = models.SummaryDetail{
				Label:   detail.Label,
				Summary: detail.Summary,
			}
		}
		summaryData.EditedDetails = editedDetails
	}

	if payload.EditedNextSteps != nil {
		summaryData.EditedNextSteps = payload.EditedNextSteps
	}

	summary.SummaryData = summaryData

	if payload.Approved != nil {
		summary.Approved = *payload.Approved
	}

	return summary
}
