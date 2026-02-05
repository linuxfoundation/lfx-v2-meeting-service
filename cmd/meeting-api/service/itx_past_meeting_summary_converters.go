// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	meetingservice "github.com/linuxfoundation/lfx-v2-meeting-service/gen/meeting_service"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/models/itx"
)

// ConvertUpdatePastMeetingSummaryPayload converts Goa payload to ITX update past meeting summary request
func ConvertUpdatePastMeetingSummaryPayload(payload *meetingservice.UpdateItxPastMeetingSummaryPayload) *itx.UpdatePastMeetingSummaryRequest {
	req := &itx.UpdatePastMeetingSummaryRequest{}

	// Optional fields - only set if provided
	if payload.EditedSummaryOverview != nil {
		req.EditedSummaryOverview = *payload.EditedSummaryOverview
	}
	if payload.EditedSummaryDetails != nil {
		req.EditedSummaryDetails = make([]itx.ZoomMeetingSummaryDetails, len(payload.EditedSummaryDetails))
		for i, d := range payload.EditedSummaryDetails {
			req.EditedSummaryDetails[i] = itx.ZoomMeetingSummaryDetails{
				Label:   ptrToString(d.Label),
				Summary: ptrToString(d.Summary),
			}
		}
	}
	if payload.EditedNextSteps != nil {
		req.EditedNextSteps = payload.EditedNextSteps
	}
	if payload.Approved != nil {
		req.Approved = payload.Approved
	}
	// Note: ModifiedBy is derived from JWT token in the ITX service, not from payload

	return req
}

// ConvertPastMeetingSummaryToGoa converts ITX past meeting summary response to Goa type
func ConvertPastMeetingSummaryToGoa(resp *itx.PastMeetingSummaryResponse) *meetingservice.ITXPastMeetingSummary {
	goaResp := &meetingservice.ITXPastMeetingSummary{
		// Identifiers
		ID:                     ptrIfNotEmpty(resp.ID),
		MeetingAndOccurrenceID: ptrIfNotEmpty(resp.MeetingAndOccurrenceID),
		MeetingID:              ptrIfNotEmpty(resp.MeetingID),
		OccurrenceID:           ptrIfNotEmpty(resp.OccurrenceID),
		ZoomMeetingUUID:        ptrIfNotEmpty(resp.ZoomMeetingUUID),

		// Summary metadata
		SummaryCreatedTime:      ptrIfNotEmpty(resp.SummaryCreatedTime),
		SummaryLastModifiedTime: ptrIfNotEmpty(resp.SummaryLastModifiedTime),
		SummaryStartTime:        ptrIfNotEmpty(resp.SummaryStartTime),
		SummaryEndTime:          ptrIfNotEmpty(resp.SummaryEndTime),

		// Original Zoom AI summary
		SummaryTitle:    ptrIfNotEmpty(resp.SummaryTitle),
		SummaryOverview: ptrIfNotEmpty(resp.SummaryOverview),
		NextSteps:       resp.NextSteps,

		// Edited versions
		EditedSummaryOverview: ptrIfNotEmpty(resp.EditedSummaryOverview),
		EditedNextSteps:       resp.EditedNextSteps,

		// Approval workflow
		RequiresApproval: ptrBool(resp.RequiresApproval),
		Approved:         ptrBool(resp.Approved),

		// Audit fields
		CreatedAt:  ptrIfNotEmpty(resp.CreatedAt),
		ModifiedAt: ptrIfNotEmpty(resp.ModifiedAt),
	}

	// Convert summary details
	if resp.SummaryDetails != nil {
		goaResp.SummaryDetails = make([]*meetingservice.ZoomMeetingSummaryDetails, len(resp.SummaryDetails))
		for i, d := range resp.SummaryDetails {
			goaResp.SummaryDetails[i] = &meetingservice.ZoomMeetingSummaryDetails{
				Label:   ptrIfNotEmpty(d.Label),
				Summary: ptrIfNotEmpty(d.Summary),
			}
		}
	}

	// Convert edited summary details
	if resp.EditedSummaryDetails != nil {
		goaResp.EditedSummaryDetails = make([]*meetingservice.ZoomMeetingSummaryDetails, len(resp.EditedSummaryDetails))
		for i, d := range resp.EditedSummaryDetails {
			goaResp.EditedSummaryDetails[i] = &meetingservice.ZoomMeetingSummaryDetails{
				Label:   ptrIfNotEmpty(d.Label),
				Summary: ptrIfNotEmpty(d.Summary),
			}
		}
	}

	// Convert created_by
	if resp.CreatedBy != nil {
		goaResp.CreatedBy = &meetingservice.ITXUser{
			Username:       ptrIfNotEmpty(resp.CreatedBy.Username),
			Name:           ptrIfNotEmpty(resp.CreatedBy.Name),
			Email:          ptrIfNotEmpty(resp.CreatedBy.Email),
			ProfilePicture: ptrIfNotEmpty(resp.CreatedBy.ProfilePicture),
		}
	}

	// Convert modified_by
	if resp.ModifiedBy != nil {
		goaResp.ModifiedBy = &meetingservice.ITXUser{
			Username:       ptrIfNotEmpty(resp.ModifiedBy.Username),
			Name:           ptrIfNotEmpty(resp.ModifiedBy.Name),
			Email:          ptrIfNotEmpty(resp.ModifiedBy.Email),
			ProfilePicture: ptrIfNotEmpty(resp.ModifiedBy.ProfilePicture),
		}
	}

	return goaResp
}

// Helper function to convert pointer to string value
func ptrToString(ptr *string) string {
	if ptr == nil {
		return ""
	}
	return *ptr
}

// Helper function to convert bool to pointer
func ptrBool(b bool) *bool {
	return &b
}
