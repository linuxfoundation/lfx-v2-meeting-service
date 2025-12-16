// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"time"

	meetingservice "github.com/linuxfoundation/lfx-v2-meeting-service/gen/meeting_service"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/utils"
)

// ConvertDomainToFullResponse converts a domain MeetingFull to service model
func ConvertDomainToFullResponse(meetingFull *models.MeetingFull) *meetingservice.MeetingFull {
	if meetingFull == nil {
		return nil
	}
	return convertDomainToFullResponseSplit(meetingFull.Base, meetingFull.Settings)
}

func convertDomainToFullResponseSplit(meetingBase *models.MeetingBase, meetingSettings *models.MeetingSettings) *meetingservice.MeetingFull {
	if meetingBase == nil {
		return nil
	}

	meetingFull := &meetingservice.MeetingFull{
		UID:         utils.StringPtr(meetingBase.UID),
		ProjectUID:  utils.StringPtr(meetingBase.ProjectUID),
		StartTime:   utils.StringPtr(meetingBase.StartTime.Format(time.RFC3339)),
		Duration:    utils.IntPtr(meetingBase.Duration),
		Timezone:    utils.StringPtr(meetingBase.Timezone),
		Recurrence:  convertDomainToRecurrenceResponse(meetingBase.Recurrence),
		Title:       utils.StringPtr(meetingBase.Title),
		Description: utils.StringPtr(meetingBase.Description),
		Restricted:  utils.BoolPtr(meetingBase.Restricted),
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
	if meetingBase.Password != "" {
		meetingFull.Password = utils.StringPtr(meetingBase.Password)
	}
	if meetingBase.EarlyJoinTimeMinutes != 0 {
		meetingFull.EarlyJoinTimeMinutes = utils.IntPtr(meetingBase.EarlyJoinTimeMinutes)
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
		meetingFull.ZoomConfig = convertDomainToZoomConfigResponse(meetingBase.ZoomConfig)
	}
	if meetingBase.RegistrantCount != 0 {
		meetingFull.RegistrantCount = utils.IntPtr(meetingBase.RegistrantCount)
	}
	if len(meetingBase.Occurrences) > 0 {
		meetingFull.Occurrences = make([]*meetingservice.Occurrence, 0, len(meetingBase.Occurrences))
		for _, o := range meetingBase.Occurrences {
			meetingFull.Occurrences = append(meetingFull.Occurrences, convertDomainToOccurrenceResponse(&o))
		}
	}
	if len(meetingBase.Committees) > 0 {
		meetingFull.Committees = make([]*meetingservice.Committee, 0, len(meetingBase.Committees))
		for _, c := range meetingBase.Committees {
			meetingFull.Committees = append(meetingFull.Committees, convertDomainToCommitteeResponse(&c))
		}
	}

	// Convert timestamps
	if meetingBase.CreatedAt != nil {
		meetingFull.CreatedAt = utils.StringPtr(meetingBase.CreatedAt.Format(time.RFC3339))
	}
	if meetingBase.UpdatedAt != nil {
		meetingFull.UpdatedAt = utils.StringPtr(meetingBase.UpdatedAt.Format(time.RFC3339))
	}

	// Convert SeriesEndDate
	if meetingBase.SeriesEndDate != nil {
		meetingFull.SeriesEndDate = utils.StringPtr(meetingBase.SeriesEndDate.Format(time.RFC3339))
	}

	if meetingSettings != nil {
		meetingFull.Organizers = meetingSettings.Organizers
	}

	return meetingFull
}

// ConvertDomainToBaseResponse converts a domain Meeting model to a Goa Meeting type for API responses
func ConvertDomainToBaseResponse(meeting *models.MeetingBase) *meetingservice.MeetingBase {
	if meeting == nil {
		return nil
	}

	goaMeeting := &meetingservice.MeetingBase{
		UID:                     utils.StringPtr(meeting.UID),
		ProjectUID:              utils.StringPtr(meeting.ProjectUID),
		StartTime:               utils.StringPtr(meeting.StartTime.Format(time.RFC3339)),
		Duration:                utils.IntPtr(meeting.Duration),
		Timezone:                utils.StringPtr(meeting.Timezone),
		Title:                   utils.StringPtr(meeting.Title),
		Description:             utils.StringPtr(meeting.Description),
		Platform:                utils.StringPtr(meeting.Platform),
		EarlyJoinTimeMinutes:    utils.IntPtr(meeting.EarlyJoinTimeMinutes),
		MeetingType:             utils.StringPtr(meeting.MeetingType),
		Visibility:              utils.StringPtr(meeting.Visibility),
		Restricted:              utils.BoolPtr(meeting.Restricted),
		ArtifactVisibility:      utils.StringPtr(meeting.ArtifactVisibility),
		PublicLink:              utils.StringPtr(meeting.PublicLink),
		Password:                utils.StringPtr(meeting.Password),
		EmailDeliveryErrorCount: utils.IntPtr(meeting.EmailDeliveryErrorCount),
		RecordingEnabled:        utils.BoolPtr(meeting.RecordingEnabled),
		TranscriptEnabled:       utils.BoolPtr(meeting.TranscriptEnabled),
		YoutubeUploadEnabled:    utils.BoolPtr(meeting.YoutubeUploadEnabled),
		RegistrantCount:         utils.IntPtr(meeting.RegistrantCount),
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
		goaMeeting.Recurrence = convertDomainToRecurrenceResponse(meeting.Recurrence)
	}

	// Convert SeriesEndDate
	if meeting.SeriesEndDate != nil {
		goaMeeting.SeriesEndDate = utils.StringPtr(meeting.SeriesEndDate.Format(time.RFC3339))
	}

	// Convert Committees
	if len(meeting.Committees) > 0 {
		goaMeeting.Committees = make([]*meetingservice.Committee, 0, len(meeting.Committees))
		for _, c := range meeting.Committees {
			goaMeeting.Committees = append(goaMeeting.Committees, convertDomainToCommitteeResponse(&c))
		}
	}

	// Convert ZoomConfig
	if meeting.ZoomConfig != nil {
		goaMeeting.ZoomConfig = convertDomainToZoomConfigResponse(meeting.ZoomConfig)
	}

	// Convert Occurrences
	if len(meeting.Occurrences) > 0 {
		goaMeeting.Occurrences = make([]*meetingservice.Occurrence, 0, len(meeting.Occurrences))
		for _, o := range meeting.Occurrences {
			goaMeeting.Occurrences = append(goaMeeting.Occurrences, convertDomainToOccurrenceResponse(&o))
		}
	}

	return goaMeeting
}

func convertDomainToZoomConfigResponse(z *models.ZoomConfig) *meetingservice.ZoomConfigFull {
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
	if z.Passcode != "" {
		zc.Passcode = utils.StringPtr(z.Passcode)
	}

	return zc
}

func convertDomainToCommitteeResponse(c *models.Committee) *meetingservice.Committee {
	if c == nil {
		return nil
	}

	return &meetingservice.Committee{
		UID:                   c.UID,
		AllowedVotingStatuses: c.AllowedVotingStatuses,
	}
}

func convertDomainToRecurrenceResponse(r *models.Recurrence) *meetingservice.Recurrence {
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

func convertDomainToOccurrenceResponse(o *models.Occurrence) *meetingservice.Occurrence {
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
	if o.ResponseCountMaybe != 0 {
		occ.ResponseCountMaybe = utils.IntPtr(o.ResponseCountMaybe)
	}
	if o.IsCancelled {
		occ.IsCancelled = utils.BoolPtr(o.IsCancelled)
	}

	if o.Recurrence != nil {
		occ.Recurrence = convertDomainToRecurrenceResponse(o.Recurrence)
	}

	return occ
}

// ConvertDomainToSettingsResponse converts a domain MeetingSettings to service model
func ConvertDomainToSettingsResponse(settings *models.MeetingSettings) *meetingservice.MeetingSettings {
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

// ConvertDomainToRegistrantResponse converts a domain Registrant model to a Goa Registrant type for API responses
func ConvertDomainToRegistrantResponse(domainRegistrant *models.Registrant) *meetingservice.Registrant {
	if domainRegistrant == nil {
		return nil
	}

	registrant := &meetingservice.Registrant{
		UID:                domainRegistrant.UID,
		MeetingUID:         domainRegistrant.MeetingUID,
		Email:              domainRegistrant.Email,
		FirstName:          utils.StringPtr(domainRegistrant.FirstName),
		LastName:           utils.StringPtr(domainRegistrant.LastName),
		Type:               string(domainRegistrant.Type),
		CommitteeUID:       domainRegistrant.CommitteeUID,
		Host:               utils.BoolPtr(domainRegistrant.Host),
		OrgIsMember:        utils.BoolPtr(domainRegistrant.OrgIsMember),
		OrgIsProjectMember: utils.BoolPtr(domainRegistrant.OrgIsProjectMember),
	}

	// Set fields that are optional and should only be set if they are not empty
	if domainRegistrant.AvatarURL != "" {
		registrant.AvatarURL = utils.StringPtr(domainRegistrant.AvatarURL)
	}
	if domainRegistrant.LinkedInProfile != "" {
		registrant.LinkedinProfile = utils.StringPtr(domainRegistrant.LinkedInProfile)
	}
	if domainRegistrant.Username != "" {
		registrant.Username = utils.StringPtr(domainRegistrant.Username)
	}
	if domainRegistrant.JobTitle != "" {
		registrant.JobTitle = utils.StringPtr(domainRegistrant.JobTitle)
	}
	if domainRegistrant.OrgName != "" {
		registrant.OrgName = utils.StringPtr(domainRegistrant.OrgName)
	}
	if domainRegistrant.OccurrenceID != "" {
		registrant.OccurrenceID = utils.StringPtr(domainRegistrant.OccurrenceID)
	}

	// Convert timestamps
	if domainRegistrant.CreatedAt != nil {
		registrant.CreatedAt = utils.StringPtr(domainRegistrant.CreatedAt.Format(time.RFC3339))
	}

	if domainRegistrant.UpdatedAt != nil {
		registrant.UpdatedAt = utils.StringPtr(domainRegistrant.UpdatedAt.Format(time.RFC3339))
	}

	return registrant
}

// ConvertDomainToPastMeetingResponse converts a domain PastMeeting to service model
func ConvertDomainToPastMeetingResponse(pastMeeting *models.PastMeeting) *meetingservice.PastMeeting {
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
		result.Recurrence = convertDomainToRecurrenceResponse(pastMeeting.Recurrence)
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
		result.ZoomConfig = convertDomainToZoomConfigResponse(pastMeeting.ZoomConfig)
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

// ConvertDomainToPastMeetingParticipantResponse converts a domain PastMeetingParticipant model to a service response type for API responses
func ConvertDomainToPastMeetingParticipantResponse(domainParticipant *models.PastMeetingParticipant) *meetingservice.PastMeetingParticipant {
	if domainParticipant == nil {
		return nil
	}

	participant := &meetingservice.PastMeetingParticipant{
		UID:                domainParticipant.UID,
		PastMeetingUID:     domainParticipant.PastMeetingUID,
		MeetingUID:         domainParticipant.MeetingUID,
		Email:              domainParticipant.Email,
		FirstName:          utils.StringPtr(domainParticipant.FirstName),
		LastName:           utils.StringPtr(domainParticipant.LastName),
		Host:               utils.BoolPtr(domainParticipant.Host),
		OrgIsMember:        utils.BoolPtr(domainParticipant.OrgIsMember),
		OrgIsProjectMember: utils.BoolPtr(domainParticipant.OrgIsProjectMember),
		IsInvited:          utils.BoolPtr(domainParticipant.IsInvited),
		IsAttended:         utils.BoolPtr(domainParticipant.IsAttended),
	}

	// Set fields that are optional and should only be set if they are not empty
	if domainParticipant.AvatarURL != "" {
		participant.AvatarURL = utils.StringPtr(domainParticipant.AvatarURL)
	}
	if domainParticipant.LinkedInProfile != "" {
		participant.LinkedinProfile = utils.StringPtr(domainParticipant.LinkedInProfile)
	}
	if domainParticipant.Username != "" {
		participant.Username = utils.StringPtr(domainParticipant.Username)
	}
	if domainParticipant.JobTitle != "" {
		participant.JobTitle = utils.StringPtr(domainParticipant.JobTitle)
	}
	if domainParticipant.OrgName != "" {
		participant.OrgName = utils.StringPtr(domainParticipant.OrgName)
	}

	// Convert timestamps
	if domainParticipant.CreatedAt != nil {
		participant.CreatedAt = utils.StringPtr(domainParticipant.CreatedAt.Format(time.RFC3339))
	}

	if domainParticipant.UpdatedAt != nil {
		participant.UpdatedAt = utils.StringPtr(domainParticipant.UpdatedAt.Format(time.RFC3339))
	}

	// Convert participant sessions
	if len(domainParticipant.Sessions) > 0 {
		var sessions []*meetingservice.ParticipantSession
		for _, s := range domainParticipant.Sessions {
			session := &meetingservice.ParticipantSession{
				UID:      s.UID,
				JoinTime: s.JoinTime.Format(time.RFC3339),
			}
			if s.LeaveTime != nil {
				leaveTime := s.LeaveTime.Format(time.RFC3339)
				session.LeaveTime = &leaveTime
			}
			if s.LeaveReason != "" {
				session.LeaveReason = utils.StringPtr(s.LeaveReason)
			}
			sessions = append(sessions, session)
		}
		participant.Sessions = sessions
	}

	return participant
}

// ConvertDomainToPastMeetingSummaryResponse converts a domain past meeting summary model to an API response
func ConvertDomainToPastMeetingSummaryResponse(summary *models.PastMeetingSummary) *meetingservice.PastMeetingSummary {
	if summary == nil {
		return nil
	}

	result := &meetingservice.PastMeetingSummary{
		UID:              summary.UID,
		PastMeetingUID:   summary.PastMeetingUID,
		MeetingUID:       summary.MeetingUID,
		Platform:         summary.Platform,
		RequiresApproval: summary.RequiresApproval,
		Approved:         summary.Approved,
		EmailSent:        summary.EmailSent,
		Password:         utils.StringPtr(summary.Password),
		SummaryData: &meetingservice.SummaryData{
			StartTime:     summary.SummaryData.StartTime.Format(time.RFC3339),
			EndTime:       summary.SummaryData.EndTime.Format(time.RFC3339),
			Title:         &summary.SummaryData.Title,
			Content:       &summary.SummaryData.Content,
			DocURL:        &summary.SummaryData.DocURL,
			EditedContent: &summary.SummaryData.EditedContent,
		},
		CreatedAt: summary.CreatedAt.Format(time.RFC3339),
		UpdatedAt: summary.UpdatedAt.Format(time.RFC3339),
	}

	if summary.ZoomConfig != nil {
		result.ZoomConfig = &meetingservice.PastMeetingSummaryZoomConfig{
			MeetingID:   utils.StringPtr(summary.ZoomConfig.MeetingID),
			MeetingUUID: utils.StringPtr(summary.ZoomConfig.MeetingUUID),
		}
	}

	return result
}

// ConvertDomainToPastMeetingAttachmentResponse converts a domain past meeting attachment model to an API response
func ConvertDomainToPastMeetingAttachmentResponse(attachment *models.PastMeetingAttachment) *meetingservice.PastMeetingAttachment {
	if attachment == nil {
		return nil
	}

	result := &meetingservice.PastMeetingAttachment{
		UID:            attachment.UID,
		PastMeetingUID: attachment.PastMeetingUID,
		Type:           attachment.Type,
		Name:           attachment.Name,
		UploadedBy:     attachment.UploadedBy,
	}

	if attachment.Link != "" {
		result.Link = utils.StringPtr(attachment.Link)
	}

	if attachment.FileName != "" {
		result.FileName = utils.StringPtr(attachment.FileName)
	}

	if attachment.FileSize > 0 {
		fileSize := attachment.FileSize
		result.FileSize = &fileSize
	}

	if attachment.ContentType != "" {
		result.ContentType = utils.StringPtr(attachment.ContentType)
	}

	if attachment.SourceObjectUID != "" {
		result.SourceObjectUID = utils.StringPtr(attachment.SourceObjectUID)
	}

	if attachment.UploadedAt != nil {
		result.UploadedAt = utils.StringPtr(attachment.UploadedAt.Format(time.RFC3339))
	}

	if attachment.Description != "" {
		result.Description = utils.StringPtr(attachment.Description)
	}

	return result
}

// ConvertRSVPToResponse converts a domain RSVPResponse to Goa RSVPResponse
func ConvertRSVPToResponse(rsvp *models.RSVPResponse) *meetingservice.RSVPResponse {
	if rsvp == nil {
		return nil
	}

	resp := &meetingservice.RSVPResponse{
		ID:           rsvp.ID,
		MeetingUID:   rsvp.MeetingUID,
		RegistrantID: rsvp.RegistrantID,
		Username:     rsvp.Username,
		Email:        rsvp.Email,
		Response:     string(rsvp.Response),
		Scope:        string(rsvp.Scope),
		OccurrenceID: rsvp.OccurrenceID,
	}

	if rsvp.CreatedAt != nil {
		resp.CreatedAt = utils.StringPtr(rsvp.CreatedAt.Format(time.RFC3339))
	}

	if rsvp.UpdatedAt != nil {
		resp.UpdatedAt = utils.StringPtr(rsvp.UpdatedAt.Format(time.RFC3339))
	}

	return resp
}

// ConvertDomainToMeetingAttachmentResponse converts a domain attachment model to an API response
func ConvertDomainToMeetingAttachmentResponse(attachment *models.MeetingAttachment) *meetingservice.MeetingAttachment {
	if attachment == nil {
		return nil
	}

	response := &meetingservice.MeetingAttachment{
		UID:        attachment.UID,
		MeetingUID: attachment.MeetingUID,
		Type:       attachment.Type,
		Name:       attachment.Name,
		UploadedBy: attachment.UploadedBy,
	}

	// Type-specific fields (file-type only)
	if attachment.Type == "file" {
		if attachment.FileName != "" {
			response.FileName = &attachment.FileName
		}
		if attachment.FileSize > 0 {
			fileSize := attachment.FileSize
			response.FileSize = &fileSize
		}
		if attachment.ContentType != "" {
			response.ContentType = &attachment.ContentType
		}
	}

	// Link field (link-type only)
	if attachment.Type == "link" && attachment.Link != "" {
		response.Link = &attachment.Link
	}

	if attachment.UploadedAt != nil {
		uploadedAt := attachment.UploadedAt.Format(time.RFC3339)
		response.UploadedAt = &uploadedAt
	}

	if attachment.Description != "" {
		response.Description = &attachment.Description
	}

	return response
}
