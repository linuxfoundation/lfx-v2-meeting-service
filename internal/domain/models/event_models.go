// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package models

// This file contains event data models for v1→v2 meeting event transformation.
// Models will be added incrementally as handlers are implemented.

// MeetingEventData represents a meeting event for indexing and access control
type MeetingEventData struct {
	// Placeholder - will be populated during handler implementation
}

// RegistrantEventData represents a registrant event for indexing and access control
type RegistrantEventData struct {
	// Placeholder - will be populated during handler implementation
}

// InviteResponseEventData represents an RSVP event for indexing
type InviteResponseEventData struct {
	// Placeholder - will be populated during handler implementation
}

// PastMeetingEventData represents a past meeting event for indexing and access control
type PastMeetingEventData struct {
	// Placeholder - will be populated during handler implementation
}

// PastMeetingParticipantEventData represents a participant (invitee/attendee) event
type PastMeetingParticipantEventData struct {
	// Placeholder - will be populated during handler implementation
}

// RecordingEventData represents a recording artifact event
type RecordingEventData struct {
	// Placeholder - will be populated during handler implementation
}

// TranscriptEventData represents a transcript artifact event
type TranscriptEventData struct {
	// Placeholder - will be populated during handler implementation
}

// SummaryEventData represents an AI-generated summary event
type SummaryEventData struct {
	// Placeholder - will be populated during handler implementation
}
