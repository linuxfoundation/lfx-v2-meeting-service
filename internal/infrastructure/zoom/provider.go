// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package zoom

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/infrastructure/zoom/api"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/redaction"
)

const (
	// MaxAgendaLength is the maximum length for a Zoom meeting agenda field
	// as confirmed by Zoom staff (not documented in official API docs)
	// Reference: https://devforum.zoom.us/t/is-there-a-size-limit-for-the-agenda-field/11199
	MaxAgendaLength = 2000
)

// ZoomProvider implements the PlatformProvider interface for Zoom
// It contains business logic and orchestrates API calls through the Client
type ZoomProvider struct {
	client      api.ClientAPI
	cachedUsers map[string]*api.ZoomUser // map[userID]*ZoomUser
	usersMu     sync.RWMutex             // protects cachedUsers map
}

// NewZoomProvider creates a new ZoomProvider with the given client
func NewZoomProvider(client api.ClientAPI) *ZoomProvider {
	return &ZoomProvider{
		client:      client,
		cachedUsers: make(map[string]*api.ZoomUser),
	}
}

// Ensure ZoomProvider implements PlatformProvider
var _ domain.PlatformProvider = (*ZoomProvider)(nil)

// CreateMeeting creates a new meeting in Zoom using business logic
func (p *ZoomProvider) CreateMeeting(ctx context.Context, meeting *models.MeetingBase) (*domain.CreateMeetingResult, error) {
	ctx = logging.AppendCtx(ctx, slog.String("zoom_operation", "create_meeting"))

	// Get the first available user to schedule the meeting for
	user, err := p.getCachedUser(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get Zoom user for meeting creation", logging.ErrKey, err)
		return nil, fmt.Errorf("failed to get Zoom user for meeting %s: %w", meeting.UID, err)
	}

	// Build the API request using business logic
	req := p.buildCreateMeetingRequest(meeting)

	// Create meeting for the selected user using pure API call
	resp, err := p.client.CreateMeeting(ctx, user.ID, req)
	if err != nil {
		slog.ErrorContext(ctx, "failed to create Zoom meeting", logging.ErrKey, err)
		return nil, fmt.Errorf("failed to create Zoom meeting %s for user %s: %w", meeting.UID, user.Email, err)
	}

	result := &domain.CreateMeetingResult{
		PlatformMeetingID: fmt.Sprintf("%d", resp.ID),
		JoinURL:           resp.JoinURL,
		Passcode:          resp.Password,
	}

	slog.InfoContext(ctx, "successfully created Zoom meeting",
		"meeting_id", result.PlatformMeetingID,
		"topic", resp.Topic)

	return result, nil
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
		return fmt.Errorf("failed to update Zoom meeting %s (platform ID: %s): %w", meeting.UID, meetingID, err)
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
		return fmt.Errorf("failed to delete Zoom meeting (platform ID: %s): %w", meetingID, err)
	}

	slog.InfoContext(ctx, "successfully deleted Zoom meeting")
	return nil
}

// StorePlatformData stores Zoom-specific data in the meeting model after creation
func (p *ZoomProvider) StorePlatformData(meeting *models.MeetingBase, result *domain.CreateMeetingResult) {
	if meeting.ZoomConfig == nil {
		meeting.ZoomConfig = &models.ZoomConfig{}
	}
	meeting.ZoomConfig.MeetingID = result.PlatformMeetingID
	meeting.ZoomConfig.Passcode = result.Passcode
	meeting.JoinURL = result.JoinURL
}

// GetPlatformMeetingID retrieves the Zoom meeting ID from the meeting model
func (p *ZoomProvider) GetPlatformMeetingID(meeting *models.MeetingBase) string {
	if meeting.ZoomConfig != nil {
		return meeting.ZoomConfig.MeetingID
	}
	return ""
}

