// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package itx

import (
	"context"
	"net/http"

	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/models/itx"
)

// CreatePastMeeting creates a past meeting record via ITX proxy.
func (c *Client) CreatePastMeeting(ctx context.Context, req *itx.CreatePastMeetingRequest) (*itx.PastMeetingResponse, error) {
	return doJSONTyped[itx.PastMeetingResponse](c, ctx, apiRequest{
		method:     http.MethodPost,
		path:       "/v2/zoom/past_meetings",
		body:       req,
		accept:     acceptJSON,
		parseError: "failed to unmarshal response",
	})
}

// GetPastMeeting retrieves a past meeting record via ITX proxy.
func (c *Client) GetPastMeeting(ctx context.Context, pastMeetingID string) (*itx.PastMeetingResponse, error) {
	return doJSONTyped[itx.PastMeetingResponse](c, ctx, apiRequest{
		method:     http.MethodGet,
		path:       "/v2/zoom/past_meetings/%s",
		pathArgs:   []any{pastMeetingID},
		accept:     acceptJSON,
		parseError: "failed to unmarshal response",
	})
}

// UpdatePastMeeting updates a past meeting record via ITX proxy.
// Returns nil on success (ITX API returns 204 No Content).
func (c *Client) UpdatePastMeeting(ctx context.Context, pastMeetingID string, req *itx.CreatePastMeetingRequest) (*itx.PastMeetingResponse, error) {
	return doJSONTypedOptional[itx.PastMeetingResponse](c, ctx, apiRequest{
		method:   http.MethodPut,
		path:     "/v2/zoom/past_meetings/%s",
		pathArgs: []any{pastMeetingID},
		body:     req,
		accept:   acceptJSON,
	})
}

// DeletePastMeeting deletes a past meeting record via ITX proxy.
func (c *Client) DeletePastMeeting(ctx context.Context, pastMeetingID string) error {
	return c.doNoContent(ctx, apiRequest{
		method:   http.MethodDelete,
		path:     "/v2/zoom/past_meetings/%s",
		pathArgs: []any{pastMeetingID},
	})
}

// GetPastMeetingSummary retrieves a past meeting summary from ITX.
func (c *Client) GetPastMeetingSummary(ctx context.Context, pastMeetingID, summaryID string) (*itx.PastMeetingSummaryResponse, error) {
	return doJSONTyped[itx.PastMeetingSummaryResponse](c, ctx, apiRequest{
		method:     http.MethodGet,
		path:       "/v2/zoom/past_meetings/%s/summaries/%s",
		pathArgs:   []any{pastMeetingID, summaryID},
		accept:     acceptJSON,
		parseError: "failed to unmarshal response",
	})
}

// UpdatePastMeetingSummary updates a past meeting summary in ITX.
func (c *Client) UpdatePastMeetingSummary(ctx context.Context, pastMeetingID, summaryID string, req *itx.UpdatePastMeetingSummaryRequest) (*itx.PastMeetingSummaryResponse, error) {
	return doJSONTyped[itx.PastMeetingSummaryResponse](c, ctx, apiRequest{
		method:     http.MethodPut,
		path:       "/v2/zoom/past_meetings/%s/summaries/%s",
		pathArgs:   []any{pastMeetingID, summaryID},
		body:       req,
		accept:     acceptJSON,
		parseError: "failed to unmarshal response",
	})
}
