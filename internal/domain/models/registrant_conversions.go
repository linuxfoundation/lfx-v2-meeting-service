// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package models

import (
	"time"

	meetingservice "github.com/linuxfoundation/lfx-v2-meeting-service/gen/meeting_service"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/utils"
)

// ToRegistrantDBModel converts a Goa Registrant type to the domain Registrant model for database storage
func ToRegistrantDBModel(goaRegistrant *meetingservice.Registrant) *Registrant {
	if goaRegistrant == nil {
		return nil
	}

	registrant := &Registrant{
		UID:                goaRegistrant.UID,
		MeetingUID:         goaRegistrant.MeetingUID,
		Email:              goaRegistrant.Email,
		FirstName:          goaRegistrant.FirstName,
		LastName:           goaRegistrant.LastName,
		Host:               utils.BoolValue(goaRegistrant.Host),
		JobTitle:           utils.StringValue(goaRegistrant.JobTitle),
		OccurrenceID:       utils.StringValue(goaRegistrant.OccurrenceID),
		OrgName:            utils.StringValue(goaRegistrant.OrgName),
		OrgIsMember:        utils.BoolValue(goaRegistrant.OrgIsMember),
		OrgIsProjectMember: utils.BoolValue(goaRegistrant.OrgIsProjectMember),
		AvatarURL:          utils.StringValue(goaRegistrant.AvatarURL),
		UserID:             utils.StringValue(goaRegistrant.UserID),
	}

	// Convert timestamps
	if goaRegistrant.CreatedAt != nil {
		createdAt, err := time.Parse(time.RFC3339, *goaRegistrant.CreatedAt)
		if err == nil {
			registrant.CreatedAt = &createdAt
		}
	}

	if goaRegistrant.UpdatedAt != nil {
		updatedAt, err := time.Parse(time.RFC3339, *goaRegistrant.UpdatedAt)
		if err == nil {
			registrant.UpdatedAt = &updatedAt
		}
	}

	return registrant
}

// FromRegistrantDBModel converts a domain Registrant model to a Goa Registrant type for API responses
func FromRegistrantDBModel(domainRegistrant *Registrant) *meetingservice.Registrant {
	if domainRegistrant == nil {
		return nil
	}

	registrant := &meetingservice.Registrant{
		UID:                domainRegistrant.UID,
		MeetingUID:         domainRegistrant.MeetingUID,
		Email:              domainRegistrant.Email,
		FirstName:          domainRegistrant.FirstName,
		LastName:           domainRegistrant.LastName,
		Host:               utils.BoolPtr(domainRegistrant.Host),
		JobTitle:           utils.StringPtr(domainRegistrant.JobTitle),
		OccurrenceID:       utils.StringPtr(domainRegistrant.OccurrenceID),
		OrgName:            utils.StringPtr(domainRegistrant.OrgName),
		OrgIsMember:        utils.BoolPtr(domainRegistrant.OrgIsMember),
		OrgIsProjectMember: utils.BoolPtr(domainRegistrant.OrgIsProjectMember),
		AvatarURL:          utils.StringPtr(domainRegistrant.AvatarURL),
		UserID:             utils.StringPtr(domainRegistrant.UserID),
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

// ToRegistrantDBModelFromCreatePayload converts a Goa CreateMeetingRegistrantPayload to a domain Registrant model
func ToRegistrantDBModelFromCreatePayload(payload *meetingservice.CreateMeetingRegistrantPayload) *Registrant {
	if payload == nil {
		return nil
	}

	now := time.Now().UTC()
	registrant := &Registrant{
		MeetingUID:         payload.MeetingUID,
		Email:              payload.Email,
		FirstName:          payload.FirstName,
		LastName:           payload.LastName,
		Host:               utils.BoolValue(payload.Host),
		JobTitle:           utils.StringValue(payload.JobTitle),
		OccurrenceID:       utils.StringValue(payload.OccurrenceID),
		OrgIsProjectMember: utils.BoolValue(payload.OrgIsProjectMember),
		AvatarURL:          utils.StringValue(payload.AvatarURL),
		UserID:             utils.StringValue(payload.UserID),
		CreatedAt:          &now,
		UpdatedAt:          &now,
	}

	return registrant
}

// ToRegistrantDBModelFromUpdatePayload converts a Goa UpdateMeetingRegistrantPayload to a domain Registrant model
func ToRegistrantDBModelFromUpdatePayload(payload *meetingservice.UpdateMeetingRegistrantPayload, existingRegistrant *Registrant) *Registrant {
	if payload == nil || existingRegistrant == nil {
		return nil
	}

	now := time.Now().UTC()
	registrant := &Registrant{
		UID:                existingRegistrant.UID,
		MeetingUID:         payload.MeetingUID,
		Email:              payload.Email,
		FirstName:          payload.FirstName,
		LastName:           payload.LastName,
		Host:               utils.BoolValue(payload.Host),
		JobTitle:           utils.StringValue(payload.JobTitle),
		OccurrenceID:       utils.StringValue(payload.OccurrenceID),
		OrgIsProjectMember: utils.BoolValue(payload.OrgIsProjectMember),
		AvatarURL:          utils.StringValue(payload.AvatarURL),
		UserID:             utils.StringValue(payload.UserID),
		CreatedAt:          existingRegistrant.CreatedAt,
		UpdatedAt:          &now,
	}

	// Preserve the org membership fields as they might be set by the system
	registrant.OrgName = existingRegistrant.OrgName
	registrant.OrgIsMember = existingRegistrant.OrgIsMember

	return registrant
}
