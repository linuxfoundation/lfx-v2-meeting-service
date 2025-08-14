// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package models

import (
	"log/slog"
	"time"

	meetingservice "github.com/linuxfoundation/lfx-v2-meeting-service/gen/meeting_service"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/utils"
)

func ToMeetingFullServiceModel(meetingBase *MeetingBase, meetingSettings *MeetingSettings) *meetingservice.MeetingFull {
	if meetingBase == nil {
		return nil
	}

	meetingFull := &meetingservice.MeetingFull{
		UID:         utils.StringPtr(meetingBase.UID),
		ProjectUID:  utils.StringPtr(meetingBase.ProjectUID),
		StartTime:   utils.StringPtr(meetingBase.StartTime.Format(time.RFC3339)),
		Duration:    utils.IntPtr(meetingBase.Duration),
		Timezone:    utils.StringPtr(meetingBase.Timezone),
		Recurrence:  fromDBRecurrence(meetingBase.Recurrence),
		Title:       utils.StringPtr(meetingBase.Title),
		Description: utils.StringPtr(meetingBase.Description),
	}

	// Only set string fields if they're not empty
	if meetingBase.Platform != "" {
		meetingFull.Platform = utils.StringPtr(meetingBase.Platform)
	}
	if meetingBase.MeetingType != "" {
		meetingFull.MeetingType = utils.StringPtr(meetingBase.MeetingType)
	}
	if meetingBase.Visibility != "" {
		meetingFull.Visibility = utils.StringPtr(meetingBase.Visibility)
	}
	if meetingBase.ArtifactVisibility != "" {
		meetingFull.ArtifactVisibility = utils.StringPtr(meetingBase.ArtifactVisibility)
	}
	if meetingBase.PublicLink != "" {
		meetingFull.PublicLink = utils.StringPtr(meetingBase.PublicLink)
	}
	if meetingBase.EmailDeliveryErrorCount != 0 {
		meetingFull.EmailDeliveryErrorCount = utils.IntPtr(meetingBase.EmailDeliveryErrorCount)
	}
	if meetingBase.RecordingEnabled {
		meetingFull.RecordingEnabled = utils.BoolPtr(meetingBase.RecordingEnabled)
	}
	if meetingBase.TranscriptEnabled {
		meetingFull.TranscriptEnabled = utils.BoolPtr(meetingBase.TranscriptEnabled)
	}
	if meetingBase.YoutubeUploadEnabled {
		meetingFull.YoutubeUploadEnabled = utils.BoolPtr(meetingBase.YoutubeUploadEnabled)
	}
	if meetingBase.ZoomConfig != nil {
		meetingFull.ZoomConfig = fromDBZoomConfig(meetingBase.ZoomConfig)
	}
	if meetingBase.RegistrantCount != 0 {
		meetingFull.RegistrantCount = utils.IntPtr(meetingBase.RegistrantCount)
	}
	if meetingBase.RegistrantResponseDeclinedCount != 0 {
		meetingFull.RegistrantResponseDeclinedCount = utils.IntPtr(meetingBase.RegistrantResponseDeclinedCount)
	}
	if meetingBase.RegistrantResponseAcceptedCount != 0 {
		meetingFull.RegistrantResponseAcceptedCount = utils.IntPtr(meetingBase.RegistrantResponseAcceptedCount)
	}
	if len(meetingBase.Occurrences) > 0 {
		meetingFull.Occurrences = make([]*meetingservice.Occurrence, 0, len(meetingBase.Occurrences))
		for _, o := range meetingBase.Occurrences {
			meetingFull.Occurrences = append(meetingFull.Occurrences, fromDBOccurrence(&o))
		}
	}

	// Convert timestamps
	if meetingBase.CreatedAt != nil {
		meetingFull.CreatedAt = utils.StringPtr(meetingBase.CreatedAt.Format(time.RFC3339))
	}
	if meetingBase.UpdatedAt != nil {
		meetingFull.UpdatedAt = utils.StringPtr(meetingBase.UpdatedAt.Format(time.RFC3339))
	}

	if meetingSettings != nil {
		meetingFull.Organizers = meetingSettings.Organizers
	}

	return meetingFull
}

