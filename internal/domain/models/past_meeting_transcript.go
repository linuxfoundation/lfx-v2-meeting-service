// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package models

import (
	"fmt"
	"time"
)

// PastMeetingTranscript represents a transcript associated with a past meeting occurrence.
// This model stores metadata about meeting transcripts and their associated files.
type PastMeetingTranscript struct {
	UID                       string               `json:"uid"`
	PastMeetingUID            string               `json:"past_meeting_uid"`
	Platform                  string               `json:"platform"`                       // Platform name (e.g., "Zoom", "Teams", etc.)
	PlatformMeetingID         string               `json:"platform_meeting_id"`            // Platform-specific meeting ID
	PlatformMeetingInstanceID string               `json:"platform_meeting_instance_id"`   // Platform-specific meeting instance ID (e.g., Zoom UUID)
	TotalSize                 int64                `json:"total_size"`                     // Total size of all transcript files
	TranscriptCount           int                  `json:"transcript_count"`               // Number of transcript files
	TranscriptFiles           []TranscriptFileData `json:"transcript_files"`               // Array of transcript files
	Sessions                  []TranscriptSession  `json:"sessions"`                       // Array of transcript sessions (kept for backward compatibility)
	CreatedAt                 time.Time            `json:"created_at"`
	UpdatedAt                 time.Time            `json:"updated_at"`
}

// TranscriptSession represents a single meeting session for a transcript.
// Starting then ending meeting is one session, but restarting the meeting counts as a new session.
type TranscriptSession struct {
	UUID      string    `json:"uuid"`       // UUID of the session (matches meeting UUID)
	ShareURL  string    `json:"share_url"`  // Share URL of the session
	TotalSize int64     `json:"total_size"` // Total size of the session in bytes
	StartTime time.Time `json:"start_time"` // Start time of the session
}

// TranscriptFileData represents individual transcript file metadata.
// This corresponds to the transcript files provided by various meeting platforms.
type TranscriptFileData struct {
	ID                string    `json:"id"`                  // Unique file ID from platform
	PlatformMeetingID string    `json:"platform_meeting_id"` // Platform meeting ID
	RecordingStart    time.Time `json:"recording_start"`     // When this file's recording started
	RecordingEnd      time.Time `json:"recording_end"`       // When this file's recording ended
	FileType          string    `json:"file_type"`           // "TRANSCRIPT", "TIMELINE", etc.
	FileSize          int64     `json:"file_size"`           // Size in bytes
	PlayURL           string    `json:"play_url"`            // URL for viewing the transcript
	DownloadURL       string    `json:"download_url"`        // URL for downloading the transcript
	Status            string    `json:"status"`
	RecordingType     string    `json:"recording_type"`
}

// Tags generates a consistent set of tags for the past meeting transcript.
// IMPORTANT: If you modify this method, please update the Meeting Tags documentation in the README.md
// to ensure consumers understand how to use these tags for searching.
func (p *PastMeetingTranscript) Tags() []string {
	tags := []string{}

	if p == nil {
		return nil
	}

	if p.UID != "" {
		// without prefix
		tags = append(tags, p.UID)
		// with prefix
		tag := fmt.Sprintf("past_meeting_transcript_uid:%s", p.UID)
		tags = append(tags, tag)
	}

	if p.PastMeetingUID != "" {
		tag := fmt.Sprintf("past_meeting_uid:%s", p.PastMeetingUID)
		tags = append(tags, tag)
	}

	if p.Platform != "" {
		tag := fmt.Sprintf("platform:%s", p.Platform)
		tags = append(tags, tag)
	}

	if p.PlatformMeetingID != "" {
		tag := fmt.Sprintf("platform_meeting_id:%s", p.PlatformMeetingID)
		tags = append(tags, tag)
	}

	if p.PlatformMeetingInstanceID != "" {
		tag := fmt.Sprintf("platform_meeting_instance_id:%s", p.PlatformMeetingInstanceID)
		tags = append(tags, tag)
	}

	return tags
}

// AddTranscriptSession adds a new transcript session to the existing transcript.
// This is used when multiple transcript completion events are received for the same meeting occurrence.
func (t *PastMeetingTranscript) AddTranscriptSession(newSession TranscriptSession) {
	// Create a map of existing sessions by UUID to avoid duplicates
	existingSessions := make(map[string]bool)
	for _, session := range t.Sessions {
		existingSessions[session.UUID] = true
	}

	// Add only if the session doesn't already exist
	if !existingSessions[newSession.UUID] {
		t.Sessions = append(t.Sessions, newSession)
		t.UpdatedAt = time.Now().UTC()
	}
}

// AddTranscriptFiles adds new transcript files to the existing transcript.
// This is used when multiple transcript completion events are received for the same meeting occurrence.
func (t *PastMeetingTranscript) AddTranscriptFiles(newFiles []TranscriptFileData) {
	// Create a map of existing files by ID to avoid duplicates
	existingFiles := make(map[string]bool)
	for _, file := range t.TranscriptFiles {
		existingFiles[file.ID] = true
	}

	// Add only new files that don't already exist
	for _, newFile := range newFiles {
		if !existingFiles[newFile.ID] {
			t.TranscriptFiles = append(t.TranscriptFiles, newFile)
		}
	}

	// Update counts and timestamps
	t.TranscriptCount = len(t.TranscriptFiles)
	t.UpdatedAt = time.Now().UTC()

	// Recalculate total size
	t.TotalSize = 0
	for _, file := range t.TranscriptFiles {
		t.TotalSize += file.FileSize
	}
}
