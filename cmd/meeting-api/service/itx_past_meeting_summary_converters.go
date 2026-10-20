// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"strings"

	meetingservice "github.com/linuxfoundation/lfx-v2-meeting-service/gen/meeting_service"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/models/itx"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/summaryformat"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/utils"
)

// ConvertUpdatePastMeetingSummaryPayload converts V2 Goa payload to ITX update request.
//
// V2 has a single edited_content field in the summary markdown (pkg/summaryformat); ITX
// has three edited fields. When an edit is present, all three are sent explicitly, with
// "" and [] for the parts the edit cleared, so ITX replaces the whole edited summary.
// When edited_content is absent, empty or whitespace, none of them is sent: an
// approval-only update must leave the edits in ITX untouched.
func ConvertUpdatePastMeetingSummaryPayload(payload *meetingservice.UpdateItxPastMeetingSummaryPayload) *itx.UpdatePastMeetingSummaryRequest {
	req := &itx.UpdatePastMeetingSummaryRequest{}

	if payload.EditedContent != nil && strings.TrimSpace(*payload.EditedContent) != "" {
		parts := summaryformat.Parse(*payload.EditedContent)
		details := parts.Details
		if details == nil {
			details = []itx.ZoomMeetingSummaryDetails{}
		}
		nextSteps := parts.NextSteps
		if nextSteps == nil {
			nextSteps = []string{}
		}
		req.EditedSummaryOverview = &parts.Overview
		req.EditedSummaryDetails = &details
		req.EditedNextSteps = &nextSteps
	}
	if payload.Approved != nil {
		req.Approved = payload.Approved
	}
	// ModifiedBy is stamped by PastMeetingSummaryService from the authenticated
	// principal (see internal/service/itx/past_meeting_summary_service.go); we don't
	// try to derive it from the JWT here because ITX persists whatever the caller
	// sends and leaving it blank produces a null audit entry.

	return req
}

// ConvertPastMeetingSummaryToGoa converts ITX response to V2 Goa type. The summary parts
// are rendered as the summary markdown (pkg/summaryformat), the same text the search
// index carries in content/edited_content.
func ConvertPastMeetingSummaryToGoa(resp *itx.PastMeetingSummaryResponse) *meetingservice.PastMeetingSummary {
	content := summaryformat.Format(summaryformat.Parts{
		Overview:  resp.SummaryOverview,
		Details:   resp.SummaryDetails,
		NextSteps: resp.NextSteps,
	})
	editedContent := summaryformat.Format(summaryformat.Parts{
		Overview:  resp.EditedSummaryOverview,
		Details:   resp.EditedSummaryDetails,
		NextSteps: resp.EditedNextSteps,
	})

	// Create the summary_data object (start_time and end_time are required)
	summaryData := &meetingservice.SummaryData{
		StartTime:     resp.SummaryStartTime,
		EndTime:       resp.SummaryEndTime,
		Title:         utils.StringPtrOmitEmpty(resp.SummaryTitle),
		Content:       utils.StringPtrOmitEmpty(content),
		DocURL:        utils.StringPtrOmitEmpty(""), // ITX doesn't provide doc_url
		EditedContent: utils.StringPtrOmitEmpty(editedContent),
	}

	// Build Zoom config if available
	var zoomConfig *meetingservice.PastMeetingSummaryZoomConfig
	if resp.MeetingID != "" || resp.ZoomMeetingUUID != "" {
		zoomConfig = &meetingservice.PastMeetingSummaryZoomConfig{
			MeetingID:   utils.StringPtrOmitEmpty(resp.MeetingID),
			MeetingUUID: utils.StringPtrOmitEmpty(resp.ZoomMeetingUUID),
		}
	}

	// Create the V2-style response (required fields are non-pointer strings and bools).
	// email_sent is optional: the ITX pointer is carried through unchanged, so the field
	// is present only when ITX returned it and a `false` ITX sent is kept.
	goaResp := &meetingservice.PastMeetingSummary{
		UID:              resp.ID,
		PastMeetingID:    resp.MeetingAndOccurrenceID,
		MeetingID:        resp.MeetingID,
		Platform:         "Zoom",
		Password:         utils.StringPtrOmitEmpty(""),
		ZoomConfig:       zoomConfig,
		SummaryData:      summaryData,
		RequiresApproval: resp.RequiresApproval,
		Approved:         resp.Approved,
		EmailSent:        resp.EmailSent,
		CreatedAt:        resp.CreatedAt,
		UpdatedAt:        resp.ModifiedAt,
	}

	return goaResp
}
