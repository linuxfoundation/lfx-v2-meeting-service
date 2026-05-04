// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	meetingservice "github.com/linuxfoundation/lfx-v2-meeting-service/gen/meeting_service"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/models/itx"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/utils"
)

// ConvertCreateITXMeetingPayloadToDomain converts Goa payload to ITX meeting request
func ConvertCreateITXMeetingPayloadToDomain(p *meetingservice.CreateItxMeetingPayload) *models.CreateITXMeetingRequest {
	req := &models.CreateITXMeetingRequest{
		ProjectUID:               p.ProjectUID,
		Title:                    p.Title,
		StartTime:                p.StartTime,
		Duration:                 p.Duration,
		Timezone:                 p.Timezone,
		Visibility:               itx.MeetingVisibility(p.Visibility),
		Description:              utils.StringValue(p.Description),
		Restricted:               utils.BoolValue(p.Restricted),
		MeetingType:              itx.MeetingType(utils.StringValue(p.MeetingType)),
		EarlyJoinTimeMinutes:     utils.IntValue(p.EarlyJoinTimeMinutes),
		RecordingEnabled:         utils.BoolValue(p.RecordingEnabled),
		TranscriptEnabled:        utils.BoolValue(p.TranscriptEnabled),
		YoutubeUploadEnabled:     utils.BoolValue(p.YoutubeUploadEnabled),
		AISummaryEnabled:         utils.BoolValue(p.AiSummaryEnabled),
		RequireAISummaryApproval: utils.BoolValue(p.RequireAiSummaryApproval),
		ArtifactVisibility:       itx.ArtifactAccess(utils.StringValue(p.ArtifactVisibility)),
	}

	// Convert committees
	if len(p.Committees) > 0 {
		req.Committees = make([]models.Committee, len(p.Committees))
		for i, c := range p.Committees {
			if c != nil {
				req.Committees[i] = models.Committee{
					UID:                   utils.StringValue(c.UID),
					AllowedVotingStatuses: utils.CastSlice[itx.CommitteeFilter](c.AllowedVotingStatuses),
				}
			}
		}
	}

	// Convert recurrence if present
	if p.Recurrence != nil {
		req.Recurrence = &models.ITXRecurrence{
			Type:           itx.RecurrenceType(utils.IntValue(p.Recurrence.Type)),
			RepeatInterval: utils.IntValue(p.Recurrence.RepeatInterval),
			WeeklyDays:     utils.StringValue(p.Recurrence.WeeklyDays),
			MonthlyDay:     utils.IntValue(p.Recurrence.MonthlyDay),
			MonthlyWeek:    utils.IntValue(p.Recurrence.MonthlyWeek),
			MonthlyWeekDay: utils.IntValue(p.Recurrence.MonthlyWeekDay),
			EndTimes:       utils.IntValue(p.Recurrence.EndTimes),
			EndDateTime:    utils.StringValue(p.Recurrence.EndDateTime),
		}
	}

	return req
}

