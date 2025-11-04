// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"log/slog"

	"github.com/linuxfoundation/lfx-v2-meeting-service/cmd/meeting-api/service"
	meetingsvc "github.com/linuxfoundation/lfx-v2-meeting-service/gen/meeting_service"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
)

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

	// Extract filename and content type from metadata store if uploading a file
	fileName := ""
	contentType := "application/octet-stream"

	if len(payload.File) > 0 {
		// Get metadata from the temporary store (populated by multipart decoder)
		if metadata, ok := getPastMeetingAttachmentMetadata(payload); ok {
			if metadata.FileName != "" {
				fileName = metadata.FileName
			}
			if metadata.ContentType != "" {
				contentType = metadata.ContentType
			}
			// Clean up the metadata after use
			deletePastMeetingAttachmentMetadata(payload)
		} else {
			fileName = "attachment"
		}
	}

	// Build request
	req := &models.CreatePastMeetingAttachmentRequest{
		PastMeetingUID: payload.PastMeetingUID,
		Username:       username,
		FileName:       fileName,
		ContentType:    contentType,
		FileData:       payload.File,
	}

	if payload.Description != nil {
		req.Description = *payload.Description
	}

	if payload.SourceObjectUID != nil {
		req.SourceObjectUID = *payload.SourceObjectUID
	}

	// Create attachment via service
	attachment, err := s.pastMeetingAttachmentService.CreatePastMeetingAttachment(ctx, req)
	if err != nil {
		return nil, handleError(err)
	}

	return service.ConvertDomainToPastMeetingAttachmentResponse(attachment), nil
}

// uploadPastMeetingAttachmentMetadata stores file metadata during multipart processing
type uploadPastMeetingAttachmentMetadata struct {
	FileName    string
	ContentType string
}

// getPastMeetingAttachmentMetadata retrieves file metadata from the temporary store
func getPastMeetingAttachmentMetadata(payload *meetingsvc.CreatePastMeetingAttachmentPayload) (uploadPastMeetingAttachmentMetadata, bool) {
	if value, ok := pastMeetingAttachmentMetadataStore.Load(payload); ok {
		if metadata, ok := value.(uploadPastMeetingAttachmentMetadata); ok {
			return metadata, true
		}
	}
	return uploadPastMeetingAttachmentMetadata{}, false
}

// deletePastMeetingAttachmentMetadata removes file metadata from the temporary store
func deletePastMeetingAttachmentMetadata(payload *meetingsvc.CreatePastMeetingAttachmentPayload) {
	pastMeetingAttachmentMetadataStore.Delete(payload)
}
