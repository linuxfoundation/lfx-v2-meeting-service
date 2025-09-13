// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package models

import (
	"fmt"
	"time"
)

// PastMeetingSummary represents a summary for a past meeting occurrence
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
	MeetingID   string `json:"meeting_id"`   // Zoom meeting ID
	MeetingUUID string `json:"meeting_uuid"` // Zoom meeting UUID (specific meeting instance)
}

// SummaryData contains the actual AI-generated summary content
type SummaryData struct {
	StartTime     time.Time `json:"start_time"`
	EndTime       time.Time `json:"end_time"`
	Title         string    `json:"title"`
	Content       string    `json:"content"`
	DocURL        string    `json:"doc_url"`
	EditedContent string    `json:"edited_content"`
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

	if p.SummaryData.Title != "" {
		tag := fmt.Sprintf("title:%s", p.SummaryData.Title)
		tags = append(tags, tag)
	}

	return tags
}
