// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package store

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
	"github.com/nats-io/nats.go/jetstream"
)

// NatsAttachmentRepository provides NATS storage operations for meeting attachments
// Metadata is stored in KV store, while actual files are stored in Object Store
type NatsAttachmentRepository struct {
	metadataKV      INatsKeyValue
	fileObjectStore INatsObjectStore
}

// NewNatsAttachmentRepository creates a new repository for meeting attachments
// metadataKV stores attachment metadata, objectStore stores the actual files
func NewNatsAttachmentRepository(metadataKV INatsKeyValue, objectStore INatsObjectStore) *NatsAttachmentRepository {
	return &NatsAttachmentRepository{
		metadataKV:      metadataKV,
		fileObjectStore: objectStore,
	}
}

// PutObject stores file in Object Store
func (r *NatsAttachmentRepository) PutObject(ctx context.Context, attachmentUID string, fileData []byte) error {
	if attachmentUID == "" {
		return domain.NewValidationError("attachment UID is required")
	}
	if len(fileData) == 0 {
		return domain.NewValidationError("file data is required")
	}

	// Store file in Object Store (no meeting reference in object store)
	// TODO: Put jetstream object API calls in a separate interface that is reusable across all the nats repos.
	objectMeta := jetstream.ObjectMeta{
		Name:        attachmentUID,
		Description: fmt.Sprintf("File for attachment %s", attachmentUID),
	}

	reader := bytes.NewReader(fileData)
	_, err := r.fileObjectStore.Put(ctx, objectMeta, reader)
	if err != nil {
		slog.ErrorContext(ctx, "error putting file to Object Store",
			logging.ErrKey, err,
			"attachment_uid", attachmentUID)
		return domain.NewInternalError(fmt.Sprintf("failed to upload attachment file: %v", err))
	}

	return nil
}

// PutMetadata stores metadata in KV store
func (r *NatsAttachmentRepository) PutMetadata(ctx context.Context, attachment *models.MeetingAttachment) error {
	if attachment == nil {
		return domain.NewValidationError("attachment cannot be nil")
	}
	if attachment.UID == "" {
		return domain.NewValidationError("attachment UID is required")
	}

	// Serialize metadata to JSON (includes meeting_uid field)
	metadataJSON, err := json.Marshal(attachment)
	if err != nil {
		slog.ErrorContext(ctx, "error marshaling attachment metadata", logging.ErrKey, err)
		return domain.NewInternalError(fmt.Sprintf("failed to marshal attachment metadata: %v", err))
	}

	// Store metadata in KV store
	_, err = r.metadataKV.Put(ctx, attachment.UID, metadataJSON)
	if err != nil {
		slog.ErrorContext(ctx, "error putting attachment metadata to KV store",
			logging.ErrKey, err,
			"attachment_uid", attachment.UID)
		return domain.NewInternalError(fmt.Sprintf("failed to store attachment metadata: %v", err))
	}

	return nil
}

// GetObject retrieves file from Object Store
func (r *NatsAttachmentRepository) GetObject(ctx context.Context, attachmentUID string) ([]byte, error) {
	if attachmentUID == "" {
		return nil, domain.NewValidationError("attachment UID is required")
	}

	// Get file from Object Store
	result, err := r.fileObjectStore.Get(ctx, attachmentUID)
	if err != nil {
		slog.ErrorContext(ctx, "error getting file from Object Store",
			logging.ErrKey, err,
			"attachment_uid", attachmentUID)
		return nil, domain.NewNotFoundError(fmt.Sprintf("attachment file not found: %s", attachmentUID))
	}
	defer func() {
		if closeErr := result.Close(); closeErr != nil {
			slog.ErrorContext(ctx, "error closing object result",
				logging.ErrKey, closeErr,
				"attachment_uid", attachmentUID)
		}
	}()

	// Read all data
	fileData, err := io.ReadAll(result)
	if err != nil {
		slog.ErrorContext(ctx, "error reading file data",
			logging.ErrKey, err,
			"attachment_uid", attachmentUID)
		return nil, domain.NewInternalError("failed to read attachment file")
	}

	return fileData, nil
}

// GetMetadata retrieves only the metadata from KV store
func (r *NatsAttachmentRepository) GetMetadata(ctx context.Context, attachmentUID string) (*models.MeetingAttachment, error) {
	if attachmentUID == "" {
		return nil, domain.NewValidationError("attachment UID is required")
	}

	// Get metadata from KV store
	entry, err := r.metadataKV.Get(ctx, attachmentUID)
	if err != nil {
		slog.ErrorContext(ctx, "error getting attachment metadata from KV store",
			logging.ErrKey, err,
			"attachment_uid", attachmentUID)
		return nil, domain.NewNotFoundError(fmt.Sprintf("attachment not found: %s", attachmentUID))
	}

	// Parse metadata
	var attachment models.MeetingAttachment
	if err := json.Unmarshal(entry.Value(), &attachment); err != nil {
		slog.ErrorContext(ctx, "error unmarshaling attachment metadata",
			logging.ErrKey, err,
			"attachment_uid", attachmentUID)
		return nil, domain.NewInternalError("failed to parse attachment metadata")
	}

	return &attachment, nil
}

// ListByMeeting retrieves all attachment metadata for a meeting
func (r *NatsAttachmentRepository) ListByMeeting(ctx context.Context, meetingUID string) ([]*models.MeetingAttachment, error) {
	if meetingUID == "" {
		return nil, domain.NewValidationError("meeting UID is required")
	}

	// Get all keys from the KV store
	keyLister, err := r.metadataKV.ListKeys(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "error listing keys from KV store",
			logging.ErrKey, err,
			"meeting_uid", meetingUID)
		return nil, domain.NewInternalError("failed to list attachments")
	}

	var keys []string
	for key := range keyLister.Keys() {
		keys = append(keys, key)
	}

	// Fetch and filter attachments
	var attachments []*models.MeetingAttachment
	for _, key := range keys {
		entry, err := r.metadataKV.Get(ctx, key)
		if err != nil {
			slog.WarnContext(ctx, "error getting attachment metadata",
				logging.ErrKey, err,
				"key", key)
			continue
		}

		var attachment models.MeetingAttachment
		if err := json.Unmarshal(entry.Value(), &attachment); err != nil {
			slog.WarnContext(ctx, "error unmarshaling attachment metadata",
				logging.ErrKey, err,
				"key", key)
			continue
		}

		// Filter by meeting UID
		if attachment.MeetingUID == meetingUID {
			attachmentCopy := attachment
			attachments = append(attachments, &attachmentCopy)
		}
	}

	slog.InfoContext(ctx, "listed meeting attachments",
		"meeting_uid", meetingUID,
		"count", len(attachments))

	return attachments, nil
}

// Delete removes only the metadata from KV store (file persists in Object Store)
func (r *NatsAttachmentRepository) Delete(ctx context.Context, attachmentUID string) error {
	if attachmentUID == "" {
		return domain.NewValidationError("attachment UID is required")
	}

	// Delete metadata from KV store
	err := r.metadataKV.Delete(ctx, attachmentUID)
	if err != nil {
		slog.ErrorContext(ctx, "error deleting attachment metadata from KV store",
			logging.ErrKey, err,
			"attachment_uid", attachmentUID)
		return domain.NewNotFoundError(fmt.Sprintf("attachment not found: %s", attachmentUID))
	}

	slog.InfoContext(ctx, "deleted attachment metadata (file preserved in Object Store)",
		"attachment_uid", attachmentUID)

	return nil
}
