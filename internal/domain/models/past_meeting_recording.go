// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package models

import (
	"time"
)

// PastMeetingRecording represents a recording associated with a past meeting occurrence.
// This model stores metadata about meeting recordings and their associated files.
type PastMeetingRecording struct {
	UID               string              `json:"uid"`
	PastMeetingUID    string              `json:"past_meeting_uid"`
	Platform          string              `json:"platform"`            // Platform name (e.g., "Zoom", "Teams", etc.)
	PlatformMeetingID string              `json:"platform_meeting_id"` // Platform-specific meeting ID
	TotalSize         int64               `json:"total_size"`          // Total size of all recording files
	RecordingCount    int                 `json:"recording_count"`     // Number of recording files
	RecordingFiles    []RecordingFileData `json:"recording_files"`     // Array of recording files
	Sessions          []RecordingSession  `json:"sessions"`            // Array of recording sessions
	CreatedAt         time.Time           `json:"created_at"`
	UpdatedAt         time.Time           `json:"updated_at"`
}

// RecordingSession represents a single meeting session for a recording.
// Starting then ending meeting is one session, but restarting the meeting counts as a new session.
type RecordingSession struct {
	UUID      string    `json:"uuid"`       // UUID of the session (matches meeting UUID)
	ShareURL  string    `json:"share_url"`  // Share URL of the session
	TotalSize int64     `json:"total_size"` // Total size of the session in bytes
	StartTime time.Time `json:"start_time"` // Start time of the session
}

// RecordingFileData represents individual recording file metadata.
// This corresponds to the recording files provided by various meeting platforms.
type RecordingFileData struct {
	ID                string    `json:"id"`                  // Unique file ID from platform
	PlatformMeetingID string    `json:"platform_meeting_id"` // Platform meeting ID
	RecordingStart    time.Time `json:"recording_start"`     // When this file's recording started
	RecordingEnd      time.Time `json:"recording_end"`       // When this file's recording ended
	FileType          string    `json:"file_type"`           // "MP4", "M4A", "TIMELINE", "TRANSCRIPT", etc.
	FileSize          int64     `json:"file_size"`           // Size in bytes
	PlayURL           string    `json:"play_url"`            // URL for playing the file
	DownloadURL       string    `json:"download_url"`        // URL for downloading the file
	Status            string    `json:"status"`              // "completed", "processing", etc.
	RecordingType     string    `json:"recording_type"`      // "shared_screen_with_speaker_view", "audio_only", etc.
}

// AddRecordingSession adds a new recording session to the existing recording.
// This is used when multiple recording completion events are received for the same meeting occurrence.
func (r *PastMeetingRecording) AddRecordingSession(newSession RecordingSession) {
	// Create a map of existing sessions by UUID to avoid duplicates
	existingSessions := make(map[string]bool)
	for _, session := range r.Sessions {
		existingSessions[session.UUID] = true
	}

	// Add only if the session doesn't already exist
	if !existingSessions[newSession.UUID] {
		r.Sessions = append(r.Sessions, newSession)
		r.UpdatedAt = time.Now().UTC()
	}
}

// AddRecordingFiles adds new recording files to the existing recording.
// This is used when multiple recording completion events are received for the same meeting occurrence.
func (r *PastMeetingRecording) AddRecordingFiles(newFiles []RecordingFileData) {
	// Create a map of existing files by ID to avoid duplicates
	existingFiles := make(map[string]bool)
	for _, file := range r.RecordingFiles {
		existingFiles[file.ID] = true
	}

	// Add only new files that don't already exist
	for _, newFile := range newFiles {
		if !existingFiles[newFile.ID] {
			r.RecordingFiles = append(r.RecordingFiles, newFile)
		}
	}

	// Update counts and timestamps
	r.RecordingCount = len(r.RecordingFiles)
	r.UpdatedAt = time.Now().UTC()

	// Recalculate total size
	r.TotalSize = 0
	for _, file := range r.RecordingFiles {
		r.TotalSize += file.FileSize
	}
}