// ToMeetingBaseDBModel converts a Goa MeetingBase type to the domain Meeting model for database storage
func ToMeetingBaseDBModel(goaMeeting *meetingservice.MeetingBase) *MeetingBase {
	if goaMeeting == nil {
		return nil
	}

	meeting := &MeetingBase{
		UID:                             utils.StringValue(goaMeeting.UID),
		ProjectUID:                      utils.StringValue(goaMeeting.ProjectUID),
		Title:                           utils.StringValue(goaMeeting.Title),
		Description:                     utils.StringValue(goaMeeting.Description),
		Timezone:                        utils.StringValue(goaMeeting.Timezone),
		Platform:                        utils.StringValue(goaMeeting.Platform),
		Duration:                        utils.IntValue(goaMeeting.Duration),
		EarlyJoinTimeMinutes:            utils.IntValue(goaMeeting.EarlyJoinTimeMinutes),
		MeetingType:                     utils.StringValue(goaMeeting.MeetingType),
		Visibility:                      utils.StringValue(goaMeeting.Visibility),
		Restricted:                      utils.BoolValue(goaMeeting.Restricted),
		ArtifactVisibility:              utils.StringValue(goaMeeting.ArtifactVisibility),
		PublicLink:                      utils.StringValue(goaMeeting.PublicLink),
		EmailDeliveryErrorCount:         utils.IntValue(goaMeeting.EmailDeliveryErrorCount),
		RecordingEnabled:                utils.BoolValue(goaMeeting.RecordingEnabled),
		TranscriptEnabled:               utils.BoolValue(goaMeeting.TranscriptEnabled),
		YoutubeUploadEnabled:            utils.BoolValue(goaMeeting.YoutubeUploadEnabled),
		RegistrantCount:                 utils.IntValue(goaMeeting.RegistrantCount),
		RegistrantResponseDeclinedCount: utils.IntValue(goaMeeting.RegistrantResponseDeclinedCount),
		RegistrantResponseAcceptedCount: utils.IntValue(goaMeeting.RegistrantResponseAcceptedCount),
	}

	// Convert StartTime
	if goaMeeting.StartTime != nil {
		startTime, err := time.Parse(time.RFC3339, *goaMeeting.StartTime)
		if err == nil {
			meeting.StartTime = startTime
		}
	}

	// Convert timestamps
	if goaMeeting.CreatedAt != nil {
		createdAt, err := time.Parse(time.RFC3339, *goaMeeting.CreatedAt)
		if err == nil {
			meeting.CreatedAt = &createdAt
		}
	}

	if goaMeeting.UpdatedAt != nil {
		updatedAt, err := time.Parse(time.RFC3339, *goaMeeting.UpdatedAt)
		if err == nil {
			meeting.UpdatedAt = &updatedAt
		}
	}

	currentTime := time.Now()
	if goaMeeting.CreatedAt != nil {
		createdAt, err := time.Parse(time.RFC3339, *goaMeeting.CreatedAt)
		if err == nil {
			meeting.CreatedAt = &createdAt
		}
	} else {
		meeting.CreatedAt = &currentTime
	}
	if goaMeeting.UpdatedAt != nil {
		updatedAt, err := time.Parse(time.RFC3339, *goaMeeting.UpdatedAt)
		if err == nil {
			meeting.UpdatedAt = &updatedAt
		}
	} else {
		meeting.UpdatedAt = &currentTime
	}

	// Convert Recurrence
	if goaMeeting.Recurrence != nil {
		meeting.Recurrence = toDBRecurrence(goaMeeting.Recurrence)
	}

	// Convert Committees
	if len(goaMeeting.Committees) > 0 {
		meeting.Committees = make([]Committee, 0, len(goaMeeting.Committees))
		for _, c := range goaMeeting.Committees {
			if c != nil {
				meeting.Committees = append(meeting.Committees, toDBCommittee(c))
			}
		}
	}

	// Convert ZoomConfig
	if goaMeeting.ZoomConfig != nil {
		meeting.ZoomConfig = toDBZoomConfig(goaMeeting.ZoomConfig)
	}

	// Convert Occurrences
	if len(goaMeeting.Occurrences) > 0 {
		meeting.Occurrences = make([]Occurrence, 0, len(goaMeeting.Occurrences))
		for _, o := range goaMeeting.Occurrences {
			if o != nil {
				meeting.Occurrences = append(meeting.Occurrences, toDBOccurrence(o))
			}
		}
	}

	return meeting
}

