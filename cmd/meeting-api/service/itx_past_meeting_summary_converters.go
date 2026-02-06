// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"fmt"
	"strings"

	meetingservice "github.com/linuxfoundation/lfx-v2-meeting-service/gen/meeting_service"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/models/itx"
)

// ConvertUpdatePastMeetingSummaryPayload converts V2 Goa payload to ITX update request
func ConvertUpdatePastMeetingSummaryPayload(payload *meetingservice.UpdateItxPastMeetingSummaryPayload) *itx.UpdatePastMeetingSummaryRequest {
	req := &itx.UpdatePastMeetingSummaryRequest{}

	// V2 has a single edited_content field, ITX has separate fields
	// For simplicity, we'll put the entire edited_content into edited_summary_overview
	// A more sophisticated implementation could parse the content into sections
	if payload.EditedContent != nil {
		req.EditedContent = *payload.EditedContent
	}
	if payload.Approved != nil {
		req.Approved = payload.Approved
	}
	// Note: ModifiedBy is derived from JWT token in the ITX service, not from payload

	return req
}

// ConvertPastMeetingSummaryToGoa converts ITX response to V2 Goa type
func ConvertPastMeetingSummaryToGoa(resp *itx.PastMeetingSummaryResponse) *meetingservice.PastMeetingSummary {
	// Build the main content from ITX summary parts (overview + details + next steps)
	content := buildContentFromITX(resp)

	// Build edited content from ITX edited parts (edited_overview + edited_details + edited_next_steps)
	editedContent := buildEditedContentFromITX(resp)

	// Create the summary_data object (start_time and end_time are required)
	summaryData := &meetingservice.SummaryData{
		StartTime:     resp.SummaryStartTime,
		EndTime:       resp.SummaryEndTime,
		Title:         ptrIfNotEmpty(resp.SummaryTitle),
		Content:       ptrIfNotEmpty(content),
		DocURL:        ptrIfNotEmpty(""), // ITX doesn't provide doc_url
		EditedContent: ptrIfNotEmpty(editedContent),
	}

	// Build Zoom config if available
	var zoomConfig *meetingservice.PastMeetingSummaryZoomConfig
	if resp.MeetingID != "" || resp.ZoomMeetingUUID != "" {
		zoomConfig = &meetingservice.PastMeetingSummaryZoomConfig{
			MeetingID:   ptrIfNotEmpty(resp.MeetingID),
			MeetingUUID: ptrIfNotEmpty(resp.ZoomMeetingUUID),
		}
	}

	// Create the V2-style response (required fields are non-pointer strings and bools)
	goaResp := &meetingservice.PastMeetingSummary{
		UID:              resp.ID,
		PastMeetingID:    resp.MeetingAndOccurrenceID,
		MeetingID:        resp.MeetingID,
		Platform:         "Zoom",
		Password:         ptrIfNotEmpty(""),
		ZoomConfig:       zoomConfig,
		SummaryData:      summaryData,
		RequiresApproval: resp.RequiresApproval,
		Approved:         resp.Approved,
		EmailSent:        false, // ITX doesn't track this
		CreatedAt:        resp.CreatedAt,
		UpdatedAt:        resp.ModifiedAt,
	}

	return goaResp
}

// buildContentFromITX combines ITX summary parts into a single content string
func buildContentFromITX(resp *itx.PastMeetingSummaryResponse) string {
	var parts []string

	if resp.SummaryOverview != "" {
		parts = append(parts, resp.SummaryOverview)
	}

	if len(resp.SummaryDetails) > 0 {
		for _, detail := range resp.SummaryDetails {
			if detail.Label != "" && detail.Summary != "" {
				parts = append(parts, fmt.Sprintf("%s: %s", detail.Label, detail.Summary))
			}
		}
	}

	if len(resp.NextSteps) > 0 {
		parts = append(parts, "Next Steps:")
		for _, step := range resp.NextSteps {
			parts = append(parts, fmt.Sprintf("- %s", step))
		}
	}

	return strings.Join(parts, "\n\n")
}

// buildEditedContentFromITX combines ITX edited summary parts into a single edited content string
func buildEditedContentFromITX(resp *itx.PastMeetingSummaryResponse) string {
	var parts []string

	if resp.EditedSummaryOverview != "" {
		parts = append(parts, resp.EditedSummaryOverview)
	}

	if len(resp.EditedSummaryDetails) > 0 {
		for _, detail := range resp.EditedSummaryDetails {
			if detail.Label != "" && detail.Summary != "" {
				parts = append(parts, fmt.Sprintf("%s: %s", detail.Label, detail.Summary))
			}
		}
	}

	if len(resp.EditedNextSteps) > 0 {
		parts = append(parts, "Next Steps:")
		for _, step := range resp.EditedNextSteps {
			parts = append(parts, fmt.Sprintf("- %s", step))
		}
	}

	return strings.Join(parts, "\n\n")
}

// Helper function to convert string to pointer
func ptrString(s string) *string {
	return &s
}

// Helper function to convert bool to pointer
func ptrBool(b bool) *bool {
	return &b
}
