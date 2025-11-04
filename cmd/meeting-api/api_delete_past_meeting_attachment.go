// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package main

import (
	"context"

	meetingsvc "github.com/linuxfoundation/lfx-v2-meeting-service/gen/meeting_service"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
)

// DeletePastMeetingAttachment deletes a past meeting attachment
func (s *MeetingsAPI) DeletePastMeetingAttachment(ctx context.Context, payload *meetingsvc.DeletePastMeetingAttachmentPayload) error {
	if payload == nil {
		return handleError(domain.NewValidationError("validation failed"))
	}

	err := s.pastMeetingAttachmentService.DeletePastMeetingAttachment(ctx, payload.PastMeetingUID, payload.UID)
	if err != nil {
		return handleError(err)
	}

	return nil
}
