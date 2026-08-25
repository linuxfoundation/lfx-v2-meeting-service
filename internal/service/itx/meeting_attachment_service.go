// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package itx

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/models/itx"
)

// MeetingAttachmentService handles ITX meeting attachment operations
type MeetingAttachmentService struct {
	auditStamper
	attachmentClient domain.ITXMeetingAttachmentClient
}

// NewMeetingAttachmentService creates a new ITX meeting attachment service.
// userMetadata may be nil (e.g. when NATS is disabled), in which case created_by /
// updated_by are limited to the JWT-derived username/email rather than blocking the
// request.
func NewMeetingAttachmentService(attachmentClient domain.ITXMeetingAttachmentClient, userMetadata domain.UserMetadataReader) *MeetingAttachmentService {
	return &MeetingAttachmentService{
		auditStamper:     auditStamper{userMetadata: userMetadata},
		attachmentClient: attachmentClient,
	}
}

// CreateMeetingAttachment creates a new meeting attachment via ITX proxy
func (s *MeetingAttachmentService) CreateMeetingAttachment(ctx context.Context, meetingID string, req *itx.CreateMeetingAttachmentRequest) (*itx.MeetingAttachment, error) {
	// Stamp created_by (with full profile enrichment when available) from the
	// authenticated principal, matching the enrichment level of the Meeting endpoint.
	req.CreatedBy = s.buildRequestingCreatedUpdatedBy(ctx)
	return s.attachmentClient.CreateMeetingAttachment(ctx, meetingID, req)
}

// GetMeetingAttachment retrieves a meeting attachment by ID via ITX proxy
func (s *MeetingAttachmentService) GetMeetingAttachment(ctx context.Context, meetingID, attachmentID string) (*itx.MeetingAttachment, error) {
	return s.attachmentClient.GetMeetingAttachment(ctx, meetingID, attachmentID)
}

// UpdateMeetingAttachment updates a meeting attachment via ITX proxy
func (s *MeetingAttachmentService) UpdateMeetingAttachment(ctx context.Context, meetingID, attachmentID string, req *itx.UpdateMeetingAttachmentRequest) error {
	// Stamp updated_by (with full profile enrichment when available) from the
	// authenticated principal so ITX overwrites the stored value instead of preserving
	// stale data.
	req.UpdatedBy = s.buildRequestingCreatedUpdatedBy(ctx)
	return s.attachmentClient.UpdateMeetingAttachment(ctx, meetingID, attachmentID, req)
}

// DeleteMeetingAttachment deletes a meeting attachment via ITX proxy
func (s *MeetingAttachmentService) DeleteMeetingAttachment(ctx context.Context, meetingID, attachmentID string) error {
	return s.attachmentClient.DeleteMeetingAttachment(ctx, meetingID, attachmentID)
}

// CreateMeetingAttachmentPresignURL generates a presigned URL for meeting attachment upload via ITX proxy
func (s *MeetingAttachmentService) CreateMeetingAttachmentPresignURL(ctx context.Context, meetingID string, req *itx.CreateAttachmentPresignRequest) (*itx.MeetingAttachmentPresignResponse, error) {
	// The presign step is also a "create" in ITX's view (it persists an attachment
	// record with upload_status=ongoing), so stamp created_by here too.
	req.CreatedBy = s.buildRequestingCreatedUpdatedBy(ctx)
	return s.attachmentClient.CreateMeetingAttachmentPresignURL(ctx, meetingID, req)
}

// GetMeetingAttachmentDownloadURL generates a presigned URL for meeting attachment download via ITX proxy
func (s *MeetingAttachmentService) GetMeetingAttachmentDownloadURL(ctx context.Context, meetingID, attachmentID string) (*itx.AttachmentDownloadResponse, error) {
	return s.attachmentClient.GetMeetingAttachmentDownloadURL(ctx, meetingID, attachmentID)
}
