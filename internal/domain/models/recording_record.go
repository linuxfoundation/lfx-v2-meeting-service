// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package models

// RecordingRecord is the past meeting recording record as read on demand from the
// v1-objects KV bucket (`itx-zoom-past-meetings-recordings.{meeting_and_occurrence_id}`),
// reduced to what API routes need. There is one record per past meeting.
//
// The file entries carry the provider's bearer links (download_url, play_url) and the
// sessions may carry a share link: those are inputs to a server-side fetch and are
// never part of an API response. The indexer path does not use this type; it keeps
// its own RecordingEventData.
type RecordingRecord struct {
	MeetingAndOccurrenceID string
	// RecordingAccess is the record's own snapshot ("public", "meeting_hosts",
	// "meeting_participants"). It is informational here: route authorization is
	// Heimdall's, on the recording_viewer relation.
	RecordingAccess string
	RecordingFiles  []RecordingFile
	Sessions        []RecordingSession
}
