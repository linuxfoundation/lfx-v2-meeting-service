// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
)

// NATS Key-Value store bucket name for past meeting summaries.
const (
	// KVStoreNamePastMeetingSummaries is the name of the KV store for past meeting summaries.
	KVStoreNamePastMeetingSummaries = "past-meeting-summaries"
)

// NatsPastMeetingSummaryRepository is the NATS KV store repository for past meeting summaries.
type NatsPastMeetingSummaryRepository struct {
	PastMeetingSummaries INatsKeyValue
}

// NewNatsPastMeetingSummaryRepository creates a new NATS KV store repository for past meeting summaries.
func NewNatsPastMeetingSummaryRepository(pastMeetingSummaries INatsKeyValue) *NatsPastMeetingSummaryRepository {
	return &NatsPastMeetingSummaryRepository{
		PastMeetingSummaries: pastMeetingSummaries,
	}
}

func (s *NatsPastMeetingSummaryRepository) get(ctx context.Context, summaryUID string) (jetstream.KeyValueEntry, error) {
	return s.PastMeetingSummaries.Get(ctx, summaryUID)
}

func (s *NatsPastMeetingSummaryRepository) unmarshal(ctx context.Context, entry jetstream.KeyValueEntry) (*models.PastMeetingSummary, error) {
	var summary models.PastMeetingSummary
	err := json.Unmarshal(entry.Value(), &summary)
	if err != nil {
		slog.ErrorContext(ctx, "error unmarshaling past meeting summary", logging.ErrKey, err)
		return nil, err
	}
	return &summary, nil
}

// Create creates a new past meeting summary in the NATS KV store.
func (s *NatsPastMeetingSummaryRepository) Create(ctx context.Context, summary *models.PastMeetingSummary) error {
	if summary.UID == "" {
		return fmt.Errorf("summary UID is required")
	}

	data, err := json.Marshal(summary)
	if err != nil {
		slog.ErrorContext(ctx, "error marshaling summary", logging.ErrKey, err, "summary_uid", summary.UID)
		return domain.ErrMarshal
	}

	_, err = s.PastMeetingSummaries.Put(ctx, summary.UID, data)
	if err != nil {
		slog.ErrorContext(ctx, "error creating summary in KV store", logging.ErrKey, err, "summary_uid", summary.UID)
		return domain.ErrInternal
	}

	slog.DebugContext(ctx, "created past meeting summary", "summary_uid", summary.UID, "past_meeting_uid", summary.PastMeetingUID)
	return nil
}

// Exists checks if a past meeting summary exists in the NATS KV store.
func (s *NatsPastMeetingSummaryRepository) Exists(ctx context.Context, summaryUID string) (bool, error) {
	_, err := s.get(ctx, summaryUID)
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			return false, nil
		}
		slog.ErrorContext(ctx, "error checking summary existence", logging.ErrKey, err, "summary_uid", summaryUID)
		return false, domain.ErrInternal
	}
	return true, nil
}

// Delete removes a past meeting summary from the NATS KV store.
func (s *NatsPastMeetingSummaryRepository) Delete(ctx context.Context, summaryUID string, revision uint64) error {
	err := s.PastMeetingSummaries.Delete(ctx, summaryUID)
	if err != nil {
		slog.ErrorContext(ctx, "error deleting summary from KV store", logging.ErrKey, err, "summary_uid", summaryUID)
		return domain.ErrInternal
	}

	slog.DebugContext(ctx, "deleted past meeting summary", "summary_uid", summaryUID)
	return nil
}

// Get retrieves a past meeting summary from the NATS KV store.
func (s *NatsPastMeetingSummaryRepository) Get(ctx context.Context, summaryUID string) (*models.PastMeetingSummary, error) {
	entry, err := s.get(ctx, summaryUID)
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			slog.DebugContext(ctx, "summary not found", "summary_uid", summaryUID)
			return nil, domain.ErrPastMeetingSummaryNotFound
		}
		slog.ErrorContext(ctx, "error getting summary from KV store", logging.ErrKey, err, "summary_uid", summaryUID)
		return nil, domain.ErrInternal
	}

	return s.unmarshal(ctx, entry)
}

