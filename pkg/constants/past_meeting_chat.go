// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package constants

import "time"

// Bounds of the past-meeting saved-chat route (GET /itx/past_meetings/{id}/chat).
// The fetch bounds match those LFX Self Serve applies to the transcript file.
const (
	// PastMeetingChatFetchTimeout bounds the fetch of one CHAT file, body included.
	PastMeetingChatFetchTimeout = 15 * time.Second

	// PastMeetingChatMaxBytes caps the bytes read from one CHAT file; a longer file
	// is cut at the last complete line and the session is marked truncated.
	PastMeetingChatMaxBytes int64 = 5 * 1024 * 1024

	// PastMeetingChatDefaultLimit is the message window size when limit is omitted.
	PastMeetingChatDefaultLimit = 500

	// PastMeetingChatMaxLimit is the largest accepted limit.
	PastMeetingChatMaxLimit = 2000

	// PastMeetingChatMaxIDLength bounds the meeting_and_occurrence_id path parameter.
	PastMeetingChatMaxIDLength = 128

	// V1ObjectsBucket is the JetStream KV bucket that holds the v1 records this
	// service reads on demand (the event pipeline watches the same bucket).
	V1ObjectsBucket = "v1-objects"
)
