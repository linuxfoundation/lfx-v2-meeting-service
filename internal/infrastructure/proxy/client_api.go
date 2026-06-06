// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package proxy

import (
	"context"
	"net/http"

	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/models/itx"
)

func (c *Client) CreateMeetingAttachmentPresignURL(ctx context.Context, meetingID string, req *itx.CreateAttachmentPresignRequest) (*itx.MeetingAttachmentPresignResponse, error) {
	return doJSONTyped[itx.MeetingAttachmentPresignResponse](c, ctx, apiRequest{
		method:      http.MethodPost,
		path:        "/v2/zoom/meetings/%s/attachments/presign",
		pathArgs:    []any{meetingID},
		body:        req,
		accept:      acceptJSON,
		debugOp:     "CreateMeetingAttachmentPresignURL",
		debugFields: []any{"meetingID", meetingID},
		parseError:  "failed to parse ITX response",
	})
}

// GetMeetingAttachmentDownloadURL generates a presigned URL for meeting attachment download.
func (c *Client) GetMeetingAttachmentDownloadURL(ctx context.Context, meetingID, attachmentID string) (*itx.AttachmentDownloadResponse, error) {
	return doJSONTyped[itx.AttachmentDownloadResponse](c, ctx, apiRequest{
		method:      http.MethodGet,
		path:        "/v2/zoom/meetings/%s/attachments/%s/download",
		pathArgs:    []any{meetingID, attachmentID},
		accept:      acceptJSON,
		debugOp:     "GetMeetingAttachmentDownloadURL",
		debugFields: []any{"meetingID", meetingID, "attachmentID", attachmentID},
		parseError:  "failed to parse ITX response",
	})
}

// CreateMeetingAttachment creates a new meeting attachment.
func (c *Client) CreateMeetingAttachment(ctx context.Context, meetingID string, req *itx.CreateMeetingAttachmentRequest) (*itx.MeetingAttachment, error) {
	return doJSONTyped[itx.MeetingAttachment](c, ctx, apiRequest{
		method:      http.MethodPost,
		path:        "/v2/zoom/meetings/%s/attachments",
		pathArgs:    []any{meetingID},
		body:        req,
		accept:      acceptJSON,
		debugOp:     "CreateMeetingAttachment",
		debugFields: []any{"meetingID", meetingID},
		parseError:  "failed to parse ITX response",
	})
}

// GetMeetingAttachment retrieves a meeting attachment by ID.
func (c *Client) GetMeetingAttachment(ctx context.Context, meetingID, attachmentID string) (*itx.MeetingAttachment, error) {
	return doJSONTyped[itx.MeetingAttachment](c, ctx, apiRequest{
		method:      http.MethodGet,
		path:        "/v2/zoom/meetings/%s/attachments/%s",
		pathArgs:    []any{meetingID, attachmentID},
		accept:      acceptJSON,
		debugOp:     "GetMeetingAttachment",
		debugFields: []any{"meetingID", meetingID, "attachmentID", attachmentID},
		parseError:  "failed to parse ITX response",
	})
}

// UpdateMeetingAttachment updates a meeting attachment.
func (c *Client) UpdateMeetingAttachment(ctx context.Context, meetingID, attachmentID string, req *itx.UpdateMeetingAttachmentRequest) error {
	return c.doNoContent(ctx, apiRequest{
		method:      http.MethodPut,
		path:        "/v2/zoom/meetings/%s/attachments/%s",
		pathArgs:    []any{meetingID, attachmentID},
		body:        req,
		accept:      acceptJSON,
		debugOp:     "UpdateMeetingAttachment",
		debugFields: []any{"meetingID", meetingID, "attachmentID", attachmentID},
	})
}

// DeleteMeetingAttachment deletes a meeting attachment.
func (c *Client) DeleteMeetingAttachment(ctx context.Context, meetingID, attachmentID string) error {
	return c.doNoContent(ctx, apiRequest{
		method:      http.MethodDelete,
		path:        "/v2/zoom/meetings/%s/attachments/%s",
		pathArgs:    []any{meetingID, attachmentID},
		debugOp:     "DeleteMeetingAttachment",
		debugFields: []any{"meetingID", meetingID, "attachmentID", attachmentID},
	})
}