// ConvertITXMeetingResponseToGoa converts ITX response to Goa response
func ConvertITXMeetingResponseToGoa(resp *itx.ZoomMeetingResponse) *meetingservice.ITXZoomMeetingResponse {
	goaResp := &meetingservice.ITXZoomMeetingResponse{
		// Request fields echoed back
		ProjectUID:                         &resp.Project,
		Title:                              &resp.Topic,
		StartTime:                          &resp.StartTime,
		Duration:                           &resp.Duration,
		Timezone:                           &resp.Timezone,
		Visibility:                         (*string)(&resp.Visibility),
		Description:                        utils.StringPtrOmitEmpty(resp.Agenda),
		Restricted:                         utils.BoolPtrOmitFalse(resp.Restricted),
		MeetingType:                        utils.StringPtrOmitEmpty(string(resp.MeetingType)),
		EarlyJoinTimeMinutes:               utils.IntPtrOmitZero(resp.EarlyJoinTime),
		RecordingEnabled:                   &resp.RecordingEnabled,
		TranscriptEnabled:                  &resp.TranscriptEnabled,
		YoutubeUploadEnabled:               &resp.YoutubeUploadEnabled,
		AiSummaryEnabled:                   &resp.ZoomAIEnabled,
		RequireAiSummaryApproval:           utils.BoolPtrOmitFalse(resp.RequireAISummaryApproval),
		ArtifactVisibility:                 utils.StringPtrOmitEmpty(string(utils.Coalesce(resp.RecordingAccess, resp.TranscriptAccess, resp.AISummaryAccess))),
		AutoEmailReminderEnabled:           utils.BoolPtrOmitFalse(resp.AutoEmailReminderEnabled),
		AutoEmailReminderTime:              utils.IntPtrOmitZero(resp.AutoEmailReminderTime),
		IsInviteResponsesEnabled:           utils.BoolPtrOmitFalse(resp.IsInviteResponsesEnabled),
		ResponseCountYes:                   utils.IntPtrOmitZero(resp.ResponseCountYes),
		ResponseCountMaybe:                 utils.IntPtrOmitZero(resp.ResponseCountMaybe),
		ResponseCountNo:                    utils.IntPtrOmitZero(resp.ResponseCountNo),
		LastBulkRegistrantJobStatus:        utils.StringPtrOmitEmpty(resp.LastBulkRegistrantJobStatus),
		LastBulkRegistrantsJobWarningCount: utils.IntPtrOmitZero(resp.LastBulkRegistrantsJobWarningCount),
		EmailDeliveryErrorCount:            utils.IntPtrOmitZero(resp.EmailDeliveryErrorCount),

		LastMailingListMembersSyncJobStatus:       utils.StringPtrOmitEmpty(resp.LastMailingListMembersSyncJobStatus),
		LastMailingListMembersSyncJobFailedCount:  utils.IntPtrOmitZero(resp.LastMailingListMembersSyncJobFailedCount),
		LastMailingListMembersSyncJobWarningCount: utils.IntPtrOmitZero(resp.LastMailingListMembersSyncJobWarningCount),

		// Read-only response fields
		ID:              &resp.ID,
		HostKey:         &resp.HostKey,
		Passcode:        &resp.Passcode,
		Password:        &resp.Password,
		PublicLink:      &resp.PublicLink,
		CreatedAt:       &resp.CreatedAt,
		ModifiedAt:      &resp.ModifiedAt,
		RegistrantCount: utils.IntPtrOmitZero(resp.RegistrantCount),
	}

	// Convert committees
	if len(resp.Committees) > 0 {
		goaResp.Committees = make([]*meetingservice.Committee, len(resp.Committees))
		for i := range resp.Committees {
			id := resp.Committees[i].ID
			goaResp.Committees[i] = &meetingservice.Committee{
				UID:                   &id,
				AllowedVotingStatuses: utils.CastSlice[string](resp.Committees[i].Filters),
			}
		}
	}

	// Convert recurrence if present
	if resp.Recurrence != nil {
		goaResp.Recurrence = &meetingservice.Recurrence{
			Type:           utils.IntPtrOmitZero(int(resp.Recurrence.Type)),
			RepeatInterval: utils.IntPtrOmitZero(resp.Recurrence.RepeatInterval),
			WeeklyDays:     utils.StringPtrOmitEmpty(resp.Recurrence.WeeklyDays),
			MonthlyDay:     utils.IntPtrOmitZero(resp.Recurrence.MonthlyDay),
			MonthlyWeek:    utils.IntPtrOmitZero(resp.Recurrence.MonthlyWeek),
			MonthlyWeekDay: utils.IntPtrOmitZero(resp.Recurrence.MonthlyWeekDay),
			EndTimes:       utils.IntPtrOmitZero(resp.Recurrence.EndTimes),
			EndDateTime:    utils.StringPtrOmitEmpty(resp.Recurrence.EndDateTime),
		}
	}

	// Convert occurrences
	if len(resp.Occurrences) > 0 {
		goaResp.Occurrences = make([]*meetingservice.ITXOccurrence, len(resp.Occurrences))
		for i := range resp.Occurrences {
			occurrenceID := resp.Occurrences[i].OccurrenceID
			startTime := resp.Occurrences[i].StartTime
			duration := resp.Occurrences[i].Duration
			status := string(resp.Occurrences[i].Status)
			goaResp.Occurrences[i] = &meetingservice.ITXOccurrence{
				OccurrenceID:    &occurrenceID,
				StartTime:       &startTime,
				Duration:        &duration,
				Status:          &status,
				RegistrantCount: utils.IntPtrOmitZero(resp.Occurrences[i].RegistrantCount),
			}
		}
	}

	return goaResp
}

