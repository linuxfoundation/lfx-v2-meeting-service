// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	meetingservice "github.com/linuxfoundation/lfx-v2-meeting-service/gen/meeting_service"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/models/itx"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/utils"
)

// ConvertCreateITXRegistrantPayloadToITX converts Goa payload to ITX registrant
func ConvertCreateITXRegistrantPayloadToITX(p *meetingservice.CreateItxRegistrantPayload) *itx.ZoomMeetingRegistrant {
	req := &itx.ZoomMeetingRegistrant{
		// Map committee_uid (proxy) to committee_id (ITX)
		CommitteeID:    utils.StringValue(p.CommitteeUID),
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
		// Map committee_uid (proxy) to committee_id (ITX)
		CommitteeID:    utils.StringValue(p.CommitteeUID),
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
		UID:  utils.StringPtrOmitEmpty(resp.ID),
		Type: utils.StringPtrOmitEmpty(string(resp.Type)),

		// Identity fields - map committee_id (ITX) to committee_uid (proxy)
		CommitteeUID: utils.StringPtrOmitEmpty(resp.CommitteeID),
		Email:        utils.StringPtrOmitEmpty(resp.Email),
		Username:     utils.StringPtrOmitEmpty(resp.Username),

		// Personal info
		FirstName:      utils.StringPtrOmitEmpty(resp.FirstName),
		LastName:       utils.StringPtrOmitEmpty(resp.LastName),
		Org:            utils.StringPtrOmitEmpty(resp.Org),
		JobTitle:       utils.StringPtrOmitEmpty(resp.JobTitle),
		ProfilePicture: utils.StringPtrOmitEmpty(resp.ProfilePicture),

		// Meeting settings
		Host:       utils.BoolPtrOmitFalse(resp.Host),
		Occurrence: utils.StringPtrOmitEmpty(resp.Occurrence),

		// Tracking fields
		AttendedOccurrenceCount:       utils.IntPtrOmitZero(resp.AttendedOccurrenceCount),
		TotalOccurrenceCount:          utils.IntPtrOmitZero(resp.TotalOccurrenceCount),
		LastInviteReceivedTime:        utils.StringPtrOmitEmpty(resp.LastInviteReceivedTime),
		LastInviteReceivedMessageID:   utils.StringPtrOmitEmpty(resp.LastInviteReceivedMessageID),
		LastInviteDeliveryStatus:      utils.StringPtrOmitEmpty(resp.LastInviteDeliveryStatus),
		LastInviteDeliveryDescription: utils.StringPtrOmitEmpty(resp.LastInviteDeliveryDescription),

		// Audit fields
		CreatedAt:  utils.StringPtrOmitEmpty(resp.CreatedAt),
		ModifiedAt: utils.StringPtrOmitEmpty(resp.ModifiedAt),
	}

	// Convert created_by user if present
	if resp.CreatedBy != nil {
		goaResp.CreatedBy = &meetingservice.ITXUser{
			Username:       utils.StringPtrOmitEmpty(resp.CreatedBy.Username),
			Name:           utils.StringPtrOmitEmpty(resp.CreatedBy.Name),
			Email:          utils.StringPtrOmitEmpty(resp.CreatedBy.Email),
			ProfilePicture: utils.StringPtrOmitEmpty(resp.CreatedBy.ProfilePicture),
		}
	}

	// Convert updated_by user if present
	if resp.UpdatedBy != nil {
		goaResp.UpdatedBy = &meetingservice.ITXUser{
			Username:       utils.StringPtrOmitEmpty(resp.UpdatedBy.Username),
			Name:           utils.StringPtrOmitEmpty(resp.UpdatedBy.Name),
			Email:          utils.StringPtrOmitEmpty(resp.UpdatedBy.Email),
			ProfilePicture: utils.StringPtrOmitEmpty(resp.UpdatedBy.ProfilePicture),
		}
	}

	return goaResp
}
