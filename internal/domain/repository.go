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
	Create(ctx context.Context, meeting *models.MeetingBase, settings *models.MeetingSettings) error
	Exists(ctx context.Context, meetingUID string) (bool, error)
	Delete(ctx context.Context, meetingUID string, revision uint64) error

	// Meeting base operations
	GetBase(ctx context.Context, meetingUID string) (*models.MeetingBase, error)
	GetBaseWithRevision(ctx context.Context, meetingUID string) (*models.MeetingBase, uint64, error)
	UpdateBase(ctx context.Context, meeting *models.MeetingBase, revision uint64) error

	// Meeting settings operations
	GetSettings(ctx context.Context, meetingUID string) (*models.MeetingSettings, error)
	GetSettingsWithRevision(ctx context.Context, meetingUID string) (*models.MeetingSettings, uint64, error)
	UpdateSettings(ctx context.Context, meetingSettings *models.MeetingSettings, revision uint64) error

	// Bulk operations
	ListAll(ctx context.Context) ([]*models.MeetingBase, []*models.MeetingSettings, error)
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
