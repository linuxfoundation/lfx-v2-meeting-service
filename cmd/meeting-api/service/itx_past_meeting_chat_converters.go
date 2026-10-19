// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"time"

	meetingservice "github.com/linuxfoundation/lfx-v2-meeting-service/gen/meeting_service"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/models/itx"
)

// ConvertPastMeetingChatToGoa converts the service's saved-chat response to the Goa
// type. Per the contract (docs/api-contracts/itx-past-meeting-chat-api.md) the
// session timestamps are omitted when the recording record carried no parsable
// value, and message_count, truncated and messages are always present; messages is
// an empty array, never null.
func ConvertPastMeetingChatToGoa(resp *itx.PastMeetingChatResponse) *meetingservice.PastMeetingChat {
	out := &meetingservice.PastMeetingChat{
		MeetingAndOccurrenceID: resp.MeetingAndOccurrenceID,
		Sessions:               make([]*meetingservice.PastMeetingChatSession, 0, len(resp.Sessions)),
	}
	for _, s := range resp.Sessions {
		messages := make([]*meetingservice.PastMeetingChatMessage, 0, len(s.Messages))
		for _, m := range s.Messages {
			messages = append(messages, &meetingservice.PastMeetingChatMessage{
				Offset:    m.Offset,
				Sender:    m.Sender,
				Recipient: m.Recipient,
				Text:      m.Text,
			})
		}
		out.Sessions = append(out.Sessions, &meetingservice.PastMeetingChatSession{
			RecordingStart: rfc3339PtrOmitZero(s.RecordingStart),
			RecordingEnd:   rfc3339PtrOmitZero(s.RecordingEnd),
			MessageCount:   s.MessageCount,
			Truncated:      s.Truncated,
			Messages:       messages,
		})
	}
	return out
}

// rfc3339PtrOmitZero formats t as RFC3339 UTC, or returns nil for a zero time.
func rfc3339PtrOmitZero(t time.Time) *string {
	if t.IsZero() {
		return nil
	}
	s := t.UTC().Format(time.RFC3339)
	return &s
}