// FromMeetingBaseDBModel converts a domain Meeting model to a Goa Meeting type for API responses
func FromMeetingBaseDBModel(meeting *MeetingBase) *meetingservice.MeetingBase {
	if meeting == nil {
		return nil
	}

	goaMeeting := &meetingservice.MeetingBase{
		UID:                             utils.StringPtr(meeting.UID),
		ProjectUID:                      utils.StringPtr(meeting.ProjectUID),
		StartTime:                       utils.StringPtr(meeting.StartTime.Format(time.RFC3339)),
		Duration:                        utils.IntPtr(meeting.Duration),
		Timezone:                        utils.StringPtr(meeting.Timezone),
		Title:                           utils.StringPtr(meeting.Title),
		Description:                     utils.StringPtr(meeting.Description),
		Platform:                        utils.StringPtr(meeting.Platform),
		EarlyJoinTimeMinutes:            utils.IntPtr(meeting.EarlyJoinTimeMinutes),
		MeetingType:                     utils.StringPtr(meeting.MeetingType),
		Visibility:                      utils.StringPtr(meeting.Visibility),
		Restricted:                      utils.BoolPtr(meeting.Restricted),
		ArtifactVisibility:              utils.StringPtr(meeting.ArtifactVisibility),
		PublicLink:                      utils.StringPtr(meeting.PublicLink),
		EmailDeliveryErrorCount:         utils.IntPtr(meeting.EmailDeliveryErrorCount),
		RecordingEnabled:                utils.BoolPtr(meeting.RecordingEnabled),
		TranscriptEnabled:               utils.BoolPtr(meeting.TranscriptEnabled),
		YoutubeUploadEnabled:            utils.BoolPtr(meeting.YoutubeUploadEnabled),
		RegistrantCount:                 utils.IntPtr(meeting.RegistrantCount),
		RegistrantResponseDeclinedCount: utils.IntPtr(meeting.RegistrantResponseDeclinedCount),
		RegistrantResponseAcceptedCount: utils.IntPtr(meeting.RegistrantResponseAcceptedCount),
	}

	// Convert timestamps
	if meeting.CreatedAt != nil {
		goaMeeting.CreatedAt = utils.StringPtr(meeting.CreatedAt.Format(time.RFC3339))
	}
	if meeting.UpdatedAt != nil {
		goaMeeting.UpdatedAt = utils.StringPtr(meeting.UpdatedAt.Format(time.RFC3339))
	}

	// Convert Recurrence
	if meeting.Recurrence != nil {
		goaMeeting.Recurrence = fromDBRecurrence(meeting.Recurrence)
	}

	// Convert Committees
	if len(meeting.Committees) > 0 {
		goaMeeting.Committees = make([]*meetingservice.Committee, 0, len(meeting.Committees))
		for _, c := range meeting.Committees {
			goaMeeting.Committees = append(goaMeeting.Committees, fromDBCommittee(&c))
		}
	}

	// Convert ZoomConfig
	if meeting.ZoomConfig != nil {
		goaMeeting.ZoomConfig = fromDBZoomConfig(meeting.ZoomConfig)
	}

	// Convert Occurrences
	if len(meeting.Occurrences) > 0 {
		goaMeeting.Occurrences = make([]*meetingservice.Occurrence, 0, len(meeting.Occurrences))
		for _, o := range meeting.Occurrences {
			goaMeeting.Occurrences = append(goaMeeting.Occurrences, fromDBOccurrence(&o))
		}
	}

	return goaMeeting
}

