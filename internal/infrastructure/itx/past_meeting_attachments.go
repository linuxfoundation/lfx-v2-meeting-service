// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package itx

import (
	"context"
	"net/http"

	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/models/itx"
)

// CreatePastMeetingAttachmentPresignURL generates a presigned URL for past meeting attachment upload.
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
