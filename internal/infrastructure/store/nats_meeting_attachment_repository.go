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

// INatsObjectStore is a NATS Object Store interface for file storage
// This interface matches jetstream.ObjectStore and allows for mocking in tests
type INatsObjectStore interface {
	Put(ctx context.Context, obj jetstream.ObjectMeta, reader io.Reader) (*jetstream.ObjectInfo, error)
	Get(ctx context.Context, name string, opts ...jetstream.GetObjectOpt) (jetstream.ObjectResult, error)
	GetInfo(ctx context.Context, name string, opts ...jetstream.GetObjectInfoOpt) (*jetstream.ObjectInfo, error)
	Delete(ctx context.Context, name string) error
}

// NatsAttachmentRepository provides NATS storage operations for meeting attachments
// Metadata is stored in KV store, while actual files are stored in Object Store
type NatsAttachmentRepository struct {
	metadataKV  jetstream.KeyValue
	objectStore INatsObjectStore
}

// NewNatsAttachmentRepository creates a new repository for meeting attachments
// metadataKV stores attachment metadata, objectStore stores the actual files
func NewNatsAttachmentRepository(metadataKV jetstream.KeyValue, objectStore INatsObjectStore) *NatsAttachmentRepository {
	return &NatsAttachmentRepository{
		metadataKV:  metadataKV,
		objectStore: objectStore,
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
	objectMeta := jetstream.ObjectMeta{
		Name:        attachmentUID,
		Description: fmt.Sprintf("File for attachment %s", attachmentUID),
	}

	reader := bytes.NewReader(fileData)
	_, err := r.objectStore.Put(ctx, objectMeta, reader)
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
	result, err := r.objectStore.Get(ctx, attachmentUID)
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
