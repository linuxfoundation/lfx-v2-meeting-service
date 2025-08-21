// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package models

import (
	"encoding/json"
	"fmt"
	"time"
)

// ZoomMeetingStartedPayload represents the payload for meeting.started webhook events
type ZoomMeetingStartedPayload struct {
	Object struct {
		UUID      string    `json:"uuid"`
		ID        string    `json:"id"` // Zoom sends as string in webhook events
		HostID    string    `json:"host_id"`
		Topic     string    `json:"topic"`
		Type      int       `json:"type"`
		StartTime time.Time `json:"start_time"`
		Timezone  string    `json:"timezone"`
		Duration  int       `json:"duration"`
	} `json:"object"`
}

// ZoomMeetingEndedPayload represents the payload for meeting.ended webhook events
type ZoomMeetingEndedPayload struct {
	Object struct {
		UUID      string    `json:"uuid"`
		ID        string    `json:"id"` // Zoom sends as string in webhook events
		HostID    string    `json:"host_id"`
		Topic     string    `json:"topic"`
		Type      int       `json:"type"`
		StartTime time.Time `json:"start_time"`
		EndTime   time.Time `json:"end_time"`
		Duration  int       `json:"duration"`
		Timezone  string    `json:"timezone"`
	} `json:"object"`
}

// ZoomMeetingDeletedPayload represents the payload for meeting.deleted webhook events
type ZoomMeetingDeletedPayload struct {
	Object struct {
		UUID   string `json:"uuid"`
		ID     string `json:"id"` // Zoom sends as string in webhook events
		HostID string `json:"host_id"`
		Topic  string `json:"topic"`
		Type   int    `json:"type"`
	} `json:"object"`
}

// ZoomParticipantJoinedPayload represents the payload for meeting.participant_joined webhook events
type ZoomParticipantJoinedPayload struct {
	Object struct {
		UUID        string    `json:"uuid"`
		ID          string    `json:"id"` // Zoom sends as string for participant events
		HostID      string    `json:"host_id"`
		Topic       string    `json:"topic"`
		Type        int       `json:"type"`
		StartTime   time.Time `json:"start_time"`
		Timezone    string    `json:"timezone"`
		Participant struct {
			UserID            string    `json:"user_id"`
			UserName          string    `json:"user_name"`
			ID                string    `json:"id"`
			JoinTime          time.Time `json:"join_time"`
			Email             string    `json:"email"`
			ParticipantUserID string    `json:"participant_user_id"`
		} `json:"participant"`
	} `json:"object"`
}

// ZoomParticipantLeftPayload represents the payload for meeting.participant_left webhook events
type ZoomParticipantLeftPayload struct {
	Object struct {
		UUID        string    `json:"uuid"`
		ID          string    `json:"id"` // Zoom sends as string for participant events
		HostID      string    `json:"host_id"`
		Topic       string    `json:"topic"`
		Type        int       `json:"type"`
		StartTime   time.Time `json:"start_time"`
		Timezone    string    `json:"timezone"`
		Participant struct {
			UserID            string    `json:"user_id"`
			UserName          string    `json:"user_name"`
			ID                string    `json:"id"`
			LeaveTime         time.Time `json:"leave_time"`
			Duration          int       `json:"duration"`
			Email             string    `json:"email"`
			ParticipantUserID string    `json:"participant_user_id"`
		} `json:"participant"`
	} `json:"object"`
}

// ZoomRecordingCompletedPayload represents the payload for recording.completed webhook events
type ZoomRecordingCompletedPayload struct {
	Object struct {
		UUID           string          `json:"uuid"`
		ID             int64           `json:"id"`
		HostID         string          `json:"host_id"`
		Topic          string          `json:"topic"`
		Type           int             `json:"type"`
		StartTime      time.Time       `json:"start_time"`
		Timezone       string          `json:"timezone"`
		Duration       int             `json:"duration"`
		TotalSize      int64           `json:"total_size"`
		RecordingCount int             `json:"recording_count"`
		RecordingFiles []RecordingFile `json:"recording_files"`
	} `json:"object"`
}

// ZoomTranscriptCompletedPayload represents the payload for recording.transcript_completed webhook events
type ZoomTranscriptCompletedPayload struct {
	Object struct {
		UUID           string          `json:"uuid"`
		ID             int64           `json:"id"`
		HostID         string          `json:"host_id"`
		Topic          string          `json:"topic"`
		Type           int             `json:"type"`
		StartTime      time.Time       `json:"start_time"`
		Timezone       string          `json:"timezone"`
		Duration       int             `json:"duration"`
		RecordingFiles []RecordingFile `json:"recording_files"`
	} `json:"object"`
}

// ZoomSummaryCompletedPayload represents the payload for meeting.summary_completed webhook events
type ZoomSummaryCompletedPayload struct {
	Object struct {
		UUID      string    `json:"uuid"`
		ID        int64     `json:"id"`
		HostID    string    `json:"host_id"`
		Topic     string    `json:"topic"`
		Type      int       `json:"type"`
		StartTime time.Time `json:"start_time"`
		EndTime   time.Time `json:"end_time"`
		Duration  int       `json:"duration"`
		Timezone  string    `json:"timezone"`
		Summary   struct {
			SummaryStartTime time.Time `json:"summary_start_time"`
			SummaryEndTime   time.Time `json:"summary_end_time"`
			NextSteps        []string  `json:"next_steps"`
			KeyPoints        []string  `json:"key_points"`
		} `json:"summary"`
	} `json:"object"`
}

