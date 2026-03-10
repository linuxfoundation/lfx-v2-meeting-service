// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package itx

// PastMeetingSummaryResponse represents a past meeting summary from ITX
type PastMeetingSummaryResponse struct {
	// Identifiers
	ID                     string `json:"id"`                          // UUID of the summary
	MeetingAndOccurrenceID string `json:"meeting_and_occurrence_id"`   // Past meeting ID
	MeetingID              string `json:"meeting_id"`                  // Zoom meeting ID
	OccurrenceID           string `json:"occurrence_id"`               // Zoom occurrence ID
	ZoomMeetingUUID        string `json:"zoom_meeting_uuid,omitempty"` // Zoom meeting UUID

	// Summary metadata
	SummaryCreatedTime      string `json:"summary_created_time,omitempty"`       // When summary was created (RFC3339)
	SummaryLastModifiedTime string `json:"summary_last_modified_time,omitempty"` // When summary was last modified (RFC3339)
	SummaryStartTime        string `json:"summary_start_time,omitempty"`         // Summary start time (RFC3339)
	SummaryEndTime          string `json:"summary_end_time,omitempty"`           // Summary end time (RFC3339)

	// Original Zoom AI summary
	SummaryTitle    string                      `json:"summary_title,omitempty"`    // Title from Zoom
	SummaryOverview string                      `json:"summary_overview,omitempty"` // Overview from Zoom
	SummaryDetails  []ZoomMeetingSummaryDetails `json:"summary_details,omitempty"`  // Details from Zoom
	NextSteps       []string                    `json:"next_steps,omitempty"`       // Next steps from Zoom

	// Edited versions
	EditedSummaryOverview string                      `json:"edited_summary_overview,omitempty"` // Edited overview
	EditedSummaryDetails  []ZoomMeetingSummaryDetails `json:"edited_summary_details,omitempty"`  // Edited details
	EditedNextSteps       []string                    `json:"edited_next_steps,omitempty"`       // Edited next steps

	// Approval workflow
	RequiresApproval bool `json:"requires_approval,omitempty"` // Whether approval is required
	Approved         bool `json:"approved,omitempty"`          // Whether approved

	// Audit fields
	CreatedAt  string `json:"created_at,omitempty"`  // Creation timestamp (RFC3339)
	CreatedBy  *User  `json:"created_by,omitempty"`  // Creator user info
	ModifiedAt string `json:"modified_at,omitempty"` // Last modified timestamp (RFC3339)
	ModifiedBy *User  `json:"modified_by,omitempty"` // Last modifier user info
}

// ZoomMeetingSummaryDetails represents a section of the meeting summary
type ZoomMeetingSummaryDetails struct {
	Label   string `json:"label,omitempty"`   // Section label
	Summary string `json:"summary,omitempty"` // Section summary text
}

// UpdatePastMeetingSummaryRequest represents the request to update a past meeting summary
type UpdatePastMeetingSummaryRequest struct {
	EditedSummaryOverview string                      `json:"edited_summary_overview,omitempty"` // Edited overview
	EditedSummaryDetails  []ZoomMeetingSummaryDetails `json:"edited_summary_details,omitempty"`  // Edited details
	EditedNextSteps       []string                    `json:"edited_next_steps,omitempty"`       // Edited next steps
	Approved              *bool                       `json:"approved,omitempty"`                // Approval status
	ModifiedBy            *User                       `json:"modified_by,omitempty"`             // User making the update
}
