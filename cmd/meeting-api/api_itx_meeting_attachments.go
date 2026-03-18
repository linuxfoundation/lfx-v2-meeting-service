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
	username, err := s.authService.ParsePrincipal(ctx, *p.BearerToken, slog.Default())
	if err != nil {
		slog.WarnContext(ctx, "failed to parse username from JWT token", logging.ErrKey, err)
		return nil, handleError(domain.NewValidationError("failed to parse username from authorization token"))
	}
	req := service.ConvertGoaToITXCreateMeetingAttachment(p, username)
	resp, err := s.itxMeetingAttachmentService.CreateMeetingAttachment(ctx, p.MeetingID, req)
	if err != nil {
		return nil, handleError(err)
	}
	return service.ConvertITXMeetingAttachmentToGoa(resp), nil
}

// GetItxMeetingAttachment retrieves a meeting attachment via ITX proxy
func (s *MeetingsAPI) GetItxMeetingAttachment(ctx context.Context, p *meetingservice.GetItxMeetingAttachmentPayload) (*meetingservice.ITXMeetingAttachment, error) {
	resp, err := s.itxMeetingAttachmentService.GetMeetingAttachment(ctx, p.MeetingID, p.AttachmentID)
	if err != nil {
		return nil, handleError(err)
	}
	return service.ConvertITXMeetingAttachmentToGoa(resp), nil
}

// UpdateItxMeetingAttachment updates a meeting attachment via ITX proxy
func (s *MeetingsAPI) UpdateItxMeetingAttachment(ctx context.Context, p *meetingservice.UpdateItxMeetingAttachmentPayload) error {
	username, err := s.authService.ParsePrincipal(ctx, *p.BearerToken, slog.Default())
	if err != nil {
		slog.WarnContext(ctx, "failed to parse username from JWT token", logging.ErrKey, err)
		return handleError(domain.NewValidationError("failed to parse username from authorization token"))
	}
	req := service.ConvertGoaToITXUpdateMeetingAttachment(p, username)
	err = s.itxMeetingAttachmentService.UpdateMeetingAttachment(ctx, p.MeetingID, p.AttachmentID, req)
	if err != nil {
		return handleError(err)
	}
	return nil
}

// DeleteItxMeetingAttachment deletes a meeting attachment via ITX proxy
func (s *MeetingsAPI) DeleteItxMeetingAttachment(ctx context.Context, p *meetingservice.DeleteItxMeetingAttachmentPayload) error {
	err := s.itxMeetingAttachmentService.DeleteMeetingAttachment(ctx, p.MeetingID, p.AttachmentID)
	if err != nil {
		return handleError(err)
	}
	return nil
}

// CreateItxMeetingAttachmentPresign generates a presigned URL for meeting attachment upload via ITX proxy
func (s *MeetingsAPI) CreateItxMeetingAttachmentPresign(ctx context.Context, p *meetingservice.CreateItxMeetingAttachmentPresignPayload) (*meetingservice.ITXMeetingAttachmentPresignResponse, error) {
	username, err := s.authService.ParsePrincipal(ctx, *p.BearerToken, slog.Default())
	if err != nil {
		slog.WarnContext(ctx, "failed to parse username from JWT token", logging.ErrKey, err)
		return nil, handleError(domain.NewValidationError("failed to parse username from authorization token"))
	}
	req := service.ConvertGoaToITXCreateMeetingAttachmentPresign(p, username)
	resp, err := s.itxMeetingAttachmentService.CreateMeetingAttachmentPresignURL(ctx, p.MeetingID, req)
	if err != nil {
		return nil, handleError(err)
	}
	return service.ConvertITXMeetingAttachmentPresignToGoa(resp), nil
}

// GetItxMeetingAttachmentDownload generates a presigned URL for meeting attachment download via ITX proxy
func (s *MeetingsAPI) GetItxMeetingAttachmentDownload(ctx context.Context, p *meetingservice.GetItxMeetingAttachmentDownloadPayload) (*meetingservice.ITXAttachmentDownloadResponse, error) {
	resp, err := s.itxMeetingAttachmentService.GetMeetingAttachmentDownloadURL(ctx, p.MeetingID, p.AttachmentID)
	if err != nil {
		return nil, handleError(err)
	}
	return service.ConvertITXAttachmentDownloadToGoa(resp), nil
}
