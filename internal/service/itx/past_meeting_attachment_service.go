// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package itx

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/models/itx"
)

// PastMeetingAttachmentService handles ITX past meeting attachment operations
type PastMeetingAttachmentService struct {
	auditStamper
	attachmentClient domain.ITXPastMeetingAttachmentClient
}

// NewPastMeetingAttachmentService creates a new ITX past meeting attachment service.
// userMetadata may be nil (e.g. when NATS is disabled), in which case created_by /
// updated_by are limited to the JWT-derived username/email rather than blocking the
// request.
func NewPastMeetingAttachmentService(attachmentClient domain.ITXPastMeetingAttachmentClient, userMetadata domain.UserMetadataReader) *PastMeetingAttachmentService {
	return &PastMeetingAttachmentService{
		auditStamper:     auditStamper{userMetadata: userMetadata},
		attachmentClient: attachmentClient,
	}
}

// CreatePastMeetingAttachment creates a new past meeting attachment via ITX proxy
func (s *PastMeetingAttachmentService) CreatePastMeetingAttachment(ctx context.Context, meetingAndOccurrenceID string, req *itx.CreatePastMeetingAttachmentRequest) (*itx.PastMeetingAttachment, error) {
	// Stamp created_by (with full profile enrichment when available) from the
	// authenticated principal, matching the enrichment level of the Meeting endpoint.
	req.CreatedBy = s.buildRequestingCreatedUpdatedBy(ctx)
	return s.attachmentClient.CreatePastMeetingAttachment(ctx, meetingAndOccurrenceID, req)
}

// GetPastMeetingAttachment retrieves a past meeting attachment by ID via ITX proxy
func (s *PastMeetingAttachmentService) GetPastMeetingAttachment(ctx context.Context, meetingAndOccurrenceID, attachmentID string) (*itx.PastMeetingAttachment, error) {
	return s.attachmentClient.GetPastMeetingAttachment(ctx, meetingAndOccurrenceID, attachmentID)
}

// UpdatePastMeetingAttachment updates a past meeting attachment via ITX proxy
func (s *PastMeetingAttachmentService) UpdatePastMeetingAttachment(ctx context.Context, meetingAndOccurrenceID, attachmentID string, req *itx.UpdatePastMeetingAttachmentRequest) error {
	// Stamp updated_by (with full profile enrichment when available) from the
	// authenticated principal so ITX overwrites the stored value instead of preserving
	// stale data.
	req.UpdatedBy = s.buildRequestingCreatedUpdatedBy(ctx)
	return s.attachmentClient.UpdatePastMeetingAttachment(ctx, meetingAndOccurrenceID, attachmentID, req)
}

// DeletePastMeetingAttachment deletes a past meeting attachment via ITX proxy
func (s *PastMeetingAttachmentService) DeletePastMeetingAttachment(ctx context.Context, meetingAndOccurrenceID, attachmentID string) error {
	return s.attachmentClient.DeletePastMeetingAttachment(ctx, meetingAndOccurrenceID, attachmentID)
}

// CreatePastMeetingAttachmentPresignURL generates a presigned URL for past meeting attachment upload via ITX proxy
func (s *PastMeetingAttachmentService) CreatePastMeetingAttachmentPresignURL(ctx context.Context, meetingAndOccurrenceID string, req *itx.CreateAttachmentPresignRequest) (*itx.PastMeetingAttachmentPresignResponse, error) {
	// The presign step is also a "create" in ITX's view (it persists an attachment
	// record with upload_status=ongoing), so stamp created_by here too.
	req.CreatedBy = s.buildRequestingCreatedUpdatedBy(ctx)
	return s.attachmentClient.CreatePastMeetingAttachmentPresignURL(ctx, meetingAndOccurrenceID, req)
}

// GetPastMeetingAttachmentDownloadURL generates a presigned URL for past meeting attachment download via ITX proxy
func (s *PastMeetingAttachmentService) GetPastMeetingAttachmentDownloadURL(ctx context.Context, meetingAndOccurrenceID, attachmentID string) (*itx.AttachmentDownloadResponse, error) {
	return s.attachmentClient.GetPastMeetingAttachmentDownloadURL(ctx, meetingAndOccurrenceID, attachmentID)
}
