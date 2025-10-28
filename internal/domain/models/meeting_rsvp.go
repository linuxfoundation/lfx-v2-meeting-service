// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package models

import (
	"fmt"
	"time"
)

// RSVPResponseType represents the type of RSVP response
type RSVPResponseType string

const (
	// RSVPResponseAccepted indicates the registrant will attend
	RSVPResponseAccepted RSVPResponseType = "accepted"
	// RSVPResponseMaybe indicates the registrant might attend
	RSVPResponseMaybe RSVPResponseType = "maybe"
	// RSVPResponseDeclined indicates the registrant will not attend
	RSVPResponseDeclined RSVPResponseType = "declined"
)

// RSVPScope represents the scope of an RSVP response
type RSVPScope string

const (
	// RSVPScopeSingle indicates the RSVP applies to a single occurrence
	RSVPScopeSingle RSVPScope = "single"
	// RSVPScopeAll indicates the RSVP applies to all occurrences in the series
	RSVPScopeAll RSVPScope = "all"
	// RSVPScopeThisAndFollowing indicates the RSVP applies to a specific occurrence and all following ones
	RSVPScopeThisAndFollowing RSVPScope = "this_and_following"
)

// CreateRSVPRequest represents a request to create or update an RSVP response.
// Either RegistrantID or Username must be provided to identify the registrant.
type CreateRSVPRequest struct {
	MeetingUID   string           // Meeting this RSVP is for
	RegistrantID string           // Registrant who is submitting this RSVP (optional if Username is provided)
	Username     string           // Username of the registrant (optional if RegistrantID is provided)
	Response     RSVPResponseType // The RSVP response (accepted/maybe/declined)
	Scope        RSVPScope        // Scope of the response (single/all/this_and_following)
	OccurrenceID *string          // Occurrence ID (required for single and this_and_following scopes)
}

// RSVPResponse represents a registrant's RSVP response to a meeting or occurrence.
type RSVPResponse struct {
	ID           string           `json:"id"`                      // Unique identifier for this RSVP
	MeetingUID   string           `json:"meeting_uid"`             // Meeting this RSVP is for
	RegistrantID string           `json:"registrant_id"`           // Registrant who submitted this RSVP
	Username     string           `json:"username"`                // Username of the registrant
	Email        string           `json:"email"`                   // Email of the registrant
	Response     RSVPResponseType `json:"response"`                // The RSVP response (accepted/maybe/declined)
	Scope        RSVPScope        `json:"scope"`                   // Scope of the response (single/all/this_and_following)
	OccurrenceID *string          `json:"occurrence_id,omitempty"` // Occurrence ID (required for single and this_and_following scopes)
	CreatedAt    *time.Time       `json:"created_at,omitempty"`
	UpdatedAt    *time.Time       `json:"updated_at,omitempty"`
}

// Tags generates a consistent set of tags for the RSVP response for searching/indexing.
func (r *RSVPResponse) Tags() []string {
	tags := []string{}

	if r == nil {
		return nil
	}

	if r.ID != "" {
		tags = append(tags, r.ID)
		tags = append(tags, fmt.Sprintf("rsvp_id:%s", r.ID))
	}

	if r.MeetingUID != "" {
		tags = append(tags, fmt.Sprintf("meeting_uid:%s", r.MeetingUID))
	}

	if r.RegistrantID != "" {
		tags = append(tags, fmt.Sprintf("registrant_id:%s", r.RegistrantID))
	}

	if r.Username != "" {
		tags = append(tags, fmt.Sprintf("username:%s", r.Username))
	}

	if r.Email != "" {
		tags = append(tags, fmt.Sprintf("email:%s", r.Email))
	}

	if r.Response != "" {
		tags = append(tags, fmt.Sprintf("response:%s", r.Response))
	}

	if r.Scope != "" {
		tags = append(tags, fmt.Sprintf("scope:%s", r.Scope))
	}

	if r.OccurrenceID != nil && *r.OccurrenceID != "" {
		tags = append(tags, fmt.Sprintf("occurrence_id:%s", *r.OccurrenceID))
	}

	return tags
}
