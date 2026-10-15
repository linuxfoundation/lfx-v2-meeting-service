// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	genserver "github.com/linuxfoundation/lfx-v2-meeting-service/gen/http/meeting_service/server"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/models/itx"
)

func TestConvertPastMeetingChatToGoa(t *testing.T) {
	start := time.Date(2026, 10, 1, 15, 0, 0, 0, time.FixedZone("plus2", 2*3600))
	end := time.Date(2026, 10, 1, 16, 2, 11, 0, time.UTC)

	t.Run("maps sessions and messages, timestamps as RFC3339 UTC", func(t *testing.T) {
		got := ConvertPastMeetingChatToGoa(&itx.PastMeetingChatResponse{
			MeetingAndOccurrenceID: "93699735000:1700000000",
			Sessions: []itx.PastMeetingChatSession{{
				RecordingStart: start,
				RecordingEnd:   end,
				MessageCount:   42,
				Truncated:      true,
				Messages: []itx.PastMeetingChatMessage{
					{Offset: "00:12:34", Sender: "Alice Example", Recipient: "Everyone", Text: "Link to the agenda is in the invite"},
				},
			}},
		})

		assert.Equal(t, "93699735000:1700000000", got.MeetingAndOccurrenceID)
		require.Len(t, got.Sessions, 1)
		s := got.Sessions[0]
		require.NotNil(t, s.RecordingStart)
		assert.Equal(t, "2026-10-01T13:00:00Z", *s.RecordingStart)
		require.NotNil(t, s.RecordingEnd)
		assert.Equal(t, "2026-10-01T16:02:11Z", *s.RecordingEnd)
		assert.Equal(t, 42, s.MessageCount)
		assert.True(t, s.Truncated)
		require.Len(t, s.Messages, 1)
		assert.Equal(t, "00:12:34", s.Messages[0].Offset)
		assert.Equal(t, "Alice Example", s.Messages[0].Sender)
		assert.Equal(t, "Everyone", s.Messages[0].Recipient)
		assert.Equal(t, "Link to the agenda is in the invite", s.Messages[0].Text)
	})

	t.Run("omits zero timestamps and keeps an empty session's messages as an array", func(t *testing.T) {
		got := ConvertPastMeetingChatToGoa(&itx.PastMeetingChatResponse{
			MeetingAndOccurrenceID: "93699735000:1700000000",
			Sessions:               []itx.PastMeetingChatSession{{Messages: []itx.PastMeetingChatMessage{}}},
		})
		require.Len(t, got.Sessions, 1)
		assert.Nil(t, got.Sessions[0].RecordingStart)
		assert.Nil(t, got.Sessions[0].RecordingEnd)
		assert.NotNil(t, got.Sessions[0].Messages)
		assert.Empty(t, got.Sessions[0].Messages)

		// The wire body, as the generated server encodes it.
		out, err := json.Marshal(genserver.NewGetItxPastMeetingChatResponseBody(got))
		require.NoError(t, err)
		assert.JSONEq(t, `{"meeting_and_occurrence_id":"93699735000:1700000000","sessions":[{"message_count":0,"truncated":false,"messages":[]}]}`, string(out))
	})

	t.Run("wire body has snake_case keys and only the contract's fields", func(t *testing.T) {
		got := ConvertPastMeetingChatToGoa(&itx.PastMeetingChatResponse{
			MeetingAndOccurrenceID: "93699735000:1700000000",
			Sessions: []itx.PastMeetingChatSession{{
				RecordingStart: end, RecordingEnd: end, MessageCount: 1,
				Messages: []itx.PastMeetingChatMessage{{Offset: "00:00:01", Sender: "Alice Example", Recipient: "Everyone", Text: "hi"}},
			}},
		})
		out, err := json.Marshal(genserver.NewGetItxPastMeetingChatResponseBody(got))
		require.NoError(t, err)

		var body map[string]any
		require.NoError(t, json.Unmarshal(out, &body))
		assert.ElementsMatch(t, []string{"meeting_and_occurrence_id", "sessions"}, keysOf(body))
		session := body["sessions"].([]any)[0].(map[string]any)
		assert.ElementsMatch(t, []string{"recording_start", "recording_end", "message_count", "truncated", "messages"}, keysOf(session))
		message := session["messages"].([]any)[0].(map[string]any)
		assert.ElementsMatch(t, []string{"offset", "sender", "recipient", "text"}, keysOf(message))

		lower := strings.ToLower(string(out))
		for _, forbidden := range []string{"url", "password", "passcode", "host", "file_size", "\"id\""} {
			assert.NotContains(t, lower, forbidden)
		}
	})
}

func keysOf(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
