// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/utils"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/concurrent"
)

const (
	// MaxFileAttachmentSize is the maximum size for file attachments (100MB)
	MaxFileAttachmentSize = 100 * 1024 * 1024
)

// MeetingAttachmentService implements meeting attachment operations
type MeetingAttachmentService struct {
	attachmentRepository domain.MeetingAttachmentRepository
	meetingRepository    domain.MeetingRepository
	indexSender          domain.MeetingAttachmentIndexSender
	accessSender         domain.MeetingAttachmentAccessSender
}

// NewMeetingAttachmentService creates a new MeetingAttachmentService.
func NewMeetingAttachmentService(
	attachmentRepository domain.MeetingAttachmentRepository,
	meetingRepository domain.MeetingRepository,
	indexSender domain.MeetingAttachmentIndexSender,
	accessSender domain.MeetingAttachmentAccessSender,
) *MeetingAttachmentService {
	return &MeetingAttachmentService{
		attachmentRepository: attachmentRepository,
		meetingRepository:    meetingRepository,
		indexSender:          indexSender,
		accessSender:         accessSender,
	}
}

// ServiceReady checks if the service is ready for use.
func (s *MeetingAttachmentService) ServiceReady() bool {
	return s.attachmentRepository != nil && s.meetingRepository != nil
}

func (s *MeetingAttachmentService) validateCreateMeetingAttachmentRequest(req *models.CreateMeetingAttachmentRequest) error {
	if req == nil {
		return domain.NewValidationError("request is nil")
	}

	// Validate inputs
	if req.MeetingUID == "" {
		return domain.NewValidationError("meeting UID is required")
	}
	if req.Username == "" {
		return domain.NewValidationError("username is required")
	}

	// Validate type
	if req.Type != models.AttachmentTypeFile && req.Type != models.AttachmentTypeLink {
		return domain.NewValidationError("type must be either 'file' or 'link'")
	}

	// Type-specific validation
	switch req.Type {
	case models.AttachmentTypeLink:
		if req.Link == "" {
			return domain.NewValidationError("link is required when type is 'link'")
		}
		// Link-type attachments should not have file-related fields
		if len(req.FileData) > 0 {
			return domain.NewValidationError("link-type attachments cannot have file data")
		}
	case models.AttachmentTypeFile:
		if req.FileName == "" {
			return domain.NewValidationError("file name is required when type is 'file'")
		}
		if len(req.FileData) == 0 {
			return domain.NewValidationError("file data is required when type is 'file'")
		}
		// Check file size
		if len(req.FileData) > MaxFileAttachmentSize {
			return domain.NewValidationError(fmt.Sprintf("file size exceeds maximum allowed size of %d bytes", MaxFileAttachmentSize))
		}
	}

	// Validate name is provided
	if req.Name == "" {
		return domain.NewValidationError("name is required")
	}

	return nil
}

