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

