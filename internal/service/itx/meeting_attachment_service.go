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
	attachmentClient domain.ITXMeetingAttachmentClient
}

// NewMeetingAttachmentService creates a new ITX meeting attachment service
func NewMeetingAttachmentService(attachmentClient domain.ITXMeetingAttachmentClient) *MeetingAttachmentService {
	return &MeetingAttachmentService{
		attachmentClient: attachmentClient,
	}
}

// CreateMeetingAttachment creates a new meeting attachment via ITX proxy
func (s *MeetingAttachmentService) CreateMeetingAttachment(ctx context.Context, meetingID string, req *itx.CreateMeetingAttachmentRequest) (*itx.MeetingAttachment, error) {
	return s.attachmentClient.CreateMeetingAttachment(ctx, meetingID, req)
}

// GetMeetingAttachment retrieves a meeting attachment by ID via ITX proxy
func (s *MeetingAttachmentService) GetMeetingAttachment(ctx context.Context, meetingID, attachmentID string) (*itx.MeetingAttachment, error) {
	return s.attachmentClient.GetMeetingAttachment(ctx, meetingID, attachmentID)
}

// UpdateMeetingAttachment updates a meeting attachment via ITX proxy
func (s *MeetingAttachmentService) UpdateMeetingAttachment(ctx context.Context, meetingID, attachmentID string, req *itx.UpdateMeetingAttachmentRequest) (*itx.MeetingAttachment, error) {
	return s.attachmentClient.UpdateMeetingAttachment(ctx, meetingID, attachmentID, req)
}

// DeleteMeetingAttachment deletes a meeting attachment via ITX proxy
func (s *MeetingAttachmentService) DeleteMeetingAttachment(ctx context.Context, meetingID, attachmentID string) error {
	return s.attachmentClient.DeleteMeetingAttachment(ctx, meetingID, attachmentID)
}

// CreateMeetingAttachmentPresignURL generates a presigned URL for meeting attachment upload via ITX proxy
func (s *MeetingAttachmentService) CreateMeetingAttachmentPresignURL(ctx context.Context, meetingID string, req *itx.CreateAttachmentPresignRequest) (*itx.MeetingAttachmentPresignResponse, error) {
	return s.attachmentClient.CreateMeetingAttachmentPresignURL(ctx, meetingID, req)
}

// GetMeetingAttachmentDownloadURL generates a presigned URL for meeting attachment download via ITX proxy
func (s *MeetingAttachmentService) GetMeetingAttachmentDownloadURL(ctx context.Context, meetingID, attachmentID string) (*itx.AttachmentDownloadResponse, error) {
	return s.attachmentClient.GetMeetingAttachmentDownloadURL(ctx, meetingID, attachmentID)
}