func (c *Client) CreatePastMeetingAttachmentPresignURL(ctx context.Context, meetingAndOccurrenceID string, req *itx.CreateAttachmentPresignRequest) (*itx.PastMeetingAttachmentPresignResponse, error) {
	return doJSONTyped[itx.PastMeetingAttachmentPresignResponse](c, ctx, apiRequest{
		method:      http.MethodPost,
		path:        "/v2/zoom/past_meetings/%s/attachments/presign",
		pathArgs:    []any{meetingAndOccurrenceID},
		body:        req,
		accept:      acceptJSON,
		debugOp:     "CreatePastMeetingAttachmentPresignURL",
		debugFields: []any{"meetingAndOccurrenceID", meetingAndOccurrenceID},
		parseError:  "failed to parse ITX response",
	})
}

// GetPastMeetingAttachmentDownloadURL generates a presigned URL for past meeting attachment download.
func (c *Client) GetPastMeetingAttachmentDownloadURL(ctx context.Context, meetingAndOccurrenceID, attachmentID string) (*itx.AttachmentDownloadResponse, error) {
	return doJSONTyped[itx.AttachmentDownloadResponse](c, ctx, apiRequest{
		method:      http.MethodGet,
		path:        "/v2/zoom/past_meetings/%s/attachments/%s/download",
		pathArgs:    []any{meetingAndOccurrenceID, attachmentID},
		accept:      acceptJSON,
		debugOp:     "GetPastMeetingAttachmentDownloadURL",
		debugFields: []any{"meetingAndOccurrenceID", meetingAndOccurrenceID, "attachmentID", attachmentID},
		parseError:  "failed to parse ITX response",
	})
}

// CreatePastMeetingAttachment creates a new past meeting attachment.
func (c *Client) CreatePastMeetingAttachment(ctx context.Context, meetingAndOccurrenceID string, req *itx.CreatePastMeetingAttachmentRequest) (*itx.PastMeetingAttachment, error) {
	return doJSONTyped[itx.PastMeetingAttachment](c, ctx, apiRequest{
		method:      http.MethodPost,
		path:        "/v2/zoom/past_meetings/%s/attachments",
		pathArgs:    []any{meetingAndOccurrenceID},
		body:        req,
		accept:      acceptJSON,
		debugOp:     "CreatePastMeetingAttachment",
		debugFields: []any{"meetingAndOccurrenceID", meetingAndOccurrenceID},
		parseError:  "failed to parse ITX response",
	})
}

// GetPastMeetingAttachment retrieves a past meeting attachment by ID.
func (c *Client) GetPastMeetingAttachment(ctx context.Context, meetingAndOccurrenceID, attachmentID string) (*itx.PastMeetingAttachment, error) {
	return doJSONTyped[itx.PastMeetingAttachment](c, ctx, apiRequest{
		method:      http.MethodGet,
		path:        "/v2/zoom/past_meetings/%s/attachments/%s",
		pathArgs:    []any{meetingAndOccurrenceID, attachmentID},
		accept:      acceptJSON,
		debugOp:     "GetPastMeetingAttachment",
		debugFields: []any{"meetingAndOccurrenceID", meetingAndOccurrenceID, "attachmentID", attachmentID},
		parseError:  "failed to parse ITX response",
	})
}

// UpdatePastMeetingAttachment updates a past meeting attachment.
func (c *Client) UpdatePastMeetingAttachment(ctx context.Context, meetingAndOccurrenceID, attachmentID string, req *itx.UpdatePastMeetingAttachmentRequest) error {
	return c.doNoContent(ctx, apiRequest{
		method:      http.MethodPut,
		path:        "/v2/zoom/past_meetings/%s/attachments/%s",
		pathArgs:    []any{meetingAndOccurrenceID, attachmentID},
		body:        req,
		accept:      acceptJSON,
		debugOp:     "UpdatePastMeetingAttachment",
		debugFields: []any{"meetingAndOccurrenceID", meetingAndOccurrenceID, "attachmentID", attachmentID},
	})
}

// DeletePastMeetingAttachment deletes a past meeting attachment.
func (c *Client) DeletePastMeetingAttachment(ctx context.Context, meetingAndOccurrenceID, attachmentID string) error {
	return c.doNoContent(ctx, apiRequest{
		method:      http.MethodDelete,
		path:        "/v2/zoom/past_meetings/%s/attachments/%s",
		pathArgs:    []any{meetingAndOccurrenceID, attachmentID},
		debugOp:     "DeletePastMeetingAttachment",
		debugFields: []any{"meetingAndOccurrenceID", meetingAndOccurrenceID, "attachmentID", attachmentID},
	})
}
