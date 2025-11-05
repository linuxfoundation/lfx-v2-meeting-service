// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package store

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
	"github.com/nats-io/nats.go/jetstream"
)

// NatsPastMeetingAttachmentRepository provides NATS storage operations for past meeting attachments
// Metadata is stored in KV store, while actual files are stored in the shared Object Store
type NatsPastMeetingAttachmentRepository struct {
	kv jetstream.KeyValue
}

// NewNatsPastMeetingAttachmentRepository creates a new repository for past meeting attachments
func NewNatsPastMeetingAttachmentRepository(kv jetstream.KeyValue) *NatsPastMeetingAttachmentRepository {
	return &NatsPastMeetingAttachmentRepository{
		kv: kv,
	}
}

// PutMetadata stores metadata in KV store
func (r *NatsPastMeetingAttachmentRepository) PutMetadata(ctx context.Context, attachment *models.PastMeetingAttachment) error {
	if attachment == nil {
		return domain.NewValidationError("attachment cannot be nil")
	}
	if attachment.UID == "" {
		return domain.NewValidationError("attachment UID is required")
	}
	if attachment.PastMeetingUID == "" {
		return domain.NewValidationError("past meeting UID is required")
	}

	// Serialize metadata to JSON
	metadataJSON, err := json.Marshal(attachment)
	if err != nil {
		slog.ErrorContext(ctx, "error marshaling past meeting attachment metadata", logging.ErrKey, err)
		return domain.NewInternalError(fmt.Sprintf("failed to marshal attachment metadata: %v", err))
	}

	// Store metadata in KV store with tags
	_, err = r.kv.Put(ctx, attachment.UID, metadataJSON)
	if err != nil {
		slog.ErrorContext(ctx, "error putting past meeting attachment metadata to KV store",
			logging.ErrKey, err,
			"attachment_uid", attachment.UID,
			"past_meeting_uid", attachment.PastMeetingUID)
		return domain.NewInternalError(fmt.Sprintf("failed to store attachment metadata: %v", err))
	}

	slog.InfoContext(ctx, "stored past meeting attachment metadata",
		"attachment_uid", attachment.UID,
		"past_meeting_uid", attachment.PastMeetingUID,
		"source_object_uid", attachment.SourceObjectUID)

	return nil
}

// GetMetadata retrieves only the metadata from KV store
func (r *NatsPastMeetingAttachmentRepository) GetMetadata(ctx context.Context, attachmentUID string) (*models.PastMeetingAttachment, error) {
	if attachmentUID == "" {
		return nil, domain.NewValidationError("attachment UID is required")
	}

	// Get metadata from KV store
	entry, err := r.kv.Get(ctx, attachmentUID)
	if err != nil {
		slog.ErrorContext(ctx, "error getting past meeting attachment metadata from KV store",
			logging.ErrKey, err,
			"attachment_uid", attachmentUID)
		return nil, domain.NewNotFoundError(fmt.Sprintf("attachment not found: %s", attachmentUID))
	}

	// Parse metadata
	var attachment models.PastMeetingAttachment
	if err := json.Unmarshal(entry.Value(), &attachment); err != nil {
		slog.ErrorContext(ctx, "error unmarshaling past meeting attachment metadata",
			logging.ErrKey, err,
			"attachment_uid", attachmentUID)
		return nil, domain.NewInternalError("failed to parse attachment metadata")
	}

	return &attachment, nil
}

// ListByPastMeeting retrieves all attachment metadata for a past meeting
func (r *NatsPastMeetingAttachmentRepository) ListByPastMeeting(ctx context.Context, pastMeetingUID string) ([]*models.PastMeetingAttachment, error) {
	if pastMeetingUID == "" {
		return nil, domain.NewValidationError("past meeting UID is required")
	}

	// Get all keys from the KV store
	keys, err := r.kv.Keys(ctx)
	if err != nil {
		// Check if error is due to empty bucket (no keys found)
		if err == jetstream.ErrNoKeysFound {
			slog.DebugContext(ctx, "no attachments found in KV store",
				"past_meeting_uid", pastMeetingUID)
			return []*models.PastMeetingAttachment{}, nil
		}
		slog.ErrorContext(ctx, "error listing keys from KV store",
			logging.ErrKey, err,
			"past_meeting_uid", pastMeetingUID)
		return nil, domain.NewInternalError("failed to list attachments")
	}

	// Fetch and filter attachments
	var attachments []*models.PastMeetingAttachment
	for _, key := range keys {
		entry, err := r.kv.Get(ctx, key)
		if err != nil {
			slog.WarnContext(ctx, "error getting attachment metadata",
				logging.ErrKey, err,
				"key", key)
			continue
		}

		var attachment models.PastMeetingAttachment
		if err := json.Unmarshal(entry.Value(), &attachment); err != nil {
			slog.WarnContext(ctx, "error unmarshaling attachment metadata",
				logging.ErrKey, err,
				"key", key)
			continue
		}

		// Filter by past meeting UID
		if attachment.PastMeetingUID == pastMeetingUID {
			attachments = append(attachments, &attachment)
		}
	}

	slog.InfoContext(ctx, "listed past meeting attachments",
		"past_meeting_uid", pastMeetingUID,
		"count", len(attachments))

	return attachments, nil
}

// Delete removes only the metadata from KV store (file persists in Object Store)
func (r *NatsPastMeetingAttachmentRepository) Delete(ctx context.Context, attachmentUID string) error {
	if attachmentUID == "" {
		return domain.NewValidationError("attachment UID is required")
	}

	// Delete metadata from KV store
	err := r.kv.Delete(ctx, attachmentUID)
	if err != nil {
		slog.ErrorContext(ctx, "error deleting past meeting attachment metadata from KV store",
			logging.ErrKey, err,
			"attachment_uid", attachmentUID)
		return domain.NewNotFoundError(fmt.Sprintf("attachment not found: %s", attachmentUID))
	}

	slog.InfoContext(ctx, "deleted past meeting attachment metadata (file preserved in Object Store)",
		"attachment_uid", attachmentUID)

	return nil
}
