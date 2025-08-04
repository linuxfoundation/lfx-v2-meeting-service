// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package domain

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
)

// MeetingRepository defines the interface for meeting storage operations.
// This interface can be implemented by different storage backends (NATS, PostgreSQL, etc.)
type MeetingRepository interface {
	// Meeting full operations
	CreateMeeting(ctx context.Context, meeting *models.Meeting) error
	MeetingExists(ctx context.Context, meetingUID string) (bool, error)
	DeleteMeeting(ctx context.Context, meetingUID string, revision uint64) error

	// Meeting base operations
	GetMeeting(ctx context.Context, meetingUID string) (*models.Meeting, error)
	GetMeetingWithRevision(ctx context.Context, meetingUID string) (*models.Meeting, uint64, error)
	UpdateMeeting(ctx context.Context, meeting *models.Meeting, revision uint64) error

	// Bulk operations
	ListAllMeetings(ctx context.Context) ([]*models.Meeting, error)
}
