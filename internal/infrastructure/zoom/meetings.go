// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package zoom

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
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

// CreateMeeting creates a new meeting in Zoom
func (c *Client) CreateMeeting(ctx context.Context, meeting *models.MeetingBase) (string, string, error) {
	ctx = logging.AppendCtx(ctx, slog.String("zoom_operation", "create_meeting"))

	// Get the first available user to schedule the meeting for
	user, err := c.GetCachedUser(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get Zoom user for meeting creation", logging.ErrKey, err)
		return "", "", fmt.Errorf("failed to get Zoom user: %w", err)
	}

	req := c.buildCreateMeetingRequest(meeting)

	// Create meeting for the selected user
	path := fmt.Sprintf("/users/%s/meetings", user.ID)
	resp, err := c.doRequest(ctx, http.MethodPost, path, req)
	if err != nil {
		slog.ErrorContext(ctx, "failed to create Zoom meeting", logging.ErrKey, err)
		return "", "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		err := parseErrorResponse(body)
		slog.ErrorContext(ctx, "Zoom API returned error", logging.ErrKey, err, "status", resp.StatusCode)
		return "", "", err
	}

	var meetingResp CreateMeetingResponse
	if err := json.NewDecoder(resp.Body).Decode(&meetingResp); err != nil {
		slog.ErrorContext(ctx, "failed to decode meeting response", logging.ErrKey, err)
		return "", "", fmt.Errorf("failed to decode meeting response: %w", err)
	}

	meetingID := fmt.Sprintf("%d", meetingResp.ID)
	slog.InfoContext(ctx, "successfully created Zoom meeting",
		"meeting_id", meetingID,
		"topic", meetingResp.Topic,
		"join_url", meetingResp.JoinURL)

	return meetingID, meetingResp.JoinURL, nil
}

// UpdateMeeting updates an existing meeting in Zoom
func (c *Client) UpdateMeeting(ctx context.Context, meetingID string, meeting *models.MeetingBase) error {
	ctx = logging.AppendCtx(ctx, slog.String("zoom_operation", "update_meeting"))
	ctx = logging.AppendCtx(ctx, slog.String("zoom_meeting_id", meetingID))

	req := c.buildUpdateMeetingRequest(meeting)

	path := fmt.Sprintf("/meetings/%s", meetingID)
	resp, err := c.doRequest(ctx, http.MethodPatch, path, req)
	if err != nil {
		slog.ErrorContext(ctx, "failed to update Zoom meeting", logging.ErrKey, err)
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		err := parseErrorResponse(body)
		slog.ErrorContext(ctx, "Zoom API returned error", logging.ErrKey, err, "status", resp.StatusCode)
		return err
	}

	slog.InfoContext(ctx, "successfully updated Zoom meeting")
	return nil
}

// DeleteMeeting deletes a meeting from Zoom
func (c *Client) DeleteMeeting(ctx context.Context, meetingID string) error {
	ctx = logging.AppendCtx(ctx, slog.String("zoom_operation", "delete_meeting"))
	ctx = logging.AppendCtx(ctx, slog.String("zoom_meeting_id", meetingID))

	path := fmt.Sprintf("/meetings/%s", meetingID)
	resp, err := c.doRequest(ctx, http.MethodDelete, path, nil)
	if err != nil {
		slog.ErrorContext(ctx, "failed to delete Zoom meeting", logging.ErrKey, err)
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	// Zoom returns 204 No Content on successful deletion
	// 404 is also acceptable as the meeting might have already been deleted
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(resp.Body)
		err := parseErrorResponse(body)
		slog.ErrorContext(ctx, "Zoom API returned error", logging.ErrKey, err, "status", resp.StatusCode)
		return err
	}

	if resp.StatusCode == http.StatusNotFound {
		slog.WarnContext(ctx, "Zoom meeting not found, may have been already deleted")
	} else {
		slog.InfoContext(ctx, "successfully deleted Zoom meeting")
	}

	return nil
}

