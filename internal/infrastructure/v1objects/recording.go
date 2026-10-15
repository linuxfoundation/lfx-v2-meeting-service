// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// Package v1objects holds the raw shapes of records stored in the v1-objects
// NATS KV bucket that more than one part of this service decodes, together
// with the coercion helpers their JSON decoding needs.
//
// The v1 records come from DynamoDB by way of the v1 sync and are loosely
// typed: numeric fields may arrive as numbers or as strings, and timestamps in
// several layouts. The types here absorb that so every reader of a record
// decodes it the same way. The event pipeline in cmd/meeting-api/eventing is
// the original consumer; the past-meeting chat route reads the recording record
// on demand through internal/infrastructure/nats.
package v1objects

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
)

// Recording is the raw past meeting recording record
// (`itx-zoom-past-meetings-recordings.{meeting_and_occurrence_id}`).
type Recording struct {
	// MeetingAndOccurrenceID is the primary key of the recording table since there is only one recording record for a past meeting.
	MeetingAndOccurrenceID string `json:"meeting_and_occurrence_id"`

	// ProjectID is the ID of the project associated with the recording.
	ProjectID string `json:"proj_id"`

	// ProjectSlug is the slug of the project associated with the recording.
	ProjectSlug string `json:"project_slug"`

	// HostEmail is the email of the host of the recorded meeting. This comes from Zoom.
	HostEmail string `json:"host_email"`

	// HostID is the Zoom user ID of the host of the recorded meeting. This comes from Zoom.
	HostID string `json:"host_id"`

	// MeetingID is the ID of the meeting associated with the recording.
	MeetingID string `json:"meeting_id"`

	// OccurrenceID is the ID of the occurrence associated with the recording.
	OccurrenceID string `json:"occurrence_id"`

	// RecordingAccess is the access type of the recording.
	RecordingAccess string `json:"recording_access"`

	// Topic is the topic of the recorded meeting.
	Topic string `json:"topic"`

	// TranscriptAccess is the access type of the transcript of the recording.
	TranscriptAccess string `json:"transcript_access"`

	// TranscriptEnabled is whether the transcript of the recording is enabled.
	TranscriptEnabled bool `json:"transcript_enabled"`

	// Visibility is the visibility of the recording on the LFX platform.
	Visibility string `json:"visibility"`

	// RecordingCount is the number of recording files in the recording.
	// A recording record can have many files due to there being multiple sessions of the same meeting,
	// and the fact that each session has an MP4 file, M4A file, and optionally a VTT and JSON file
	// if there is a transcript available.
	RecordingCount int `json:"recording_count"`

	// RecordingFiles is the list of files in the recording.
	RecordingFiles []RecordingFile `json:"recording_files"`

	// Sessions is the list of sessions in the recording.
	// There can be multiple sessions in a recording due to the fact that a meeting can be restarted
	// and that is considered a new session in Zoom.
	Sessions []RecordingSession `json:"sessions"`

	// StartTime is the start time of the recording in RFC3339 format.
	StartTime string `json:"start_time"`

	// TotalSize is the total size of the recording in bytes.
	TotalSize int64 `json:"total_size"`

	// CreatedAt is the creation time of the recording in RFC3339 format.
	CreatedAt string `json:"created_at"`

	// ModifiedAt is the last modification time of the recording in RFC3339 format.
	ModifiedAt string `json:"modified_at"`

	// CreatedBy is the user who created the recording record in this system.
	CreatedBy models.CreatedBy `json:"created_by"`

	// UpdatedBy is the user who last updated the recording record in this system.
	UpdatedBy models.UpdatedBy `json:"updated_by"`
}

// UnmarshalJSON implements custom unmarshaling to handle both string and number inputs for numeric fields.
func (r *Recording) UnmarshalJSON(data []byte) error {
	type Alias Recording
	tmp := struct {
		RecordingCount interface{} `json:"recording_count"`
		TotalSize      interface{} `json:"total_size"`
		*Alias
	}{
		Alias: (*Alias)(r),
	}

	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}

	if err := CoerceInt(&r.RecordingCount, tmp.RecordingCount, "recording_count"); err != nil {
		return err
	}
	if err := CoerceInt64(&r.TotalSize, tmp.TotalSize, "total_size"); err != nil {
		return err
	}
	return nil
}