// CreateMeetingAttachment creates a file or link attachment for a meeting
func (s *MeetingAttachmentService) CreateMeetingAttachment(ctx context.Context, req *models.CreateMeetingAttachmentRequest, sync bool) (*models.MeetingAttachment, error) {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "service not initialized", logging.PriorityCritical())
		return nil, domain.NewUnavailableError("attachment service is not ready")
	}

	// Validate request
	if err := s.validateCreateMeetingAttachmentRequest(req); err != nil {
		return nil, err
	}

	ctx = logging.AppendCtx(ctx, slog.String("meeting_uid", req.MeetingUID))

	// Verify meeting exists
	_, err := s.meetingRepository.GetBase(ctx, req.MeetingUID)
	if err != nil {
		return nil, err
	}

	// Create attachment metadata
	now := time.Now()
	var attachment *models.MeetingAttachment

	switch req.Type {
	case models.AttachmentTypeLink:
		// Create link-type attachment (metadata only, no file storage)
		attachment = &models.MeetingAttachment{
			UID:         uuid.New().String(),
			MeetingUID:  req.MeetingUID,
			Type:        models.AttachmentTypeLink,
			Link:        req.Link,
			Name:        req.Name,
			UploadedBy:  req.Username,
			UploadedAt:  &now,
			Description: req.Description,
		}
	case models.AttachmentTypeFile:
		// Create file-type attachment
		attachment = &models.MeetingAttachment{
			UID:         uuid.New().String(),
			MeetingUID:  req.MeetingUID,
			Type:        models.AttachmentTypeFile,
			Name:        req.Name,
			FileName:    req.FileName,
			FileSize:    int64(len(req.FileData)),
			ContentType: req.ContentType,
			UploadedBy:  req.Username,
			UploadedAt:  &now,
			Description: req.Description,
		}

		// Upload file to Object Store first
		if err := s.attachmentRepository.PutObject(ctx, attachment.UID, req.FileData); err != nil {
			slog.ErrorContext(ctx, "failed to upload attachment file", logging.ErrKey, err, "attachment_uid", attachment.UID)
			return nil, err
		}
	}

	// Create metadata in KV store
	if err := s.attachmentRepository.PutMetadata(ctx, attachment); err != nil {
		slog.ErrorContext(ctx, "failed to create attachment metadata", logging.ErrKey, err, "attachment_uid", attachment.UID, logging.PriorityCritical())
		// Note: File remains in Object Store even if metadata creation fails
		// This is acceptable as orphaned files can be cleaned up separately
		return nil, err
	}

	slog.InfoContext(ctx, "created attachment",
		"attachment_uid", attachment.UID,
		"type", attachment.Type,
		"file_size", attachment.FileSize,
		"link", attachment.Link)

	// Send indexer and access control messages concurrently
	pool := concurrent.NewWorkerPool(2)
	errors := pool.RunAll(ctx,
		func() error {
			if s.indexSender == nil {
				return nil
			}
			if err := s.indexSender.SendIndexMeetingAttachment(ctx, models.ActionCreated, *attachment, sync); err != nil {
				slog.WarnContext(ctx, "failed to send index message for attachment",
					logging.ErrKey, err,
					"attachment_uid", attachment.UID)
				return err
			}
			return nil
		},
		func() error {
			if s.accessSender == nil {
				return nil
			}
			accessMsg := models.MeetingAttachmentAccessMessage{
				UID:        attachment.UID,
				MeetingUID: attachment.MeetingUID,
			}
			if err := s.accessSender.SendUpdateAccessMeetingAttachment(ctx, accessMsg, sync); err != nil {
				slog.WarnContext(ctx, "failed to send access control message for attachment",
					logging.ErrKey, err,
					"attachment_uid", attachment.UID)
				return err
			}
			return nil
		},
	)

	// Log any errors but don't fail the operation - attachment was created successfully
	if len(errors) > 0 {
		slog.WarnContext(ctx, "some messaging operations failed for attachment",
			"attachment_uid", attachment.UID,
			"error_count", len(errors))
	}

	return attachment, nil
}

// GetAttachment retrieves a file attachment by UID
func (s *MeetingAttachmentService) GetAttachment(ctx context.Context, meetingUID, attachmentUID string) (*models.MeetingAttachment, []byte, error) {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "service not initialized", logging.PriorityCritical())
		return nil, nil, domain.NewUnavailableError("attachment service is not ready")
	}

	ctx = logging.AppendCtx(ctx, slog.String("meeting_uid", meetingUID))
	ctx = logging.AppendCtx(ctx, slog.String("attachment_uid", attachmentUID))

	// Validate inputs
	if meetingUID == "" {
		return nil, nil, domain.NewValidationError("meeting UID is required")
	}
	if attachmentUID == "" {
		return nil, nil, domain.NewValidationError("attachment UID is required")
	}

	// Get attachment metadata
	attachmentMetadata, err := s.attachmentRepository.GetMetadata(ctx, attachmentUID)
	if err != nil {
		return nil, nil, err
	}

	// Verify attachment belongs to the requested meeting
	if attachmentMetadata.MeetingUID != meetingUID {
		return nil, nil, domain.NewNotFoundError("attachment not found for this meeting")
	}

	// Cannot download link-type attachments
	if attachmentMetadata.Type == models.AttachmentTypeLink {
		slog.WarnContext(ctx, "attempted to download link-type attachment",
			"attachment_uid", attachmentUID,
			"link", attachmentMetadata.Link)
		return nil, nil, domain.NewValidationError("cannot download link-type attachments")
	}

	// Get file data
	attachmentFileData, err := s.attachmentRepository.GetObject(ctx, attachmentUID)
	if err != nil {
		return nil, nil, err
	}

	return attachmentMetadata, attachmentFileData, nil
}

// GetAttachmentMetadata retrieves only the metadata for an attachment without downloading the file
func (s *MeetingAttachmentService) GetAttachmentMetadata(ctx context.Context, meetingUID, attachmentUID string) (*models.MeetingAttachment, error) {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "service not initialized", logging.PriorityCritical())
		return nil, domain.NewUnavailableError("attachment service is not ready")
	}

	ctx = logging.AppendCtx(ctx, slog.String("meeting_uid", meetingUID))
	ctx = logging.AppendCtx(ctx, slog.String("attachment_uid", attachmentUID))

	// Validate inputs
	if meetingUID == "" {
		return nil, domain.NewValidationError("meeting UID is required")
	}
	if attachmentUID == "" {
		return nil, domain.NewValidationError("attachment UID is required")
	}

	// Get attachment metadata
	attachment, err := s.attachmentRepository.GetMetadata(ctx, attachmentUID)
	if err != nil {
		return nil, err
	}

	// Verify attachment belongs to the requested meeting
	if attachment.MeetingUID != meetingUID {
		return nil, domain.NewNotFoundError("attachment not found for this meeting")
	}

	return attachment, nil
}

