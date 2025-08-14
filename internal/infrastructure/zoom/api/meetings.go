// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Meeting type constants for Zoom API
const (
	MeetingTypeInstant              = 1
	MeetingTypeScheduled            = 2
	MeetingTypeRecurringNoFixedTime = 3
	MeetingTypeRecurringFixedTime   = 8
)

// Recurrence type constants for Zoom API
const (
	RecurrenceTypeDaily   = 1
	RecurrenceTypeWeekly  = 2
	RecurrenceTypeMonthly = 3
)

// CreateMeetingRequest represents the request to create a Zoom meeting
type CreateMeetingRequest struct {
	Topic      string              `json:"topic"`
	Type       int                 `json:"type"`
	StartTime  string              `json:"start_time,omitempty"`
	Duration   int                 `json:"duration,omitempty"`
	Timezone   string              `json:"timezone,omitempty"`
	Agenda     string              `json:"agenda,omitempty"`
	Recurrence *RecurrenceSettings `json:"recurrence,omitempty"`
	Settings   *MeetingSettings    `json:"settings,omitempty"`
}

// UpdateMeetingRequest represents the request to update a Zoom meeting
type UpdateMeetingRequest struct {
	Topic      string              `json:"topic,omitempty"`
	Type       int                 `json:"type,omitempty"`
	StartTime  string              `json:"start_time,omitempty"`
	Duration   int                 `json:"duration,omitempty"`
	Timezone   string              `json:"timezone,omitempty"`
	Agenda     string              `json:"agenda,omitempty"`
	Recurrence *RecurrenceSettings `json:"recurrence,omitempty"`
	Settings   *MeetingSettings    `json:"settings,omitempty"`
}

// RecurrenceSettings represents Zoom meeting recurrence settings
type RecurrenceSettings struct {
	Type           int    `json:"type"`
	RepeatInterval int    `json:"repeat_interval,omitempty"`
	WeeklyDays     string `json:"weekly_days,omitempty"`
	MonthlyDay     int    `json:"monthly_day,omitempty"`
	MonthlyWeek    int    `json:"monthly_week,omitempty"`
	MonthlyWeekDay int    `json:"monthly_week_day,omitempty"`
	EndTimes       int    `json:"end_times,omitempty"`
	EndDateTime    string `json:"end_date_time,omitempty"`
}

// MeetingSettings represents Zoom meeting settings
type MeetingSettings struct {
	HostVideo             bool   `json:"host_video"`
	ParticipantVideo      bool   `json:"participant_video"`
	JoinBeforeHost        bool   `json:"join_before_host"`
	MuteUponEntry         bool   `json:"mute_upon_entry"`
	Watermark             bool   `json:"watermark"`
	UsePMI                bool   `json:"use_pmi"`
	ApprovalType          int    `json:"approval_type"`
	RegistrationType      int    `json:"registration_type,omitempty"`
	Audio                 string `json:"audio"`
	AutoRecording         string `json:"auto_recording"`
	WaitingRoom           bool   `json:"waiting_room"`
	MeetingAuthentication bool   `json:"meeting_authentication"`
	JoinBeforeHostMinutes int    `json:"jbh_time,omitempty"`
	// AI Companion settings
	MeetingSummary      bool `json:"meeting_summary,omitempty"`
	MeetingQueryEnabled bool `json:"meeting_query_enabled,omitempty"`
}

// CreateMeetingResponse represents the response from creating a Zoom meeting
type CreateMeetingResponse struct {
	ID                int64               `json:"id"`
	UUID              string              `json:"uuid"`
	HostID            string              `json:"host_id"`
	HostEmail         string              `json:"host_email"`
	Topic             string              `json:"topic"`
	Type              int                 `json:"type"`
	Status            string              `json:"status"`
	StartTime         string              `json:"start_time"`
	Duration          int                 `json:"duration"`
	Timezone          string              `json:"timezone"`
	CreatedAt         string              `json:"created_at"`
	StartURL          string              `json:"start_url"`
	JoinURL           string              `json:"join_url"`
	Password          string              `json:"password"`
	H323Password      string              `json:"h323_password"`
	PSINPassword      string              `json:"pstn_password"`
	EncryptedPassword string              `json:"encrypted_password"`
	Settings          *MeetingSettings    `json:"settings"`
	Recurrence        *RecurrenceSettings `json:"recurrence"`
}

// CreateMeeting creates a new meeting in Zoom for the specified user
// This is a pure API call with no business logic
func (c *Client) CreateMeeting(ctx context.Context, userID string, request *CreateMeetingRequest) (*CreateMeetingResponse, error) {
	path := fmt.Sprintf("/users/%s/meetings", userID)
	resp, err := c.doRequest(ctx, http.MethodPost, path, request)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, parseErrorResponse(body)
	}

	var meetingResp CreateMeetingResponse
	if err := json.NewDecoder(resp.Body).Decode(&meetingResp); err != nil {
		return nil, fmt.Errorf("failed to decode meeting response: %w", err)
	}

	return &meetingResp, nil
}

// UpdateMeeting updates an existing meeting in Zoom
// This is a pure API call with no business logic
func (c *Client) UpdateMeeting(ctx context.Context, meetingID string, request *UpdateMeetingRequest) error {
	path := fmt.Sprintf("/meetings/%s", meetingID)
	resp, err := c.doRequest(ctx, http.MethodPatch, path, request)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return parseErrorResponse(body)
	}

	return nil
}

// DeleteMeeting deletes a meeting from Zoom
// This is a pure API call with no business logic
func (c *Client) DeleteMeeting(ctx context.Context, meetingID string) error {
	path := fmt.Sprintf("/meetings/%s", meetingID)
	resp, err := c.doRequest(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return parseErrorResponse(body)
	}

	return nil
}
