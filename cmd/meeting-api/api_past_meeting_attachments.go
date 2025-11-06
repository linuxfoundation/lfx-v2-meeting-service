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

// GetPastMeetingAttachments gets all attachments for a past meeting
func (s *MeetingsAPI) GetPastMeetingAttachments(ctx context.Context, payload *meetingsvc.GetPastMeetingAttachmentsPayload) (*meetingsvc.GetPastMeetingAttachmentsResult, error) {
	if payload == nil || payload.UID == nil {
		return nil, handleError(domain.NewValidationError("validation failed"))
	}

	attachments, err := s.pastMeetingAttachmentService.ListPastMeetingAttachments(ctx, *payload.UID)
	if err != nil {
		return nil, handleError(err)
	}

	var responseAttachments []*meetingsvc.PastMeetingAttachment
	for _, attachment := range attachments {
		responseAttachments = append(responseAttachments, service.ConvertDomainToPastMeetingAttachmentResponse(attachment))
	}

	return &meetingsvc.GetPastMeetingAttachmentsResult{
		Attachments:  responseAttachments,
		CacheControl: nil,
	}, nil
}

// CreatePastMeetingAttachment creates a new past meeting attachment
func (s *MeetingsAPI) CreatePastMeetingAttachment(ctx context.Context, payload *meetingsvc.CreatePastMeetingAttachmentPayload) (*meetingsvc.PastMeetingAttachment, error) {
	if payload == nil {
		return nil, handleError(domain.NewValidationError("validation failed"))
	}

	// Parse username from JWT token
	username, err := s.authService.ParsePrincipal(ctx, *payload.BearerToken, slog.Default())
	if err != nil {
		slog.WarnContext(ctx, "failed to parse username from JWT token", logging.ErrKey, err)
		return nil, handleError(domain.NewValidationError("failed to parse username from authorization token"))
	}

	// Extract filename and content type from payload fields if uploading a file
	// (populated by the decoder during multipart form processing)
	fileName := "attachment"
	contentType := "application/octet-stream"

	if payload.FileName != nil && *payload.FileName != "" {
		fileName = *payload.FileName
	}
	if payload.FileContentType != nil && *payload.FileContentType != "" {
		contentType = *payload.FileContentType
	}

	req := service.ConvertCreatePastMeetingAttachmentPayloadToDomain(payload, username, fileName, contentType)

	attachment, err := s.pastMeetingAttachmentService.CreatePastMeetingAttachment(ctx, req)
	if err != nil {
		return nil, handleError(err)
	}

	return service.ConvertDomainToPastMeetingAttachmentResponse(attachment), nil
}

// GetPastMeetingAttachment downloads a past meeting attachment file
func (s *MeetingsAPI) GetPastMeetingAttachment(ctx context.Context, payload *meetingsvc.GetPastMeetingAttachmentPayload) ([]byte, error) {
	if payload == nil {
		return nil, handleError(domain.NewValidationError("validation failed"))
	}

	attachment, fileData, err := s.pastMeetingAttachmentService.GetPastMeetingAttachment(ctx, payload.PastMeetingUID, payload.UID)
	if err != nil {
		return nil, handleError(err)
	}

	// Store attachment metadata for the encoder to access
	// We use the request context's unique identifier to key the metadata
	if requestID, ok := ctx.Value(constants.RequestIDContextID).(string); ok {
		pastMeetingDownloadMetadataStore.Store(requestID, attachment)
	}

	return fileData, nil
}

// GetPastMeetingAttachmentMetadata retrieves only the metadata for a past meeting attachment
func (s *MeetingsAPI) GetPastMeetingAttachmentMetadata(ctx context.Context, payload *meetingsvc.GetPastMeetingAttachmentMetadataPayload) (*meetingsvc.PastMeetingAttachment, error) {
	if payload == nil {
		return nil, handleError(domain.NewValidationError("validation failed"))
	}

	attachment, err := s.pastMeetingAttachmentService.GetPastMeetingAttachmentMetadata(ctx, payload.PastMeetingUID, payload.UID)
	if err != nil {
		return nil, handleError(err)
	}

	return service.ConvertDomainToPastMeetingAttachmentResponse(attachment), nil
}

// DeletePastMeetingAttachment deletes a past meeting attachment
func (s *MeetingsAPI) DeletePastMeetingAttachment(ctx context.Context, payload *meetingsvc.DeletePastMeetingAttachmentPayload) error {
	if payload == nil {
		return handleError(domain.NewValidationError("validation failed"))
	}

	err := s.pastMeetingAttachmentService.DeletePastMeetingAttachment(ctx, payload.PastMeetingUID, payload.UID)
	if err != nil {
		return handleError(err)
	}

	return nil
}

// pastMeetingDownloadMetadataStore temporarily stores attachment metadata for response encoding
var pastMeetingDownloadMetadataStore sync.Map

// getPastMeetingDownloadAttachmentMetadata retrieves attachment metadata for the response encoder
func getPastMeetingDownloadAttachmentMetadata(ctx context.Context) (*models.PastMeetingAttachment, bool) {
	// Use request ID to look up the metadata
	if requestID, ok := ctx.Value(constants.RequestIDContextID).(string); ok {
		if value, ok := pastMeetingDownloadMetadataStore.Load(requestID); ok {
			if attachment, ok := value.(*models.PastMeetingAttachment); ok {
				return attachment, true
			}
		}
	}
	return nil, false
}

// deletePastMeetingDownloadAttachmentMetadata cleans up attachment metadata after encoding
func deletePastMeetingDownloadAttachmentMetadata(ctx context.Context) {
	// Use request ID to delete the metadata
	if requestID, ok := ctx.Value(constants.RequestIDContextID).(string); ok {
		pastMeetingDownloadMetadataStore.Delete(requestID)
	}
}
