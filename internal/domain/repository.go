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
	ListByCommittee(ctx context.Context, committeeUID string) ([]*models.MeetingBase, []*models.MeetingSettings, error)

	// Platform-specific operations
	GetByZoomMeetingID(ctx context.Context, zoomMeetingID string) (*models.MeetingBase, error)
}

// RegistrantRepository defines the interface for registrant storage operations.
// This interface can be implemented by different storage backends (NATS, PostgreSQL, etc.)
type RegistrantRepository interface {
	// Registrant full operations
	Create(ctx context.Context, registrant *models.Registrant) error
	Exists(ctx context.Context, registrantUID string) (bool, error)
	ExistsByMeetingAndEmail(ctx context.Context, meetingUID, email string) (bool, error)
	Delete(ctx context.Context, registrantUID string, revision uint64) error

	// Registrant base operations
	Get(ctx context.Context, registrantUID string) (*models.Registrant, error)
	GetWithRevision(ctx context.Context, registrantUID string) (*models.Registrant, uint64, error)
	Update(ctx context.Context, registrant *models.Registrant, revision uint64) error

	// Bulk operations
	ListByMeeting(ctx context.Context, meetingUID string) ([]*models.Registrant, error)
	ListByEmail(ctx context.Context, email string) ([]*models.Registrant, error)
}

// PastMeetingRepository defines the interface for past meeting storage operations.
// This interface can be implemented by different storage backends (NATS, PostgreSQL, etc.)
type PastMeetingRepository interface {
	// PastMeeting full operations
	Create(ctx context.Context, pastMeeting *models.PastMeeting) error
	Exists(ctx context.Context, pastMeetingUID string) (bool, error)
	Delete(ctx context.Context, pastMeetingUID string, revision uint64) error

	// PastMeeting base operations
	Get(ctx context.Context, pastMeetingUID string) (*models.PastMeeting, error)
	GetWithRevision(ctx context.Context, pastMeetingUID string) (*models.PastMeeting, uint64, error)
	Update(ctx context.Context, pastMeeting *models.PastMeeting, revision uint64) error

	// Bulk operations
	ListAll(ctx context.Context) ([]*models.PastMeeting, error)
	ListByMeeting(ctx context.Context, meetingUID string) ([]*models.PastMeeting, error)
	GetByMeetingAndOccurrence(ctx context.Context, meetingUID, occurrenceID string) (*models.PastMeeting, error)
	GetByPlatformMeetingID(ctx context.Context, platform, platformMeetingID string) (*models.PastMeeting, error)
	GetByPlatformMeetingIDAndOccurrence(ctx context.Context, platform, platformMeetingID, occurrenceID string) (*models.PastMeeting, error)
}

// PastMeetingParticipantRepository defines the interface for past meeting participant storage operations.
// This interface can be implemented by different storage backends (NATS, PostgreSQL, etc.)
type PastMeetingParticipantRepository interface {
	// PastMeetingParticipant full operations
	Create(ctx context.Context, participant *models.PastMeetingParticipant) error
	Exists(ctx context.Context, participantUID string) (bool, error)
	Delete(ctx context.Context, participantUID string, revision uint64) error

	// PastMeetingParticipant base operations
	Get(ctx context.Context, participantUID string) (*models.PastMeetingParticipant, error)
	GetWithRevision(ctx context.Context, participantUID string) (*models.PastMeetingParticipant, uint64, error)
	Update(ctx context.Context, participant *models.PastMeetingParticipant, revision uint64) error

	// Bulk operations
	ListByPastMeeting(ctx context.Context, pastMeetingUID string) ([]*models.PastMeetingParticipant, error)
	ListByEmail(ctx context.Context, email string) ([]*models.PastMeetingParticipant, error)
	GetByPastMeetingAndEmail(ctx context.Context, pastMeetingUID, email string) (*models.PastMeetingParticipant, error)
}

// PastMeetingRecordingRepository defines the interface for past meeting recording storage operations.
// This interface can be implemented by different storage backends (NATS, PostgreSQL, etc.)
type PastMeetingRecordingRepository interface {
	Create(ctx context.Context, recording *models.PastMeetingRecording) error
	Exists(ctx context.Context, recordingUID string) (bool, error)
	Delete(ctx context.Context, recordingUID string, revision uint64) error
	Get(ctx context.Context, recordingUID string) (*models.PastMeetingRecording, error)
	GetWithRevision(ctx context.Context, recordingUID string) (*models.PastMeetingRecording, uint64, error)
	Update(ctx context.Context, recording *models.PastMeetingRecording, revision uint64) error
	GetByPastMeetingUID(ctx context.Context, pastMeetingUID string) (*models.PastMeetingRecording, error)
	ListByPastMeeting(ctx context.Context, pastMeetingUID string) ([]*models.PastMeetingRecording, error)
	ListAll(ctx context.Context) ([]*models.PastMeetingRecording, error)
}
