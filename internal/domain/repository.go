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
	ListByProject(ctx context.Context, projectUID string) ([]*models.MeetingBase, error)

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
	ListByEmailAndCommittee(ctx context.Context, email string, committeeUID string) ([]*models.Registrant, error)
	GetByMeetingAndEmail(ctx context.Context, meetingUID, email string) (*models.Registrant, uint64, error)
	GetByMeetingAndUsername(ctx context.Context, meetingUID, username string) (*models.Registrant, uint64, error)
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
	GetByPlatformMeetingInstanceID(ctx context.Context, platform, platformMeetingInstanceID string) (*models.PastMeetingRecording, error)
	ListByPastMeeting(ctx context.Context, pastMeetingUID string) ([]*models.PastMeetingRecording, error)
	ListAll(ctx context.Context) ([]*models.PastMeetingRecording, error)
}

// PastMeetingTranscriptRepository defines the interface for past meeting transcript storage operations.
// This interface can be implemented by different storage backends (NATS, PostgreSQL, etc.)
type PastMeetingTranscriptRepository interface {
	Create(ctx context.Context, transcript *models.PastMeetingTranscript) error
	Exists(ctx context.Context, transcriptUID string) (bool, error)
	Delete(ctx context.Context, transcriptUID string, revision uint64) error
	Get(ctx context.Context, transcriptUID string) (*models.PastMeetingTranscript, error)
	GetWithRevision(ctx context.Context, transcriptUID string) (*models.PastMeetingTranscript, uint64, error)
	Update(ctx context.Context, transcript *models.PastMeetingTranscript, revision uint64) error
	GetByPastMeetingUID(ctx context.Context, pastMeetingUID string) (*models.PastMeetingTranscript, error)
	GetByPlatformMeetingInstanceID(ctx context.Context, platform, platformMeetingInstanceID string) (*models.PastMeetingTranscript, error)
	ListByPastMeeting(ctx context.Context, pastMeetingUID string) ([]*models.PastMeetingTranscript, error)
	ListAll(ctx context.Context) ([]*models.PastMeetingTranscript, error)
}

// PastMeetingSummaryRepository defines the interface for past meeting summary storage operations.
type PastMeetingSummaryRepository interface {
	Create(ctx context.Context, summary *models.PastMeetingSummary) error
	Get(ctx context.Context, summaryUID string) (*models.PastMeetingSummary, error)
	GetWithRevision(ctx context.Context, summaryUID string) (*models.PastMeetingSummary, uint64, error)
	Update(ctx context.Context, summary *models.PastMeetingSummary, revision uint64) error
	GetByPastMeetingUID(ctx context.Context, pastMeetingUID string) (*models.PastMeetingSummary, error)
	ListByPastMeeting(ctx context.Context, pastMeetingUID string) ([]*models.PastMeetingSummary, error)
}

// MeetingRSVPRepository defines the interface for meeting RSVP storage operations.
type MeetingRSVPRepository interface {
	// RSVP full operations
	Create(ctx context.Context, rsvp *models.RSVPResponse) error
	Exists(ctx context.Context, rsvpID string) (bool, error)
	Delete(ctx context.Context, rsvpID string, revision uint64) error

	// RSVP base operations
	Get(ctx context.Context, rsvpID string) (*models.RSVPResponse, error)
	GetWithRevision(ctx context.Context, rsvpID string) (*models.RSVPResponse, uint64, error)
	Update(ctx context.Context, rsvp *models.RSVPResponse, revision uint64) error

	// Query operations
	ListByMeeting(ctx context.Context, meetingUID string) ([]*models.RSVPResponse, error)
}

// MeetingAttachmentRepository defines the interface for meeting attachment storage operations.
// Metadata is stored in NATS KV store (includes meeting_uid field), while actual files are stored in NATS Object Store.
// Each metadata record associates a meeting with a file. Multiple metadata records can reference the same file.
type MeetingAttachmentRepository interface {
	// PutObject stores file in Object Store
	PutObject(ctx context.Context, attachmentUID string, fileData []byte) error

	// PutMetadata stores metadata in KV store
	PutMetadata(ctx context.Context, attachment *models.MeetingAttachment) error

	// GetObject retrieves file from Object Store
	GetObject(ctx context.Context, attachmentUID string) ([]byte, error)

	// GetMetadata retrieves only the metadata from KV store
	GetMetadata(ctx context.Context, attachmentUID string) (*models.MeetingAttachment, error)

	// ListByMeeting retrieves all attachment metadata for a meeting
	ListByMeeting(ctx context.Context, meetingUID string) ([]*models.MeetingAttachment, error)

	// Delete removes only the metadata from KV store (file persists in Object Store)
	Delete(ctx context.Context, attachmentUID string) error
}

// PastMeetingAttachmentRepository defines the interface for past meeting attachment storage operations.
// Metadata is stored in NATS KV store (includes past_meeting_uid and source_object_uid fields).
// Files are stored in the same Object Store as meeting attachments, allowing reuse via source_object_uid.
// This repository only manages metadata - files are accessed via the shared Object Store on the [MeetingAttachmentRepository].
type PastMeetingAttachmentRepository interface {
	// PutMetadata stores metadata in KV store
	PutMetadata(ctx context.Context, attachment *models.PastMeetingAttachment) error

	// GetMetadata retrieves only the metadata from KV store
	GetMetadata(ctx context.Context, attachmentUID string) (*models.PastMeetingAttachment, error)

	// ListByPastMeeting retrieves all attachment metadata for a past meeting
	ListByPastMeeting(ctx context.Context, pastMeetingUID string) ([]*models.PastMeetingAttachment, error)

	// Delete removes the metadata from KV store (file persists in Object Store)
	Delete(ctx context.Context, attachmentUID string) error
}
