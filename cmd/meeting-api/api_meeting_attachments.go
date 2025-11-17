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

// CreateMeetingAttachment handles creation of a meeting attachment
func (s *MeetingsAPI) CreateMeetingAttachment(ctx context.Context, payload *meetingsvc.CreateMeetingAttachmentPayload) (*meetingsvc.MeetingAttachment, error) {
	if payload == nil {
		return nil, handleError(domain.NewValidationError("validation failed"))
	}

	sync := payload.XSync != nil && *payload.XSync

	// Parse username from JWT token
	username, err := s.authService.ParsePrincipal(ctx, *payload.BearerToken, slog.Default())
	if err != nil {
		slog.WarnContext(ctx, "failed to parse username from JWT token", logging.ErrKey, err)
		return nil, handleError(domain.NewValidationError("failed to parse username from authorization token"))
	}

	// Extract filename and content type from payload fields
	// (populated by the decoder during multipart form processing)
	fileName := "attachment"
	contentType := "application/octet-stream"

	if payload.FileName != nil && *payload.FileName != "" {
		fileName = *payload.FileName
	}
	if payload.FileContentType != nil && *payload.FileContentType != "" {
		contentType = *payload.FileContentType
	}

	createReq := service.ConvertCreateMeetingAttachmentPayloadToDomain(payload, username, fileName, contentType)

	attachment, err := s.attachmentService.CreateMeetingAttachment(ctx, createReq, sync)
	if err != nil {
		return nil, handleError(err)
	}

	return service.ConvertDomainToMeetingAttachmentResponse(attachment), nil
}

// GetMeetingAttachment retrieves a file attachment for download
func (s *MeetingsAPI) GetMeetingAttachment(ctx context.Context, payload *meetingsvc.GetMeetingAttachmentPayload) ([]byte, error) {
	if payload == nil {
		return nil, handleError(domain.NewValidationError("validation failed"))
	}

	attachment, fileData, err := s.attachmentService.GetAttachment(ctx, payload.MeetingUID, payload.UID)
	if err != nil {
		return nil, handleError(err)
	}

	// Store attachment metadata for the encoder to access
	// We use the request context's unique identifier to key the metadata
	if requestID, ok := ctx.Value(constants.RequestIDContextID).(string); ok {
		downloadMetadataStore.Store(requestID, attachment)
	}

	return fileData, nil
}

// GetMeetingAttachmentMetadata retrieves only the metadata for an attachment
func (s *MeetingsAPI) GetMeetingAttachmentMetadata(ctx context.Context, payload *meetingsvc.GetMeetingAttachmentMetadataPayload) (*meetingsvc.MeetingAttachment, error) {
	if payload == nil {
		return nil, handleError(domain.NewValidationError("validation failed"))
	}

	attachment, err := s.attachmentService.GetAttachmentMetadata(ctx, payload.MeetingUID, payload.UID)
	if err != nil {
		return nil, handleError(err)
	}

	return service.ConvertDomainToMeetingAttachmentResponse(attachment), nil
}

// DeleteMeetingAttachment handles deletion of a file attachment
func (s *MeetingsAPI) DeleteMeetingAttachment(ctx context.Context, payload *meetingsvc.DeleteMeetingAttachmentPayload) error {
	if payload == nil {
		return handleError(domain.NewValidationError("validation failed"))
	}

	sync := payload.XSync != nil && *payload.XSync

	err := s.attachmentService.DeleteAttachment(ctx, payload.MeetingUID, payload.UID, sync)
	if err != nil {
		return handleError(err)
	}

	return nil
}

// downloadMetadataStore temporarily stores attachment metadata for response encoding
var downloadMetadataStore sync.Map

// getMeetingDownloadAttachmentMetadata retrieves meeting attachment metadata for the response encoder
func getMeetingDownloadAttachmentMetadata(ctx context.Context) (*models.MeetingAttachment, bool) {
	if requestID, ok := ctx.Value(constants.RequestIDContextID).(string); ok {
		if value, ok := downloadMetadataStore.Load(requestID); ok {
			if attachment, ok := value.(*models.MeetingAttachment); ok {
				return attachment, true
			}
		}
	}
	return nil, false
}

// deleteMeetingDownloadAttachmentMetadata cleans up meeting attachment metadata after encoding
func deleteMeetingDownloadAttachmentMetadata(ctx context.Context) {
	if requestID, ok := ctx.Value(constants.RequestIDContextID).(string); ok {
		downloadMetadataStore.Delete(requestID)
	}
}
