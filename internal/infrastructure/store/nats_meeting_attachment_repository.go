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

// NatsAttachmentRepository provides NATS Object Store operations for meeting attachments
type NatsAttachmentRepository struct {
	objectStore INatsObjectStore
}

// NewNatsAttachmentRepository creates a new repository for meeting attachments using NATS Object Store
func NewNatsAttachmentRepository(objectStore INatsObjectStore) *NatsAttachmentRepository {
	return &NatsAttachmentRepository{
		objectStore: objectStore,
	}
}

// Put uploads a file attachment with metadata to the object store
func (r *NatsAttachmentRepository) Put(ctx context.Context, attachment *models.MeetingAttachment, fileData []byte) error {
	if attachment == nil {
		return domain.NewValidationError("attachment cannot be nil")
	}
	if attachment.UID == "" {
		return domain.NewValidationError("attachment UID is required")
	}
	if attachment.MeetingUID == "" {
		return domain.NewValidationError("meeting UID is required")
	}

	// Serialize metadata to JSON
	metadataJSON, err := json.Marshal(attachment)
	if err != nil {
		slog.ErrorContext(ctx, "error marshaling attachment metadata", logging.ErrKey, err)
		return domain.NewInternalError(fmt.Sprintf("failed to marshal attachment metadata: %v", err))
	}

	// Create object metadata with custom headers for attachment metadata
	objectMeta := jetstream.ObjectMeta{
		Name:        attachment.UID,
		Description: attachment.Description,
		Headers: map[string][]string{
			"Attachment-Metadata": {string(metadataJSON)},
			"Content-Type":        {attachment.ContentType},
		},
	}

	// Upload the file
	reader := bytes.NewReader(fileData)
	_, err = r.objectStore.Put(ctx, objectMeta, reader)
	if err != nil {
		slog.ErrorContext(ctx, "error putting object to NATS Object Store",
			logging.ErrKey, err,
			"attachment_uid", attachment.UID,
			"meeting_uid", attachment.MeetingUID)
		return domain.NewInternalError(fmt.Sprintf("failed to upload attachment: %v", err))
	}

	return nil
}

// Get retrieves a file attachment and its metadata from the object store
func (r *NatsAttachmentRepository) Get(ctx context.Context, attachmentUID string) (*models.MeetingAttachment, []byte, error) {
	if attachmentUID == "" {
		return nil, nil, domain.NewValidationError("attachment UID is required")
	}

	// First get the info to extract metadata
	info, err := r.objectStore.GetInfo(ctx, attachmentUID)
	if err != nil {
		slog.ErrorContext(ctx, "error getting object info from NATS Object Store",
			logging.ErrKey, err,
			"attachment_uid", attachmentUID)
		return nil, nil, domain.NewNotFoundError(fmt.Sprintf("attachment not found: %s", attachmentUID))
	}

	// Parse metadata from headers
	attachment, err := r.parseAttachmentMetadata(info)
	if err != nil {
		return nil, nil, err
	}

	// Get the object data
	result, err := r.objectStore.Get(ctx, attachmentUID)
	if err != nil {
		slog.ErrorContext(ctx, "error getting object from NATS Object Store",
			logging.ErrKey, err,
			"attachment_uid", attachmentUID)
		return nil, nil, domain.NewNotFoundError(fmt.Sprintf("attachment not found: %s", attachmentUID))
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
		slog.ErrorContext(ctx, "error reading object data",
			logging.ErrKey, err,
			"attachment_uid", attachmentUID)
		return nil, nil, domain.NewInternalError("failed to read attachment data")
	}

	return attachment, fileData, nil
}

// GetInfo retrieves only the metadata for an attachment without downloading the file
func (r *NatsAttachmentRepository) GetInfo(ctx context.Context, attachmentUID string) (*models.MeetingAttachment, error) {
	if attachmentUID == "" {
		return nil, domain.NewValidationError("attachment UID is required")
	}

	info, err := r.objectStore.GetInfo(ctx, attachmentUID)
	if err != nil {
		slog.ErrorContext(ctx, "error getting object info from NATS Object Store",
			logging.ErrKey, err,
			"attachment_uid", attachmentUID)
		return nil, domain.NewNotFoundError(fmt.Sprintf("attachment not found: %s", attachmentUID))
	}

	return r.parseAttachmentMetadata(info)
}

// Delete removes a file attachment from the object store
func (r *NatsAttachmentRepository) Delete(ctx context.Context, attachmentUID string) error {
	if attachmentUID == "" {
		return domain.NewValidationError("attachment UID is required")
	}

	err := r.objectStore.Delete(ctx, attachmentUID)
	if err != nil {
		slog.ErrorContext(ctx, "error deleting object from NATS Object Store",
			logging.ErrKey, err,
			"attachment_uid", attachmentUID)
		return domain.NewNotFoundError(fmt.Sprintf("attachment not found: %s", attachmentUID))
	}

	return nil
}

// parseAttachmentMetadata extracts attachment metadata from object info headers
func (r *NatsAttachmentRepository) parseAttachmentMetadata(info *jetstream.ObjectInfo) (*models.MeetingAttachment, error) {
	metadataJSON, ok := info.Headers["Attachment-Metadata"]
	if !ok || len(metadataJSON) == 0 {
		return nil, domain.NewInternalError("attachment metadata not found in object headers")
	}

	var attachment models.MeetingAttachment
	if err := json.Unmarshal([]byte(metadataJSON[0]), &attachment); err != nil {
		return nil, domain.NewInternalError("failed to parse attachment metadata")
	}

	return &attachment, nil
}
