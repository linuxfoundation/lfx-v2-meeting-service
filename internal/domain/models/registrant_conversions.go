// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package models

import (
	"time"

	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/utils"
)

// MergeUpdateRegistrantRequest merges an update request with an existing registrant
func MergeUpdateRegistrantRequest(reqRegistrant *Registrant, existingRegistrant *Registrant) *Registrant {
	if reqRegistrant == nil && existingRegistrant == nil {
		return nil
	}

	// If there's no existing registrant but there's a payload, create new registrant
	if existingRegistrant == nil {
		return reqRegistrant
	}

	// If there's existing registrant but no payload, preserve existing registrant
	if reqRegistrant == nil {
		return existingRegistrant
	}

	now := time.Now().UTC()
	registrant := &Registrant{
		UID:          existingRegistrant.UID,
		MeetingUID:   existingRegistrant.MeetingUID,
		Email:        utils.CoalesceString(reqRegistrant.Email, existingRegistrant.Email),
		FirstName:    utils.CoalesceString(reqRegistrant.FirstName, existingRegistrant.FirstName),
		LastName:     utils.CoalesceString(reqRegistrant.LastName, existingRegistrant.LastName),
		Host:         reqRegistrant.Host,
		Type:         existingRegistrant.Type, // Preserve existing type
		JobTitle:     utils.CoalesceString(reqRegistrant.JobTitle, existingRegistrant.JobTitle),
		OccurrenceID: utils.CoalesceString(reqRegistrant.OccurrenceID, existingRegistrant.OccurrenceID),
		OrgName:      utils.CoalesceString(reqRegistrant.OrgName, existingRegistrant.OrgName),
		AvatarURL:    utils.CoalesceString(reqRegistrant.AvatarURL, existingRegistrant.AvatarURL),
		Username:     utils.CoalesceString(reqRegistrant.Username, existingRegistrant.Username),
		CreatedAt:    existingRegistrant.CreatedAt,
		UpdatedAt:    &now,
	}

	// TODO: get the actual values from the system once there is an org service
	// because the org name could be changing and thus need to be recalculated
	registrant.OrgIsProjectMember = existingRegistrant.OrgIsProjectMember
	registrant.OrgIsMember = existingRegistrant.OrgIsMember

	return registrant
}
