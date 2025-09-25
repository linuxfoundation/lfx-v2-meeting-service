// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package store

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
)

// NatsPastMeetingSummaryRepository is the NATS KV store repository for past meeting summaries.
type NatsPastMeetingSummaryRepository struct {
	*NatsBaseRepository[models.PastMeetingSummary]
}

// NewNatsPastMeetingSummaryRepository creates a new NATS KV store repository for past meeting summaries.
func NewNatsPastMeetingSummaryRepository(kvStore INatsKeyValue) *NatsPastMeetingSummaryRepository {
	baseRepo := NewNatsBaseRepository[models.PastMeetingSummary](kvStore, "past meeting summary")

	return &NatsPastMeetingSummaryRepository{
		NatsBaseRepository: baseRepo,
	}
}

// Create creates a new past meeting summary
func (r *NatsPastMeetingSummaryRepository) Create(ctx context.Context, summary *models.PastMeetingSummary) error {
	if summary.UID == "" {
		return fmt.Errorf("summary UID is required")
	}

	return r.NatsBaseRepository.Create(ctx, summary.UID, summary)
}

// Get retrieves a past meeting summary by UID
func (r *NatsPastMeetingSummaryRepository) Get(ctx context.Context, summaryUID string) (*models.PastMeetingSummary, error) {
	summary, _, err := r.GetWithRevision(ctx, summaryUID)
	return summary, err
}

// GetWithRevision retrieves a past meeting summary with revision by UID
func (r *NatsPastMeetingSummaryRepository) GetWithRevision(ctx context.Context, summaryUID string) (*models.PastMeetingSummary, uint64, error) {
	return r.NatsBaseRepository.GetWithRevision(ctx, summaryUID)
}

// Update updates an existing past meeting summary
func (r *NatsPastMeetingSummaryRepository) Update(ctx context.Context, summary *models.PastMeetingSummary, revision uint64) error {
	if summary.UID == "" {
		return fmt.Errorf("summary UID is required")
	}

	return r.NatsBaseRepository.Update(ctx, summary.UID, summary, revision)
}

// ListAll lists all past meeting summaries
func (r *NatsPastMeetingSummaryRepository) ListAll(ctx context.Context) ([]*models.PastMeetingSummary, error) {
	return r.ListEntities(ctx, "")
}

// GetByPastMeetingUID retrieves a past meeting summary by past meeting UID.
func (r *NatsPastMeetingSummaryRepository) GetByPastMeetingUID(ctx context.Context, pastMeetingUID string) (*models.PastMeetingSummary, error) {
	summaries, err := r.ListByPastMeeting(ctx, pastMeetingUID)
	if err != nil {
		return nil, err
	}

	if len(summaries) == 0 {
		slog.DebugContext(ctx, "no summaries found for past meeting", "past_meeting_uid", pastMeetingUID)
		return nil, domain.NewNotFoundError("summary not found")
	}

	// Return the first summary found (there could be multiple summaries per past meeting)
	return summaries[0], nil
}

// ListByPastMeeting retrieves all past meeting summaries for a given past meeting UID.
func (r *NatsPastMeetingSummaryRepository) ListByPastMeeting(ctx context.Context, pastMeetingUID string) ([]*models.PastMeetingSummary, error) {
	allSummaries, err := r.ListAll(ctx)
	if err != nil {
		return nil, err
	}

	var matchingSummaries []*models.PastMeetingSummary
	for _, summary := range allSummaries {
		if summary.PastMeetingUID == pastMeetingUID {
			matchingSummaries = append(matchingSummaries, summary)
		}
	}

	return matchingSummaries, nil
}