// ConvertGetJoinLinkPayloadToITX converts Goa payload to ITX join link request
func ConvertGetJoinLinkPayloadToITX(p *meetingservice.GetItxJoinLinkPayload) *itx.GetJoinLinkRequest {
	req := &itx.GetJoinLinkRequest{
		MeetingID: p.MeetingID,
	}

	if p.UseEmail != nil {
		req.UseEmail = *p.UseEmail
	}
	if p.UserID != nil {
		req.UserID = *p.UserID
	}
	if p.Name != nil {
		req.Name = *p.Name
	}
	if p.Email != nil {
		req.Email = *p.Email
	}
	if p.Register != nil {
		req.Register = *p.Register
	}

	return req
}

// ConvertITXJoinLinkResponseToGoa converts ITX join link response to Goa response
func ConvertITXJoinLinkResponseToGoa(resp *itx.ZoomMeetingJoinLink) *meetingservice.ITXZoomMeetingJoinLink {
	return &meetingservice.ITXZoomMeetingJoinLink{
		Link: resp.Link,
	}
}

// ConvertUpdateOccurrencePayloadToITX converts Goa payload to ITX update occurrence request
func ConvertUpdateOccurrencePayloadToITX(p *meetingservice.UpdateItxOccurrencePayload) *itx.UpdateOccurrenceRequest {
	req := &itx.UpdateOccurrenceRequest{}

	if p.StartTime != nil {
		req.StartTime = *p.StartTime
	}
	if p.Duration != nil {
		req.Duration = *p.Duration
	}
	if p.Topic != nil {
		req.Topic = *p.Topic
	}
	if p.Agenda != nil {
		req.Agenda = *p.Agenda
	}
	if p.Recurrence != nil {
		req.Recurrence = &itx.Recurrence{
			Type:           itx.RecurrenceType(utils.IntValue(p.Recurrence.Type)),
			RepeatInterval: utils.IntValue(p.Recurrence.RepeatInterval),
			WeeklyDays:     utils.StringValue(p.Recurrence.WeeklyDays),
			MonthlyDay:     utils.IntValue(p.Recurrence.MonthlyDay),
			MonthlyWeek:    utils.IntValue(p.Recurrence.MonthlyWeek),
			MonthlyWeekDay: utils.IntValue(p.Recurrence.MonthlyWeekDay),
			EndTimes:       utils.IntValue(p.Recurrence.EndTimes),
			EndDateTime:    utils.StringValue(p.Recurrence.EndDateTime),
		}
	}

	return req
}

// ConvertSubmitITXMeetingResponsePayloadToITX converts Goa payload to ITX meeting response request
func ConvertSubmitITXMeetingResponsePayloadToITX(p *meetingservice.SubmitItxMeetingResponsePayload) *itx.MeetingResponseRequest {
	return &itx.MeetingResponseRequest{
		Response:     p.Response,
		Scope:        p.Scope,
		RegistrantID: p.RegistrantID,
	}
}

// ConvertITXMeetingResponseResultToGoa converts an ITX meeting response result to a Goa response
func ConvertITXMeetingResponseResultToGoa(r *itx.MeetingResponseResult) *meetingservice.ITXMeetingResponseResult {
	return &meetingservice.ITXMeetingResponseResult{
		ID:           r.ID,
		MeetingID:    r.MeetingID,
		RegistrantID: r.RegistrantID,
		Username:     utils.StringPtrOmitEmpty(r.Username),
		Email:        utils.StringPtrOmitEmpty(r.Email),
		Response:     r.Response,
		Scope:        r.Scope,
		OccurrenceID: utils.StringPtrOmitEmpty(r.OccurrenceID),
		CreatedAt:    utils.StringPtrOmitEmpty(r.CreatedAt),
		UpdatedAt:    utils.StringPtrOmitEmpty(r.UpdatedAt),
	}
}
