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
		ProjectUID:           p.ProjectUID,
		Title:                p.Title,
		StartTime:            p.StartTime,
		Duration:             p.Duration,
		Timezone:             p.Timezone,
		Visibility:           p.Visibility,
		Description:          utils.StringValue(p.Description),
		Restricted:           utils.BoolValue(p.Restricted),
		MeetingType:          utils.StringValue(p.MeetingType),
		EarlyJoinTimeMinutes: utils.IntValue(p.EarlyJoinTimeMinutes),
		RecordingEnabled:     utils.BoolValue(p.RecordingEnabled),
		TranscriptEnabled:    utils.BoolValue(p.TranscriptEnabled),
		YoutubeUploadEnabled: utils.BoolValue(p.YoutubeUploadEnabled),
		ArtifactVisibility:   utils.StringValue(p.ArtifactVisibility),
	}

	// Convert committees
	if p.Committees != nil && len(p.Committees) > 0 {
		req.Committees = make([]models.Committee, len(p.Committees))
		for i, c := range p.Committees {
			if c != nil {
				req.Committees[i] = models.Committee{
					UID:                   c.UID,
					AllowedVotingStatuses: c.AllowedVotingStatuses,
				}
			}
		}
	}

	// Convert recurrence if present
	if p.Recurrence != nil {
		req.Recurrence = &models.ITXRecurrence{
			Type:           p.Recurrence.Type,
			RepeatInterval: p.Recurrence.RepeatInterval,
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
		ProjectUID:           &resp.Project,
		Title:                &resp.Topic,
		StartTime:            &resp.StartTime,
		Duration:             &resp.Duration,
		Timezone:             &resp.Timezone,
		Visibility:           &resp.Visibility,
		Description:          ptrIfNotEmpty(resp.Agenda),
		Restricted:           ptrIfTrue(resp.Restricted),
		MeetingType:          ptrIfNotEmpty(resp.MeetingType),
		EarlyJoinTimeMinutes: ptrIfNotZero(resp.EarlyJoinTime),
		RecordingEnabled:     ptrIfTrue(resp.RecordingEnabled),
		TranscriptEnabled:    ptrIfTrue(resp.TranscriptEnabled),
		YoutubeUploadEnabled: ptrIfTrue(resp.YoutubeUploadEnabled),
		ArtifactVisibility:   ptrIfNotEmpty(resp.RecordingAccess),

		// Read-only response fields
		ID:              &resp.ID,
		HostKey:         &resp.HostKey,
		Passcode:        &resp.Passcode,
		Password:        &resp.Password,
		PublicLink:      &resp.PublicLink,
		CreatedAt:       &resp.CreatedAt,
		ModifiedAt:      &resp.ModifiedAt,
		RegistrantCount: ptrIfNotZero(resp.RegistrantCount),
	}

	// Convert committees
	if len(resp.Committees) > 0 {
		goaResp.Committees = make([]*meetingservice.Committee, len(resp.Committees))
		for i, c := range resp.Committees {
			goaResp.Committees[i] = &meetingservice.Committee{
				UID:                   c.ID,
				AllowedVotingStatuses: c.Filters,
			}
		}
	}

	// Convert recurrence if present
	if resp.Recurrence != nil {
		goaResp.Recurrence = &meetingservice.Recurrence{
			Type:           resp.Recurrence.Type,
			RepeatInterval: resp.Recurrence.RepeatInterval,
			WeeklyDays:     ptrIfNotEmpty(resp.Recurrence.WeeklyDays),
			MonthlyDay:     ptrIfNotZero(resp.Recurrence.MonthlyDay),
			MonthlyWeek:    ptrIfNotZero(resp.Recurrence.MonthlyWeek),
			MonthlyWeekDay: ptrIfNotZero(resp.Recurrence.MonthlyWeekDay),
			EndTimes:       ptrIfNotZero(resp.Recurrence.EndTimes),
			EndDateTime:    ptrIfNotEmpty(resp.Recurrence.EndDateTime),
		}
	}

	// Convert occurrences
	if len(resp.Occurrences) > 0 {
		goaResp.Occurrences = make([]*meetingservice.ITXOccurrence, len(resp.Occurrences))
		for i, occ := range resp.Occurrences {
			goaResp.Occurrences[i] = &meetingservice.ITXOccurrence{
				OccurrenceID:    &occ.OccurrenceID,
				StartTime:       &occ.StartTime,
				Duration:        &occ.Duration,
				Status:          &occ.Status,
				RegistrantCount: ptrIfNotZero(occ.RegistrantCount),
			}
		}
	}

	return goaResp
}

