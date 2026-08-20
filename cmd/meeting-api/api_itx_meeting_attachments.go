// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"log/slog"

	"github.com/linuxfoundation/lfx-v2-meeting-service/cmd/meeting-api/service"
	meetingservice "github.com/linuxfoundation/lfx-v2-meeting-service/gen/meeting_service"
)

// CreateItxMeetingAttachment creates a meeting attachment via ITX proxy.
// created_by is stamped by MeetingAttachmentService from the principal on ctx
// (populated by the JWT auth middleware), so we don't re-parse the token here.
func (s *MeetingsAPI) CreateItxMeetingAttachment(ctx context.Context, p *meetingservice.CreateItxMeetingAttachmentPayload) (*meetingservice.ITXMeetingAttachment, error) {
	req := service.ConvertGoaToITXCreateMeetingAttachment(p)
	resp, err := s.itxMeetingAttachmentService.CreateMeetingAttachment(ctx, p.MeetingID, req)
	if err != nil {
		return nil, handleError(ctx, err)
	}
	slog.InfoContext(ctx, "meeting attachment created", "meeting_id", p.MeetingID, "attachment_id", resp.ID)
	return service.ConvertITXMeetingAttachmentToGoa(resp), nil
}

// GetItxMeetingAttachment retrieves a meeting attachment via ITX proxy
func (s *MeetingsAPI) GetItxMeetingAttachment(ctx context.Context, p *meetingservice.GetItxMeetingAttachmentPayload) (*meetingservice.ITXMeetingAttachment, error) {
	resp, err := s.itxMeetingAttachmentService.GetMeetingAttachment(ctx, p.MeetingID, p.AttachmentID)
	if err != nil {
		return nil, handleError(ctx, err)
	}
	return service.ConvertITXMeetingAttachmentToGoa(resp), nil
}

// UpdateItxMeetingAttachment updates a meeting attachment via ITX proxy.
// updated_by is stamped by MeetingAttachmentService from the principal on ctx.
func (s *MeetingsAPI) UpdateItxMeetingAttachment(ctx context.Context, p *meetingservice.UpdateItxMeetingAttachmentPayload) error {
	req := service.ConvertGoaToITXUpdateMeetingAttachment(p)
	err := s.itxMeetingAttachmentService.UpdateMeetingAttachment(ctx, p.MeetingID, p.AttachmentID, req)
	if err != nil {
		return handleError(ctx, err)
	}
	slog.InfoContext(ctx, "meeting attachment updated", "meeting_id", p.MeetingID, "attachment_id", p.AttachmentID)
	return nil
}

// DeleteItxMeetingAttachment deletes a meeting attachment via ITX proxy
func (s *MeetingsAPI) DeleteItxMeetingAttachment(ctx context.Context, p *meetingservice.DeleteItxMeetingAttachmentPayload) error {
	err := s.itxMeetingAttachmentService.DeleteMeetingAttachment(ctx, p.MeetingID, p.AttachmentID)
	if err != nil {
		return handleError(ctx, err)
	}
	slog.InfoContext(ctx, "meeting attachment deleted", "meeting_id", p.MeetingID, "attachment_id", p.AttachmentID)
	return nil
}

// CreateItxMeetingAttachmentPresign generates a presigned URL for meeting attachment
// upload via ITX proxy. created_by is stamped by MeetingAttachmentService from the
// principal on ctx.
func (s *MeetingsAPI) CreateItxMeetingAttachmentPresign(ctx context.Context, p *meetingservice.CreateItxMeetingAttachmentPresignPayload) (*meetingservice.ITXMeetingAttachmentPresignResponse, error) {
	req := service.ConvertGoaToITXCreateMeetingAttachmentPresign(p)
	resp, err := s.itxMeetingAttachmentService.CreateMeetingAttachmentPresignURL(ctx, p.MeetingID, req)
	if err != nil {
		return nil, handleError(ctx, err)
	}
	return service.ConvertITXMeetingAttachmentPresignToGoa(resp), nil
}

// GetItxMeetingAttachmentDownload generates a presigned URL for meeting attachment download via ITX proxy
func (s *MeetingsAPI) GetItxMeetingAttachmentDownload(ctx context.Context, p *meetingservice.GetItxMeetingAttachmentDownloadPayload) (*meetingservice.ITXAttachmentDownloadResponse, error) {
	resp, err := s.itxMeetingAttachmentService.GetMeetingAttachmentDownloadURL(ctx, p.MeetingID, p.AttachmentID)
	if err != nil {
		return nil, handleError(ctx, err)
	}
	return service.ConvertITXAttachmentDownloadToGoa(resp), nil
}
