// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"sync/atomic"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/service"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/concurrent"
)

// ProjectRole represents a role type in project settings
type ProjectRole string

const (
	// ProjectRoleWriter represents the writer role
	ProjectRoleWriter ProjectRole = "writer"
	// ProjectRoleMeetingCoordinator represents the meeting coordinator role
	ProjectRoleMeetingCoordinator ProjectRole = "meeting_coordinator"
	// ProjectRoleAuditor represents the auditor role
	ProjectRoleAuditor ProjectRole = "auditor"
)

// ProjectHandlers handles project-related messages and events.
type ProjectHandlers struct {
	meetingService *service.MeetingService
}

// NewProjectHandlers creates a new project handlers instance.
func NewProjectHandlers(
	meetingService *service.MeetingService,
) *ProjectHandlers {
	return &ProjectHandlers{
		meetingService: meetingService,
	}
}

func (h *ProjectHandlers) HandlerReady() bool {
	return h.meetingService.ServiceReady()
}

// HandleMessage implements domain.MessageHandler interface
func (h *ProjectHandlers) HandleMessage(ctx context.Context, msg domain.Message) {
	subject := msg.Subject()
	ctx = logging.AppendCtx(ctx, slog.String("subject", subject))
	slog.DebugContext(ctx, "handling project NATS message")

	var response []byte
	var err error

	handlers := map[string]func(ctx context.Context, msg domain.Message) ([]byte, error){
		models.ProjectSettingsUpdatedSubject: h.HandleProjectSettingsUpdated,
	}

	handler, ok := handlers[subject]
	if !ok {
		slog.WarnContext(ctx, "unknown project message subject", "subject", subject)
		return
	}

	response, err = handler(ctx, msg)
	if err != nil {
		slog.ErrorContext(ctx, "error handling project message", logging.ErrKey, err)
	} else {
		slog.DebugContext(ctx, "project message handled successfully", "response", string(response))
	}
}

// HandleProjectSettingsUpdated is the message handler for the project-settings-updated subject.
// It processes changes to project settings and removes users from meeting organizers
// when they are removed from the writers or meeting_coordinators lists.
func (h *ProjectHandlers) HandleProjectSettingsUpdated(ctx context.Context, msg domain.Message) ([]byte, error) {
	if !h.meetingService.ServiceReady() {
		slog.ErrorContext(ctx, "service not ready")
		return nil, fmt.Errorf("service not ready")
	}

	slog.DebugContext(ctx, "handling project settings updated message", "message", string(msg.Data()))

	// Parse the project settings updated message
	var payload models.ProjectSettingsUpdatedPayload
	err := json.Unmarshal(msg.Data(), &payload)
	if err != nil {
		slog.ErrorContext(ctx, "error unmarshaling project settings updated message", logging.ErrKey, err)
		return nil, err
	}

	if payload.ProjectUID == "" {
		slog.WarnContext(ctx, "invalid project settings updated message: missing project UID")
		return nil, fmt.Errorf("project UID is required")
	}

	if payload.OldSettings == nil || payload.NewSettings == nil {
		slog.WarnContext(ctx, "invalid project settings updated message: missing settings data")
		return nil, fmt.Errorf("both old and new settings are required")
	}

	ctx = logging.AppendCtx(ctx, slog.String("project_uid", payload.ProjectUID))
	slog.InfoContext(ctx, "processing project settings update")

	// Find users removed from writers or meeting_coordinators (roles that grant organizer access)
	rolesToCheck := []ProjectRole{ProjectRoleWriter, ProjectRoleMeetingCoordinator}
	removedUsernames := h.findRemovedUsernamesByRoles(payload.OldSettings, payload.NewSettings, rolesToCheck)
	if len(removedUsernames) == 0 {
		slog.DebugContext(ctx, "no users removed from writers or meeting coordinators")
		return []byte("success"), nil
	}

	slog.InfoContext(ctx, "users removed from writers/meeting coordinators",
		"removed_usernames", removedUsernames,
		"count", len(removedUsernames))

	// Get all meetings for this project
	meetings, err := h.meetingService.ListMeetingsByProject(ctx, payload.ProjectUID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to list meetings by project", logging.ErrKey, err)
		return nil, fmt.Errorf("failed to list meetings by project: %w", err)
	}

	if len(meetings) == 0 {
		slog.InfoContext(ctx, "no meetings found for project")
		return []byte("success"), nil
	}

	slog.InfoContext(ctx, "found meetings for project",
		"meeting_count", len(meetings))

	// Process each meeting to remove organizers
	var successCount int64

	tasks := make([]func() error, 0, len(meetings))
	for _, meeting := range meetings {
		m := meeting

		tasks = append(tasks, func() error {
			err := h.tryToRemoveOrganizersFromMeeting(ctx, m.UID, removedUsernames)
			if err != nil {
				return err
			}
			atomic.AddInt64(&successCount, 1)
			return nil
		})
	}

	workerPool := concurrent.NewWorkerPool(10)
	errors := workerPool.RunAll(ctx, tasks...)
	if len(errors) > 0 {
		slog.ErrorContext(ctx, "some organizer removal operations failed",
			"project_uid", payload.ProjectUID,
			"errors", errors,
			"errors_count", len(errors),
		)
	}

	slog.InfoContext(ctx, "completed project settings update processing",
		"project_uid", payload.ProjectUID,
		"total_meetings", len(meetings),
		"successful_updates", atomic.LoadInt64(&successCount),
		"failed_updates", len(errors))

	return []byte("success"), nil
}

