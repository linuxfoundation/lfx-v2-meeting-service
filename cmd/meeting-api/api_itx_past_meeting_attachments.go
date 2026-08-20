// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package main

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-meeting-service/cmd/meeting-api/service"
	meetingservice "github.com/linuxfoundation/lfx-v2-meeting-service/gen/meeting_service"
)

// CreateItxPastMeetingAttachment creates a past meeting attachment via ITX proxy.
// created_by is stamped by PastMeetingAttachmentService from the principal on ctx
// (populated by the JWT auth middleware), so we don't re-parse the token here.
func (s *MeetingsAPI) CreateItxPastMeetingAttachment(ctx context.Context, p *meetingservice.CreateItxPastMeetingAttachmentPayload) (*meetingservice.ITXPastMeetingAttachment, error) {
	req := service.ConvertGoaToITXCreatePastMeetingAttachment(p)
	resp, err := s.itxPastMeetingAttachmentService.CreatePastMeetingAttachment(ctx, p.MeetingAndOccurrenceID, req)
	if err != nil {
		return nil, handleError(ctx, err)
	}
	return service.ConvertITXPastMeetingAttachmentToGoa(resp), nil
}

// GetItxPastMeetingAttachment retrieves a past meeting attachment via ITX proxy
func (s *MeetingsAPI) GetItxPastMeetingAttachment(ctx context.Context, p *meetingservice.GetItxPastMeetingAttachmentPayload) (*meetingservice.ITXPastMeetingAttachment, error) {
	resp, err := s.itxPastMeetingAttachmentService.GetPastMeetingAttachment(ctx, p.MeetingAndOccurrenceID, p.AttachmentID)
	if err != nil {
		return nil, handleError(ctx, err)
	}
	return service.ConvertITXPastMeetingAttachmentToGoa(resp), nil
}

// UpdateItxPastMeetingAttachment updates a past meeting attachment via ITX proxy.
// updated_by is stamped by PastMeetingAttachmentService from the principal on ctx.
func (s *MeetingsAPI) UpdateItxPastMeetingAttachment(ctx context.Context, p *meetingservice.UpdateItxPastMeetingAttachmentPayload) error {
	req := service.ConvertGoaToITXUpdatePastMeetingAttachment(p)
	err := s.itxPastMeetingAttachmentService.UpdatePastMeetingAttachment(ctx, p.MeetingAndOccurrenceID, p.AttachmentID, req)
	if err != nil {
		return handleError(ctx, err)
	}
	return nil
}

// DeleteItxPastMeetingAttachment deletes a past meeting attachment via ITX proxy
func (s *MeetingsAPI) DeleteItxPastMeetingAttachment(ctx context.Context, p *meetingservice.DeleteItxPastMeetingAttachmentPayload) error {
	err := s.itxPastMeetingAttachmentService.DeletePastMeetingAttachment(ctx, p.MeetingAndOccurrenceID, p.AttachmentID)
	if err != nil {
		return handleError(ctx, err)
	}
	return nil
}

// CreateItxPastMeetingAttachmentPresign generates a presigned URL for past meeting
// attachment upload via ITX proxy. created_by is stamped by
// PastMeetingAttachmentService from the principal on ctx.
func (s *MeetingsAPI) CreateItxPastMeetingAttachmentPresign(ctx context.Context, p *meetingservice.CreateItxPastMeetingAttachmentPresignPayload) (*meetingservice.ITXPastMeetingAttachmentPresignResponse, error) {
	req := service.ConvertGoaToITXCreatePastMeetingAttachmentPresign(p)
	resp, err := s.itxPastMeetingAttachmentService.CreatePastMeetingAttachmentPresignURL(ctx, p.MeetingAndOccurrenceID, req)
	if err != nil {
		return nil, handleError(ctx, err)
	}
	return service.ConvertITXPastMeetingAttachmentPresignToGoa(resp), nil
}

// GetItxPastMeetingAttachmentDownload generates a presigned URL for past meeting attachment download via ITX proxy
func (s *MeetingsAPI) GetItxPastMeetingAttachmentDownload(ctx context.Context, p *meetingservice.GetItxPastMeetingAttachmentDownloadPayload) (*meetingservice.ITXAttachmentDownloadResponse, error) {
	resp, err := s.itxPastMeetingAttachmentService.GetPastMeetingAttachmentDownloadURL(ctx, p.MeetingAndOccurrenceID, p.AttachmentID)
	if err != nil {
		return nil, handleError(ctx, err)
	}
	return service.ConvertITXAttachmentDownloadToGoa(resp), nil
}