// ToMeetingDBModelFromCreatePayload converts a Goa CreateMeetingPayload to a domain Meeting model
func ToMeetingDBModelFromCreatePayload(payload *meetingservice.CreateMeetingPayload) *MeetingBase {
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
	meeting := &MeetingBase{
		ProjectUID:           payload.ProjectUID,
		StartTime:            startTime,
		Duration:             payload.Duration,
		Timezone:             payload.Timezone,
		Recurrence:           toDBRecurrence(payload.Recurrence),
		Title:                payload.Title,
		Description:          payload.Description,
		Committees:           toDBCommittees(payload.Committees),
		Platform:             utils.StringValue(payload.Platform),
		EarlyJoinTimeMinutes: utils.IntValue(payload.EarlyJoinTimeMinutes),
		MeetingType:          utils.StringValue(payload.MeetingType),
		Visibility:           utils.StringValue(payload.Visibility),
		Restricted:           utils.BoolValue(payload.Restricted),
		ArtifactVisibility:   utils.StringValue(payload.ArtifactVisibility),
		PublicLink:           utils.StringValue(payload.PublicLink),
		RecordingEnabled:     utils.BoolValue(payload.RecordingEnabled),
		TranscriptEnabled:    utils.BoolValue(payload.TranscriptEnabled),
		YoutubeUploadEnabled: utils.BoolValue(payload.YoutubeUploadEnabled),
		ZoomConfig:           toDBZoomConfigFromPost(payload.ZoomConfig),
		CreatedAt:            &now,
		UpdatedAt:            &now,
	}

	return meeting
}