// buildCreateMeetingRequest builds a Zoom API request from our domain model
func (p *ZoomProvider) buildCreateMeetingRequest(meeting *models.MeetingBase) *api.CreateMeetingRequest {
	req := &api.CreateMeetingRequest{
		Type:     api.MeetingTypeRecurringNoFixedTime,
		Topic:    meeting.Title,
		Timezone: meeting.Timezone,
		Duration: meeting.Duration,
		Agenda:   meeting.Description,
	}

	req.Settings = new(api.MeetingSettings)

	// Enable recording if requested
	if meeting.RecordingEnabled {
		req.Settings.AutoRecording = "cloud"
	}

	// Enable AI Companion settings if AI companion is enabled on meeting zoom config
	if meeting.ZoomConfig != nil && meeting.ZoomConfig.AICompanionEnabled {
		req.Settings.AutoStartAICompanionQuestions = true
		req.Settings.AutoStartMeetingSummary = true
	}

	return req
}

// buildUpdateMeetingRequest builds a Zoom API update request from our domain model
func (p *ZoomProvider) buildUpdateMeetingRequest(meeting *models.MeetingBase) *api.UpdateMeetingRequest {
	req := &api.UpdateMeetingRequest{
		Type:     api.MeetingTypeRecurringNoFixedTime,
		Topic:    meeting.Title,
		Timezone: meeting.Timezone,
		Duration: meeting.Duration,
		Agenda:   meeting.Description,
	}

	req.Settings = new(api.MeetingSettings)

	// Enable recording if requested
	if meeting.RecordingEnabled {
		req.Settings.AutoRecording = "cloud"
	}

	// Enable AI Companion settings if requested
	if meeting.ZoomConfig != nil && meeting.ZoomConfig.AICompanionEnabled {
		req.Settings.AutoStartAICompanionQuestions = true
		req.Settings.AutoStartMeetingSummary = true
	}

	return req
}

// getCachedUser gets the cached users or fetches them if not cached, then returns the first available user
func (p *ZoomProvider) getCachedUser(ctx context.Context) (*api.ZoomUser, error) {
	// Check if we have cached users using read lock
	p.usersMu.RLock()
	hasCachedUsers := len(p.cachedUsers) > 0
	p.usersMu.RUnlock()

	// If we have valid cached users, find the first available one from cache
	if hasCachedUsers {
		user := p.getFirstAvailableUserFromCache()
		if user != nil {
			slog.DebugContext(ctx, "using cached Zoom user", "user_id", user.ID)
			return user, nil
		}
	}

	// Fetch and cache all users, then return the first available one
	return p.fetchAndCacheUsers(ctx)
}

// fetchAndCacheUsers fetches all users from Zoom API, caches them, and returns the first available user
func (p *ZoomProvider) fetchAndCacheUsers(ctx context.Context) (*api.ZoomUser, error) {
	ctx = logging.AppendCtx(ctx, slog.String("zoom_operation", "fetch_and_cache_users"))

	users, err := p.client.GetUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get users: %w", err)
	}

	if len(users) == 0 {
		return nil, fmt.Errorf("no users found in Zoom account")
	}

	// Cache all users by their ID with write lock
	p.usersMu.Lock()
	p.cachedUsers = make(map[string]*api.ZoomUser)
	for i := range users {
		user := &users[i] // Important: take address of slice element
		p.cachedUsers[user.ID] = user
	}
	userCount := len(p.cachedUsers)
	p.usersMu.Unlock()

	slog.InfoContext(ctx, "cached Zoom users", "user_count", userCount)

	// Return the first available user from the newly cached users
	user := p.getFirstAvailableUserFromCache()
	if user == nil {
		return nil, fmt.Errorf("no active users found in Zoom account")
	}

	slog.InfoContext(ctx, "selected Zoom user for meeting creation",
		"user_id", user.ID,
		"email", redaction.RedactEmail(user.Email),
		"name", redaction.Redact(fmt.Sprintf("%s %s", user.FirstName, user.LastName)),
		"type", user.Type)

	return user, nil
}

// getFirstAvailableUserFromCache finds the first active licensed user from cached users
func (p *ZoomProvider) getFirstAvailableUserFromCache() *api.ZoomUser {
	p.usersMu.RLock()
	defer p.usersMu.RUnlock()

	// Find first active licensed user (type 2 = licensed)
	for _, user := range p.cachedUsers {
		if user.Status == api.UserStatusActive && user.Type == api.UserTypeLicensed {
			return user
		}
	}

	// If no licensed users, fall back to any active user
	for _, user := range p.cachedUsers {
		if user.Status == api.UserStatusActive {
			return user
		}
	}

	return nil
}
