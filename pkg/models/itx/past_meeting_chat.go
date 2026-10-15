// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package itx

import "time"

// PastMeetingChatResponse is the proxy's own response for
// GET /itx/past_meetings/{meeting_and_occurrence_id}/chat. Unlike the other types
// in this package it is not an ITX wire shape: the chat is fetched from the
// recording provider and parsed by this service.
//
// By contract the response carries no link and no credential: no download_url,
// play_url, share_url, password or passcode, and no host identifier. Keep it that
// way; a test marshals it from a record that has all of those set and asserts
// none appears.
type PastMeetingChatResponse struct {
	MeetingAndOccurrenceID string                   `json:"meeting_and_occurrence_id"`
	Sessions               []PastMeetingChatSession `json:"sessions"`
}

// PastMeetingChatSession is the chat of one recording session (one CHAT file),
// ordered by RecordingStart within the response.
type PastMeetingChatSession struct {
	// RecordingStart and RecordingEnd come from the CHAT file entry of the recording
	// record; a zero value means the record did not carry a parsable timestamp.
	RecordingStart time.Time `json:"recording_start"`
	RecordingEnd   time.Time `json:"recording_end"`
	// MessageCount is the number of messages parsed from the file, before the window.
	MessageCount int `json:"message_count"`
	// Truncated is true when the byte cap cut the file or the window ends before
	// the last message.
	Truncated bool `json:"truncated"`
	// Messages is the window [offset, offset+limit) of the parsed messages. Never nil.
	Messages []PastMeetingChatMessage `json:"messages"`
}

// PastMeetingChatMessage is one chat message. Text is data: returned as the
// provider wrote it, never interpreted.
type PastMeetingChatMessage struct {
	Offset    string `json:"offset"`
	Sender    string `json:"sender"`
	Recipient string `json:"recipient"`
	Text      string `json:"text"`
}
