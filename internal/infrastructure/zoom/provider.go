// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package zoom

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/infrastructure/zoom/api"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
)

// ZoomProvider implements the PlatformProvider interface for Zoom
// It contains business logic and orchestrates API calls through the Client
type ZoomProvider struct {
	client         *api.Client
	cachedUser     *api.ZoomUser
	userCacheValid bool
}

// NewZoomProvider creates a new ZoomProvider with the given client
func NewZoomProvider(client *api.Client) *ZoomProvider {
	return &ZoomProvider{
		client: client,
	}
}

// Ensure ZoomProvider implements PlatformProvider
var _ domain.PlatformProvider = (*ZoomProvider)(nil)

// CreateMeeting creates a new meeting in Zoom using business logic
func (p *ZoomProvider) CreateMeeting(ctx context.Context, meeting *models.MeetingBase) (string, string, error) {
	ctx = logging.AppendCtx(ctx, slog.String("zoom_operation", "create_meeting"))

	// Get the first available user to schedule the meeting for
	user, err := p.getCachedUser(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get Zoom user for meeting creation", logging.ErrKey, err)
		return "", "", fmt.Errorf("failed to get Zoom user: %w", err)
	}

	// Build the API request using business logic
	req := p.buildCreateMeetingRequest(meeting)

	// Create meeting for the selected user using pure API call
	resp, err := p.client.CreateMeeting(ctx, user.ID, req)
	if err != nil {
		slog.ErrorContext(ctx, "failed to create Zoom meeting", logging.ErrKey, err)
		return "", "", err
	}

	meetingID := fmt.Sprintf("%d", resp.ID)
	slog.InfoContext(ctx, "successfully created Zoom meeting",
		"meeting_id", meetingID,
		"topic", resp.Topic,
		"join_url", resp.JoinURL)

	return meetingID, resp.JoinURL, nil
}

// UpdateMeeting updates an existing meeting in Zoom using business logic
func (p *ZoomProvider) UpdateMeeting(ctx context.Context, meetingID string, meeting *models.MeetingBase) error {
	ctx = logging.AppendCtx(ctx, slog.String("zoom_operation", "update_meeting"))
	ctx = logging.AppendCtx(ctx, slog.String("zoom_meeting_id", meetingID))

	// Build the API request using business logic
	req := p.buildUpdateMeetingRequest(meeting)

	// Update meeting using pure API call
	if err := p.client.UpdateMeeting(ctx, meetingID, req); err != nil {
		slog.ErrorContext(ctx, "failed to update Zoom meeting", logging.ErrKey, err)
		return err
	}

	slog.InfoContext(ctx, "successfully updated Zoom meeting")
	return nil
}

// DeleteMeeting deletes a meeting from Zoom using business logic
func (p *ZoomProvider) DeleteMeeting(ctx context.Context, meetingID string) error {
	ctx = logging.AppendCtx(ctx, slog.String("zoom_operation", "delete_meeting"))
	ctx = logging.AppendCtx(ctx, slog.String("zoom_meeting_id", meetingID))

	// Delete meeting using pure API call
	if err := p.client.DeleteMeeting(ctx, meetingID); err != nil {
		slog.ErrorContext(ctx, "failed to delete Zoom meeting", logging.ErrKey, err)
		return err
	}

	slog.InfoContext(ctx, "successfully deleted Zoom meeting")
	return nil
}

// buildCreateMeetingRequest builds a Zoom API request from our domain model
func (p *ZoomProvider) buildCreateMeetingRequest(meeting *models.MeetingBase) *api.CreateMeetingRequest {
	req := &api.CreateMeetingRequest{
		Topic:    meeting.Title,
		Timezone: meeting.Timezone,
		Duration: meeting.Duration,
		Agenda:   meeting.Description,
	}

	// Set meeting type based on recurrence
	if meeting.Recurrence != nil {
		req.Type = api.MeetingTypeRecurringFixedTime
		req.Recurrence = p.convertRecurrence(meeting.Recurrence)
	} else {
		req.Type = api.MeetingTypeScheduled
		req.StartTime = meeting.StartTime.Format(time.RFC3339)
	}

	// Configure settings
	req.Settings = &api.MeetingSettings{
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
func (p *ZoomProvider) buildUpdateMeetingRequest(meeting *models.MeetingBase) *api.UpdateMeetingRequest {
	req := &api.UpdateMeetingRequest{
		Topic:    meeting.Title,
		Timezone: meeting.Timezone,
		Duration: meeting.Duration,
		Agenda:   meeting.Description,
	}

	// Set meeting type based on recurrence
	if meeting.Recurrence != nil {
		req.Type = api.MeetingTypeRecurringFixedTime
		req.Recurrence = p.convertRecurrence(meeting.Recurrence)
	} else {
		req.Type = api.MeetingTypeScheduled
		req.StartTime = meeting.StartTime.Format(time.RFC3339)
	}

	// Configure settings
	req.Settings = &api.MeetingSettings{
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
func (p *ZoomProvider) convertRecurrence(rec *models.Recurrence) *api.RecurrenceSettings {
	if rec == nil {
		return nil
	}

	zoomRec := &api.RecurrenceSettings{
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

// getCachedUser gets the cached user or fetches the first available user if not cached
func (p *ZoomProvider) getCachedUser(ctx context.Context) (*api.ZoomUser, error) {
	if p.userCacheValid && p.cachedUser != nil {
		slog.DebugContext(ctx, "using cached Zoom user", "user_id", p.cachedUser.ID)
		return p.cachedUser, nil
	}

	user, err := p.getFirstAvailableUser(ctx)
	if err != nil {
		return nil, err
	}

	// Cache the user for future use
	p.cachedUser = user
	p.userCacheValid = true

	return user, nil
}

// getFirstAvailableUser gets the first active licensed user from the account
func (p *ZoomProvider) getFirstAvailableUser(ctx context.Context) (*api.ZoomUser, error) {
	ctx = logging.AppendCtx(ctx, slog.String("zoom_operation", "get_first_available_user"))

	users, err := p.client.GetUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get users: %w", err)
	}

	if len(users) == 0 {
		return nil, fmt.Errorf("no users found in Zoom account")
	}

	// Find first active licensed user (type 2 = licensed)
	for _, user := range users {
		if user.Status == api.UserStatusActive && user.Type == api.UserTypeLicensed {
			slog.InfoContext(ctx, "selected Zoom user for meeting creation",
				"user_id", user.ID,
				"email", user.Email,
				"name", fmt.Sprintf("%s %s", user.FirstName, user.LastName))
			return &user, nil
		}
	}

	// If no licensed users, fall back to any active user
	for _, user := range users {
		if user.Status == api.UserStatusActive {
			slog.InfoContext(ctx, "selected Zoom user for meeting creation (fallback)",
				"user_id", user.ID,
				"email", user.Email,
				"name", fmt.Sprintf("%s %s", user.FirstName, user.LastName),
				"type", user.Type)
			return &user, nil
		}
	}

	return nil, fmt.Errorf("no active users found in Zoom account")
}
