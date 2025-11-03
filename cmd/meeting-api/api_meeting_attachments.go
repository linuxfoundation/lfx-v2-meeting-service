// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"log/slog"
	"sync"

	"github.com/linuxfoundation/lfx-v2-meeting-service/cmd/meeting-api/service"
	meetingsvc "github.com/linuxfoundation/lfx-v2-meeting-service/gen/meeting_service"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/constants"
)

// getAttachmentMetadata retrieves file metadata from the temporary store
func getAttachmentMetadata(payload *meetingsvc.UploadMeetingAttachmentPayload) (uploadMeetingAttachmentMetadata, bool) {
	if value, ok := attachmentMetadataStore.Load(payload); ok {
		if metadata, ok := value.(uploadMeetingAttachmentMetadata); ok {
			return metadata, true
		}
	}
	return uploadMeetingAttachmentMetadata{}, false
}

// deleteAttachmentMetadata removes file metadata from the temporary store
func deleteAttachmentMetadata(payload *meetingsvc.UploadMeetingAttachmentPayload) {
	attachmentMetadataStore.Delete(payload)
}

// UploadMeetingAttachment handles file upload for a meeting
func (s *MeetingsAPI) UploadMeetingAttachment(ctx context.Context, payload *meetingsvc.UploadMeetingAttachmentPayload) (*meetingsvc.MeetingAttachment, error) {
	if payload == nil {
		return nil, handleError(domain.NewValidationError("validation failed"))
	}

	// Parse username from JWT token
	username, err := s.authService.ParsePrincipal(ctx, *payload.BearerToken, slog.Default())
	if err != nil {
		slog.WarnContext(ctx, "failed to parse username from JWT token", logging.ErrKey, err)
		return nil, handleError(domain.NewValidationError("failed to parse username from authorization token"))
	}

	// Extract filename and content type from metadata store
	// (populated by the decoder during multipart form processing)
	fileName := "attachment"
	contentType := "application/octet-stream"

	if metadata, ok := getAttachmentMetadata(payload); ok {
		if metadata.FileName != "" {
			fileName = metadata.FileName
		}
		if metadata.ContentType != "" {
			contentType = metadata.ContentType
		}
		// Clean up the metadata after use
		deleteAttachmentMetadata(payload)
	}

	// Convert payload to domain request
	uploadReq := service.ConvertUploadAttachmentPayloadToDomain(payload, username, fileName, contentType)

	// Upload attachment via service
	attachment, err := s.attachmentService.UploadAttachment(ctx, uploadReq)
	if err != nil {
		return nil, handleError(err)
	}

	return service.ConvertDomainToAttachmentResponse(attachment), nil
}

// GetMeetingAttachment retrieves a file attachment for download
func (s *MeetingsAPI) GetMeetingAttachment(ctx context.Context, payload *meetingsvc.GetMeetingAttachmentPayload) ([]byte, error) {
	if payload == nil {
		return nil, handleError(domain.NewValidationError("validation failed"))
	}

	// Get attachment via service
	attachment, fileData, err := s.attachmentService.GetAttachment(ctx, payload.MeetingUID, payload.UID)
	if err != nil {
		return nil, handleError(err)
	}

	// Store attachment metadata for the encoder to access
	// We use the request context's unique identifier to key the metadata
	setDownloadAttachmentMetadata(ctx, attachment)

	return fileData, nil
}

// setDownloadAttachmentMetadata stores attachment metadata for the response encoder
func setDownloadAttachmentMetadata(ctx context.Context, attachment *models.MeetingAttachment) {
	// Use request ID as the key so we can retrieve it in the encoder
	if requestID, ok := ctx.Value(constants.RequestIDContextID).(string); ok {
		downloadMetadataStore.Store(requestID, attachment)
	}
}

// getDownloadAttachmentMetadata retrieves attachment metadata for the response encoder
func getDownloadAttachmentMetadata(ctx context.Context) (*models.MeetingAttachment, bool) {
	// Use request ID to look up the metadata
	if requestID, ok := ctx.Value(constants.RequestIDContextID).(string); ok {
		if value, ok := downloadMetadataStore.Load(requestID); ok {
			if attachment, ok := value.(*models.MeetingAttachment); ok {
				return attachment, true
			}
		}
	}
	return nil, false
}

// deleteDownloadAttachmentMetadata cleans up attachment metadata after encoding
func deleteDownloadAttachmentMetadata(ctx context.Context) {
	// Use request ID to delete the metadata
	if requestID, ok := ctx.Value(constants.RequestIDContextID).(string); ok {
		downloadMetadataStore.Delete(requestID)
	}
}

// downloadMetadataStore temporarily stores attachment metadata for response encoding
var downloadMetadataStore sync.Map

// GetMeetingAttachmentMetadata retrieves only the metadata for an attachment
func (s *MeetingsAPI) GetMeetingAttachmentMetadata(ctx context.Context, payload *meetingsvc.GetMeetingAttachmentMetadataPayload) (*meetingsvc.MeetingAttachment, error) {
	if payload == nil {
		return nil, handleError(domain.NewValidationError("validation failed"))
	}

	// Get attachment metadata via service
	attachment, err := s.attachmentService.GetAttachmentMetadata(ctx, payload.MeetingUID, payload.UID)
	if err != nil {
		return nil, handleError(err)
	}

	return service.ConvertDomainToAttachmentResponse(attachment), nil
}

// DeleteMeetingAttachment handles deletion of a file attachment
func (s *MeetingsAPI) DeleteMeetingAttachment(ctx context.Context, payload *meetingsvc.DeleteMeetingAttachmentPayload) error {
	if payload == nil {
		return handleError(domain.NewValidationError("validation failed"))
	}

	// Delete attachment via service
	err := s.attachmentService.DeleteAttachment(ctx, payload.MeetingUID, payload.UID)
	if err != nil {
		return handleError(err)
	}

	return nil
}