// getUsernamesByRole extracts usernames for a specific role from project settings
func getUsernamesByRole(settings *models.ProjectSettings, role ProjectRole) map[string]bool {
	usernames := make(map[string]bool)
	if settings == nil {
		return usernames
	}

	var users []models.ProjectUserInfo
	switch role {
	case ProjectRoleWriter:
		users = settings.Writers
	case ProjectRoleMeetingCoordinator:
		users = settings.MeetingCoordinators
	case ProjectRoleAuditor:
		users = settings.Auditors
	}

	for _, user := range users {
		usernames[user.Username] = true
	}
	return usernames
}

// findRemovedUsernamesByRoles finds usernames that were removed from any of the specified roles
func (h *ProjectHandlers) findRemovedUsernamesByRoles(
	oldSettings, newSettings *models.ProjectSettings,
	roles []ProjectRole,
) []string {
	removedUsernames := make(map[string]bool)

	for _, role := range roles {
		oldUsernames := getUsernamesByRole(oldSettings, role)
		newUsernames := getUsernamesByRole(newSettings, role)

		// Check each old username - if not in new, they're removed from this role
		for username := range oldUsernames {
			if !newUsernames[username] {
				removedUsernames[username] = true
			}
		}
	}

	// Convert map to slice
	result := make([]string, 0, len(removedUsernames))
	for username := range removedUsernames {
		result = append(result, username)
	}

	return result
}

// tryToRemoveOrganizersFromMeeting removes specified usernames from meeting organizers if they are in the organizers list
func (h *ProjectHandlers) tryToRemoveOrganizersFromMeeting(
	ctx context.Context,
	meetingUID string,
	usernamesToRemove []string,
) error {
	// Fetch settings with revision for optimistic locking
	settings, revisionStr, err := h.meetingService.GetMeetingSettings(ctx, meetingUID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get meeting settings",
			"meeting_uid", meetingUID,
			logging.ErrKey, err)
		return fmt.Errorf("failed to get meeting settings for %s: %w", meetingUID, err)
	}

	if settings == nil || len(settings.Organizers) == 0 {
		return nil
	}

	// Check if any of the usernames to remove are in the organizers list
	var newOrganizers []string
	removed := false
	for _, organizer := range settings.Organizers {
		if slices.Contains(usernamesToRemove, organizer) {
			removed = true
			slog.InfoContext(ctx, "removing organizer from meeting",
				"meeting_uid", meetingUID,
				"organizer", organizer)
		} else {
			newOrganizers = append(newOrganizers, organizer)
		}
	}

	if !removed {
		slog.DebugContext(ctx, "no organizers to remove from meeting", "meeting_uid", meetingUID)
		return nil
	}

	revision, err := strconv.ParseUint(revisionStr, 10, 64)
	if err != nil {
		slog.ErrorContext(ctx, "failed to parse revision",
			"meeting_uid", meetingUID,
			"revision_str", revisionStr,
			logging.ErrKey, err)
		return fmt.Errorf("failed to parse revision for %s: %w", meetingUID, err)
	}

	// Update the meeting settings with the new organizers list
	settings.Organizers = newOrganizers
	_, err = h.meetingService.UpdateMeetingSettings(ctx, settings, revision, false)
	if err != nil {
		slog.ErrorContext(ctx, "failed to update meeting organizers",
			"meeting_uid", meetingUID,
			logging.ErrKey, err)
		return fmt.Errorf("failed to update meeting organizers for %s: %w", meetingUID, err)
	}

	slog.InfoContext(ctx, "successfully updated meeting organizers",
		"meeting_uid", meetingUID,
		"new_organizer_count", len(newOrganizers))

	return nil
}