func ToMeetingBaseDBModelFromUpdatePayload(payload *meetingservice.UpdateMeetingBasePayload, existingMeeting *MeetingBase) *MeetingBase {
	if payload == nil || existingMeeting == nil {
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
	meeting := &MeetingBase{
		UID:                  existingMeeting.UID,
		ProjectUID:           payload.ProjectUID,
		StartTime:            startTime,
		Duration:             payload.Duration,
		Timezone:             payload.Timezone,
		Recurrence:           toDBRecurrence(payload.Recurrence),
		Title:                payload.Title,
		Description:          payload.Description,
		Committees:           toDBCommittees(payload.Committees),
		Platform:             utils.StringValue(payload.Platform),
		EarlyJoinTimeMinutes: utils.IntValue(payload.EarlyJoinTimeMinutes),
		MeetingType:          utils.StringValue(payload.MeetingType),
		Visibility:           utils.StringValue(payload.Visibility),
		Restricted:           utils.BoolValue(payload.Restricted),
		ArtifactVisibility:   utils.StringValue(payload.ArtifactVisibility),
		PublicLink:           utils.StringValue(payload.PublicLink),
		RecordingEnabled:     utils.BoolValue(payload.RecordingEnabled),
		TranscriptEnabled:    utils.BoolValue(payload.TranscriptEnabled),
		YoutubeUploadEnabled: utils.BoolValue(payload.YoutubeUploadEnabled),
		ZoomConfig:           mergeZoomConfig(existingMeeting.ZoomConfig, payload.ZoomConfig),
		CreatedAt:            existingMeeting.CreatedAt,
		UpdatedAt:            &now,
	}

	return meeting
}

// Helper functions for nested types

func toDBZoomConfigFromPost(z *meetingservice.ZoomConfigPost) *ZoomConfig {
	if z == nil {
		return nil
	}

	return &ZoomConfig{
		MeetingID:                "", // TODO: replace with actual zoom meeting ID once we have zoom integration
		AICompanionEnabled:       utils.BoolValue(z.AiCompanionEnabled),
		AISummaryRequireApproval: utils.BoolValue(z.AiSummaryRequireApproval),
	}
}

// mergeZoomConfig merges the existing ZoomConfig with updates from the payload,
// preserving the MeetingID from the existing config
func mergeZoomConfig(existing *ZoomConfig, payload *meetingservice.ZoomConfigPost) *ZoomConfig {
	// If there's no existing config and no payload, return nil
	if existing == nil && payload == nil {
		return nil
	}

	// If there's no existing config but there's a payload, create new config
	if existing == nil {
		return toDBZoomConfigFromPost(payload)
	}

	// If there's existing config but no payload, preserve existing config
	if payload == nil {
		return existing
	}

	// Merge existing config with payload updates, preserving MeetingID
	return &ZoomConfig{
		MeetingID:                existing.MeetingID, // Preserve existing MeetingID
		AICompanionEnabled:       utils.BoolValue(payload.AiCompanionEnabled),
		AISummaryRequireApproval: utils.BoolValue(payload.AiSummaryRequireApproval),
	}
}

func toDBCommittees(committees []*meetingservice.Committee) []Committee {
	dbCommittees := make([]Committee, 0, len(committees))
	for _, c := range committees {
		if c != nil {
			dbCommittees = append(dbCommittees, toDBCommittee(c))
		}
	}
	return dbCommittees
}

func toDBCommittee(c *meetingservice.Committee) Committee {
	if c == nil {
		return Committee{}
	}

	return Committee{
		UID:                   c.UID,
		AllowedVotingStatuses: c.AllowedVotingStatuses,
	}
}

func fromDBCommittee(c *Committee) *meetingservice.Committee {
	if c == nil {
		return nil
	}

	return &meetingservice.Committee{
		UID:                   c.UID,
		AllowedVotingStatuses: c.AllowedVotingStatuses,
	}
}

func toDBRecurrence(r *meetingservice.Recurrence) *Recurrence {
	if r == nil {
		return nil
	}

	recurrence := &Recurrence{
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

func fromDBRecurrence(r *Recurrence) *meetingservice.Recurrence {
	if r == nil {
		return nil
	}

	rec := &meetingservice.Recurrence{
		Type:           r.Type,
		RepeatInterval: r.RepeatInterval,
	}

	if r.WeeklyDays != "" {
		rec.WeeklyDays = utils.StringPtr(r.WeeklyDays)
	}
	if r.MonthlyDay != 0 {
		rec.MonthlyDay = utils.IntPtr(r.MonthlyDay)
	}
	if r.MonthlyWeek != 0 {
		rec.MonthlyWeek = utils.IntPtr(r.MonthlyWeek)
	}
	if r.MonthlyWeekDay != 0 {
		rec.MonthlyWeekDay = utils.IntPtr(r.MonthlyWeekDay)
	}
	if r.EndTimes != 0 {
		rec.EndTimes = utils.IntPtr(r.EndTimes)
	}
	if r.EndDateTime != nil {
		rec.EndDateTime = utils.StringPtr(r.EndDateTime.Format(time.RFC3339))
	}

	return rec
}

func toDBZoomConfig(z *meetingservice.ZoomConfigFull) *ZoomConfig {
	if z == nil {
		return nil
	}

	return &ZoomConfig{
		MeetingID:                utils.StringValue(z.MeetingID),
		AICompanionEnabled:       utils.BoolValue(z.AiCompanionEnabled),
		AISummaryRequireApproval: utils.BoolValue(z.AiSummaryRequireApproval),
	}
}

func fromDBZoomConfig(z *ZoomConfig) *meetingservice.ZoomConfigFull {
	if z == nil {
		return nil
	}

	zc := &meetingservice.ZoomConfigFull{
		AiCompanionEnabled:       utils.BoolPtr(z.AICompanionEnabled),
		AiSummaryRequireApproval: utils.BoolPtr(z.AISummaryRequireApproval),
	}

	if z.MeetingID != "" {
		zc.MeetingID = utils.StringPtr(z.MeetingID)
	}

	return zc
}

func toDBOccurrence(o *meetingservice.Occurrence) Occurrence {
	if o == nil {
		return Occurrence{}
	}

	occ := Occurrence{
		OccurrenceID:     utils.StringValue(o.OccurrenceID),
		Title:            utils.StringValue(o.Title),
		Description:      utils.StringValue(o.Description),
		Duration:         utils.IntValue(o.Duration),
		RegistrantCount:  utils.IntValue(o.RegistrantCount),
		ResponseCountNo:  utils.IntValue(o.ResponseCountNo),
		ResponseCountYes: utils.IntValue(o.ResponseCountYes),
		Status:           utils.StringValue(o.Status),
	}

	// Convert StartTime
	if o.StartTime != nil {
		startTime, err := time.Parse(time.RFC3339, *o.StartTime)
		if err == nil {
			occ.StartTime = &startTime
		}
	}

	if o.Recurrence != nil {
		occ.Recurrence = toDBRecurrence(o.Recurrence)
	}

	return occ
}

func fromDBOccurrence(o *Occurrence) *meetingservice.Occurrence {
	if o == nil {
		return nil
	}

	occ := &meetingservice.Occurrence{}

	if o.OccurrenceID != "" {
		occ.OccurrenceID = utils.StringPtr(o.OccurrenceID)
	}
	if o.StartTime != nil {
		occ.StartTime = utils.StringPtr(o.StartTime.Format(time.RFC3339))
	}
	if o.Title != "" {
		occ.Title = utils.StringPtr(o.Title)
	}
	if o.Description != "" {
		occ.Description = utils.StringPtr(o.Description)
	}
	if o.Duration != 0 {
		occ.Duration = utils.IntPtr(o.Duration)
	}
	if o.RegistrantCount != 0 {
		occ.RegistrantCount = utils.IntPtr(o.RegistrantCount)
	}
	if o.ResponseCountNo != 0 {
		occ.ResponseCountNo = utils.IntPtr(o.ResponseCountNo)
	}
	if o.ResponseCountYes != 0 {
		occ.ResponseCountYes = utils.IntPtr(o.ResponseCountYes)
	}
	if o.Status != "" {
		occ.Status = utils.StringPtr(o.Status)
	}

	if o.Recurrence != nil {
		occ.Recurrence = fromDBRecurrence(o.Recurrence)
	}

	return occ
}

// ToMeetingSettingsServiceModel converts a domain MeetingSettings to service model
func ToMeetingSettingsServiceModel(settings *MeetingSettings) *meetingservice.MeetingSettings {
	if settings == nil {
		return nil
	}

	result := &meetingservice.MeetingSettings{
		UID:        utils.StringPtr(settings.UID),
		Organizers: settings.Organizers,
	}

	if settings.CreatedAt != nil {
		result.CreatedAt = utils.StringPtr(settings.CreatedAt.Format(time.RFC3339))
	}
	if settings.UpdatedAt != nil {
		result.UpdatedAt = utils.StringPtr(settings.UpdatedAt.Format(time.RFC3339))
	}

	return result
}

// FromMeetingSettingsServiceModel converts a service MeetingSettings to domain model
func FromMeetingSettingsServiceModel(settings *meetingservice.MeetingSettings) *MeetingSettings {
	if settings == nil {
		return nil
	}

	result := &MeetingSettings{
		UID:        utils.StringValue(settings.UID),
		Organizers: settings.Organizers,
	}

	if settings.CreatedAt != nil {
		createdAt, err := time.Parse(time.RFC3339, *settings.CreatedAt)
		if err == nil {
			result.CreatedAt = &createdAt
		} else {
			slog.Warn("failed to parse created_at", logging.ErrKey, err)
		}
	}

	if settings.UpdatedAt != nil {
		updatedAt, err := time.Parse(time.RFC3339, *settings.UpdatedAt)
		if err == nil {
			result.UpdatedAt = &updatedAt
		} else {
			slog.Warn("failed to parse updated_at", logging.ErrKey, err)
		}
	}

	return result
}