// RecordingFile represents a recording file in webhook payloads
type RecordingFile struct {
	ID             string    `json:"id"`
	MeetingID      string    `json:"meeting_id"`
	RecordingStart time.Time `json:"recording_start"`
	RecordingEnd   time.Time `json:"recording_end"`
	FileType       string    `json:"file_type"`
	FileSize       int64     `json:"file_size"`
	PlayURL        string    `json:"play_url"`
	DownloadURL    string    `json:"download_url"`
	Status         string    `json:"status"`
	RecordingType  string    `json:"recording_type"`
}

// Helper methods to convert from ZoomWebhookEventMessage to typed payloads

// ToMeetingStartedPayload converts the webhook event to a typed meeting started payload
func (z *ZoomWebhookEventMessage) ToMeetingStartedPayload() (*ZoomMeetingStartedPayload, error) {
	if z.EventType != "meeting.started" {
		return nil, fmt.Errorf("invalid event type: expected meeting.started, got %s", z.EventType)
	}

	data, err := json.Marshal(z.Payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	var payload ZoomMeetingStartedPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("failed to unmarshal to meeting started payload: %w", err)
	}

	return &payload, nil
}

// ToMeetingEndedPayload converts the webhook event to a typed meeting ended payload
func (z *ZoomWebhookEventMessage) ToMeetingEndedPayload() (*ZoomMeetingEndedPayload, error) {
	if z.EventType != "meeting.ended" {
		return nil, fmt.Errorf("invalid event type: expected meeting.ended, got %s", z.EventType)
	}

	data, err := json.Marshal(z.Payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	var payload ZoomMeetingEndedPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("failed to unmarshal to meeting ended payload: %w", err)
	}

	return &payload, nil
}

// ToMeetingDeletedPayload converts the webhook event to a typed meeting deleted payload
func (z *ZoomWebhookEventMessage) ToMeetingDeletedPayload() (*ZoomMeetingDeletedPayload, error) {
	if z.EventType != "meeting.deleted" {
		return nil, fmt.Errorf("invalid event type: expected meeting.deleted, got %s", z.EventType)
	}

	data, err := json.Marshal(z.Payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	var payload ZoomMeetingDeletedPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("failed to unmarshal to meeting deleted payload: %w", err)
	}

	return &payload, nil
}

// ToParticipantJoinedPayload converts the webhook event to a typed participant joined payload
func (z *ZoomWebhookEventMessage) ToParticipantJoinedPayload() (*ZoomParticipantJoinedPayload, error) {
	if z.EventType != "meeting.participant_joined" {
		return nil, fmt.Errorf("invalid event type: expected meeting.participant_joined, got %s", z.EventType)
	}

	data, err := json.Marshal(z.Payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	var payload ZoomParticipantJoinedPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("failed to unmarshal to participant joined payload: %w", err)
	}

	return &payload, nil
}

// ToParticipantLeftPayload converts the webhook event to a typed participant left payload
func (z *ZoomWebhookEventMessage) ToParticipantLeftPayload() (*ZoomParticipantLeftPayload, error) {
	if z.EventType != "meeting.participant_left" {
		return nil, fmt.Errorf("invalid event type: expected meeting.participant_left, got %s", z.EventType)
	}

	data, err := json.Marshal(z.Payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	var payload ZoomParticipantLeftPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("failed to unmarshal to participant left payload: %w", err)
	}

	return &payload, nil
}

// ToRecordingCompletedPayload converts the webhook event to a typed recording completed payload
func (z *ZoomWebhookEventMessage) ToRecordingCompletedPayload() (*ZoomRecordingCompletedPayload, error) {
	if z.EventType != "recording.completed" {
		return nil, fmt.Errorf("invalid event type: expected recording.completed, got %s", z.EventType)
	}

	data, err := json.Marshal(z.Payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	var payload ZoomRecordingCompletedPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("failed to unmarshal to recording completed payload: %w", err)
	}

	return &payload, nil
}

// ToTranscriptCompletedPayload converts the webhook event to a typed transcript completed payload
func (z *ZoomWebhookEventMessage) ToTranscriptCompletedPayload() (*ZoomTranscriptCompletedPayload, error) {
	if z.EventType != "recording.transcript_completed" {
		return nil, fmt.Errorf("invalid event type: expected recording.transcript_completed, got %s", z.EventType)
	}

	data, err := json.Marshal(z.Payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	var payload ZoomTranscriptCompletedPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("failed to unmarshal to transcript completed payload: %w", err)
	}

	return &payload, nil
}

// ToSummaryCompletedPayload converts the webhook event to a typed summary completed payload
func (z *ZoomWebhookEventMessage) ToSummaryCompletedPayload() (*ZoomSummaryCompletedPayload, error) {
	if z.EventType != "meeting.summary_completed" {
		return nil, fmt.Errorf("invalid event type: expected meeting.summary_completed, got %s", z.EventType)
	}

	data, err := json.Marshal(z.Payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	var payload ZoomSummaryCompletedPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("failed to unmarshal to summary completed payload: %w", err)
	}

	return &payload, nil
}