// GetWithRevision retrieves a past meeting summary with its revision from the NATS KV store.
func (s *NatsPastMeetingSummaryRepository) GetWithRevision(ctx context.Context, summaryUID string) (*models.PastMeetingSummary, uint64, error) {
	entry, err := s.get(ctx, summaryUID)
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			slog.DebugContext(ctx, "summary not found", "summary_uid", summaryUID)
			return nil, 0, domain.ErrPastMeetingSummaryNotFound
		}
		slog.ErrorContext(ctx, "error getting summary from KV store", logging.ErrKey, err, "summary_uid", summaryUID)
		return nil, 0, domain.ErrInternal
	}

	summary, err := s.unmarshal(ctx, entry)
	if err != nil {
		return nil, 0, domain.ErrUnmarshal
	}

	return summary, entry.Revision(), nil
}

// Update updates a past meeting summary in the NATS KV store.
func (s *NatsPastMeetingSummaryRepository) Update(ctx context.Context, summary *models.PastMeetingSummary, revision uint64) error {
	if summary.UID == "" {
		return fmt.Errorf("summary UID is required")
	}

	data, err := json.Marshal(summary)
	if err != nil {
		slog.ErrorContext(ctx, "error marshaling summary", logging.ErrKey, err, "summary_uid", summary.UID)
		return domain.ErrMarshal
	}

	_, err = s.PastMeetingSummaries.Update(ctx, summary.UID, data, revision)
	if err != nil {
		slog.ErrorContext(ctx, "error updating summary in KV store", logging.ErrKey, err, "summary_uid", summary.UID, "revision", revision)
		return domain.ErrInternal
	}

	slog.DebugContext(ctx, "updated past meeting summary", "summary_uid", summary.UID, "past_meeting_uid", summary.PastMeetingUID, "revision", revision)
	return nil
}

// GetByPastMeetingUID retrieves a past meeting summary by past meeting UID.
func (s *NatsPastMeetingSummaryRepository) GetByPastMeetingUID(ctx context.Context, pastMeetingUID string) (*models.PastMeetingSummary, error) {
	summaries, err := s.ListByPastMeeting(ctx, pastMeetingUID)
	if err != nil {
		return nil, err
	}

	if len(summaries) == 0 {
		slog.DebugContext(ctx, "no summaries found for past meeting", "past_meeting_uid", pastMeetingUID)
		return nil, domain.ErrPastMeetingSummaryNotFound
	}

	// Return the first summary found (there could be multiple summaries per past meeting)
	return summaries[0], nil
}

// ListByPastMeeting retrieves all past meeting summaries for a given past meeting UID.
func (s *NatsPastMeetingSummaryRepository) ListByPastMeeting(ctx context.Context, pastMeetingUID string) ([]*models.PastMeetingSummary, error) {
	keys, err := s.PastMeetingSummaries.ListKeys(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "error listing keys from KV store", logging.ErrKey, err)
		return nil, domain.ErrInternal
	}

	var summaries []*models.PastMeetingSummary
	for key := range keys.Keys() {
		summary, err := s.Get(ctx, key)
		if err != nil {
			// Skip entries that can't be read but continue processing others
			slog.WarnContext(ctx, "failed to get summary during list operation", logging.ErrKey, err, "summary_uid", key)
			continue
		}

		if summary.PastMeetingUID == pastMeetingUID {
			summaries = append(summaries, summary)
		}
	}

	return summaries, nil
}

// ListAll retrieves all past meeting summaries from the NATS KV store.
func (s *NatsPastMeetingSummaryRepository) ListAll(ctx context.Context) ([]*models.PastMeetingSummary, error) {
	keys, err := s.PastMeetingSummaries.ListKeys(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "error listing keys from KV store", logging.ErrKey, err)
		return nil, domain.ErrInternal
	}

	var summaries []*models.PastMeetingSummary
	for key := range keys.Keys() {
		summary, err := s.Get(ctx, key)
		if err != nil {
			// Skip entries that can't be read but continue processing others
			slog.WarnContext(ctx, "failed to get summary during list operation", logging.ErrKey, err, "summary_uid", key)
			continue
		}

		summaries = append(summaries, summary)
	}

	return summaries, nil
}
