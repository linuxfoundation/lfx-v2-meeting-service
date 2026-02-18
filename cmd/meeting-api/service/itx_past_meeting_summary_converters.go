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
	// Parse the edited_content into overview, details, and next steps
	if payload.EditedContent != nil && *payload.EditedContent != "" {
		overview, details, nextSteps := parseContentIntoITXParts(*payload.EditedContent)
		req.EditedSummaryOverview = overview
		req.EditedSummaryDetails = details
		req.EditedNextSteps = nextSteps
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

// parseContentIntoITXParts parses a V2 content string into ITX overview, details, and next steps
// This is a best-effort parser that handles common patterns but may not be perfect for all cases
func parseContentIntoITXParts(content string) (overview string, details []itx.ZoomMeetingSummaryDetails, nextSteps []string) {
	if content == "" {
		return "", nil, nil
	}

	// Split content into paragraphs (separated by double newlines)
	paragraphs := strings.Split(content, "\n\n")

	var overviewParts []string
	var inNextSteps bool

	for _, para := range paragraphs {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}

		// Check if this is the "Next Steps:" section
		if strings.HasPrefix(para, "Next Steps:") {
			inNextSteps = true
			// Extract next steps from this paragraph
			lines := strings.Split(para, "\n")
			for i, line := range lines {
				if i == 0 {
					continue // Skip "Next Steps:" header
				}
				line = strings.TrimSpace(line)
				// Remove leading dash and whitespace
				line = strings.TrimPrefix(line, "-")
				line = strings.TrimPrefix(line, "•")
				line = strings.TrimSpace(line)
				if line != "" {
					nextSteps = append(nextSteps, line)
				}
			}
			continue
		}

		if inNextSteps {
			// After "Next Steps:", treat each line as a next step
			lines := strings.Split(para, "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				line = strings.TrimPrefix(line, "-")
				line = strings.TrimPrefix(line, "•")
				line = strings.TrimSpace(line)
				if line != "" {
					nextSteps = append(nextSteps, line)
				}
			}
			continue
		}

		// Check if this paragraph contains "Label: Summary" patterns (details)
		lines := strings.Split(para, "\n")
		hasLabelPattern := false
		for _, line := range lines {
			if strings.Contains(line, ":") && !strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "•") {
				// This looks like a "Label: Summary" pattern
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					label := strings.TrimSpace(parts[0])
					summary := strings.TrimSpace(parts[1])
					if label != "" && summary != "" {
						details = append(details, itx.ZoomMeetingSummaryDetails{
							Label:   label,
							Summary: summary,
						})
						hasLabelPattern = true
					}
				}
			}
		}

		// If this paragraph doesn't have label patterns, treat as overview
		if !hasLabelPattern {
			overviewParts = append(overviewParts, para)
		}
	}

	// Combine overview parts
	if len(overviewParts) > 0 {
		overview = strings.Join(overviewParts, "\n\n")
	}

	return overview, details, nextSteps
}

// Helper function to convert bool to pointer
func ptrBool(b bool) *bool {
	return &b
}