func (s *MeetingAttachmentService) GetMeetingAttachmentsForEmail(ctx context.Context, meetingUID string) ([]*models.MeetingAttachment, []*domain.EmailAttachment, error) {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "service not initialized", logging.PriorityCritical())
		return nil, nil, domain.NewUnavailableError("attachment service is not ready")
	}

	ctx = logging.AppendCtx(ctx, slog.String("meeting_uid", meetingUID))

	// Fetch meeting attachments to include in the email
	var meetingAttachments []*models.MeetingAttachment
	var fileAttachments []*domain.EmailAttachment
	if s.attachmentRepository != nil {
		meetingAttachments, _ = s.attachmentRepository.ListByMeeting(ctx, meetingUID)
		// Ignore error - attachments are optional, email should still be sent without them

		// Fetch file data for file-type attachments to include as email attachments
		for _, attachment := range meetingAttachments {
			if attachment.Type == models.AttachmentTypeFile {
				fileData, err := s.attachmentRepository.GetObject(ctx, attachment.UID)
				if err != nil {
					slog.WarnContext(ctx, "failed to fetch file attachment data, skipping",
						"attachment_uid", attachment.UID,
						"error", err)
					continue
				}

				// Encode file data to base64
				encodedContent := base64.StdEncoding.EncodeToString(fileData)
				fileAttachments = append(fileAttachments, &domain.EmailAttachment{
					Filename:    attachment.FileName,
					ContentType: attachment.ContentType,
					Content:     encodedContent,
				})
			}
		}
	}

	return meetingAttachments, fileAttachments, nil
}

// DeleteAttachment deletes a file attachment by UID
func (s *MeetingAttachmentService) DeleteAttachment(ctx context.Context, meetingUID, attachmentUID string, sync bool) error {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "service not initialized", logging.PriorityCritical())
		return domain.NewUnavailableError("attachment service is not ready")
	}

	ctx = logging.AppendCtx(ctx, slog.String("meeting_uid", meetingUID))
	ctx = logging.AppendCtx(ctx, slog.String("attachment_uid", attachmentUID))

	// Validate inputs
	if meetingUID == "" {
		return domain.NewValidationError("meeting UID is required")
	}
	if attachmentUID == "" {
		return domain.NewValidationError("attachment UID is required")
	}

	// First verify the attachment exists and belongs to this meeting
	attachment, err := s.attachmentRepository.GetMetadata(ctx, attachmentUID)
	if err != nil {
		return err
	}

	if attachment.MeetingUID != meetingUID {
		return domain.NewNotFoundError("attachment not found for this meeting")
	}

	// Delete the attachment
	if err := s.attachmentRepository.Delete(ctx, attachmentUID); err != nil {
		slog.ErrorContext(ctx, "failed to delete attachment", logging.ErrKey, err, "attachment_uid", attachmentUID)
		return err
	}

	slog.InfoContext(ctx, "deleted attachment", "attachment_uid", attachmentUID)

	// Send indexer and access control delete messages concurrently
	pool := concurrent.NewWorkerPool(2)
	errors := pool.RunAll(ctx,
		func() error {
			if s.indexSender == nil {
				return nil
			}
			if err := s.indexSender.SendDeleteIndexMeetingAttachment(ctx, attachmentUID, sync); err != nil {
				slog.WarnContext(ctx, "failed to send delete index message for attachment",
					logging.ErrKey, err,
					"attachment_uid", attachmentUID)
				return err
			}
			return nil
		},
		func() error {
			if s.accessSender == nil {
				return nil
			}
			if err := s.accessSender.SendDeleteAccessMeetingAttachment(ctx, attachmentUID, sync); err != nil {
				slog.WarnContext(ctx, "failed to send delete access control message for attachment",
					logging.ErrKey, err,
					"attachment_uid", attachmentUID)
				return err
			}
			return nil
		},
	)

	// Log any errors but don't fail the operation - attachment was deleted successfully
	if len(errors) > 0 {
		slog.WarnContext(ctx, "some messaging operations failed for attachment deletion",
			"attachment_uid", attachmentUID,
			"error_count", len(errors))
	}

	return nil
}

// GetAttachmentsByMeetingUID retrieves all attachments for a given meeting UID
func (s *MeetingAttachmentService) GetAttachmentsByMeetingUID(ctx context.Context, meetingUID string) ([]*models.MeetingAttachment, error) {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "service not initialized", logging.PriorityCritical())
		return nil, domain.NewUnavailableError("attachment service is not ready")
	}

	ctx = logging.AppendCtx(ctx, slog.String("meeting_uid", meetingUID))

	// Validate input
	if meetingUID == "" {
		return nil, domain.NewValidationError("meeting UID is required")
	}

	// Get all attachments for the meeting
	attachments, err := s.attachmentRepository.ListByMeeting(ctx, meetingUID)
	if err != nil {
		return nil, err
	}

	return attachments, nil
}

