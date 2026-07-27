// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package itx

// CreatePastMeetingRequest represents the request to create or update a past meeting.
// The same struct is reused for PUT; CreatedBy is only meaningful on POST and UpdatedBy
// only on PUT (the service stamps whichever is appropriate).
type CreatePastMeetingRequest struct {
	// Required fields
	MeetingID    string `json:"meeting_id"`    // Zoom meeting ID
	OccurrenceID string `json:"occurrence_id"` // Zoom occurrence ID (Unix timestamp)
	ProjectID    string `json:"project_id"`    // LF project ID
	StartTime    string `json:"start_time"`    // Meeting start time in RFC3339 format
	Duration     int    `json:"duration"`      // Meeting duration in minutes
	Timezone     string `json:"timezone"`      // Meeting timezone

	// Optional fields
	Topic             string            `json:"topic,omitempty"`              // Meeting title/topic
	Agenda            string            `json:"agenda,omitempty"`             // Meeting description/agenda
	Restricted        bool              `json:"restricted,omitempty"`         // Whether meeting was restricted
	Committees        []Committee       `json:"committees,omitempty"`         // Associated committees
	CommitteeID       string            `json:"committee_id,omitempty"`       // Single committee ID
	CommitteeFilters  []string          `json:"committee_filters,omitempty"`  // Committee member filters
	MeetingType       MeetingType       `json:"meeting_type,omitempty"`       // Meeting type
	RecordingEnabled  bool              `json:"recording_enabled,omitempty"`  // Was recording enabled
	RecordingAccess   ArtifactAccess    `json:"recording_access,omitempty"`   // Who can access recordings
	TranscriptEnabled bool              `json:"transcript_enabled,omitempty"` // Was transcription enabled
	TranscriptAccess  ArtifactAccess    `json:"transcript_access,omitempty"`  // Who can access transcripts
	Visibility        MeetingVisibility `json:"visibility,omitempty"`         // Meeting visibility (public/private)

	// CreatedBy identifies the requesting user at creation time (POST only). Leave nil
	// on updates.
	CreatedBy *User `json:"created_by,omitempty"`

	// UpdatedBy identifies the requesting user on update requests. ITX only overwrites
	// the stored updated_by / updated_by_list when this field is non-zero, so it must
	// be populated on every update to keep the audit trail accurate. Leave nil on
	// create requests.
	UpdatedBy *User `json:"updated_by,omitempty"`
}

// PastMeetingResponse represents the response from creating/retrieving a past meeting
type PastMeetingResponse struct {
	// Identifiers
	PastMeetingID string `json:"past_meeting_id"` // Past meeting ID (meeting_id or meeting_id-occurrence_id)
	MeetingID     string `json:"meeting_id"`      // Zoom meeting ID
	OccurrenceID  string `json:"occurrence_id"`   // Zoom occurrence ID
	ProjectID     string `json:"project_id"`      // LF project ID

	// Meeting details
	Topic      string            `json:"topic,omitempty"`      // Meeting title
	Agenda     string            `json:"agenda,omitempty"`     // Meeting description
	StartTime  string            `json:"start_time"`           // Meeting start time (RFC3339)
	Duration   int               `json:"duration"`             // Meeting duration in minutes
	Timezone   string            `json:"timezone"`             // Meeting timezone
	Visibility MeetingVisibility `json:"visibility,omitempty"` // Meeting visibility
	Restricted bool              `json:"restricted"`           // Whether meeting was restricted

	// Committee association
	Committees       []Committee `json:"committees,omitempty"`        // Associated committees
	CommitteeID      string      `json:"committee_id,omitempty"`      // Single committee ID
	CommitteeFilters []string    `json:"committee_filters,omitempty"` // Committee filters

	// Meeting type
	MeetingType MeetingType `json:"meeting_type,omitempty"` // Type of meeting

	// Recording/Transcript settings
	RecordingEnabled  bool           `json:"recording_enabled"`           // Was recording enabled
	RecordingAccess   ArtifactAccess `json:"recording_access,omitempty"`  // Who can access recordings
	TranscriptEnabled bool           `json:"transcript_enabled"`          // Was transcription enabled
	TranscriptAccess  ArtifactAccess `json:"transcript_access,omitempty"` // Who can access transcripts

	// Metadata
	IsManuallyCreated bool `json:"is_manually_created,omitempty"` // Whether manually created

	// Security
	MeetingPassword string `json:"meeting_password,omitempty"` // ITX-generated UUID for securing the past meeting join page
}
