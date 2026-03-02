// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"log/slog"

	"github.com/linuxfoundation/lfx-v2-meeting-service/cmd/meeting-api/service"
	meetingservice "github.com/linuxfoundation/lfx-v2-meeting-service/gen/meeting_service"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
)

// CreateItxMeetingAttachment creates a meeting attachment via ITX proxy
func (s *MeetingsAPI) CreateItxMeetingAttachment(ctx context.Context, p *meetingservice.CreateItxMeetingAttachmentPayload) (*meetingservice.ITXMeetingAttachment, error) {
	// Parse JWT token to get username
	username, err := s.authService.ParsePrincipal(ctx, *p.BearerToken, slog.Default())
	if err != nil {
		slog.WarnContext(ctx, "failed to parse username from JWT token", logging.ErrKey, err)
		return nil, handleError(domain.NewValidationError("failed to parse username from authorization token"))
	}

	// Convert Goa payload to ITX request
	req := service.ConvertGoaToITXCreateMeetingAttachment(p, username)

	// Call ITX service
	resp, err := s.itxMeetingAttachmentService.CreateMeetingAttachment(ctx, p.MeetingID, req)
	if err != nil {
		return nil, handleError(err)
	}

	// Convert ITX response to Goa response
	goaResp := service.ConvertITXMeetingAttachmentToGoa(resp)
	return goaResp, nil
}

// GetItxMeetingAttachment retrieves a meeting attachment via ITX proxy
func (s *MeetingsAPI) GetItxMeetingAttachment(ctx context.Context, p *meetingservice.GetItxMeetingAttachmentPayload) (*meetingservice.ITXMeetingAttachment, error) {
	// Call ITX service
	resp, err := s.itxMeetingAttachmentService.GetMeetingAttachment(ctx, p.MeetingID, p.AttachmentID)
	if err != nil {
		return nil, handleError(err)
	}

	// Convert ITX response to Goa response
	goaResp := service.ConvertITXMeetingAttachmentToGoa(resp)
	return goaResp, nil
}

// UpdateItxMeetingAttachment updates a meeting attachment via ITX proxy
func (s *MeetingsAPI) UpdateItxMeetingAttachment(ctx context.Context, p *meetingservice.UpdateItxMeetingAttachmentPayload) error {
	// Parse JWT token to get username
	username, err := s.authService.ParsePrincipal(ctx, *p.BearerToken, slog.Default())
	if err != nil {
		slog.WarnContext(ctx, "failed to parse username from JWT token", logging.ErrKey, err)
		return handleError(domain.NewValidationError("failed to parse username from authorization token"))
	}

	// Convert Goa payload to ITX request
	req := service.ConvertGoaToITXUpdateMeetingAttachment(p, username)

	// Call ITX service
	err = s.itxMeetingAttachmentService.UpdateMeetingAttachment(ctx, p.MeetingID, p.AttachmentID, req)
	if err != nil {
		return handleError(err)
	}

	return nil
}

// DeleteItxMeetingAttachment deletes a meeting attachment via ITX proxy
func (s *MeetingsAPI) DeleteItxMeetingAttachment(ctx context.Context, p *meetingservice.DeleteItxMeetingAttachmentPayload) error {
	// Call ITX service
	err := s.itxMeetingAttachmentService.DeleteMeetingAttachment(ctx, p.MeetingID, p.AttachmentID)
	if err != nil {
		return handleError(err)
	}

	return nil
}

// CreateItxMeetingAttachmentPresign generates a presigned URL for meeting attachment upload via ITX proxy
func (s *MeetingsAPI) CreateItxMeetingAttachmentPresign(ctx context.Context, p *meetingservice.CreateItxMeetingAttachmentPresignPayload) (*meetingservice.ITXMeetingAttachmentPresignResponse, error) {
	// Parse JWT token to get username
	username, err := s.authService.ParsePrincipal(ctx, *p.BearerToken, slog.Default())
	if err != nil {
		slog.WarnContext(ctx, "failed to parse username from JWT token", logging.ErrKey, err)
		return nil, handleError(domain.NewValidationError("failed to parse username from authorization token"))
	}

	// Convert Goa payload to ITX request
	req := service.ConvertGoaToITXCreateMeetingAttachmentPresign(p, username)

	// Call ITX service
	resp, err := s.itxMeetingAttachmentService.CreateMeetingAttachmentPresignURL(ctx, p.MeetingID, req)
	if err != nil {
		return nil, handleError(err)
	}

	// Convert ITX response to Goa response
	goaResp := service.ConvertITXMeetingAttachmentPresignToGoa(resp)
	return goaResp, nil
}

// GetItxMeetingAttachmentDownload generates a presigned URL for meeting attachment download via ITX proxy
func (s *MeetingsAPI) GetItxMeetingAttachmentDownload(ctx context.Context, p *meetingservice.GetItxMeetingAttachmentDownloadPayload) (*meetingservice.ITXAttachmentDownloadResponse, error) {
	// Call ITX service
	resp, err := s.itxMeetingAttachmentService.GetMeetingAttachmentDownloadURL(ctx, p.MeetingID, p.AttachmentID)
	if err != nil {
		return nil, handleError(err)
	}

	// Convert ITX response to Goa response
	goaResp := service.ConvertITXAttachmentDownloadToGoa(resp)
	return goaResp, nil
}
