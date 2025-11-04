// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package main

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-meeting-service/cmd/meeting-api/service"
	meetingsvc "github.com/linuxfoundation/lfx-v2-meeting-service/gen/meeting_service"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
)

// GetPastMeetingAttachments gets all attachments for a past meeting
func (s *MeetingsAPI) GetPastMeetingAttachments(ctx context.Context, payload *meetingsvc.GetPastMeetingAttachmentsPayload) (*meetingsvc.GetPastMeetingAttachmentsResult, error) {
	if payload == nil || payload.UID == nil {
		return nil, handleError(domain.NewValidationError("validation failed"))
	}

	attachments, err := s.pastMeetingAttachmentService.ListPastMeetingAttachments(ctx, *payload.UID)
	if err != nil {
		return nil, handleError(err)
	}

	var responseAttachments []*meetingsvc.PastMeetingAttachment
	for _, attachment := range attachments {
		responseAttachments = append(responseAttachments, service.ConvertDomainToPastMeetingAttachmentResponse(attachment))
	}

	return &meetingsvc.GetPastMeetingAttachmentsResult{
		Attachments:  responseAttachments,
		CacheControl: nil,
	}, nil
}
