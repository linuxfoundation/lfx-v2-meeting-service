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

// CreateItxPastMeetingAttachment creates a past meeting attachment via ITX proxy
func (s *MeetingsAPI) CreateItxPastMeetingAttachment(ctx context.Context, p *meetingservice.CreateItxPastMeetingAttachmentPayload) (*meetingservice.ITXPastMeetingAttachment, error) {
	username, err := s.authService.ParsePrincipal(ctx, *p.BearerToken, slog.Default())
	if err != nil {
		slog.WarnContext(ctx, "failed to parse username from JWT token", logging.ErrKey, err)
		return nil, handleError(domain.NewValidationError("failed to parse username from authorization token"))
	}
	req := service.ConvertGoaToITXCreatePastMeetingAttachment(p, username)
	resp, err := s.itxPastMeetingAttachmentService.CreatePastMeetingAttachment(ctx, p.MeetingAndOccurrenceID, req)
	if err != nil {
		return nil, handleError(err)
	}
	return service.ConvertITXPastMeetingAttachmentToGoa(resp), nil
}

// GetItxPastMeetingAttachment retrieves a past meeting attachment via ITX proxy
func (s *MeetingsAPI) GetItxPastMeetingAttachment(ctx context.Context, p *meetingservice.GetItxPastMeetingAttachmentPayload) (*meetingservice.ITXPastMeetingAttachment, error) {
	resp, err := s.itxPastMeetingAttachmentService.GetPastMeetingAttachment(ctx, p.MeetingAndOccurrenceID, p.AttachmentID)
	if err != nil {
		return nil, handleError(err)
	}
	return service.ConvertITXPastMeetingAttachmentToGoa(resp), nil
}

// UpdateItxPastMeetingAttachment updates a past meeting attachment via ITX proxy
func (s *MeetingsAPI) UpdateItxPastMeetingAttachment(ctx context.Context, p *meetingservice.UpdateItxPastMeetingAttachmentPayload) error {
	username, err := s.authService.ParsePrincipal(ctx, *p.BearerToken, slog.Default())
	if err != nil {
		slog.WarnContext(ctx, "failed to parse username from JWT token", logging.ErrKey, err)
		return handleError(domain.NewValidationError("failed to parse username from authorization token"))
	}
	req := service.ConvertGoaToITXUpdatePastMeetingAttachment(p, username)
	err = s.itxPastMeetingAttachmentService.UpdatePastMeetingAttachment(ctx, p.MeetingAndOccurrenceID, p.AttachmentID, req)
	if err != nil {
		return handleError(err)
	}
	return nil
}

// DeleteItxPastMeetingAttachment deletes a past meeting attachment via ITX proxy
func (s *MeetingsAPI) DeleteItxPastMeetingAttachment(ctx context.Context, p *meetingservice.DeleteItxPastMeetingAttachmentPayload) error {
	err := s.itxPastMeetingAttachmentService.DeletePastMeetingAttachment(ctx, p.MeetingAndOccurrenceID, p.AttachmentID)
	if err != nil {
		return handleError(err)
	}
	return nil
}

// CreateItxPastMeetingAttachmentPresign generates a presigned URL for past meeting attachment upload via ITX proxy
func (s *MeetingsAPI) CreateItxPastMeetingAttachmentPresign(ctx context.Context, p *meetingservice.CreateItxPastMeetingAttachmentPresignPayload) (*meetingservice.ITXPastMeetingAttachmentPresignResponse, error) {
	username, err := s.authService.ParsePrincipal(ctx, *p.BearerToken, slog.Default())
	if err != nil {
		slog.WarnContext(ctx, "failed to parse username from JWT token", logging.ErrKey, err)
		return nil, handleError(domain.NewValidationError("failed to parse username from authorization token"))
	}
	req := service.ConvertGoaToITXCreatePastMeetingAttachmentPresign(p, username)
	resp, err := s.itxPastMeetingAttachmentService.CreatePastMeetingAttachmentPresignURL(ctx, p.MeetingAndOccurrenceID, req)
	if err != nil {
		return nil, handleError(err)
	}
	return service.ConvertITXPastMeetingAttachmentPresignToGoa(resp), nil
}

// GetItxPastMeetingAttachmentDownload generates a presigned URL for past meeting attachment download via ITX proxy
func (s *MeetingsAPI) GetItxPastMeetingAttachmentDownload(ctx context.Context, p *meetingservice.GetItxPastMeetingAttachmentDownloadPayload) (*meetingservice.ITXAttachmentDownloadResponse, error) {
	resp, err := s.itxPastMeetingAttachmentService.GetPastMeetingAttachmentDownloadURL(ctx, p.MeetingAndOccurrenceID, p.AttachmentID)
	if err != nil {
		return nil, handleError(err)
	}
	return service.ConvertITXAttachmentDownloadToGoa(resp), nil
}
