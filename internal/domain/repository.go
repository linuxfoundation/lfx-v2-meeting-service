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
	Create(ctx context.Context, meeting *models.Meeting) error
	Exists(ctx context.Context, meetingUID string) (bool, error)
	Delete(ctx context.Context, meetingUID string, revision uint64) error

	// Meeting base operations
	Get(ctx context.Context, meetingUID string) (*models.Meeting, error)
	GetWithRevision(ctx context.Context, meetingUID string) (*models.Meeting, uint64, error)
	Update(ctx context.Context, meeting *models.Meeting, revision uint64) error

	// Bulk operations
	ListAll(ctx context.Context) ([]*models.Meeting, error)
}

// RegistrantRepository defines the interface for registrant storage operations.
// This interface can be implemented by different storage backends (NATS, PostgreSQL, etc.)
type RegistrantRepository interface {
	// Registrant full operations
	Create(ctx context.Context, registrant *models.Registrant) error
	Exists(ctx context.Context, registrantUID string) (bool, error)
	Delete(ctx context.Context, registrantUID string, revision uint64) error

	// Registrant base operations
	Get(ctx context.Context, registrantUID string) (*models.Registrant, error)
	GetWithRevision(ctx context.Context, registrantUID string) (*models.Registrant, uint64, error)
	Update(ctx context.Context, registrant *models.Registrant, revision uint64) error

	// Bulk operations
	ListByMeeting(ctx context.Context, meetingUID string) ([]*models.Registrant, error)
	ListByEmail(ctx context.Context, email string) ([]*models.Registrant, error)
}
