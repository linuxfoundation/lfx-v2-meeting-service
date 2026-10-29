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

// itxNoUserID is the value ITX stores in place of a blank user_id, because the column is indexed and cannot be
// empty. ITX maps it back to blank in its responses but does not accept it in a request: on a registrant with no
// linked user, a non-empty user_id is read as a new link and looked up, so "NONE" fails as an invalid user.
const itxNoUserID = "NONE"

// normalizeITXUserID returns the user_id as ITX expects to receive it, with its blank sentinel mapped to empty.
func normalizeITXUserID(userID string) string {
	if userID == itxNoUserID {
		return ""
	}
	return userID
}

// ConvertUpdateITXRegistrantPayloadToITX converts Goa update payload to ITX registrant.
//
// user_id and username are forwarded because ITX compares an update's values against the stored ones and rejects
// any difference with "user_id cannot be changed" / "username cannot be changed". A blank counts as a difference
// for a registrant that is linked to a user, so leaving them out fails the update.
//
// On a registrant with no linked user there is nothing to compare, and ITX reads a supplied user_id (or username)
// as a new link to that user. That is already true of username, and the update route requires the organizer
// relation, so user_id adds no new capability. Create does not map user_id; it links through email or username.
func ConvertUpdateITXRegistrantPayloadToITX(p *meetingservice.UpdateItxRegistrantPayload) *itx.ZoomMeetingRegistrant {
	req := &itx.ZoomMeetingRegistrant{
		// Map committee_uid (proxy) to committee_id (ITX)
		CommitteeID:    utils.StringValue(p.CommitteeUID),
		UserID:         normalizeITXUserID(utils.StringValue(p.UserID)),
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

// ConvertSelfRegisterPayloadToITX converts a self-registration Goa payload to an ITX registrant.
// Email and username are intentionally absent from the payload. Email is set by the service layer
// from the JWT claim, falling back to the auth-service profile when the claim is absent. Username
// is always derived from the authenticated JWT principal. Neither can be supplied by the caller.
func ConvertSelfRegisterPayloadToITX(p *meetingservice.SelfRegisterItxMeetingPayload) *itx.ZoomMeetingRegistrant {
	return &itx.ZoomMeetingRegistrant{
		FirstName:  p.FirstName,
		LastName:   p.LastName,
		Org:        utils.StringValue(p.Org),
		JobTitle:   utils.StringValue(p.JobTitle),
		Occurrence: utils.StringValue(p.Occurrence),
	}
}

// ConvertITXRegistrantToGoa converts ITX registrant to Goa response
func ConvertITXRegistrantToGoa(resp *itx.ZoomMeetingRegistrant) *meetingservice.ITXZoomMeetingRegistrant {
	goaResp := &meetingservice.ITXZoomMeetingRegistrant{
		// Read-only fields
		UID:  utils.StringPtrOmitEmpty(resp.ID),
		Type: utils.StringPtrOmitEmpty(string(resp.Type)),

		// Identity fields - map committee_id (ITX) to committee_uid (proxy)
		CommitteeUID: utils.StringPtrOmitEmpty(resp.CommitteeID),
		UserID:       utils.StringPtrOmitEmpty(normalizeITXUserID(resp.UserID)),
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