// ValidateAndExtractLinks extracts URLs from a description for validation purposes.
// This is used for early validation before creating meetings or attachments.
// Returns the list of extracted URLs and an error if extraction fails.
// This method does NOT create any attachments - it only extracts URLs.
// Note: An empty list of URLs is not an error - it just means no URLs were found.
func (s *MeetingAttachmentService) ValidateAndExtractLinks(ctx context.Context, description string) ([]string, error) {
	if description == "" {
		return []string{}, nil
	}

	// Extract URLs from description
	// The extraction process itself shouldn't fail unless there's a critical error
	extractedURLs := utils.ExtractURLs(description)

	if len(extractedURLs) > 0 {
		slog.DebugContext(ctx, "validated and extracted URLs from description", "url_count", len(extractedURLs))
	}

	return extractedURLs, nil
}

// FilterNewLinks filters out URLs that already exist as link attachments
// Returns only the URLs that are not already present in the existing attachments
func (s *MeetingAttachmentService) FilterNewLinks(existingAttachments []*models.MeetingAttachment, extractedURLs []string) []string {
	if len(extractedURLs) == 0 {
		return []string{}
	}

	// Create a set of existing link URLs for fast lookup
	existingLinks := make(map[string]bool)
	for _, attachment := range existingAttachments {
		if attachment.Type == models.AttachmentTypeLink && attachment.Link != "" {
			existingLinks[attachment.Link] = true
		}
	}

	// Filter out URLs that already exist
	newLinks := make([]string, 0, len(extractedURLs))
	for _, url := range extractedURLs {
		if !existingLinks[url] {
			newLinks = append(newLinks, url)
		}
	}

	return newLinks
}

// CreateLinksFromDescription extracts URLs from a meeting description and creates link attachments
// for any URLs that don't already exist as attachments. This method is called during meeting
// creation and updates to automatically capture agenda links.
//
// Parameters:
//   - ctx: Context for the operation
//   - meetingUID: The UID of the meeting to attach links to
//   - description: The meeting description text to extract URLs from
//   - username: The username to attribute link creation to
//   - sync: Whether to send messaging operations synchronously
//
// Returns an error if:
//   - The service is not ready
//   - URL extraction fails
//   - Fetching existing attachments fails
//   - Any link attachment creation fails
func (s *MeetingAttachmentService) CreateLinksFromDescription(ctx context.Context, meetingUID, description, username string, sync bool) error {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "service not initialized", logging.PriorityCritical())
		return domain.NewUnavailableError("attachment service is not ready")
	}

	ctx = logging.AppendCtx(ctx, slog.String("meeting_uid", meetingUID))

	// Extract URLs from description
	extractedURLs := utils.ExtractURLs(description)
	if len(extractedURLs) == 0 {
		slog.DebugContext(ctx, "no URLs found in meeting description")
		return nil
	}

	slog.InfoContext(ctx, "extracted URLs from meeting description", "url_count", len(extractedURLs))

	// Fetch existing attachments to check for duplicates
	existingAttachments, err := s.GetAttachmentsByMeetingUID(ctx, meetingUID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to fetch existing attachments for duplicate check", logging.ErrKey, err)
		return domain.NewInternalError("failed to fetch existing attachments for duplicate check", err)
	}

	// Filter out URLs that already exist as attachments
	newLinks := s.FilterNewLinks(existingAttachments, extractedURLs)
	if len(newLinks) == 0 {
		slog.DebugContext(ctx, "all extracted URLs already exist as attachments")
		return nil
	}

	slog.InfoContext(ctx, "creating link attachments for new URLs", "new_link_count", len(newLinks))

	// Create link attachments for each new URL
	for _, url := range newLinks {
		// Extract domain from URL to use as the name
		domainName := utils.ExtractDomain(url)

		req := &models.CreateMeetingAttachmentRequest{
			MeetingUID:  meetingUID,
			Type:        models.AttachmentTypeLink,
			Link:        url,
			Name:        domainName,
			Description: "",
			Username:    username, // Username is required and must be provided by caller
		}

		attachment, err := s.CreateMeetingAttachment(ctx, req, sync)
		if err != nil {
			slog.ErrorContext(ctx, "failed to create link attachment from description",
				logging.ErrKey, err,
				"url", url,
				"meeting_uid", meetingUID)
			return domain.NewInternalError("failed to create link attachment for URL", err)
		}

		slog.InfoContext(ctx, "created link attachment from description",
			"attachment_uid", attachment.UID,
			"url", url)
	}

	return nil
}
