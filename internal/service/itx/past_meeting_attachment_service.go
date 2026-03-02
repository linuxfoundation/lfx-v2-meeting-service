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
	attachmentClient domain.ITXPastMeetingAttachmentClient
}

// NewPastMeetingAttachmentService creates a new ITX past meeting attachment service
func NewPastMeetingAttachmentService(attachmentClient domain.ITXPastMeetingAttachmentClient) *PastMeetingAttachmentService {
	return &PastMeetingAttachmentService{
		attachmentClient: attachmentClient,
	}
}

// CreatePastMeetingAttachment creates a new past meeting attachment via ITX proxy
func (s *PastMeetingAttachmentService) CreatePastMeetingAttachment(ctx context.Context, meetingAndOccurrenceID string, req *itx.CreatePastMeetingAttachmentRequest) (*itx.PastMeetingAttachment, error) {
	return s.attachmentClient.CreatePastMeetingAttachment(ctx, meetingAndOccurrenceID, req)
}

// GetPastMeetingAttachment retrieves a past meeting attachment by ID via ITX proxy
func (s *PastMeetingAttachmentService) GetPastMeetingAttachment(ctx context.Context, meetingAndOccurrenceID, attachmentID string) (*itx.PastMeetingAttachment, error) {
	return s.attachmentClient.GetPastMeetingAttachment(ctx, meetingAndOccurrenceID, attachmentID)
}

// UpdatePastMeetingAttachment updates a past meeting attachment via ITX proxy
func (s *PastMeetingAttachmentService) UpdatePastMeetingAttachment(ctx context.Context, meetingAndOccurrenceID, attachmentID string, req *itx.UpdatePastMeetingAttachmentRequest) error {
	return s.attachmentClient.UpdatePastMeetingAttachment(ctx, meetingAndOccurrenceID, attachmentID, req)
}

// DeletePastMeetingAttachment deletes a past meeting attachment via ITX proxy
func (s *PastMeetingAttachmentService) DeletePastMeetingAttachment(ctx context.Context, meetingAndOccurrenceID, attachmentID string) error {
	return s.attachmentClient.DeletePastMeetingAttachment(ctx, meetingAndOccurrenceID, attachmentID)
}

// CreatePastMeetingAttachmentPresignURL generates a presigned URL for past meeting attachment upload via ITX proxy
func (s *PastMeetingAttachmentService) CreatePastMeetingAttachmentPresignURL(ctx context.Context, meetingAndOccurrenceID string, req *itx.CreateAttachmentPresignRequest) (*itx.PastMeetingAttachmentPresignResponse, error) {
	return s.attachmentClient.CreatePastMeetingAttachmentPresignURL(ctx, meetingAndOccurrenceID, req)
}

// GetPastMeetingAttachmentDownloadURL generates a presigned URL for past meeting attachment download via ITX proxy
func (s *PastMeetingAttachmentService) GetPastMeetingAttachmentDownloadURL(ctx context.Context, meetingAndOccurrenceID, attachmentID string) (*itx.AttachmentDownloadResponse, error) {
	return s.attachmentClient.GetPastMeetingAttachmentDownloadURL(ctx, meetingAndOccurrenceID, attachmentID)
}