// buildCreateMeetingRequest builds a Zoom API request from our domain model
func (c *Client) buildCreateMeetingRequest(meeting *models.MeetingBase) *CreateMeetingRequest {
	req := &CreateMeetingRequest{
		Topic:    meeting.Title,
		Timezone: meeting.Timezone,
		Duration: meeting.Duration,
		Agenda:   meeting.Description,
	}

	// Set meeting type based on recurrence
	if meeting.Recurrence != nil {
		req.Type = MeetingTypeRecurringFixedTime
		req.Recurrence = convertRecurrence(meeting.Recurrence)
	} else {
		req.Type = MeetingTypeScheduled
		req.StartTime = meeting.StartTime.Format(time.RFC3339)
	}

	// Configure settings
	req.Settings = &MeetingSettings{
		HostVideo:        true,
		ParticipantVideo: false,
		JoinBeforeHost:   meeting.EarlyJoinTimeMinutes > 0,
		MuteUponEntry:    false,
		WaitingRoom:      meeting.Restricted,
		Audio:            "both",
		AutoRecording:    "none",
		ApprovalType:     0, // No registration approval required
	}

	// Set join before host time if specified
	if meeting.EarlyJoinTimeMinutes > 0 {
		req.Settings.JoinBeforeHostMinutes = meeting.EarlyJoinTimeMinutes
	}

	// Enable recording if requested
	if meeting.RecordingEnabled {
		req.Settings.AutoRecording = "cloud"
	}

	// Configure AI Companion settings if Zoom config is provided
	if meeting.ZoomConfig != nil {
		req.Settings.MeetingSummary = meeting.ZoomConfig.AICompanionEnabled
		req.Settings.MeetingQueryEnabled = meeting.ZoomConfig.AICompanionEnabled
	}

	return req
}

// buildUpdateMeetingRequest builds a Zoom API update request from our domain model
func (c *Client) buildUpdateMeetingRequest(meeting *models.MeetingBase) *UpdateMeetingRequest {
	req := &UpdateMeetingRequest{
		Topic:    meeting.Title,
		Timezone: meeting.Timezone,
		Duration: meeting.Duration,
		Agenda:   meeting.Description,
	}

	// Set meeting type based on recurrence
	if meeting.Recurrence != nil {
		req.Type = MeetingTypeRecurringFixedTime
		req.Recurrence = convertRecurrence(meeting.Recurrence)
	} else {
		req.Type = MeetingTypeScheduled
		req.StartTime = meeting.StartTime.Format(time.RFC3339)
	}

	// Configure settings
	req.Settings = &MeetingSettings{
		HostVideo:        true,
		ParticipantVideo: false,
		JoinBeforeHost:   meeting.EarlyJoinTimeMinutes > 0,
		MuteUponEntry:    false,
		WaitingRoom:      meeting.Restricted,
		Audio:            "both",
		AutoRecording:    "none",
	}

	// Set join before host time if specified
	if meeting.EarlyJoinTimeMinutes > 0 {
		req.Settings.JoinBeforeHostMinutes = meeting.EarlyJoinTimeMinutes
	}

	// Enable recording if requested
	if meeting.RecordingEnabled {
		req.Settings.AutoRecording = "cloud"
	}

	// Configure AI Companion settings if Zoom config is provided
	if meeting.ZoomConfig != nil {
		req.Settings.MeetingSummary = meeting.ZoomConfig.AICompanionEnabled
		req.Settings.MeetingQueryEnabled = meeting.ZoomConfig.AICompanionEnabled
	}

	return req
}

// convertRecurrence converts our domain recurrence model to Zoom's format
func convertRecurrence(rec *models.Recurrence) *RecurrenceSettings {
	if rec == nil {
		return nil
	}

	zoomRec := &RecurrenceSettings{
		Type:           rec.Type,
		RepeatInterval: rec.RepeatInterval,
		WeeklyDays:     rec.WeeklyDays,
		MonthlyDay:     rec.MonthlyDay,
		MonthlyWeek:    rec.MonthlyWeek,
		MonthlyWeekDay: rec.MonthlyWeekDay,
		EndTimes:       rec.EndTimes,
	}

	if rec.EndDateTime != nil {
		zoomRec.EndDateTime = rec.EndDateTime.Format(time.RFC3339)
	}

	return zoomRec
}
