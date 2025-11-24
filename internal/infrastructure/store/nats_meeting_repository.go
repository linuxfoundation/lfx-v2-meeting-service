// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package store

import (
	"context"
	"log/slog"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
)

// NatsMeetingRepository is the NATS KV store repository for meetings.
// It manages both MeetingBase and MeetingSettings using separate base repositories.
type NatsMeetingRepository struct {
	baseRepo     *NatsBaseRepository[models.MeetingBase]
	settingsRepo *NatsBaseRepository[models.MeetingSettings]
}

// NewNatsMeetingRepository creates a new NATS KV store repository for meetings.
func NewNatsMeetingRepository(meetingKV, settingsKV INatsKeyValue) *NatsMeetingRepository {
	baseRepo := NewNatsBaseRepository[models.MeetingBase](meetingKV, "meeting")
	settingsRepo := NewNatsBaseRepository[models.MeetingSettings](settingsKV, "meeting settings")

	return &NatsMeetingRepository{
		baseRepo:     baseRepo,
		settingsRepo: settingsRepo,
	}
}

// GetBase retrieves a meeting base by UID
func (r *NatsMeetingRepository) GetBase(ctx context.Context, meetingUID string) (*models.MeetingBase, error) {
	return r.baseRepo.Get(ctx, meetingUID)
}

// GetBaseWithRevision retrieves a meeting base with revision by UID
func (r *NatsMeetingRepository) GetBaseWithRevision(ctx context.Context, meetingUID string) (*models.MeetingBase, uint64, error) {
	return r.baseRepo.GetWithRevision(ctx, meetingUID)
}

// GetSettings retrieves meeting settings by UID
func (r *NatsMeetingRepository) GetSettings(ctx context.Context, meetingUID string) (*models.MeetingSettings, error) {
	return r.settingsRepo.Get(ctx, meetingUID)
}

// GetSettingsWithRevision retrieves meeting settings with revision by UID
func (r *NatsMeetingRepository) GetSettingsWithRevision(ctx context.Context, meetingUID string) (*models.MeetingSettings, uint64, error) {
	return r.settingsRepo.GetWithRevision(ctx, meetingUID)
}

// Exists checks if a meeting exists (checks base only)
func (r *NatsMeetingRepository) Exists(ctx context.Context, meetingUID string) (bool, error) {
	return r.baseRepo.Exists(ctx, meetingUID)
}

// ListAllBase lists all meeting bases
func (r *NatsMeetingRepository) ListAllBase(ctx context.Context) ([]*models.MeetingBase, error) {
	return r.baseRepo.ListEntities(ctx, "")
}

// ListAllSettings lists all meeting settings
func (r *NatsMeetingRepository) ListAllSettings(ctx context.Context) ([]*models.MeetingSettings, error) {
	return r.settingsRepo.ListEntities(ctx, "")
}

// ListAll lists all meeting bases and settings
func (r *NatsMeetingRepository) ListAll(ctx context.Context) ([]*models.MeetingBase, []*models.MeetingSettings, error) {
	bases, err := r.ListAllBase(ctx)
	if err != nil {
		return nil, nil, err
	}

	settings, err := r.ListAllSettings(ctx)
	if err != nil {
		return nil, nil, err
	}

	return bases, settings, nil
}

// ListByCommittee lists meetings by committee UID
func (r *NatsMeetingRepository) ListByCommittee(ctx context.Context, committeeUID string) ([]*models.MeetingBase, []*models.MeetingSettings, error) {
	// Get all meetings and filter by committee
	allBases, allSettings, err := r.ListAll(ctx)
	if err != nil {
		return nil, nil, err
	}

	var matchingBases []*models.MeetingBase
	var matchingSettings []*models.MeetingSettings

	// Filter bases by committee
	for _, base := range allBases {
		hasCommittee := false
		for _, committee := range base.Committees {
			if committee.UID == committeeUID {
				hasCommittee = true
				break
			}
		}
		if hasCommittee {
			matchingBases = append(matchingBases, base)
		}
	}

	// Filter settings by the matching base UIDs
	baseUIDs := make(map[string]bool)
	for _, base := range matchingBases {
		baseUIDs[base.UID] = true
	}

	for _, setting := range allSettings {
		if baseUIDs[setting.UID] {
			matchingSettings = append(matchingSettings, setting)
		}
	}

	return matchingBases, matchingSettings, nil
}

// ListByProject lists meeting bases by project UID
func (r *NatsMeetingRepository) ListByProject(ctx context.Context, projectUID string) ([]*models.MeetingBase, error) {
	// Get all meeting bases and filter by project
	allBases, err := r.ListAllBase(ctx)
	if err != nil {
		return nil, err
	}

	var matchingBases []*models.MeetingBase

	// Filter bases by project
	for _, base := range allBases {
		if base.ProjectUID == projectUID {
			matchingBases = append(matchingBases, base)
		}
	}

	return matchingBases, nil
}

// Create creates both meeting base and settings
func (r *NatsMeetingRepository) Create(ctx context.Context, meetingBase *models.MeetingBase, meetingSettings *models.MeetingSettings) error {
	// Create base first
	if err := r.baseRepo.Create(ctx, meetingBase.UID, meetingBase); err != nil {
		return err
	}

	// Create settings
	if err := r.settingsRepo.Create(ctx, meetingSettings.UID, meetingSettings); err != nil {
		// If settings creation fails, we should ideally rollback the base creation
		slog.ErrorContext(ctx, "failed to create meeting settings, base already created",
			logging.ErrKey, err, "meeting_uid", meetingBase.UID)
		return err
	}

	return nil
}

// UpdateBase updates meeting base
func (r *NatsMeetingRepository) UpdateBase(ctx context.Context, meetingBase *models.MeetingBase, revision uint64) error {
	return r.baseRepo.Update(ctx, meetingBase.UID, meetingBase, revision)
}

// UpdateSettings updates meeting settings
func (r *NatsMeetingRepository) UpdateSettings(ctx context.Context, meetingSettings *models.MeetingSettings, revision uint64) error {
	return r.settingsRepo.Update(ctx, meetingSettings.UID, meetingSettings, revision)
}

// Delete removes both meeting base and settings
func (r *NatsMeetingRepository) Delete(ctx context.Context, meetingUID string, revision uint64) error {
	// Delete settings first (less critical)
	if err := r.settingsRepo.DeleteWithoutRevision(ctx, meetingUID); err != nil {
		slog.WarnContext(ctx, "failed to delete meeting settings",
			logging.ErrKey, err, "meeting_uid", meetingUID)
		// Continue with base deletion
	}

	// Delete base
	return r.baseRepo.Delete(ctx, meetingUID, revision)
}

// GetByZoomMeetingID retrieves a meeting base by Zoom meeting ID
func (r *NatsMeetingRepository) GetByZoomMeetingID(ctx context.Context, zoomMeetingID string) (*models.MeetingBase, error) {
	// This would need an index in the full implementation
	// For now, list all and filter (inefficient but demonstrates concept)
	allBases, err := r.ListAllBase(ctx)
	if err != nil {
		return nil, err
	}

	for _, base := range allBases {
		if base.Platform == models.PlatformZoom && base.ZoomConfig != nil && base.ZoomConfig.MeetingID == zoomMeetingID {
			return base, nil
		}
	}

	return nil, domain.NewNotFoundError("meeting not found by Zoom meeting ID")
}
