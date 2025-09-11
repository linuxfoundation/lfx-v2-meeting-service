// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package models

import (
	"fmt"
	"time"
)

// PastMeetingSummary represents an AI-generated summary for a past meeting occurrence
// It captures summary data from platform providers (e.g., Zoom AI summary)
type PastMeetingSummary struct {
	UID              string                        `json:"uid"`
	PastMeetingUID   string                        `json:"past_meeting_uid"`
	MeetingUID       string                        `json:"meeting_uid"`
	Platform         string                        `json:"platform"`
	Password         string                        `json:"password"`
	ZoomConfig       *PastMeetingSummaryZoomConfig `json:"zoom_config,omitempty"`
	SummaryData      SummaryData                   `json:"summary_data"`
	RequiresApproval bool                          `json:"requires_approval"`
	Approved         bool                          `json:"approved"`
	EmailSent        bool                          `json:"email_sent"`
	CreatedAt        time.Time                     `json:"created_at"`
	UpdatedAt        time.Time                     `json:"updated_at"`
}

// PastMeetingSummaryZoomConfig contains Zoom-specific summary configuration and metadata
type PastMeetingSummaryZoomConfig struct {
	MeetingID string `json:"meeting_id"` // Zoom meeting ID
	UUID      string `json:"uuid"`       // Zoom meeting UUID
}

// SummaryData contains the actual AI-generated summary content
type SummaryData struct {
	StartTime       time.Time       `json:"start_time"`
	EndTime         time.Time       `json:"end_time"`
	Title           string          `json:"title"`
	Overview        string          `json:"overview"`
	NextSteps       []string        `json:"next_steps"`
	Details         []SummaryDetail `json:"details"`
	EditedOverview  string          `json:"edited_overview"`
	EditedDetails   []SummaryDetail `json:"edited_details"`
	EditedNextSteps []string        `json:"edited_next_steps"`
}

// Tags generates a consistent set of tags for the past meeting summary.
// IMPORTANT: If you modify this method, please update the PastMeetingSummary Tags documentation in the README.md
// to ensure consumers understand how to use these tags for searching.
func (p *PastMeetingSummary) Tags() []string {
	tags := []string{}

	if p == nil {
		return nil
	}

	if p.UID != "" {
		// without prefix
		tags = append(tags, p.UID)
		// with prefix
		tag := fmt.Sprintf("past_meeting_summary_uid:%s", p.UID)
		tags = append(tags, tag)
	}

	if p.PastMeetingUID != "" {
		tag := fmt.Sprintf("past_meeting_uid:%s", p.PastMeetingUID)
		tags = append(tags, tag)
	}

	if p.MeetingUID != "" {
		tag := fmt.Sprintf("meeting_uid:%s", p.MeetingUID)
		tags = append(tags, tag)
	}

	if p.Platform != "" {
		tag := fmt.Sprintf("platform:%s", p.Platform)
		tags = append(tags, tag)
	}

	// Add summary title and overview as searchable tags (without prefix for full-text search)
	if p.SummaryData.Title != "" {
		tags = append(tags, p.SummaryData.Title)
	}

	if p.SummaryData.Overview != "" {
		tags = append(tags, p.SummaryData.Overview)
	}

	// Add edited overview as searchable tags if present
	if p.SummaryData.EditedOverview != "" {
		tags = append(tags, p.SummaryData.EditedOverview)
	}

	// Add next steps as searchable tags (without prefix for full-text search)
	for _, nextStep := range p.SummaryData.NextSteps {
		if nextStep != "" {
			tags = append(tags, nextStep)
		}
	}

	// Add edited next steps as searchable tags if present
	for _, editedNextStep := range p.SummaryData.EditedNextSteps {
		if editedNextStep != "" {
			tags = append(tags, editedNextStep)
		}
	}

	return tags
}
