// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"log/slog"
	"time"

	meetingservice "github.com/linuxfoundation/lfx-v2-meeting-service/gen/meeting_service"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/utils"
)

// ConvertCreateMeetingPayloadToDomain converts a Goa CreateMeetingPayload to domain model
func ConvertCreateMeetingPayloadToDomain(payload *meetingservice.CreateMeetingPayload) *models.MeetingFull {
	// Convert payload to domain - split into Base and Settings
	base := convertCreateMeetingBasePayloadToDomain(payload)
	settings := convertCreateMeetingSettingsPayloadToDomain(payload)

	request := &models.MeetingFull{
		Base:     base,
		Settings: settings,
	}

	return request
}

func convertCreateMeetingBasePayloadToDomain(payload *meetingservice.CreateMeetingPayload) *models.MeetingBase {
	if payload == nil {
		return nil
	}

	startTime, err := time.Parse(time.RFC3339, payload.StartTime)
	if err != nil {
		slog.Error("failed to parse start time", logging.ErrKey, err,
			"start_time", payload.StartTime,
		)
		return nil
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

	return meeting
}

func convertCreateMeetingSettingsPayloadToDomain(payload *meetingservice.CreateMeetingPayload) *models.MeetingSettings {
	if payload == nil {
		return nil
	}

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
func ConvertMeetingUpdatePayloadToDomain(payload *meetingservice.UpdateMeetingBasePayload) *models.MeetingBase {
	if payload == nil {
		return nil
	}

	startTime, err := time.Parse(time.RFC3339, payload.StartTime)
	if err != nil {
		slog.Error("failed to parse start time", logging.ErrKey, err,
			"start_time", payload.StartTime,
		)
		return nil
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

	return meeting
}

// ConvertUpdateSettingsPayloadToDomain converts a Goa UpdateMeetingSettingsPayload to domain model
func ConvertUpdateSettingsPayloadToDomain(payload *meetingservice.UpdateMeetingSettingsPayload) *models.MeetingSettings {
	if payload == nil {
		return nil
	}

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
	if goaRegistrant == nil {
		return nil
	}

	now := time.Now().UTC()
	registrant := &models.Registrant{
		UID:                "", // This will get populated by the service
		MeetingUID:         goaRegistrant.MeetingUID,
		Email:              goaRegistrant.Email,
		FirstName:          goaRegistrant.FirstName,
		LastName:           goaRegistrant.LastName,
		Host:               utils.BoolValue(goaRegistrant.Host),
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
	if payload == nil {
		return nil
	}

	now := time.Now().UTC()
	registrant := &models.Registrant{
		UID:                *payload.UID,
		MeetingUID:         payload.MeetingUID,
		Email:              payload.Email,
		FirstName:          payload.FirstName,
		LastName:           payload.LastName,
		Host:               utils.BoolValue(payload.Host),
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
	if payload == nil {
		return nil
	}

	now := time.Now().UTC()
	participant := &models.PastMeetingParticipant{
		UID:                "", // This will get populated by the service
		PastMeetingUID:     utils.StringValue(payload.UID),
		MeetingUID:         "", // This will need to be populated by the service from the past meeting
		Email:              payload.Email,
		FirstName:          payload.FirstName,
		LastName:           payload.LastName,
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
	if payload == nil {
		return nil
	}

	now := time.Now().UTC()
	participant := &models.PastMeetingParticipant{
		UID:                utils.StringValue(payload.UID),
		PastMeetingUID:     utils.StringValue(payload.PastMeetingUID),
		MeetingUID:         "", // This will get populated by the service
		Email:              payload.Email,
		FirstName:          payload.FirstName,
		LastName:           payload.LastName,
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
