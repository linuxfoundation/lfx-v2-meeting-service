// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
)

// User type constants for Zoom API
const (
	UserTypeBasic    = 1
	UserTypeLicensed = 2
	UserTypeOnPrem   = 3
)

// User status constants for Zoom API
const (
	UserStatusActive   = "active"
	UserStatusInactive = "inactive"
	UserStatusPending  = "pending"
)

// ZoomUser represents a user in the Zoom account
type ZoomUser struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Type      int    `json:"type"`
	Status    string `json:"status"`
}

// ZoomUsersResponse represents the response from the users API
type ZoomUsersResponse struct {
	PageCount   int        `json:"page_count"`
	PageNumber  int        `json:"page_number"`
	PageSize    int        `json:"page_size"`
	TotalRecord int        `json:"total_records"`
	Users       []ZoomUser `json:"users"`
}

// GetUsers retrieves users from the Zoom account
func (c *Client) GetUsers(ctx context.Context) ([]ZoomUser, error) {
	ctx = logging.AppendCtx(ctx, slog.String("zoom_operation", "get_users"))

	resp, err := c.doRequest(ctx, http.MethodGet, "/users?status=active&page_size=100", nil)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get Zoom users", logging.ErrKey, err)
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		err := parseErrorResponse(body)
		slog.ErrorContext(ctx, "Zoom API returned error", logging.ErrKey, err, "status", resp.StatusCode)
		return nil, err
	}

	var usersResp ZoomUsersResponse
	if err := json.NewDecoder(resp.Body).Decode(&usersResp); err != nil {
		slog.ErrorContext(ctx, "failed to decode users response", logging.ErrKey, err)
		return nil, fmt.Errorf("failed to decode users response: %w", err)
	}

	slog.InfoContext(ctx, "successfully retrieved Zoom users",
		"user_count", len(usersResp.Users),
		"total_records", usersResp.TotalRecord)

	return usersResp.Users, nil
}