// ConvertCreateITXRegistrantPayloadToITX converts Goa payload to ITX registrant
func ConvertCreateITXRegistrantPayloadToITX(p *meetingservice.CreateItxRegistrantPayload) *itx.ZoomMeetingRegistrant {
	req := &itx.ZoomMeetingRegistrant{
		CommitteeID:    utils.StringValue(p.CommitteeID),
		UserID:         utils.StringValue(p.UserID),
		Email:          utils.StringValue(p.Email),
		Username:       utils.StringValue(p.Username),
		FirstName:      utils.StringValue(p.FirstName),
		LastName:       utils.StringValue(p.LastName),
		Org:            utils.StringValue(p.Org),
		JobTitle:       utils.StringValue(p.JobTitle),
		ProfilePicture: utils.StringValue(p.ProfilePicture),
		Host:           utils.BoolValue(p.Host),
		Occurrence:     utils.StringValue(p.Occurrence),
	}
	return req
}

// ConvertUpdateITXRegistrantPayloadToITX converts Goa update payload to ITX registrant
func ConvertUpdateITXRegistrantPayloadToITX(p *meetingservice.UpdateItxRegistrantPayload) *itx.ZoomMeetingRegistrant {
	req := &itx.ZoomMeetingRegistrant{
		CommitteeID:    utils.StringValue(p.CommitteeID),
		UserID:         utils.StringValue(p.UserID),
		Email:          utils.StringValue(p.Email),
		Username:       utils.StringValue(p.Username),
		FirstName:      utils.StringValue(p.FirstName),
		LastName:       utils.StringValue(p.LastName),
		Org:            utils.StringValue(p.Org),
		JobTitle:       utils.StringValue(p.JobTitle),
		ProfilePicture: utils.StringValue(p.ProfilePicture),
		Host:           utils.BoolValue(p.Host),
		Occurrence:     utils.StringValue(p.Occurrence),
	}
	return req
}

// ConvertITXRegistrantToGoa converts ITX registrant to Goa response
func ConvertITXRegistrantToGoa(resp *itx.ZoomMeetingRegistrant) *meetingservice.ITXZoomMeetingRegistrant {
	goaResp := &meetingservice.ITXZoomMeetingRegistrant{
		// Read-only fields
		ID:   ptrIfNotEmpty(resp.ID),
		Type: ptrIfNotEmpty(resp.Type),

		// Identity fields
		CommitteeID: ptrIfNotEmpty(resp.CommitteeID),
		UserID:      ptrIfNotEmpty(resp.UserID),
		Email:       ptrIfNotEmpty(resp.Email),
		Username:    ptrIfNotEmpty(resp.Username),

		// Personal info
		FirstName:      ptrIfNotEmpty(resp.FirstName),
		LastName:       ptrIfNotEmpty(resp.LastName),
		Org:            ptrIfNotEmpty(resp.Org),
		JobTitle:       ptrIfNotEmpty(resp.JobTitle),
		ProfilePicture: ptrIfNotEmpty(resp.ProfilePicture),

		// Meeting settings
		Host:       ptrIfTrue(resp.Host),
		Occurrence: ptrIfNotEmpty(resp.Occurrence),

		// Tracking fields
		AttendedOccurrenceCount:       ptrIfNotZero(resp.AttendedOccurrenceCount),
		TotalOccurrenceCount:          ptrIfNotZero(resp.TotalOccurrenceCount),
		LastInviteReceivedTime:        ptrIfNotEmpty(resp.LastInviteReceivedTime),
		LastInviteReceivedMessageID:   ptrIfNotEmpty(resp.LastInviteReceivedMessageID),
		LastInviteDeliveryStatus:      ptrIfNotEmpty(resp.LastInviteDeliveryStatus),
		LastInviteDeliveryDescription: ptrIfNotEmpty(resp.LastInviteDeliveryDescription),

		// Audit fields
		CreatedAt:  ptrIfNotEmpty(resp.CreatedAt),
		ModifiedAt: ptrIfNotEmpty(resp.ModifiedAt),
	}

	// Convert created_by user if present
	if resp.CreatedBy != nil {
		goaResp.CreatedBy = &meetingservice.ITXUser{
			ID:             ptrIfNotEmpty(resp.CreatedBy.ID),
			Username:       ptrIfNotEmpty(resp.CreatedBy.Username),
			Name:           ptrIfNotEmpty(resp.CreatedBy.Name),
			Email:          ptrIfNotEmpty(resp.CreatedBy.Email),
			ProfilePicture: ptrIfNotEmpty(resp.CreatedBy.ProfilePicture),
		}
	}

	// Convert updated_by user if present
	if resp.UpdatedBy != nil {
		goaResp.UpdatedBy = &meetingservice.ITXUser{
			ID:             ptrIfNotEmpty(resp.UpdatedBy.ID),
			Username:       ptrIfNotEmpty(resp.UpdatedBy.Username),
			Name:           ptrIfNotEmpty(resp.UpdatedBy.Name),
			Email:          ptrIfNotEmpty(resp.UpdatedBy.Email),
			ProfilePicture: ptrIfNotEmpty(resp.UpdatedBy.ProfilePicture),
		}
	}

	return goaResp
}

// Helper functions for pointer conversion
func ptrIfNotEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func ptrIfTrue(b bool) *bool {
	if !b {
		return nil
	}
	return &b
}

func ptrIfNotZero(i int) *int {
	if i == 0 {
		return nil
	}
	return &i
}