// RecordingFile is one file entry of a raw recording record. DownloadURL and
// PlayURL are bearer links copied from the provider: they are read to fetch a
// file server side and must never be returned to a client or logged.
type RecordingFile struct {
	DownloadURL    string `json:"download_url,omitempty"`
	FileExtension  string `json:"file_extension"`
	FileSize       int64  `json:"file_size"`
	FileType       string `json:"file_type"`
	ID             string `json:"id"`
	MeetingID      string `json:"meeting_id"`
	PlayURL        string `json:"play_url,omitempty"`
	RecordingStart string `json:"recording_start"`
	RecordingEnd   string `json:"recording_end"`
	RecordingType  string `json:"recording_type"`
	Status         string `json:"status"`
}

// UnmarshalJSON implements custom unmarshaling to handle both string and number inputs for numeric fields.
func (r *RecordingFile) UnmarshalJSON(data []byte) error {
	type Alias RecordingFile
	tmp := struct {
		FileSize interface{} `json:"file_size"`
		*Alias
	}{
		Alias: (*Alias)(r),
	}

	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}

	if err := CoerceInt64(&r.FileSize, tmp.FileSize, "file_size"); err != nil {
		return err
	}
	return nil
}

// RecordingSession is one session entry of a raw recording record. ShareURL
// and Password are provider credentials: neither is indexed nor returned.
type RecordingSession struct {
	UUID      string `json:"uuid"`
	ShareURL  string `json:"share_url"`
	TotalSize int64  `json:"total_size"`
	StartTime string `json:"start_time"`
	Password  string `json:"password"`
}

// UnmarshalJSON implements custom unmarshaling to handle both string and number inputs for numeric fields.
func (r *RecordingSession) UnmarshalJSON(data []byte) error {
	type Alias RecordingSession
	tmp := struct {
		TotalSize interface{} `json:"total_size"`
		*Alias
	}{
		Alias: (*Alias)(r),
	}

	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}

	if err := CoerceInt64(&r.TotalSize, tmp.TotalSize, "total_size"); err != nil {
		return err
	}
	return nil
}

// CoerceInt decodes a JSON-decoded interface{} into *dest.
// Accepts string (numeric or empty), float64, int, and nil; returns an error for any other type.
func CoerceInt(dest *int, v interface{}, field string) error {
	switch val := v.(type) {
	case string:
		if val == "" {
			return nil
		}
		n, err := strconv.Atoi(val)
		if err != nil {
			return fmt.Errorf("invalid value for %s: %w", field, err)
		}
		*dest = n
	case float64:
		*dest = int(val)
	case int:
		*dest = val
	case nil:
		// leave dest unchanged
	default:
		return fmt.Errorf("invalid type for %s: %T", field, v)
	}
	return nil
}

// CoerceInt64 decodes a JSON-decoded interface{} into *dest.
// Accepts string (numeric or empty), float64, int64, int, and nil; returns an error for any other type.
func CoerceInt64(dest *int64, v interface{}, field string) error {
	switch val := v.(type) {
	case string:
		if val == "" {
			return nil
		}
		n, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid value for %s: %w", field, err)
		}
		*dest = n
	case float64:
		*dest = int64(val)
	case int64:
		*dest = val
	case int:
		*dest = int64(val)
	case nil:
		// leave dest unchanged
	default:
		return fmt.Errorf("invalid type for %s: %T", field, v)
	}
	return nil
}

// ParseTime parses the timestamp layouts found in v1 records: RFC3339 first,
// then ISO 8601 without offset, with milliseconds, and space-separated.
func ParseTime(timeStr string) (time.Time, error) {
	if timeStr == "" {
		return time.Time{}, fmt.Errorf("empty time string")
	}

	// Try RFC3339 first
	t, err := time.Parse(time.RFC3339, timeStr)
	if err == nil {
		return t, nil
	}

	// Try ISO 8601
	t, err = time.Parse("2006-01-02T15:04:05Z", timeStr)
	if err == nil {
		return t, nil
	}

	// Try with milliseconds
	t, err = time.Parse("2006-01-02T15:04:05.000Z", timeStr)
	if err == nil {
		return t, nil
	}

	// Try space-separated format
	t, err = time.Parse("2006-01-02 15:04:05", timeStr)
	if err == nil {
		return t, nil
	}

	return time.Time{}, fmt.Errorf("unable to parse time: %s", timeStr)
}
