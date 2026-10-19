// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package main

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-meeting-service/cmd/meeting-api/service"
	meetingsvc "github.com/linuxfoundation/lfx-v2-meeting-service/gen/meeting_service"
	itxservice "github.com/linuxfoundation/lfx-v2-meeting-service/internal/service/itx"
)

// GetItxPastMeetingChat returns the chat saved with a past meeting's recording.
// Authorization is Heimdall's recording_viewer check on the past meeting (chart
// ruleset); the service reads the recording record and fetches the chat files.
func (s *MeetingsAPI) GetItxPastMeetingChat(ctx context.Context, p *meetingsvc.GetItxPastMeetingChatPayload) (*meetingsvc.PastMeetingChat, error) {
	resp, err := s.itxPastMeetingChatService.GetPastMeetingChat(ctx, p.MeetingAndOccurrenceID, itxservice.PastMeetingChatWindow{
		Limit:  p.Limit,
		Offset: p.Offset,
	})
	if err != nil {
		return nil, handleError(ctx, err)
	}
	return service.ConvertPastMeetingChatToGoa(resp), nil
}
